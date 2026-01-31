// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package gogpu

import (
	"math"
	"testing"
	"time"

	"github.com/gogpu/gpucontext"
)

func TestGestureRecognizer_NewGestureRecognizer(t *testing.T) {
	g := NewGestureRecognizer()
	if g == nil {
		t.Fatal("NewGestureRecognizer returned nil")
	}
	if g.activePointers == nil {
		t.Error("activePointers map not initialized")
	}
	if g.NumActivePointers() != 0 {
		t.Errorf("NumActivePointers: got %d, want 0", g.NumActivePointers())
	}
}

func TestGestureRecognizer_SinglePointer(t *testing.T) {
	g := NewGestureRecognizer()

	// Single pointer down
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         100,
		Y:         100,
	})

	if g.NumActivePointers() != 1 {
		t.Errorf("NumActivePointers: got %d, want 1", g.NumActivePointers())
	}

	// EndFrame should return no gesture (need 2+ pointers)
	ev := g.EndFrame()
	if ev.NumPointers != 1 {
		t.Errorf("NumPointers: got %d, want 1", ev.NumPointers)
	}
	if ev.ZoomDelta != 1.0 {
		t.Errorf("ZoomDelta: got %f, want 1.0", ev.ZoomDelta)
	}
	if ev.PinchType != gpucontext.PinchNone {
		t.Errorf("PinchType: got %v, want PinchNone", ev.PinchType)
	}
}

func TestGestureRecognizer_TwoPointers_Zoom(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers down, 100 pixels apart
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})

	// First EndFrame establishes baseline
	ev := g.EndFrame()
	if ev.NumPointers != 2 {
		t.Errorf("NumPointers: got %d, want 2", ev.NumPointers)
	}
	// First frame after touches changed: no delta yet
	if ev.ZoomDelta != 1.0 {
		t.Errorf("ZoomDelta on first frame: got %f, want 1.0", ev.ZoomDelta)
	}

	// Move pointers apart (zoom in)
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 1,
		X:         -50,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 2,
		X:         150,
		Y:         0,
	})

	ev = g.EndFrame()
	// Distance changed from 100 to 200 = 2x zoom
	if math.Abs(ev.ZoomDelta-2.0) > 0.01 {
		t.Errorf("ZoomDelta: got %f, want ~2.0", ev.ZoomDelta)
	}
}

func TestGestureRecognizer_TwoPointers_Pan(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers down
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})

	// Establish baseline
	_ = g.EndFrame()

	// Move both pointers together (pan)
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 1,
		X:         50,
		Y:         30,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 2,
		X:         150,
		Y:         30,
	})

	ev := g.EndFrame()

	// Centroid moved from (50, 0) to (100, 30)
	// Delta should be (50, 30)
	if math.Abs(ev.TranslationDelta.X-50) > 0.01 {
		t.Errorf("TranslationDelta.X: got %f, want 50", ev.TranslationDelta.X)
	}
	if math.Abs(ev.TranslationDelta.Y-30) > 0.01 {
		t.Errorf("TranslationDelta.Y: got %f, want 30", ev.TranslationDelta.Y)
	}

	// Center should be at (100, 30)
	if math.Abs(ev.Center.X-100) > 0.01 {
		t.Errorf("Center.X: got %f, want 100", ev.Center.X)
	}
	if math.Abs(ev.Center.Y-30) > 0.01 {
		t.Errorf("Center.Y: got %f, want 30", ev.Center.Y)
	}
}

func TestGestureRecognizer_TwoPointers_Rotate(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers: one at center, one at right
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})

	// Establish baseline
	_ = g.EndFrame()

	// Rotate pointer 2 by 90 degrees (to top)
	// Pointer 1 stays, pointer 2 moves to (50, 50) - 45 degree rotation from center
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 2,
		X:         50,
		Y:         50,
	})

	ev := g.EndFrame()

	// There should be some rotation delta
	// Note: Exact value depends on implementation details
	if ev.RotationDelta == 0 {
		t.Error("RotationDelta should be non-zero after rotation")
	}
}

func TestGestureRecognizer_PinchType_Horizontal(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers spread horizontally (dx >> dy)
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         50,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100, // dx = 100
		Y:         50,  // dy = 0
	})

	ev := g.EndFrame()
	if ev.PinchType != gpucontext.PinchHorizontal {
		t.Errorf("PinchType: got %v, want PinchHorizontal", ev.PinchType)
	}
}

func TestGestureRecognizer_PinchType_Vertical(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers spread vertically (dy >> dx)
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         50,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         50,  // dx = 0
		Y:         100, // dy = 100
	})

	ev := g.EndFrame()
	if ev.PinchType != gpucontext.PinchVertical {
		t.Errorf("PinchType: got %v, want PinchVertical", ev.PinchType)
	}
}

func TestGestureRecognizer_PinchType_Proportional(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers spread diagonally (similar dx and dy)
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100, // dx = 100
		Y:         100, // dy = 100
	})

	ev := g.EndFrame()
	if ev.PinchType != gpucontext.PinchProportional {
		t.Errorf("PinchType: got %v, want PinchProportional", ev.PinchType)
	}
}

