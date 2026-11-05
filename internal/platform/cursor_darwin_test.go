//go:build darwin

package platform

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// fakeCursorSystem records what darwinCursor asks of macOS. Coordinates are
// Cocoa screen coordinates, as in the real system: origin bottom-left, y up.
type fakeCursorSystem struct {
	focused    bool
	content    darwin.NSRect
	location   darwin.NSPoint
	associated bool
	hideCount  int // hides minus unhides, as NSCursor counts them
	warpCount  int
}

func (s *fakeCursorSystem) Hide()                      { s.hideCount++ }
func (s *fakeCursorSystem) Unhide()                    { s.hideCount-- }
func (s *fakeCursorSystem) Associate(associated bool)  { s.associated = associated }
func (s *fakeCursorSystem) Location() darwin.NSPoint   { return s.location }
func (s *fakeCursorSystem) Focused() bool              { return s.focused }
func (s *fakeCursorSystem) ContentRect() darwin.NSRect { return s.content }

func (s *fakeCursorSystem) Warp(point darwin.NSPoint) {
	s.location = point
	s.warpCount++
}

// newFocusedCursorSystem is an 800x600 content area at (100, 200) on screen,
// focused, with the cursor at (150, 250): inside, near the bottom-left corner.
func newFocusedCursorSystem() *fakeCursorSystem {
	return &fakeCursorSystem{
		focused: true,
		content: darwin.NSRect{
			Origin: darwin.NSPoint{X: 100, Y: 200},
			Size:   darwin.NSSize{Width: 800, Height: 600},
		},
		location:   darwin.NSPoint{X: 150, Y: 250},
		associated: true,
	}
}

func TestDarwinCursorLockHidesDetachesAndCentres(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}

	cursor.setMode(cursorModeLocked)

	if system.hideCount != 1 {
		t.Errorf("hideCount = %d, want 1", system.hideCount)
	}
	if system.associated {
		t.Error("cursor still associated with the mouse")
	}
	if want := (darwin.NSPoint{X: 500, Y: 500}); system.location != want {
		t.Errorf("location = %v, want the content centre %v", system.location, want)
	}
	if got := cursor.mode(); got != cursorModeLocked {
		t.Errorf("mode() = %d, want Locked", got)
	}
}

func TestDarwinCursorLockedMoveReportsDeltaAtFrozenPosition(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)

	// The warp from (150, 250) to the centre (500, 500) shows up in the next
	// event's delta: +350 across, and 250 up, which is -250 in y-down deltas.
	motion, reported := cursor.move(400, 300, 350+5, -250-3)
	if !reported {
		t.Fatal("first locked move not reported")
	}
	if want := (pointerMotion{X: 400, Y: 300, DeltaX: 5, DeltaY: -3}); motion != want {
		t.Errorf("first move = %+v, want %+v", motion, want)
	}

	motion, reported = cursor.move(123, 456, 2, 1)
	if !reported {
		t.Fatal("second locked move not reported")
	}
	if want := (pointerMotion{X: 400, Y: 300, DeltaX: 2, DeltaY: 1}); motion != want {
		t.Errorf("second move = %+v, want %+v", motion, want)
	}

	if _, reported := cursor.move(400, 300, 0, 0); reported {
		t.Error("a locked move without motion was reported")
	}
}

func TestDarwinCursorUnlockRestoresCursor(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)

	cursor.setMode(cursorModeNormal)

	if system.hideCount != 0 {
		t.Errorf("hideCount = %d, want 0", system.hideCount)
	}
	if !system.associated {
		t.Error("cursor left detached from the mouse")
	}
	if want := (darwin.NSPoint{X: 150, Y: 250}); system.location != want {
		t.Errorf("location = %v, want it back at %v", system.location, want)
	}
	if got := cursor.mode(); got != cursorModeNormal {
		t.Errorf("mode() = %d, want Normal", got)
	}
}

