package gogpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"math"
	"os"
	"sync"

	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
)

// texQuadUniformSize is the size of the uniform buffer for textured quads.
// Layout: rect(4 floats) + screen(2 floats) + alpha(1 float) + premultiplied(1 float) = 32 bytes
const texQuadUniformSize = 32

// windowSurface holds per-window GPU rendering state.
// Each window gets its own surface, format, dimensions, and frame state.
// In multi-window mode, one Renderer holds multiple windowSurface instances.
// Currently, only the primary windowSurface is used (single-window backward compat).
type windowSurface struct {
	renderer *Renderer // back-reference to shared GPU state

	surface *wgpu.Surface
	format  gputypes.TextureFormat
	width   uint32
	height  uint32

	configured bool // Whether surface has been configured with valid dimensions

	// Current frame state (reused, zero alloc per frame)
	currentSurfaceTexture *wgpu.SurfaceTexture
	currentView           *wgpu.TextureView
	frameCleared          bool // Whether the frame has been cleared (for LoadOp selection)

	// Deferred clear -- eliminates separate Clear render pass.
	// ClearColor stores the color and sets hasPendingClear=true.
	// The next drawTexturedQuad uses LoadOpClear with this color
	// instead of a separate render pass (avoids double RT->PRESENT->RT
	// state transition that can lose content on DX12 FLIP_DISCARD).
	pendingClearColor gputypes.Color
	hasPendingClear   bool

	// VSync preference for this window
	vsync bool

	// damageRects holds the dirty regions for the current frame (physical pixels,
	// top-left origin). Set by Context.SetDamageRects(), consumed and cleared by
	// present(). When nil, the full surface is presented (backward compatible).
	// Passed to wgpu Surface.PresentWithDamage() which forwards to the platform
	// compositor (Vulkan VK_KHR_incremental_present, DX12 Present1, GLES
	// eglSwapBuffersWithDamageKHR, Software partial BitBlt/XPutImage).
	damageRects []image.Rectangle
}

// Renderer manages the GPU rendering pipeline.
// It handles device initialization, surface management, and frame presentation.
//
// The renderer uses the wgpu public API for all GPU operations. Both the native
// (Pure Go) and Rust backends are accessed through this unified API layer.
//
// Architecture: Renderer holds shared GPU state (instance, adapter, device,
// pipelines) and a primary windowSurface for per-window rendering state.
// This split prepares for multi-window support where one Renderer serves
// multiple windows.
type Renderer struct {
	// Shared GPU objects
	instance *wgpu.Instance
	adapter  *wgpu.Adapter
	device   *wgpu.Device

	// Backend metadata
	backendName string

	// Submission tracker for non-blocking resource recycling.
	// Each Submit returns a submission index; Poll returns the last completed.
	// Command buffers are freed when their submission completes.
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
	texQuadUniformBuffer  *wgpu.Buffer
	texQuadUniformBindGrp *wgpu.BindGroup
	texQuadUniformData    []byte // Pre-allocated buffer for uniform data (reduces GC pressure)
	texQuadPipelineInited bool

	// Texture bind group cache - avoids creating new bind groups per draw call.
	// Keyed by *wgpu.TextureView pointer identity.
	// Device-level resource, shared across all windows.
	texBindGroupCache map[*wgpu.TextureView]*wgpu.BindGroup

	// Deferred destruction queue for resources enqueued by runtime.AddCleanup.
	// These are resources that were garbage collected without explicit Close/Destroy.
	// Drained at the start of each frame (BeginFrame) when GPU is idle.
	deferredDestroys   []func()
	deferredDestroysMu sync.Mutex

	// Platform window reference for surface creation and frame queries
	platWindow platform.PlatformWindow

	// PowerPreference for adapter selection
	powerPreference gputypes.PowerPreference

	// Primary window surface (single-window backward compatibility).
	// In multi-window mode, additional surfaces will be stored separately.
	primary *windowSurface

	// currentSurface is set during multi-window rendering to the windowSurface
	// of the window being drawn. Draw methods (drawTexturedQuad, DrawTriangle)
	// use activeSurface() which returns currentSurface if set, otherwise primary.
	// Set by the multi-window frame loop before each window's draw callback,
	// cleared after endFrame.
	currentSurface *windowSurface
}

