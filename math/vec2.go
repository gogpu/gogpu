// Package math provides vector and matrix types for gogpu.
package math

import (
	"fmt"
	"math"
)

// Vec2 represents a 2D vector.
type Vec2 struct {
	X, Y float32
}

// NewVec2 creates a new Vec2.
func NewVec2(x, y float32) Vec2 {
	return Vec2{X: x, Y: y}
}

// Zero2 returns a zero vector.
func Zero2() Vec2 {
	return Vec2{0, 0}
}

// One2 returns a unit vector.
func One2() Vec2 {
	return Vec2{1, 1}
}

// Add returns v + other.
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{v.X + other.X, v.Y + other.Y}
}

// Sub returns v - other.
func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{v.X - other.X, v.Y - other.Y}
}

// Mul returns v * scalar.
func (v Vec2) Mul(scalar float32) Vec2 {
	return Vec2{v.X * scalar, v.Y * scalar}
}

// Div returns v / scalar.
func (v Vec2) Div(scalar float32) Vec2 {
	return Vec2{v.X / scalar, v.Y / scalar}
}

// Dot returns the dot product of v and other.
func (v Vec2) Dot(other Vec2) float32 {
	return v.X*other.X + v.Y*other.Y
}

// Length returns the magnitude of the vector.
func (v Vec2) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y)))
}

// LengthSquared returns the squared magnitude (faster than Length).
func (v Vec2) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y
}

// Normalize returns a unit vector in the same direction.
func (v Vec2) Normalize() Vec2 {
	l := v.Length()
	if l == 0 {
		return Zero2()
	}
	return v.Div(l)
}

// Lerp returns linear interpolation between v and other.
func (v Vec2) Lerp(other Vec2, t float32) Vec2 {
	return Vec2{
		X: v.X + (other.X-v.X)*t,
		Y: v.Y + (other.Y-v.Y)*t,
	}
}

// Distance returns the distance between v and other.
func (v Vec2) Distance(other Vec2) float32 {
	return v.Sub(other).Length()
}

// Angle returns the angle in radians between v and other.
func (v Vec2) Angle(other Vec2) float32 {
	return float32(math.Atan2(float64(other.Y-v.Y), float64(other.X-v.X)))
}

// Rotate returns v rotated by angle radians.
func (v Vec2) Rotate(angle float32) Vec2 {
	cos := float32(math.Cos(float64(angle)))
	sin := float32(math.Sin(float64(angle)))
	return Vec2{
		X: v.X*cos - v.Y*sin,
		Y: v.X*sin + v.Y*cos,
	}
}

// Perpendicular returns a perpendicular vector.
func (v Vec2) Perpendicular() Vec2 {
	return Vec2{-v.Y, v.X}
}

// Abs returns a vector with absolute values.
func (v Vec2) Abs() Vec2 {
	return Vec2{
		X: float32(math.Abs(float64(v.X))),
		Y: float32(math.Abs(float64(v.Y))),
	}
}

// Min returns the component-wise minimum.
func (v Vec2) Min(other Vec2) Vec2 {
	return Vec2{
		X: float32(math.Min(float64(v.X), float64(other.X))),
		Y: float32(math.Min(float64(v.Y), float64(other.Y))),
	}
}

// Max returns the component-wise maximum.
func (v Vec2) Max(other Vec2) Vec2 {
	return Vec2{
		X: float32(math.Max(float64(v.X), float64(other.X))),
		Y: float32(math.Max(float64(v.Y), float64(other.Y))),
	}
}

// Clamp returns v clamped to [min, max].
func (v Vec2) Clamp(min, max Vec2) Vec2 {
	return v.Max(min).Min(max)
}

// String returns a string representation.
func (v Vec2) String() string {
	return fmt.Sprintf("Vec2(%f, %f)", v.X, v.Y)
}
