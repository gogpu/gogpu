package gogpu

import (
	"fmt"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/backend/rust"
	"github.com/gogpu/gogpu/internal/platform"
)

// Renderer manages the GPU rendering pipeline.
// It handles device initialization, surface management, and frame presentation.
type Renderer struct {
	// Backend abstraction
	backend gpu.Backend

	// GPU handles
	instance gpu.Instance
	adapter  gpu.Adapter
	device   gpu.Device
	queue    gpu.Queue
	surface  gpu.Surface

	// Surface configuration
	format gpu.TextureFormat
	width  uint32
	height uint32

	// Current frame state
	currentTexture gpu.Texture
	currentView    gpu.TextureView

	// Built-in pipelines
	trianglePipeline gpu.RenderPipeline
	triangleShader   gpu.ShaderModule

	// Platform reference
	platform platform.Platform
}

// newRenderer creates and initializes a new renderer.
func newRenderer(plat platform.Platform, backendType gpu.BackendType) (*Renderer, error) {
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
func createBackend(typ gpu.BackendType) (gpu.Backend, error) {
	switch typ {
	case gpu.BackendRust:
		if !rust.IsAvailable() {
			return nil, fmt.Errorf("rust backend not available on this platform")
		}
		return rust.New(), nil
	case gpu.BackendGo:
		return native.New(), nil
	case gpu.BackendAuto:
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
	r.surface, err = r.backend.CreateSurface(r.instance, gpu.SurfaceHandle{
		Instance: hinstance,
		Window:   hwnd,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}

	// Request adapter
	r.adapter, err = r.backend.RequestAdapter(r.instance, &gpu.AdapterOptions{
		PowerPreference: gpu.PowerPreferenceHighPerformance,
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
	width, height := r.platform.GetSize()
	r.width = uint32(width)
	r.height = uint32(height)

	// Use BGRA8Unorm which is common on Windows
	r.format = gpu.TextureFormatBGRA8Unorm

	r.backend.ConfigureSurface(r.surface, r.device, &gpu.SurfaceConfig{
		Format:      r.format,
		Usage:       gpu.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   gpu.AlphaModeOpaque,
		PresentMode: gpu.PresentModeFifo, // VSync
	})

	return nil
}

// Resize handles window resize.
func (r *Renderer) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	r.width = uint32(width)
	r.height = uint32(height)

	r.backend.ConfigureSurface(r.surface, r.device, &gpu.SurfaceConfig{
		Format:      r.format,
		Usage:       gpu.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   gpu.AlphaModeOpaque,
		PresentMode: gpu.PresentModeFifo,
	})
}

// BeginFrame prepares a new frame for rendering.
// Returns false if frame cannot be acquired.
func (r *Renderer) BeginFrame() bool {
	surfTex, err := r.backend.GetCurrentTexture(r.surface)
	if err != nil || surfTex.Status != gpu.SurfaceStatusSuccess {
		// Surface needs reconfiguration
		r.backend.ConfigureSurface(r.surface, r.device, &gpu.SurfaceConfig{
			Format:      r.format,
			Usage:       gpu.TextureUsageRenderAttachment,
			Width:       r.width,
			Height:      r.height,
			AlphaMode:   gpu.AlphaModeOpaque,
			PresentMode: gpu.PresentModeFifo,
		})
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

	renderPass := r.backend.BeginRenderPass(encoder, &gpu.RenderPassDescriptor{
		ColorAttachments: []gpu.ColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     gpu.LoadOpClear,
				StoreOp:    gpu.StoreOpStore,
				ClearColor: gpu.Color{R: red, G: green, B: blue, A: alpha},
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
func (r *Renderer) Format() gpu.TextureFormat {
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
	r.trianglePipeline, err = r.backend.CreateRenderPipeline(r.device, &gpu.RenderPipelineDescriptor{
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

	renderPass := r.backend.BeginRenderPass(encoder, &gpu.RenderPassDescriptor{
		ColorAttachments: []gpu.ColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     gpu.LoadOpClear,
				StoreOp:    gpu.StoreOpStore,
				ClearColor: gpu.Color{R: clearR, G: clearG, B: clearB, A: clearA},
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
