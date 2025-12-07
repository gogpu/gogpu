package gmath

import (
	"math"
	"testing"
)

func TestIdentity4(t *testing.T) {
	m := Identity4()

	// Check diagonal
	if m[0] != 1 || m[5] != 1 || m[10] != 1 || m[15] != 1 {
		t.Errorf("Identity diagonal incorrect: %v", m)
	}

	// Check off-diagonal (should all be 0)
	for i := 0; i < 16; i++ {
		if i == 0 || i == 5 || i == 10 || i == 15 {
			continue
		}
		if m[i] != 0 {
			t.Errorf("Identity off-diagonal[%d] = %f, want 0", i, m[i])
		}
	}
}

func TestZero4x4(t *testing.T) {
	m := Zero4x4()

	for i := 0; i < 16; i++ {
		if m[i] != 0 {
			t.Errorf("Zero4x4[%d] = %f, want 0", i, m[i])
		}
	}
}

func TestNewMat4(t *testing.T) {
	values := [16]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	m := NewMat4(values)

	for i := 0; i < 16; i++ {
		if m[i] != float32(i+1) {
			t.Errorf("NewMat4[%d] = %f, want %d", i, m[i], i+1)
		}
	}
}

func TestTranslation(t *testing.T) {
	m := Translation(1, 2, 3)

	// Translation should be in column 4 (indices 12, 13, 14)
	if m[12] != 1 || m[13] != 2 || m[14] != 3 {
		t.Errorf("Translation = (%f, %f, %f), want (1, 2, 3)", m[12], m[13], m[14])
	}

	// m[15] should be 1
	if m[15] != 1 {
		t.Errorf("Translation[15] = %f, want 1", m[15])
	}
}

func TestTranslationVec(t *testing.T) {
	v := NewVec3(1, 2, 3)
	m := TranslationVec(v)

	if m[12] != 1 || m[13] != 2 || m[14] != 3 {
		t.Errorf("TranslationVec = (%f, %f, %f)", m[12], m[13], m[14])
	}
}

func TestScale(t *testing.T) {
	m := Scale(2, 3, 4)

	if m[0] != 2 || m[5] != 3 || m[10] != 4 || m[15] != 1 {
		t.Errorf("Scale diagonal = (%f, %f, %f, %f)", m[0], m[5], m[10], m[15])
	}
}

func TestScaleVec(t *testing.T) {
	v := NewVec3(2, 3, 4)
	m := ScaleVec(v)

	if m[0] != 2 || m[5] != 3 || m[10] != 4 {
		t.Errorf("ScaleVec = (%f, %f, %f)", m[0], m[5], m[10])
	}
}

func TestScaleUniform(t *testing.T) {
	m := ScaleUniform(2)

	if m[0] != 2 || m[5] != 2 || m[10] != 2 {
		t.Errorf("ScaleUniform = (%f, %f, %f)", m[0], m[5], m[10])
	}
}

func TestRotationX(t *testing.T) {
	// Rotate 90 degrees around X axis
	m := RotationX(float32(math.Pi / 2))

	// Apply to unit Y vector, should get unit Z
	v := NewVec4(0, 1, 0, 1)
	result := m.MulVec4(v)

	if !almostEqual(result.X, 0) || !almostEqual(result.Y, 0) || !almostEqual(result.Z, 1) {
		t.Errorf("RotationX(90) * (0,1,0) = %v, want (0,0,1)", result)
	}
}

func TestRotationY(t *testing.T) {
	// Rotate 90 degrees around Y axis
	m := RotationY(float32(math.Pi / 2))

	// Apply to unit Z vector, should get unit X
	v := NewVec4(0, 0, 1, 1)
	result := m.MulVec4(v)

	if !almostEqual(result.X, 1) || !almostEqual(result.Y, 0) || !almostEqual(result.Z, 0) {
		t.Errorf("RotationY(90) * (0,0,1) = %v, want (1,0,0)", result)
	}
}

func TestRotationZ(t *testing.T) {
	// Rotate 90 degrees around Z axis
	m := RotationZ(float32(math.Pi / 2))

	// Apply to unit X vector, should get unit Y
	v := NewVec4(1, 0, 0, 1)
	result := m.MulVec4(v)

	if !almostEqual(result.X, 0) || !almostEqual(result.Y, 1) || !almostEqual(result.Z, 0) {
		t.Errorf("RotationZ(90) * (1,0,0) = %v, want (0,1,0)", result)
	}
}

