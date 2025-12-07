package gmath

import (
	"fmt"
	"math"
)

// Vec3 represents a 3D vector.
type Vec3 struct {
	X, Y, Z float32
}

// NewVec3 creates a new Vec3.
func NewVec3(x, y, z float32) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

// Zero3 returns a zero vector.
func Zero3() Vec3 {
	return Vec3{0, 0, 0}
}

// One3 returns a unit vector.
func One3() Vec3 {
	return Vec3{1, 1, 1}
}

// UnitX returns the X axis unit vector.
func UnitX() Vec3 {
	return Vec3{1, 0, 0}
}

// UnitY returns the Y axis unit vector.
func UnitY() Vec3 {
	return Vec3{0, 1, 0}
}

// UnitZ returns the Z axis unit vector.
func UnitZ() Vec3 {
	return Vec3{0, 0, 1}
}

// Add returns v + other.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns v - other.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Mul returns v * scalar.
func (v Vec3) Mul(scalar float32) Vec3 {
	return Vec3{v.X * scalar, v.Y * scalar, v.Z * scalar}
}

// Div returns v / scalar.
func (v Vec3) Div(scalar float32) Vec3 {
	return Vec3{v.X / scalar, v.Y / scalar, v.Z / scalar}
}

// Dot returns the dot product of v and other.
func (v Vec3) Dot(other Vec3) float32 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Cross returns the cross product of v and other.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

// Length returns the magnitude of the vector.
func (v Vec3) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// LengthSquared returns the squared magnitude (faster than Length).
func (v Vec3) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

// Normalize returns a unit vector in the same direction.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l == 0 {
		return Zero3()
	}
	return v.Div(l)
}

// Lerp returns linear interpolation between v and other.
func (v Vec3) Lerp(other Vec3, t float32) Vec3 {
	return Vec3{
		X: v.X + (other.X-v.X)*t,
		Y: v.Y + (other.Y-v.Y)*t,
		Z: v.Z + (other.Z-v.Z)*t,
	}
}

// Distance returns the distance between v and other.
func (v Vec3) Distance(other Vec3) float32 {
	return v.Sub(other).Length()
}

// Reflect returns v reflected off a surface with normal n.
func (v Vec3) Reflect(n Vec3) Vec3 {
	return v.Sub(n.Mul(2 * v.Dot(n)))
}

// Abs returns a vector with absolute values.
func (v Vec3) Abs() Vec3 {
	return Vec3{
		X: float32(math.Abs(float64(v.X))),
		Y: float32(math.Abs(float64(v.Y))),
		Z: float32(math.Abs(float64(v.Z))),
	}
}

// Min returns the component-wise minimum.
func (v Vec3) Min(other Vec3) Vec3 {
	return Vec3{
		X: float32(math.Min(float64(v.X), float64(other.X))),
		Y: float32(math.Min(float64(v.Y), float64(other.Y))),
		Z: float32(math.Min(float64(v.Z), float64(other.Z))),
	}
}

// Max returns the component-wise maximum.
func (v Vec3) Max(other Vec3) Vec3 {
	return Vec3{
		X: float32(math.Max(float64(v.X), float64(other.X))),
		Y: float32(math.Max(float64(v.Y), float64(other.Y))),
		Z: float32(math.Max(float64(v.Z), float64(other.Z))),
	}
}

// Clamp returns v clamped to [minVal, maxVal].
func (v Vec3) Clamp(minVal, maxVal Vec3) Vec3 {
	return v.Max(minVal).Min(maxVal)
}

// XY returns a Vec2 with X and Y components.
func (v Vec3) XY() Vec2 {
	return Vec2{v.X, v.Y}
}

// String returns a string representation.
func (v Vec3) String() string {
	return fmt.Sprintf("Vec3(%f, %f, %f)", v.X, v.Y, v.Z)
}