// newRenderer creates and initializes a new renderer.
func newRenderer(platWin platform.PlatformWindow, backendType types.BackendType, graphicsAPI types.GraphicsAPI, vsync bool, powerPref gputypes.PowerPreference) (*Renderer, error) {
	r := &Renderer{
		platWindow:      platWin,
		powerPreference: powerPref,
	}
	// Store vsync temporarily; initCommon will move it to the primary windowSurface.
	r.primary = &windowSurface{
		renderer: r,
		vsync:    vsync,
	}

	if err := r.init(backendType, graphicsAPI); err != nil {
		return nil, err
	}

	return r, nil
}

// init initializes WebGPU and creates the rendering pipeline.
func (r *Renderer) init(backendType types.BackendType, graphicsAPI types.GraphicsAPI) error {
	// Select backend and initialize via the appropriate path.
	// BackendRust requires -tags rust build.
	// BackendNative/BackendGo uses the pure Go wgpu implementation.
	// BackendAuto prefers Rust if available, otherwise falls back to native.

	useRust := false
	switch backendType {
	case types.BackendRust:
		if !rustHalAvailable() {
			return fmt.Errorf("gogpu: rust backend requested but not available (build with -tags rust)")
		}
		useRust = true
	case types.BackendNative:
		// Use native (pure Go) path
	default: // BackendAuto
		if rustHalAvailable() {
			useRust = true
		}
	}

	if useRust {
		return r.initRust()
	}
	return r.initNative(graphicsAPI)
}

// initNative initializes the renderer using the pure Go wgpu path.
// This uses wgpu.CreateInstance() which discovers HAL backends registered
// by the native backend package imports (vulkan, metal, dx12, gles).
func (r *Renderer) initNative(graphicsAPI types.GraphicsAPI) error {
	// Get backend metadata. The import side-effects in native.BackendInfo
	// register the HAL backends (vulkan, metal, etc.) via init() functions.
	var backendVariant gputypes.Backend
	r.backendName, backendVariant = native.BackendInfo(graphicsAPI)

	// Create WebGPU instance via the wgpu public API.
	// Enable debug/validation layer when GOGPU_DEBUG=1 is set. This catches
	// GPU-side errors (invalid shaders, bad PSO, etc.) before submission,
	// preventing driver-level crashes (e.g. DPC_WATCHDOG_VIOLATION BSOD on DX12).
	var instanceFlags gputypes.InstanceFlags
	if os.Getenv("GOGPU_DEBUG") == "1" {
		instanceFlags = gputypes.InstanceFlagsDebug | gputypes.InstanceFlagsValidation
	}
	var err error
	r.instance, err = wgpu.CreateInstance(&wgpu.InstanceDescriptor{
		Backends: 1 << backendVariant,
		Flags:    instanceFlags,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create instance: %w", err)
	}

	// Get platform handles for surface creation
	displayHandle, windowHandle := r.platWindow.GetHandle()

	// Create surface via wgpu public API — stored on primary windowSurface
	surface, err := r.instance.CreateSurface(displayHandle, windowHandle)
	if err != nil {
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}
	r.primary.surface = surface

	// Request adapter compatible with the surface.
	// Passing CompatibleSurface is required for GLES backends which defer
	// adapter enumeration until a surface (GL context) is available.
	r.adapter, err = r.instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface: r.primary.surface,
		PowerPreference:   r.powerPreference,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to request adapter: %w", err)
	}
	slog.Info("adapter selected", "name", r.adapter.Info().Name, "type", r.adapter.Info().DeviceType)

	// Request device with default features and limits
	r.device, err = r.adapter.RequestDevice(nil)
	if err != nil {
		return fmt.Errorf("gogpu: failed to request device: %w", err)
	}

	return r.initCommon()
}