func TestRotationAxis(t *testing.T) {
	// Rotate 90 degrees around Z axis (same as RotationZ)
	m := RotationAxis(UnitZ(), float32(math.Pi/2))

	v := NewVec4(1, 0, 0, 1)
	result := m.MulVec4(v)

	if !almostEqual(result.X, 0) || !almostEqual(result.Y, 1) || !almostEqual(result.Z, 0) {
		t.Errorf("RotationAxis(Z, 90) * (1,0,0) = %v, want (0,1,0)", result)
	}
}

func TestMatrixMultiplication(t *testing.T) {
	// Identity * anything = anything
	id := Identity4()
	other := Translation(1, 2, 3)
	result := id.Mul(other)

	for i := 0; i < 16; i++ {
		if result[i] != other[i] {
			t.Errorf("Identity * M != M at index %d", i)
		}
	}

	// anything * Identity = anything
	result2 := other.Mul(id)
	for i := 0; i < 16; i++ {
		if result2[i] != other[i] {
			t.Errorf("M * Identity != M at index %d", i)
		}
	}
}

func TestMulVec4(t *testing.T) {
	// Translation should move point
	m := Translation(10, 20, 30)
	v := NewVec4(1, 2, 3, 1)
	result := m.MulVec4(v)

	if result.X != 11 || result.Y != 22 || result.Z != 33 || result.W != 1 {
		t.Errorf("Translation * point = %v, want (11, 22, 33, 1)", result)
	}
}

func TestMulVec3(t *testing.T) {
	m := Translation(10, 20, 30)
	v := NewVec3(1, 2, 3)
	result := m.MulVec3(v)

	if result.X != 11 || result.Y != 22 || result.Z != 33 {
		t.Errorf("Translation * Vec3 = %v, want (11, 22, 33)", result)
	}
}

func TestTranspose(t *testing.T) {
	m := NewMat4FromRows(
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	)

	tr := m.Transpose()

	// After transpose, rows become columns
	expected := NewMat4FromRows(
		1, 5, 9, 13,
		2, 6, 10, 14,
		3, 7, 11, 15,
		4, 8, 12, 16,
	)

	for i := 0; i < 16; i++ {
		if tr[i] != expected[i] {
			t.Errorf("Transpose[%d] = %f, want %f", i, tr[i], expected[i])
		}
	}
}

func TestDeterminant(t *testing.T) {
	// Identity has determinant 1
	id := Identity4()
	det := id.Determinant()
	if !almostEqual(det, 1) {
		t.Errorf("Identity determinant = %f, want 1", det)
	}

	// Scale matrix has determinant = product of scales
	scale := Scale(2, 3, 4)
	det2 := scale.Determinant()
	if !almostEqual(det2, 24) { // 2*3*4
		t.Errorf("Scale determinant = %f, want 24", det2)
	}
}

func TestPerspective(t *testing.T) {
	fov := float32(math.Pi / 4) // 45 degrees
	aspect := float32(16.0 / 9.0)
	near := float32(0.1)
	far := float32(100.0)

	m := Perspective(fov, aspect, near, far)

	// Verify m[15] is 0 (perspective divide)
	if m[15] != 0 {
		t.Errorf("Perspective[15] = %f, want 0", m[15])
	}

	// Verify m[11] is -1 (perspective w-coordinate)
	if m[11] != -1 {
		t.Errorf("Perspective[11] = %f, want -1", m[11])
	}
}

func TestOrthographic(t *testing.T) {
	m := Orthographic(-1, 1, -1, 1, 0, 1)

	// Orthographic should have 1 in w position
	if m[15] != 1 {
		t.Errorf("Orthographic[15] = %f, want 1", m[15])
	}
}

func TestLookAt(t *testing.T) {
	eye := NewVec3(0, 0, 5)
	target := Zero3()
	up := UnitY()

	m := LookAt(eye, target, up)

	// The view matrix should be valid (determinant != 0)
	det := m.Determinant()
	if almostEqual(det, 0) {
		t.Error("LookAt produced singular matrix")
	}
}

func TestMat4String(t *testing.T) {
	m := Identity4()
	s := m.String()

	if s == "" {
		t.Error("String() returned empty string")
	}
}
