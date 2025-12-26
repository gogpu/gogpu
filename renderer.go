package gogpu

import (
	"fmt"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/backend/rust"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/platform"
)

// Renderer manages the GPU rendering pipeline.
// It handles device initialization, surface management, and frame presentation.
type Renderer struct {
	// Backend abstraction
	backend gpu.Backend

	// GPU handles
	instance types.Instance
	adapter  types.Adapter
	device   types.Device
	queue    types.Queue
	surface  types.Surface

	// Surface configuration
	format            types.TextureFormat
	width             uint32
	height            uint32
	surfaceConfigured bool // Whether surface has been configured with valid dimensions

	// Current frame state
	currentTexture types.Texture
	currentView    types.TextureView

	// Built-in pipelines
	trianglePipeline types.RenderPipeline
	triangleShader   types.ShaderModule

	// Platform reference
	platform platform.Platform
}

// newRenderer creates and initializes a new renderer.
func newRenderer(plat platform.Platform, backendType types.BackendType) (*Renderer, error) {
	// Create backend based on type
	backend, err := createBackend(backendType)
	if err != nil {
		return nil, err
	}

	r := &Renderer{
		backend:  backend,
		platform: plat,
	}

	if err := r.init(); err != nil {
		backend.Destroy()
		return nil, err
	}

	return r, nil
}

// createBackend creates a backend of the specified type.
func createBackend(typ types.BackendType) (gpu.Backend, error) {
	switch typ {
	case types.BackendRust:
		if !rust.IsAvailable() {
			return nil, fmt.Errorf("rust backend not available on this platform")
		}
		return rust.New(), nil
	case types.BackendGo:
		return native.New(), nil
	case types.BackendAuto:
		// Auto: prefer Rust backend if available, fallback to native
		if rust.IsAvailable() {
			return rust.New(), nil
		}
		return native.New(), nil
	default:
		if rust.IsAvailable() {
			return rust.New(), nil
		}
		return native.New(), nil
	}
}

// init initializes WebGPU and creates the rendering pipeline.
func (r *Renderer) init() error {
	var err error

	// Initialize backend
	if err = r.backend.Init(); err != nil {
		return fmt.Errorf("gogpu: failed to init backend: %w", err)
	}

	// Create WebGPU instance
	r.instance, err = r.backend.CreateInstance()
	if err != nil {
		return fmt.Errorf("gogpu: failed to create instance: %w", err)
	}

	// Get platform handles for surface creation
	hinstance, hwnd := r.platform.GetHandle()

	// Create surface
	r.surface, err = r.backend.CreateSurface(r.instance, types.SurfaceHandle{
		Instance: hinstance,
		Window:   hwnd,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}

	// Request adapter
	r.adapter, err = r.backend.RequestAdapter(r.instance, &types.AdapterOptions{
		PowerPreference: types.PowerPreferenceHighPerformance,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to request adapter: %w", err)
	}

	// Request device
	r.device, err = r.backend.RequestDevice(r.adapter, nil)
	if err != nil {
		return fmt.Errorf("gogpu: failed to request device: %w", err)
	}

	// Get queue
	r.queue = r.backend.GetQueue(r.device)

	// Configure surface
	// Get current window dimensions. On some platforms (especially macOS),
	// the window may not have valid dimensions immediately after creation.
	// In that case, we defer surface configuration until the first Resize event.
	width, height := r.platform.GetSize()

	// Use BGRA8Unorm which is common across platforms
	r.format = types.TextureFormatBGRA8Unorm

	// Only configure surface if dimensions are valid.
	// If dimensions are zero (window not yet visible, minimized, or timing issue),
	// defer configuration until Resize is called with valid dimensions.
	// This matches wgpu-core behavior which returns ConfigureSurfaceError::ZeroArea.
	if width > 0 && height > 0 {
		r.width = uint32(width)   //nolint:gosec // G115: validated positive above
		r.height = uint32(height) //nolint:gosec // G115: validated positive above

		r.backend.ConfigureSurface(r.surface, r.device, &types.SurfaceConfig{
			Format:      r.format,
			Usage:       types.TextureUsageRenderAttachment,
			Width:       r.width,
			Height:      r.height,
			AlphaMode:   types.AlphaModeOpaque,
			PresentMode: types.PresentModeFifo, // VSync
		})
		r.surfaceConfigured = true
	}
	// If dimensions are zero, surfaceConfigured remains false.
	// The surface will be configured on the first Resize event with valid dimensions.

	return nil
}

// Resize handles window resize.
// This also handles deferred surface configuration when the window
// first becomes visible with valid dimensions (especially important on macOS).
func (r *Renderer) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	// Note: width/height validated positive above
	r.width = uint32(width)   //nolint:gosec // G115: validated positive above
	r.height = uint32(height) //nolint:gosec // G115: validated positive above

	r.backend.ConfigureSurface(r.surface, r.device, &types.SurfaceConfig{
		Format:      r.format,
		Usage:       types.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   types.AlphaModeOpaque,
		PresentMode: types.PresentModeFifo,
	})
	r.surfaceConfigured = true
}

// BeginFrame prepares a new frame for rendering.
// Returns false if frame cannot be acquired (surface not configured, minimized, etc.).
func (r *Renderer) BeginFrame() bool {
	// Skip if surface is not configured yet.
	// This happens when the window has zero dimensions (minimized, not yet visible).
	if !r.surfaceConfigured {
		return false
	}

	surfTex, err := r.backend.GetCurrentTexture(r.surface)
	if err != nil || surfTex.Status != types.SurfaceStatusSuccess {
		// Surface needs reconfiguration.
		// Only attempt if we have valid dimensions.
		if r.width > 0 && r.height > 0 {
			r.backend.ConfigureSurface(r.surface, r.device, &types.SurfaceConfig{
				Format:      r.format,
				Usage:       types.TextureUsageRenderAttachment,
				Width:       r.width,
				Height:      r.height,
				AlphaMode:   types.AlphaModeOpaque,
				PresentMode: types.PresentModeFifo,
			})
		}
		return false
	}

	r.currentTexture = surfTex.Texture

	// Create texture view for rendering
	r.currentView = r.backend.CreateTextureView(r.currentTexture, nil)
	return r.currentView != 0
}

// EndFrame presents the rendered frame.
func (r *Renderer) EndFrame() {
	if r.currentView != 0 {
		r.backend.ReleaseTextureView(r.currentView)
		r.currentView = 0
	}

	r.backend.Present(r.surface)

	if r.currentTexture != 0 {
		r.backend.ReleaseTexture(r.currentTexture)
		r.currentTexture = 0
	}
}

// Clear submits a clear command with the specified color.
func (r *Renderer) Clear(red, green, blue, alpha float64) {
	if r.currentView == 0 {
		return
	}

	encoder := r.backend.CreateCommandEncoder(r.device)
	if encoder == 0 {
		return
	}

	renderPass := r.backend.BeginRenderPass(encoder, &types.RenderPassDescriptor{
		ColorAttachments: []types.ColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     types.LoadOpClear,
				StoreOp:    types.StoreOpStore,
				ClearValue: types.Color{R: red, G: green, B: blue, A: alpha},
			},
		},
	})

	r.backend.EndRenderPass(renderPass)
	r.backend.ReleaseRenderPass(renderPass)

	commands := r.backend.FinishEncoder(encoder)
	r.backend.ReleaseCommandEncoder(encoder)

	r.backend.Submit(r.queue, commands)
	r.backend.ReleaseCommandBuffer(commands)
}

