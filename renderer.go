package gogpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const (
	shaderEntryVS = "vs_main"
	shaderEntryFS = "fs_main"

	// texQuadUniformSize is the size of the uniform buffer for textured quads.
	// Layout: rect(4 floats) + screen(2 floats) + alpha(1 float) + premultiplied(1 float) = 32 bytes
	texQuadUniformSize = 32
)

// texQuadUniformStride is the dynamic-uniform alignment required by WebGPU's
// minimum uniform-buffer offset limit. Keeping each draw in its own aligned
// slot is required once textured quads share a command encoder: a queue write
// must not overwrite the parameters recorded for an earlier pass.
const texQuadUniformStride = 256

// texQuadUniformChunkSlots bounds the initial per-frame allocation. A new
// chunk is added only for unusually large frames; older chunks stay alive until
// the frame boundary because already-recorded passes may still reference them.
const texQuadUniformChunkSlots = 64

type texQuadUniformChunk struct {
	buffer   *wgpu.Buffer
	bindGrp  *wgpu.BindGroup
	stride   uint32
	nextSlot uint32
}

type texQuadUniformAllocation struct {
	buffer  *wgpu.Buffer
	bindGrp *wgpu.BindGroup
	offset  uint32
}

type texQuadUniformWrite struct {
	buffer *wgpu.Buffer
	offset uint32
	data   [texQuadUniformSize]byte
}

// SurfaceState tracks the lifecycle state of a GPU surface.
// Transitions follow the WebGPU spec + wgpu framework.rs recovery pattern:
//
//	SurfaceNone → SurfaceReady (surface assigned) → SurfaceConfigured (dimensions set)
//	SurfaceConfigured → SurfaceLost (device lost / fatal) → SurfaceNone (recreate)
//	SurfaceConfigured → SurfaceReady (outdated / resize / minimize unconfigures)
type SurfaceState int

const (
	SurfaceNone       SurfaceState = iota // No surface (headless, Android suspended, pre-init)
	SurfaceReady                          // Surface assigned but not yet configured
	SurfaceConfigured                     // Configured with valid dimensions — can render
	SurfaceLost                           // Device lost or fatal error — must recreate
)

// RenderTarget holds per-window GPU rendering state.
// Each window gets its own surface, format, dimensions, and frame state.
// In multi-window mode, one Renderer holds multiple RenderTarget instances.
type RenderTarget struct {
	renderer *Renderer // back-reference to shared GPU state

	platWindow platform.PlatformWindow // platform window for PrepareFrame and handle access

	surface *wgpu.Surface
	format  gputypes.TextureFormat
	width   uint32
	height  uint32

	state SurfaceState

	// Current frame state (reused, zero alloc per frame)
	currentSurfaceTexture *wgpu.SurfaceTexture
	currentView           *wgpu.TextureView
	frameCleared          bool // Whether the frame has been cleared (for LoadOp selection)
	externalContent       bool // External renderer (g3d) has content on surface (MarkPreserveContent)
	// frameEncoder is borrowed by external renderers and owned by this surface.
	// It is normally finished and submitted at frame end; standalone rendering
	// paths may flush it earlier to preserve call order.
	frameEncoder *wgpu.CommandEncoder
	// texturedQuadPass stays open while consecutive textured quads target the
	// same frame. Boundary operations (external passes, clears, blits, and frame
	// submission) close it before recording their own pass so call order remains
	// observable and arbitrary external interleaving remains valid.
	texturedQuadPass *wgpu.RenderPassEncoder
	// Uniform writes are deferred until texturedQuadPass closes. Queue writes
	// recorded while a pass is still open could survive a failed pass and target
	// an unsubmitted frame buffer; retaining one immutable payload per draw keeps
	// the A3 failure/lifetime guarantee while allowing one pass per contiguous run.
	pendingTexQuadUniformWrites   []texQuadUniformWrite
	texturedQuadPassConsumedClear bool

	// Textured-quad uniform chunks are owned by this surface frame. Each draw
	// gets a distinct dynamic-uniform slot so recording N passes into the shared
	// encoder preserves every quad's parameters on direct-write backends too.
	texQuadUniformChunks []texQuadUniformChunk

	// Deferred clear -- eliminates separate Clear render pass.
	// ClearColor stores the color and sets hasPendingClear=true.
	// The next drawTexturedQuad uses LoadOpClear with this color
	// instead of a separate render pass (avoids double RT->PRESENT->RT
	// state transition that can lose content on DX12 FLIP_DISCARD).
	pendingClearColor gputypes.Color
	hasPendingClear   bool

	// VSync preference for this window
	vsync bool

	// transparent requests CompositeAlphaModePremultiplied for this surface
	// when the adapter advertises support (ADR-060, gogpu#361).
	transparent bool

	// presentationSyncRequested asks the platform to gate the frame following
	// the next successful present until the compositor acknowledges it. It is
	// retained across empty or failed frames.
	presentationSyncRequested bool

	// damageSources holds all registered damage reporters for this surface.
	// Each independent renderer (gg, g3d, video, compose) registers one source
	// via Context.RegisterDamageSource. At present time, all sources are unioned
	// into a single damage region (ADR-065). Sources are reset after present —
	// each must report damage every frame that content changes.
	damageSources []*compositor.DamageSource

	// debugOverlays holds registered debug visualization layers (ADR-066).
	// Overlays draw in registration order after all content renderers,
	// before present. GTK4 GtkInspectorOverlay pattern.
	debugOverlays []gpucontext.DebugOverlay

	// frameNumber is a monotonic counter incremented each frame, exposed to
	// debug overlays via DebugOverlayContext.FrameNumber for statistics,
	// logging, and frame-based calculations (e.g., rolling FPS average).
	frameNumber uint64

	// overlayNeedsRedraw is set when any debug overlay's Draw returns true,
	// signaling it needs another frame to complete visualization (e.g., fade
	// animation, FPS counter). The caller (App frame loop) checks this and
	// calls RequestRedraw for a self-sustaining render loop that automatically
	// stops when all overlays return false (Chromium pattern).
	overlayNeedsRedraw bool

	// hasGPUWork tracks whether any draw calls were issued this frame.
	// When false after OnDraw, no swapchain acquire/present happened
	// (lazy acquire pattern). Reset at frame boundary.
	hasGPUWork bool

	// presentLogCount tracks how many times HiDPI diagnostic logs have been
	// emitted for this surface. Limited to avoid log spam even at Debug level.
	presentLogCount int

	// pixelPresented is set when PresentPixels bypassed the normal
	// Acquire→Render→Present cycle (ADR-052). endFrameForSurface skips
	// present and cleans up stale state instead.
	pixelPresented bool

	// frameStarted tracks whether beginFrame was called this frame cycle.
	// With lazy acquire, beginFrame is deferred until the first draw call.
	// If OnDraw produces no GPU work, beginFrame is never called → no
	// swapchain acquire/present → zero GPU overhead.
	frameStarted bool

	// acquireFailed records that a draw call asked for the swapchain this
	// frame and could not have it. It is what separates "the callback drew
	// nothing", which is what lazy acquire is FOR, from "the callback drew and
	// the surface was unavailable", which is worth another frame. Without the
	// distinction the demand-driven loop reschedules itself forever against a
	// UI that correctly draws nothing when nothing changed.
	acquireFailed bool

	// Compositor-owned composition texture (ADR-067).
	// When debug overlays are active, content renderers draw into this
	// intermediate texture instead of the swapchain image directly.
	// gogpu composites the overlays on top, then blits the result to the
	// swapchain. This isolates overlay rendering from content rendering,
	// preventing bind group lifecycle conflicts.
	// composTex is the per-frame composition surface. Content + overlay are
	// drawn here, then blitted to swapchain. Rebuilt each frame from contentTex.
	composTex  *wgpu.Texture
	composView *wgpu.TextureView
	composW    uint32
	composH    uint32

	// Note: contentTex is NOT needed. composView IS the content cache.
	// Overlays draw on swapchain (after blit), not on composView.
	// composView stays clean (content only) automatically.

	// pendingBlitBindGroup holds the blit bind group created in blitComposToSwapchain.
	// Released at next frame boundary (prepareLazyAcquire) after GPU completion.
	pendingBlitBindGroup *wgpu.BindGroup

	// compositeState holds per-surface persistent bind group state for the MSAA
	// overlay alpha-composite path (compositor.CompositeState).
	compositeState compositor.CompositeState
}

// lockDisplay acquires the platform display lock if the window supports it.
// On Wayland, this serializes wl_display access between the main thread
// and the render thread. On other platforms this is a no-op.
// ADR-041 Phase 2: Wayland wl_display thread safety.
func lockDisplay(pw platform.PlatformWindow) {
	if locker, ok := pw.(platform.DisplayLocker); ok {
		locker.DisplayLock()
	}
}

// unlockDisplay releases the platform display lock if the window supports it.
func unlockDisplay(pw platform.PlatformWindow) {
	if locker, ok := pw.(platform.DisplayLocker); ok {
		locker.DisplayUnlock()
	}
}

