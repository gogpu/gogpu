package gmath

import (
	"testing"
)

func TestNewVec4(t *testing.T) {
	v := NewVec4(1, 2, 3, 4)
	if v.X != 1 || v.Y != 2 || v.Z != 3 || v.W != 4 {
		t.Errorf("NewVec4(1, 2, 3, 4) = %v", v)
	}
}

func TestZero4(t *testing.T) {
	v := Zero4()
	if v.X != 0 || v.Y != 0 || v.Z != 0 || v.W != 0 {
		t.Errorf("Zero4() = %v", v)
	}
}

func TestOne4(t *testing.T) {
	v := One4()
	if v.X != 1 || v.Y != 1 || v.Z != 1 || v.W != 1 {
		t.Errorf("One4() = %v", v)
	}
}

func TestFromVec3(t *testing.T) {
	v3 := NewVec3(1, 2, 3)
	v4 := FromVec3(v3, 4)

	if v4.X != 1 || v4.Y != 2 || v4.Z != 3 || v4.W != 4 {
		t.Errorf("FromVec3 = %v", v4)
	}
}

func TestVec4Add(t *testing.T) {
	v1 := NewVec4(1, 2, 3, 4)
	v2 := NewVec4(5, 6, 7, 8)
	result := v1.Add(v2)

	if result.X != 6 || result.Y != 8 || result.Z != 10 || result.W != 12 {
		t.Errorf("Add = %v", result)
	}
}

func TestVec4Sub(t *testing.T) {
	v1 := NewVec4(5, 6, 7, 8)
	v2 := NewVec4(1, 2, 3, 4)
	result := v1.Sub(v2)

	if result.X != 4 || result.Y != 4 || result.Z != 4 || result.W != 4 {
		t.Errorf("Sub = %v", result)
	}
}

func TestVec4Mul(t *testing.T) {
	v := NewVec4(1, 2, 3, 4)
	result := v.Mul(2)

	if result.X != 2 || result.Y != 4 || result.Z != 6 || result.W != 8 {
		t.Errorf("Mul = %v", result)
	}
}

func TestVec4Div(t *testing.T) {
	v := NewVec4(2, 4, 6, 8)
	result := v.Div(2)

	if result.X != 1 || result.Y != 2 || result.Z != 3 || result.W != 4 {
		t.Errorf("Div = %v", result)
	}
}

func TestVec4Dot(t *testing.T) {
	v1 := NewVec4(1, 2, 3, 4)
	v2 := NewVec4(5, 6, 7, 8)
	result := v1.Dot(v2)

	// 1*5 + 2*6 + 3*7 + 4*8 = 5 + 12 + 21 + 32 = 70
	if result != 70 {
		t.Errorf("Dot = %f, want 70", result)
	}
}

func TestVec4Length(t *testing.T) {
	v := NewVec4(2, 0, 0, 0)
	result := v.Length()

	if result != 2 {
		t.Errorf("Length = %f, want 2", result)
	}
}

func TestVec4LengthSquared(t *testing.T) {
	v := NewVec4(1, 2, 3, 4)
	result := v.LengthSquared()

	// 1 + 4 + 9 + 16 = 30
	if result != 30 {
		t.Errorf("LengthSquared = %f, want 30", result)
	}
}

func TestVec4Normalize(t *testing.T) {
	v := NewVec4(4, 0, 0, 0)
	result := v.Normalize()

	if result.X != 1 || result.Y != 0 || result.Z != 0 || result.W != 0 {
		t.Errorf("Normalize = %v, want (1, 0, 0, 0)", result)
	}

	if !almostEqual(result.Length(), 1) {
		t.Errorf("Normalized length = %f, want 1", result.Length())
	}
}

func TestVec4NormalizeZero(t *testing.T) {
	v := Zero4()
	result := v.Normalize()

	if result.X != 0 || result.Y != 0 || result.Z != 0 || result.W != 0 {
		t.Errorf("Zero4().Normalize() = %v", result)
	}
}

func TestVec4Lerp(t *testing.T) {
	v1 := Zero4()
	v2 := NewVec4(10, 20, 30, 40)

	result := v1.Lerp(v2, 0.5)
	if result.X != 5 || result.Y != 10 || result.Z != 15 || result.W != 20 {
		t.Errorf("Lerp(0.5) = %v", result)
	}
}

func TestVec4XYZ(t *testing.T) {
	v := NewVec4(1, 2, 3, 4)
	xyz := v.XYZ()

	if xyz.X != 1 || xyz.Y != 2 || xyz.Z != 3 {
		t.Errorf("XYZ = %v", xyz)
	}
}

func TestVec4XY(t *testing.T) {
	v := NewVec4(1, 2, 3, 4)
	xy := v.XY()

	if xy.X != 1 || xy.Y != 2 {
		t.Errorf("XY = %v", xy)
	}
}

func TestVec4String(t *testing.T) {
	v := NewVec4(1, 2, 3, 4)
	s := v.String()

	if s == "" {
		t.Error("String() returned empty string")
	}
}
