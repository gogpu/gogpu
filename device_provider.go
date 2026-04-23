package gogpu

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// DeviceProvider provides access to GPU resources for external libraries.
// This interface enables dependency injection of GPU capabilities without
// creating circular dependencies between packages.
//
// For cross-package integration (e.g., with gg), prefer using
// gpucontext.DeviceProvider via App.GPUContextProvider().
type DeviceProvider interface {
	// Device returns the wgpu GPU device.
	Device() *wgpu.Device

	// Queue returns the wgpu GPU command queue.
	Queue() *wgpu.Queue

	// SurfaceFormat returns the preferred texture format for rendering.
	SurfaceFormat() gputypes.TextureFormat
}

// rendererDeviceProvider wraps Renderer to implement DeviceProvider.
type rendererDeviceProvider struct {
	renderer *Renderer
}

// Device returns the wgpu GPU device.
func (p *rendererDeviceProvider) Device() *wgpu.Device {
	return p.renderer.device
}

// Queue returns the wgpu GPU command queue.
func (p *rendererDeviceProvider) Queue() *wgpu.Queue {
	if p.renderer.device == nil {
		return nil
	}
	return p.renderer.device.Queue()
}

// SurfaceFormat returns the preferred texture format.
func (p *rendererDeviceProvider) SurfaceFormat() gputypes.TextureFormat {
	return p.renderer.primary.format
}

// Ensure rendererDeviceProvider implements DeviceProvider.
var _ DeviceProvider = (*rendererDeviceProvider)(nil)
