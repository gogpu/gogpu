package gogpu

import (
	"io"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// gpuContextAdapter bridges gogpu to gpucontext.DeviceProvider interface.
// This allows external libraries (like gg) to use gogpu's GPU resources
// through the standard gpucontext interface.
type gpuContextAdapter struct {
	renderer *Renderer
	tracker  *resourceTracker
	app      *App
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
	return mapTextureFormat(a.renderer.format)
}

// Adapter returns the GPU adapter implementing gpucontext.Adapter.
func (a *gpuContextAdapter) Adapter() gpucontext.Adapter {
	if a.renderer == nil {
		return nil
	}
	return &adapterAdapter{renderer: a.renderer}
}

// HalDevice returns the HAL device for direct GPU access.
// Implements gpucontext.HalProvider.
func (a *gpuContextAdapter) HalDevice() any {
	if a.renderer == nil || a.renderer.device == nil {
		return nil
	}
	// Access the underlying HAL device through the wgpu Device wrapper.
	return a.renderer.device.HalDevice()
}

// HalQueue returns the HAL queue for direct GPU access.
// Implements gpucontext.HalProvider.
func (a *gpuContextAdapter) HalQueue() any {
	if a.renderer == nil || a.renderer.device == nil {
		return nil
	}
	// Access the underlying HAL queue through the wgpu Device wrapper.
	return a.renderer.device.HalQueue()
}

// Size returns the current window size in logical points (DIP).
// Implements gpucontext.WindowProvider.
func (a *gpuContextAdapter) Size() (width, height int) {
	if a.app != nil {
		return a.app.Size()
	}
	return 0, 0
}

// ScaleFactor returns the DPI scale factor from the platform.
// Implements gpucontext.WindowProvider.
func (a *gpuContextAdapter) ScaleFactor() float64 {
	if a.app != nil {
		return a.app.ScaleFactor()
	}
	return 1.0
}

// RequestRedraw requests the host to render a new frame.
// Implements gpucontext.WindowProvider.
func (a *gpuContextAdapter) RequestRedraw() {
	if a.app != nil {
		a.app.RequestRedraw()
	}
}

// TrackResource registers an io.Closer for automatic cleanup during shutdown.
func (a *gpuContextAdapter) TrackResource(c io.Closer) {
	if a.tracker != nil {
		a.tracker.Track(c, "")
	}
}

// UntrackResource removes a resource from automatic cleanup tracking.
func (a *gpuContextAdapter) UntrackResource(c io.Closer) {
	if a.tracker != nil {
		a.tracker.Untrack(c)
	}
}

// Ensure gpuContextAdapter implements gpucontext.DeviceProvider.
var _ gpucontext.DeviceProvider = (*gpuContextAdapter)(nil)

// Ensure gpuContextAdapter implements gpucontext.HalProvider.
var _ gpucontext.HalProvider = (*gpuContextAdapter)(nil)

// Ensure gpuContextAdapter implements gpucontext.WindowProvider.
var _ gpucontext.WindowProvider = (*gpuContextAdapter)(nil)

// deviceAdapter wraps gogpu renderer to implement gpucontext.Device.
type deviceAdapter struct {
	renderer *Renderer
}

// Poll processes pending GPU operations.
func (d *deviceAdapter) Poll(wait bool) {
	_ = wait
}

// Destroy releases device resources.
func (d *deviceAdapter) Destroy() {
	// Device lifecycle is managed by Renderer.
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
func mapTextureFormat(format gputypes.TextureFormat) gputypes.TextureFormat {
	switch format {
	case gputypes.TextureFormatRGBA8Unorm:
		return gputypes.TextureFormatRGBA8Unorm
	case gputypes.TextureFormatBGRA8Unorm:
		return gputypes.TextureFormatBGRA8Unorm
	default:
		return gputypes.TextureFormatUndefined
	}
}

// GPUContextProvider returns a gpucontext.DeviceProvider for use with gg and other libraries.
func (a *App) GPUContextProvider() gpucontext.DeviceProvider {
	if a.renderer == nil {
		return nil
	}
	if a.tracker == nil {
		a.tracker = &resourceTracker{}
	}
	return &gpuContextAdapter{renderer: a.renderer, tracker: a.tracker, app: a}
}
