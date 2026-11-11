//go:build linux

package platform

import (
	"testing"
	"time"

	"github.com/gogpu/gpucontext"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gogpu/internal/platform/wayland"
)

// newWaylandScrollTestWindow is a Wayland window under the pointer.
func newWaylandScrollTestWindow() *waylandWindow {
	return &waylandWindow{
		pointerIn: true,
		startTime: time.Now(),
		events:    eventqueue.New[Event](eventqueue.DefaultCapacity),
	}
}

// popScroll answers the next scroll event w queued.
func popScroll(t *testing.T, w *waylandWindow) gpucontext.ScrollEvent {
	t.Helper()
	event, ok := w.events.Pop()
	if !ok || event.Type != EventScroll {
		t.Fatalf("queued %+v (%v), want a scroll", event, ok)
	}
	return event.Scroll
}

func TestWaylandAWheelNotchScrollsOneLine(t *testing.T) {
	w := newWaylandScrollTestWindow()

	w.holdScrollNotches(wayland.PointerAxisVerticalScroll, 1)
	w.dispatchPointerAxis(0, wayland.PointerAxisVerticalScroll, 15)
	w.holdScrollNotches(wayland.PointerAxisHorizontalScroll, -1)
	w.dispatchPointerAxis(0, wayland.PointerAxisHorizontalScroll, -15)

	if scroll := popScroll(t, w); scroll.DeltaY != 1 || scroll.DeltaX != 0 || scroll.DeltaMode != gpucontext.ScrollDeltaLine {
		t.Errorf("a notch down scrolled %+v, want one line down", scroll)
	}
	if scroll := popScroll(t, w); scroll.DeltaX != -1 || scroll.DeltaY != 0 || scroll.DeltaMode != gpucontext.ScrollDeltaLine {
		t.Errorf("a notch left scrolled %+v, want one line left", scroll)
	}
}

func TestWaylandATouchpadScrollsInPoints(t *testing.T) {
	w := newWaylandScrollTestWindow()

	w.dispatchPointerAxis(0, wayland.PointerAxisVerticalScroll, 7.5)

	if scroll := popScroll(t, w); scroll.DeltaY != 7.5 || scroll.DeltaMode != gpucontext.ScrollDeltaPixel {
		t.Errorf("a touchpad scrolled %+v, want 7.5 points down", scroll)
	}
}

func TestWaylandNotchesScrollOnlyTheAxisEventTheyPrecede(t *testing.T) {
	w := newWaylandScrollTestWindow()

	w.holdScrollNotches(wayland.PointerAxisHorizontalScroll, 2)
	w.dispatchPointerAxis(0, wayland.PointerAxisVerticalScroll, 4)
	w.dispatchPointerAxis(0, wayland.PointerAxisHorizontalScroll, 30)
	w.dispatchPointerAxis(0, wayland.PointerAxisHorizontalScroll, 6)

	if scroll := popScroll(t, w); scroll.DeltaY != 4 || scroll.DeltaMode != gpucontext.ScrollDeltaPixel {
		t.Errorf("a vertical scroll after a horizontal notch scrolled %+v, want 4 points", scroll)
	}
	if scroll := popScroll(t, w); scroll.DeltaX != 2 || scroll.DeltaMode != gpucontext.ScrollDeltaLine {
		t.Errorf("the horizontal axis event after two notches scrolled %+v, want two lines", scroll)
	}
	if scroll := popScroll(t, w); scroll.DeltaX != 6 || scroll.DeltaMode != gpucontext.ScrollDeltaPixel {
		t.Errorf("the next horizontal axis event scrolled %+v, want 6 points", scroll)
	}
}
