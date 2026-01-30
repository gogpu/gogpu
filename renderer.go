package gogpu

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gogpu/gogpu/gpu"
	_ "github.com/gogpu/gogpu/gpu/backend/native" // Register native backend
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gputypes"
)

// texQuadUniformSize is the size of the uniform buffer for textured quads.
// Layout: rect(4 floats) + screen(2 floats) + alpha(1 float) + pad(1 float) = 32 bytes
const texQuadUniformSize = 32

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
	format            gputypes.TextureFormat
	width             uint32
	height            uint32
	surfaceConfigured bool // Whether surface has been configured with valid dimensions

	// Current frame state
	currentTexture types.Texture
	currentView    types.TextureView
	frameCleared   bool // Whether the frame has been cleared (for LoadOp selection)

	// Built-in pipelines
	trianglePipeline types.RenderPipeline
	triangleShader   types.ShaderModule

	// Textured quad pipeline resources
	texQuadPipeline       types.RenderPipeline
	texQuadShader         types.ShaderModule
	texQuadUniformLayout  types.BindGroupLayout
	texQuadTextureLayout  types.BindGroupLayout
	texQuadPipelineLayout types.PipelineLayout
	texQuadUniformBuffer  types.Buffer
	texQuadUniformBindGrp types.BindGroup
	texQuadPipelineInited bool

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

// createBackend creates a backend of the specified type using the registry.
// Backends are registered via init() in their respective packages.
// Build with -tags rust to enable the Rust backend.
func createBackend(typ types.BackendType) (gpu.Backend, error) {
	switch typ {
	case types.BackendRust:
		if !gpu.IsBackendRegistered("rust") {
			return nil, fmt.Errorf("rust backend not available (build with -tags rust)")
		}
		return gpu.CreateBackend("rust"), nil

	case types.BackendNative:
		if !gpu.IsBackendRegistered("native") {
			return nil, fmt.Errorf("native backend not available")
		}
		return gpu.CreateBackend("native"), nil

	case types.BackendAuto:
		// Auto: use best available backend (rust > native)
		if b := gpu.SelectBestBackend(); b != nil {
			return b, nil
		}
		return nil, gpu.ErrNoBackendRegistered

	default:
		// Default: same as Auto
		if b := gpu.SelectBestBackend(); b != nil {
			return b, nil
		}
		return nil, gpu.ErrNoBackendRegistered
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
		PowerPreference: gputypes.PowerPreferenceHighPerformance,
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
	r.format = gputypes.TextureFormatBGRA8Unorm

	// Only configure surface if dimensions are valid.
	// If dimensions are zero (window not yet visible, minimized, or timing issue),
	// defer configuration until Resize is called with valid dimensions.
	// This matches wgpu-core behavior which returns ConfigureSurfaceError::ZeroArea.
	if width > 0 && height > 0 {
		r.width = uint32(width)   //nolint:gosec // G115: validated positive above
		r.height = uint32(height) //nolint:gosec // G115: validated positive above

		r.backend.ConfigureSurface(r.surface, r.device, &types.SurfaceConfig{
			Format:      r.format,
			Usage:       gputypes.TextureUsageRenderAttachment,
			Width:       r.width,
			Height:      r.height,
			AlphaMode:   gputypes.CompositeAlphaModeOpaque,
			PresentMode: gputypes.PresentModeFifo, // VSync
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
		Usage:       gputypes.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
		PresentMode: gputypes.PresentModeFifo,
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
	if err != nil {
		return false
	}

	// Handle different surface statuses
	switch surfTex.Status {
	case types.SurfaceStatusSuccess:
		// OK, continue
	case types.SurfaceStatusTimeout:
		// Frame not available yet - skip without reconfiguring.
		// This keeps the window responsive.
		return false
	default:
		// Surface needs reconfiguration.
		// Only attempt if we have valid dimensions.
		if r.width > 0 && r.height > 0 {
			r.backend.ConfigureSurface(r.surface, r.device, &types.SurfaceConfig{
				Format:      r.format,
				Usage:       gputypes.TextureUsageRenderAttachment,
				Width:       r.width,
				Height:      r.height,
				AlphaMode:   gputypes.CompositeAlphaModeOpaque,
				PresentMode: gputypes.PresentModeFifo,
			})
		}
		return false
	}

	r.currentTexture = surfTex.Texture

	// Create texture view for rendering
	r.currentView = r.backend.CreateTextureView(r.currentTexture, nil)

	// Reset frame state for new frame
	r.frameCleared = false

	return r.currentView != 0
}

// EndFrame presents the rendered frame.
func (r *Renderer) EndFrame() {
	// Present first while texture is still valid.
	// On Metal (macOS), releasing the texture view before present
	// can invalidate the drawable, causing blank frames.
	r.backend.Present(r.surface)

	// Reset command pool to reclaim memory from submitted command buffers.
	// This is a temporary solution that blocks on GPU completion.
	// TODO: Implement per-frame command pools for non-blocking cleanup.
	r.backend.ResetCommandPool(r.device)

	// Release resources after presentation
	if r.currentView != 0 {
		r.backend.ReleaseTextureView(r.currentView)
		r.currentView = 0
	}
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
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: red, G: green, B: blue, A: alpha},
			},
		},
	})

	r.backend.EndRenderPass(renderPass)
	r.backend.ReleaseRenderPass(renderPass)

	commands := r.backend.FinishEncoder(encoder)
	r.backend.ReleaseCommandEncoder(encoder)

	r.backend.Submit(r.queue, commands)
	r.backend.ReleaseCommandBuffer(commands)

	// Mark frame as cleared for subsequent draw calls
	r.frameCleared = true
}

