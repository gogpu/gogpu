//go:build !windows

// Package rust provides the WebGPU backend using wgpu-native (Rust).
// This stub is used on non-Windows platforms where go-webgpu/goffi is not yet supported.
package rust

import (
	"github.com/gogpu/gogpu/gpu"
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

func (b *Backend) CreateInstance() (gpu.Instance, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) RequestAdapter(instance gpu.Instance, opts *gpu.AdapterOptions) (gpu.Adapter, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) RequestDevice(adapter gpu.Adapter, opts *gpu.DeviceOptions) (gpu.Device, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) GetQueue(device gpu.Device) gpu.Queue {
	return 0
}

func (b *Backend) CreateSurface(instance gpu.Instance, handle gpu.SurfaceHandle) (gpu.Surface, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) ConfigureSurface(surface gpu.Surface, device gpu.Device, config *gpu.SurfaceConfig) {
}

func (b *Backend) GetCurrentTexture(surface gpu.Surface) (gpu.SurfaceTexture, error) {
	return gpu.SurfaceTexture{Status: gpu.SurfaceStatusError}, gpu.ErrBackendNotAvailable
}

func (b *Backend) Present(surface gpu.Surface) {}

func (b *Backend) CreateShaderModuleWGSL(device gpu.Device, code string) (gpu.ShaderModule, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) CreateRenderPipeline(device gpu.Device, desc *gpu.RenderPipelineDescriptor) (gpu.RenderPipeline, error) {
	return 0, gpu.ErrBackendNotAvailable
}

func (b *Backend) CreateCommandEncoder(device gpu.Device) gpu.CommandEncoder {
	return 0
}

func (b *Backend) BeginRenderPass(encoder gpu.CommandEncoder, desc *gpu.RenderPassDescriptor) gpu.RenderPass {
	return 0
}

func (b *Backend) EndRenderPass(pass gpu.RenderPass) {}

func (b *Backend) FinishEncoder(encoder gpu.CommandEncoder) gpu.CommandBuffer {
	return 0
}

func (b *Backend) Submit(queue gpu.Queue, commands gpu.CommandBuffer) {}

func (b *Backend) SetPipeline(pass gpu.RenderPass, pipeline gpu.RenderPipeline) {}

func (b *Backend) Draw(pass gpu.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
}

func (b *Backend) CreateTextureView(texture gpu.Texture, desc *gpu.TextureViewDescriptor) gpu.TextureView {
	return 0
}

func (b *Backend) ReleaseTextureView(view gpu.TextureView)       {}
func (b *Backend) ReleaseTexture(texture gpu.Texture)            {}
func (b *Backend) ReleaseCommandBuffer(buffer gpu.CommandBuffer) {}
func (b *Backend) ReleaseCommandEncoder(encoder gpu.CommandEncoder) {}
func (b *Backend) ReleaseRenderPass(pass gpu.RenderPass)         {}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
