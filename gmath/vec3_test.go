package gmath

import (
	"testing"
)

func TestNewVec3(t *testing.T) {
	v := NewVec3(1, 2, 3)
	if v.X != 1 || v.Y != 2 || v.Z != 3 {
		t.Errorf("NewVec3(1, 2, 3) = %v", v)
	}
}

func TestZero3(t *testing.T) {
	v := Zero3()
	if v.X != 0 || v.Y != 0 || v.Z != 0 {
		t.Errorf("Zero3() = %v, want (0, 0, 0)", v)
	}
}

func TestOne3(t *testing.T) {
	v := One3()
	if v.X != 1 || v.Y != 1 || v.Z != 1 {
		t.Errorf("One3() = %v, want (1, 1, 1)", v)
	}
}

func TestUnitVectors(t *testing.T) {
	ux := UnitX()
	uy := UnitY()
	uz := UnitZ()

	if ux.X != 1 || ux.Y != 0 || ux.Z != 0 {
		t.Errorf("UnitX() = %v", ux)
	}
	if uy.X != 0 || uy.Y != 1 || uy.Z != 0 {
		t.Errorf("UnitY() = %v", uy)
	}
	if uz.X != 0 || uz.Y != 0 || uz.Z != 1 {
		t.Errorf("UnitZ() = %v", uz)
	}
}

func TestVec3Add(t *testing.T) {
	v1 := NewVec3(1, 2, 3)
	v2 := NewVec3(4, 5, 6)
	result := v1.Add(v2)

	if result.X != 5 || result.Y != 7 || result.Z != 9 {
		t.Errorf("Add = %v, want (5, 7, 9)", result)
	}
}

func TestVec3Sub(t *testing.T) {
	v1 := NewVec3(5, 7, 9)
	v2 := NewVec3(1, 2, 3)
	result := v1.Sub(v2)

	if result.X != 4 || result.Y != 5 || result.Z != 6 {
		t.Errorf("Sub = %v, want (4, 5, 6)", result)
	}
}

func TestVec3Mul(t *testing.T) {
	v := NewVec3(1, 2, 3)
	result := v.Mul(2)

	if result.X != 2 || result.Y != 4 || result.Z != 6 {
		t.Errorf("Mul = %v, want (2, 4, 6)", result)
	}
}

func TestVec3Div(t *testing.T) {
	v := NewVec3(2, 4, 6)
	result := v.Div(2)

	if result.X != 1 || result.Y != 2 || result.Z != 3 {
		t.Errorf("Div = %v, want (1, 2, 3)", result)
	}
}

func TestVec3Dot(t *testing.T) {
	v1 := NewVec3(1, 2, 3)
	v2 := NewVec3(4, 5, 6)
	result := v1.Dot(v2)

	// 1*4 + 2*5 + 3*6 = 4 + 10 + 18 = 32
	if result != 32 {
		t.Errorf("Dot = %f, want 32", result)
	}
}

func TestVec3Cross(t *testing.T) {
	// Cross product of unit X and unit Y should be unit Z
	x := UnitX()
	y := UnitY()
	result := x.Cross(y)

	if result.X != 0 || result.Y != 0 || result.Z != 1 {
		t.Errorf("UnitX.Cross(UnitY) = %v, want (0, 0, 1)", result)
	}

	// Cross product of unit Y and unit X should be negative unit Z
	result2 := y.Cross(x)
	if result2.X != 0 || result2.Y != 0 || result2.Z != -1 {
		t.Errorf("UnitY.Cross(UnitX) = %v, want (0, 0, -1)", result2)
	}

	// Cross product of a vector with itself is zero
	v := NewVec3(1, 2, 3)
	self := v.Cross(v)
	if self.X != 0 || self.Y != 0 || self.Z != 0 {
		t.Errorf("v.Cross(v) = %v, want (0, 0, 0)", self)
	}
}

func TestVec3Length(t *testing.T) {
	v := NewVec3(2, 3, 6)
	result := v.Length()

	// sqrt(4 + 9 + 36) = sqrt(49) = 7
	if !almostEqual(result, 7) {
		t.Errorf("Length = %f, want 7", result)
	}
}

func TestVec3LengthSquared(t *testing.T) {
	v := NewVec3(2, 3, 6)
	result := v.LengthSquared()

	// 4 + 9 + 36 = 49
	if result != 49 {
		t.Errorf("LengthSquared = %f, want 49", result)
	}
}

func TestVec3Normalize(t *testing.T) {
	v := NewVec3(0, 0, 5)
	result := v.Normalize()

	if result.X != 0 || result.Y != 0 || result.Z != 1 {
		t.Errorf("Normalize = %v, want (0, 0, 1)", result)
	}

	// Length should be 1
	if !almostEqual(result.Length(), 1) {
		t.Errorf("Normalized length = %f, want 1", result.Length())
	}
}

func TestVec3NormalizeZero(t *testing.T) {
	v := Zero3()
	result := v.Normalize()

	if result.X != 0 || result.Y != 0 || result.Z != 0 {
		t.Errorf("Zero3().Normalize() = %v, want (0, 0, 0)", result)
	}
}

func TestVec3Lerp(t *testing.T) {
	v1 := Zero3()
	v2 := NewVec3(10, 20, 30)

	result := v1.Lerp(v2, 0.5)
	if result.X != 5 || result.Y != 10 || result.Z != 15 {
		t.Errorf("Lerp(0.5) = %v, want (5, 10, 15)", result)
	}
}

func TestVec3Distance(t *testing.T) {
	v1 := Zero3()
	v2 := NewVec3(2, 3, 6)
	result := v1.Distance(v2)

	if !almostEqual(result, 7) {
		t.Errorf("Distance = %f, want 7", result)
	}
}

func TestVec3Reflect(t *testing.T) {
	// Reflect (1, -1, 0) off a horizontal surface (0, 1, 0)
	v := NewVec3(1, -1, 0).Normalize()
	n := UnitY()
	result := v.Reflect(n)

	// Reflected direction should be (1, 1, 0) normalized
	expected := NewVec3(1, 1, 0).Normalize()
	if !almostEqual(result.X, expected.X) || !almostEqual(result.Y, expected.Y) || !almostEqual(result.Z, expected.Z) {
		t.Errorf("Reflect = %v, want %v", result, expected)
	}
}

func TestVec3Abs(t *testing.T) {
	v := NewVec3(-1, -2, -3)
	result := v.Abs()

	if result.X != 1 || result.Y != 2 || result.Z != 3 {
		t.Errorf("Abs = %v, want (1, 2, 3)", result)
	}
}

func TestVec3MinMax(t *testing.T) {
	v1 := NewVec3(1, 5, 3)
	v2 := NewVec3(3, 2, 4)

	minResult := v1.Min(v2)
	maxResult := v1.Max(v2)

	if minResult.X != 1 || minResult.Y != 2 || minResult.Z != 3 {
		t.Errorf("Min = %v, want (1, 2, 3)", minResult)
	}
	if maxResult.X != 3 || maxResult.Y != 5 || maxResult.Z != 4 {
		t.Errorf("Max = %v, want (3, 5, 4)", maxResult)
	}
}

func TestVec3XY(t *testing.T) {
	v := NewVec3(1, 2, 3)
	xy := v.XY()

	if xy.X != 1 || xy.Y != 2 {
		t.Errorf("XY = %v, want (1, 2)", xy)
	}
}

func TestVec3String(t *testing.T) {
	v := NewVec3(1, 2, 3)
	s := v.String()

	if s == "" {
		t.Error("String() returned empty string")
	}
}
