package gogpu

import (
	"io"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// gpuContextAdapter bridges gogpu to gpucontext.DeviceProvider interface.
// This allows external libraries (like gg) to use gogpu's GPU resources
// through the standard gpucontext interface.
//
// Device() and Queue() return the actual *wgpu.Device and *wgpu.Queue
// wrapped as gpucontext.Device and gpucontext.Queue. Consumers type-assert
// to the concrete wgpu types when they need the full API:
//
//	dev := provider.Device().(*wgpu.Device)
//	halDevice := dev.HalDevice()
type gpuContextAdapter struct {
	renderer *Renderer
	tracker  *resourceTracker
	app      *App
}

// Device returns the underlying *wgpu.Device as gpucontext.Device.
// Consumers should type-assert to *wgpu.Device for full API access.
func (a *gpuContextAdapter) Device() gpucontext.Device {
	if a.renderer == nil || a.renderer.device == nil {
		return nil
	}
	return a.renderer.device
}

// Queue returns the underlying *wgpu.Queue as gpucontext.Queue.
// Consumers should type-assert to *wgpu.Queue for full API access.
func (a *gpuContextAdapter) Queue() gpucontext.Queue {
	if a.renderer == nil || a.renderer.device == nil {
		return nil
	}
	return a.renderer.device.Queue()
}

// SurfaceFormat returns the preferred texture format for the surface.
func (a *gpuContextAdapter) SurfaceFormat() gputypes.TextureFormat {
	if a.renderer == nil {
		return gputypes.TextureFormatUndefined
	}
	return mapTextureFormat(a.renderer.format)
}

// Adapter returns the GPU adapter as gpucontext.Adapter.
// Consumers should type-assert to *wgpu.Adapter for full API access.
func (a *gpuContextAdapter) Adapter() gpucontext.Adapter {
	if a.renderer == nil || a.renderer.adapter == nil {
		return nil
	}
	return a.renderer.adapter
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

// Ensure gpuContextAdapter implements gpucontext.WindowProvider.
var _ gpucontext.WindowProvider = (*gpuContextAdapter)(nil)

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
