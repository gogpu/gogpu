package gogpu

import (
	"fmt"
	"runtime"

	"github.com/go-webgpu/webgpu/wgpu"

	"github.com/gogpu/gogpu/internal/platform"
)

// Renderer manages the GPU rendering pipeline.
// It handles device initialization, surface management, and frame presentation.
type Renderer struct {
	// WebGPU core objects
	instance *wgpu.Instance
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue
	surface  *wgpu.Surface

	// Surface configuration
	surfaceConfig *wgpu.SurfaceConfiguration
	format        wgpu.TextureFormat
	width         uint32
	height        uint32

	// Current frame state
	currentTexture *wgpu.SurfaceTexture
	currentView    *wgpu.TextureView

	// Built-in pipelines
	trianglePipeline *wgpu.RenderPipeline

	// Platform reference
	platform platform.Platform
}

// newRenderer creates and initializes a new renderer.
func newRenderer(plat platform.Platform) (*Renderer, error) {
	r := &Renderer{
		platform: plat,
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return r, nil
}

// init initializes WebGPU and creates the rendering pipeline.
func (r *Renderer) init() error {
	var err error

	// Create WebGPU instance
	r.instance, err = wgpu.CreateInstance(nil)
	if err != nil {
		return fmt.Errorf("gogpu: failed to create wgpu instance: %w", err)
	}

	// Get platform handles for surface creation
	hinstance, hwnd := r.platform.GetHandle()

	// Create surface based on platform
	switch runtime.GOOS {
	case "windows":
		r.surface, err = r.instance.CreateSurfaceFromWindowsHWND(hinstance, hwnd)
	default:
		return ErrPlatformNotSupported
	}
	if err != nil {
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}

	// Request adapter
	r.adapter, err = r.instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference: wgpu.PowerPreferenceHighPerformance,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to request adapter: %w", err)
	}

	// Request device
	r.device, err = r.adapter.RequestDevice(nil)
	if err != nil {
		return fmt.Errorf("gogpu: failed to request device: %w", err)
	}

	// Get queue
	r.queue = r.device.GetQueue()

	// Configure surface
	width, height := r.platform.GetSize()
	r.width = uint32(width)
	r.height = uint32(height)

	// Use BGRA8Unorm which is common on Windows
	r.format = wgpu.TextureFormatBGRA8Unorm

	r.surfaceConfig = &wgpu.SurfaceConfiguration{
		Device:      r.device,
		Format:      r.format,
		Usage:       wgpu.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   wgpu.CompositeAlphaModeOpaque,
		PresentMode: wgpu.PresentModeFifo, // VSync
	}

	r.surface.Configure(r.surfaceConfig)

	return nil
}

// Resize handles window resize.
func (r *Renderer) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	r.width = uint32(width)
	r.height = uint32(height)

	r.surfaceConfig.Width = r.width
	r.surfaceConfig.Height = r.height
	r.surface.Configure(r.surfaceConfig)
}

// BeginFrame prepares a new frame for rendering.
// Returns false if frame cannot be acquired.
func (r *Renderer) BeginFrame() bool {
	var err error
	r.currentTexture, err = r.surface.GetCurrentTexture()
	if err != nil {
		// Surface needs reconfiguration
		r.surface.Configure(r.surfaceConfig)
		return false
	}

	// Create texture view for rendering
	r.currentView = r.currentTexture.Texture.CreateView(nil)
	return true
}

// EndFrame presents the rendered frame.
func (r *Renderer) EndFrame() {
	if r.currentView != nil {
		r.currentView.Release()
		r.currentView = nil
	}

	r.surface.Present()

	if r.currentTexture != nil && r.currentTexture.Texture != nil {
		r.currentTexture.Texture.Release()
	}
	r.currentTexture = nil
}

// Clear submits a clear command with the specified color.
func (r *Renderer) Clear(red, green, blue, alpha float64) {
	if r.currentView == nil {
		return
	}

	encoder := r.device.CreateCommandEncoder(nil)

	renderPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     wgpu.LoadOpClear,
				StoreOp:    wgpu.StoreOpStore,
				ClearValue: wgpu.Color{R: red, G: green, B: blue, A: alpha},
			},
		},
	})
	renderPass.End()
	renderPass.Release()

	commands := encoder.Finish(nil)
	encoder.Release()

	r.queue.Submit(commands)
	commands.Release()
}

// Size returns the current render target size.
func (r *Renderer) Size() (width, height int) {
	return int(r.width), int(r.height)
}

// Format returns the surface texture format.
func (r *Renderer) Format() wgpu.TextureFormat {
	return r.format
}

// Device returns the underlying WebGPU device.
// Use for advanced rendering operations.
func (r *Renderer) Device() *wgpu.Device {
	return r.device
}

// Queue returns the command queue.
func (r *Renderer) Queue() *wgpu.Queue {
	return r.queue
}

// CurrentView returns the current frame's texture view.
// Only valid between BeginFrame and EndFrame.
func (r *Renderer) CurrentView() *wgpu.TextureView {
	return r.currentView
}

// initTrianglePipeline creates the built-in triangle render pipeline.
func (r *Renderer) initTrianglePipeline() error {
	if r.trianglePipeline != nil {
		return nil // Already initialized
	}

	// Create shader module
	shaderModule := r.device.CreateShaderModuleWGSL(coloredTriangleShaderSource)
	if shaderModule == nil {
		return fmt.Errorf("gogpu: failed to create shader module")
	}
	defer shaderModule.Release()

	// Create render pipeline using the simple helper
	r.trianglePipeline = r.device.CreateRenderPipelineSimple(
		nil, // auto layout
		shaderModule, "vs_main",
		shaderModule, "fs_main",
		r.format,
	)
	if r.trianglePipeline == nil {
		return fmt.Errorf("gogpu: failed to create render pipeline")
	}

	return nil
}

// DrawTriangle draws the built-in colored triangle.
func (r *Renderer) DrawTriangle(clearR, clearG, clearB, clearA float64) error {
	if r.currentView == nil {
		return nil
	}

	// Initialize pipeline on first use
	if r.trianglePipeline == nil {
		if err := r.initTrianglePipeline(); err != nil {
			return err
		}
	}

	encoder := r.device.CreateCommandEncoder(nil)

	renderPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     wgpu.LoadOpClear,
				StoreOp:    wgpu.StoreOpStore,
				ClearValue: wgpu.Color{R: clearR, G: clearG, B: clearB, A: clearA},
			},
		},
	})

	renderPass.SetPipeline(r.trianglePipeline)
	renderPass.Draw(3, 1, 0, 0) // 3 vertices, 1 instance

	renderPass.End()
	renderPass.Release()

	commands := encoder.Finish(nil)
	encoder.Release()

	r.queue.Submit(commands)
	commands.Release()

	return nil
}

// Destroy releases all GPU resources.
func (r *Renderer) Destroy() {
	if r.trianglePipeline != nil {
		r.trianglePipeline.Release()
		r.trianglePipeline = nil
	}
	if r.currentView != nil {
		r.currentView.Release()
		r.currentView = nil
	}
	if r.currentTexture != nil && r.currentTexture.Texture != nil {
		r.currentTexture.Texture.Release()
		r.currentTexture = nil
	}
	if r.surface != nil {
		r.surface.Release()
		r.surface = nil
	}
	if r.queue != nil {
		r.queue.Release()
		r.queue = nil
	}
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
