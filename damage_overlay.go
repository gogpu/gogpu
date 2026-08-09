package gogpu

import (
	"encoding/binary"
	"image"
	"image/color"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Compile-time interface check: damageDebugOverlay must implement DebugOverlay.
var _ gpucontext.DebugOverlay = (*damageDebugOverlay)(nil)

// damageOverlayUniformSize is the size of the uniform buffer for the damage
// overlay shader. Layout: rect(4) + screen(2) + pad(2) + color(4) = 48 bytes.
const damageOverlayUniformSize = 48

// overlayNameDamage is the overlay identifier for the damage debug overlay.
const overlayNameDamage = "damage"

// damageFlashDuration is the time a damage flash remains visible before fading
// out completely. Matches Chromium's ShowDebugBorders 400ms flash duration.
const damageFlashDuration = 400 * time.Millisecond

// damageDebugMode controls which damage debug features are active.
type damageDebugMode struct {
	overlay bool // Visual overlay with colored rects
	log     bool // Structured slog output per frame
}

// parseDamageDebugMode parses GOGPU_DEBUG_DAMAGE env var.
// Supports: "overlay", "log", "overlay,log".
func parseDamageDebugMode() damageDebugMode {
	val := os.Getenv("GOGPU_DEBUG_DAMAGE")
	if val == "" {
		return damageDebugMode{}
	}
	var mode damageDebugMode
	for _, part := range strings.Split(val, ",") {
		switch strings.TrimSpace(part) {
		case overlayModeOverlay:
			mode.overlay = true
		case overlayModeLog:
			mode.log = true
		}
	}
	return mode
}

var (
	damageDebugOnce sync.Once
	damageDebugConf damageDebugMode
)

// getDamageDebugMode returns the parsed damage debug configuration, cached
// after first call via sync.Once.
func getDamageDebugMode() damageDebugMode {
	damageDebugOnce.Do(func() {
		damageDebugConf = parseDamageDebugMode()
	})
	return damageDebugConf
}

// damageFlash tracks one active damage visualization with time-based fade.
type damageFlash struct {
	name   string
	color  color.RGBA
	rect   image.Rectangle
	full   bool
	reason gpucontext.DamageReason
	time   time.Time
}

// damageDebugOverlay is the built-in damage debug overlay implementing
// gpucontext.DebugOverlay. Renders flat-color quads showing per-source damage
// rects with a 400ms time-based fade effect.
//
// Pipeline resources are created lazily on first Draw call to avoid any GPU
// overhead when the overlay is not active. The overlay auto-registers with the
// compositor when GOGPU_DEBUG_DAMAGE=overlay is set.
type damageDebugOverlay struct {
	// Active flashes being rendered (fade in progress).
	flashes []damageFlash

	// Custom renderer for text-enhanced overlay (registered by gg).
	// When non-nil, Draw delegates to this renderer instead of using the
	// built-in flat-color pipeline.
	customRenderer gpucontext.DamageOverlayRenderer

	// damageSources is a reference to the RenderTarget's damage sources.
	// Set during auto-registration. The overlay reads snapshots before
	// sources are reset by present().
	damageSources *[]*DamageSource

	// hasGPUWork is a reference to the RenderTarget's hasGPUWork flag.
	// When true and no external sources reported damage, the overlay
	// synthesizes a "gogpu" full-surface snapshot so built-in DrawTriangle
	// and similar calls are visible in the overlay.
	hasGPUWork *bool

	// scaleFactor is the DPI scale factor for coordinate conversion.
	// Damage sources (e.g., gg) may report rects in logical coordinates,
	// while the overlay renders in physical surface pixels. Rects are scaled
	// by this factor before rendering.
	scaleFactor float64

	// Shared GPU pipeline resources — lazy init on first Draw.
	*overlayPipeline

	// device is the GPU device for pipeline creation and uniform writes.
	device *wgpu.Device

	// surfaceFormat is the surface texture format for pipeline creation.
	surfaceFormat gputypes.TextureFormat

	// mode captures overlay vs log settings.
	mode damageDebugMode
}

// Name returns the overlay identifier for registration and env var filtering.
func (o *damageDebugOverlay) Name() string { return overlayNameDamage }

// Draw renders the damage overlay for the current frame.
//
// Flow:
//  1. Collect DamageSourceSnapshot from registered sources.
//  2. Update flash state (prune expired, add new, refresh existing).
//  3. If custom renderer registered, delegate.
//     Otherwise render flat-color quads via built-in pipeline.
//  4. If log mode, emit slog.
//  5. Return true if any flashes still active (self-sustaining loop).
func (o *damageDebugOverlay) Draw(ctx gpucontext.DebugOverlayContext) bool {
	snapshots := o.collectSnapshots()
	o.updateFlashes(snapshots)

	if o.mode.log {
		o.logDamage(snapshots, ctx.FrameNumber)
	}

	if o.mode.overlay {
		if o.customRenderer != nil {
			info := gpucontext.DamageOverlayInfo{
				Sources:       snapshots,
				FrameNumber:   ctx.FrameNumber,
				SurfaceWidth:  ctx.SurfaceWidth,
				SurfaceHeight: ctx.SurfaceHeight,
				Encoder:       ctx.Encoder,
				SurfaceView:   ctx.SurfaceView,
			}
			o.customRenderer.RenderDamageOverlay(info)
		} else {
			o.renderBuiltIn(ctx)
		}
	}

	return len(o.flashes) > 0
}

// collectSnapshots reads current damage state from all registered sources.
// Called before present() resets the sources.
//
// When no external damage sources are registered but the surface has GPU work
// (e.g., DrawTriangle), a synthetic "gogpu" full-surface snapshot is generated
// so the overlay has something to display. This ensures built-in rendering
// examples show damage feedback without requiring explicit DamageSource registration.
//
// Damage rects from sources may be in logical coordinates (e.g., gg reports in
// user space). When scaleFactor > 1.0, rects are scaled to physical surface
// pixels for correct overlay positioning on HiDPI/Retina displays.
func (o *damageDebugOverlay) collectSnapshots() []gpucontext.DamageSourceSnapshot {
	// Synthetic snapshot for built-in GPU work without registered sources.
	if o.damageSources == nil || len(*o.damageSources) == 0 {
		if o.hasGPUWork != nil && *o.hasGPUWork {
			return []gpucontext.DamageSourceSnapshot{{
				Name:  "gogpu",
				Color: damagePalette[0], // green
				Full:  true,
			}}
		}
		return nil
	}

	sources := *o.damageSources
	snapshots := make([]gpucontext.DamageSourceSnapshot, len(sources))
	for i, ds := range sources {
		rects := append([]image.Rectangle(nil), ds.rects...)
		// NOTE: gg's trackDamage() already scales logical → physical via deviceScale.
		// Do NOT scale again here — double scaling causes wrong overlay positions.
		snapshots[i] = gpucontext.DamageSourceSnapshot{
			Name:   ds.name,
			Color:  ds.color,
			Rects:  rects,
			Full:   ds.full,
			Reason: ds.reason,
		}
	}
	return snapshots
}

// updateFlashes prunes expired flashes, refreshes timestamps for
// still-active rects, and adds new flashes from the current frame's
// snapshots. "Refresh-or-create" prevents duplicate overlapping flashes
// for the same rect appearing across consecutive frames.
func (o *damageDebugOverlay) updateFlashes(snapshots []gpucontext.DamageSourceSnapshot) {
	now := time.Now()

	// Prune expired flashes.
	alive := o.flashes[:0]
	for _, f := range o.flashes {
		if now.Sub(f.time) < damageFlashDuration {
			alive = append(alive, f)
		}
	}
	o.flashes = alive

	// Add or refresh flashes from current snapshots.
	for _, snap := range snapshots {
		if snap.Full {
			o.refreshOrAddFlash(snap.Name, snap.Color, image.Rectangle{}, true, snap.Reason, now)
			continue
		}
		for _, r := range snap.Rects {
			o.refreshOrAddFlash(snap.Name, snap.Color, r, false, snap.Reason, now)
		}
	}
}

// refreshOrAddFlash refreshes the timestamp of an existing flash with
// matching name+rect, or creates a new one. This prevents duplicate
// flashes for the same damage area across consecutive frames.
func (o *damageDebugOverlay) refreshOrAddFlash(name string, c color.RGBA, rect image.Rectangle, full bool, reason gpucontext.DamageReason, now time.Time) {
	for i := range o.flashes {
		f := &o.flashes[i]
		if f.name == name && f.rect == rect && f.full == full {
			f.time = now
			f.reason = reason
			return
		}
	}
	o.flashes = append(o.flashes, damageFlash{
		name:   name,
		color:  c,
		rect:   rect,
		full:   full,
		reason: reason,
		time:   now,
	})
}

// renderBuiltIn renders flat-color quads for all active flashes using the
// built-in GPU pipeline.
func (o *damageDebugOverlay) renderBuiltIn(ctx gpucontext.DebugOverlayContext) {
	if len(o.flashes) == 0 {
		return
	}

	if o.overlayPipeline == nil || !o.inited {
		p, err := initOverlayPipeline(o.device, o.surfaceFormat, "Damage", damageOverlayShaderSource)
		if err != nil {
			slog.Error("gogpu: damage overlay pipeline init failed", "err", err)
			return
		}
		o.overlayPipeline = p
	}

	now := time.Now()
	encoder := (*wgpu.CommandEncoder)(ctx.Encoder.Pointer())
	view := (*wgpu.TextureView)(ctx.SurfaceView.Pointer())

	for i := range o.flashes {
		f := &o.flashes[i]
		alpha := o.fadeAlpha(f, now)
		if alpha <= 0 {
			continue
		}

		rect := f.rect
		if f.full {
			rect = image.Rect(0, 0, int(ctx.SurfaceWidth), int(ctx.SurfaceHeight))
		}
		if rect.Empty() {
			continue
		}

		// Single fill rect with moderate alpha. Border rendering requires
		// separate uniform buffers per draw (GPU race on shared buffer).
		// Borders will be added by gg text-enhanced overlay (DamageOverlayRenderer).
		// Chromium FadedGreen(60) fill = ~24% max alpha.
		fillAlpha := alpha * 0.18
		o.drawRect(encoder, view, ctx.SurfaceWidth, ctx.SurfaceHeight, rect, f.color, fillAlpha)
	}
}

// fadeAlpha computes the current alpha for a flash based on elapsed time.
// Returns 1.0 at flash start, linearly fading to 0.0 at damageFlashDuration.
func (o *damageDebugOverlay) fadeAlpha(f *damageFlash, now time.Time) float32 {
	elapsed := now.Sub(f.time)
	if elapsed >= damageFlashDuration {
		return 0
	}
	return 1.0 - float32(elapsed.Seconds()/damageFlashDuration.Seconds())
}

// drawRect renders a single flat-color quad with the given rect and faded color.
func (o *damageDebugOverlay) drawRect(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	surfW, surfH uint32,
	rect image.Rectangle,
	c color.RGBA,
	alpha float32,
) {
	// Pre-multiply color by the caller-provided alpha.
	// For fill: alpha ~0.12 (subtle tint). For border: alpha ~0.7 (visible outline).
	r := float32(c.R) / 255.0 * alpha
	g := float32(c.G) / 255.0 * alpha
	b := float32(c.B) / 255.0 * alpha
	baseAlpha := alpha

	// Write uniforms: rect(4f) + screen(2f) + pad(2f) + color(4f) = 48 bytes
	binary.LittleEndian.PutUint32(o.uniformData[0:4], math.Float32bits(float32(rect.Min.X)))
	binary.LittleEndian.PutUint32(o.uniformData[4:8], math.Float32bits(float32(rect.Min.Y)))
	binary.LittleEndian.PutUint32(o.uniformData[8:12], math.Float32bits(float32(rect.Dx())))
	binary.LittleEndian.PutUint32(o.uniformData[12:16], math.Float32bits(float32(rect.Dy())))
	binary.LittleEndian.PutUint32(o.uniformData[16:20], math.Float32bits(float32(surfW)))
	binary.LittleEndian.PutUint32(o.uniformData[20:24], math.Float32bits(float32(surfH)))
	// padding bytes 24-31 (zeroed at alloc, no write needed)
	binary.LittleEndian.PutUint32(o.uniformData[32:36], math.Float32bits(r))
	binary.LittleEndian.PutUint32(o.uniformData[36:40], math.Float32bits(g))
	binary.LittleEndian.PutUint32(o.uniformData[40:44], math.Float32bits(b))
	binary.LittleEndian.PutUint32(o.uniformData[44:48], math.Float32bits(baseAlpha))

	if err := o.device.Queue().WriteBuffer(o.uniformBuffer, 0, o.uniformData); err != nil {
		slog.Error("gogpu: damage overlay WriteBuffer failed", "err", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    view,
				LoadOp:  gputypes.LoadOpLoad,
				StoreOp: gputypes.StoreOpStore,
			},
		},
	})
	if err != nil {
		slog.Error("gogpu: damage overlay BeginRenderPass failed", "err", err)
		return
	}

	renderPass.SetPipeline(o.pipeline)
	renderPass.SetBindGroup(0, o.uniformBindGrp, nil)
	renderPass.Draw(6, 1, 0, 0)

	if err := renderPass.End(); err != nil {
		slog.Error("gogpu: damage overlay End render pass failed", "err", err)
	}
}