// Renderer manages the GPU rendering pipeline.
// It holds shared GPU state (instance, adapter, device, pipelines) that is
// independent of any specific window. Per-window state lives in RenderTarget
// (owned by Window, not Renderer). This enables multi-window, headless, and
// mobile suspend/resume where surfaces come and go (ADR-026).
type Renderer struct {
	// Shared GPU objects — independent of any window
	instance *wgpu.Instance
	adapter  *wgpu.Adapter
	device   *wgpu.Device

	// Surface format — device-level constant, same for all windows
	surfaceFormat gputypes.TextureFormat

	// Backend metadata
	backendName string

	// Submission tracker for non-blocking resource recycling.
	tracker submissionTracker

	// Built-in pipelines (shared across all windows)
	trianglePipeline       *wgpu.RenderPipeline
	trianglePipelineLayout *wgpu.PipelineLayout
	triangleShader         *wgpu.ShaderModule

	// Textured quad pipeline resources (shared across all windows)
	texQuadPipeline       *wgpu.RenderPipeline
	texQuadShader         *wgpu.ShaderModule
	texQuadUniformLayout  *wgpu.BindGroupLayout
	texQuadTextureLayout  *wgpu.BindGroupLayout
	texQuadPipelineLayout *wgpu.PipelineLayout
	texQuadUniformData    []byte
	texQuadPipelineInited bool

	// Dedicated composition blit pipeline (ADR-067).
	// Owns blit + composite pipelines, shader, layouts, sampler, uniform buffer.
	// Lives in internal/compositor to keep renderer.go focused on frame orchestration.
	blitPipeline compositor.BlitPipeline

	// Texture bind group cache — device-level, shared across all windows.
	texBindGroupCache map[*wgpu.TextureView]*wgpu.BindGroup

	// Deferred destruction queue for GC-enqueued resources.
	deferredDestroys   []func()
	deferredDestroysMu sync.Mutex

	// PowerPreference for adapter selection
	powerPreference gputypes.PowerPreference

	// Primary RenderTarget — backward compatibility for single-window API.
	// TODO(lifecycle-phase3): remove once all callers use per-window surfaces.
	primary *RenderTarget

	// currentSurface is the RenderTarget being drawn in the current frame.
	// Set by the multi-window frame loop before each window's draw callback.
	currentSurface *RenderTarget

	// secondaryFrameGatePending avoids scanning the window list in the common
	// single-window path. It is set only when a secondary platform window
	// actually prepares compositor frame gating.
	secondaryFrameGatePending atomic.Bool
}

// newRenderer creates and initializes a new renderer.
func newRenderer(platWin platform.PlatformWindow, graphicsAPI types.GraphicsAPI, vsync bool, powerPref gputypes.PowerPreference, transparent bool) (*Renderer, error) {
	r := &Renderer{
		powerPreference: powerPref,
	}
	r.primary = &RenderTarget{
		renderer:    r,
		platWindow:  platWin,
		vsync:       vsync,
		transparent: transparent,
	}

	// Phase 1: Create GPU instance with backend mask (may include multiple backends).
	if err := r.initInstance(graphicsAPI); err != nil {
		return nil, err
	}

	// Phase 2: Create the surface before adapter enumeration.
	// GLES (EGL) requires a window handle for context creation, but creating
	// the surface early is harmless for other backends and enables multi-backend
	// Auto mode where GLES participates in adapter selection alongside Vulkan/DX12.
	if err := r.createSurface(r.primary); err != nil {
		return nil, err
	}

	// Phase 3: Request adapter (with surface hint for GLES deferred enumeration) and device.
	if err := r.initAdapterDevice(r.primary.surface); err != nil {
		return nil, err
	}

	// Phase 4: Configure surface dimensions.
	if err := r.configureSurface(r.primary); err != nil {
		return nil, err
	}

	return r, nil
}