// initRust initializes the renderer using the Rust (wgpu-native) backend.
// The Rust backend creates HAL objects which are wrapped into wgpu types
// via NewDeviceFromHAL and NewSurfaceFromHAL for a unified API.
func (r *Renderer) initRust() error {
	halBackend, backendName, backendVariant := newRustHalBackend()
	r.backendName = backendName

	// Create HAL instance via Rust backend
	halInstance, err := halBackend.CreateInstance(&hal.InstanceDescriptor{
		Backends: gputypes.Backends(backendVariant),
		Flags:    gputypes.InstanceFlagsDebug | gputypes.InstanceFlagsValidation,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create rust instance: %w", err)
	}

	// Get platform handles for surface creation
	displayHandle, windowHandle := r.platWindow.GetHandle()

	// Create HAL surface
	halSurface, err := halInstance.CreateSurface(displayHandle, windowHandle)
	if err != nil {
		halInstance.Destroy()
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}

	// Enumerate adapters and pick the first compatible one
	adapters := halInstance.EnumerateAdapters(halSurface)
	if len(adapters) == 0 {
		halSurface.Destroy()
		halInstance.Destroy()
		return fmt.Errorf("gogpu: no compatible GPU adapters found")
	}
	exposed := adapters[0]

	// Open device with default features and limits
	openDevice, err := exposed.Adapter.Open(0, gputypes.DefaultLimits())
	if err != nil {
		halSurface.Destroy()
		halInstance.Destroy()
		return fmt.Errorf("gogpu: failed to open device: %w", err)
	}

	// Wrap HAL objects into wgpu types for the unified API.
	// NewDeviceFromHAL creates a core.Device internally and wraps it.
	r.device, err = wgpu.NewDeviceFromHAL(
		openDevice.Device,
		openDevice.Queue,
		exposed.Features,
		exposed.Capabilities.Limits,
		"Rust Device",
	)
	if err != nil {
		halSurface.Destroy()
		halInstance.Destroy()
		return fmt.Errorf("gogpu: failed to wrap rust device: %w", err)
	}

	// Wrap HAL surface into wgpu.Surface — stored on primary windowSurface.
	r.primary.surface = wgpu.NewSurfaceFromHAL(halSurface, "Rust Surface")

	// Note: We don't wrap halInstance and exposed.Adapter into wgpu types
	// because the renderer only needs them for cleanup. We store nil for
	// instance/adapter -- the halInstance will be cleaned up when the
	// wgpu device/surface are released (they hold the HAL references).
	// TODO: Proper lifecycle management for Rust HAL instance/adapter.

	return r.initCommon()
}

// initCommon performs common initialization after device and surface are ready.
// This is shared between the native and Rust init paths.
func (r *Renderer) initCommon() error {
	// Submission tracker is zero-value ready — no initialization needed.

	// Configure primary window surface with PHYSICAL pixel dimensions.
	// GPU surfaces operate in device pixels, not logical points.
	// On some platforms (especially macOS), the window may not have valid
	// dimensions immediately after creation. In that case, we defer surface
	// configuration until the first Resize event.
	width, height := r.platWindow.PhysicalSize()

	// Use BGRA8Unorm which is common across platforms
	r.primary.format = gputypes.TextureFormatBGRA8Unorm

	// Only configure surface if dimensions are valid.
	// If dimensions are zero (window not yet visible, minimized, or timing issue),
	// defer configuration until Resize is called with valid dimensions.
	// This matches wgpu-core behavior which returns ConfigureSurfaceError::ZeroArea.
	if width > 0 && height > 0 {
		r.primary.width = uint32(width)   //nolint:gosec // G115: validated positive above
		r.primary.height = uint32(height) //nolint:gosec // G115: validated positive above

		if err := r.primary.configure(r.device, r.adapter); err != nil {
			return fmt.Errorf("gogpu: failed to configure surface: %w", err)
		}
		r.primary.configured = true
	}
	// If dimensions are zero, configured remains false.
	// The surface will be configured on the first Resize event with valid dimensions.

	return nil
}

// configure configures the wgpu surface with current dimensions and format.
func (ws *windowSurface) configure(device *wgpu.Device, adapter *wgpu.Adapter) error {
	presentMode := ws.resolvePresentMode(adapter)

	return ws.surface.Configure(device, &wgpu.SurfaceConfiguration{
		Format:      ws.format,
		Usage:       gputypes.TextureUsageRenderAttachment,
		Width:       ws.width,
		Height:      ws.height,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
		PresentMode: presentMode,
	})
}

// resolvePresentMode selects the best available present mode following the
// Rust wgpu fallback pattern. For VSync on (AutoVsync): FifoRelaxed -> Fifo.
// For VSync off (AutoNoVsync): Immediate -> Mailbox -> Fifo.
// Falls back to Fifo which is guaranteed by the Vulkan spec.
func (ws *windowSurface) resolvePresentMode(adapter *wgpu.Adapter) gputypes.PresentMode {
	caps := adapter.GetSurfaceCapabilities(ws.surface)
	if caps == nil {
		// No capabilities available — use safe default.
		mode := gputypes.PresentModeFifo
		if !ws.vsync {
			mode = gputypes.PresentModeImmediate
		}
		slog.Debug("gogpu: no surface capabilities, using default present mode",
			"mode", mode, "vsync", ws.vsync)
		return mode
	}

	supported := caps.PresentModes
	var mode gputypes.PresentMode

	if ws.vsync {
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
		"mode", mode, "vsync", ws.vsync, "supported", supported)
	return mode
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

// activeSurface returns the currently active windowSurface for draw operations.
// During multi-window rendering, this returns the surface of the window being drawn.
// Otherwise, it returns the primary window surface.
func (r *Renderer) activeSurface() *windowSurface {
	if r.currentSurface != nil {
		return r.currentSurface
	}
	return r.primary
}

// Resize handles window resize.
// This also handles deferred surface configuration when the window
// first becomes visible with valid dimensions (especially important on macOS).
func (r *Renderer) Resize(width, height int) {
	r.primary.resize(width, height, r.device, r.adapter)
}

// resize handles window resize for this surface.
func (ws *windowSurface) resize(width, height int, device *wgpu.Device, adapter *wgpu.Adapter) {
	if width <= 0 || height <= 0 {
		// Window minimized or invisible -- unconfigure surface to prevent
		// zero-extent swapchain creation on the next frame (VK-VAL-001).
		if ws.configured {
			ws.surface.Unconfigure()
			ws.configured = false
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

	// Note: width/height validated positive above
	ws.width = uint32(width)   //nolint:gosec // G115: validated positive above
	ws.height = uint32(height) //nolint:gosec // G115: validated positive above

	// Configure surface with new dimensions.
	if err := ws.configure(device, adapter); err != nil {
		// Restore old dimensions to keep surface consistent with swapchain.
		// Next frame will retry with the new size.
		ws.width = oldWidth
		ws.height = oldHeight
		return
	}
	ws.configured = true
}

// BeginFrame prepares a new frame for rendering.
// Returns false if frame cannot be acquired (surface not configured, minimized, etc.).
func (r *Renderer) BeginFrame() bool {
	// Drain deferred destruction queue at frame boundary.
	// Resources enqueued by runtime.AddCleanup are destroyed here
	// on the render thread where GPU operations are safe.
	r.DrainDeferredDestroys()

	return r.primary.beginFrame(r.platWindow, r.device, r.adapter)
}

// beginFrame acquires the next surface texture for rendering on this window.
// Returns false if frame cannot be acquired (surface not configured, minimized, etc.).
func (ws *windowSurface) beginFrame(platWin platform.PlatformWindow, device *wgpu.Device, adapter *wgpu.Adapter) bool {
	// Skip if surface is not configured yet.
	// This happens when the window has zero dimensions (minimized, not yet visible).
	if !ws.configured {
		return false
	}

	// Before acquiring surface texture, let platform update surface state
	// (e.g., CAMetalLayer.contentsScale on macOS for HiDPI/multi-monitor).
	if platWin != nil {
		result := platWin.PrepareFrame()
		if result.ScaleChanged && result.PhysicalWidth > 0 && result.PhysicalHeight > 0 {
			// Scale changed (window moved between monitors with different DPI).
			// Reconfigure surface with new physical dimensions.
			ws.width = result.PhysicalWidth
			ws.height = result.PhysicalHeight
			_ = ws.configure(device, adapter)
		}
	}

	// Acquire the next surface texture via wgpu public API.
	surfaceTexture, _, err := ws.surface.GetCurrentTexture()
	if err != nil {
		slog.Error("GET TEXTURE ERROR", "err", err)
		// Surface needs reconfiguration (outdated or lost).
		// Only attempt if we have valid dimensions.
		if ws.width > 0 && ws.height > 0 {
			_ = ws.configure(device, adapter)
		}
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
	ws.hasPendingClear = false

	return true
}

// EndFrame presents the rendered frame on the primary window.
func (r *Renderer) EndFrame() {
	// Flush any pending clear that wasn't consumed by a draw call.
	// This handles the case where user calls ClearColor without drawing.
	r.primary.flushClear(r.device, r)

	// Present the surface texture on the primary window.
	r.primary.present()

	// Non-blocking submission tracking: free resources for completed submissions.
	r.pollSubmissions()

	// Release per-frame resources after presentation.
	r.primary.releaseFrame()
}

// endFrameForSurface flushes, presents, and releases frame resources for a
// specific windowSurface. Used by the multi-window frame loop. Unlike EndFrame,
// it does NOT poll submissions -- the caller polls once after all windows.
func (r *Renderer) endFrameForSurface(ws *windowSurface) {
	ws.flushClear(r.device, r)
	ws.present()
	ws.releaseFrame()
}

// pollSubmissions performs non-blocking submission tracking: frees GPU resources
// for completed submissions. Called once per frame after all windows are presented.
func (r *Renderer) pollSubmissions() {
	completedIdx := r.device.Queue().Poll()
	r.tracker.triage(completedIdx, r.device)
}

// present presents the surface texture to the screen, passing any
// damage rects to the platform compositor. Damage rects are consumed
// (set to nil) after presentation so they don't leak to the next frame.
func (ws *windowSurface) present() {
	if ws.currentSurfaceTexture != nil {
		if err := ws.surface.PresentWithDamage(ws.currentSurfaceTexture, ws.damageRects); err != nil {
			slog.Error("PRESENT ERROR", "err", err)
		}
		ws.damageRects = nil
	}
}

// releaseFrame releases per-frame resources after presentation.
func (ws *windowSurface) releaseFrame() {
	if ws.currentView != nil {
		ws.currentView.Release()
		ws.currentView = nil
	}
	// SurfaceTexture is consumed by Present, no need to destroy it
	ws.currentSurfaceTexture = nil
}

// Clear defers a clear command to be applied at the start of the next render pass.
// This avoids a separate render pass for clearing, which on DX12 FLIP_DISCARD
// swapchains can cause content loss due to the intermediate RT->PRESENT->RT
// state transition between Clear and the subsequent draw pass.
func (r *Renderer) Clear(red, green, blue, alpha float64) {
	r.activeSurface().clear(red, green, blue, alpha)
}

// clear defers a clear command on this window's surface.
func (ws *windowSurface) clear(red, green, blue, alpha float64) {
	if ws.currentView == nil {
		return
	}
	ws.pendingClearColor = gputypes.Color{R: red, G: green, B: blue, A: alpha}
	ws.hasPendingClear = true
}

// flushClear applies any pending clear immediately as a standalone render pass.
// Called by EndFrame if no draw calls consumed the pending clear.
func (ws *windowSurface) flushClear(device *wgpu.Device, r *Renderer) {
	if !ws.hasPendingClear || ws.currentView == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "Clear",
	})
	if err != nil {
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       ws.currentView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: ws.pendingClearColor,
			},
		},
	})
	if err != nil {
		return
	}

	if err := renderPass.End(); err != nil {
		return
	}

	commands, err := encoder.Finish()
	if err != nil {
		return
	}

	r.submitTracked(commands)
	ws.hasPendingClear = false
	ws.frameCleared = true
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
			EntryPoint: "vs_main",
		},
		Fragment: &wgpu.FragmentState{
			Module:     r.triangleShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{
				{
					Format:    r.primary.format,
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
	if ws.currentView == nil {
		return nil
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
				View:       ws.currentView,
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
//
//nolint:funlen // pipeline init is inherently sequential setup code
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
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: texQuadUniformSize,
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
			EntryPoint: "vs_main",
		},
		Primitive: gputypes.PrimitiveState{
			Topology: gputypes.PrimitiveTopologyTriangleList,
			CullMode: gputypes.CullModeNone,
		},
		Fragment: &wgpu.FragmentState{
			Module:     r.texQuadShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{
				{
					Format:    r.primary.format,
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

	// Create uniform buffer
	r.texQuadUniformBuffer, err = r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Textured Quad Uniforms",
		Size:             texQuadUniformSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageMapWrite,
		MappedAtCreation: true,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create uniform buffer: %w", err)
	}

	// Create bind group for uniforms (group 0)
	r.texQuadUniformBindGrp, err = r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Textured Quad Uniform Bind Group",
		Layout: r.texQuadUniformLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  r.texQuadUniformBuffer,
				Size:    texQuadUniformSize,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create uniform bind group: %w", err)
	}

	// Pre-allocate uniform data buffer to avoid per-frame allocations
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
	if ws.currentView == nil {
		return nil // No frame in progress
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

	// Create command encoder
	encoder, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "DrawTexturedQuad",
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create command encoder: %w", err)
	}

	// Upload uniform data — screen dimensions come from per-window state
	binary.LittleEndian.PutUint32(r.texQuadUniformData[0:4], math.Float32bits(opts.X))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[4:8], math.Float32bits(opts.Y))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[8:12], math.Float32bits(opts.Width))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[12:16], math.Float32bits(opts.Height))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[16:20], math.Float32bits(float32(ws.width)))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[20:24], math.Float32bits(float32(ws.height)))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[24:28], math.Float32bits(opts.Alpha))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[28:32], math.Float32bits(premulFlag))
	if err := r.device.Queue().WriteBuffer(r.texQuadUniformBuffer, 0, r.texQuadUniformData); err != nil {
		return fmt.Errorf("gogpu: WriteBuffer uniform failed: %w", err)
	}

	// Determine LoadOp: consume pending clear if available, otherwise preserve content.
	loadOp := gputypes.LoadOpClear
	clearValue := gputypes.Color{R: 0, G: 0, B: 0, A: 1}
	if ws.hasPendingClear {
		clearValue = ws.pendingClearColor
		ws.hasPendingClear = false
	} else if ws.frameCleared {
		loadOp = gputypes.LoadOpLoad
	}

	// Begin render pass
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       ws.currentView,
				LoadOp:     loadOp,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: clearValue,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to begin render pass: %w", err)
	}

	// Set pipeline and bind groups
	renderPass.SetPipeline(r.texQuadPipeline)
	renderPass.SetBindGroup(0, r.texQuadUniformBindGrp, nil)
	renderPass.SetBindGroup(1, texBindGroup, nil)

	// Draw 6 vertices (2 triangles for quad)
	renderPass.Draw(6, 1, 0, 0)

	// End render pass
	if err := renderPass.End(); err != nil {
		return fmt.Errorf("gogpu: failed to end render pass: %w", err)
	}

	// Finish and submit
	commands, err := encoder.Finish()
	if err != nil {
		return fmt.Errorf("gogpu: failed to finish encoding: %w", err)
	}

	// Submit with fence tracking (command buffer released when GPU done)
	r.submitTracked(commands)

	// Mark frame as having content (for subsequent LoadOp)
	ws.frameCleared = true

	return nil
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

	// Release textured quad pipeline resources (reverse order)
	if r.texQuadUniformBindGrp != nil {
		r.texQuadUniformBindGrp.Release()
		r.texQuadUniformBindGrp = nil
	}
	if r.texQuadUniformBuffer != nil {
		r.texQuadUniformBuffer.Release()
		r.texQuadUniformBuffer = nil
	}
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
	if r.instance != nil {
		r.instance.Release()
		r.instance = nil
	}
}

// destroy releases all resources owned by this window surface.
func (ws *windowSurface) destroy() {
	if ws.currentView != nil {
		ws.currentView.Release()
		ws.currentView = nil
	}
	ws.currentSurfaceTexture = nil

	if ws.surface != nil {
		ws.surface.Release()
		ws.surface = nil
	}
}
