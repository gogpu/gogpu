package gogpu

import (
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Context provides drawing operations for a single frame.
// It is only valid during the OnDraw callback and should not be stored.
type Context struct {
	renderer    *Renderer
	scaleFactor float64 // DPI scale factor (1.0 = standard, 2.0 = Retina/HiDPI)
	cleared     bool
}

// newContext creates a new drawing context for a frame.
func newContext(renderer *Renderer, scaleFactor float64) *Context {
	if scaleFactor <= 0 {
		scaleFactor = 1.0
	}
	return &Context{
		renderer:    renderer,
		scaleFactor: scaleFactor,
	}
}

// Clear clears the framebuffer with the specified RGBA color.
// Values should be in the range [0.0, 1.0].
func (c *Context) Clear(r, g, b, a float32) {
	c.renderer.Clear(float64(r), float64(g), float64(b), float64(a))
	c.cleared = true
}

// ClearColor clears the framebuffer with a Color value.
func (c *Context) ClearColor(color gmath.Color) {
	c.Clear(color.R, color.G, color.B, color.A)
}

// Size returns the window dimensions in logical points (DIP).
// Use this for layout, UI coordinates, and user-facing dimensions.
// On Retina/HiDPI displays, this is smaller than FramebufferSize by ScaleFactor.
func (c *Context) Size() (width, height int) {
	pw, ph := c.renderer.Size()
	return int(float64(pw) / c.scaleFactor), int(float64(ph) / c.scaleFactor)
}

// Width returns the window width in logical points (DIP).
func (c *Context) Width() int {
	w, _ := c.Size()
	return w
}

// Height returns the window height in logical points (DIP).
func (c *Context) Height() int {
	_, h := c.Size()
	return h
}

// FramebufferSize returns the GPU framebuffer dimensions in physical device pixels.
// Use this for GPU operations, texture allocation, and pixel-precise rendering.
func (c *Context) FramebufferSize() (width, height int) {
	return c.renderer.Size()
}

// FramebufferWidth returns the GPU framebuffer width in physical device pixels.
func (c *Context) FramebufferWidth() int {
	w, _ := c.renderer.Size()
	return w
}

// FramebufferHeight returns the GPU framebuffer height in physical device pixels.
func (c *Context) FramebufferHeight() int {
	_, h := c.renderer.Size()
	return h
}

// ScaleFactor returns the DPI scale factor.
// 1.0 = standard (96 DPI on Windows), 2.0 = Retina/HiDPI.
func (c *Context) ScaleFactor() float64 {
	return c.scaleFactor
}

// AspectRatio returns width/height as a float32 (based on logical size).
func (c *Context) AspectRatio() float32 {
	w, h := c.Size()
	if h == 0 {
		return 1.0
	}
	return float32(w) / float32(h)
}

// Format returns the surface texture format.
// Useful for creating compatible pipelines.
func (c *Context) Format() gputypes.TextureFormat {
	return c.renderer.Format()
}

// Backend returns the name of the active backend.
// Returns "Rust (wgpu-gpu)" or "Pure Go (gogpu/wgpu)".
func (c *Context) Backend() string {
	return c.renderer.Backend()
}

// DrawTriangle draws a built-in RGB-colored triangle.
// This is a convenience method for quick demos and testing.
// The background is cleared with the specified color before drawing.
func (c *Context) DrawTriangle(bgR, bgG, bgB, bgA float32) error {
	err := c.renderer.DrawTriangle(float64(bgR), float64(bgG), float64(bgB), float64(bgA))

	c.cleared = true
	return err
}

// DrawTriangleColor draws a triangle with a background Color.
func (c *Context) DrawTriangleColor(bg gmath.Color) error {
	err := c.DrawTriangle(bg.R, bg.G, bg.B, bg.A)
	return err
}

// Renderer returns the underlying Renderer for texture creation.
// This allows creating textures from within the OnDraw callback.
// Note: Textures should be created once and reused, not every frame.
func (c *Context) Renderer() *Renderer {
	return c.renderer
}

// SurfaceView returns the current frame's surface texture view.
// This is the GPU texture view that will be presented to the screen.
// Returns nil if no frame is in progress.
//
// Use this with ggcanvas.RenderDirect for zero-copy GPU rendering,
// bypassing the GPU→CPU→GPU readback path.
func (c *Context) SurfaceView() *wgpu.TextureView {
	return c.renderer.currentView
}

// CheckDeviceHealth returns nil if the GPU device is operational, or an error
// describing why the device was removed. This is a diagnostic method for
// debugging DX12 DEVICE_REMOVED issues.
func (c *Context) CheckDeviceHealth() error {
	type healthChecker interface {
		CheckHealth(label string) error
	}
	// Check the underlying HAL device for health (e.g., DX12 DEVICE_REMOVED).
	if c.renderer.device == nil {
		return nil
	}
	halDev := c.renderer.device.HalDevice()
	if hc, ok := halDev.(healthChecker); ok {
		return hc.CheckHealth("Context.CheckDeviceHealth")
	}
	return nil // Backend doesn't support health check
}

// SurfaceSize returns the current GPU surface dimensions in physical device pixels.
// This is the same as FramebufferSize but returns uint32 for GPU API compatibility.
func (c *Context) SurfaceSize() (width, height uint32) {
	w, h := c.renderer.Size()
	return uint32(w), uint32(h) //nolint:gosec // G115: renderer validates dimensions
}