// initInstance creates the wgpu Instance for the requested graphics API.
// For Auto mode, the mask includes multiple backends (e.g., DX12|Vulkan|GLES on Windows)
// so wgpu can enumerate all available adapters and pick the best GPU.
func (r *Renderer) initInstance(graphicsAPI types.GraphicsAPI) error {
	var backendMask gputypes.Backends
	r.backendName, backendMask = native.BackendInfo(graphicsAPI)

	var instanceFlags gputypes.InstanceFlags
	if os.Getenv("GOGPU_DEBUG") == "1" {
		instanceFlags = gputypes.InstanceFlagsDebug | gputypes.InstanceFlagsValidation
	}
	var err error
	r.instance, err = wgpu.CreateInstance(&wgpu.InstanceDescriptor{
		Backends: backendMask,
		Flags:    instanceFlags,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create instance: %w", err)
	}
	r.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
	return nil
}

// initAdapterDevice requests the adapter and creates the logical device.
// surfaceHint is non-nil only for GLES: it feeds EnumerateAdapters so the
// backend can return a real adapter backed by a live EGL context.
func (r *Renderer) initAdapterDevice(surfaceHint *wgpu.Surface) error {
	opts := &wgpu.RequestAdapterOptions{
		PowerPreference:   r.powerPreference,
		CompatibleSurface: surfaceHint,
	}
	var err error
	r.adapter, err = r.instance.RequestAdapter(opts)
	if err != nil {
		return fmt.Errorf("gogpu: failed to request adapter: %w", err)
	}
	info := r.adapter.Info()
	slog.Info("adapter selected", "name", info.Name, "backend", info.Backend, "type", info.DeviceType)
	r.backendName = "Pure Go (" + info.Backend.String() + ")"

	r.device, err = r.adapter.RequestDevice(nil)
	if err != nil {
		return fmt.Errorf("gogpu: failed to request device: %w", err)
	}
	return nil
}

// createSurface creates the wgpu Surface from the window handles.
// Does not configure dimensions — call configureSurface after device is ready.
func (r *Renderer) createSurface(ws *RenderTarget) error {
	displayHandle, windowHandle := ws.platWindow.GetHandle()
	surface, err := r.instance.CreateSurface(displayHandle, windowHandle)
	if err != nil {
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}
	ws.surface = surface
	ws.state = SurfaceReady
	ws.format = r.surfaceFormat
	return nil
}

// configureSurface configures the surface with the current window dimensions.
// Requires device and adapter to be initialized. Zero dimensions are skipped —
// common on Wayland before the compositor sends xdg_surface.configure
// (ADR-041 defense-in-depth); the first resize event completes configuration.
func (r *Renderer) configureSurface(ws *RenderTarget) error {
	width, height := ws.platWindow.PhysicalSize()
	if width > 0 && height > 0 {
		ws.width = uint32(width)   //nolint:gosec // G115: validated positive above
		ws.height = uint32(height) //nolint:gosec // G115: validated positive above
		if err := ws.configure(r.device, r.adapter); err != nil {
			return fmt.Errorf("gogpu: failed to configure surface: %w", err)
		}
		ws.state = SurfaceConfigured
	} else {
		slog.Debug("gogpu: configureSurface skipping (zero dimensions)",
			"width", width, "height", height)
	}
	return nil
}

// configure configures the wgpu surface with current dimensions and format.
func (ws *RenderTarget) configure(device *wgpu.Device, adapter *wgpu.Adapter) error {
	caps := adapter.GetSurfaceCapabilities(ws.surface)
	presentMode := resolvePresentMode(caps, ws.vsync)
	alphaMode := resolveAlphaMode(caps, ws.transparent)

	return ws.surface.Configure(device, &wgpu.SurfaceConfiguration{
		Format:      ws.format,
		Usage:       gputypes.TextureUsageRenderAttachment,
		Width:       ws.width,
		Height:      ws.height,
		AlphaMode:   alphaMode,
		PresentMode: presentMode,
	})
}

// resolvePresentMode selects the best available present mode following the
// Rust wgpu fallback pattern. For VSync on (AutoVsync): FifoRelaxed -> Fifo.
// For VSync off (AutoNoVsync): Immediate -> Mailbox -> Fifo.
// Falls back to Fifo which is guaranteed by the Vulkan spec.
func resolvePresentMode(caps *wgpu.SurfaceCapabilities, vsync bool) gputypes.PresentMode {
	if caps == nil {
		// No capabilities available — use safe default.
		mode := gputypes.PresentModeFifo
		if !vsync {
			mode = gputypes.PresentModeImmediate
		}
		slog.Debug("gogpu: no surface capabilities, using default present mode",
			"mode", mode, "vsync", vsync)
		return mode
	}

	supported := caps.PresentModes
	var mode gputypes.PresentMode

	if vsync {
		// VSync on: FifoRelaxed -> Fifo (like Rust AutoVsync).
		mode = pickPresentMode(supported,
			gputypes.PresentModeFifoRelaxed,
			gputypes.PresentModeFifo,
		)
	} else {
		// VSync off: Immediate -> Mailbox -> Fifo (like Rust AutoNoVsync).
		mode = pickPresentMode(supported,
			gputypes.PresentModeImmediate,
			gputypes.PresentModeMailbox,
			gputypes.PresentModeFifo,
		)
	}

	slog.Debug("gogpu: resolved present mode",
		"mode", mode, "vsync", vsync, "supported", supported)
	return mode
}

// resolveAlphaMode selects the surface alpha compositing mode.
// Transparent windows request CompositeAlphaModePremultiplied; if the
// adapter does not advertise it (or capabilities are unavailable) the
// surface falls back to Opaque, preserving pre-ADR-060 behavior.
func resolveAlphaMode(caps *wgpu.SurfaceCapabilities, transparent bool) gputypes.CompositeAlphaMode {
	if !transparent {
		return gputypes.CompositeAlphaModeOpaque
	}
	if caps == nil {
		slog.Debug("gogpu: no surface capabilities, using opaque alpha mode")
		return gputypes.CompositeAlphaModeOpaque
	}
	for _, mode := range caps.AlphaModes {
		if mode == gputypes.CompositeAlphaModePremultiplied {
			return mode
		}
	}
	slog.Debug("gogpu: premultiplied alpha unsupported, using opaque alpha mode",
		"supported", caps.AlphaModes)
	return gputypes.CompositeAlphaModeOpaque
}

// pickPresentMode returns the first mode from preferred that is in supported.
// Falls back to Fifo if none match (guaranteed by Vulkan spec).
func pickPresentMode(supported []gputypes.PresentMode, preferred ...gputypes.PresentMode) gputypes.PresentMode {
	for _, pref := range preferred {
		for _, sup := range supported {
			if pref == sup {
				return pref
			}
		}
	}

	slog.Warn("gogpu: no preferred present mode available, falling back to Fifo",
		"supported", supported, "preferred", preferred)
	return gputypes.PresentModeFifo
}

// activeSurface returns the currently active RenderTarget for draw operations.
// During multi-window rendering, this returns the surface of the window being drawn.
// Otherwise, it returns the primary window surface.
func (r *Renderer) activeSurface() *RenderTarget {
	if r.currentSurface != nil {
		return r.currentSurface
	}
	return r.primary
}

// Resize handles window resize.
// This also handles deferred surface configuration when the window
// first becomes visible with valid dimensions (especially important on macOS).
func (r *Renderer) Resize(width, height int) {
	r.ResizeSurface(r.primary, width, height)
}

// ResizeSurface handles window resize for any surface.
func (r *Renderer) ResizeSurface(ws *RenderTarget, width, height int) {
	ws.resize(width, height, r.device, r.adapter)
}

// resize handles window resize for this surface.
func (ws *RenderTarget) resize(width, height int, device *wgpu.Device, adapter *wgpu.Adapter) {
	if width <= 0 || height <= 0 {
		// Window minimized or invisible -- unconfigure surface to prevent
		// zero-extent swapchain creation on the next frame (VK-VAL-001).
		if ws.state == SurfaceConfigured {
			ws.surface.Unconfigure()
			ws.state = SurfaceReady
		}
		return
	}

	// Skip no-op resize. ConfigureSurface is expensive (involves device wait idle
	// and surface reconfiguration), so avoid calling it when dimensions are unchanged.
	if uint32(width) == ws.width && uint32(height) == ws.height { //nolint:gosec // G115: validated positive above
		return
	}

	// Save old dimensions in case Configure fails -- we must keep
	// width/height consistent with the actual swapchain size.
	oldWidth, oldHeight := ws.width, ws.height

	slog.Debug("gogpu: surface resize",
		"oldWidth", oldWidth, "oldHeight", oldHeight,
		"newWidth", width, "newHeight", height,
	)

	// Note: width/height validated positive above
	ws.width = uint32(width)   //nolint:gosec // G115: validated positive above
	ws.height = uint32(height) //nolint:gosec // G115: validated positive above

	// Release stale composition texture — it will be recreated at the new
	// size during the next frame if overlays are active.
	ws.releaseCompositionTexture()

	// Configure surface with new dimensions.
	if err := ws.configure(device, adapter); err != nil {
		// Restore old dimensions to keep surface consistent with swapchain.
		// Next frame will retry with the new size.
		ws.width = oldWidth
		ws.height = oldHeight
		return
	}
	ws.state = SurfaceConfigured
}

// BeginFrame prepares a new frame for rendering on the primary surface.
// Backward compatibility wrapper — multi-window code uses beginFrameForSurface.
func (r *Renderer) BeginFrame() bool {
	r.DrainDeferredDestroys()
	return r.beginFrameForSurface(r.primary)
}

// beginFrameForSurface acquires the next texture for any surface.
func (r *Renderer) beginFrameForSurface(ws *RenderTarget) bool {
	return ws.beginFrame(ws.platWindow, r.device, r.adapter)
}

// CanRender reports whether this surface is ready for draw operations.
func (ws *RenderTarget) CanRender() bool {
	return ws.state == SurfaceConfigured && ws.surface != nil
}

// beginFrame acquires the next surface texture for rendering on this window.
// Returns false if frame cannot be acquired (surface not configured, minimized, etc.).
// Recovery follows the wgpu framework.rs pattern:
//   - ErrSurfaceOutdated → reconfigure (swapchain stale after resize/DPI change)
//   - ErrSurfaceLost → mark SurfaceLost (caller must recreate)
func (ws *RenderTarget) beginFrame(platWin platform.PlatformWindow, device *wgpu.Device, adapter *wgpu.Adapter) bool {
	// The app frame loop normally releases these in prepareLazyAcquire, but the
	// exported Renderer.BeginFrame/EndFrame pair reaches beginFrame directly.
	// Start every acquired frame with a fresh uniform arena so manual callers do
	// not retain one chunk per frame indefinitely.
	ws.releaseTexQuadUniformChunks()

	if !ws.CanRender() {
		return false
	}

	// Before acquiring surface texture, let platform update surface state
	// (e.g., CAMetalLayer.contentsScale on macOS for HiDPI/multi-monitor).
	// Reconfigure when the physical surface dimensions changed — including
	// during live resize (Rio pattern): the surface must match the window
	// every tick so the presented frame is pixel-exact, and the Metal layer's
	// blocking nextDrawable + transaction present keep pool churn bounded.
	if platWin != nil {
		result := platWin.PrepareFrame()
		if result.PhysicalWidth > 0 && result.PhysicalHeight > 0 &&
			(result.PhysicalWidth != ws.width || result.PhysicalHeight != ws.height) {
			ws.width = result.PhysicalWidth
			ws.height = result.PhysicalHeight
			_ = ws.configure(device, adapter)
		}
	}

	// Acquire the next surface texture via wgpu public API.
	// On Wayland, vkAcquireNextImageKHR may touch wl_display internally.
	// Lock display to prevent races with main thread's DispatchDefaultQueue
	// (ADR-041 Phase 2).
	lockDisplay(platWin)
	surfaceTexture, _, err := ws.surface.GetCurrentTexture()
	unlockDisplay(platWin)
	if err != nil {
		ws.recoverFromAcquireError(err, device, adapter)
		return false
	}

	ws.currentSurfaceTexture = surfaceTexture

	// Create texture view for rendering
	view, err := surfaceTexture.CreateView(nil)
	if err != nil {
		ws.surface.DiscardTexture()
		ws.currentSurfaceTexture = nil
		return false
	}
	ws.currentView = view

	// Reset frame state for new frame
	ws.frameCleared = false
	ws.externalContent = false
	ws.hasPendingClear = false
	ws.hasGPUWork = false

	return true
}

// recoverFromAcquireError handles surface texture acquisition failures.
// Outdated surfaces are reconfigured (common after resize/DPI change).
// Lost surfaces transition to SurfaceLost for caller-level recreation.
func (ws *RenderTarget) recoverFromAcquireError(err error, device *wgpu.Device, adapter *wgpu.Adapter) {
	switch {
	case errors.Is(err, wgpu.ErrSurfaceOutdated):
		slog.Debug("gogpu: surface outdated, reconfiguring", "width", ws.width, "height", ws.height)
		if ws.width > 0 && ws.height > 0 {
			if cfgErr := ws.configure(device, adapter); cfgErr != nil {
				slog.Error("gogpu: reconfigure after outdated failed", "err", cfgErr)
				ws.state = SurfaceLost
			}
		}
	case errors.Is(err, wgpu.ErrSurfaceLost):
		slog.Error("gogpu: surface lost", "err", err)
		ws.state = SurfaceLost
	default:
		slog.Error("gogpu: surface texture acquire failed", "err", err)
		if ws.width > 0 && ws.height > 0 {
			_ = ws.configure(device, adapter)
		}
	}
}

// EndFrame presents the rendered frame on the primary surface.
// Backward compatibility wrapper — multi-window code uses endFrameForSurface.
func (r *Renderer) EndFrame() {
	if !r.primary.frameStarted {
		r.pollSubmissions()
		return
	}
	// Outdated-reconfigure recovery is left to the next BeginFrame/EndFrame —
	// this manual path has no retained draw callback to replay.
	r.endFrameForSurface(r.primary)
	r.pollSubmissions()
}

// endFrameForSurface flushes, presents, and releases frame resources for a
// specific RenderTarget. Used by the multi-window frame loop. Unlike EndFrame,
// it does NOT poll submissions -- the caller polls once after all windows.
// Returns true if present() reconfigured an outdated surface (see present).
func (r *Renderer) endFrameForSurface(ws *RenderTarget) bool {
	// PresentPixels already presented — skip normal present path (ADR-052).
	if ws.pixelPresented {
		ws.discardFrameEncoder()
		ws.pixelPresented = false
		ws.hasPendingClear = false
		ws.releaseFrame()
		return false
	}

	// ADR-067: when composition texture is active, content renderers already
	// handled the clear via their own render passes (LoadOpClear). Don't
	// flush a pending clear here — it would wipe the rendered content.
	// Only flush when rendering directly to swapchain (no composition texture).
	if ws.composView == nil {
		ws.flushClear(r.device, r)
	} else {
		ws.hasPendingClear = false
	}

	// ADR-067: blit cached content to swapchain FIRST, then overlays on top.
	// composView = clean content cache (no overlay). Overlay draws on swapchain
	// after blit → never accumulates. composView stays clean for next frame.
	if ws.composView != nil {
		r.blitComposToSwapchain(ws)
	}

	ws.drawDebugOverlays()

	ws.submitFrameEncoder(r)
	ws.frameNumber++
	// Request frame callback BEFORE present (winit pre_present_notify pattern).
	// Wayland spec: "The frame request will take effect on the next commit."
	// The present's internal wl_surface.commit activates it atomically.
	syncAttempt := syncFrameForPresent(ws)
	reconfigured, presented := ws.present()
	finishPresentationSync(ws, syncAttempt, presented)
	ws.releaseFrame()
	return reconfigured
}

type presentationSyncAttempt struct {
	consumedRequest bool
	cancelable      bool
}

// syncFrameForPresent applies the platform's normal frame policy, unless the
// surface has a pending one-shot request. Platforms with pre-present semantics
// report whether they actually prepared a cancelable synchronization request.
func syncFrameForPresent(ws *RenderTarget) presentationSyncAttempt {
	if ws == nil || ws.platWindow == nil {
		return presentationSyncAttempt{}
	}
	force := ws.presentationSyncRequested
	if syncer, ok := ws.platWindow.(platform.PresentationSyncer); ok {
		prepared := syncer.PrepareFrameSync(force)
		if prepared {
			markSecondaryFrameGatePending(ws)
		}
		if force && !prepared {
			// A failed preparation must not consume the request. This also covers
			// an unexpected already-pending callback; the frame gate normally
			// prevents rendering in that state.
			return presentationSyncAttempt{}
		}
		ws.presentationSyncRequested = false
		return presentationSyncAttempt{consumedRequest: force, cancelable: prepared}
	}

	ws.platWindow.SyncFrame()
	if force {
		ws.presentationSyncRequested = false
	}
	return presentationSyncAttempt{consumedRequest: force}
}

// syncFrameBeforePixelPresent prepares frame synchronization only on platforms
// that require it before the surface commit (Wayland). Other platforms retain
// their existing post-present SyncFrame timing.
func syncFrameBeforePixelPresent(ws *RenderTarget) (presentationSyncAttempt, bool) {
	if ws == nil || ws.platWindow == nil {
		return presentationSyncAttempt{}, false
	}
	if _, ok := ws.platWindow.(platform.PresentationSyncer); !ok {
		return presentationSyncAttempt{}, false
	}
	return syncFrameForPresent(ws), true
}

func syncFrameAfterPixelPresent(ws *RenderTarget) {
	if ws == nil || ws.platWindow == nil {
		return
	}
	ws.platWindow.SyncFrame()
	ws.presentationSyncRequested = false
}

func finishPresentationSync(ws *RenderTarget, attempt presentationSyncAttempt, presented bool) {
	if ws == nil || presented {
		return
	}
	if attempt.cancelable {
		if syncer, ok := ws.platWindow.(platform.PresentationSyncer); ok {
			syncer.CancelFrameSync()
		}
	}
	if attempt.consumedRequest {
		ws.presentationSyncRequested = true
	}
}

func markSecondaryFrameGatePending(ws *RenderTarget) {
	if ws == nil || ws.renderer == nil || ws == ws.renderer.primary {
		return
	}
	ws.renderer.secondaryFrameGatePending.Store(true)
}

// pollSubmissions performs non-blocking submission tracking: frees GPU resources
// for completed submissions. Called once per frame after all windows are presented.
func (r *Renderer) pollSubmissions() {
	if r.device == nil {
		return
	}
	completedIdx := r.device.Queue().Poll()
	r.tracker.triage(completedIdx, r.device)
}

// present presents the surface texture to the screen, passing the union of
// all registered damage sources to the platform compositor. Damage sources
// are reset after presentation so each must report again next frame.
//
// On Wayland, Vulkan WSI internally calls wl_surface_attach / wl_surface_commit /
// wl_display_flush during vkQueuePresentKHR. The display lock serializes this with
// the main thread's DispatchDefaultQueue (ADR-041 Phase 2).
//
// Returns whether the surface was outdated and reconfigured and whether the
// submitted frame was successfully presented.
func (ws *RenderTarget) present() (reconfigured, presented bool) {
	if ws.currentSurfaceTexture == nil {
		return false, false
	}
	finalDamage := compositor.UnionAllSources(ws.damageSources)
	lockDisplay(ws.platWindow)
	err := ws.surface.PresentWithDamage(ws.currentSurfaceTexture, finalDamage)
	unlockDisplay(ws.platWindow)
	for _, ds := range ws.damageSources {
		ds.Reset()
	}
	if err == nil {
		return false, true
	}
	// Mirror recoverFromAcquireError: outdated is expected (resize/DPI/monitor),
	// not an error — reconfigure and signal the caller to re-render.
	if errors.Is(err, wgpu.ErrSurfaceOutdated) {
		slog.Debug("gogpu: surface outdated on present, reconfiguring", "width", ws.width, "height", ws.height)
		if ws.width > 0 && ws.height > 0 {
			if cfgErr := ws.configure(ws.renderer.device, ws.renderer.adapter); cfgErr != nil {
				slog.Error("gogpu: reconfigure after outdated failed", "err", cfgErr)
				ws.state = SurfaceLost
				return false, false
			}
			return true, false
		}
		return false, false
	}
	if errors.Is(err, wgpu.ErrSurfaceLost) {
		slog.Error("gogpu: surface lost on present", "err", err)
		ws.state = SurfaceLost
		return false, false
	}
	slog.Error("PRESENT ERROR", "err", err)
	return false, false
}

// setTransactionPresent toggles Core Animation transaction-based present on
// Metal surfaces (no-op elsewhere). Enabled during macOS live resize so the
// drawable swap commits atomically with the window-resize CA transaction —
// eliminating both frame stretching and IOSurface drawable stockpiling.
// Must be called on the render thread.
func (ws *RenderTarget) setTransactionPresent(enabled bool) {
	if ws.surface == nil {
		return
	}
	if tp, ok := any(ws.surface).(interface{ SetPresentsWithTransaction(bool) }); ok {
		tp.SetPresentsWithTransaction(enabled)
	}
}

// prepareLazyAcquire resets per-frame state for deferred beginFrame.
// The actual swapchain acquire happens on first draw call via ensureFrameStarted.
// Uses ws.platWindow and ws.renderer.{device,adapter} — no parameters needed.
func (ws *RenderTarget) prepareLazyAcquire() {
	ws.discardFrameEncoder()
	ws.releaseTexQuadUniformChunks()
	ws.frameStarted = false
	ws.hasGPUWork = false
	ws.pixelPresented = false
	ws.acquireFailed = false
	// Release blit bind group from previous frame (GPU completed by VSync).
	if ws.pendingBlitBindGroup != nil {
		ws.pendingBlitBindGroup.Release()
		ws.pendingBlitBindGroup = nil
	}
	ws.compositeState.Release()
}

// ensureFrameStarted calls beginFrame on first draw call (lazy acquire pattern).
// Returns true if frame is ready for rendering.
func (ws *RenderTarget) ensureFrameStarted() bool {
	if ws.frameStarted {
		return ws.currentView != nil
	}
	if ws.beginFrame(ws.platWindow, ws.renderer.device, ws.renderer.adapter) {
		ws.frameStarted = true
		return true
	}
	// Surface may not be configured yet (0×0 at init). Retry once after resize.
	if !ws.CanRender() && ws.platWindow != nil {
		pw, ph := ws.platWindow.PhysicalSize()
		if pw > 0 && ph > 0 {
			ws.resize(pw, ph, ws.renderer.device, ws.renderer.adapter)
			if ws.beginFrame(ws.platWindow, ws.renderer.device, ws.renderer.adapter) {
				ws.frameStarted = true
				return true
			}
		}
	}
	ws.acquireFailed = true
	return false
}

// resetLazyState clears per-frame state after frame cycle.
// Note: overlayNeedsRedraw is NOT cleared here — it survives into the app
// frame loop so the self-sustaining render loop (ADR-066 Chromium pattern)
// can call RequestRedraw. It is cleared at the start of drawDebugOverlays
// each frame.
func (ws *RenderTarget) resetLazyState() {
	ws.frameStarted = false
	ws.hasGPUWork = false
	ws.pixelPresented = false
	ws.acquireFailed = false
}

// tryOverlayOnlyFrame attempts to start an overlay-only frame when the
// composition texture holds content from a previous frame and an overlay
// requests another draw (ADR-067). Returns true if a frame was successfully
// started — the caller should fall through to endFrame. Returns false if
// no overlay-only frame is possible.
func (ws *RenderTarget) tryOverlayOnlyFrame() bool {
	if !ws.overlayNeedsRedraw || ws.composView == nil {
		return false
	}
	return ws.ensureFrameStarted()
}

// ensureFrameEncoder returns the framework-owned encoder for this surface
// frame, creating it lazily. Ownership remains with RenderTarget.
func (ws *RenderTarget) ensureFrameEncoder() (*wgpu.CommandEncoder, error) {
	if !ws.ensureFrameStarted() {
		return nil, fmt.Errorf("gogpu: surface frame not available")
	}
	if ws.frameEncoder != nil {
		return ws.frameEncoder, nil
	}
	encoder, err := ws.renderer.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "gogpu_shared_frame",
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: create shared frame encoder: %w", err)
	}
	ws.frameEncoder = encoder
	return encoder, nil
}

