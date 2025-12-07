package gmath

import (
	"math"
	"testing"
)

const epsilon = 1e-6

func almostEqual(a, b float32) bool {
	return math.Abs(float64(a-b)) < epsilon
}

func TestNewVec2(t *testing.T) {
	v := NewVec2(3, 4)
	if v.X != 3 || v.Y != 4 {
		t.Errorf("NewVec2(3, 4) = %v, want (3, 4)", v)
	}
}

func TestZero2(t *testing.T) {
	v := Zero2()
	if v.X != 0 || v.Y != 0 {
		t.Errorf("Zero2() = %v, want (0, 0)", v)
	}
}

func TestOne2(t *testing.T) {
	v := One2()
	if v.X != 1 || v.Y != 1 {
		t.Errorf("One2() = %v, want (1, 1)", v)
	}
}

func TestVec2Add(t *testing.T) {
	v1 := NewVec2(1, 2)
	v2 := NewVec2(3, 4)
	result := v1.Add(v2)

	if result.X != 4 || result.Y != 6 {
		t.Errorf("Vec2(1,2).Add(Vec2(3,4)) = %v, want (4, 6)", result)
	}
}

func TestVec2Sub(t *testing.T) {
	v1 := NewVec2(5, 7)
	v2 := NewVec2(2, 3)
	result := v1.Sub(v2)

	if result.X != 3 || result.Y != 4 {
		t.Errorf("Vec2(5,7).Sub(Vec2(2,3)) = %v, want (3, 4)", result)
	}
}

func TestVec2Mul(t *testing.T) {
	v := NewVec2(2, 3)
	result := v.Mul(4)

	if result.X != 8 || result.Y != 12 {
		t.Errorf("Vec2(2,3).Mul(4) = %v, want (8, 12)", result)
	}
}

func TestVec2Div(t *testing.T) {
	v := NewVec2(8, 12)
	result := v.Div(4)

	if result.X != 2 || result.Y != 3 {
		t.Errorf("Vec2(8,12).Div(4) = %v, want (2, 3)", result)
	}
}

func TestVec2Dot(t *testing.T) {
	v1 := NewVec2(1, 2)
	v2 := NewVec2(3, 4)
	result := v1.Dot(v2)

	// 1*3 + 2*4 = 11
	if result != 11 {
		t.Errorf("Vec2(1,2).Dot(Vec2(3,4)) = %f, want 11", result)
	}
}

func TestVec2Length(t *testing.T) {
	v := NewVec2(3, 4)
	result := v.Length()

	// sqrt(3^2 + 4^2) = 5
	if !almostEqual(result, 5) {
		t.Errorf("Vec2(3,4).Length() = %f, want 5", result)
	}
}

func TestVec2LengthSquared(t *testing.T) {
	v := NewVec2(3, 4)
	result := v.LengthSquared()

	// 3^2 + 4^2 = 25
	if result != 25 {
		t.Errorf("Vec2(3,4).LengthSquared() = %f, want 25", result)
	}
}

func TestVec2Normalize(t *testing.T) {
	v := NewVec2(3, 4)
	result := v.Normalize()

	// Normalized (3,4) = (0.6, 0.8)
	if !almostEqual(result.X, 0.6) || !almostEqual(result.Y, 0.8) {
		t.Errorf("Vec2(3,4).Normalize() = %v, want (0.6, 0.8)", result)
	}

	// Length should be 1
	if !almostEqual(result.Length(), 1) {
		t.Errorf("Normalized vector length = %f, want 1", result.Length())
	}
}

func TestVec2NormalizeZero(t *testing.T) {
	v := Zero2()
	result := v.Normalize()

	// Normalizing zero vector should return zero
	if result.X != 0 || result.Y != 0 {
		t.Errorf("Zero2().Normalize() = %v, want (0, 0)", result)
	}
}

func TestVec2Lerp(t *testing.T) {
	v1 := NewVec2(0, 0)
	v2 := NewVec2(10, 20)

	tests := []struct {
		t        float32
		expected Vec2
	}{
		{0, NewVec2(0, 0)},
		{0.5, NewVec2(5, 10)},
		{1, NewVec2(10, 20)},
	}

	for _, tt := range tests {
		result := v1.Lerp(v2, tt.t)
		if !almostEqual(result.X, tt.expected.X) || !almostEqual(result.Y, tt.expected.Y) {
			t.Errorf("Lerp(t=%f) = %v, want %v", tt.t, result, tt.expected)
		}
	}
}

func TestVec2Distance(t *testing.T) {
	v1 := NewVec2(0, 0)
	v2 := NewVec2(3, 4)
	result := v1.Distance(v2)

	if !almostEqual(result, 5) {
		t.Errorf("Distance((0,0), (3,4)) = %f, want 5", result)
	}
}

func TestVec2Rotate(t *testing.T) {
	v := NewVec2(1, 0)

	// Rotate 90 degrees (pi/2)
	result := v.Rotate(float32(math.Pi / 2))

	if !almostEqual(result.X, 0) || !almostEqual(result.Y, 1) {
		t.Errorf("Vec2(1,0).Rotate(pi/2) = %v, want (0, 1)", result)
	}
}

func TestVec2Perpendicular(t *testing.T) {
	v := NewVec2(3, 4)
	result := v.Perpendicular()

	// Perpendicular to (3,4) is (-4,3)
	if result.X != -4 || result.Y != 3 {
		t.Errorf("Vec2(3,4).Perpendicular() = %v, want (-4, 3)", result)
	}

	// Dot product of perpendicular vectors should be 0
	dot := v.Dot(result)
	if !almostEqual(dot, 0) {
		t.Errorf("Dot product of perpendicular = %f, want 0", dot)
	}
}

func TestVec2Abs(t *testing.T) {
	v := NewVec2(-3, -4)
	result := v.Abs()

	if result.X != 3 || result.Y != 4 {
		t.Errorf("Vec2(-3,-4).Abs() = %v, want (3, 4)", result)
	}
}

func TestVec2MinMax(t *testing.T) {
	v1 := NewVec2(1, 5)
	v2 := NewVec2(3, 2)

	minResult := v1.Min(v2)
	maxResult := v1.Max(v2)

	if minResult.X != 1 || minResult.Y != 2 {
		t.Errorf("Min = %v, want (1, 2)", minResult)
	}
	if maxResult.X != 3 || maxResult.Y != 5 {
		t.Errorf("Max = %v, want (3, 5)", maxResult)
	}
}

func TestVec2Clamp(t *testing.T) {
	v := NewVec2(5, -2)
	minV := NewVec2(0, 0)
	maxV := NewVec2(3, 3)
	result := v.Clamp(minV, maxV)

	if result.X != 3 || result.Y != 0 {
		t.Errorf("Clamp = %v, want (3, 0)", result)
	}
}

func TestVec2String(t *testing.T) {
	v := NewVec2(1.5, 2.5)
	s := v.String()

	if s == "" {
		t.Error("String() returned empty string")
	}
}
