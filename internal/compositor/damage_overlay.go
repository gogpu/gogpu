package compositor

import (
	"image"
	"image/color"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Compile-time interface check: DamageDebugOverlay must implement DebugOverlay.
var _ gpucontext.DebugOverlay = (*DamageDebugOverlay)(nil)

// OverlayNameDamage is the overlay identifier for the damage debug overlay.
const OverlayNameDamage = "damage"

// DamageFlashDuration is the time a damage flash remains visible before fading
// out completely. Matches Chromium's ShowDebugBorders 400ms flash duration.
const DamageFlashDuration = 400 * time.Millisecond

// DamageDebugMode controls which damage debug features are active.
type DamageDebugMode struct {
	Overlay bool // Visual overlay with colored rects
	Log     bool // Structured slog output per frame
}

// ParseDamageDebugMode parses GOGPU_DEBUG_DAMAGE env var.
// Supports: "overlay", "log", "overlay,log".
func ParseDamageDebugMode() DamageDebugMode {
	val := os.Getenv("GOGPU_DEBUG_DAMAGE")
	if val == "" {
		return DamageDebugMode{}
	}
	var mode DamageDebugMode
	for _, part := range strings.Split(val, ",") {
		switch strings.TrimSpace(part) {
		case ModeOverlay:
			mode.Overlay = true
		case ModeLog:
			mode.Log = true
		}
	}
	return mode
}

var (
	damageDebugOnce sync.Once
	damageDebugConf DamageDebugMode
)

// GetDamageDebugMode returns the parsed damage debug configuration, cached
// after first call via sync.Once.
func GetDamageDebugMode() DamageDebugMode {
	damageDebugOnce.Do(func() {
		damageDebugConf = ParseDamageDebugMode()
	})
	return damageDebugConf
}

// DamageFlash tracks one active damage visualization with time-based fade.
type DamageFlash struct {
	Name   string
	Color  color.RGBA
	Rect   image.Rectangle
	Full   bool
	Reason gpucontext.DamageReason
	Time   time.Time
}

// DamageDebugOverlay is the built-in damage debug overlay implementing
// gpucontext.DebugOverlay. Renders flat-color quads showing per-source damage
// rects with a 400ms time-based fade effect.
//
// Pipeline resources are created lazily on first Draw call to avoid any GPU
// overhead when the overlay is not active. The overlay auto-registers with the
// compositor when GOGPU_DEBUG_DAMAGE=overlay is set.
type DamageDebugOverlay struct {
	// Flashes are the active flashes being rendered (fade in progress).
	Flashes []DamageFlash

	// CustomRenderer for text-enhanced overlay (registered by gg).
	// When non-nil, Draw delegates to this renderer instead of using the
	// built-in flat-color pipeline.
	CustomRenderer gpucontext.DamageOverlayRenderer

	// DamageSources is a reference to the RenderTarget's damage sources.
	// Set during auto-registration. The overlay reads snapshots before
	// sources are reset by present().
	DamageSources *[]*DamageSource

	// HasGPUWork is a reference to the RenderTarget's hasGPUWork flag.
	// When true and no external sources reported damage, the overlay
	// synthesizes a "gogpu" full-surface snapshot so built-in DrawTriangle
	// and similar calls are visible in the overlay.
	HasGPUWork *bool

	// ScaleFactor is the DPI scale factor for coordinate conversion.
	// Damage sources (e.g., gg) may report rects in logical coordinates,
	// while the overlay renders in physical surface pixels. Rects are scaled
	// by this factor before rendering.
	ScaleFactor float64

	// Shared GPU pipeline resources -- lazy init on first Draw.
	*OverlayPipeline

	// Device is the GPU device for pipeline creation and uniform writes.
	Device *wgpu.Device

	// SurfaceFormat is the surface texture format for pipeline creation.
	SurfaceFormat gputypes.TextureFormat

	// Mode captures overlay vs log settings.
	Mode DamageDebugMode
}

// Name returns the overlay identifier for registration and env var filtering.
func (o *DamageDebugOverlay) Name() string { return OverlayNameDamage }

// Draw renders the damage overlay for the current frame.
//
// Flow:
//  1. Collect DamageSourceSnapshot from registered sources.
//  2. Update flash state (prune expired, add new, refresh existing).
//  3. If custom renderer registered, delegate.
//     Otherwise render flat-color quads via built-in pipeline.
//  4. If log mode, emit slog.
//  5. Return true if any flashes still active (self-sustaining loop).
func (o *DamageDebugOverlay) Draw(ctx gpucontext.DebugOverlayContext) bool {
	snapshots := o.collectSnapshots()
	o.updateFlashes(snapshots)

	if o.Mode.Log {
		o.logDamage(snapshots, ctx.FrameNumber)
	}

	if o.Mode.Overlay {
		if o.CustomRenderer != nil {
			info := gpucontext.DamageOverlayInfo{
				Sources:       snapshots,
				FrameNumber:   ctx.FrameNumber,
				SurfaceWidth:  ctx.SurfaceWidth,
				SurfaceHeight: ctx.SurfaceHeight,
				Encoder:       ctx.Encoder,
				SurfaceView:   ctx.SurfaceView,
			}
			o.CustomRenderer.RenderDamageOverlay(info)
		} else {
			o.renderBuiltIn(ctx)
		}
	}

	return len(o.Flashes) > 0
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
func (o *DamageDebugOverlay) collectSnapshots() []gpucontext.DamageSourceSnapshot {
	// Synthetic snapshot for built-in GPU work without registered sources.
	if o.DamageSources == nil || len(*o.DamageSources) == 0 {
		if o.HasGPUWork != nil && *o.HasGPUWork {
			return []gpucontext.DamageSourceSnapshot{{
				Name:  "gogpu",
				Color: DamagePalette[0], // green
				Full:  true,
			}}
		}
		return nil
	}

	sources := *o.DamageSources
	snapshots := make([]gpucontext.DamageSourceSnapshot, len(sources))
	for i, ds := range sources {
		rects := append([]image.Rectangle(nil), ds.Rects...)
		// NOTE: gg's trackDamage() already scales logical -> physical via deviceScale.
		// Do NOT scale again here -- double scaling causes wrong overlay positions.
		snapshots[i] = gpucontext.DamageSourceSnapshot{
			Name:   ds.Name,
			Color:  ds.Color,
			Rects:  rects,
			Full:   ds.Full,
			Reason: ds.Reason,
		}
	}
	return snapshots
}

// updateFlashes prunes expired flashes, refreshes timestamps for
// still-active rects, and adds new flashes from the current frame's
// snapshots. "Refresh-or-create" prevents duplicate overlapping flashes
// for the same rect appearing across consecutive frames.
func (o *DamageDebugOverlay) updateFlashes(snapshots []gpucontext.DamageSourceSnapshot) {
	now := time.Now()

	// Prune expired flashes.
	alive := o.Flashes[:0]
	for _, f := range o.Flashes {
		if now.Sub(f.Time) < DamageFlashDuration {
			alive = append(alive, f)
		}
	}
	o.Flashes = alive

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
func (o *DamageDebugOverlay) refreshOrAddFlash(name string, c color.RGBA, rect image.Rectangle, full bool, reason gpucontext.DamageReason, now time.Time) {
	for i := range o.Flashes {
		f := &o.Flashes[i]
		if f.Name == name && f.Rect == rect && f.Full == full {
			f.Time = now
			f.Reason = reason
			return
		}
	}
	o.Flashes = append(o.Flashes, DamageFlash{
		Name:   name,
		Color:  c,
		Rect:   rect,
		Full:   full,
		Reason: reason,
		Time:   now,
	})
}

// renderBuiltIn renders flat-color quads for all active flashes using the
// built-in instanced GPU pipeline. All quads are packed into a single
// instance buffer and drawn with one Draw(6, N) call.
func (o *DamageDebugOverlay) renderBuiltIn(ctx gpucontext.DebugOverlayContext) {
	if len(o.Flashes) == 0 {
		return
	}

	if o.OverlayPipeline == nil || !o.Inited {
		p, err := InitOverlayPipeline(o.Device, o.SurfaceFormat, "Damage", OverlayShaderSource)
		if err != nil {
			slog.Error("gogpu: damage overlay pipeline init failed", "err", err)
			return
		}
		o.OverlayPipeline = p
	}

	now := time.Now()
	encoder := (*wgpu.CommandEncoder)(ctx.Encoder.Pointer())
	view := (*wgpu.TextureView)(ctx.SurfaceView.Pointer())

	// Pack all instances into a staging buffer.
	var instances []byte
	for i := range o.Flashes {
		f := &o.Flashes[i]
		alpha := o.fadeAlpha(f, now)
		if alpha <= 0 {
			continue
		}

		rect := f.Rect
		if f.Full {
			rect = image.Rect(0, 0, int(ctx.SurfaceWidth), int(ctx.SurfaceHeight))
		}
		if rect.Empty() {
			continue
		}

		// Pre-multiply color by fade alpha for fill quad.
		// Chromium FadedGreen(60) fill = ~24% max alpha; we use 18%.
		fillAlpha := alpha * 0.18
		r := float32(f.Color.R) / 255.0 * fillAlpha
		g := float32(f.Color.G) / 255.0 * fillAlpha
		b := float32(f.Color.B) / 255.0 * fillAlpha
		AppendInstance(&instances, rect, r, g, b, fillAlpha)

		// Border quads: 1px-wide strips at ~50% alpha (visible outline).
		const borderWidth = 1
		if rect.Dx() > borderWidth*2 && rect.Dy() > borderWidth*2 {
			borderAlpha := alpha * 0.5
			br := float32(f.Color.R) / 255.0 * borderAlpha
			bg := float32(f.Color.G) / 255.0 * borderAlpha
			bb := float32(f.Color.B) / 255.0 * borderAlpha

			// Top border
			AppendInstance(&instances, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+borderWidth), br, bg, bb, borderAlpha)
			// Bottom border
			AppendInstance(&instances, image.Rect(rect.Min.X, rect.Max.Y-borderWidth, rect.Max.X, rect.Max.Y), br, bg, bb, borderAlpha)
			// Left border (between top and bottom)
			AppendInstance(&instances, image.Rect(rect.Min.X, rect.Min.Y+borderWidth, rect.Min.X+borderWidth, rect.Max.Y-borderWidth), br, bg, bb, borderAlpha)
			// Right border (between top and bottom)
			AppendInstance(&instances, image.Rect(rect.Max.X-borderWidth, rect.Min.Y+borderWidth, rect.Max.X, rect.Max.Y-borderWidth), br, bg, bb, borderAlpha)
		}
	}

	// One render pass, one draw call for all quads.
	o.RenderInstances(o.Device, encoder, view, ctx.SurfaceWidth, ctx.SurfaceHeight, instances)
}

// fadeAlpha computes the current alpha for a flash based on elapsed time.
// Returns 1.0 at flash start, linearly fading to 0.0 at DamageFlashDuration.
func (o *DamageDebugOverlay) fadeAlpha(f *DamageFlash, now time.Time) float32 {
	elapsed := now.Sub(f.Time)
	if elapsed >= DamageFlashDuration {
		return 0
	}
	return 1.0 - float32(elapsed.Seconds()/DamageFlashDuration.Seconds())
}

// logDamage emits structured slog output for the current frame's damage.
func (o *DamageDebugOverlay) logDamage(snapshots []gpucontext.DamageSourceSnapshot, frameNumber uint64) {
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
