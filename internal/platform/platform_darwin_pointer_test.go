//go:build darwin

package platform

import (
	"os"
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// AppKit opens windows on the main thread alone, which the darwin package's
// init keeps for the main goroutine: TestMain runs the tests on another
// goroutine and serves runOnMain's calls.
var mainThreadCalls = make(chan func())

func TestMain(m *testing.M) {
	code := make(chan int, 1)
	go func() {
		code <- m.Run()
		close(mainThreadCalls)
	}()
	for call := range mainThreadCalls {
		call()
	}
	os.Exit(<-code)
}

// runOnMain runs fn on the main thread and waits for it. fn must not call
// t.Fatal: it would end the main goroutine.
func runOnMain(fn func()) {
	done := make(chan struct{})
	mainThreadCalls <- func() {
		defer close(done)
		fn()
	}
	<-done
}

// TestDarwinPollEvents_TakesAWindowlessMoveOverItsWindow posts a mouse move
// that comes without a window, as AppKit hands them over after a window
// leaves native fullscreen (#552), at a point over the window: PollEvents
// reports it for the window, at the point in the window's coordinates.
func TestDarwinPollEvents_TakesAWindowlessMoveOverItsWindow(t *testing.T) {
	_, events := postWindowlessMove(t, false, func(window, _ darwin.NSRect) darwin.NSPoint {
		// 100 points from the left, 50 down from the top of a 240-point content.
		return darwin.MakePoint(window.Origin.X+100, window.Origin.Y+240-50)
	})

	moves := pointerMoves(events)
	if len(moves) != 1 {
		t.Fatalf("PollEvents reported %d pointer moves, want 1: %+v", len(moves), events)
	}
	if moves[0].Pointer.X != 100 || moves[0].Pointer.Y != 50 {
		t.Errorf("pointer move at (%v, %v), want (100, 50)", moves[0].Pointer.X, moves[0].Pointer.Y)
	}
}

// TestDarwinPollEvents_DropsAWindowlessMoveOutsideItsWindow posts a mouse move
// that comes without a window at a point left of the window: PollEvents
// reports none.
func TestDarwinPollEvents_DropsAWindowlessMoveOutsideItsWindow(t *testing.T) {
	_, events := postWindowlessMove(t, false, func(window, _ darwin.NSRect) darwin.NSPoint {
		return darwin.MakePoint(window.Origin.X-20, window.Origin.Y+120)
	})

	if moves := pointerMoves(events); len(moves) != 0 {
		t.Errorf("PollEvents reported %d pointer moves for a move outside the window, want none", len(moves))
	}
}

// TestDarwinPollEvents_DropsAWindowlessMoveWhereAnotherWindowCoversIt posts a
// mouse move that comes without a window at a point inside the window's frame
// that a window above it covers: PollEvents reports none for the window
// below.
func TestDarwinPollEvents_DropsAWindowlessMoveWhereAnotherWindowCoversIt(t *testing.T) {
	below, events := postWindowlessMove(t, true, func(_, cover darwin.NSRect) darwin.NSPoint {
		return darwin.MakePoint(cover.Origin.X+cover.Size.Width/2, cover.Origin.Y+cover.Size.Height/2)
	})

	moves := pointerMoves(events)
	for i := range moves {
		if moves[i].WindowID == below {
			t.Errorf("PollEvents reported a pointer move for the covered window at (%v, %v)", moves[i].Pointer.X, moves[i].Pointer.Y)
		}
	}
}

// postWindowlessMove opens a 320x240 window over every normal window and,
// when covered, a 160x120 window over its middle. It drains the events their
// opening queued, posts a mouse move with no window at the screen point at
// answers from the two frames (the second one zero when not covered), and
// returns the first window's ID and the events PollEvents reports after it.
func postWindowlessMove(t *testing.T, covered bool, at func(window, cover darwin.NSRect) darwin.NSPoint) (WindowID, []Event) {
	t.Helper()
	// Like the darwin package's window tests, it skips on CI runners.
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("opens windows, which CI runners skip")
	}
	var id WindowID
	var events []Event
	var err error
	runOnMain(func() {
		p := &darwinPlatform{}
		if err = p.Init(); err != nil {
			return
		}
		defer p.Destroy()
		var window, cover *darwinPlatformWindow
		if window, err = openRaisedWindow(p, 320, 240); err != nil {
			return
		}
		id = window.ID()
		frame := window.window.Frame()
		var coverFrame darwin.NSRect
		if covered {
			if cover, err = openRaisedWindow(p, 160, 120); err != nil {
				return
			}
			origin := darwin.MakePoint(frame.Origin.X+80, frame.Origin.Y+60)
			cover.window.NSWindow().SendPoint(darwin.RegisterSelector("setFrameOrigin:"), origin)
			coverFrame = cover.window.Frame()
		}
		pollAll(p)
		if err = postMouseMovedAt(at(frame, coverFrame), 0); err != nil {
			return
		}
		events = pollAll(p)
	})
	if err != nil {
		t.Fatal(err)
	}
	return id, events
}

