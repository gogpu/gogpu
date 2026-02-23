package gogpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// texQuadUniformSize is the size of the uniform buffer for textured quads.
// Layout: rect(4 floats) + screen(2 floats) + alpha(1 float) + premultiplied(1 float) = 32 bytes
const texQuadUniformSize = 32

// Renderer manages the GPU rendering pipeline.
// It handles device initialization, surface management, and frame presentation.
type Renderer struct {
	// HAL objects (direct interfaces, no uintptr handles)
	halBackend hal.Backend
	instance   hal.Instance
	adapter    hal.Adapter
	device     hal.Device
	queue      hal.Queue
	surface    hal.Surface

	// Backend metadata
	backendName string

	// Surface configuration
	format            gputypes.TextureFormat
	width             uint32
	height            uint32
	surfaceConfigured bool // Whether surface has been configured with valid dimensions

	// Current frame state
	currentSurfaceTexture hal.SurfaceTexture
	currentView           hal.TextureView
	frameCleared          bool // Whether the frame has been cleared (for LoadOp selection)

	// Deferred clear — eliminates separate Clear render pass.
	// ClearColor stores the color and sets hasPendingClear=true.
	// The next drawTexturedQuad uses LoadOpClear with this color
	// instead of a separate render pass (avoids double RT→PRESENT→RT
	// state transition that can lose content on DX12 FLIP_DISCARD).
	pendingClearColor gputypes.Color
	hasPendingClear   bool

	// FencePool for non-blocking submission tracking (wgpu-rs pattern).
	// Each submission gets its own fence from the pool.
	// Non-blocking: poll fence status to determine completed submissions.
	fencePool         *FencePool
	nextSubmissionIdx uint64

	// Built-in pipelines
	trianglePipeline       hal.RenderPipeline
	trianglePipelineLayout hal.PipelineLayout
	triangleShader         hal.ShaderModule

	// Textured quad pipeline resources
	texQuadPipeline       hal.RenderPipeline
	texQuadShader         hal.ShaderModule
	texQuadUniformLayout  hal.BindGroupLayout
	texQuadTextureLayout  hal.BindGroupLayout
	texQuadPipelineLayout hal.PipelineLayout
	texQuadUniformBuffer  hal.Buffer
	texQuadUniformBindGrp hal.BindGroup
	texQuadUniformData    []byte // Pre-allocated buffer for uniform data (reduces GC pressure)
	texQuadPipelineInited bool

	// Texture bind group cache - avoids creating new bind groups per draw call
	texBindGroupCache map[hal.TextureView]hal.BindGroup

	// Deferred destruction queue for resources enqueued by runtime.AddCleanup.
	// These are resources that were garbage collected without explicit Close/Destroy.
	// Drained at the start of each frame (BeginFrame) when GPU is idle.
	deferredDestroys   []func()
	deferredDestroysMu sync.Mutex

	// Platform reference
	platform platform.Platform
}

// newRenderer creates and initializes a new renderer.
func newRenderer(plat platform.Platform, backendType types.BackendType, graphicsAPI types.GraphicsAPI) (*Renderer, error) {
	r := &Renderer{
		platform: plat,
	}

	if err := r.init(backendType, graphicsAPI); err != nil {
		return nil, err
	}

	return r, nil
}

