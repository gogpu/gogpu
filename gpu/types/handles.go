package types

// Handle types - opaque references to backend-specific objects.
// These are type-safe wrappers around uintptr for safe passing between
// the Backend interface and user code.
//
// Handles are created by backend implementations and should be treated
// as opaque values. Do not attempt to dereference or modify them directly.

type (
	// Instance represents a WebGPU instance.
	// Created via Backend.CreateInstance().
	Instance uintptr

	// Adapter represents a physical GPU.
	// Created via Backend.RequestAdapter().
	Adapter uintptr

	// Device represents a logical GPU device.
	// Created via Backend.RequestDevice().
	Device uintptr

	// Queue represents a command submission queue.
	// Retrieved via Backend.GetQueue().
	Queue uintptr

	// Surface represents a platform window surface.
	// Created via Backend.CreateSurface().
	Surface uintptr

	// Texture represents a GPU texture resource.
	Texture uintptr

	// TextureView represents a view into a texture.
	// Created via Backend.CreateTextureView().
	TextureView uintptr

	// ShaderModule represents a compiled shader.
	// Created via Backend.CreateShaderModuleWGSL().
	ShaderModule uintptr

	// RenderPipeline represents a render pipeline state.
	// Created via Backend.CreateRenderPipeline().
	RenderPipeline uintptr

	// CommandEncoder records GPU commands.
	// Created via Backend.CreateCommandEncoder().
	CommandEncoder uintptr

	// CommandBuffer contains recorded GPU commands.
	// Created via Backend.FinishEncoder().
	CommandBuffer uintptr

	// RenderPass represents an active render pass.
	// Created via Backend.BeginRenderPass().
	RenderPass uintptr

	// Buffer represents a GPU buffer for vertex/index/uniform data.
	// Created via Backend.CreateBuffer().
	Buffer uintptr

	// Sampler represents a texture sampler.
	// Created via Backend.CreateSampler().
	Sampler uintptr

	// BindGroupLayout defines the structure of a bind group.
	// Created via Backend.CreateBindGroupLayout().
	BindGroupLayout uintptr

	// BindGroup represents a set of resources bound together.
	// Created via Backend.CreateBindGroup().
	BindGroup uintptr

	// PipelineLayout defines the layout of bind groups for a pipeline.
	// Created via Backend.CreatePipelineLayout().
	PipelineLayout uintptr
)

// SurfaceTexture is returned by GetCurrentTexture.
type SurfaceTexture struct {
	Texture Texture
	Status  SurfaceStatus
}

// SurfaceHandle contains platform-specific window handles.
type SurfaceHandle struct {
	// Windows: HINSTANCE and HWND
	// macOS: NSView pointer
	// Linux: Display and Window (X11)
	Instance uintptr
	Window   uintptr
}

// SurfaceStatus indicates the result of GetCurrentTexture.
type SurfaceStatus uint32

const (
	SurfaceStatusSuccess SurfaceStatus = iota
	SurfaceStatusTimeout
	SurfaceStatusOutdated
	SurfaceStatusLost
	SurfaceStatusError
)