// allocateTexQuadUniform reserves one aligned uniform slot for a textured
// quad recorded in the current frame. Chunks are frame-owned rather than
// renderer-owned so their data cannot be overwritten while another window's
// frame is still in flight.
func (ws *RenderTarget) allocateTexQuadUniform(r *Renderer) (texQuadUniformAllocation, error) {
	stride := uint32(texQuadUniformStride)
	if limit := r.device.Limits().MinUniformBufferOffsetAlignment; limit > stride {
		stride = limit
	}
	if len(ws.texQuadUniformChunks) == 0 ||
		ws.texQuadUniformChunks[len(ws.texQuadUniformChunks)-1].nextSlot >= texQuadUniformChunkSlots {
		buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
			Label: "Textured Quad Frame Uniforms",
			Size:  uint64(stride) * uint64(texQuadUniformChunkSlots),
			Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		})
		if err != nil {
			return texQuadUniformAllocation{}, fmt.Errorf("gogpu: create textured quad frame uniforms: %w", err)
		}
		bindGrp, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Textured Quad Frame Uniform Bind Group",
			Layout: r.texQuadUniformLayout,
			Entries: []wgpu.BindGroupEntry{
				{
					Binding: 0,
					Buffer:  buffer,
					Size:    texQuadUniformSize,
				},
			},
		})
		if err != nil {
			buffer.Release()
			return texQuadUniformAllocation{}, fmt.Errorf("gogpu: create textured quad frame uniform bind group: %w", err)
		}
		ws.texQuadUniformChunks = append(ws.texQuadUniformChunks, texQuadUniformChunk{
			buffer:  buffer,
			bindGrp: bindGrp,
			stride:  stride,
		})
	}

	chunk := &ws.texQuadUniformChunks[len(ws.texQuadUniformChunks)-1]
	offset := chunk.nextSlot * chunk.stride
	chunk.nextSlot++
	return texQuadUniformAllocation{
		buffer:  chunk.buffer,
		bindGrp: chunk.bindGrp,
		offset:  offset,
	}, nil
}

