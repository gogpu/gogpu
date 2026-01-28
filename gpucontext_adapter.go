package gogpu

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// gpuContextAdapter bridges gogpu to gpucontext.DeviceProvider interface.
// This allows external libraries (like gg) to use gogpu's GPU resources
// through the standard gpucontext interface.
type gpuContextAdapter struct {
	renderer *Renderer
}

// Device returns the GPU device implementing gpucontext.Device.
func (a *gpuContextAdapter) Device() gpucontext.Device {
	if a.renderer == nil {
		return nil
	}
	return &deviceAdapter{renderer: a.renderer}
}

// Queue returns the GPU command queue implementing gpucontext.Queue.
func (a *gpuContextAdapter) Queue() gpucontext.Queue {
	if a.renderer == nil {
		return nil
	}
	return &queueAdapter{renderer: a.renderer}
}

// SurfaceFormat returns the preferred texture format for the surface.
func (a *gpuContextAdapter) SurfaceFormat() gputypes.TextureFormat {
	if a.renderer == nil {
		return gputypes.TextureFormatUndefined
	}
	// Map gogpu format to gputypes format.
	// Both use the same values (WebGPU spec), but we convert for type safety.
	return mapTextureFormat(a.renderer.format)
}

// Adapter returns the GPU adapter implementing gpucontext.Adapter.
func (a *gpuContextAdapter) Adapter() gpucontext.Adapter {
	if a.renderer == nil {
		return nil
	}
	return &adapterAdapter{renderer: a.renderer}
}

// Ensure gpuContextAdapter implements gpucontext.DeviceProvider.
var _ gpucontext.DeviceProvider = (*gpuContextAdapter)(nil)

// deviceAdapter wraps gogpu renderer to implement gpucontext.Device.
type deviceAdapter struct {
	renderer *Renderer
}

// Poll processes pending GPU operations.
func (d *deviceAdapter) Poll(wait bool) {
	// gogpu backend handles polling internally during frame submission.
	// This is a no-op for now as the renderer manages device lifecycle.
	_ = wait
}

// Destroy releases device resources.
func (d *deviceAdapter) Destroy() {
	// Device lifecycle is managed by Renderer.
	// External code should not destroy the device directly.
}

// Ensure deviceAdapter implements gpucontext.Device.
var _ gpucontext.Device = (*deviceAdapter)(nil)

// queueAdapter wraps gogpu renderer to implement gpucontext.Queue.
type queueAdapter struct {
	renderer *Renderer
}

// Ensure queueAdapter implements gpucontext.Queue.
var _ gpucontext.Queue = (*queueAdapter)(nil)

// adapterAdapter wraps gogpu renderer to implement gpucontext.Adapter.
type adapterAdapter struct {
	renderer *Renderer
}

// Ensure adapterAdapter implements gpucontext.Adapter.
var _ gpucontext.Adapter = (*adapterAdapter)(nil)

// mapTextureFormat converts gogpu TextureFormat to gputypes TextureFormat.
func mapTextureFormat(format types.TextureFormat) gputypes.TextureFormat {
	switch format {
	case types.TextureFormatRGBA8Unorm:
		return gputypes.TextureFormatRGBA8Unorm
	case types.TextureFormatBGRA8Unorm:
		return gputypes.TextureFormatBGRA8Unorm
	default:
		return gputypes.TextureFormatUndefined
	}
}

// GPUContextProvider returns a gpucontext.DeviceProvider for use with gg and other libraries.
// This enables enterprise-grade dependency injection between gogpu and external packages.
//
// Example:
//
//	app := gogpu.NewApp(gogpu.Config{Title: "My App"})
//
//	app.OnDraw(func(ctx *gogpu.Context) {
//	    // Get gpucontext provider for gg
//	    provider := app.GPUContextProvider()
//	    // ... use with gg
//	})
//
// Note: GPUContextProvider is only valid after Run() has initialized
// the renderer. Calling before Run() returns nil.
func (a *App) GPUContextProvider() gpucontext.DeviceProvider {
	if a.renderer == nil {
		return nil
	}
	return &gpuContextAdapter{renderer: a.renderer}
}