// init initializes WebGPU and creates the rendering pipeline.
func (r *Renderer) init(backendType types.BackendType, graphicsAPI types.GraphicsAPI) error {
	// Select HAL backend based on user preference.
	// BackendRust requires -tags rust build and Windows.
	// BackendNative/BackendGo uses the pure Go wgpu implementation.
	// BackendAuto prefers Rust if available, otherwise falls back to native.
	//
	// graphicsAPI selects the graphics API (Vulkan/DX12/Metal).
	// For Native backend, this controls which HAL implementation is used.
	// For Rust backend, wgpu-native handles API selection internally (TODO: pass through).
	var backendVariant gputypes.Backend

	switch backendType {
	case types.BackendRust:
		if !rustHalAvailable() {
			return fmt.Errorf("gogpu: rust backend requested but not available (build with -tags rust)")
		}
		r.halBackend, r.backendName, backendVariant = newRustHalBackend()

	case types.BackendNative:
		r.halBackend, r.backendName, backendVariant = native.NewHalBackend(graphicsAPI)

	default: // BackendAuto
		if rustHalAvailable() {
			r.halBackend, r.backendName, backendVariant = newRustHalBackend()
		} else {
			r.halBackend, r.backendName, backendVariant = native.NewHalBackend(graphicsAPI)
		}
	}

	// Create WebGPU instance.
	// Enable debug/validation flags so that GPU-side errors (invalid shaders,
	// bad PSO, etc.) are caught on the CPU before submission, preventing
	// driver-level crashes (e.g. DPC_WATCHDOG_VIOLATION BSOD on DX12).
	var err error
	r.instance, err = r.halBackend.CreateInstance(&hal.InstanceDescriptor{
		Backends: gputypes.Backends(backendVariant),
		Flags:    gputypes.InstanceFlagsDebug | gputypes.InstanceFlagsValidation,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create instance: %w", err)
	}

	// Get platform handles for surface creation
	hinstance, hwnd := r.platform.GetHandle()

	// Create surface
	r.surface, err = r.instance.CreateSurface(hinstance, hwnd)
	if err != nil {
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}

	// Enumerate adapters and pick the first compatible one
	adapters := r.instance.EnumerateAdapters(r.surface)
	if len(adapters) == 0 {
		return fmt.Errorf("gogpu: no compatible GPU adapters found")
	}
	exposed := adapters[0]
	r.adapter = exposed.Adapter

	// Open device with default features and limits
	openDevice, err := r.adapter.Open(0, gputypes.DefaultLimits())
	if err != nil {
		return fmt.Errorf("gogpu: failed to open device: %w", err)
	}
	r.device = openDevice.Device
	r.queue = openDevice.Queue

	// Create fence pool for non-blocking submission tracking (wgpu-rs pattern).
	// Each submission gets its own fence, enabling true non-blocking completion checks.
	r.fencePool = NewFencePool(r.device)

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

		if err := r.surface.Configure(r.device, &hal.SurfaceConfiguration{
			Format:      r.format,
			Usage:       gputypes.TextureUsageRenderAttachment,
			Width:       r.width,
			Height:      r.height,
			AlphaMode:   gputypes.CompositeAlphaModeOpaque,
			PresentMode: gputypes.PresentModeFifo, // VSync
		}); err != nil {
			return fmt.Errorf("gogpu: failed to configure surface: %w", err)
		}
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
		// Window minimized or invisible -- unconfigure surface to prevent
		// zero-extent swapchain creation on the next frame (VK-VAL-001).
		if r.surfaceConfigured {
			r.surface.Unconfigure(r.device)
			r.surfaceConfigured = false
		}
		return
	}

	// Skip no-op resize. ConfigureSurface is expensive (involves device wait idle
	// and surface reconfiguration), so avoid calling it when dimensions are unchanged.
	if uint32(width) == r.width && uint32(height) == r.height { //nolint:gosec // G115: validated positive above
		return
	}

	// Save old dimensions in case Configure fails — we must keep
	// r.width/r.height consistent with the actual swapchain size.
	oldWidth, oldHeight := r.width, r.height

	// Note: width/height validated positive above
	r.width = uint32(width)   //nolint:gosec // G115: validated positive above
	r.height = uint32(height) //nolint:gosec // G115: validated positive above

	// Configure surface with new dimensions.
	if err := r.surface.Configure(r.device, &hal.SurfaceConfiguration{
		Format:      r.format,
		Usage:       gputypes.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
		PresentMode: gputypes.PresentModeFifo,
	}); err != nil {
		// Restore old dimensions to keep renderer consistent with swapchain.
		// Next frame will retry with the new size.
		r.width = oldWidth
		r.height = oldHeight
		return
	}
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

	// Drain deferred destruction queue at frame boundary.
	// Resources enqueued by runtime.AddCleanup are destroyed here
	// on the render thread where GPU operations are safe.
	r.DrainDeferredDestroys()

	// Acquire the next surface texture via HAL.
	// Pass nil fence — we don't need a fence for acquisition.
	acquired, err := r.surface.AcquireTexture(nil)
	if err != nil {
		// Surface needs reconfiguration (outdated or lost).
		// Only attempt if we have valid dimensions.
		if r.width > 0 && r.height > 0 {
			_ = r.surface.Configure(r.device, &hal.SurfaceConfiguration{
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

	r.currentSurfaceTexture = acquired.Texture

	// Create texture view for rendering
	view, err := r.device.CreateTextureView(r.currentSurfaceTexture, nil)
	if err != nil {
		r.surface.DiscardTexture(r.currentSurfaceTexture)
		r.currentSurfaceTexture = nil
		return false
	}
	r.currentView = view

	// Reset frame state for new frame
	r.frameCleared = false
	r.hasPendingClear = false

	return true
}

// EndFrame presents the rendered frame.
func (r *Renderer) EndFrame() {
	// Flush any pending clear that wasn't consumed by a draw call.
	// This handles the case where user calls ClearColor without drawing.
	r.flushClear()

	// Present the surface texture via queue.
	if r.currentSurfaceTexture != nil {
		// Call platform-specific pre-submit hook (Metal drawable attachment).
		// For Vulkan this is a no-op.
		// Note: We pass nil cmdBuffer here because Present doesn't need it.
		// Metal's presentDrawable is handled internally by queue.Present.
		_ = r.queue.Present(r.surface, r.currentSurfaceTexture)
	}

	// Non-blocking submission tracking: poll completed submissions.
	// This is the wgpu-rs FencePool pattern where each submission has its own fence.
	// PollCompleted checks all active fences, recycles completed fences,
	// and releases command buffers back to the pool via FreeCommandBuffer.
	// No ResetCommandPool needed — individual buffers are freed when fences signal.
	if r.fencePool != nil {
		r.fencePool.PollCompleted()
	}

	// Release resources after presentation
	if r.currentView != nil {
		r.device.DestroyTextureView(r.currentView)
		r.currentView = nil
	}
	// SurfaceTexture is consumed by Present, no need to destroy it
	r.currentSurfaceTexture = nil
}

// Clear defers a clear command to be applied at the start of the next render pass.
// This avoids a separate render pass for clearing, which on DX12 FLIP_DISCARD
// swapchains can cause content loss due to the intermediate RT→PRESENT→RT
// state transition between Clear and the subsequent draw pass.
func (r *Renderer) Clear(red, green, blue, alpha float64) {
	if r.currentView == nil {
		return
	}
	r.pendingClearColor = gputypes.Color{R: red, G: green, B: blue, A: alpha}
	r.hasPendingClear = true
}

// flushClear applies any pending clear immediately as a standalone render pass.
// Called by EndFrame if no draw calls consumed the pending clear.
func (r *Renderer) flushClear() {
	if !r.hasPendingClear || r.currentView == nil {
		return
	}

	encoder, err := r.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{
		Label: "Clear",
	})
	if err != nil {
		return
	}

	if err := encoder.BeginEncoding("Clear"); err != nil {
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		ColorAttachments: []hal.RenderPassColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: r.pendingClearColor,
			},
		},
	})

	renderPass.End()

	commands, err := encoder.EndEncoding()
	if err != nil {
		return
	}

	r.submitWithFence(commands)
	r.hasPendingClear = false
	r.frameCleared = true
}