// logDamage emits structured slog output for the current frame's damage.
func (o *damageDebugOverlay) logDamage(snapshots []gpucontext.DamageSourceSnapshot, frameNumber uint64) {
	if len(snapshots) == 0 {
		return
	}

	totalRects := 0
	totalArea := 0
	activeSources := 0
	for _, snap := range snapshots {
		if snap.Full || len(snap.Rects) > 0 {
			activeSources++
		}
		if snap.Full {
			totalRects++
		} else {
			totalRects += len(snap.Rects)
			for _, r := range snap.Rects {
				totalArea += r.Dx() * r.Dy()
			}
		}
	}

	if activeSources == 0 {
		return
	}

	slog.Debug("gogpu: damage",
		"frame", frameNumber,
		"sources", activeSources,
		"total_rects", totalRects,
		"total_area_px", totalArea,
	)
	for _, snap := range snapshots {
		if !snap.Full && len(snap.Rects) == 0 {
			continue
		}
		attrs := []any{
			"source", snap.Name,
		}
		if snap.Full {
			attrs = append(attrs, "full", true)
		} else {
			area := 0
			for _, r := range snap.Rects {
				area += r.Dx() * r.Dy()
			}
			attrs = append(attrs, "rects", len(snap.Rects), "area_px", area)
		}
		if snap.Reason.Category != gpucontext.DamageCategoryContent || snap.Reason.Detail != "" {
			attrs = append(attrs, "category", snap.Reason.Category.String())
			if snap.Reason.Detail != "" {
				attrs = append(attrs, "reason", snap.Reason.Detail)
			}
		}
		slog.Debug("gogpu: damage source", attrs...)
	}
}

