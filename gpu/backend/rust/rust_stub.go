//go:build !windows

// Package rust provides the WebGPU backend using wgpu-native (Rust).
// This stub is used on non-Windows platforms where go-webgpu/goffi is not yet supported.
package rust

import (
	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
)

// Backend is a stub for non-Windows platforms.
type Backend struct{}

// New returns nil on non-Windows platforms.
// Use the native backend instead.
func New() *Backend {
	return nil
}

// IsAvailable returns false on non-Windows platforms.
func IsAvailable() bool {
	return false
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "Rust (not available on this platform)"
}

// Init returns an error on non-Windows platforms.
func (b *Backend) Init() error {
	return gpu.ErrBackendNotAvailable
}

// Destroy is a no-op on non-Windows platforms.
func (b *Backend) Destroy() {}

// All other methods return zero values or errors.

func (b *Backend) CreateInstance() (types.Instance, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) RequestAdapter(instance types.Instance, opts *types.AdapterOptions) (types.Adapter, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) RequestDevice(adapter types.Adapter, opts *types.DeviceOptions) (types.Device, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) GetQueue(device types.Device) types.Queue {
	return 0
}

func (b *Backend) CreateSurface(instance types.Instance, handle types.SurfaceHandle) (types.Surface, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) ConfigureSurface(surface types.Surface, device types.Device, config *types.SurfaceConfig) {
}

func (b *Backend) GetCurrentTexture(surface types.Surface) (types.SurfaceTexture, error) {
	return types.SurfaceTexture{Status: types.SurfaceStatusError}, gpu.ErrBackendNotAvailable
}

func (b *Backend) Present(surface types.Surface) {}

func (b *Backend) CreateShaderModuleWGSL(device types.Device, code string) (types.ShaderModule, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) CreateRenderPipeline(device types.Device, desc *types.RenderPipelineDescriptor) (types.RenderPipeline, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) CreateCommandEncoder(device types.Device) types.CommandEncoder {
	return 0
}

func (b *Backend) BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass {
	return 0
}

func (b *Backend) EndRenderPass(pass types.RenderPass) {}

func (b *Backend) FinishEncoder(encoder types.CommandEncoder) types.CommandBuffer {
	return 0
}

func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer) {}

func (b *Backend) SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline) {}

func (b *Backend) Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
}

func (b *Backend) CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView {
	return 0
}

func (b *Backend) ReleaseTextureView(view types.TextureView)          {}
func (b *Backend) ReleaseTexture(texture types.Texture)               {}
func (b *Backend) ReleaseCommandBuffer(buffer types.CommandBuffer)    {}
func (b *Backend) ReleaseCommandEncoder(encoder types.CommandEncoder) {}
func (b *Backend) ReleaseRenderPass(pass types.RenderPass)            {}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
