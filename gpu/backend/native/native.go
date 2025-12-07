// Package native provides the WebGPU backend using pure Go (gogpu/wgpu).
// This backend offers zero dependencies and simple cross-compilation.
// Currently a stub - returns ErrNotImplemented for all operations.
package native

import (
	"github.com/gogpu/gogpu/gpu"
)

// Backend implements gpu.Backend using pure Go.
type Backend struct{}

// New creates a new Pure Go backend.
func New() *Backend {
	return &Backend{}
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "Pure Go (gogpu/wgpu)"
}

// Init initializes the backend.
func (b *Backend) Init() error {
	return gpu.ErrNotImplemented
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	// Nothing to destroy yet
}

// CreateInstance creates a WebGPU instance.
func (b *Backend) CreateInstance() (gpu.Instance, error) {
	return 0, gpu.ErrNotImplemented
}

// RequestAdapter requests a GPU adapter.
func (b *Backend) RequestAdapter(instance gpu.Instance, opts *gpu.AdapterOptions) (gpu.Adapter, error) {
	return 0, gpu.ErrNotImplemented
}

// RequestDevice requests a GPU device.
func (b *Backend) RequestDevice(adapter gpu.Adapter, opts *gpu.DeviceOptions) (gpu.Device, error) {
	return 0, gpu.ErrNotImplemented
}

// GetQueue gets the device queue.
func (b *Backend) GetQueue(device gpu.Device) gpu.Queue {
	return 0
}

// CreateSurface creates a rendering surface.
func (b *Backend) CreateSurface(instance gpu.Instance, handle gpu.SurfaceHandle) (gpu.Surface, error) {
	return 0, gpu.ErrNotImplemented
}

// ConfigureSurface configures the surface.
func (b *Backend) ConfigureSurface(surface gpu.Surface, device gpu.Device, config *gpu.SurfaceConfig) {
	// Not implemented
}

// GetCurrentTexture gets the current surface texture.
func (b *Backend) GetCurrentTexture(surface gpu.Surface) (gpu.SurfaceTexture, error) {
	return gpu.SurfaceTexture{Status: gpu.SurfaceStatusError}, gpu.ErrNotImplemented
}

// Present presents the surface.
func (b *Backend) Present(surface gpu.Surface) {
	// Not implemented
}

// CreateShaderModuleWGSL creates a shader module from WGSL code.
func (b *Backend) CreateShaderModuleWGSL(device gpu.Device, code string) (gpu.ShaderModule, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateRenderPipeline creates a render pipeline.
func (b *Backend) CreateRenderPipeline(device gpu.Device, desc *gpu.RenderPipelineDescriptor) (gpu.RenderPipeline, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateCommandEncoder creates a command encoder.
func (b *Backend) CreateCommandEncoder(device gpu.Device) gpu.CommandEncoder {
	return 0
}

// BeginRenderPass begins a render pass.
func (b *Backend) BeginRenderPass(encoder gpu.CommandEncoder, desc *gpu.RenderPassDescriptor) gpu.RenderPass {
	return 0
}

// EndRenderPass ends a render pass.
func (b *Backend) EndRenderPass(pass gpu.RenderPass) {
	// Not implemented
}

// FinishEncoder finishes the command encoder.
func (b *Backend) FinishEncoder(encoder gpu.CommandEncoder) gpu.CommandBuffer {
	return 0
}

// Submit submits commands to the queue.
func (b *Backend) Submit(queue gpu.Queue, commands gpu.CommandBuffer) {
	// Not implemented
}

// SetPipeline sets the render pipeline.
func (b *Backend) SetPipeline(pass gpu.RenderPass, pipeline gpu.RenderPipeline) {
	// Not implemented
}

// Draw issues a draw call.
func (b *Backend) Draw(pass gpu.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	// Not implemented
}

// CreateTextureView creates a texture view.
func (b *Backend) CreateTextureView(texture gpu.Texture, desc *gpu.TextureViewDescriptor) gpu.TextureView {
	return 0
}

// ReleaseTextureView releases a texture view.
func (b *Backend) ReleaseTextureView(view gpu.TextureView) {
	// Not implemented
}

// ReleaseTexture releases a texture.
func (b *Backend) ReleaseTexture(texture gpu.Texture) {
	// Not implemented
}

// ReleaseCommandBuffer releases a command buffer.
func (b *Backend) ReleaseCommandBuffer(buffer gpu.CommandBuffer) {
	// Not implemented
}

// ReleaseCommandEncoder releases a command encoder.
func (b *Backend) ReleaseCommandEncoder(encoder gpu.CommandEncoder) {
	// Not implemented
}

// ReleaseRenderPass releases a render pass.
func (b *Backend) ReleaseRenderPass(pass gpu.RenderPass) {
	// Not implemented
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