// releaseTexQuadUniformChunks releases frame uniform resources after the
// previous frame's submission has completed (the same boundary used for
// composition bind groups). It is also called by RenderTarget.destroy.
func (ws *RenderTarget) releaseTexQuadUniformChunks() {
	for _, chunk := range ws.texQuadUniformChunks {
		if chunk.bindGrp != nil {
			chunk.bindGrp.Release()
		}
		if chunk.buffer != nil {
			chunk.buffer.Release()
		}
	}
	ws.texQuadUniformChunks = nil
	ws.pendingTexQuadUniformWrites = ws.pendingTexQuadUniformWrites[:0]
	ws.texturedQuadPassConsumedClear = false
}

// submitFrameEncoder finishes and submits the framework-owned shared encoder.
func (ws *RenderTarget) submitFrameEncoder(r *Renderer) {
	if err := ws.endTexturedQuadPass(); err != nil {
		slog.Error("finish textured quad render pass failed", "err", err)
		return
	}
	encoder := ws.frameEncoder
	ws.frameEncoder = nil
	if encoder == nil {
		return
	}
	commands, err := encoder.Finish()
	if err != nil {
		slog.Error("finish shared frame encoder failed", "err", err)
		return
	}
	r.submitTracked(commands)
}

// endTexturedQuadPass closes the pass used by the current contiguous textured
// quad run and publishes its deferred uniform writes. It is the only normal
// path that ends this pass; callers use it before recording any external or
// framework-owned pass so pass order remains explicit.
func (ws *RenderTarget) endTexturedQuadPass() error {
	pass := ws.texturedQuadPass
	if pass == nil {
		return nil
	}
	ws.texturedQuadPass = nil
	if err := pass.End(); err != nil {
		ws.abortTexturedQuadPassState()
		ws.discardFrameEncoder()
		return fmt.Errorf("gogpu: end textured quad render pass: %w", err)
	}

	writes := ws.pendingTexQuadUniformWrites
	for _, write := range writes {
		if err := ws.renderer.device.Queue().WriteBuffer(write.buffer, uint64(write.offset), write.data[:]); err != nil {
			ws.pendingTexQuadUniformWrites = ws.pendingTexQuadUniformWrites[:0]
			ws.abortTexturedQuadPassState()
			ws.discardFrameEncoder()
			return fmt.Errorf("gogpu: WriteBuffer uniform failed: %w", err)
		}
	}
	ws.pendingTexQuadUniformWrites = ws.pendingTexQuadUniformWrites[:0]
	ws.texturedQuadPassConsumedClear = false
	return nil
}

// beginTexturedQuadPass starts a pass for a contiguous textured-quad run.
// The caller has already closed any prior run when an explicit clear is
// pending, so this helper only handles the first pass state for the run.
func (ws *RenderTarget) beginTexturedQuadPass(encoder *wgpu.CommandEncoder) error {
	loadOp := gputypes.LoadOpClear
	clearValue := gputypes.Color{R: 0, G: 0, B: 0, A: 0}
	consumePendingClear := false
	switch {
	case ws.hasPendingClear:
		clearValue = ws.pendingClearColor
		consumePendingClear = true
	case ws.frameCleared:
		loadOp = gputypes.LoadOpLoad
	}

	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       ws.renderView(),
			LoadOp:     loadOp,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: clearValue,
		}},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to begin render pass: %w", err)
	}
	ws.texturedQuadPass = pass
	ws.texturedQuadPassConsumedClear = consumePendingClear
	if consumePendingClear {
		ws.hasPendingClear = false
	}
	return nil
}

// abortTexturedQuadPassState drops the pass-local state after a failed pass or
// an abandoned frame. The shared encoder itself is discarded by the caller.
func (ws *RenderTarget) abortTexturedQuadPassState() {
	if ws.texturedQuadPass != nil {
		_ = ws.texturedQuadPass.End()
		ws.texturedQuadPass = nil
	}
	if ws.texturedQuadPassConsumedClear {
		ws.hasPendingClear = true
	}
	ws.texturedQuadPassConsumedClear = false
	ws.pendingTexQuadUniformWrites = ws.pendingTexQuadUniformWrites[:0]
}

// drawDebugOverlays iterates registered debug overlays and calls their Draw
// method with the current frame's GPU context. Called after all content
// renderers have finished and before submitFrameEncoder/present.
//
// When overlays are present and a composition texture exists, overlays draw
// onto the composition texture (which already contains the content). The
// caller then blits the composition texture to the swapchain. When no
// composition texture is available (no overlays registered before draw),
// overlays draw directly onto the swapchain as before.
//
// Overlays use the shared frame encoder and surface view to record render
// passes with LoadOp::Load (compositing on top of content). When any overlay
// returns true (needs another frame), overlayNeedsRedraw is set so the caller
// (App frame loop) can call RequestRedraw for a self-sustaining render loop
// that automatically stops when all overlays return false.
//
// GTK4 pattern: GtkInspectorOverlay list iterated by gsk_renderer_render
// after gsk_render_node_draw, before compositor flip.
func (ws *RenderTarget) drawDebugOverlays() {
	// Auto-register debug overlays if their env vars are set.
	// Must run before the empty check so overlays can self-register.
	initDamageOverlayIfNeeded(ws)
	initFPSOverlayIfNeeded(ws)

	if len(ws.debugOverlays) == 0 {
		return
	}
	if ws.currentView == nil {
		return
	}
	if err := ws.endTexturedQuadPass(); err != nil {
		slog.Error("gogpu: end textured quad pass before debug overlays failed", "err", err)
		return
	}

	encoder, err := ws.ensureFrameEncoder()
	if err != nil {
		slog.Error("gogpu: debug overlay encoder unavailable", "err", err)
		return
	}

	// ADR-067: overlays ALWAYS draw on swapchain (currentView), AFTER
	// blitComposToSwapchain copies cached content. composView stays clean
	// (content-only cache). Overlay never accumulates on composView.
	overlayView := ws.currentView
	ctx := gpucontext.DebugOverlayContext{
		SurfaceWidth:  ws.width,
		SurfaceHeight: ws.height,
		Encoder:       gpucontext.NewCommandEncoder(unsafe.Pointer(encoder)),  //nolint:gosec // Go spec Rule 1: *T -> unsafe.Pointer
		SurfaceView:   gpucontext.NewTextureView(unsafe.Pointer(overlayView)), //nolint:gosec // Go spec Rule 1: *T -> unsafe.Pointer
		FrameNumber:   ws.frameNumber,
	}

	ws.overlayNeedsRedraw = false
	for _, overlay := range ws.debugOverlays {
		if overlay.Draw(ctx) {
			ws.overlayNeedsRedraw = true
		}
	}
}

// discardFrameEncoder abandons an unsubmitted shared encoder on cancellation.
func (ws *RenderTarget) discardFrameEncoder() {
	ws.abortTexturedQuadPassState()
	if ws.frameEncoder == nil {
		return
	}
	ws.frameEncoder.DiscardEncoding()
	ws.frameEncoder = nil
}

// releaseFrame releases per-frame resources after presentation.
func (ws *RenderTarget) releaseFrame() {
	ws.discardFrameEncoder()
	if ws.currentView != nil {
		ws.currentView.Release()
		ws.currentView = nil
	}
	// SurfaceTexture is consumed by Present, no need to destroy it
	ws.currentSurfaceTexture = nil
}

