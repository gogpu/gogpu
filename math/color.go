package math

import (
	"fmt"
)

// Color represents an RGBA color with float32 components.
// Components are in the range [0, 1].
type Color struct {
	R, G, B, A float32
}

// NewColor creates a new color.
func NewColor(r, g, b, a float32) Color {
	return Color{R: r, G: g, B: b, A: a}
}

// RGB creates a color from RGB values (alpha = 1).
func RGB(r, g, b float32) Color {
	return Color{R: r, G: g, B: b, A: 1}
}

// RGBA creates a color from RGBA values.
func RGBA(r, g, b, a float32) Color {
	return Color{R: r, G: g, B: b, A: a}
}

// Hex creates a color from a hex value (0xRRGGBB or 0xRRGGBBAA).
func Hex(hex uint32) Color {
	if hex <= 0xFFFFFF {
		return Color{
			R: float32((hex>>16)&0xFF) / 255,
			G: float32((hex>>8)&0xFF) / 255,
			B: float32(hex&0xFF) / 255,
			A: 1,
		}
	}
	return Color{
		R: float32((hex>>24)&0xFF) / 255,
		G: float32((hex>>16)&0xFF) / 255,
		B: float32((hex>>8)&0xFF) / 255,
		A: float32(hex&0xFF) / 255,
	}
}

// ToVec4 converts color to Vec4.
func (c Color) ToVec4() Vec4 {
	return Vec4{c.R, c.G, c.B, c.A}
}

// Lerp interpolates between two colors.
func (c Color) Lerp(other Color, t float32) Color {
	return Color{
		R: c.R + (other.R-c.R)*t,
		G: c.G + (other.G-c.G)*t,
		B: c.B + (other.B-c.B)*t,
		A: c.A + (other.A-c.A)*t,
	}
}

// WithAlpha returns a new color with modified alpha.
func (c Color) WithAlpha(a float32) Color {
	return Color{c.R, c.G, c.B, a}
}

// Premultiply returns the premultiplied alpha version.
func (c Color) Premultiply() Color {
	return Color{c.R * c.A, c.G * c.A, c.B * c.A, c.A}
}

// String returns a string representation.
func (c Color) String() string {
	return fmt.Sprintf("Color(%f, %f, %f, %f)", c.R, c.G, c.B, c.A)
}

// Predefined colors
var (
	// Basic colors
	Black       = Color{0, 0, 0, 1}
	White       = Color{1, 1, 1, 1}
	Red         = Color{1, 0, 0, 1}
	Green       = Color{0, 1, 0, 1}
	Blue        = Color{0, 0, 1, 1}
	Yellow      = Color{1, 1, 0, 1}
	Cyan        = Color{0, 1, 1, 1}
	Magenta     = Color{1, 0, 1, 1}
	Transparent = Color{0, 0, 0, 0}

	// Grays
	Gray      = Color{0.5, 0.5, 0.5, 1}
	DarkGray  = Color{0.25, 0.25, 0.25, 1}
	LightGray = Color{0.75, 0.75, 0.75, 1}

	// Common colors
	Orange = Color{1, 0.647, 0, 1}
	Pink   = Color{1, 0.753, 0.796, 1}
	Purple = Color{0.5, 0, 0.5, 1}
	Brown  = Color{0.647, 0.165, 0.165, 1}

	// Gopher blue
	GopherBlue = Color{0.0, 0.686, 0.843, 1} // #00AFD7
)
