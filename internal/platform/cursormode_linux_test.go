//go:build linux

package platform

import (
	"testing"
	"time"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gogpu/internal/platform/wayland"
)

// newWaylandCursorTestWindow is a Wayland window whose application asked for
// mode, and the input callbacks its compositor's events reach.
func newWaylandCursorTestWindow(mode int) (*waylandPlatform, *waylandPlatformWindow, *wayland.InputCallbacks) {
	platform := &waylandPlatform{
		primary: &waylandWindow{
			cursorMode: mode,
			startTime:  time.Now(),
			events:     eventqueue.New[Event](eventqueue.DefaultCapacity),
		},
		libwl: &wayland.LibwaylandHandle{},
	}
	window := &waylandPlatformWindow{platform: platform, id: 1}
	callbacks := platform.newInputCallbacks()
	return platform, window, callbacks
}

func TestWaylandCursorModeIsTheLockTheCompositorPutsInForce(t *testing.T) {
	_, window, callbacks := newWaylandCursorTestWindow(1)

	if mode := window.CursorMode(); mode != 0 {
		t.Errorf("a lock the compositor has not activated answers %d, want Normal", mode)
	}
	callbacks.OnLockedPointerLocked()
	if mode := window.CursorMode(); mode != 1 {
		t.Errorf("a lock the compositor activated answers %d, want Locked", mode)
	}
	callbacks.OnLockedPointerUnlocked() // the window lost the focus
	if mode := window.CursorMode(); mode != 0 {
		t.Errorf("a lock the compositor deactivated answers %d, want Normal", mode)
	}
	callbacks.OnLockedPointerLocked() // and regained it
	if mode := window.CursorMode(); mode != 1 {
		t.Errorf("a lock the compositor activated again answers %d, want Locked", mode)
	}
}

func TestWaylandCursorModeIsTheConfinementTheCompositorPutsInForce(t *testing.T) {
	_, window, callbacks := newWaylandCursorTestWindow(2)

	callbacks.OnConfinedPointerConfined()
	if mode := window.CursorMode(); mode != 2 {
		t.Errorf("a confinement the compositor activated answers %d, want Confined", mode)
	}
	callbacks.OnConfinedPointerUnconfined()
	if mode := window.CursorMode(); mode != 0 {
		t.Errorf("a confinement the compositor deactivated answers %d, want Normal", mode)
	}
}

func TestWaylandCursorModeIsNormalOnceTheConstraintsAreReleased(t *testing.T) {
	platform, window, callbacks := newWaylandCursorTestWindow(1)
	callbacks.OnLockedPointerLocked()

	// The compositor sends no unlocked event for a lock its client destroys.
	platform.releasePointerConstraints()
	if mode := window.CursorMode(); mode != 0 {
		t.Errorf("a released lock answers %d, want Normal", mode)
	}
}

func TestWaylandRelativeMotionIsReportedOnlyWhileTheLockIsInForce(t *testing.T) {
	platform, _, callbacks := newWaylandCursorTestWindow(1)

	callbacks.OnRelativePointerMotion(0, 3, 4, 3, 4)
	if event, ok := platform.primary.events.Pop(); ok {
		t.Errorf("relative motion before the lock is in force reported %+v", event)
	}
	callbacks.OnLockedPointerLocked()
	callbacks.OnRelativePointerMotion(0, 3, 4, 3, 4)
	event, ok := platform.primary.events.Pop()
	if !ok || event.Type != EventPointerMove || event.Pointer.DeltaX != 3 || event.Pointer.DeltaY != 4 {
		t.Errorf("relative motion while locked reported %+v, %v, want a move by (3, 4)", event, ok)
	}
}
