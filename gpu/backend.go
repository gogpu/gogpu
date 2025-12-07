package gpu

import (
	"errors"

	"github.com/gogpu/gogpu/gpu/types"
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

	// Pipeline operations
	CreateRenderPipeline(device types.Device, desc *types.RenderPipelineDescriptor) (types.RenderPipeline, error)

	// Command operations
	CreateCommandEncoder(device types.Device) types.CommandEncoder
	BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass
	EndRenderPass(pass types.RenderPass)
	FinishEncoder(encoder types.CommandEncoder) types.CommandBuffer
	Submit(queue types.Queue, commands types.CommandBuffer)

	// Render pass operations
	SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline)
	Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32)

	// Texture operations
	CreateTexture(device types.Device, desc *types.TextureDescriptor) (types.Texture, error)
	CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView
	WriteTexture(queue types.Queue, dst *types.ImageCopyTexture, data []byte, layout *types.ImageDataLayout, size *types.Extent3D)

	// Sampler operations
	CreateSampler(device types.Device, desc *types.SamplerDescriptor) (types.Sampler, error)

	// Buffer operations
	CreateBuffer(device types.Device, desc *types.BufferDescriptor) (types.Buffer, error)
	WriteBuffer(queue types.Queue, buffer types.Buffer, offset uint64, data []byte)

	// Bind group operations
	CreateBindGroupLayout(device types.Device, desc *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error)
	CreateBindGroup(device types.Device, desc *types.BindGroupDescriptor) (types.BindGroup, error)
	CreatePipelineLayout(device types.Device, desc *types.PipelineLayoutDescriptor) (types.PipelineLayout, error)

	// Render pass operations (extended)
	SetBindGroup(pass types.RenderPass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32)
	SetVertexBuffer(pass types.RenderPass, slot uint32, buffer types.Buffer, offset, size uint64)
	SetIndexBuffer(pass types.RenderPass, buffer types.Buffer, format types.IndexFormat, offset, size uint64)
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
