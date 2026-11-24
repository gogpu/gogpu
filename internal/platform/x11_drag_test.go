//go:build linux

package platform

import (
	"slices"
	"testing"

	"github.com/gogpu/gogpu/internal/platform/x11"
)

// TestTranslateX11Event_Drag pins how translateX11Event carries an XDND drag
// into a platform.Event, the link between the X11 drag state machine
// (x11/xdnd.go) and App.OnDragEnter / OnDragMove / OnDragDrop / OnDragLeave:
//
//   - enter and drop carry the paths and the position;
//   - move carries the position only;
//   - leave carries neither.
//
// The position passes through unchanged: X11 reports it in physical pixels,
// and the translation does not scale it (unlike EventTypeResize). Every input
// below carries paths and a position, so a case that should drop one of them
// fails if it leaks through.
func TestTranslateX11Event_Drag(t *testing.T) {
	const wantID = WindowID(7)
	paths := []string{"/home/user/a.txt", "/home/user/b c.png"}

	cases := []struct {
		name      string
		in        x11.EventType
		want      EventType
		wantPaths []string
		wantX     float64
		wantY     float64
	}{
		{"enter", x11.EventTypeDragEnter, EventDragEnter, paths, 120.5, 48.25},
		{"move", x11.EventTypeDragMove, EventDragMove, nil, 120.5, 48.25},
		{"drop", x11.EventTypeDragDrop, EventDragDrop, paths, 120.5, 48.25},
		{"leave", x11.EventTypeDragLeave, EventDragLeave, nil, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := x11.PlatformEvent{Type: tc.in, DragPaths: paths, DragX: 120.5, DragY: 48.25}

			// inner is nil: the drag cases must not need the X11 connection.
			got := translateX11Event(in, nil, wantID)

			if got.Type != tc.want {
				t.Fatalf("Type = %v, want %v", got.Type, tc.want)
			}
			if got.WindowID != wantID {
				t.Errorf("WindowID = %v, want %v", got.WindowID, wantID)
			}
			if !slices.Equal(got.DragPaths, tc.wantPaths) {
				t.Errorf("DragPaths = %q, want %q", got.DragPaths, tc.wantPaths)
			}
			if got.DragX != tc.wantX || got.DragY != tc.wantY {
				t.Errorf("position = (%v, %v), want (%v, %v)", got.DragX, got.DragY, tc.wantX, tc.wantY)
			}
		})
	}
}
