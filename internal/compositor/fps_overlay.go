package compositor

import (
	"fmt"
	"image"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Compile-time interface check: FPSDebugOverlay must implement DebugOverlay.
var _ gpucontext.DebugOverlay = (*FPSDebugOverlay)(nil)

// FPSRingSize is the number of frame times kept for rolling statistics.
const FPSRingSize = 120

// FPSMinSamples is the minimum number of frame time samples required before
// rendering the FPS bar. Early frames have unreliable timing (startup latency,
// pipeline compilation) that would produce a misleading RED bar flash.
const FPSMinSamples = 10

// FPSLogInterval is the minimum duration between slog emissions.
const FPSLogInterval = time.Second

// FPSBarWidth is the fixed width of the FPS bar in physical pixels.
const FPSBarWidth = 60

// FPSBarMaxHeight is the maximum height of the FPS bar in physical pixels.
// A 60fps frame (16.67ms) fills about half the bar; slower frames grow taller.
const FPSBarMaxHeight = 100

// OverlayNameFPS is the overlay identifier for the FPS debug overlay.
const OverlayNameFPS = "fps"

// FPSDebugMode controls which FPS debug features are active.
type FPSDebugMode struct {
	Overlay bool // Visual bar in top-right corner
	Log     bool // Structured slog output every ~1 second
}

// ParseFPSDebugMode parses GOGPU_DEBUG_FPS env var.
// Supports: "overlay", "log", "overlay,log", "1" (= overlay).
func ParseFPSDebugMode() FPSDebugMode {
	val := os.Getenv("GOGPU_DEBUG_FPS")
	if val == "" {
		return FPSDebugMode{}
	}
	if val == "1" {
		return FPSDebugMode{Overlay: true}
	}
	var mode FPSDebugMode
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
	fpsDebugOnce sync.Once
	fpsDebugConf FPSDebugMode
)

// GetFPSDebugMode returns the parsed FPS debug configuration, cached
// after first call via sync.Once.
func GetFPSDebugMode() FPSDebugMode {
	fpsDebugOnce.Do(func() {
		fpsDebugConf = ParseFPSDebugMode()
	})
	return fpsDebugConf
}

// FPSDebugOverlay is the built-in FPS counter debug overlay implementing
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
type FPSDebugOverlay struct {
	// Frame time ring buffer.
	frameTimes [FPSRingSize]time.Duration
	ringIdx    int
	ringCount  int

	// Timing state.
	lastDraw time.Time
	lastLog  time.Time

	// Shared GPU pipeline resources -- lazy init on first Draw.
	*OverlayPipeline

	// Device is the GPU device for pipeline creation and uniform writes.
	Device *wgpu.Device

	// SurfaceFormat is the surface texture format for pipeline creation.
	SurfaceFormat gputypes.TextureFormat

	// Mode captures overlay vs log settings.
	Mode FPSDebugMode
}

// Name returns the overlay identifier for registration and env var filtering.
func (o *FPSDebugOverlay) Name() string { return OverlayNameFPS }

// Draw renders the FPS overlay for the current frame.
//
// Returns true when the visual overlay is active, requesting another frame
// so the FPS counter stays updated (self-sustaining render loop pattern).
// Returns false in log-only mode since no visual update is needed.
func (o *FPSDebugOverlay) Draw(ctx gpucontext.DebugOverlayContext) bool {
	now := time.Now()

	// Record frame time delta.
	if !o.lastDraw.IsZero() {
		dt := now.Sub(o.lastDraw)
		o.frameTimes[o.ringIdx] = dt
		o.ringIdx = (o.ringIdx + 1) % FPSRingSize
		if o.ringCount < FPSRingSize {
			o.ringCount++
		}
	}
	o.lastDraw = now

	if o.Mode.Log {
		o.logFPS(now, ctx.FrameNumber)
	}

	if o.Mode.Overlay && o.ringCount >= FPSMinSamples {
		o.renderBar(ctx)
	}

	return o.Mode.Overlay
}

// FPSStats holds computed frame time statistics.
type FPSStats struct {
	Avg time.Duration
	Min time.Duration
	Max time.Duration
	FPS float64
}

// computeStats calculates average, min, max frame time and FPS from the
// ring buffer.
func (o *FPSDebugOverlay) computeStats() FPSStats {
	if o.ringCount == 0 {
		return FPSStats{}
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
	return FPSStats{Avg: avg, Min: minDt, Max: maxDt, FPS: fps}
}

// logFPS emits structured slog output approximately every FPSLogInterval.
func (o *FPSDebugOverlay) logFPS(now time.Time, frameNumber uint64) {
	if now.Sub(o.lastLog) < FPSLogInterval {
		return
	}
	o.lastLog = now
	stats := o.computeStats()
	slog.Debug("gogpu: fps",
		"fps", fmt.Sprintf("%.1f", stats.FPS),
		"frame_time_avg", fmt.Sprintf("%.1fms", float64(stats.Avg.Microseconds())/1000.0),
		"min", fmt.Sprintf("%.1fms", float64(stats.Min.Microseconds())/1000.0),
		"max", fmt.Sprintf("%.1fms", float64(stats.Max.Microseconds())/1000.0),
		"frame", frameNumber,
	)
}

// renderBar draws the FPS bar (background + colored bar) in the top-right
// corner of the surface via instanced draw (2 instances, 1 draw call).
func (o *FPSDebugOverlay) renderBar(ctx gpucontext.DebugOverlayContext) {
	if o.OverlayPipeline == nil || !o.Inited {
		p, err := InitOverlayPipeline(o.Device, o.SurfaceFormat, "FPS", OverlayShaderSource)
		if err != nil {
			slog.Error("gogpu: fps overlay pipeline init failed", "err", err)
			return
		}
		o.OverlayPipeline = p
	}

	stats := o.computeStats()

	// Bar height proportional to frame time: 16.67ms -> ~50px, clamped to max.
	barHeight := float32(stats.Avg.Seconds()*1000.0) * 3.0 // 3 px per ms
	if barHeight > FPSBarMaxHeight {
		barHeight = FPSBarMaxHeight
	}
	if barHeight < 2 {
		barHeight = 2
	}

	// Position: top-right corner with 8px margin.
	const margin = 8
	barX := float32(ctx.SurfaceWidth) - FPSBarWidth - margin
	barY := float32(margin)

	encoder := (*wgpu.CommandEncoder)(ctx.Encoder.Pointer())
	view := (*wgpu.TextureView)(ctx.SurfaceView.Pointer())

	// Pack both quads as instances.
	var instances []byte

	// 1. Background quad (semi-transparent dark, pre-multiplied: 0*0.5=0).
	bgRect := image.Rect(int(barX)-2, int(barY)-2, int(barX)+FPSBarWidth+2, int(barY)+FPSBarMaxHeight+2)
	AppendInstance(&instances, bgRect, 0.0, 0.0, 0.0, 0.5)

	// 2. Colored bar: green >= 55fps, yellow 30-54, red < 30.
	var r, g, b float32
	switch {
	case stats.FPS >= 55:
		r, g, b = 0.0, 0.8, 0.0 // green
	case stats.FPS >= 30:
		r, g, b = 0.9, 0.7, 0.0 // yellow
	default:
		r, g, b = 0.9, 0.1, 0.0 // red
	}

	// Bar grows from bottom to top within the background area.
	barBottom := barY + FPSBarMaxHeight
	barTop := barBottom - barHeight
	barRect := image.Rect(int(barX), int(barTop), int(barX)+FPSBarWidth, int(barBottom))
	// Pre-multiply color by alpha (0.7).
	const barAlpha = 0.7
	AppendInstance(&instances, barRect, r*barAlpha, g*barAlpha, b*barAlpha, barAlpha)

	// One render pass, one draw call for both quads.
	o.RenderInstances(o.Device, encoder, view, ctx.SurfaceWidth, ctx.SurfaceHeight, instances)
}