// renderView returns the view that content renderers should target.
// When a composition texture is active (ADR-067), content goes to
// the intermediate texture; otherwise directly to the swapchain.
func (ws *RenderTarget) renderView() *wgpu.TextureView {
	// ADR-067: ensure composition texture exists before returning the view.
	// Content renderers call SurfaceView() → renderView() at frame start,
	// BEFORE drawDebugOverlays. Without this, composView is nil and content
	// renders directly to swapchain, then blitComposToSwapchain overwrites it.
	ws.ensureCompositionTexture()
	if ws.composView != nil {
		return ws.composView
	}
	return ws.currentView
}

// ensureCompositionTexture creates the intermediate composition texture
// lazily when debug overlays are present. The texture matches the surface
// dimensions and format. It uses RenderAttachment (content draws into it)
// and TextureBinding (blit samples from it).
func (ws *RenderTarget) ensureCompositionTexture() {
	if ws.renderer == nil || ws.renderer.device == nil {
		return
	}
	if ws.currentView == nil {
		return
	}
	// Only create when debug overlays are registered or env vars indicate
	// they will be. Without overlays, render directly to swapchain (zero overhead).
	if len(ws.debugOverlays) == 0 && !ws.hasRegisteredOverlayEnv() {
		return
	}

	// Already sized correctly.
	if ws.composView != nil && ws.composW == ws.width && ws.composH == ws.height {
		return
	}

	// Release stale texture.
	ws.releaseCompositionTexture()

	tex, err := ws.renderer.device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "CompositionTexture",
		Size:          wgpu.Extent3D{Width: ws.width, Height: ws.height, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        ws.format,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
	})
	if err != nil {
		slog.Error("gogpu: create composition texture failed", "err", err)
		return
	}

	view, err := ws.renderer.device.CreateTextureView(tex, nil)
	if err != nil {
		tex.Release()
		slog.Error("gogpu: create composition texture view failed", "err", err)
		return
	}

	ws.composTex = tex
	ws.composView = view
	ws.composW = ws.width
	ws.composH = ws.height
}

// releaseCompositionTexture frees the intermediate composition texture.
func (ws *RenderTarget) releaseCompositionTexture() {
	if ws.composView != nil {
		ws.composView.Release()
		ws.composView = nil
	}
	if ws.composTex != nil {
		ws.composTex.Release()
		ws.composTex = nil
	}
	ws.composW = 0
	ws.composH = 0
}

// hasRegisteredOverlayEnv checks if any debug overlay env vars are set,
// indicating overlays will be registered during drawDebugOverlays. This
// enables ensureCompositionTexture to be called proactively before overlays
// self-register, so content renders into the composition texture from the
// start of the frame.
func (ws *RenderTarget) hasRegisteredOverlayEnv() bool {
	mode := compositor.GetDamageDebugMode()
	if mode.Overlay || mode.Log {
		return true
	}
	fpsMode := compositor.GetFPSDebugMode()
	return fpsMode.Overlay || fpsMode.Log
}

// initBlitPipeline delegates to the compositor BlitPipeline.Init with the
// shared positionedQuadShaderSource (which stays in root — used by texQuadPipeline too).
func (r *Renderer) initBlitPipeline() error {
	return r.blitPipeline.Init(r.device, r.surfaceFormat, positionedQuadShaderSource)
}

// blitComposToSwapchain draws a full-screen quad sampling the composition
// texture onto the swapchain image. Uses the dedicated blit pipeline (NOT
// the shared texQuadPipeline) to avoid bind group lifetime conflicts with
// gg's render session. See ADR-067.
func (r *Renderer) blitComposToSwapchain(ws *RenderTarget) {
	if ws.composView == nil || ws.currentView == nil {
		return
	}
	if err := ws.endTexturedQuadPass(); err != nil {
		slog.Error("gogpu: end textured quad pass before composition blit failed", "err", err)
		return
	}
	if !r.blitPipeline.Inited {
		if err := r.initBlitPipeline(); err != nil {
			slog.Error("gogpu: blit pipeline init failed", "err", err)
			return
		}
	}

	encoder, err := ws.ensureFrameEncoder()
	if err != nil {
		slog.Error("gogpu: blit encoder unavailable", "err", err)
		return
	}

	// Bind group is consumed by the shared frame encoder — do NOT defer
	// Release() here. The encoder is submitted in submitFrameEncoder AFTER
	// this function returns. Track for release at next frame boundary
	// (BeginFrame), where VSync guarantees GPU completion.
	texBindGrp, err := r.blitPipeline.BlitToSwapchain(encoder, ws.currentView, ws.composView, ws.width, ws.height)
	if err != nil {
		slog.Error("gogpu: blit to swapchain failed", "err", err)
		return
	}
	ws.pendingBlitBindGroup = texBindGrp
}

// Clear defers a clear command to be applied at the start of the next render pass.
// This avoids a separate render pass for clearing, which on DX12 FLIP_DISCARD
// swapchains can cause content loss due to the intermediate RT->PRESENT->RT
// state transition between Clear and the subsequent draw pass.
func (r *Renderer) Clear(red, green, blue, alpha float64) {
	r.activeSurface().clear(red, green, blue, alpha)
}

// clear defers a clear command on this window's surface.
func (ws *RenderTarget) clear(red, green, blue, alpha float64) {
	if !ws.ensureFrameStarted() {
		return
	}
	ws.pendingClearColor = gputypes.Color{R: red, G: green, B: blue, A: alpha}
	ws.hasPendingClear = true
	ws.hasGPUWork = true
}

// flushClear applies any pending clear immediately as a standalone render pass.
// Called by EndFrame if no draw calls consumed the pending clear.
func (ws *RenderTarget) flushClear(device *wgpu.Device, r *Renderer) bool {
	if err := ws.endTexturedQuadPass(); err != nil {
		slog.Error("end textured quad pass before clear failed", "err", err)
		return false
	}
	if !ws.hasPendingClear || ws.currentView == nil {
		return true
	}

	encoder := ws.frameEncoder
	shared := encoder != nil
	if !shared {
		var err error
		encoder, err = device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Clear"})
		if err != nil {
			slog.Error("create clear command encoder failed", "err", err)
			return false
		}
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       ws.renderView(),
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: ws.pendingClearColor,
			},
		},
	})
	if err != nil {
		if shared {
			ws.discardFrameEncoder()
		} else {
			encoder.DiscardEncoding()
		}
		slog.Error("begin clear render pass failed", "err", err)
		return false
	}

	if err := renderPass.End(); err != nil {
		if shared {
			ws.discardFrameEncoder()
		} else {
			encoder.DiscardEncoding()
		}
		slog.Error("end clear render pass failed", "err", err)
		return false
	}
	ws.hasPendingClear = false
	ws.frameCleared = true
	if shared {
		return true
	}

	commands, err := encoder.Finish()
	if err != nil {
		slog.Error("finish clear command encoder failed", "err", err)
		return false
	}

	r.submitTracked(commands)
	return true
}

// submitTracked submits commands with non-blocking tracking.
// The command buffer is stored and released only when GPU finishes using it.
// BUG-GOGPU-004: HAL manages fences internally — single vkQueueSubmit per frame.
func (r *Renderer) submitTracked(commands *wgpu.CommandBuffer) {
	subIdx, err := r.device.Queue().Submit(commands)
	if err != nil {
		slog.Error("submit failed", "err", err)
		return
	}
	r.tracker.track(subIdx, commands)
}

// Size returns the current render target size.
// During multi-window rendering, returns the size of the active window surface.
func (r *Renderer) Size() (width, height int) {
	ws := r.activeSurface()
	return int(ws.width), int(ws.height)
}

// Format returns the surface texture format.
// During multi-window rendering, returns the format of the active window surface.
func (r *Renderer) Format() gputypes.TextureFormat {
	return r.activeSurface().format
}

// Backend returns the name of the active backend.
func (r *Renderer) Backend() string {
	return r.backendName
}

// initTrianglePipeline creates the built-in triangle render pipeline.
func (r *Renderer) initTrianglePipeline() error {
	if r.trianglePipeline != nil {
		return nil // Already initialized
	}

	var err error

	// Create shader module
	r.triangleShader, err = r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Triangle Shader",
		WGSL:  coloredTriangleShaderSource,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create shader module: %w", err)
	}

	// Create empty pipeline layout (no bind groups needed for triangle)
	r.trianglePipelineLayout, err = r.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "Triangle Pipeline Layout",
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create triangle pipeline layout: %w", err)
	}

	// Create render pipeline
	r.trianglePipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "Triangle Pipeline",
		Layout: r.trianglePipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     r.triangleShader,
			EntryPoint: shaderEntryVS,
		},
		Fragment: &wgpu.FragmentState{
			Module:     r.triangleShader,
			EntryPoint: shaderEntryFS,
			Targets: []gputypes.ColorTargetState{
				{
					Format:    r.surfaceFormat,
					WriteMask: gputypes.ColorWriteMaskAll,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create render pipeline: %w", err)
	}

	return nil
}

// DrawTriangle draws the built-in colored triangle.
func (r *Renderer) DrawTriangle(clearR, clearG, clearB, clearA float64) error {
	ws := r.activeSurface()
	if !ws.ensureFrameStarted() {
		return nil
	}
	ws.hasGPUWork = true
	// DrawTriangle retains its standalone submission contract. Flush any
	// shared textured-quad encoder first so a triangle interleaved with quads
	// cannot overtake the earlier draws while the quad pass is still open.
	if ws.frameEncoder != nil {
		ws.submitFrameEncoder(r)
	}

	// Initialize pipeline on first use
	if r.trianglePipeline == nil {
		if err := r.initTrianglePipeline(); err != nil {
			return err
		}
	}

	encoder, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "DrawTriangle",
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create command encoder: %w", err)
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       ws.renderView(),
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: clearR, G: clearG, B: clearB, A: clearA},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to begin render pass: %w", err)
	}

	renderPass.SetPipeline(r.trianglePipeline)
	renderPass.Draw(3, 1, 0, 0) // 3 vertices, 1 instance

	if err := renderPass.End(); err != nil {
		return fmt.Errorf("gogpu: failed to end render pass: %w", err)
	}

	commands, err := encoder.Finish()
	if err != nil {
		return fmt.Errorf("gogpu: failed to finish encoding: %w", err)
	}

	// Submit with fence tracking (command buffer released when GPU done)
	r.submitTracked(commands)

	return nil
}

