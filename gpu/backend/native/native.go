// Package native provides the WebGPU backend using pure Go (gogpu/wgpu).
// This backend offers zero dependencies and simple cross-compilation.
// Currently a stub - returns ErrNotImplemented for all operations.
package native

import (
	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
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
func (b *Backend) CreateInstance() (types.Instance, error) {
	return 0, gpu.ErrNotImplemented
}

// RequestAdapter requests a GPU adapter.
func (b *Backend) RequestAdapter(instance types.Instance, opts *types.AdapterOptions) (types.Adapter, error) {
	return 0, gpu.ErrNotImplemented
}

// RequestDevice requests a GPU device.
func (b *Backend) RequestDevice(adapter types.Adapter, opts *types.DeviceOptions) (types.Device, error) {
	return 0, gpu.ErrNotImplemented
}

// GetQueue gets the device queue.
func (b *Backend) GetQueue(device types.Device) types.Queue {
	return 0
}

// CreateSurface creates a rendering surface.
func (b *Backend) CreateSurface(instance types.Instance, handle types.SurfaceHandle) (types.Surface, error) {
	return 0, gpu.ErrNotImplemented
}

// ConfigureSurface configures the surface.
func (b *Backend) ConfigureSurface(surface types.Surface, device types.Device, config *types.SurfaceConfig) {
	// Not implemented
}

// GetCurrentTexture gets the current surface texture.
func (b *Backend) GetCurrentTexture(surface types.Surface) (types.SurfaceTexture, error) {
	return types.SurfaceTexture{Status: types.SurfaceStatusError}, gpu.ErrNotImplemented
}

// Present presents the surface.
func (b *Backend) Present(surface types.Surface) {
	// Not implemented
}

// CreateShaderModuleWGSL creates a shader module from WGSL code.
func (b *Backend) CreateShaderModuleWGSL(device types.Device, code string) (types.ShaderModule, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateRenderPipeline creates a render pipeline.
func (b *Backend) CreateRenderPipeline(device types.Device, desc *types.RenderPipelineDescriptor) (types.RenderPipeline, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateCommandEncoder creates a command encoder.
func (b *Backend) CreateCommandEncoder(device types.Device) types.CommandEncoder {
	return 0
}

// BeginRenderPass begins a render pass.
func (b *Backend) BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass {
	return 0
}

// EndRenderPass ends a render pass.
func (b *Backend) EndRenderPass(pass types.RenderPass) {
	// Not implemented
}

// FinishEncoder finishes the command encoder.
func (b *Backend) FinishEncoder(encoder types.CommandEncoder) types.CommandBuffer {
	return 0
}

// Submit submits commands to the queue.
func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer) {
	// Not implemented
}

// SetPipeline sets the render pipeline.
func (b *Backend) SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline) {
	// Not implemented
}

// Draw issues a draw call.
func (b *Backend) Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	// Not implemented
}

// CreateTextureView creates a texture view.
func (b *Backend) CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView {
	return 0
}

// ReleaseTextureView releases a texture view.
func (b *Backend) ReleaseTextureView(view types.TextureView) {
	// Not implemented
}

// ReleaseTexture releases a texture.
func (b *Backend) ReleaseTexture(texture types.Texture) {
	// Not implemented
}

// ReleaseCommandBuffer releases a command buffer.
func (b *Backend) ReleaseCommandBuffer(buffer types.CommandBuffer) {
	// Not implemented
}

// ReleaseCommandEncoder releases a command encoder.
func (b *Backend) ReleaseCommandEncoder(encoder types.CommandEncoder) {
	// Not implemented
}

// ReleaseRenderPass releases a render pass.
func (b *Backend) ReleaseRenderPass(pass types.RenderPass) {
	// Not implemented
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