// Size returns the current render target size.
func (r *Renderer) Size() (width, height int) {
	return int(r.width), int(r.height)
}

// Format returns the surface texture format.
func (r *Renderer) Format() gputypes.TextureFormat {
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
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: clearR, G: clearG, B: clearB, A: clearA},
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

// initTexturedQuadPipeline creates the GPU resources for textured quad rendering.
// This is called lazily on the first DrawTexture call.
func (r *Renderer) initTexturedQuadPipeline() error {
	if r.texQuadPipelineInited {
		return nil
	}

	var err error

	// Create shader module
	r.texQuadShader, err = r.backend.CreateShaderModuleWGSL(r.device, positionedQuadShaderSource)
	if err != nil {
		return fmt.Errorf("gogpu: failed to create textured quad shader: %w", err)
	}

	// Create bind group layout for uniforms (group 0)
	r.texQuadUniformLayout, err = r.backend.CreateBindGroupLayout(r.device, &types.BindGroupLayoutDescriptor{
		Label: "Textured Quad Uniform Layout",
		Entries: []types.BindGroupLayoutEntry{
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
	r.texQuadTextureLayout, err = r.backend.CreateBindGroupLayout(r.device, &types.BindGroupLayoutDescriptor{
		Label: "Textured Quad Texture Layout",
		Entries: []types.BindGroupLayoutEntry{
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
	r.texQuadPipelineLayout, err = r.backend.CreatePipelineLayout(r.device, &types.PipelineLayoutDescriptor{
		Label:            "Textured Quad Pipeline Layout",
		BindGroupLayouts: []types.BindGroupLayout{r.texQuadUniformLayout, r.texQuadTextureLayout},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create pipeline layout: %w", err)
	}

	// Create render pipeline with alpha blending
	r.texQuadPipeline, err = r.backend.CreateRenderPipeline(r.device, &types.RenderPipelineDescriptor{
		Label:            "Textured Quad Pipeline",
		VertexShader:     r.texQuadShader,
		VertexEntryPoint: "vs_main",
		FragmentShader:   r.texQuadShader,
		FragmentEntry:    "fs_main",
		TargetFormat:     r.format,
		Topology:         gputypes.PrimitiveTopologyTriangleList,
		CullMode:         gputypes.CullModeNone,
		Layout:           r.texQuadPipelineLayout,
		Blend: &gputypes.BlendState{
			Color: gputypes.BlendComponent{
				Operation: gputypes.BlendOperationAdd,
				SrcFactor: gputypes.BlendFactorSrcAlpha,
				DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
			},
			Alpha: gputypes.BlendComponent{
				Operation: gputypes.BlendOperationAdd,
				SrcFactor: gputypes.BlendFactorOne,
				DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create render pipeline: %w", err)
	}

	// Create uniform buffer
	r.texQuadUniformBuffer, err = r.backend.CreateBuffer(r.device, &types.BufferDescriptor{
		Label: "Textured Quad Uniforms",
		Size:  texQuadUniformSize,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create uniform buffer: %w", err)
	}

	// Create bind group for uniforms (group 0)
	r.texQuadUniformBindGrp, err = r.backend.CreateBindGroup(r.device, &types.BindGroupDescriptor{
		Label:  "Textured Quad Uniform Bind Group",
		Layout: r.texQuadUniformLayout,
		Entries: []types.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  r.texQuadUniformBuffer,
				Offset:  0,
				Size:    texQuadUniformSize,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create uniform bind group: %w", err)
	}

	r.texQuadPipelineInited = true
	return nil
}

// drawTexturedQuad draws a textured quad with the given options.
// This is an internal method called by Context.DrawTextureEx.
func (r *Renderer) drawTexturedQuad(tex *Texture, opts DrawTextureOptions) error {
	if r.currentView == 0 {
		return nil // No frame in progress
	}

	// Initialize pipeline on first use
	if !r.texQuadPipelineInited {
		if err := r.initTexturedQuadPipeline(); err != nil {
			return err
		}
	}

	// Prepare uniform data
	// Layout: rect(x,y,w,h) + screen(w,h) + alpha + pad
	uniformData := make([]byte, texQuadUniformSize)
	binary.LittleEndian.PutUint32(uniformData[0:4], math.Float32bits(opts.X))
	binary.LittleEndian.PutUint32(uniformData[4:8], math.Float32bits(opts.Y))
	binary.LittleEndian.PutUint32(uniformData[8:12], math.Float32bits(opts.Width))
	binary.LittleEndian.PutUint32(uniformData[12:16], math.Float32bits(opts.Height))
	binary.LittleEndian.PutUint32(uniformData[16:20], math.Float32bits(float32(r.width)))
	binary.LittleEndian.PutUint32(uniformData[20:24], math.Float32bits(float32(r.height)))
	binary.LittleEndian.PutUint32(uniformData[24:28], math.Float32bits(opts.Alpha))
	binary.LittleEndian.PutUint32(uniformData[28:32], 0) // padding

	// Upload uniform data
	r.backend.WriteBuffer(r.queue, r.texQuadUniformBuffer, 0, uniformData)

	// Create bind group for texture (group 1) - per-draw resource
	texBindGroup, err := r.backend.CreateBindGroup(r.device, &types.BindGroupDescriptor{
		Label:  "Textured Quad Texture Bind Group",
		Layout: r.texQuadTextureLayout,
		Entries: []types.BindGroupEntry{
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
		return fmt.Errorf("gogpu: failed to create texture bind group: %w", err)
	}
	defer r.backend.ReleaseBindGroup(texBindGroup)

	// Create command encoder
	encoder := r.backend.CreateCommandEncoder(r.device)
	if encoder == 0 {
		return fmt.Errorf("gogpu: failed to create command encoder")
	}

	// Determine LoadOp based on whether frame was already cleared
	loadOp := gputypes.LoadOpClear
	if r.frameCleared {
		loadOp = gputypes.LoadOpLoad
	}

	// Begin render pass
	renderPass := r.backend.BeginRenderPass(encoder, &types.RenderPassDescriptor{
		ColorAttachments: []types.ColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     loadOp,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 1}, // Only used if LoadOpClear
			},
		},
	})

	// Set pipeline and bind groups
	r.backend.SetPipeline(renderPass, r.texQuadPipeline)
	r.backend.SetBindGroup(renderPass, 0, r.texQuadUniformBindGrp, nil)
	r.backend.SetBindGroup(renderPass, 1, texBindGroup, nil)

	// Draw 6 vertices (2 triangles for quad)
	r.backend.Draw(renderPass, 6, 1, 0, 0)

	// End render pass
	r.backend.EndRenderPass(renderPass)
	r.backend.ReleaseRenderPass(renderPass)

	// Finish and submit
	commands := r.backend.FinishEncoder(encoder)
	r.backend.ReleaseCommandEncoder(encoder)

	r.backend.Submit(r.queue, commands)
	r.backend.ReleaseCommandBuffer(commands)

	// Mark frame as having content (for subsequent LoadOp)
	r.frameCleared = true

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

	// Release textured quad pipeline resources
	if r.texQuadUniformBindGrp != 0 {
		r.backend.ReleaseBindGroup(r.texQuadUniformBindGrp)
		r.texQuadUniformBindGrp = 0
	}
	if r.texQuadUniformBuffer != 0 {
		r.backend.ReleaseBuffer(r.texQuadUniformBuffer)
		r.texQuadUniformBuffer = 0
	}
	if r.texQuadPipelineLayout != 0 {
		r.backend.ReleasePipelineLayout(r.texQuadPipelineLayout)
		r.texQuadPipelineLayout = 0
	}
	if r.texQuadTextureLayout != 0 {
		r.backend.ReleaseBindGroupLayout(r.texQuadTextureLayout)
		r.texQuadTextureLayout = 0
	}
	if r.texQuadUniformLayout != 0 {
		r.backend.ReleaseBindGroupLayout(r.texQuadUniformLayout)
		r.texQuadUniformLayout = 0
	}
	if r.texQuadShader != 0 {
		r.backend.ReleaseShaderModule(r.texQuadShader)
		r.texQuadShader = 0
	}
	// Note: texQuadPipeline is not released separately as it's managed by backend

	// Backend handles cleanup of all resources
	if r.backend != nil {
		r.backend.Destroy()
	}
}