// submitWithFence submits commands with a fence for non-blocking tracking.
// The command buffer is stored and released only when GPU finishes using it.
// This follows wgpu-rs pattern: resources must remain alive until GPU completes.
func (r *Renderer) submitWithFence(commands hal.CommandBuffer) {
	if r.fencePool == nil {
		// No fence pool - submit and release immediately (legacy behavior)
		_ = r.queue.Submit([]hal.CommandBuffer{commands}, nil, 0)
		r.device.FreeCommandBuffer(commands)
		return
	}

	// Acquire fence from pool
	fence, err := r.fencePool.AcquireFence()
	if err != nil {
		// Fence acquisition failed - submit and release immediately
		_ = r.queue.Submit([]hal.CommandBuffer{commands}, nil, 0)
		r.device.FreeCommandBuffer(commands)
		return
	}

	// Increment submission index
	r.nextSubmissionIdx++
	subIdx := r.nextSubmissionIdx

	// Submit with fence signaling
	_ = r.queue.Submit([]hal.CommandBuffer{commands}, fence, subIdx)

	// Track submission WITH command buffer for deferred release.
	// Command buffer will be released when fence signals (GPU done).
	r.fencePool.TrackSubmission(subIdx, fence, commands)
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
	return r.backendName
}

// initTrianglePipeline creates the built-in triangle render pipeline.
func (r *Renderer) initTrianglePipeline() error {
	if r.trianglePipeline != nil {
		return nil // Already initialized
	}

	var err error

	// Create shader module
	r.triangleShader, err = r.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label:  "Triangle Shader",
		Source: hal.ShaderSource{WGSL: coloredTriangleShaderSource},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create shader module: %w", err)
	}

	// Create empty pipeline layout (no bind groups needed for triangle)
	r.trianglePipelineLayout, err = r.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label: "Triangle Pipeline Layout",
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create triangle pipeline layout: %w", err)
	}

	// Create render pipeline
	r.trianglePipeline, err = r.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Triangle Pipeline",
		Layout: r.trianglePipelineLayout,
		Vertex: hal.VertexState{
			Module:     r.triangleShader,
			EntryPoint: "vs_main",
		},
		Fragment: &hal.FragmentState{
			Module:     r.triangleShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{
				{
					Format:    r.format,
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
	if r.currentView == nil {
		return nil
	}

	// Initialize pipeline on first use
	if r.trianglePipeline == nil {
		if err := r.initTrianglePipeline(); err != nil {
			return err
		}
	}

	encoder, err := r.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{
		Label: "DrawTriangle",
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create command encoder: %w", err)
	}

	if err := encoder.BeginEncoding("DrawTriangle"); err != nil {
		return fmt.Errorf("gogpu: failed to begin encoding: %w", err)
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		ColorAttachments: []hal.RenderPassColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: clearR, G: clearG, B: clearB, A: clearA},
			},
		},
	})

	renderPass.SetPipeline(r.trianglePipeline)
	renderPass.Draw(3, 1, 0, 0) // 3 vertices, 1 instance

	renderPass.End()

	commands, err := encoder.EndEncoding()
	if err != nil {
		return fmt.Errorf("gogpu: failed to finish encoding: %w", err)
	}

	// Submit with fence tracking (command buffer released when GPU done)
	r.submitWithFence(commands)

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
	r.texQuadShader, err = r.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label:  "Textured Quad Shader",
		Source: hal.ShaderSource{WGSL: positionedQuadShaderSource},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create textured quad shader: %w", err)
	}

	// Create bind group layout for uniforms (group 0)
	r.texQuadUniformLayout, err = r.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
	r.texQuadTextureLayout, err = r.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
	r.texQuadPipelineLayout, err = r.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Textured Quad Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{r.texQuadUniformLayout, r.texQuadTextureLayout},
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create pipeline layout: %w", err)
	}

	// Create render pipeline with premultiplied alpha blending.
	// The shader outputs premultiplied data for both straight and premultiplied
	// input textures (controlled by uniform flag), so the blend state is always:
	// Source-over: Src * 1 + Dst * (1 - SrcA)
	r.texQuadPipeline, err = r.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Textured Quad Pipeline",
		Layout: r.texQuadPipelineLayout,
		Vertex: hal.VertexState{
			Module:     r.texQuadShader,
			EntryPoint: "vs_main",
		},
		Primitive: gputypes.PrimitiveState{
			Topology: gputypes.PrimitiveTopologyTriangleList,
			CullMode: gputypes.CullModeNone,
		},
		Fragment: &hal.FragmentState{
			Module:     r.texQuadShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{
				{
					Format:    r.format,
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

	// Create uniform buffer on upload heap for direct CPU writes.
	// This avoids staging buffer + GPU copy per frame, reducing from
	// 4 to 3 command encoder creations during resize. Upload heap buffers
	// are CPU-writable and GPU-readable (coherent memory on DX12).
	// MappedAtCreation keeps the buffer persistently mapped for zero-overhead writes.
	r.texQuadUniformBuffer, err = r.device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Textured Quad Uniforms",
		Size:             texQuadUniformSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageMapWrite,
		MappedAtCreation: true,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create uniform buffer: %w", err)
	}

	// Create bind group for uniforms (group 0)
	r.texQuadUniformBindGrp, err = r.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Textured Quad Uniform Bind Group",
		Layout: r.texQuadUniformLayout,
		Entries: []gputypes.BindGroupEntry{
			{
				Binding: 0,
				Resource: gputypes.BufferBinding{
					Buffer: r.texQuadUniformBuffer.NativeHandle(),
					Offset: 0,
					Size:   texQuadUniformSize,
				},
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
func (r *Renderer) getOrCreateTexBindGroup(tex *Texture) (hal.BindGroup, error) {
	// Initialize cache lazily
	if r.texBindGroupCache == nil {
		r.texBindGroupCache = make(map[hal.TextureView]hal.BindGroup)
	}

	// Check cache first
	if bg, ok := r.texBindGroupCache[tex.view]; ok {
		return bg, nil
	}

	// Create new bind group for this texture
	bg, err := r.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Textured Quad Texture Bind Group",
		Layout: r.texQuadTextureLayout,
		Entries: []gputypes.BindGroupEntry{
			{
				Binding: 0,
				Resource: gputypes.SamplerBinding{
					Sampler: tex.sampler.NativeHandle(),
				},
			},
			{
				Binding: 1,
				Resource: gputypes.TextureViewBinding{
					TextureView: tex.view.NativeHandle(),
				},
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
	if r.currentView == nil {
		return nil // No frame in progress
	}

	// Ensure pipeline is initialized (lazy init on first draw)
	if !r.texQuadPipelineInited {
		if err := r.initTexturedQuadPipeline(); err != nil {
			return err
		}
	}

	// Premultiplied flag: 1.0 for premultiplied textures, 0.0 for straight alpha.
	// The shader uses this to decide whether to premultiply RGB by alpha.
	var premulFlag float32
	if tex.premultiplied {
		premulFlag = 1.0
	}

	// Get or create cached bind group for texture (group 1)
	texBindGroup, err := r.getOrCreateTexBindGroup(tex)
	if err != nil {
		return fmt.Errorf("gogpu: failed to get texture bind group: %w", err)
	}

	// Create command encoder BEFORE writing uniform data.
	// BeginEncoding calls waitForGPU which ensures all prior GPU work
	// (including the previous frame's render pass reading the uniform buffer)
	// has completed. Writing uniform data before this would race with the GPU.
	encoder, err := r.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{
		Label: "DrawTexturedQuad",
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create command encoder: %w", err)
	}

	if err := encoder.BeginEncoding("DrawTexturedQuad"); err != nil {
		return fmt.Errorf("gogpu: failed to begin encoding: %w", err)
	}

	// Upload uniform data AFTER waitForGPU (inside BeginEncoding) to avoid
	// racing with the GPU reading the uniform buffer from a previous frame.
	// For UPLOAD heap buffers, WriteBuffer is a direct CPU memcpy — safe to
	// call between BeginEncoding and BeginRenderPass.
	binary.LittleEndian.PutUint32(r.texQuadUniformData[0:4], math.Float32bits(opts.X))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[4:8], math.Float32bits(opts.Y))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[8:12], math.Float32bits(opts.Width))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[12:16], math.Float32bits(opts.Height))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[16:20], math.Float32bits(float32(r.width)))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[20:24], math.Float32bits(float32(r.height)))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[24:28], math.Float32bits(opts.Alpha))
	binary.LittleEndian.PutUint32(r.texQuadUniformData[28:32], math.Float32bits(premulFlag))
	r.queue.WriteBuffer(r.texQuadUniformBuffer, 0, r.texQuadUniformData)

	// Determine LoadOp: consume pending clear if available, otherwise preserve content.
	// This merges ClearColor + DrawTexture into a single render pass, avoiding
	// the intermediate RT→PRESENT→RT transition that loses content on DX12.
	loadOp := gputypes.LoadOpClear
	clearValue := gputypes.Color{R: 0, G: 0, B: 0, A: 1}
	if r.hasPendingClear {
		clearValue = r.pendingClearColor
		r.hasPendingClear = false
	} else if r.frameCleared {
		loadOp = gputypes.LoadOpLoad
	}

	// Begin render pass
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		ColorAttachments: []hal.RenderPassColorAttachment{
			{
				View:       r.currentView,
				LoadOp:     loadOp,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: clearValue,
			},
		},
	})

	// Set pipeline and bind groups
	renderPass.SetPipeline(r.texQuadPipeline)
	renderPass.SetBindGroup(0, r.texQuadUniformBindGrp, nil)
	renderPass.SetBindGroup(1, texBindGroup, nil)

	// Draw 6 vertices (2 triangles for quad)
	renderPass.Draw(6, 1, 0, 0)

	// End render pass
	renderPass.End()

	// Finish and submit
	commands, err := encoder.EndEncoding()
	if err != nil {
		return fmt.Errorf("gogpu: failed to finish encoding: %w", err)
	}

	// Submit with fence tracking (command buffer released when GPU done)
	r.submitWithFence(commands)

	// Mark frame as having content (for subsequent LoadOp)
	r.frameCleared = true

	return nil
}

