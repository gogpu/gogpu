//go:build linux

package platform

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/x11"
)

// TestTranslateX11Event_StampsWindowID guards against the regression this
// fix addresses: App.dispatchKeyEvent/dispatchPointerEvent/dispatchScrollEvent/
// dispatchCharEvent (app.go) route events via windowManager.getByPlatformID,
// which only resolves a *Window for a nonzero, registered WindowID. Before
// this fix, translateX11Event's predecessor left WindowID at its zero value
// for every case except Close/Expose, so per-window callbacks (SetOnKeyPress
// etc.) were silently never invoked for keyboard/pointer/scroll/char/focus
// events on either the primary or a secondary window.
func TestTranslateX11Event_StampsWindowID(t *testing.T) {
	const wantID = WindowID(7)

	cases := []struct {
		name string
		in   x11.PlatformEvent
		want EventType
	}{
		{"close", x11.PlatformEvent{Type: x11.EventTypeClose}, EventClose},
		{"focus", x11.PlatformEvent{Type: x11.EventTypeFocus, Focused: true}, EventFocus},
		{"keydown", x11.PlatformEvent{Type: x11.EventTypeKeyDown}, EventKeyDown},
		{"keyup", x11.PlatformEvent{Type: x11.EventTypeKeyUp}, EventKeyUp},
		{"char", x11.PlatformEvent{Type: x11.EventTypeChar, Char: 'a'}, EventChar},
		{"pointerdown", x11.PlatformEvent{Type: x11.EventTypePointerDown}, EventPointerDown},
		{"pointerup", x11.PlatformEvent{Type: x11.EventTypePointerUp}, EventPointerUp},
		{"pointermove", x11.PlatformEvent{Type: x11.EventTypePointerMove}, EventPointerMove},
		{"pointerenter", x11.PlatformEvent{Type: x11.EventTypePointerEnter}, EventPointerEnter},
		{"pointerleave", x11.PlatformEvent{Type: x11.EventTypePointerLeave}, EventPointerLeave},
		{"scroll", x11.PlatformEvent{Type: x11.EventTypeScroll}, EventScroll},
		{"expose", x11.PlatformEvent{Type: x11.EventTypeExpose}, EventExpose},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateX11Event(tc.in, nil, wantID)
			if got.Type != tc.want {
				t.Fatalf("Type = %v, want %v", got.Type, tc.want)
			}
			if got.WindowID != wantID {
				t.Fatalf("WindowID = %v, want %v (event type %s would silently miss getByPlatformID)", got.WindowID, wantID, tc.name)
			}
		})
	}
}

// TestTranslateX11Event_DistinctWindowIDsPerWindow guards specifically
// against the secondary-window case: two windows' events must carry their
// own distinct WindowID, not both collapse to zero or to the primary's ID.
func TestTranslateX11Event_DistinctWindowIDsPerWindow(t *testing.T) {
	primary := translateX11Event(x11.PlatformEvent{Type: x11.EventTypeKeyDown}, nil, WindowID(1))
	secondary := translateX11Event(x11.PlatformEvent{Type: x11.EventTypeKeyDown}, nil, WindowID(2))

	if primary.WindowID == secondary.WindowID {
		t.Fatalf("primary and secondary events both got WindowID %v; secondary window input would be indistinguishable from primary's", primary.WindowID)
	}
	if primary.WindowID != 1 || secondary.WindowID != 2 {
		t.Fatalf("got primary=%v secondary=%v, want primary=1 secondary=2", primary.WindowID, secondary.WindowID)
	}
}