func TestDarwinCursorSetModeTwiceIsNoOp(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)
	warpCount := system.warpCount

	cursor.setMode(cursorModeLocked)

	if system.hideCount != 1 {
		t.Errorf("hideCount = %d, want 1", system.hideCount)
	}
	if system.warpCount != warpCount {
		t.Errorf("warped again: %d warps, want %d", system.warpCount, warpCount)
	}
}

func TestDarwinCursorConfineBringsCursorInside(t *testing.T) {
	system := newFocusedCursorSystem()
	system.location = darwin.NSPoint{X: 50, Y: 900} // left of and above the content
	cursor := darwinCursor{system: system}

	cursor.setMode(cursorModeConfined)

	// Window (0, 0), the content's top-left point, is screen (100, 800).
	if want := (darwin.NSPoint{X: 100, Y: 800}); system.location != want {
		t.Errorf("location = %v, want %v", system.location, want)
	}
	if system.hideCount != 0 {
		t.Errorf("hideCount = %d, want the cursor visible", system.hideCount)
	}
	if system.associated {
		t.Error("confined cursor still moved by the system")
	}
	if got := cursor.mode(); got != cursorModeConfined {
		t.Errorf("mode() = %d, want Confined", got)
	}
}

func TestDarwinCursorConfineInsideDetachesWithoutMoving(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}

	cursor.setMode(cursorModeConfined)

	if want := (darwin.NSPoint{X: 150, Y: 250}); system.location != want {
		t.Errorf("location = %v, want the cursor left at %v", system.location, want)
	}
	if system.associated {
		t.Error("confined cursor still moved by the system")
	}
}

func TestDarwinCursorConfinedMoveFollowsDeltaWithinContent(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeConfined)

	// The cursor starts at screen (150, 250): window (50, 550). Window
	// coordinates have their origin at the content's top-left, y down.
	motion, reported := cursor.move(50, 550, 800, -600)
	if !reported {
		t.Fatal("confined move not reported")
	}
	if want := (pointerMotion{X: 799, Y: 0}); motion != want {
		t.Errorf("move = %+v, want it clamped to %+v", motion, want)
	}
	// Window (799, 0) is the content's top-right point: screen (899, 800).
	if want := (darwin.NSPoint{X: 899, Y: 800}); system.location != want {
		t.Errorf("location = %v, want the cursor moved to %v", system.location, want)
	}

	// The next event sits where the warp put the cursor, and its delta
	// carries the warp: 749 right, 550 up.
	motion, reported = cursor.move(799, 0, 749-789, -550+20)
	if !reported {
		t.Fatal("second confined move not reported")
	}
	if want := (pointerMotion{X: 10, Y: 20}); motion != want {
		t.Errorf("second move = %+v, want %+v", motion, want)
	}
	if want := (darwin.NSPoint{X: 110, Y: 780}); system.location != want {
		t.Errorf("location = %v, want %v", system.location, want)
	}
}

func TestDarwinCursorConfinedMoveAgainstEdgeIsNotReported(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeConfined)
	cursor.move(50, 550, -50, 0) // to the left edge, window (0, 550)
	warpCount := system.warpCount

	if _, reported := cursor.move(0, 550, -50-10, 0); reported {
		t.Error("a move into the edge was reported")
	}
	if system.warpCount != warpCount {
		t.Error("a move into the edge warped the cursor")
	}
}

func TestDarwinCursorEventBeforeWarpKeepsItsDelta(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeConfined)

	// Two events queued before any warp, both found the cursor at (50, 550).
	cursor.move(50, 550, 10, 0) // warps to (60, 550)
	motion, _ := cursor.move(50, 550, 5, 0)
	if want := (pointerMotion{X: 65, Y: 550}); motion != want {
		t.Errorf("queued move = %+v, want %+v", motion, want)
	}

	// The next event has seen both warps: 10 then 5 to the right.
	motion, _ = cursor.move(65, 550, 15+1, 0)
	if want := (pointerMotion{X: 66, Y: 550}); motion != want {
		t.Errorf("move after the warps = %+v, want %+v", motion, want)
	}
}

