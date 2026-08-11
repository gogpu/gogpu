package compositor

import (
	"image"
	"testing"
)

func TestComputeDamageScissor(t *testing.T) {
	tests := []struct {
		name      string
		groupClip *[4]uint32
		surfaceW  uint32
		surfaceH  uint32
		damage    image.Rectangle
		wantX     uint32
		wantY     uint32
		wantW     uint32
		wantH     uint32
		wantValid bool
	}{
		{
			name:      "damage only, no group clip",
			groupClip: nil,
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458),
			wantX:  170, wantY: 410, wantW: 48, wantH: 48,
			wantValid: true,
		},
		{
			name:      "damage intersects group clip",
			groupClip: &[4]uint32{150, 400, 100, 100},
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458),
			wantX:  170, wantY: 410, wantW: 48, wantH: 48,
			wantValid: true,
		},
		{
			name:      "group clip smaller than damage",
			groupClip: &[4]uint32{180, 420, 20, 20},
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458),
			wantX:  180, wantY: 420, wantW: 20, wantH: 20,
			wantValid: true,
		},
		{
			name:      "group clip outside damage",
			groupClip: &[4]uint32{0, 0, 50, 50},
			surfaceW:  800, surfaceH: 600,
			damage:    image.Rect(170, 410, 218, 458),
			wantValid: false,
		},
		{
			name:      "damage clamped to surface bounds",
			groupClip: nil,
			surfaceW:  200, surfaceH: 200,
			damage: image.Rect(170, 180, 300, 300),
			wantX:  170, wantY: 180, wantW: 30, wantH: 20,
			wantValid: true,
		},
		{
			name:      "damage fully outside surface",
			groupClip: nil,
			surfaceW:  100, surfaceH: 100,
			damage:    image.Rect(200, 200, 300, 300),
			wantValid: false,
		},
		{
			name:      "zero-size damage",
			groupClip: nil,
			surfaceW:  800, surfaceH: 600,
			damage:    image.Rect(100, 100, 100, 100),
			wantValid: false,
		},
		{
			name:      "negative coords clamped to 0",
			groupClip: nil,
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(-10, -10, 50, 50),
			wantX:  0, wantY: 0, wantW: 50, wantH: 50,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, w, h, valid := ComputeDamageScissor(tt.groupClip, tt.surfaceW, tt.surfaceH, tt.damage)
			if valid != tt.wantValid {
				t.Fatalf("valid = %v, want %v", valid, tt.wantValid)
			}
			if !valid {
				return
			}
			if x != tt.wantX || y != tt.wantY || w != tt.wantW || h != tt.wantH {
				t.Errorf("scissor = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
					x, y, w, h, tt.wantX, tt.wantY, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestDamageRectsUnion(t *testing.T) {
	tests := []struct {
		name  string
		rects []image.Rectangle
		want  image.Rectangle
	}{
		{"empty", nil, image.Rectangle{}},
		{"single", []image.Rectangle{image.Rect(10, 20, 50, 60)}, image.Rect(10, 20, 50, 60)},
		{
			"two_distant",
			[]image.Rectangle{image.Rect(10, 10, 58, 58), image.Rect(500, 50, 600, 82)},
			image.Rect(10, 10, 600, 82),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DamageRectsUnion(tt.rects)
			if got != tt.want {
				t.Errorf("DamageRectsUnion = %v, want %v", got, tt.want)
			}
		})
	}
}
