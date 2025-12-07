package gmath

import (
	"testing"
)

func TestNewColor(t *testing.T) {
	c := NewColor(0.1, 0.2, 0.3, 0.4)
	if c.R != 0.1 || c.G != 0.2 || c.B != 0.3 || c.A != 0.4 {
		t.Errorf("NewColor = %v, want (0.1, 0.2, 0.3, 0.4)", c)
	}
}

func TestRGB(t *testing.T) {
	c := RGB(0.5, 0.6, 0.7)
	if c.R != 0.5 || c.G != 0.6 || c.B != 0.7 || c.A != 1 {
		t.Errorf("RGB = %v, want (0.5, 0.6, 0.7, 1)", c)
	}
}

func TestRGBA(t *testing.T) {
	c := RGBA(0.1, 0.2, 0.3, 0.4)
	if c.R != 0.1 || c.G != 0.2 || c.B != 0.3 || c.A != 0.4 {
		t.Errorf("RGBA = %v, want (0.1, 0.2, 0.3, 0.4)", c)
	}
}

func TestHexRGB(t *testing.T) {
	// Test RGB hex (0xRRGGBB)
	c := Hex(0xFF8000) // Orange

	if !almostEqual(c.R, 1.0) {
		t.Errorf("Hex R = %f, want 1.0", c.R)
	}
	if !almostEqual(c.G, 0.5019608) { // 128/255
		t.Errorf("Hex G = %f, want ~0.502", c.G)
	}
	if !almostEqual(c.B, 0) {
		t.Errorf("Hex B = %f, want 0", c.B)
	}
	if c.A != 1 {
		t.Errorf("Hex A = %f, want 1", c.A)
	}
}

func TestHexRGBA(t *testing.T) {
	// Test RGBA hex (0xRRGGBBAA)
	c := Hex(0xFF000080) // Red with 50% alpha

	if !almostEqual(c.R, 1.0) {
		t.Errorf("Hex R = %f, want 1.0", c.R)
	}
	if !almostEqual(c.G, 0) {
		t.Errorf("Hex G = %f, want 0", c.G)
	}
	if !almostEqual(c.B, 0) {
		t.Errorf("Hex B = %f, want 0", c.B)
	}
	if !almostEqual(c.A, 0.5019608) { // 128/255
		t.Errorf("Hex A = %f, want ~0.502", c.A)
	}
}

func TestHexBlack(t *testing.T) {
	c := Hex(0x000000)
	if c.R != 0 || c.G != 0 || c.B != 0 || c.A != 1 {
		t.Errorf("Hex(0x000000) = %v, want black", c)
	}
}

func TestHexWhite(t *testing.T) {
	c := Hex(0xFFFFFF)
	if c.R != 1 || c.G != 1 || c.B != 1 || c.A != 1 {
		t.Errorf("Hex(0xFFFFFF) = %v, want white", c)
	}
}

func TestColorToVec4(t *testing.T) {
	c := NewColor(0.1, 0.2, 0.3, 0.4)
	v := c.ToVec4()

	if v.X != 0.1 || v.Y != 0.2 || v.Z != 0.3 || v.W != 0.4 {
		t.Errorf("ToVec4() = %v, want (0.1, 0.2, 0.3, 0.4)", v)
	}
}

func TestColorLerp(t *testing.T) {
	c1 := Black
	c2 := White

	tests := []struct {
		t        float32
		expected Color
	}{
		{0, Black},
		{0.5, Gray},
		{1, White},
	}

	for _, tt := range tests {
		result := c1.Lerp(c2, tt.t)
		if !almostEqual(result.R, tt.expected.R) ||
			!almostEqual(result.G, tt.expected.G) ||
			!almostEqual(result.B, tt.expected.B) {
			t.Errorf("Lerp(t=%f) = %v, want %v", tt.t, result, tt.expected)
		}
	}
}

func TestColorWithAlpha(t *testing.T) {
	c := Red.WithAlpha(0.5)

	if c.R != 1 || c.G != 0 || c.B != 0 || c.A != 0.5 {
		t.Errorf("Red.WithAlpha(0.5) = %v", c)
	}
}

func TestColorPremultiply(t *testing.T) {
	c := NewColor(1, 0.5, 0.25, 0.5)
	p := c.Premultiply()

	if !almostEqual(p.R, 0.5) || !almostEqual(p.G, 0.25) || !almostEqual(p.B, 0.125) || !almostEqual(p.A, 0.5) {
		t.Errorf("Premultiply() = %v, want (0.5, 0.25, 0.125, 0.5)", p)
	}
}

func TestColorString(t *testing.T) {
	c := Red
	s := c.String()

	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestPredefinedColors(t *testing.T) {
	tests := []struct {
		name       string
		color      Color
		r, g, b, a float32
	}{
		{"Black", Black, 0, 0, 0, 1},
		{"White", White, 1, 1, 1, 1},
		{"Red", Red, 1, 0, 0, 1},
		{"Green", Green, 0, 1, 0, 1},
		{"Blue", Blue, 0, 0, 1, 1},
		{"Yellow", Yellow, 1, 1, 0, 1},
		{"Cyan", Cyan, 0, 1, 1, 1},
		{"Magenta", Magenta, 1, 0, 1, 1},
		{"Transparent", Transparent, 0, 0, 0, 0},
		{"Gray", Gray, 0.5, 0.5, 0.5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.R != tt.r || tt.color.G != tt.g || tt.color.B != tt.b || tt.color.A != tt.a {
				t.Errorf("%s = %v, want (%f, %f, %f, %f)", tt.name, tt.color, tt.r, tt.g, tt.b, tt.a)
			}
		})
	}
}

func TestGopherBlue(t *testing.T) {
	// Gopher blue is #00AFD7
	c := GopherBlue

	// R = 0x00/255 = 0
	if c.R != 0 {
		t.Errorf("GopherBlue.R = %f, want 0", c.R)
	}

	// G = 0xAF/255 = 175/255 ≈ 0.686
	if !almostEqual(c.G, 0.686) {
		t.Errorf("GopherBlue.G = %f, want ~0.686", c.G)
	}

	// B = 0xD7/255 = 215/255 ≈ 0.843
	if !almostEqual(c.B, 0.843) {
		t.Errorf("GopherBlue.B = %f, want ~0.843", c.B)
	}
}