// Size returns the current render target size.
func (r *Renderer) Size() (width, height int) {
	return int(r.width), int(r.height)
}

// Format returns the surface texture format.
func (r *Renderer) Format() types.TextureFormat {
	return r.format
}

// Backend returns the name of the active backend.
func (r *Renderer) Backend() string {
	return r.backend.Name()
}

// initTrianglePipeline creates the built-in triangle render pipeline.
func (r *Renderer) initTrianglePipeline() error {
	if r.trianglePipeline != 0 {
		return nil // Already initialized
	}

	var err error

	// Create shader module
	r.triangleShader, err = r.backend.CreateShaderModuleWGSL(r.device, coloredTriangleShaderSource)
	if err != nil {
		return fmt.Errorf("gogpu: failed to create shader module: %w", err)
	}

	// Create render pipeline
	r.trianglePipeline, err = r.backend.CreateRenderPipeline(r.device, &types.RenderPipelineDescriptor{
		VertexShader:     r.triangleShader,
		VertexEntryPoint: "vs_main",
		FragmentShader:   r.triangleShader,
		FragmentEntry:    "fs_main",
		TargetFormat:     r.format,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create render pipeline: %w", err)
	}

	return nil
}

// DrawTriangle draws the built-in colored triangle.
func (r *Renderer) DrawTriangle(clearR, clearG, clearB, clearA float64) error {
	if r.currentView == 0 {
		return nil
	}

	// Initialize pipeline on first use
	if r.trianglePipeline == 0 {
		if err := r.initTrianglePipeline(); err != nil {
			return err
		}
	}

	encoder := r.backend.CreateCommandEncoder(r.device)
	if encoder == 0 {
		return fmt.Errorf("gogpu: failed to create command encoder")
	}

	renderPass := r.backend.BeginRenderPass(encoder, &types.RenderPassDescriptor{
		ColorAttachments: []types.ColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     types.LoadOpClear,
				StoreOp:    types.StoreOpStore,
				ClearValue: types.Color{R: clearR, G: clearG, B: clearB, A: clearA},
			},
		},
	})

	r.backend.SetPipeline(renderPass, r.trianglePipeline)
	r.backend.Draw(renderPass, 3, 1, 0, 0) // 3 vertices, 1 instance

	r.backend.EndRenderPass(renderPass)
	r.backend.ReleaseRenderPass(renderPass)

	commands := r.backend.FinishEncoder(encoder)
	r.backend.ReleaseCommandEncoder(encoder)

	r.backend.Submit(r.queue, commands)
	r.backend.ReleaseCommandBuffer(commands)

	return nil
}

// Destroy releases all GPU resources.
func (r *Renderer) Destroy() {
	if r.currentView != 0 {
		r.backend.ReleaseTextureView(r.currentView)
		r.currentView = 0
	}
	if r.currentTexture != 0 {
		r.backend.ReleaseTexture(r.currentTexture)
		r.currentTexture = 0
	}

	// Backend handles cleanup of all resources
	if r.backend != nil {
		r.backend.Destroy()
	}
}
