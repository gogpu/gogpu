package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

// TestEventSourceOnResizeFiresOnWindowResize is the P2-EVENT-007 regression:
// the EventSource contract has OnResize, but nothing dispatched window resizes
// to it, so subscribers never heard about a resize (found in the #458 review).
func TestEventSourceOnResizeFiresOnWindowResize(t *testing.T) {
	mgr := &singleEventManager{
		event: platform.Event{
			Type:          platform.EventResize,
			WindowID:      0,
			Width:         800,
			Height:        600,
			PhysicalWidth: 1600, PhysicalHeight: 1200,
		},
	}
	app := &App{
		manager:     mgr,
		renderLoop:  &mockRenderLoop{},
		platWindow:  &mockWindowWithSizeMove{inSizeMove: false},
		invalidator: newInvalidator(func() {}),
	}

	var gotW, gotH int
	app.EventSource().OnResize(func(w, h int) { gotW, gotH = w, h })

	app.processEventsMultiThread()

	if gotW != 800 || gotH != 600 {
		t.Fatalf("EventSource.OnResize got (%d,%d), want (800,600): resize never reached the event source", gotW, gotH)
	}
}