// initTexturedQuadPipeline creates the GPU resources for textured quad rendering.
// This is called lazily on the first DrawTexture call.
func (r *Renderer) initTexturedQuadPipeline() error {
	if r.texQuadPipelineInited {
		return nil
	}

	var err error

	// Create shader module
	r.texQuadShader, err = r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Textured Quad Shader",
		WGSL:  positionedQuadShaderSource,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create textured quad shader: %w", err)
	}

	// Create bind group layout for uniforms (group 0)
	r.texQuadUniformLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Textured Quad Uniform Layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: true,
					MinBindingSize:   texQuadUniformSize,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create uniform bind group layout: %w", err)
	}

	// Create bind group layout for texture+sampler (group 1)
	r.texQuadTextureLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Textured Quad Texture Layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create texture bind group layout: %w", err)
	}

	// Create pipeline layout with both bind group layouts
	r.texQuadPipelineLayout, err = r.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Textured Quad Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.texQuadUniformLayout, r.texQuadTextureLayout},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create pipeline layout: %w", err)
	}

	// Create render pipeline with premultiplied alpha blending.
	r.texQuadPipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "Textured Quad Pipeline",
		Layout: r.texQuadPipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     r.texQuadShader,
			EntryPoint: shaderEntryVS,
		},
		Primitive: gputypes.PrimitiveState{
			Topology: gputypes.PrimitiveTopologyTriangleList,
			CullMode: gputypes.CullModeNone,
		},
		Fragment: &wgpu.FragmentState{
			Module:     r.texQuadShader,
			EntryPoint: shaderEntryFS,
			Targets: []gputypes.ColorTargetState{
				{
					Format:    r.surfaceFormat,
					WriteMask: gputypes.ColorWriteMaskAll,
					Blend: &gputypes.BlendState{
						Color: gputypes.BlendComponent{
							Operation: gputypes.BlendOperationAdd,
							SrcFactor: gputypes.BlendFactorOne,
							DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
						},
						Alpha: gputypes.BlendComponent{
							Operation: gputypes.BlendOperationAdd,
							SrcFactor: gputypes.BlendFactorOne,
							DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create render pipeline: %w", err)
	}

	// Reuse the CPU-side payload while Queue.WriteBuffer copies it into the
	// backend's staging/direct-write path. The GPU-visible data is still kept
	// in per-draw dynamic slots above, so this scratch buffer is safe to reuse
	// between calls on the render thread.
	r.texQuadUniformData = make([]byte, texQuadUniformSize)
	r.texQuadPipelineInited = true
	return nil
}

// getOrCreateTexBindGroup returns a cached bind group for the texture, or creates one.
// This avoids creating a new GPU bind group for every draw call with the same texture.
func (r *Renderer) getOrCreateTexBindGroup(tex *Texture) (*wgpu.BindGroup, error) {
	// Initialize cache lazily
	if r.texBindGroupCache == nil {
		r.texBindGroupCache = make(map[*wgpu.TextureView]*wgpu.BindGroup)
	}

	// Check cache first
	if bg, ok := r.texBindGroupCache[tex.view]; ok {
		return bg, nil
	}

	// Create new bind group for this texture
	bg, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Textured Quad Texture Bind Group",
		Layout: r.texQuadTextureLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Sampler: tex.sampler,
			},
			{
				Binding:     1,
				TextureView: tex.view,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Store in cache
	r.texBindGroupCache[tex.view] = bg
	return bg, nil
}

// drawTexturedQuad draws a textured quad with the given options.
// This is an internal method called by Context.DrawTextureEx.
func (r *Renderer) drawTexturedQuad(tex *Texture, opts DrawTextureOptions) error {
	ws := r.activeSurface()
	if !ws.ensureFrameStarted() {
		return fmt.Errorf("gogpu: surface frame not available")
	}
	ws.hasGPUWork = true

	// HiDPI diagnostic logging (first 3 frames per surface to avoid spam).
	if ws.presentLogCount < 3 {
		ws.presentLogCount++
		slog.Debug("gogpu: drawTexturedQuad",
			"quadW", opts.Width, "quadH", opts.Height,
			"texW", tex.width, "texH", tex.height,
			"surfaceW", ws.width, "surfaceH", ws.height,
			"frame", ws.presentLogCount,
		)
	}

	// Ensure pipeline is initialized (lazy init on first draw)
	if !r.texQuadPipelineInited {
		if err := r.initTexturedQuadPipeline(); err != nil {
			return err
		}
	}

	// Premultiplied flag: 1.0 for premultiplied textures, 0.0 for straight alpha.
	var premulFlag float32
	if tex.premultiplied {
		premulFlag = 1.0
	}

	// Get or create cached bind group for texture (group 1)
	texBindGroup, err := r.getOrCreateTexBindGroup(tex)
	if err != nil {
		return fmt.Errorf("gogpu: failed to get texture bind group: %w", err)
	}

	// Record into the frame-owned encoder. Consecutive textured quads share one
	// render pass; boundary operations close it before recording their own pass,
	// preserving arbitrary texture ordering and external interleaving.
	encoder, err := ws.ensureFrameEncoder()
	if err != nil {
		return fmt.Errorf("gogpu: shared frame encoder unavailable: %w", err)
	}
	if ws.texturedQuadPass != nil && ws.hasPendingClear {
		if err := ws.endTexturedQuadPass(); err != nil {
			return err
		}
	}
	uniform, err := ws.allocateTexQuadUniform(r)
	if err != nil {
		return err
	}

	if ws.texturedQuadPass == nil {
		if err := ws.beginTexturedQuadPass(encoder); err != nil {
			return err
		}
	}

	// Set pipeline and bind groups
	ws.texturedQuadPass.SetPipeline(r.texQuadPipeline)
	ws.texturedQuadPass.SetBindGroup(0, uniform.bindGrp, []uint32{uniform.offset})
	ws.texturedQuadPass.SetBindGroup(1, texBindGroup, nil)

	// Draw 6 vertices (2 triangles for quad)
	ws.texturedQuadPass.Draw(6, 1, 0, 0)

	// Retain an immutable payload until the pass closes. Queue writes may be
	// deferred in a staging encoder and are prepended before user command buffers
	// at Submit, so publishing only after End avoids targeting an unreferenced
	// frame buffer when pass validation fails.
	binary.LittleEndian.PutUint32(r.texQuadUniformData[0:4], math.Float32bits(opts.X))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[4:8], math.Float32bits(opts.Y))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[8:12], math.Float32bits(opts.Width))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[12:16], math.Float32bits(opts.Height))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[16:20], math.Float32bits(float32(ws.width)))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[20:24], math.Float32bits(float32(ws.height)))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[24:28], math.Float32bits(opts.Alpha))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[28:32], math.Float32bits(premulFlag))
	ws.pendingTexQuadUniformWrites = append(ws.pendingTexQuadUniformWrites, texQuadUniformWrite{
		buffer: uniform.buffer,
		offset: uniform.offset,
	})
	copy(ws.pendingTexQuadUniformWrites[len(ws.pendingTexQuadUniformWrites)-1].data[:], r.texQuadUniformData)

	// Mark frame as having content (for subsequent LoadOp)
	ws.frameCleared = true

	return nil
}