func TestDarwinCursorLockedEventBeforeLockKeepsItsDelta(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)

	// Queued before the lock's warp: the cursor was still at window (50, 550).
	motion, _ := cursor.move(50, 550, 3, 4)
	if want := (pointerMotion{X: 400, Y: 300, DeltaX: 3, DeltaY: 4}); motion != want {
		t.Errorf("queued move = %+v, want %+v", motion, want)
	}

	motion, _ = cursor.move(400, 300, 350+1, -250+1)
	if want := (pointerMotion{X: 400, Y: 300, DeltaX: 1, DeltaY: 1}); motion != want {
		t.Errorf("move after the lock = %+v, want %+v", motion, want)
	}
}

func TestDarwinCursorNormalMovePassesThrough(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}

	motion, reported := cursor.move(-30, 900, 4, -2)

	if !reported {
		t.Fatal("normal move not reported")
	}
	if want := (pointerMotion{X: -30, Y: 900}); motion != want {
		t.Errorf("move = %+v, want %+v", motion, want)
	}
	if system.warpCount != 0 {
		t.Error("a normal move warped the cursor")
	}
}

func TestDarwinCursorFocusLossReleasesAndFocusRestores(t *testing.T) {
	for _, mode := range []int{cursorModeLocked, cursorModeConfined} {
		system := newFocusedCursorSystem()
		cursor := darwinCursor{system: system}
		cursor.setMode(mode)

		system.focused = false
		cursor.update()

		if system.hideCount != 0 {
			t.Errorf("mode %d unfocused: hideCount = %d, want 0", mode, system.hideCount)
		}
		if !system.associated {
			t.Errorf("mode %d unfocused: cursor detached from the mouse", mode)
		}
		if got := cursor.mode(); got != cursorModeNormal {
			t.Errorf("mode %d unfocused: mode() = %d, want Normal", mode, got)
		}
		motion, _ := cursor.move(-30, 900, 4, -2)
		if want := (pointerMotion{X: -30, Y: 900}); motion != want {
			t.Errorf("mode %d unfocused: move = %+v, want %+v", mode, motion, want)
		}

		system.focused = true
		cursor.update()

		if got := cursor.mode(); got != mode {
			t.Errorf("mode %d refocused: mode() = %d", mode, got)
		}
	}
}

func TestDarwinCursorLockWhileUnfocusedWaitsForFocus(t *testing.T) {
	system := newFocusedCursorSystem()
	system.focused = false
	cursor := darwinCursor{system: system}

	cursor.setMode(cursorModeLocked)

	if system.hideCount != 0 || !system.associated {
		t.Error("cursor taken while the window is not focused")
	}
	if got := cursor.mode(); got != cursorModeNormal {
		t.Errorf("mode() = %d, want Normal", got)
	}

	system.focused = true
	cursor.update()

	if system.hideCount != 1 || system.associated {
		t.Error("lock not applied on focus")
	}
	if got := cursor.mode(); got != cursorModeLocked {
		t.Errorf("mode() = %d, want Locked", got)
	}
}

func TestDarwinCursorLockedToConfinedShowsCursorInside(t *testing.T) {
	system := newFocusedCursorSystem()
	system.location = darwin.NSPoint{X: 50, Y: 250} // left of the content
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)

	cursor.setMode(cursorModeConfined)

	if system.hideCount != 0 {
		t.Errorf("hideCount = %d, want 0", system.hideCount)
	}
	if system.associated {
		t.Error("confined cursor still moved by the system")
	}
	if want := (darwin.NSPoint{X: 100, Y: 250}); system.location != want {
		t.Errorf("location = %v, want the restored point clamped to %v", system.location, want)
	}
	if got := cursor.mode(); got != cursorModeConfined {
		t.Errorf("mode() = %d, want Confined", got)
	}
}

