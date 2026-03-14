package gogpu

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
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

	// HalDevice returns the underlying HAL device for direct GPU access.
	// This is needed by gg's GPU accelerator which operates at the HAL level.
	HalDevice() hal.Device

	// HalQueue returns the underlying HAL queue for direct GPU access.
	HalQueue() hal.Queue

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

// HalDevice returns the underlying HAL device.
func (p *rendererDeviceProvider) HalDevice() hal.Device {
	if p.renderer.device == nil {
		return nil
	}
	return p.renderer.device.HalDevice()
}

// HalQueue returns the underlying HAL queue.
func (p *rendererDeviceProvider) HalQueue() hal.Queue {
	if p.renderer.device == nil {
		return nil
	}
	return p.renderer.device.HalQueue()
}

// SurfaceFormat returns the preferred texture format.
func (p *rendererDeviceProvider) SurfaceFormat() gputypes.TextureFormat {
	return p.renderer.format
}

// Ensure rendererDeviceProvider implements DeviceProvider.
var _ DeviceProvider = (*rendererDeviceProvider)(nil)
