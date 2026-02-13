package gogpu

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// DeviceProvider provides access to GPU resources for external libraries.
// This interface enables dependency injection of GPU capabilities without
// creating circular dependencies between packages.
//
// For cross-package integration (e.g., with gg), prefer using
// gpucontext.DeviceProvider via App.GPUContextProvider().
//
// Example:
//
//	app := gogpu.NewApp(gogpu.Config{Title: "My App"})
//	provider := app.DeviceProvider()
//
//	// Access GPU resources for custom rendering
//	device := provider.Device()
//	queue := provider.Queue()
//	format := provider.SurfaceFormat()
//
// This pattern follows enterprise DI best practices, similar to
// database/sql.DB or http.Client with custom Transport.
type DeviceProvider interface {
	// Device returns the HAL GPU device.
	Device() hal.Device

	// Queue returns the HAL GPU command queue.
	Queue() hal.Queue

	// SurfaceFormat returns the preferred texture format for rendering.
	SurfaceFormat() gputypes.TextureFormat
}

// rendererDeviceProvider wraps Renderer to implement DeviceProvider.
type rendererDeviceProvider struct {
	renderer *Renderer
}

// Device returns the HAL GPU device.
func (p *rendererDeviceProvider) Device() hal.Device {
	return p.renderer.device
}

// Queue returns the HAL GPU command queue.
func (p *rendererDeviceProvider) Queue() hal.Queue {
	return p.renderer.queue
}

// SurfaceFormat returns the preferred texture format.
func (p *rendererDeviceProvider) SurfaceFormat() gputypes.TextureFormat {
	return p.renderer.format
}

// Ensure rendererDeviceProvider implements DeviceProvider.
var _ DeviceProvider = (*rendererDeviceProvider)(nil)
