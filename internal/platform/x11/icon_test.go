//go:build linux

package x11

import (
	"image"
	"image/color"
	"testing"
)

// TestSetNetWMIcon_NoOp verifies the early-return paths that need no X server.
func TestSetNetWMIcon_NoOp(t *testing.T) {
	atoms := &StandardAtoms{NetWMIcon: AtomNone}

	// nil img with AtomNone → nil error, no panic
	if err := (*Connection)(nil).SetNetWMIcon(0, atoms, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// non-nil img but AtomNone → still returns early
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if err := (*Connection)(nil).SetNetWMIcon(0, atoms, img); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// zero-size image with a valid atom → returns early before any X call
	atoms2 := &StandardAtoms{NetWMIcon: Atom(1)}
	empty := image.NewNRGBA(image.Rect(0, 0, 0, 0))
	if err := (*Connection)(nil).SetNetWMIcon(0, atoms2, empty); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSetNetWMIcon_E2E creates a real X11 window, sets _NET_WM_ICON, reads it
// back with GetProperty and verifies the ARGB encoding round-trips correctly.
// Skips when no X server is reachable.
func TestSetNetWMIcon_E2E(t *testing.T) {
	conn, err := Connect()
	if err != nil {
		t.Skipf("no X server: %v", err)
	}
	defer conn.Close()

	atoms, err := conn.InternStandardAtoms()
	if err != nil {
		t.Fatalf("InternStandardAtoms: %v", err)
	}
	if atoms.NetWMIcon == AtomNone {
		t.Skip("_NET_WM_ICON atom not supported by this X server")
	}

	window, err := conn.CreateWindow(WindowConfig{Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("CreateWindow: %v", err)
	}
	defer conn.DestroyWindow(window)

	// 2×2 image with four distinct cases: opaque, semi-transparent, transparent, mixed.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})   // opaque red
	img.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 255, B: 0, A: 128})   // half-alpha green
	img.SetNRGBA(0, 1, color.NRGBA{R: 0, G: 0, B: 255, A: 0})     // fully transparent
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, B: 0, A: 200}) // yellow, partial alpha

	if err := conn.SetNetWMIcon(window, atoms, img); err != nil {
		t.Fatalf("SetNetWMIcon: %v", err)
	}

	// Read back: 2 header words + 4 pixels = 6 CARDINAL elements.
	data, actualType, format, err := conn.GetProperty(window, atoms.NetWMIcon, AtomCardinal, 0, 6, false)
	if err != nil {
		t.Fatalf("GetProperty: %v", err)
	}
	if actualType == AtomNone {
		t.Fatal("_NET_WM_ICON property not set after SetNetWMIcon")
	}
	if format != 32 {
		t.Errorf("format = %d, want 32", format)
	}
	if len(data) < 6*4 {
		t.Fatalf("data too short: %d bytes, want at least 24", len(data))
	}

	get := func(off int) uint32 {
		return uint32(data[off]) | uint32(data[off+1])<<8 |
			uint32(data[off+2])<<16 | uint32(data[off+3])<<24
	}

	if got := get(0); got != 2 {
		t.Errorf("width = %d, want 2", got)
	}
	if got := get(4); got != 2 {
		t.Errorf("height = %d, want 2", got)
	}

	tests := []struct {
		name string
		off  int
		want uint32
	}{
		{"(0,0) opaque red", 8, 0xFFFF0000},
		{"(1,0) half-alpha green", 12, 0x8000FF00},
		{"(0,1) transparent", 16, 0x00000000},
		{"(1,1) yellow A=200", 20, 0xC8FFFF00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := get(tt.off); got != tt.want {
				t.Errorf("pixel = %08x, want %08x", got, tt.want)
			}
		})
	}
}
