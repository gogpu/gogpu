package gpu

import (
	"errors"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
)

// Common backend errors.
var (
	ErrBackendNotAvailable = errors.New("gpu: backend not available")
	ErrNotImplemented      = errors.New("gpu: not implemented")
)

// Backend is the interface that both Rust and Pure Go implementations satisfy.
// This abstraction allows users to switch backends without changing their code.
//
// The interface uses types from the gpu/types package for all WebGPU objects,
// ensuring clean separation between the interface and type definitions.
type Backend interface {
	// Name returns the backend identifier.
	Name() string

	// Init initializes the backend.
	Init() error

	// Destroy releases all backend resources.
	Destroy()

	// Instance operations
	CreateInstance() (types.Instance, error)

	// Adapter operations
	RequestAdapter(instance types.Instance, opts *types.AdapterOptions) (types.Adapter, error)

	// Device operations
	RequestDevice(adapter types.Adapter, opts *types.DeviceOptions) (types.Device, error)
	GetQueue(device types.Device) types.Queue

	// Surface operations
	CreateSurface(instance types.Instance, handle types.SurfaceHandle) (types.Surface, error)
	ConfigureSurface(surface types.Surface, device types.Device, config *types.SurfaceConfig)
	GetCurrentTexture(surface types.Surface) (types.SurfaceTexture, error)
	Present(surface types.Surface)

	// Shader operations
	CreateShaderModuleWGSL(device types.Device, code string) (types.ShaderModule, error)
	CreateShaderModuleSPIRV(device types.Device, spirv []uint32) (types.ShaderModule, error)

	// Pipeline operations
	CreateRenderPipeline(device types.Device, desc *types.RenderPipelineDescriptor) (types.RenderPipeline, error)
	CreateComputePipeline(device types.Device, desc *types.ComputePipelineDescriptor) (types.ComputePipeline, error)

	// Command operations
	CreateCommandEncoder(device types.Device) types.CommandEncoder
	BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass
	EndRenderPass(pass types.RenderPass)
	BeginComputePass(encoder types.CommandEncoder) types.ComputePass
	EndComputePass(pass types.ComputePass)
	FinishEncoder(encoder types.CommandEncoder) types.CommandBuffer

	// Submit submits commands to the queue with optional fence signaling.
	// If fence is non-zero, it will be signaled with fenceValue when commands complete.
	// Returns the submission index for tracking completion.
	Submit(queue types.Queue, commands types.CommandBuffer, fence types.Fence, fenceValue uint64) types.SubmissionIndex

	// Render pass operations
	SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline)
	Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32)

	// Compute pass operations
	SetComputePipeline(pass types.ComputePass, pipeline types.ComputePipeline)
	SetComputeBindGroup(pass types.ComputePass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32)
	DispatchWorkgroups(pass types.ComputePass, x, y, z uint32)

	// Texture operations
	CreateTexture(device types.Device, desc *types.TextureDescriptor) (types.Texture, error)
	CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView
	WriteTexture(queue types.Queue, dst *types.ImageCopyTexture, data []byte, layout *types.ImageDataLayout, size *gputypes.Extent3D)

	// Sampler operations
	CreateSampler(device types.Device, desc *types.SamplerDescriptor) (types.Sampler, error)

	// Buffer operations
	CreateBuffer(device types.Device, desc *types.BufferDescriptor) (types.Buffer, error)
	WriteBuffer(queue types.Queue, buffer types.Buffer, offset uint64, data []byte)
	MapBufferRead(buffer types.Buffer) ([]byte, error)
	UnmapBuffer(buffer types.Buffer)

	// Bind group operations
	CreateBindGroupLayout(device types.Device, desc *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error)
	CreateBindGroup(device types.Device, desc *types.BindGroupDescriptor) (types.BindGroup, error)
	CreatePipelineLayout(device types.Device, desc *types.PipelineLayoutDescriptor) (types.PipelineLayout, error)

	// Render pass operations (extended)
	SetBindGroup(pass types.RenderPass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32)
	SetVertexBuffer(pass types.RenderPass, slot uint32, buffer types.Buffer, offset, size uint64)
	SetIndexBuffer(pass types.RenderPass, buffer types.Buffer, format gputypes.IndexFormat, offset, size uint64)
	DrawIndexed(pass types.RenderPass, indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32)

	// Resource release
	ReleaseTexture(texture types.Texture)
	ReleaseTextureView(view types.TextureView)
	ReleaseSampler(sampler types.Sampler)
	ReleaseBuffer(buffer types.Buffer)
	ReleaseBindGroupLayout(layout types.BindGroupLayout)
	ReleaseBindGroup(group types.BindGroup)
	ReleasePipelineLayout(layout types.PipelineLayout)
	ReleaseCommandBuffer(buffer types.CommandBuffer)
	ReleaseCommandEncoder(encoder types.CommandEncoder)
	ReleaseRenderPass(pass types.RenderPass)
	ReleaseComputePipeline(pipeline types.ComputePipeline)
	ReleaseComputePass(pass types.ComputePass)
	ReleaseShaderModule(module types.ShaderModule)

	// Maintenance operations
	// ResetCommandPool resets the command pool to reclaim command buffer memory.
	// This should be called periodically (e.g., after Present) when GPU is idle.
	// Note: This is a temporary solution; proper fix requires per-frame command pools.
	ResetCommandPool(device types.Device)

	// Fence operations for GPU synchronization

	// CreateFence creates a new fence in the unsignaled state.
	CreateFence(device types.Device) (types.Fence, error)

	// GetFenceStatus returns true if the fence is signaled (non-blocking).
	// Use this for polling completion without blocking.
	GetFenceStatus(fence types.Fence) (bool, error)

	// WaitFence waits for a fence to be signaled.
	// Returns true if signaled, false if timed out.
	// timeout is in nanoseconds. Use 0 for non-blocking check.
	WaitFence(device types.Device, fence types.Fence, timeout uint64) (bool, error)

	// ResetFence resets a fence to the unsignaled state.
	// The fence must not be in use by the GPU.
	ResetFence(device types.Device, fence types.Fence) error

	// DestroyFence destroys a fence.
	DestroyFence(device types.Device, fence types.Fence)
}

// activeBackend is the currently selected backend.
var activeBackend Backend

// SetBackend sets the active backend.
func SetBackend(b Backend) {
	activeBackend = b
}

// GetBackend returns the active backend.
func GetBackend() Backend {
	return activeBackend
}
