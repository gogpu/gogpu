package gogpu

import (
	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
)

// DeviceProvider provides access to GPU resources for external libraries.
// This interface enables dependency injection of GPU capabilities into
// libraries like gg without creating circular dependencies.
//
// Example usage with gg:
//
//	app := gogpu.NewApp(gogpu.Config{Title: "My App"})
//	provider := app.DeviceProvider()
//
//	// Pass to gg's GPU renderer
//	gpuRenderer := ggrender.New(provider)
//	dc := gg.NewContext(800, 600, gg.WithRenderer(gpuRenderer))
//
// This pattern follows enterprise DI best practices, similar to
// database/sql.DB or http.Client with custom Transport.
type DeviceProvider interface {
	// Backend returns the GPU backend (rust or native).
	Backend() gpu.Backend

	// Device returns the GPU device handle.
	Device() types.Device

	// Queue returns the GPU command queue.
	Queue() types.Queue

	// SurfaceFormat returns the preferred texture format for rendering.
	SurfaceFormat() types.TextureFormat
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
func (p *rendererDeviceProvider) SurfaceFormat() types.TextureFormat {
	return p.renderer.format
}

// Ensure rendererDeviceProvider implements DeviceProvider.
var _ DeviceProvider = (*rendererDeviceProvider)(nil)