func TestGestureRecognizer_PointerUp_RemovesPointer(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers down
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})

	if g.NumActivePointers() != 2 {
		t.Errorf("NumActivePointers: got %d, want 2", g.NumActivePointers())
	}

	// One pointer up
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerUp,
		PointerID: 1,
	})

	if g.NumActivePointers() != 1 {
		t.Errorf("NumActivePointers after up: got %d, want 1", g.NumActivePointers())
	}
}

func TestGestureRecognizer_PointerCancel_RemovesPointer(t *testing.T) {
	g := NewGestureRecognizer()

	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})

	if g.NumActivePointers() != 1 {
		t.Errorf("NumActivePointers: got %d, want 1", g.NumActivePointers())
	}

	// Pointer cancel
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerCancel,
		PointerID: 1,
	})

	if g.NumActivePointers() != 0 {
		t.Errorf("NumActivePointers after cancel: got %d, want 0", g.NumActivePointers())
	}
}

func TestGestureRecognizer_Reset(t *testing.T) {
	g := NewGestureRecognizer()

	// Add some pointers
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})

	_ = g.EndFrame()

	if g.NumActivePointers() != 2 {
		t.Errorf("NumActivePointers: got %d, want 2", g.NumActivePointers())
	}

	// Reset
	g.Reset()

	if g.NumActivePointers() != 0 {
		t.Errorf("NumActivePointers after reset: got %d, want 0", g.NumActivePointers())
	}

	// EndFrame should return empty gesture
	ev := g.EndFrame()
	if ev.NumPointers != 0 {
		t.Errorf("NumPointers after reset: got %d, want 0", ev.NumPointers)
	}
}

func TestGestureRecognizer_TouchesChanged_ResetsDeltas(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers down
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})

	// Establish baseline
	_ = g.EndFrame()

	// Move pointers for zoom
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 1,
		X:         -50,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 2,
		X:         150,
		Y:         0,
	})

	ev := g.EndFrame()
	if math.Abs(ev.ZoomDelta-2.0) > 0.01 {
		t.Errorf("ZoomDelta: got %f, want ~2.0", ev.ZoomDelta)
	}

	// Add a third pointer - touches changed!
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 3,
		X:         50,
		Y:         50,
	})

	// Next frame should have no deltas because touches changed
	ev = g.EndFrame()
	if ev.ZoomDelta != 1.0 {
		t.Errorf("ZoomDelta after touch change: got %f, want 1.0", ev.ZoomDelta)
	}
}

func TestGestureRecognizer_Timestamp(t *testing.T) {
	g := NewGestureRecognizer()

	ts := 500 * time.Millisecond

	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
		Timestamp: ts,
	})

	ev := g.EndFrame()
	if ev.Timestamp != ts {
		t.Errorf("Timestamp: got %v, want %v", ev.Timestamp, ts)
	}
}

func TestGestureRecognizer_ConcurrentAccess(t *testing.T) {
	g := NewGestureRecognizer()
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			g.HandlePointer(gpucontext.PointerEvent{
				Type:      gpucontext.PointerDown,
				PointerID: i % 5,
				X:         float64(i),
				Y:         float64(i),
			})
			g.HandlePointer(gpucontext.PointerEvent{
				Type:      gpucontext.PointerUp,
				PointerID: i % 5,
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = g.EndFrame()
			_ = g.NumActivePointers()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestGestureRecognizer_ThreePointers(t *testing.T) {
	g := NewGestureRecognizer()

	// Three pointers forming a triangle
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         0,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         100,
		Y:         0,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 3,
		X:         50,
		Y:         100,
	})

	ev := g.EndFrame()
	if ev.NumPointers != 3 {
		t.Errorf("NumPointers: got %d, want 3", ev.NumPointers)
	}

	// Centroid should be at (50, 33.33)
	expectedCenterX := 50.0
	expectedCenterY := 100.0 / 3.0
	if math.Abs(ev.Center.X-expectedCenterX) > 0.01 {
		t.Errorf("Center.X: got %f, want %f", ev.Center.X, expectedCenterX)
	}
	if math.Abs(ev.Center.Y-expectedCenterY) > 0.01 {
		t.Errorf("Center.Y: got %f, want %f", ev.Center.Y, expectedCenterY)
	}
}

func TestGestureRecognizer_ZoomIn_ZoomOut(t *testing.T) {
	g := NewGestureRecognizer()

	// Two pointers
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 1,
		X:         40,
		Y:         50,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerDown,
		PointerID: 2,
		X:         60,
		Y:         50,
	})

	// Baseline
	_ = g.EndFrame()

	// Zoom OUT (fingers closer)
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 1,
		X:         45,
		Y:         50,
	})
	g.HandlePointer(gpucontext.PointerEvent{
		Type:      gpucontext.PointerMove,
		PointerID: 2,
		X:         55,
		Y:         50,
	})

	ev := g.EndFrame()
	// Original distance: 20, new distance: 10 = 0.5x zoom
	if ev.ZoomDelta >= 1.0 {
		t.Errorf("ZoomDelta for zoom out: got %f, want < 1.0", ev.ZoomDelta)
	}
}