// openRaisedWindow opens a window and lifts it to the floating level, in
// front of every normal window whichever application is active, so that the
// window server answers it at its location.
func openRaisedWindow(p *darwinPlatform, width, height int) (*darwinPlatformWindow, error) {
	w, err := p.CreateWindow(Config{Title: "pointer test", Width: width, Height: height})
	if err != nil {
		return nil, err
	}
	window := w.(*darwinPlatformWindow)
	const floatingWindowLevel = 3
	window.window.NSWindow().SendInt(darwin.RegisterSelector("setLevel:"), floatingWindowLevel)
	window.window.NSWindow().Send(darwin.RegisterSelector("orderFrontRegardless"))
	return window, nil
}

// pollAll polls until no event is left, and answers the events.
func pollAll(p *darwinPlatform) []Event {
	var events []Event
	for range 1000 {
		e := p.PollEvents()
		if e.Type == EventNone {
			break
		}
		events = append(events, e)
	}
	return events
}

// pointerMoves answers the pointer moves among events.
func pointerMoves(events []Event) []Event {
	var moves []Event
	for i := range events {
		if events[i].Type == EventPointerMove {
			moves = append(moves, events[i])
		}
	}
	return moves
}

// postMouseMovedAt queues a mouse move at location, as AppKit queues one:
// [NSEvent mouseEventWithType:location:modifierFlags:timestamp:windowNumber:
// context:eventNumber:clickCount:pressure:], then [NSApp postEvent:atStart:].
// With windowNumber 0 the event has no window, and location is in screen
// coordinates.
func postMouseMovedAt(location darwin.NSPoint, windowNumber int64) error {
	libobjc, err := ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
	if err != nil {
		return err
	}
	msgSend, err := ffi.GetSymbol(libobjc, "objc_msgSend")
	if err != nil {
		return err
	}
	point := &types.TypeDescriptor{
		Kind:    types.StructType,
		Members: []*types.TypeDescriptor{types.DoubleTypeDescriptor, types.DoubleTypeDescriptor},
	}
	var call types.CallInterface
	err = ffi.PrepareCallInterface(&call, types.DefaultCall, types.PointerTypeDescriptor, []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // NSEvent
		types.PointerTypeDescriptor, // selector
		types.UInt64TypeDescriptor,  // type
		point,                       // location
		types.UInt64TypeDescriptor,  // modifierFlags
		types.DoubleTypeDescriptor,  // timestamp
		types.SInt64TypeDescriptor,  // windowNumber
		types.PointerTypeDescriptor, // context
		types.SInt64TypeDescriptor,  // eventNumber
		types.SInt64TypeDescriptor,  // clickCount
		types.FloatTypeDescriptor,   // pressure
	})
	if err != nil {
		return err
	}
	arguments := struct {
		class        uintptr
		selector     uintptr
		eventType    uint64
		location     darwin.NSPoint
		flags        uint64
		timestamp    float64
		windowNumber int64
		context      uintptr
		eventNumber  int64
		clickCount   int64
		pressure     float32
	}{
		class:        uintptr(darwin.GetClass("NSEvent")),
		selector:     uintptr(darwin.RegisterSelector("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:")),
		eventType:    uint64(darwin.NSEventTypeMouseMoved),
		location:     location,
		windowNumber: windowNumber,
	}
	var event uintptr
	_, err = ffi.CallFunction(&call, msgSend, unsafe.Pointer(&event), []unsafe.Pointer{
		unsafe.Pointer(&arguments.class),
		unsafe.Pointer(&arguments.selector),
		unsafe.Pointer(&arguments.eventType),
		unsafe.Pointer(&arguments.location),
		unsafe.Pointer(&arguments.flags),
		unsafe.Pointer(&arguments.timestamp),
		unsafe.Pointer(&arguments.windowNumber),
		unsafe.Pointer(&arguments.context),
		unsafe.Pointer(&arguments.eventNumber),
		unsafe.Pointer(&arguments.clickCount),
		unsafe.Pointer(&arguments.pressure),
	})
	if err != nil {
		return err
	}
	darwin.GetApplication().NSApp().SendPtrs(darwin.RegisterSelector("postEvent:atStart:"), event, 0)
	return nil
}
