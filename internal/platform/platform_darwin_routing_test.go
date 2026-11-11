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

// TestDarwinPollEvents_RoutesInputToItsWindow posts a mouse move to the second
// of two windows: PollEvents reports it for that window, as WaitEvents does,
// not for the first.
func TestDarwinPollEvents_RoutesInputToItsWindow(t *testing.T) {
	second, events := actOnSecondOfTwoWindows(t, func(w *darwinPlatformWindow) error {
		return postMouseMoved(w.window, 10, 10)
	})

	var pointerEvents int
	for _, e := range events {
		if e.Type != EventPointerEnter && e.Type != EventPointerMove {
			continue
		}
		pointerEvents++
		if e.WindowID != second {
			t.Errorf("event of type %d reported for window %d, want the second window, %d", e.Type, e.WindowID, second)
		}
	}
	if pointerEvents == 0 {
		t.Error("no pointer event after a mouse move posted to the second window")
	}
}

// TestDarwinPollEvents_RoutesResizeToItsWindow resizes the second of two
// windows: PollEvents reports the resize for that window alone.
func TestDarwinPollEvents_RoutesResizeToItsWindow(t *testing.T) {
	second, events := actOnSecondOfTwoWindows(t, func(w *darwinPlatformWindow) error {
		w.RequestSize(400, 300)
		return nil
	})

	var resizes []Event
	for _, e := range events {
		if e.Type == EventResize {
			resizes = append(resizes, e)
		}
	}
	if len(resizes) != 1 || resizes[0].WindowID != second || resizes[0].Width != 400 || resizes[0].Height != 300 {
		t.Errorf("resizes = %+v, want one, of the second window (%d) at 400x300", resizes, second)
	}
}

// actOnSecondOfTwoWindows opens two windows on the main thread, polls away the
// events their opening queued, and calls act on the second. It returns the
// second window's ID and the events PollEvents reports after act.
func actOnSecondOfTwoWindows(t *testing.T, act func(second *darwinPlatformWindow) error) (WindowID, []Event) {
	t.Helper()
	// Like the darwin package's window tests, it skips on CI runners.
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("opens windows, which CI runners skip")
	}
	var second WindowID
	var events []Event
	var err error
	runOnMain(func() {
		p := &darwinPlatform{}
		if err = p.Init(); err != nil {
			return
		}
		var windows [2]*darwinPlatformWindow
		for i, title := range []string{"first", "second"} {
			var w PlatformWindow
			if w, err = p.CreateWindow(Config{Title: title, Width: 320, Height: 240}); err != nil {
				p.Destroy()
				return
			}
			windows[i] = w.(*darwinPlatformWindow)
		}
		defer func() {
			// The platform's Destroy closes its primary window alone.
			p.Destroy()
			windows[1].window.Destroy()
		}()
		second = windows[1].ID()
		pollAll(p)
		if err = act(windows[1]); err != nil {
			return
		}
		events = pollAll(p)
	})
	if err != nil {
		t.Fatal(err)
	}
	return second, events
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

// postMouseMoved queues a mouse move at (x, y) in the window's coordinates, as
// AppKit queues one: [NSEvent mouseEventWithType:location:modifierFlags:
// timestamp:windowNumber:context:eventNumber:clickCount:pressure:], then
// [NSApp postEvent:atStart:].
func postMouseMoved(window *darwin.Window, x, y float64) error {
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
		location:     darwin.MakePoint(x, y),
		windowNumber: window.NSID().GetInt64(darwin.RegisterSelector("windowNumber")),
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
