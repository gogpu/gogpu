package gogpu

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// GPU statistics follow a two-level design validated by Rust wgpu
// (Device.get_internal_counters, feature-gated) and Flutter (FrameTiming):
//
//   - DeviceCounters — cumulative resource counts + bytes on the Renderer
//     (device owner). Cheap atomics; zero-cost when stats are disabled.
//   - FrameStats — per-frame upload/present metrics on the Context
//     (frame owner). Reset at the start of each frame.
//
// Enable with GOGPU_STATS=1 or EnableStats(). When disabled, record paths
// are a single atomic load and return immediately (Rust wgpu counters pattern).

var statsEnabled atomic.Bool

func init() {
	if os.Getenv("GOGPU_STATS") == "1" {
		statsEnabled.Store(true)
	}
}

// EnableStats turns on GPU statistics collection.
// Safe to call from any goroutine. Prefer GOGPU_STATS=1 for CI/prod toggles.
func EnableStats() { statsEnabled.Store(true) }

// DisableStats turns off GPU statistics collection.
// In-flight counters are not cleared; call ResetCounters on Renderer if needed.
func DisableStats() { statsEnabled.Store(false) }

// StatsEnabled reports whether GPU statistics collection is active.
func StatsEnabled() bool { return statsEnabled.Load() }

// DeviceCounters holds cumulative GPU resource statistics owned by Renderer.
// Matches Rust wgpu Device.get_internal_counters() shape at the app-framework
// layer (buffers/textures/bytes), without importing wgpu internals.
type DeviceCounters struct {
	// Textures is the number of live gogpu.Texture objects tracked by this renderer.
	Textures int64
	// TextureBytesAllocated is the sum of width*height*bytesPerPixel for live textures.
	TextureBytesAllocated int64
	// UploadBytesTotal is the cumulative number of bytes uploaded via UpdateData/UpdateRegion.
	UploadBytesTotal int64
	// UploadRegionsTotal is the cumulative number of UpdateRegion calls (UpdateData counts as 1).
	UploadRegionsTotal int64
}

// FrameStats holds per-frame upload and present metrics (Flutter FrameTiming pattern).
// Values accumulate during OnDraw and are snapshotted for the completed frame.
type FrameStats struct {
	// UploadBytes is the total bytes uploaded this frame (UpdateData + UpdateRegion).
	UploadBytes int64
	// UploadRegions is the number of upload operations this frame.
	UploadRegions int
	// PresentSkipped is true when the frame did not present to the swapchain
	// (no frame started, present failed, or idle path skipped the draw).
	PresentSkipped bool
	// FrameDuration is wall time from frame begin to EndFrame completion.
	// Zero when stats were disabled for the frame or timing was unavailable.
	FrameDuration time.Duration
}

// frameStatsState is the mutable per-renderer frame accumulator.
// Accessed only on the render thread when stats are enabled, but protected
// by a mutex so GetFrameStats / GetCounters remain safe for CI polling.
type frameStatsState struct {
	mu             sync.Mutex
	uploadBytes    int64
	uploadRegions  int
	presentSkipped bool
	frameStart     time.Time
	last           FrameStats
}

// GetCounters returns a snapshot of cumulative device-level counters.
// Returns a zero value when stats are disabled (callers should check StatsEnabled
// or treat zeros as "not collected").
func (r *Renderer) GetCounters() DeviceCounters {
	if r == nil || !statsEnabled.Load() {
		return DeviceCounters{}
	}
	return DeviceCounters{
		Textures:              r.statsTextures.Load(),
		TextureBytesAllocated: r.statsTextureBytes.Load(),
		UploadBytesTotal:      r.statsUploadBytes.Load(),
		UploadRegionsTotal:    r.statsUploadRegions.Load(),
	}
}

// ResetCounters zeroes cumulative device counters. Frame stats are unaffected.
// Intended for tests and long-running process observability windows.
func (r *Renderer) ResetCounters() {
	if r == nil {
		return
	}
	r.statsTextures.Store(0)
	r.statsTextureBytes.Store(0)
	r.statsUploadBytes.Store(0)
	r.statsUploadRegions.Store(0)
}

// GetFrameStats returns the last completed frame's FrameStats snapshot.
// During OnDraw, prefer Context.FrameStats() for the in-progress frame.
func (r *Renderer) GetFrameStats() FrameStats {
	if r == nil || !statsEnabled.Load() {
		return FrameStats{}
	}
	r.frameStats.mu.Lock()
	defer r.frameStats.mu.Unlock()
	return r.frameStats.last
}

// FrameStats returns the in-progress frame's upload/present metrics.
// Safe to call during OnDraw after texture uploads to assert upload budgets
// (e.g. CI: dirty rows → UploadBytes < 10% of full frame).
func (c *Context) FrameStats() FrameStats {
	if c == nil || c.renderer == nil || !statsEnabled.Load() {
		return FrameStats{}
	}
	fs := &c.renderer.frameStats
	fs.mu.Lock()
	defer fs.mu.Unlock()
	out := FrameStats{
		UploadBytes:    fs.uploadBytes,
		UploadRegions:  fs.uploadRegions,
		PresentSkipped: fs.presentSkipped,
	}
	if !fs.frameStart.IsZero() {
		out.FrameDuration = time.Since(fs.frameStart)
	}
	return out
}

func (r *Renderer) beginFrameStats() {
	if r == nil || !statsEnabled.Load() {
		return
	}
	fs := &r.frameStats
	fs.mu.Lock()
	fs.uploadBytes = 0
	fs.uploadRegions = 0
	fs.presentSkipped = false
	fs.frameStart = time.Now()
	fs.mu.Unlock()
}

func (r *Renderer) endFrameStats(presentSkipped bool) {
	if r == nil || !statsEnabled.Load() {
		return
	}
	fs := &r.frameStats
	fs.mu.Lock()
	if presentSkipped {
		fs.presentSkipped = true
	}
	last := FrameStats{
		UploadBytes:    fs.uploadBytes,
		UploadRegions:  fs.uploadRegions,
		PresentSkipped: fs.presentSkipped,
	}
	if !fs.frameStart.IsZero() {
		last.FrameDuration = time.Since(fs.frameStart)
	}
	fs.last = last
	fs.mu.Unlock()
}

func (r *Renderer) recordUpload(bytes int64, regions int) {
	if r == nil || !statsEnabled.Load() || bytes < 0 {
		return
	}
	if bytes > 0 {
		r.statsUploadBytes.Add(bytes)
	}
	if regions > 0 {
		r.statsUploadRegions.Add(int64(regions))
	}
	fs := &r.frameStats
	fs.mu.Lock()
	fs.uploadBytes += bytes
	fs.uploadRegions += regions
	fs.mu.Unlock()
}

func (r *Renderer) recordTextureAlloc(bytes int64) {
	if r == nil || !statsEnabled.Load() || bytes <= 0 {
		return
	}
	r.statsTextures.Add(1)
	r.statsTextureBytes.Add(bytes)
}

func (r *Renderer) recordTextureFree(bytes int64) {
	if r == nil || !statsEnabled.Load() || bytes <= 0 {
		return
	}
	r.statsTextures.Add(-1)
	r.statsTextureBytes.Add(-bytes)
}