func TestDarwinCursorContentChangeRecentresLock(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)
	cursor.move(400, 300, 350, -250) // consume the lock's warp

	system.content.Size = darwin.NSSize{Width: 400, Height: 200}
	cursor.contentChanged()

	if want := (darwin.NSPoint{X: 300, Y: 300}); system.location != want {
		t.Errorf("location = %v, want the new centre %v", system.location, want)
	}
	// The re-centring warp moved the cursor 200 left and 200 down.
	motion, _ := cursor.move(200, 100, -200+1, 200+1)
	if want := (pointerMotion{X: 200, Y: 100, DeltaX: 1, DeltaY: 1}); motion != want {
		t.Errorf("move = %+v, want %+v", motion, want)
	}
}

// refocus takes focus away from the window and gives it back, with update
// seeing both.
func refocus(system *fakeCursorSystem, cursor *darwinCursor) {
	system.focused = false
	cursor.update()
	system.focused = true
	cursor.update()
}

func TestDarwinCursorFirstLockedMoveAfterRefocusDropsDelta(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)
	cursor.move(400, 300, 350, -250) // consume the lock's warp

	refocus(system, &cursor)

	// macOS puts the motion made while the app was in the background into
	// the first event after it is back.
	if motion, reported := cursor.move(400, 300, 836, -2); reported {
		t.Errorf("first move after refocus reported %+v", motion)
	}
	motion, _ := cursor.move(400, 300, 2, 1)
	if want := (pointerMotion{X: 400, Y: 300, DeltaX: 2, DeltaY: 1}); motion != want {
		t.Errorf("second move = %+v, want %+v", motion, want)
	}
}

func TestDarwinCursorFirstConfinedMoveAfterRefocusDropsDelta(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeConfined) // the cursor stays at window (50, 550)

	refocus(system, &cursor)

	motion, _ := cursor.move(50, 550, 300, 0)
	if want := (pointerMotion{X: 50, Y: 550}); motion != want {
		t.Errorf("first move after refocus = %+v, want the cursor kept at %+v", motion, want)
	}
}

func TestDarwinCursorLockAfterRefocusKeepsFirstDeltaOnceAMoveWasSeen(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	refocus(system, &cursor)
	cursor.move(50, 550, 836, -2) // the stale move, seen in Normal

	cursor.setMode(cursorModeLocked)

	motion, _ := cursor.move(400, 300, 350+5, -250-3)
	if want := (pointerMotion{X: 400, Y: 300, DeltaX: 5, DeltaY: -3}); motion != want {
		t.Errorf("first locked move = %+v, want %+v", motion, want)
	}
}

func TestDarwinCursorUnlockAfterRefocusRestoresWhereLockWasAsked(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked) // asked with the cursor at (150, 250)

	system.focused = false
	cursor.update()
	system.location = darwin.NSPoint{X: 2000, Y: 50} // moved away, outside the window
	system.focused = true
	cursor.update()

	cursor.setMode(cursorModeNormal)

	if want := (darwin.NSPoint{X: 150, Y: 250}); system.location != want {
		t.Errorf("location = %v, want it back where the lock was asked, %v", system.location, want)
	}
}

func TestDarwinCursorUnlockRestoresWhereLockWasFirstTaken(t *testing.T) {
	system := newFocusedCursorSystem()
	system.focused = false
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)

	system.location = darwin.NSPoint{X: 300, Y: 300}
	system.focused = true
	cursor.update() // the lock is taken here, with the cursor at (300, 300)
	cursor.setMode(cursorModeNormal)

	if want := (darwin.NSPoint{X: 300, Y: 300}); system.location != want {
		t.Errorf("location = %v, want %v", system.location, want)
	}
}

func TestDarwinCursorNewLockRequestSavesNewLocation(t *testing.T) {
	system := newFocusedCursorSystem()
	cursor := darwinCursor{system: system}
	cursor.setMode(cursorModeLocked)
	cursor.setMode(cursorModeNormal)

	system.location = darwin.NSPoint{X: 600, Y: 400}
	cursor.setMode(cursorModeLocked)
	cursor.setMode(cursorModeNormal)

	if want := (darwin.NSPoint{X: 600, Y: 400}); system.location != want {
		t.Errorf("location = %v, want %v", system.location, want)
	}
}
