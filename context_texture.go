package gogpu

import (
	"errors"

	"github.com/gogpu/gpucontext"
)

// DrawTextureOptions configures texture drawing with full transform control.
type DrawTextureOptions struct {
	// X is the horizontal position of the texture's top-left corner in pixels.
	X float32

	// Y is the vertical position of the texture's top-left corner in pixels.
	Y float32

	// Width is the target width in pixels.
	// If 0, the texture's original width is used.
	Width float32

	// Height is the target height in pixels.
	// If 0, the texture's original height is used.
	Height float32

	// Alpha is the opacity value in the range [0.0, 1.0].
	// 0.0 is fully transparent, 1.0 is fully opaque.
	// If 0, defaults to 1.0 (fully opaque).
	Alpha float32
}

// ErrTextureNil is returned when attempting to draw a nil texture.
var ErrTextureNil = errors.New("gogpu: texture is nil")

// ErrTextureDestroyed is returned when attempting to draw a destroyed texture.
var ErrTextureDestroyed = errors.New("gogpu: texture has been destroyed")

// ErrInvalidDimensions is returned when texture dimensions are invalid.
var ErrInvalidDimensions = errors.New("gogpu: invalid texture dimensions")

// ErrNotImplemented is returned for features that are not yet implemented.
var ErrNotImplemented = errors.New("gogpu: feature not implemented")

// DrawTexture draws a texture at the specified position.
// The texture is drawn at its original size with the top-left corner
// positioned at (x, y).
//
// Coordinate system:
//   - (0, 0) is the top-left corner of the framebuffer
//   - Positive X goes right
//   - Positive Y goes down
//   - Coordinates are in pixels
//
// Returns an error if the texture is nil, has been destroyed,
// or if the rendering pipeline is not available.
func (c *Context) DrawTexture(tex *Texture, x, y float32) error {
	if err := validateTexture(tex); err != nil {
		return err
	}

	return c.DrawTextureEx(tex, DrawTextureOptions{
		X:     x,
		Y:     y,
		Alpha: 1.0,
	})
}

// DrawTextureScaled draws a texture scaled to fit the given rectangle.
// The texture is stretched or compressed to exactly fit the specified
// width and height.
//
// Coordinate system:
//   - (0, 0) is the top-left corner of the framebuffer
//   - Positive X goes right
//   - Positive Y goes down
//   - Coordinates are in pixels
//
// Returns an error if:
//   - The texture is nil
//   - The texture has been destroyed
//   - Width or height is negative
//   - The rendering pipeline is not available
func (c *Context) DrawTextureScaled(tex *Texture, x, y, w, h float32) error {
	if err := validateTexture(tex); err != nil {
		return err
	}

	if w < 0 || h < 0 {
		return ErrInvalidDimensions
	}

	return c.DrawTextureEx(tex, DrawTextureOptions{
		X:      x,
		Y:      y,
		Width:  w,
		Height: h,
		Alpha:  1.0,
	})
}

// DrawTextureEx draws a texture with full control over positioning and appearance.
// This is the most flexible texture drawing method, allowing control over
// position, size, and opacity.
//
// If Width is 0, the texture's original width is used.
// If Height is 0, the texture's original height is used.
// If Alpha is 0, it defaults to 1.0 (fully opaque).
//
// Coordinate system:
//   - (0, 0) is the top-left corner of the framebuffer
//   - Positive X goes right
//   - Positive Y goes down
//   - Coordinates are in pixels
//
// Returns an error if:
//   - The texture is nil
//   - The texture has been destroyed
//   - Width or height is negative
//   - The rendering pipeline is not available
func (c *Context) DrawTextureEx(tex *Texture, opts DrawTextureOptions) error {
	if err := validateTexture(tex); err != nil {
		return err
	}

	if opts.Width < 0 || opts.Height < 0 {
		return ErrInvalidDimensions
	}

	// Apply defaults
	if opts.Width == 0 {
		opts.Width = float32(tex.Width())
	}
	if opts.Height == 0 {
		opts.Height = float32(tex.Height())
	}
	if opts.Alpha == 0 {
		opts.Alpha = 1.0
	}

	// Clamp alpha to valid range
	if opts.Alpha < 0 {
		opts.Alpha = 0
	}
	if opts.Alpha > 1 {
		opts.Alpha = 1
	}

	// Delegate to renderer's internal method
	// This will be implemented in INT-002
	return c.renderer.drawTexturedQuad(tex, opts)
}

// validateTexture checks if a texture is valid for drawing.
func validateTexture(tex *Texture) error {
	if tex == nil {
		return ErrTextureNil
	}

	// A texture is considered destroyed if its HAL texture interface is nil
	if tex.texture == nil {
		return ErrTextureDestroyed
	}

	return nil
}

// ErrInvalidTextureType is returned when a texture has an unexpected type.
var ErrInvalidTextureType = errors.New("gogpu: texture must be *Texture")

// Compile-time interface compliance checks.
var (
	_ gpucontext.TextureDrawer  = (*contextTextureDrawer)(nil)
	_ gpucontext.TextureCreator = (*rendererTextureCreator)(nil)
	_ gpucontext.Texture        = (*Texture)(nil)
	_ gpucontext.TextureUpdater = (*Texture)(nil)
)

// contextTextureDrawer adapts Context to gpucontext.TextureDrawer interface.
// This follows the GoF Adapter pattern for clean interface implementation.
type contextTextureDrawer struct {
	ctx     *Context
	creator *rendererTextureCreator
}

// DrawTexture implements gpucontext.TextureDrawer.
func (a *contextTextureDrawer) DrawTexture(tex gpucontext.Texture, x, y float32) error {
	t, ok := tex.(*Texture)
	if !ok {
		return ErrInvalidTextureType
	}
	return a.ctx.DrawTexture(t, x, y)
}

// TextureCreator implements gpucontext.TextureDrawer.
func (a *contextTextureDrawer) TextureCreator() gpucontext.TextureCreator {
	return a.creator
}

// rendererTextureCreator adapts Renderer to gpucontext.TextureCreator interface.
type rendererTextureCreator struct {
	renderer *Renderer
}

// NewTextureFromRGBA implements gpucontext.TextureCreator.
func (c *rendererTextureCreator) NewTextureFromRGBA(width, height int, data []byte) (gpucontext.Texture, error) {
	return c.renderer.NewTextureFromRGBA(width, height, data)
}

// AsTextureDrawer returns an adapter implementing gpucontext.TextureDrawer.
// This enables integration with packages like ggcanvas without circular deps.
//
// Example:
//
//	drawer := dc.AsTextureDrawer()
//	tex, _ := drawer.TextureCreator().NewTextureFromRGBA(800, 600, data)
//	drawer.DrawTexture(tex, 0, 0)
func (c *Context) AsTextureDrawer() gpucontext.TextureDrawer {
	return &contextTextureDrawer{
		ctx:     c,
		creator: &rendererTextureCreator{renderer: c.renderer},
	}
}
