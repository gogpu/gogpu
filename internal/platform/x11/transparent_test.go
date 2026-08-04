//go:build linux

package x11

import (
	"bytes"
	"testing"
)

// TestFindARGBVisual_SelectsDepth32 verifies that a 32-bit TrueColor visual
// with an unoccupied alpha byte is selected over the root visual.
func TestFindARGBVisual_SelectsDepth32(t *testing.T) {
	screen := &ScreenInfo{
		RootVisual: 0x10,
		RootDepth:  24,
		Depths: []DepthInfo{
			{
				Depth: 24,
				Visuals: []VisualType{
					{VisualID: 0x10, Class: classTrueColor, RedMask: 0xFF0000, GreenMask: 0x00FF00, BlueMask: 0x0000FF},
				},
			},
			{
				Depth: 32,
				Visuals: []VisualType{
					{VisualID: 0x20, Class: classTrueColor, RedMask: 0x00FF0000, GreenMask: 0x0000FF00, BlueMask: 0x000000FF},
				},
			},
		},
	}

	visual, depth, ok := findARGBVisual(screen)
	if !ok {
		t.Fatal("findARGBVisual returned ok=false, want an ARGB visual")
	}
	if visual.VisualID != 0x20 {
		t.Errorf("VisualID = %#x, want 0x20", visual.VisualID)
	}
	if depth != 32 {
		t.Errorf("depth = %d, want 32", depth)
	}
}

// TestFindARGBVisual_NoARGBReturnsFalse verifies the fallback path when the
// screen has no usable 32-bit visual.
func TestFindARGBVisual_NoARGBReturnsFalse(t *testing.T) {
	screen := &ScreenInfo{
		RootVisual: 0x10,
		RootDepth:  24,
		Depths: []DepthInfo{
			{
				Depth: 24,
				Visuals: []VisualType{
					{VisualID: 0x10, Class: classTrueColor, RedMask: 0xFF0000, GreenMask: 0x00FF00, BlueMask: 0x0000FF},
				},
			},
			{
				Depth: 32,
				Visuals: []VisualType{
					// Masks cover all 32 bits — no alpha byte.
					{VisualID: 0x21, Class: classTrueColor, RedMask: 0xFF000000, GreenMask: 0x00FF0000, BlueMask: 0x0000FFFF},
				},
			},
		},
	}

	if _, _, ok := findARGBVisual(screen); ok {
		t.Error("findARGBVisual returned ok=true, want false for a visual without alpha")
	}
	if _, _, ok := findARGBVisual(nil); ok {
		t.Error("findARGBVisual(nil) returned ok=true, want false")
	}
}

// TestCreateColormapRequest_LittleEndian verifies the wire encoding of the
// CreateColormap request on a little-endian connection.
func TestCreateColormapRequest_LittleEndian(t *testing.T) {
	got := createColormapRequest(LSBFirst, 0x11223344, 0x55667788, 0x99AABBCC, allocNone)
	want := []byte{
		78, 0, 5, 0, // opcode, unused, length
		0x44, 0x33, 0x22, 0x11, // colormap
		0x88, 0x77, 0x66, 0x55, // window
		0xCC, 0xBB, 0xAA, 0x99, // visual
		0,       // alloc
		0, 0, 0, // pad to 20 bytes
	}
	if !bytes.Equal(got, want) {
		t.Errorf("little-endian request = %v, want %v", got, want)
	}
}

// TestCreateColormapRequest_BigEndian verifies the wire encoding of the
// CreateColormap request on a big-endian connection.
func TestCreateColormapRequest_BigEndian(t *testing.T) {
	got := createColormapRequest(MSBFirst, 0x11223344, 0x55667788, 0x99AABBCC, allocNone)
	want := []byte{
		78, 0, 0, 5, // opcode, unused, length
		0x11, 0x22, 0x33, 0x44, // colormap
		0x55, 0x66, 0x77, 0x88, // window
		0x99, 0xAA, 0xBB, 0xCC, // visual
		0,       // alloc
		0, 0, 0, // pad to 20 bytes
	}
	if !bytes.Equal(got, want) {
		t.Errorf("big-endian request = %v, want %v", got, want)
	}
}
