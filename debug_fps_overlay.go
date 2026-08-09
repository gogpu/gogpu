package gogpu

import (
	"encoding/binary"
	"fmt"
	"image"
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

// Compile-time interface check: fpsDebugOverlay must implement DebugOverlay.
var _ gpucontext.DebugOverlay = (*fpsDebugOverlay)(nil)

// fpsRingSize is the number of frame times kept for rolling statistics.
const fpsRingSize = 120

// fpsMinSamples is the minimum number of frame time samples required before
// rendering the FPS bar. Early frames have unreliable timing (startup latency,
// pipeline compilation) that would produce a misleading RED bar flash.
const fpsMinSamples = 10

// fpsLogInterval is the minimum duration between slog emissions.
const fpsLogInterval = time.Second

// fpsBarWidth is the fixed width of the FPS bar in physical pixels.
const fpsBarWidth = 60

// fpsBarMaxHeight is the maximum height of the FPS bar in physical pixels.
// A 60fps frame (16.67ms) fills about half the bar; slower frames grow taller.
const fpsBarMaxHeight = 100

// overlayNameFPS is the overlay identifier for the FPS debug overlay.
const overlayNameFPS = "fps"

// fpsDebugMode controls which FPS debug features are active.
type fpsDebugMode struct {
	overlay bool // Visual bar in top-right corner
	log     bool // Structured slog output every ~1 second
}

// parseFPSDebugMode parses GOGPU_DEBUG_FPS env var.
// Supports: "overlay", "log", "overlay,log", "1" (= overlay).
func parseFPSDebugMode() fpsDebugMode {
	val := os.Getenv("GOGPU_DEBUG_FPS")
	if val == "" {
		return fpsDebugMode{}
	}
	if val == "1" {
		return fpsDebugMode{overlay: true}
	}
	var mode fpsDebugMode
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
	fpsDebugOnce sync.Once
	fpsDebugConf fpsDebugMode
)

// getFPSDebugMode returns the parsed FPS debug configuration, cached
// after first call via sync.Once.
func getFPSDebugMode() fpsDebugMode {
	fpsDebugOnce.Do(func() {
		fpsDebugConf = parseFPSDebugMode()
	})
	return fpsDebugConf
}

// fpsDebugOverlay is the built-in FPS counter debug overlay implementing
// gpucontext.DebugOverlay. Renders a colored bar in the top-right corner
// showing frame time (GTK4 fpsoverlay.c:201 pattern).
//
// Bar color indicates performance:
//   - Green: 55+ FPS (healthy)
//   - Yellow: 30-54 FPS (warning)
//   - Red: below 30 FPS (critical)
//
// Pipeline resources are created lazily on first Draw call to avoid any GPU
// overhead when the overlay is not active.
type fpsDebugOverlay struct {
	// Frame time ring buffer.
	frameTimes [fpsRingSize]time.Duration
	ringIdx    int
	ringCount  int

	// Timing state.
	lastDraw time.Time
	lastLog  time.Time

	// Shared GPU pipeline resources -- lazy init on first Draw.
	*overlayPipeline

	// device is the GPU device for pipeline creation and uniform writes.
	device *wgpu.Device

	// surfaceFormat is the surface texture format for pipeline creation.
	surfaceFormat gputypes.TextureFormat

	// mode captures overlay vs log settings.
	mode fpsDebugMode
}

// Name returns the overlay identifier for registration and env var filtering.
func (o *fpsDebugOverlay) Name() string { return overlayNameFPS }

// Draw renders the FPS overlay for the current frame.
//
// Always returns true because the FPS counter needs continuous frames to
// maintain accurate statistics (self-sustaining render loop pattern).
func (o *fpsDebugOverlay) Draw(ctx gpucontext.DebugOverlayContext) bool {
	now := time.Now()

	// Record frame time delta.
	if !o.lastDraw.IsZero() {
		dt := now.Sub(o.lastDraw)
		o.frameTimes[o.ringIdx] = dt
		o.ringIdx = (o.ringIdx + 1) % fpsRingSize
		if o.ringCount < fpsRingSize {
			o.ringCount++
		}
	}
	o.lastDraw = now

	if o.mode.log {
		o.logFPS(now, ctx.FrameNumber)
	}

	if o.mode.overlay && o.ringCount >= fpsMinSamples {
		o.renderBar(ctx)
	}

	return true
}

// fpsStats holds computed frame time statistics.
type fpsStats struct {
	avg time.Duration
	min time.Duration
	max time.Duration
	fps float64
}

// computeStats calculates average, min, max frame time and FPS from the
// ring buffer.
func (o *fpsDebugOverlay) computeStats() fpsStats {
	if o.ringCount == 0 {
		return fpsStats{}
	}
	var total time.Duration
	minDt := o.frameTimes[0]
	maxDt := o.frameTimes[0]
	for i := 0; i < o.ringCount; i++ {
		dt := o.frameTimes[i]
		total += dt
		if dt < minDt {
			minDt = dt
		}
		if dt > maxDt {
			maxDt = dt
		}
	}
	avg := total / time.Duration(o.ringCount)
	fps := 0.0
	if avg > 0 {
		fps = float64(time.Second) / float64(avg)
	}
	return fpsStats{avg: avg, min: minDt, max: maxDt, fps: fps}
}

// logFPS emits structured slog output approximately every fpsLogInterval.
func (o *fpsDebugOverlay) logFPS(now time.Time, frameNumber uint64) {
	if now.Sub(o.lastLog) < fpsLogInterval {
		return
	}
	o.lastLog = now
	stats := o.computeStats()
	slog.Debug("gogpu: fps",
		"fps", fmt.Sprintf("%.1f", stats.fps),
		"frame_time_avg", fmt.Sprintf("%.1fms", float64(stats.avg.Microseconds())/1000.0),
		"min", fmt.Sprintf("%.1fms", float64(stats.min.Microseconds())/1000.0),
		"max", fmt.Sprintf("%.1fms", float64(stats.max.Microseconds())/1000.0),
		"frame", frameNumber,
	)
}

// renderBar draws the FPS bar (background + colored bar) in the top-right
// corner of the surface.
func (o *fpsDebugOverlay) renderBar(ctx gpucontext.DebugOverlayContext) {
	if o.overlayPipeline == nil || !o.inited {
		p, err := initOverlayPipeline(o.device, o.surfaceFormat, "FPS", damageOverlayShaderSource)
		if err != nil {
			slog.Error("gogpu: fps overlay pipeline init failed", "err", err)
			return
		}
		o.overlayPipeline = p
	}

	stats := o.computeStats()

	// Bar height proportional to frame time: 16.67ms -> ~50px, clamped to max.
	barHeight := float32(stats.avg.Seconds()*1000.0) * 3.0 // 3 px per ms
	if barHeight > fpsBarMaxHeight {
		barHeight = fpsBarMaxHeight
	}
	if barHeight < 2 {
		barHeight = 2
	}

	// Position: top-right corner with 8px margin.
	const margin = 8
	barX := float32(ctx.SurfaceWidth) - fpsBarWidth - margin
	barY := float32(margin)

	encoder := (*wgpu.CommandEncoder)(ctx.Encoder.Pointer())
	view := (*wgpu.TextureView)(ctx.SurfaceView.Pointer())

	// 1. Background quad (semi-transparent dark).
	bgRect := image.Rect(int(barX)-2, int(barY)-2, int(barX)+fpsBarWidth+2, int(barY)+fpsBarMaxHeight+2)
	o.drawQuad(encoder, view, ctx.SurfaceWidth, ctx.SurfaceHeight, bgRect, 0.0, 0.0, 0.0, 0.5)

	// 2. Colored bar: green >= 55fps, yellow 30-54, red < 30.
	var r, g, b float32
	switch {
	case stats.fps >= 55:
		r, g, b = 0.0, 0.8, 0.0 // green
	case stats.fps >= 30:
		r, g, b = 0.9, 0.7, 0.0 // yellow
	default:
		r, g, b = 0.9, 0.1, 0.0 // red
	}

	// Bar grows from bottom to top within the background area.
	barBottom := barY + fpsBarMaxHeight
	barTop := barBottom - barHeight
	barRect := image.Rect(int(barX), int(barTop), int(barX)+fpsBarWidth, int(barBottom))
	o.drawQuad(encoder, view, ctx.SurfaceWidth, ctx.SurfaceHeight, barRect, r, g, b, 0.7)
}

// drawQuad renders a single flat-color quad with the given rect and
// pre-multiplied RGBA color.
func (o *fpsDebugOverlay) drawQuad(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	surfW, surfH uint32,
	rect image.Rectangle,
	r, g, b, a float32,
) {
	// Pre-multiply color by alpha.
	pr := r * a
	pg := g * a
	pb := b * a

	// Write uniforms: rect(4f) + screen(2f) + pad(2f) + color(4f) = 48 bytes
	binary.LittleEndian.PutUint32(o.uniformData[0:4], math.Float32bits(float32(rect.Min.X)))
	binary.LittleEndian.PutUint32(o.uniformData[4:8], math.Float32bits(float32(rect.Min.Y)))
	binary.LittleEndian.PutUint32(o.uniformData[8:12], math.Float32bits(float32(rect.Dx())))
	binary.LittleEndian.PutUint32(o.uniformData[12:16], math.Float32bits(float32(rect.Dy())))
	binary.LittleEndian.PutUint32(o.uniformData[16:20], math.Float32bits(float32(surfW)))
	binary.LittleEndian.PutUint32(o.uniformData[20:24], math.Float32bits(float32(surfH)))
	// padding bytes 24-31 (zeroed at alloc, no write needed)
	binary.LittleEndian.PutUint32(o.uniformData[32:36], math.Float32bits(pr))
	binary.LittleEndian.PutUint32(o.uniformData[36:40], math.Float32bits(pg))
	binary.LittleEndian.PutUint32(o.uniformData[40:44], math.Float32bits(pb))
	binary.LittleEndian.PutUint32(o.uniformData[44:48], math.Float32bits(a))

	if err := o.device.Queue().WriteBuffer(o.uniformBuffer, 0, o.uniformData); err != nil {
		slog.Error("gogpu: fps overlay WriteBuffer failed", "err", err)
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
		slog.Error("gogpu: fps overlay BeginRenderPass failed", "err", err)
		return
	}

	renderPass.SetPipeline(o.pipeline)
	renderPass.SetBindGroup(0, o.uniformBindGrp, nil)
	renderPass.Draw(6, 1, 0, 0)

	if err := renderPass.End(); err != nil {
		slog.Error("gogpu: fps overlay End render pass failed", "err", err)
	}
}

// initFPSOverlayIfNeeded checks the GOGPU_DEBUG_FPS env var and
// auto-registers the FPS overlay on the given RenderTarget. Called once
// per surface during the first drawDebugOverlays call.
//
// This function is called from drawDebugOverlays in renderer.go. The overlay
// self-registers into the RenderTarget's debugOverlays list.
func initFPSOverlayIfNeeded(ws *RenderTarget) {
	mode := getFPSDebugMode()
	if !mode.overlay && !mode.log {
		return
	}
	// Check if already registered.
	for _, ov := range ws.debugOverlays {
		if ov.Name() == overlayNameFPS {
			return
		}
	}
	overlay := &fpsDebugOverlay{
		device:        ws.renderer.device,
		surfaceFormat: ws.renderer.surfaceFormat,
		mode:          mode,
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}
