package gogpu

import (
	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
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
	// Backend returns the GPU backend (rust or gpu).
	Backend() gpu.Backend

	// Device returns the GPU device handle.
	Device() types.Device

	// Queue returns the GPU command queue.
	Queue() types.Queue

	// SurfaceFormat returns the preferred texture format for rendering.
	SurfaceFormat() gputypes.TextureFormat
}

// rendererDeviceProvider wraps Renderer to implement DeviceProvider.
type rendererDeviceProvider struct {
	renderer *Renderer
}

// Backend returns the GPU backend.
func (p *rendererDeviceProvider) Backend() gpu.Backend {
	return p.renderer.backend
}

// Device returns the GPU device handle.
func (p *rendererDeviceProvider) Device() types.Device {
	return p.renderer.device
}

// Queue returns the GPU command queue.
func (p *rendererDeviceProvider) Queue() types.Queue {
	return p.renderer.queue
}

// SurfaceFormat returns the preferred texture format.
func (p *rendererDeviceProvider) SurfaceFormat() gputypes.TextureFormat {
	return p.renderer.format
}

// Ensure rendererDeviceProvider implements DeviceProvider.
var _ DeviceProvider = (*rendererDeviceProvider)(nil)
