//go:build linux

package x11

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gpucontext"
)

// requestRecorder is the X server end of a Connection: it keeps each request
// written to it, and grants each GrabPointer.
type requestRecorder struct {
	net.Conn
	connection *Connection
	requests   [][]byte
}

func (r *requestRecorder) Write(data []byte) (int, error) {
	r.requests = append(r.requests, append([]byte(nil), data...))
	if data[0] == OpcodeGrabPointer {
		reply := make([]byte, 32)
		reply[0] = 1 // a reply; byte 1, the status, is Success
		r.connection.dispatchReply(uint16(r.connection.nextSeq.Load()), reply)
	}
	return len(data), nil
}

// lastWarpPointer returns the destination of the last WarpPointer request, and
// whether there was one.
func (r *requestRecorder) lastWarpPointer() (x, y int16, found bool) {
	for i := len(r.requests) - 1; i >= 0; i-- {
		request := r.requests[i]
		if request[0] == OpcodeWarpPointer {
			x = int16(binary.LittleEndian.Uint16(request[20:22]))
			y = int16(binary.LittleEndian.Uint16(request[22:24]))
			return x, y, true
		}
	}
	return 0, 0, false
}

// newCursorTestPlatform is an 800x600 point window at scale 2: 1600x1200
// physical pixels.
func newCursorTestPlatform() (*Platform, *requestRecorder) {
	recorder := &requestRecorder{}
	recorder.connection = &Connection{
		conn:           recorder,
		byteOrder:      LSBFirst,
		pendingReplies: map[uint16]chan []byte{},
	}
	platform := &Platform{
		conn:        recorder.connection,
		scaleFactor: 2,
		primary: &x11Window{
			window: 7,
			width:  1600,
			height: 1200,
			events: eventqueue.New[PlatformEvent](16),
		},
	}
	return platform, recorder
}

// motion delivers a MotionNotify at physical (x, y) and returns the pointer
// moves it queued.
func motion(platform *Platform, x, y int16) []gpucontext.PointerEvent {
	platform.handleMotionNotify(platform.primary, &MotionNotifyEvent{EventX: x, EventY: y})
	var moves []gpucontext.PointerEvent
	for {
		event, ok := platform.primary.events.Pop()
		if !ok {
			return moves
		}
		if event.Type == EventTypePointerMove {
			moves = append(moves, event.Pointer)
		}
	}
}

func TestSetCursorPositionMovesToPhysicalPoint(t *testing.T) {
	platform, recorder := newCursorTestPlatform()

	if err := platform.SetCursorPosition(10.25, 20); err != nil {
		t.Fatalf("SetCursorPosition = %v", err)
	}

	x, y, found := recorder.lastWarpPointer()
	if !found {
		t.Fatal("no WarpPointer sent")
	}
	if x != 21 || y != 40 {
		t.Errorf("moved to (%d, %d), want physical (21, 40)", x, y)
	}
}

func TestSetCursorPositionMotionIsNotReported(t *testing.T) {
	platform, _ := newCursorTestPlatform()
	if err := platform.SetCursorPosition(10, 20); err != nil {
		t.Fatalf("SetCursorPosition = %v", err)
	}

	if moves := motion(platform, 20, 40); len(moves) != 0 {
		t.Errorf("the set position's motion reported: %+v", moves)
	}
	if platform.primary.mouseX != 10 || platform.primary.mouseY != 20 {
		t.Errorf("pointer at (%v, %v), want the set position (10, 20)",
			platform.primary.mouseX, platform.primary.mouseY)
	}

	moves := motion(platform, 22, 40)
	if len(moves) != 1 || moves[0].X != 11 || moves[0].Y != 20 {
		t.Errorf("the user's motion after setting the position = %+v, want one move to (11, 20)", moves)
	}
}

