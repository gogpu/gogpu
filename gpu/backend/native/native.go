//go:build !windows && !linux && !darwin

// Package native provides the WebGPU backend using pure Go (gogpu/wgpu).
// This backend offers zero dependencies and simple cross-compilation.
// Currently a stub - returns ErrNotImplemented for all operations.
// TODO: Implement OpenGL/Metal backends for Linux/macOS.
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

// Submit submits commands to the queue with optional fence signaling.
// If fence is non-zero, it will be signaled with fenceValue when commands complete.
// Returns the submission index for tracking completion.
func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer, fence types.Fence, fenceValue uint64) types.SubmissionIndex {
	// Not implemented
	return 0
}

// GetFenceStatus returns true if the fence is signaled (non-blocking).
func (b *Backend) GetFenceStatus(fence types.Fence) (bool, error) {
	return true, nil // Always signaled for stub
}

// SetPipeline sets the render pipeline.
func (b *Backend) SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline) {
	// Not implemented
}

// Draw issues a draw call.
func (b *Backend) Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	// Not implemented
}

// CreateTexture creates a GPU texture.
func (b *Backend) CreateTexture(device types.Device, desc *types.TextureDescriptor) (types.Texture, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateTextureView creates a texture view.
func (b *Backend) CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView {
	return 0
}

// WriteTexture writes data to a texture.
func (b *Backend) WriteTexture(queue types.Queue, dst *types.ImageCopyTexture, data []byte, layout *types.ImageDataLayout, size *gputypes.Extent3D) {
	// Not implemented
}

// CreateSampler creates a texture sampler.
func (b *Backend) CreateSampler(device types.Device, desc *types.SamplerDescriptor) (types.Sampler, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateBuffer creates a GPU buffer.
func (b *Backend) CreateBuffer(device types.Device, desc *types.BufferDescriptor) (types.Buffer, error) {
	return 0, gpu.ErrNotImplemented
}

// WriteBuffer writes data to a buffer.
func (b *Backend) WriteBuffer(queue types.Queue, buffer types.Buffer, offset uint64, data []byte) {
	// Not implemented
}

// CreateBindGroupLayout creates a bind group layout.
func (b *Backend) CreateBindGroupLayout(device types.Device, desc *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateBindGroup creates a bind group.
func (b *Backend) CreateBindGroup(device types.Device, desc *types.BindGroupDescriptor) (types.BindGroup, error) {
	return 0, gpu.ErrNotImplemented
}

// CreatePipelineLayout creates a pipeline layout.
func (b *Backend) CreatePipelineLayout(device types.Device, desc *types.PipelineLayoutDescriptor) (types.PipelineLayout, error) {
	return 0, gpu.ErrNotImplemented
}

// SetBindGroup sets a bind group for a render pass.
func (b *Backend) SetBindGroup(pass types.RenderPass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32) {
	// Not implemented
}

// SetVertexBuffer sets a vertex buffer for a render pass.
func (b *Backend) SetVertexBuffer(pass types.RenderPass, slot uint32, buffer types.Buffer, offset, size uint64) {
	// Not implemented
}

// SetIndexBuffer sets an index buffer for a render pass.
func (b *Backend) SetIndexBuffer(pass types.RenderPass, buffer types.Buffer, format gputypes.IndexFormat, offset, size uint64) {
	// Not implemented
}

// DrawIndexed issues an indexed draw call.
func (b *Backend) DrawIndexed(pass types.RenderPass, indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	// Not implemented
}

// --- Compute shader operations ---

// CreateShaderModuleSPIRV creates a shader module from SPIR-V bytecode.
func (b *Backend) CreateShaderModuleSPIRV(device types.Device, spirv []uint32) (types.ShaderModule, error) {
	return 0, gpu.ErrNotImplemented
}

// CreateComputePipeline creates a compute pipeline.
func (b *Backend) CreateComputePipeline(device types.Device, desc *types.ComputePipelineDescriptor) (types.ComputePipeline, error) {
	return 0, gpu.ErrNotImplemented
}

// BeginComputePass begins a compute pass.
func (b *Backend) BeginComputePass(encoder types.CommandEncoder) types.ComputePass {
	return 0
}

// EndComputePass ends a compute pass.
func (b *Backend) EndComputePass(pass types.ComputePass) {
	// Not implemented
}

// SetComputePipeline sets the compute pipeline for a compute pass.
func (b *Backend) SetComputePipeline(pass types.ComputePass, pipeline types.ComputePipeline) {
	// Not implemented
}

// SetComputeBindGroup sets a bind group for a compute pass.
func (b *Backend) SetComputeBindGroup(pass types.ComputePass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32) {
	// Not implemented
}

// DispatchWorkgroups dispatches compute work.
func (b *Backend) DispatchWorkgroups(pass types.ComputePass, x, y, z uint32) {
	// Not implemented
}

// MapBufferRead maps a buffer for reading and returns its contents.
func (b *Backend) MapBufferRead(buffer types.Buffer) ([]byte, error) {
	return nil, gpu.ErrNotImplemented
}

// UnmapBuffer unmaps a previously mapped buffer.
func (b *Backend) UnmapBuffer(buffer types.Buffer) {
	// Not implemented
}

// ReleaseTexture releases a texture.
func (b *Backend) ReleaseTexture(texture types.Texture) {
	// Not implemented
}

// ReleaseTextureView releases a texture view.
func (b *Backend) ReleaseTextureView(view types.TextureView) {
	// Not implemented
}

// ReleaseSampler releases a sampler.
func (b *Backend) ReleaseSampler(sampler types.Sampler) {
	// Not implemented
}

// ReleaseBuffer releases a buffer.
func (b *Backend) ReleaseBuffer(buffer types.Buffer) {
	// Not implemented
}

// ReleaseBindGroupLayout releases a bind group layout.
func (b *Backend) ReleaseBindGroupLayout(layout types.BindGroupLayout) {
	// Not implemented
}

// ReleaseBindGroup releases a bind group.
func (b *Backend) ReleaseBindGroup(group types.BindGroup) {
	// Not implemented
}

// ReleasePipelineLayout releases a pipeline layout.
func (b *Backend) ReleasePipelineLayout(layout types.PipelineLayout) {
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

// ReleaseComputePipeline releases a compute pipeline.
func (b *Backend) ReleaseComputePipeline(pipeline types.ComputePipeline) {
	// Not implemented
}

// ReleaseComputePass releases a compute pass.
func (b *Backend) ReleaseComputePass(pass types.ComputePass) {
	// Not implemented
}

// ReleaseShaderModule releases a shader module.
func (b *Backend) ReleaseShaderModule(module types.ShaderModule) {
	// Not implemented
}

// ResetCommandPool resets the command pool to reclaim command buffer memory.
func (b *Backend) ResetCommandPool(device types.Device) {
	// Not implemented - platform-specific backends override this
}

// CreateFence creates a new fence in the unsignaled state.
func (b *Backend) CreateFence(device types.Device) (types.Fence, error) {
	return 0, gpu.ErrNotImplemented
}

// WaitFence waits for a fence to be signaled.
func (b *Backend) WaitFence(device types.Device, fence types.Fence, timeout uint64) (bool, error) {
	return true, nil // Always "signaled" for stub
}

// ResetFence resets a fence to the unsignaled state.
func (b *Backend) ResetFence(device types.Device, fence types.Fence) error {
	return nil
}

// DestroyFence destroys a fence.
func (b *Backend) DestroyFence(device types.Device, fence types.Fence) {
	// Not implemented
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
