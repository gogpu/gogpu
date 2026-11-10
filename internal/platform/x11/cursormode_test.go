//go:build linux

package x11

import "testing"

func TestCursorModeIsNormalWhileTheWindowLacksTheFocus(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1

	platform.SetCursorMode(1)
	if mode := platform.GetCursorMode(); mode != 1 {
		t.Fatalf("a locked window with the focus answers %d, want Locked", mode)
	}
	platform.handleFocus(platform.primary, false)
	if mode := platform.GetCursorMode(); mode != 0 {
		t.Errorf("a locked window without the focus answers %d, want Normal", mode)
	}
	if ungrabs := recorder.countRequests(OpcodeUngrabPointer); ungrabs != 1 {
		t.Errorf("losing the focus released the grab %d times, want once", ungrabs)
	}
	platform.handleFocus(platform.primary, true)
	if mode := platform.GetCursorMode(); mode != 1 {
		t.Errorf("a locked window that regained the focus answers %d, want Locked", mode)
	}
}

func TestCursorModeAskedWithoutTheFocusIsInForceOnceFocused(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.primary.focused = false

	platform.SetCursorMode(2)
	if mode := platform.GetCursorMode(); mode != 0 {
		t.Errorf("a confined window without the focus answers %d, want Normal", mode)
	}
	if grabs := recorder.countRequests(OpcodeGrabPointer); grabs != 0 {
		t.Errorf("a window without the focus grabbed the pointer %d times", grabs)
	}
	platform.handleFocus(platform.primary, true)
	if mode := platform.GetCursorMode(); mode != 2 {
		t.Errorf("a confined window that gained the focus answers %d, want Confined", mode)
	}
}

func TestCursorModeIsNormalWhenTheServerRefusesTheGrab(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1
	recorder.grabStatus = 1 // AlreadyGrabbed: another client holds the pointer

	platform.SetCursorMode(1)
	if mode := platform.GetCursorMode(); mode != 0 {
		t.Errorf("a lock the server refused answers %d, want Normal", mode)
	}
}

func TestLockParksThePointerBeforeGrabbingIt(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1

	platform.SetCursorMode(1)

	// A grab confines a pointer outside the window to its edge, whose
	// motion would be reported as a locked move.
	warp := -1
	grab := -1
	for i, request := range recorder.requests {
		switch {
		case request[0] == OpcodeWarpPointer && warp < 0:
			warp = i
		case request[0] == OpcodeGrabPointer && grab < 0:
			grab = i
		}
	}
	if warp < 0 || grab < 0 || warp > grab {
		t.Errorf("the lock warped at request %d and grabbed at %d, want the warp to the center first", warp, grab)
	}
	if x, y, _ := recorder.lastWarpPointer(); x != 800 || y != 600 {
		t.Errorf("the lock parked the pointer at (%d, %d), want the center, physical (800, 600)", x, y)
	}
}

func TestMotionWithoutTheFocusIsNotLocked(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1
	platform.SetCursorMode(1)
	platform.handleFocus(platform.primary, false)
	recorder.requests = nil

	moves := motion(platform, 300, 500)
	if len(moves) != 1 || moves[0].X != 150 || moves[0].Y != 250 || moves[0].DeltaX != 0 || moves[0].DeltaY != 0 {
		t.Errorf("a move over a locked window without the focus = %+v, want one plain move to (150, 250)", moves)
	}
	if _, _, found := recorder.lastWarpPointer(); found {
		t.Error("a move over a locked window without the focus warped the pointer back")
	}
}

func TestLockReleasedWithoutTheFocusLeavesThePointer(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1
	platform.SetCursorMode(1)
	platform.handleFocus(platform.primary, false)
	recorder.requests = nil

	platform.SetCursorMode(0)
	if _, _, found := recorder.lastWarpPointer(); found {
		t.Error("releasing the lock of a window without the focus moved the pointer")
	}
}