// WaitForGPU blocks until all submitted GPU work completes.
// Call this before destroying user-created GPU resources to prevent
// Vulkan validation errors about resources still in use by command buffers.
func (r *Renderer) WaitForGPU() {
	if r.fencePool != nil {
		r.fencePool.WaitAll(time.Second)
	}
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
	// With per-frame fence tracking (SYNC-OPT), the last frame may still be
	// in-flight. WaitIdle() ensures all GPU work is done before releasing resources.
	if r.device != nil {
		_ = r.device.WaitIdle()
	}

	// FencePool.Destroy() waits for all active user fence submissions to complete.
	if r.fencePool != nil {
		r.fencePool.Destroy()
		r.fencePool = nil
	}

	if r.currentView != nil {
		r.device.DestroyTextureView(r.currentView)
		r.currentView = nil
	}
	r.currentSurfaceTexture = nil

	// Release cached texture bind groups
	for view, bg := range r.texBindGroupCache {
		r.device.DestroyBindGroup(bg)
		delete(r.texBindGroupCache, view)
	}

	// Release textured quad pipeline resources
	if r.texQuadUniformBindGrp != nil {
		r.device.DestroyBindGroup(r.texQuadUniformBindGrp)
		r.texQuadUniformBindGrp = nil
	}
	if r.texQuadUniformBuffer != nil {
		r.device.DestroyBuffer(r.texQuadUniformBuffer)
		r.texQuadUniformBuffer = nil
	}
	if r.texQuadPipelineLayout != nil {
		r.device.DestroyPipelineLayout(r.texQuadPipelineLayout)
		r.texQuadPipelineLayout = nil
	}
	if r.texQuadTextureLayout != nil {
		r.device.DestroyBindGroupLayout(r.texQuadTextureLayout)
		r.texQuadTextureLayout = nil
	}
	if r.texQuadUniformLayout != nil {
		r.device.DestroyBindGroupLayout(r.texQuadUniformLayout)
		r.texQuadUniformLayout = nil
	}
	if r.texQuadShader != nil {
		r.device.DestroyShaderModule(r.texQuadShader)
		r.texQuadShader = nil
	}
	if r.texQuadPipeline != nil {
		r.device.DestroyRenderPipeline(r.texQuadPipeline)
		r.texQuadPipeline = nil
	}
	if r.triangleShader != nil {
		r.device.DestroyShaderModule(r.triangleShader)
		r.triangleShader = nil
	}
	if r.trianglePipeline != nil {
		r.device.DestroyRenderPipeline(r.trianglePipeline)
		r.trianglePipeline = nil
	}
	if r.trianglePipelineLayout != nil {
		r.device.DestroyPipelineLayout(r.trianglePipelineLayout)
		r.trianglePipelineLayout = nil
	}

	// Destroy core resources in reverse order of creation
	if r.surface != nil {
		r.surface.Destroy()
		r.surface = nil
	}
	if r.device != nil {
		r.device.Destroy()
		r.device = nil
	}
	if r.adapter != nil {
		r.adapter.Destroy()
		r.adapter = nil
	}
	if r.instance != nil {
		r.instance.Destroy()
		r.instance = nil
	}
}
