package gogpu

import (
	"reflect"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

// --- OS file drag-and-drop dispatch tests ---

// The platform layers report a file drag in progress (enter, move, leave), not
// only the drop. Each reaches its own callback, in order, with the position in
// the same units as the drop.
func TestDragEvents_ReachTheirCallbacks(t *testing.T) {
	app := newTestApp()
	var got []string
	app.OnDragEnter(func(paths []string, x, y float64) {
		got = append(got, "enter")
		if !reflect.DeepEqual(paths, []string{"/tmp/a.txt"}) || x != 10 || y != 20 {
			t.Errorf("enter = %v at %v,%v", paths, x, y)
		}
	})
	app.OnDragMove(func(x, y float64) {
		got = append(got, "move")
		if x != 30 || y != 40 {
			t.Errorf("move at %v,%v", x, y)
		}
	})
	app.OnDragLeave(func() { got = append(got, "leave") })
	app.OnDragDrop(func(paths []string, x, y float64) {
		got = append(got, "drop")
		if !reflect.DeepEqual(paths, []string{"/tmp/a.txt"}) || x != 50 || y != 60 {
			t.Errorf("drop = %v at %v,%v", paths, x, y)
		}
	})

	for _, ev := range []platform.Event{
		{Type: platform.EventDragEnter, DragPaths: []string{"/tmp/a.txt"}, DragX: 10, DragY: 20},
		{Type: platform.EventDragMove, DragX: 30, DragY: 40},
		{Type: platform.EventDragLeave},
		{Type: platform.EventDragEnter, DragPaths: []string{"/tmp/a.txt"}, DragX: 10, DragY: 20},
		{Type: platform.EventDragDrop, DragPaths: []string{"/tmp/a.txt"}, DragX: 50, DragY: 60},
	} {
		app.classifyEvent(&ev, nil, nil)
	}

	if want := []string{"enter", "move", "leave", "enter", "drop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("callbacks fired %v, want %v", got, want)
	}
	if !app.pendingRedraw.Load() {
		t.Error("a drag event must request a redraw so the app can show the drop target")
	}
}

// An application that only registers OnDragDrop keeps working as before: the
// in-progress events are ignored without a callback.
func TestDragEvents_WithoutCallbacks(t *testing.T) {
	app := newTestApp()
	dropped := false
	app.OnDragDrop(func([]string, float64, float64) { dropped = true })

	for _, ev := range []platform.Event{
		{Type: platform.EventDragEnter, DragX: 1, DragY: 2},
		{Type: platform.EventDragMove, DragX: 3, DragY: 4},
		{Type: platform.EventDragLeave},
		{Type: platform.EventDragDrop, DragPaths: []string{"/tmp/b"}},
	} {
		app.classifyEvent(&ev, nil, nil)
	}
	if !dropped {
		t.Fatal("the drop callback must still fire")
	}
}
