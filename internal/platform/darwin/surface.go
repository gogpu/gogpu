//go:build darwin

package darwin

import (
	"errors"
)

// Errors returned by Surface operations.
var (
	ErrMetalLayerCreationFailed = errors.New("darwin: failed to create CAMetalLayer")
	ErrMetalDeviceNotSet        = errors.New("darwin: Metal device not set on layer")
	ErrNoDrawableAvailable      = errors.New("darwin: no drawable available")
)

// MetalPixelFormat represents Metal pixel formats.
type MetalPixelFormat uint

// Common Metal pixel formats.
const (
	// MetalPixelFormatBGRA8UNorm is the standard BGRA 8-bit format.
	MetalPixelFormatBGRA8UNorm MetalPixelFormat = 80

	// MetalPixelFormatBGRA8UNormSRGB is BGRA 8-bit with sRGB gamma.
	MetalPixelFormatBGRA8UNormSRGB MetalPixelFormat = 81

	// MetalPixelFormatRGBA8UNorm is the standard RGBA 8-bit format.
	MetalPixelFormatRGBA8UNorm MetalPixelFormat = 70

	// MetalPixelFormatRGBA8UNormSRGB is RGBA 8-bit with sRGB gamma.
	MetalPixelFormatRGBA8UNormSRGB MetalPixelFormat = 71

	// MetalPixelFormatRGBA16Float is RGBA 16-bit float (HDR).
	MetalPixelFormatRGBA16Float MetalPixelFormat = 115
)

// MetalLayer wraps a CAMetalLayer for Metal rendering.
type MetalLayer struct {
	id ID
}

// NewMetalLayer creates a new CAMetalLayer.
func NewMetalLayer() (*MetalLayer, error) {
	initSelectors()
	initClasses()

	// Create CAMetalLayer
	layer := classes.CAMetalLayer.Send(selectors.new)
	if layer.IsNil() {
		return nil, ErrMetalLayerCreationFailed
	}

	return &MetalLayer{id: layer}, nil
}

// ID returns the underlying Objective-C object ID.
func (l *MetalLayer) ID() ID {
	if l == nil {
		return 0
	}
	return l.id
}

// Ptr returns the layer as a uintptr for FFI.
func (l *MetalLayer) Ptr() uintptr {
	if l == nil {
		return 0
	}
	return l.id.Ptr()
}

// SetDevice sets the Metal device for the layer.
// device should be an MTLDevice pointer.
func (l *MetalLayer) SetDevice(device uintptr) {
	if l == nil || l.id.IsNil() {
		return
	}

	l.id.SendPtr(selectors.setDevice, device)
}

// Device returns the Metal device used by the layer.
func (l *MetalLayer) Device() uintptr {
	if l == nil || l.id.IsNil() {
		return 0
	}

	return l.id.Send(selectors.device).Ptr()
}

// SetPixelFormat sets the pixel format for the layer.
func (l *MetalLayer) SetPixelFormat(format MetalPixelFormat) {
	if l == nil || l.id.IsNil() {
		return
	}

	l.id.SendUint(selectors.setPixelFormat, uint64(format))
}

// PixelFormat returns the current pixel format.
func (l *MetalLayer) PixelFormat() MetalPixelFormat {
	if l == nil || l.id.IsNil() {
		return 0
	}

	result := l.id.Send(selectors.pixelFormat)
	return MetalPixelFormat(result)
}

// SetDrawableSize sets the size of the layer's drawable textures.
func (l *MetalLayer) SetDrawableSize(width, height int) {
	if l == nil || l.id.IsNil() {
		return
	}

	size := NSSize{Width: CGFloat(width), Height: CGFloat(height)}
	l.id.SendSize(selectors.setDrawableSize, size)
}

// DrawableSize returns the current drawable size.
func (l *MetalLayer) DrawableSize() (width, height int) {
	if l == nil || l.id.IsNil() {
		return 0, 0
	}

	// Get size struct - this requires GetSize method which we need to add
	// For now, return 0,0 - this would need proper implementation
	return 0, 0
}

// SetFramebufferOnly sets whether textures are used only for rendering.
// Setting this to true may improve performance.
func (l *MetalLayer) SetFramebufferOnly(framebufferOnly bool) {
	if l == nil || l.id.IsNil() {
		return
	}

	l.id.SendBool(selectors.setFramebufferOnly, framebufferOnly)
}

// SetMaximumDrawableCount sets the maximum number of drawables.
// Valid values are 2 (double buffering) or 3 (triple buffering).
func (l *MetalLayer) SetMaximumDrawableCount(count int) {
	if l == nil || l.id.IsNil() {
		return
	}

	// Clamp to valid range
	if count < 2 {
		count = 2
	}
	if count > 3 {
		count = 3
	}

	l.id.SendUint(selectors.setMaximumDrawableCount, uint64(count))
}

// SetDisplaySyncEnabled enables or disables VSync.
func (l *MetalLayer) SetDisplaySyncEnabled(enabled bool) {
	if l == nil || l.id.IsNil() {
		return
	}

	l.id.SendBool(selectors.setDisplaySyncEnabled, enabled)
}