func TestSetCursorPositionMotionBeforeItIsReported(t *testing.T) {
	platform, _ := newCursorTestPlatform()
	if err := platform.SetCursorPosition(10, 20); err != nil {
		t.Fatalf("SetCursorPosition = %v", err)
	}

	// Queued before the set position's own MotionNotify.
	if moves := motion(platform, 100, 100); len(moves) != 1 {
		t.Errorf("motion queued before the set position's: %d moves, want 1", len(moves))
	}
	if moves := motion(platform, 20, 40); len(moves) != 0 {
		t.Errorf("the set position's motion reported: %+v", moves)
	}
}

func TestSetCursorPositionConfinedClampsToWindow(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.primary.cursorMode = 2

	if err := platform.SetCursorPosition(-5, 5000); err != nil {
		t.Fatalf("SetCursorPosition = %v", err)
	}

	x, y, _ := recorder.lastWarpPointer()
	if x != 0 || y != 1199 {
		t.Errorf("moved to (%d, %d), want the window's corner (0, 1199)", x, y)
	}
	if moves := motion(platform, 0, 1199); len(moves) != 0 {
		t.Errorf("the clamped position's motion reported: %+v", moves)
	}
}

func TestSetCursorPositionLockedSetsWhereUnlockPutsPointer(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.primary.cursorMode = 1

	if err := platform.SetCursorPosition(10, 20); err != nil {
		t.Fatalf("SetCursorPosition = %v", err)
	}
	if _, _, found := recorder.lastWarpPointer(); found {
		t.Fatal("setting the position while locked moved the pointer")
	}

	platform.SetCursorMode(0)

	x, y, found := recorder.lastWarpPointer()
	if !found || x != 20 || y != 40 {
		t.Errorf("unlock warped to (%d, %d), want the set position, physical (20, 40)", x, y)
	}
}

func TestLockSavesPhysicalPointerPosition(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	motion(platform, 300, 500) // the pointer at logical (150, 250)
	platform.blankCursorID = 1

	platform.SetCursorMode(1)
	platform.SetCursorMode(0)

	x, y, _ := recorder.lastWarpPointer()
	if x != 300 || y != 500 {
		t.Errorf("unlock warped to (%d, %d), want the pointer back at physical (300, 500)", x, y)
	}
}

func TestLockedMotionAtScaleReportsLogicalDeltaAtCentre(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1
	platform.SetCursorMode(1) // parks the pointer at the center, physical (800, 600)

	if moves := motion(platform, 800, 600); len(moves) != 0 {
		t.Errorf("the lock's own warp to the center reported: %+v", moves)
	}

	moves := motion(platform, 810, 590)
	if len(moves) != 1 {
		t.Fatalf("a locked move: %d moves, want 1", len(moves))
	}
	move := moves[0]
	if move.DeltaX != 5 || move.DeltaY != -5 {
		t.Errorf("delta = (%v, %v), want logical (5, -5)", move.DeltaX, move.DeltaY)
	}
	if move.X != 400 || move.Y != 300 {
		t.Errorf("position = (%v, %v), want the frozen center, logical (400, 300)", move.X, move.Y)
	}
	if x, y, _ := recorder.lastWarpPointer(); x != 800 || y != 600 {
		t.Errorf("warped back to (%d, %d), want the center, physical (800, 600)", x, y)
	}
}

func TestLockedSetPositionSurvivesFocusLoss(t *testing.T) {
	platform, recorder := newCursorTestPlatform()
	platform.blankCursorID = 1
	platform.SetCursorMode(1)
	if err := platform.SetCursorPosition(10, 20); err != nil {
		t.Fatalf("SetCursorPosition = %v", err)
	}

	platform.handleFocusOut(platform.primary)
	platform.primary.mouseX = 150 // the user left with the pointer elsewhere
	platform.primary.mouseY = 250
	platform.handleFocusIn(platform.primary)
	platform.SetCursorMode(0)

	if x, y, _ := recorder.lastWarpPointer(); x != 20 || y != 40 {
		t.Errorf("unlock warped to (%d, %d), want the set position, physical (20, 40)", x, y)
	}
}
