package gogpu

import (
	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/math"
)

// Context provides drawing operations for a single frame.
// It is only valid during the OnDraw callback and should not be stored.
type Context struct {
	renderer *Renderer
	cleared  bool
}

// newContext creates a new drawing context for a frame.
func newContext(renderer *Renderer) *Context {
	return &Context{
		renderer: renderer,
	}
}

// Clear clears the framebuffer with the specified RGBA color.
// Values should be in the range [0.0, 1.0].
func (c *Context) Clear(r, g, b, a float32) {
	c.renderer.Clear(float64(r), float64(g), float64(b), float64(a))
	c.cleared = true
}

// ClearColor clears the framebuffer with a Color value.
func (c *Context) ClearColor(color math.Color) {
	c.Clear(color.R, color.G, color.B, color.A)
}

// Size returns the current framebuffer dimensions in pixels.
func (c *Context) Size() (width, height int) {
	return c.renderer.Size()
}

// Width returns the framebuffer width in pixels.
func (c *Context) Width() int {
	w, _ := c.renderer.Size()
	return w
}

// Height returns the framebuffer height in pixels.
func (c *Context) Height() int {
	_, h := c.renderer.Size()
	return h
}

// AspectRatio returns width/height as a float32.
func (c *Context) AspectRatio() float32 {
	w, h := c.renderer.Size()
	if h == 0 {
		return 1.0
	}
	return float32(w) / float32(h)
}

// Format returns the surface texture format.
// Useful for creating compatible pipelines.
func (c *Context) Format() gpu.TextureFormat {
	return c.renderer.Format()
}

// Backend returns the name of the active backend.
// Returns "Rust (wgpu-native)" or "Pure Go (gogpu/wgpu)".
func (c *Context) Backend() string {
	return c.renderer.Backend()
}

// DrawTriangle draws a built-in RGB-colored triangle.
// This is a convenience method for quick demos and testing.
// The background is cleared with the specified color before drawing.
func (c *Context) DrawTriangle(bgR, bgG, bgB, bgA float32) {
	c.renderer.DrawTriangle(float64(bgR), float64(bgG), float64(bgB), float64(bgA))
	c.cleared = true
}

// DrawTriangleColor draws a triangle with a background Color.
func (c *Context) DrawTriangleColor(bg math.Color) {
	c.DrawTriangle(bg.R, bg.G, bg.B, bg.A)
}