// SetContentsScale sets the scale factor for the layer.
// This should match the window's backing scale factor for Retina displays.
func (l *MetalLayer) SetContentsScale(scale float64) {
	if l == nil || l.id.IsNil() {
		return
	}

	// Send CGFloat (double) argument
	l.id.SendUint(selectors.setContentsScale, uint64(scale))
}

// NextDrawable returns the next available drawable.
// Returns a CAMetalDrawable object ID, or 0 if none available.
func (l *MetalLayer) NextDrawable() ID {
	if l == nil || l.id.IsNil() {
		return 0
	}

	return l.id.Send(selectors.nextDrawable)
}

// Release releases the layer.
func (l *MetalLayer) Release() {
	if l != nil && l.id != 0 {
		l.id.Send(selectors.release)
		l.id = 0
	}
}

// MetalDrawable wraps a CAMetalDrawable for presentation.
type MetalDrawable struct {
	id ID
}

// NewMetalDrawableFromID creates a MetalDrawable from an ID.
func NewMetalDrawableFromID(id ID) *MetalDrawable {
	if id.IsNil() {
		return nil
	}
	return &MetalDrawable{id: id}
}

// ID returns the underlying Objective-C object ID.
func (d *MetalDrawable) ID() ID {
	if d == nil {
		return 0
	}
	return d.id
}

// Ptr returns the drawable as a uintptr for FFI.
func (d *MetalDrawable) Ptr() uintptr {
	if d == nil {
		return 0
	}
	return d.id.Ptr()
}

// Texture returns the drawable's texture (MTLTexture).
// The returned uintptr is an MTLTexture pointer.
func (d *MetalDrawable) Texture() uintptr {
	if d == nil || d.id.IsNil() {
		return 0
	}

	texture := RegisterSelector("texture")
	return d.id.Send(texture).Ptr()
}

// Present presents the drawable.
// This should be called after rendering is complete.
func (d *MetalDrawable) Present() {
	if d == nil || d.id.IsNil() {
		return
	}

	present := RegisterSelector("present")
	d.id.Send(present)
}

// Surface provides a Metal rendering surface for a window.
type Surface struct {
	layer  *MetalLayer
	window *Window
}

// NewSurface creates a new Metal surface for the given window.
func NewSurface(window *Window) (*Surface, error) {
	if window == nil {
		return nil, errors.New("darwin: window is nil")
	}

	// Create Metal layer
	layer, err := NewMetalLayer()
	if err != nil {
		return nil, err
	}

	// Set default configuration
	layer.SetPixelFormat(MetalPixelFormatBGRA8UNorm)
	layer.SetFramebufferOnly(true)
	layer.SetMaximumDrawableCount(3) // Triple buffering

	// Get window size and set drawable size
	width, height := window.Size()
	layer.SetDrawableSize(width, height)

	// Attach layer to window
	window.SetMetalLayer(layer.ID())

	return &Surface{
		layer:  layer,
		window: window,
	}, nil
}

// Layer returns the underlying Metal layer.
func (s *Surface) Layer() *MetalLayer {
	return s.layer
}

// LayerPtr returns the CAMetalLayer pointer for Vulkan/Metal surface creation.
func (s *Surface) LayerPtr() uintptr {
	if s == nil || s.layer == nil {
		return 0
	}
	return s.layer.Ptr()
}

// Resize updates the surface size.
// Call this when the window is resized.
func (s *Surface) Resize(width, height int) {
	if s == nil || s.layer == nil {
		return
	}

	s.layer.SetDrawableSize(width, height)
}

// AcquireDrawable acquires the next drawable for rendering.
func (s *Surface) AcquireDrawable() (*MetalDrawable, error) {
	if s == nil || s.layer == nil {
		return nil, ErrMetalLayerCreationFailed
	}

	drawableID := s.layer.NextDrawable()
	if drawableID.IsNil() {
		return nil, ErrNoDrawableAvailable
	}

	return NewMetalDrawableFromID(drawableID), nil
}

// Destroy releases surface resources.
func (s *Surface) Destroy() {
	if s == nil {
		return
	}

	if s.layer != nil {
		s.layer.Release()
		s.layer = nil
	}

	s.window = nil
}

// ConfigureSurface applies surface configuration.
type SurfaceConfig struct {
	PixelFormat          MetalPixelFormat
	FramebufferOnly      bool
	MaximumDrawableCount int
	DisplaySync          bool
	ContentsScale        float64
}

// DefaultSurfaceConfig returns a default surface configuration.
func DefaultSurfaceConfig() SurfaceConfig {
	return SurfaceConfig{
		PixelFormat:          MetalPixelFormatBGRA8UNorm,
		FramebufferOnly:      true,
		MaximumDrawableCount: 3,
		DisplaySync:          true,
		ContentsScale:        1.0,
	}
}

// Configure applies configuration to the surface.
func (s *Surface) Configure(config SurfaceConfig) {
	if s == nil || s.layer == nil {
		return
	}

	s.layer.SetPixelFormat(config.PixelFormat)
	s.layer.SetFramebufferOnly(config.FramebufferOnly)
	s.layer.SetMaximumDrawableCount(config.MaximumDrawableCount)
	s.layer.SetDisplaySyncEnabled(config.DisplaySync)

	if config.ContentsScale > 0 {
		s.layer.SetContentsScale(config.ContentsScale)
	}
}
