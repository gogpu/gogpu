//go:build linux

package platform

import (
	"testing"
	"time"
)

// TestWaylandPollEvents_ScaleChangeAloneQueuesResize models a window moved to
// an output of another scale, which the compositor reports with no configure:
// the pump queues the scale change, then a resize at the same logical size and
// the new physical size, so the window's resize callback hears of it before
// any frame is drawn.
func TestWaylandPollEvents_ScaleChangeAloneQueuesResize(t *testing.T) {
	w, _ := newWaylandPrepareFrameFixture(800, 600, 2.0, 1.0)

	scaleChanged := w.platform.PollEvents()
	resized := w.platform.PollEvents()
	rest := w.platform.PollEvents()

	if scaleChanged.Type != EventScaleChanged || scaleChanged.WindowID != 1 || scaleChanged.ScaleFactor != 2.0 ||
		scaleChanged.Width != 800 || scaleChanged.Height != 600 {
		t.Errorf("first event = %+v, want EventScaleChanged of window 1 to 2.0 at 800x600", scaleChanged)
	}
	if resized.Type != EventResize || resized.WindowID != 1 || resized.Width != 800 || resized.Height != 600 ||
		resized.PhysicalWidth != 1600 || resized.PhysicalHeight != 1200 {
		t.Errorf("second event = %+v, want EventResize of window 1 at 800x600, 1600x1200 physical", resized)
	}
	if rest.Type != EventNone {
		t.Errorf("third event = %+v, want EventNone", rest)
	}
	if r := callPrepareFrameWithTimeout(t, w, 2*time.Second); r.ScaleChanged {
		t.Error("PrepareFrame after the pump: ScaleChanged = true, want false: the pump reported it")
	}
}

// TestWaylandPollEvents_SteadyScaleQueuesNothing covers the first pump, which
// records the scale without reporting it, and the pumps after it.
func TestWaylandPollEvents_SteadyScaleQueuesNothing(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 2.0, 0)

	for i := range 2 {
		if e := w.platform.PollEvents(); e.Type != EventNone {
			t.Fatalf("pump %d: event = %+v, want EventNone", i, e)
		}
	}
	if wp.lastScale != 2.0 {
		t.Errorf("lastScale = %v, want 2.0 after the first pump", wp.lastScale)
	}
}

// TestWaylandPrepareFrame_ScaleChangeQueuesResize covers the change PrepareFrame
// sees first: it queues the same resize the pump would have.
func TestWaylandPrepareFrame_ScaleChangeQueuesResize(t *testing.T) {
	w, wp := newWaylandSecondaryPrepareFrameFixture(1024, 768, 2.0, 1.0)

	callPrepareFrameWithTimeout(t, w, 2*time.Second)

	if e, _ := wp.events.Pop(); e.Type != EventScaleChanged {
		t.Fatalf("first event = %+v, want EventScaleChanged", e)
	}
	resized, _ := wp.events.Pop()
	if resized.Type != EventResize || resized.WindowID != 2 || resized.Width != 1024 || resized.Height != 768 ||
		resized.PhysicalWidth != 2048 || resized.PhysicalHeight != 1536 {
		t.Errorf("second event = %+v, want EventResize of window 2 at 1024x768, 2048x1536 physical", resized)
	}
}