// initDamageOverlayIfNeeded checks the GOGPU_DEBUG_DAMAGE env var and
// auto-registers the damage overlay on the given RenderTarget. Called once
// per surface during the first frame that has damage sources.
//
// This function is called from drawDebugOverlays in renderer.go. The overlay
// self-registers into the RenderTarget's debugOverlays list.
func initDamageOverlayIfNeeded(ws *RenderTarget) {
	mode := getDamageDebugMode()
	if !mode.overlay && !mode.log {
		return
	}
	// Check if already registered.
	for _, ov := range ws.debugOverlays {
		if ov.Name() == overlayNameDamage {
			return
		}
	}
	scale := 1.0
	if ws.platWindow != nil {
		if s := ws.platWindow.ScaleFactor(); s > 0 {
			scale = s
		}
	}
	overlay := &damageDebugOverlay{
		damageSources: &ws.damageSources,
		hasGPUWork:    &ws.hasGPUWork,
		scaleFactor:   scale,
		device:        ws.renderer.device,
		surfaceFormat: ws.renderer.surfaceFormat,
		mode:          mode,
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}

// setCustomDamageRenderer sets or replaces the custom damage overlay renderer.
// When set, the overlay delegates rendering to this renderer instead of using
// the built-in flat-color pipeline. If the overlay is not yet registered, this
// is stored for later use.
func (ws *RenderTarget) setCustomDamageRenderer(renderer gpucontext.DamageOverlayRenderer) {
	for _, ov := range ws.debugOverlays {
		if dov, ok := ov.(*damageDebugOverlay); ok {
			dov.customRenderer = renderer
			return
		}
	}
	// Overlay not registered yet. Force-register it with overlay mode so
	// the custom renderer can take effect when drawing starts.
	mode := getDamageDebugMode()
	mode.overlay = true
	scale := 1.0
	if ws.platWindow != nil {
		if s := ws.platWindow.ScaleFactor(); s > 0 {
			scale = s
		}
	}
	overlay := &damageDebugOverlay{
		damageSources:  &ws.damageSources,
		hasGPUWork:     &ws.hasGPUWork,
		scaleFactor:    scale,
		device:         ws.renderer.device,
		surfaceFormat:  ws.renderer.surfaceFormat,
		mode:           mode,
		customRenderer: renderer,
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}
