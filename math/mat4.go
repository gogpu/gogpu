package math

import (
	"fmt"
	"math"
)

// Mat4 represents a 4x4 matrix in column-major order.
// This matches the layout expected by GPU APIs.
type Mat4 [16]float32

// Identity4 returns the identity matrix.
func Identity4() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Zero4x4 returns the zero matrix.
func Zero4x4() Mat4 {
	return Mat4{}
}

// NewMat4 creates a matrix from values in column-major order.
func NewMat4(values [16]float32) Mat4 {
	return Mat4(values)
}

// NewMat4FromRows creates a matrix from row values.
func NewMat4FromRows(
	m00, m01, m02, m03 float32,
	m10, m11, m12, m13 float32,
	m20, m21, m22, m23 float32,
	m30, m31, m32, m33 float32,
) Mat4 {
	return Mat4{
		m00, m10, m20, m30,
		m01, m11, m21, m31,
		m02, m12, m22, m32,
		m03, m13, m23, m33,
	}
}

// Translation creates a translation matrix.
func Translation(x, y, z float32) Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		x, y, z, 1,
	}
}

// TranslationVec creates a translation matrix from Vec3.
func TranslationVec(v Vec3) Mat4 {
	return Translation(v.X, v.Y, v.Z)
}

// Scale creates a scaling matrix.
func Scale(x, y, z float32) Mat4 {
	return Mat4{
		x, 0, 0, 0,
		0, y, 0, 0,
		0, 0, z, 0,
		0, 0, 0, 1,
	}
}

// ScaleVec creates a scaling matrix from Vec3.
func ScaleVec(v Vec3) Mat4 {
	return Scale(v.X, v.Y, v.Z)
}

// ScaleUniform creates a uniform scaling matrix.
func ScaleUniform(s float32) Mat4 {
	return Scale(s, s, s)
}

// RotationX creates a rotation matrix around the X axis.
func RotationX(radians float32) Mat4 {
	c := float32(math.Cos(float64(radians)))
	s := float32(math.Sin(float64(radians)))
	return Mat4{
		1, 0, 0, 0,
		0, c, s, 0,
		0, -s, c, 0,
		0, 0, 0, 1,
	}
}

// RotationY creates a rotation matrix around the Y axis.
func RotationY(radians float32) Mat4 {
	c := float32(math.Cos(float64(radians)))
	s := float32(math.Sin(float64(radians)))
	return Mat4{
		c, 0, -s, 0,
		0, 1, 0, 0,
		s, 0, c, 0,
		0, 0, 0, 1,
	}
}

// RotationZ creates a rotation matrix around the Z axis.
func RotationZ(radians float32) Mat4 {
	c := float32(math.Cos(float64(radians)))
	s := float32(math.Sin(float64(radians)))
	return Mat4{
		c, s, 0, 0,
		-s, c, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// RotationAxis creates a rotation matrix around an arbitrary axis.
func RotationAxis(axis Vec3, radians float32) Mat4 {
	axis = axis.Normalize()
	c := float32(math.Cos(float64(radians)))
	s := float32(math.Sin(float64(radians)))
	t := 1 - c
	x, y, z := axis.X, axis.Y, axis.Z

	return Mat4{
		t*x*x + c, t*x*y + s*z, t*x*z - s*y, 0,
		t*x*y - s*z, t*y*y + c, t*y*z + s*x, 0,
		t*x*z + s*y, t*y*z - s*x, t*z*z + c, 0,
		0, 0, 0, 1,
	}
}

// Perspective creates a perspective projection matrix.
func Perspective(fovY, aspect, near, far float32) Mat4 {
	f := 1 / float32(math.Tan(float64(fovY/2)))
	nf := 1 / (near - far)

	return Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) * nf, -1,
		0, 0, 2 * far * near * nf, 0,
	}
}

// Orthographic creates an orthographic projection matrix.
func Orthographic(left, right, bottom, top, near, far float32) Mat4 {
	rl := 1 / (right - left)
	tb := 1 / (top - bottom)
	fn := 1 / (far - near)

	return Mat4{
		2 * rl, 0, 0, 0,
		0, 2 * tb, 0, 0,
		0, 0, -2 * fn, 0,
		-(right + left) * rl, -(top + bottom) * tb, -(far + near) * fn, 1,
	}
}

// LookAt creates a view matrix looking from eye to target.
func LookAt(eye, target, up Vec3) Mat4 {
	f := target.Sub(eye).Normalize()
	s := f.Cross(up).Normalize()
	u := s.Cross(f)

	return Mat4{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-s.Dot(eye), -u.Dot(eye), f.Dot(eye), 1,
	}
}

// Mul multiplies two matrices.
func (m Mat4) Mul(other Mat4) Mat4 {
	var result Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				result[j*4+i] += m[k*4+i] * other[j*4+k]
			}
		}
	}
	return result
}

// MulVec4 multiplies matrix by Vec4.
func (m Mat4) MulVec4(v Vec4) Vec4 {
	return Vec4{
		X: m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]*v.W,
		Y: m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]*v.W,
		Z: m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]*v.W,
		W: m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]*v.W,
	}
}

// MulVec3 multiplies matrix by Vec3 (assumes w=1).
func (m Mat4) MulVec3(v Vec3) Vec3 {
	w := m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]
	return Vec3{
		X: (m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]) / w,
		Y: (m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]) / w,
		Z: (m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]) / w,
	}
}

// Transpose returns the transposed matrix.
func (m Mat4) Transpose() Mat4 {
	return Mat4{
		m[0], m[4], m[8], m[12],
		m[1], m[5], m[9], m[13],
		m[2], m[6], m[10], m[14],
		m[3], m[7], m[11], m[15],
	}
}

// Determinant returns the matrix determinant.
func (m Mat4) Determinant() float32 {
	a00, a01, a02, a03 := m[0], m[1], m[2], m[3]
	a10, a11, a12, a13 := m[4], m[5], m[6], m[7]
	a20, a21, a22, a23 := m[8], m[9], m[10], m[11]
	a30, a31, a32, a33 := m[12], m[13], m[14], m[15]

	b00 := a00*a11 - a01*a10
	b01 := a00*a12 - a02*a10
	b02 := a00*a13 - a03*a10
	b03 := a01*a12 - a02*a11
	b04 := a01*a13 - a03*a11
	b05 := a02*a13 - a03*a12
	b06 := a20*a31 - a21*a30
	b07 := a20*a32 - a22*a30
	b08 := a20*a33 - a23*a30
	b09 := a21*a32 - a22*a31
	b10 := a21*a33 - a23*a31
	b11 := a22*a33 - a23*a32

	return b00*b11 - b01*b10 + b02*b09 + b03*b08 - b04*b07 + b05*b06
}

// String returns a string representation.
func (m Mat4) String() string {
	return fmt.Sprintf("Mat4[\n  %f, %f, %f, %f\n  %f, %f, %f, %f\n  %f, %f, %f, %f\n  %f, %f, %f, %f\n]",
		m[0], m[4], m[8], m[12],
		m[1], m[5], m[9], m[13],
		m[2], m[6], m[10], m[14],
		m[3], m[7], m[11], m[15])
}
