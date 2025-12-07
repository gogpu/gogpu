package gmath

import (
	"fmt"
	"math"
)

// Vec4 represents a 4D vector.
type Vec4 struct {
	X, Y, Z, W float32
}

// NewVec4 creates a new Vec4.
func NewVec4(x, y, z, w float32) Vec4 {
	return Vec4{X: x, Y: y, Z: z, W: w}
}

// Zero4 returns a zero vector.
func Zero4() Vec4 {
	return Vec4{0, 0, 0, 0}
}

// One4 returns a unit vector.
func One4() Vec4 {
	return Vec4{1, 1, 1, 1}
}

// FromVec3 creates a Vec4 from Vec3 with w component.
func FromVec3(v Vec3, w float32) Vec4 {
	return Vec4{v.X, v.Y, v.Z, w}
}

// Add returns v + other.
func (v Vec4) Add(other Vec4) Vec4 {
	return Vec4{v.X + other.X, v.Y + other.Y, v.Z + other.Z, v.W + other.W}
}

// Sub returns v - other.
func (v Vec4) Sub(other Vec4) Vec4 {
	return Vec4{v.X - other.X, v.Y - other.Y, v.Z - other.Z, v.W - other.W}
}

// Mul returns v * scalar.
func (v Vec4) Mul(scalar float32) Vec4 {
	return Vec4{v.X * scalar, v.Y * scalar, v.Z * scalar, v.W * scalar}
}

// Div returns v / scalar.
func (v Vec4) Div(scalar float32) Vec4 {
	return Vec4{v.X / scalar, v.Y / scalar, v.Z / scalar, v.W / scalar}
}

// Dot returns the dot product of v and other.
func (v Vec4) Dot(other Vec4) float32 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z + v.W*other.W
}

// Length returns the magnitude of the vector.
func (v Vec4) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z + v.W*v.W)))
}

// LengthSquared returns the squared magnitude.
func (v Vec4) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z + v.W*v.W
}

// Normalize returns a unit vector in the same direction.
func (v Vec4) Normalize() Vec4 {
	l := v.Length()
	if l == 0 {
		return Zero4()
	}
	return v.Div(l)
}

// Lerp returns linear interpolation between v and other.
func (v Vec4) Lerp(other Vec4, t float32) Vec4 {
	return Vec4{
		X: v.X + (other.X-v.X)*t,
		Y: v.Y + (other.Y-v.Y)*t,
		Z: v.Z + (other.Z-v.Z)*t,
		W: v.W + (other.W-v.W)*t,
	}
}

// XYZ returns a Vec3 with X, Y, Z components.
func (v Vec4) XYZ() Vec3 {
	return Vec3{v.X, v.Y, v.Z}
}

// XY returns a Vec2 with X, Y components.
func (v Vec4) XY() Vec2 {
	return Vec2{v.X, v.Y}
}

// String returns a string representation.
func (v Vec4) String() string {
	return fmt.Sprintf("Vec4(%f, %f, %f, %f)", v.X, v.Y, v.Z, v.W)
}