// RenderToImage renders a single frame to an off-screen texture and returns
// the result as *image.RGBA. No window or display is required.
//
// The draw callback receives a Context whose drawing methods (DrawTriangleColor,
// DrawTexture, Clear, etc.) write to a temporary off-screen texture of the
// given size. After draw returns, any pending lazy clear is flushed, the
// texture is copied to CPU memory, and the pixels are returned.
//
// The renderer uses its current surfaceFormat for the off-screen texture so
// that already-compiled pipelines (triangle, textured-quad) remain valid
// without re-creation.
//
// Not safe for concurrent use with the same renderer instance.
//
// TODO: expose server-side / headless image generation via App or Context so
// callers outside this package can use it without reaching into Renderer.
func (r *Renderer) RenderToImage(width, height int, draw func(*Context)) (*image.RGBA, error) {
	if r.device == nil {
		return nil, errors.New("gogpu: RenderToImage: device not initialized")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("gogpu: RenderToImage: invalid size %dx%d", width, height)
	}

	// Use the renderer's surface format so existing pipelines (triangle, texquad)
	// share their ColorTargetState.Format without needing re-creation.
	texFmt := r.surfaceFormat

	offscreen, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "RenderToImage",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}, //nolint:gosec // G115: validated positive above
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        texFmt,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: create texture: %w", err)
	}
	defer offscreen.Release()

	view, err := r.device.CreateTextureView(offscreen, nil)
	if err != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: create view: %w", err)
	}
	defer view.Release()

	// Inject a synthetic RenderTarget so all draw methods write to our
	// off-screen view instead of a window surface.
	prevPrimary := r.primary
	synthetic := &RenderTarget{
		renderer:     r,
		width:        uint32(width),  //nolint:gosec // G115: validated positive above
		height:       uint32(height), //nolint:gosec // G115: validated positive above
		format:       texFmt,
		currentView:  view,
		frameStarted: true,
	}
	defer synthetic.releaseTexQuadUniformChunks()
	r.primary = synthetic

	ctx := newContext(r, 1.0)
	draw(ctx)
	// Flush Context.Clear() calls that were deferred as a pending clear.
	synthetic.flushClear(r.device, r)
	// Window frames submit their shared encoder in EndFrame. RenderToImage has
	// no window lifecycle, so finish the synthetic frame before issuing the
	// readback command buffer below.
	synthetic.submitFrameEncoder(r)

	r.primary = prevPrimary

	return r.renderToImageReadback(offscreen, texFmt, width, height)
}

// renderToImageReadback copies an off-screen texture to CPU memory and returns
// the pixels as *image.RGBA. Extracted from RenderToImage for funlen compliance.
func (r *Renderer) renderToImageReadback(offscreen *wgpu.Texture, texFmt gputypes.TextureFormat, width, height int) (*image.RGBA, error) {
	// wgpu requires BytesPerRow to be a multiple of 256.
	const rowAlign = 256
	rowBytes := uint32(width) * 4 //nolint:gosec // G115: validated positive in RenderToImage
	if rem := rowBytes % rowAlign; rem != 0 {
		rowBytes += rowAlign - rem
	}
	bufSize := uint64(rowBytes) * uint64(height) //nolint:gosec // G115: validated positive in RenderToImage

	stagingBuf, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "RenderToImage-staging",
		Size:  bufSize,
		Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: create staging buffer: %w", err)
	}
	defer stagingBuf.Release()

	enc, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "RenderToImage-copy",
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: create encoder: %w", err)
	}

	enc.CopyTextureToBuffer(offscreen, stagingBuf, []wgpu.BufferTextureCopy{
		{
			TextureBase: wgpu.ImageCopyTexture{
				Texture:  offscreen,
				MipLevel: 0,
				Aspect:   gputypes.TextureAspectAll,
			},
			BufferLayout: wgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  rowBytes,
				RowsPerImage: uint32(height), //nolint:gosec // G115: validated positive in RenderToImage
			},
			Size: wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}, //nolint:gosec // G115: validated positive in RenderToImage
		},
	})

	cmds, err := enc.Finish()
	if err != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: finish encoder: %w", err)
	}
	r.submitTracked(cmds)

	// Wait for the copy to land in the staging buffer.
	r.device.Poll(wgpu.PollWait)

	pending, err := stagingBuf.MapAsync(wgpu.MapModeRead, 0, bufSize)
	if err != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: MapAsync: %w", err)
	}
	r.device.Poll(wgpu.PollWait)

	ready, statusErr := pending.Status()
	if !ready {
		return nil, errors.New("gogpu: RenderToImage: buffer not ready after poll")
	}
	if statusErr != nil {
		return nil, fmt.Errorf("gogpu: RenderToImage: map status: %w", statusErr)
	}

	mapped, err := stagingBuf.MappedRange(0, bufSize)
	if err != nil {
		_ = stagingBuf.Unmap()
		return nil, fmt.Errorf("gogpu: RenderToImage: MappedRange: %w", err)
	}
	raw := mapped.Bytes()

	// BGRA8Unorm (default surfaceFormat) has B and R swapped vs image.RGBA.
	isBGRA := texFmt == gputypes.TextureFormatBGRA8Unorm || texFmt == gputypes.TextureFormatBGRA8UnormSrgb
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			src := y*int(rowBytes) + x*4
			dst := img.PixOffset(x, y)
			if isBGRA {
				img.Pix[dst+0] = raw[src+2] // R ← blue slot
				img.Pix[dst+1] = raw[src+1] // G
				img.Pix[dst+2] = raw[src+0] // B ← red slot
				img.Pix[dst+3] = raw[src+3] // A
			} else {
				copy(img.Pix[dst:dst+4], raw[src:src+4])
			}
		}
	}

	_ = stagingBuf.Unmap()
	return img, nil
}

// WaitForGPU blocks until all submitted GPU work completes.
// Call this before destroying user-created GPU resources to prevent
// Vulkan validation errors about resources still in use by command buffers.
func (r *Renderer) WaitForGPU() {
	r.tracker.waitAll(r.device)
}

// EnqueueDeferredDestroy adds a destruction function to the deferred queue.
// This is called from runtime.AddCleanup callbacks when a GPU resource is
// garbage collected without explicit Destroy/Close. The actual destruction
// happens on the render thread during DrainDeferredDestroys.
//
// Safe to call from any goroutine (including GC finalizer goroutines).
func (r *Renderer) EnqueueDeferredDestroy(fn func()) {
	r.deferredDestroysMu.Lock()
	r.deferredDestroys = append(r.deferredDestroys, fn)
	r.deferredDestroysMu.Unlock()
}

// DrainDeferredDestroys executes all pending deferred destruction functions.
// Called during shutdown and optionally at frame boundaries to release
// GPU resources that were enqueued by runtime.AddCleanup.
//
// Must be called on the render thread.
func (r *Renderer) DrainDeferredDestroys() {
	r.deferredDestroysMu.Lock()
	fns := r.deferredDestroys
	r.deferredDestroys = nil
	r.deferredDestroysMu.Unlock()

	for _, fn := range fns {
		fn()
	}
}

// Destroy releases all GPU resources.
func (r *Renderer) Destroy() {
	// Wait for all GPU work to complete before destroying resources.
	if r.device != nil {
		_ = r.device.WaitIdle()
	}

	// Wait for all tracked submissions and free their command buffers.
	r.tracker.waitAll(r.device)

	// Destroy primary window surface (per-window resources first).
	if r.primary != nil {
		r.primary.destroy()
	}

	// Release cached texture bind groups (device-level, shared)
	for view, bg := range r.texBindGroupCache {
		bg.Release()
		delete(r.texBindGroupCache, view)
	}

	r.destroyBlitPipeline()

	// Release textured quad pipeline resources (reverse order)
	if r.texQuadPipelineLayout != nil {
		r.texQuadPipelineLayout.Release()
		r.texQuadPipelineLayout = nil
	}
	if r.texQuadTextureLayout != nil {
		r.texQuadTextureLayout.Release()
		r.texQuadTextureLayout = nil
	}
	if r.texQuadUniformLayout != nil {
		r.texQuadUniformLayout.Release()
		r.texQuadUniformLayout = nil
	}
	r.texQuadUniformData = nil
	if r.texQuadShader != nil {
		r.texQuadShader.Release()
		r.texQuadShader = nil
	}
	if r.texQuadPipeline != nil {
		r.texQuadPipeline.Release()
		r.texQuadPipeline = nil
	}
	if r.triangleShader != nil {
		r.triangleShader.Release()
		r.triangleShader = nil
	}
	if r.trianglePipeline != nil {
		r.trianglePipeline.Release()
		r.trianglePipeline = nil
	}
	if r.trianglePipelineLayout != nil {
		r.trianglePipelineLayout.Release()
		r.trianglePipelineLayout = nil
	}

	// Destroy shared GPU resources in reverse order of creation
	if r.device != nil {
		r.device.Release()
		r.device = nil
	}
	if r.adapter != nil {
		r.adapter.Release()
		r.adapter = nil
	}
	// NOTE: the Vulkan instance is released separately via ReleaseInstance so
	// the X11 Display* can be closed while the driver (ICD) is still loaded.
	// On X11/NVIDIA, releasing the instance unloads the ICD; XCloseDisplay then
	// jumps into the driver's freed XESetCloseDisplay hook and SIGSEGVs.
}

// ReleaseInstance releases the Vulkan instance. Split from Destroy so callers
// can close platform display handles (X11 XCloseDisplay) before the instance —
// and thus the driver ICD — is unloaded. Safe to call after Destroy; idempotent.
func (r *Renderer) ReleaseInstance() {
	if r.instance != nil {
		r.instance.Release()
		r.instance = nil
	}
}

// destroyBlitPipeline releases the dedicated composition blit pipeline
// resources. Extracted from Destroy to keep cyclomatic complexity bounded.
func (r *Renderer) destroyBlitPipeline() {
	r.blitPipeline.Destroy()
}

// destroy releases all resources owned by this window surface.
func (ws *RenderTarget) destroy() {
	ws.discardFrameEncoder()
	ws.releaseTexQuadUniformChunks()
	ws.releaseCompositionTexture()
	if ws.currentView != nil {
		ws.currentView.Release()
		ws.currentView = nil
	}
	ws.currentSurfaceTexture = nil

	if ws.surface != nil {
		ws.surface.Release()
		ws.surface = nil
	}
	ws.state = SurfaceNone
}
