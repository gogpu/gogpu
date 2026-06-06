package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestEventTypeConstants_Distinct(t *testing.T) {
	types := []EventType{
		EventNone,
		EventClose,
		EventResize,
		EventFocus,
		EventKeyDown,
		EventKeyUp,
		EventChar,
		EventPointerDown,
		EventPointerUp,
		EventPointerMove,
		EventPointerEnter,
		EventPointerLeave,
		EventScroll,
	}

	seen := make(map[EventType]string, len(types))
	names := []string{
		"EventNone",
		"EventClose",
		"EventResize",
		"EventFocus",
		"EventKeyDown",
		"EventKeyUp",
		"EventChar",
		"EventPointerDown",
		"EventPointerUp",
		"EventPointerMove",
		"EventPointerEnter",
		"EventPointerLeave",
		"EventScroll",
	}

	for i, et := range types {
		if prev, ok := seen[et]; ok {
			t.Errorf("EventType %s has same value (%d) as %s", names[i], et, prev)
		}
		seen[et] = names[i]
	}
}

func TestEventTypeConstants_Sequential(t *testing.T) {
	if EventNone != 0 {
		t.Errorf("EventNone = %d, want 0", EventNone)
	}
	if EventScroll != 12 {
		t.Errorf("EventScroll = %d, want 12 (last constant)", EventScroll)
	}
}

func TestEvent_AllPayloadsCoexist(t *testing.T) {
	ev := Event{
		WindowID:       42,
		Type:           EventKeyDown,
		Width:          800,
		Height:         600,
		PhysicalWidth:  1600,
		PhysicalHeight: 1200,
		Focused:        true,
		Key:            gpucontext.KeyA,
		Mods:           gpucontext.ModShift | gpucontext.ModControl,
		Char:           'X',
		Pointer: gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerID:   1,
			PointerType: gpucontext.PointerTypeMouse,
			X:           100.5,
			Y:           200.5,
		},
		Scroll: gpucontext.ScrollEvent{
			DeltaX: 10.0,
			DeltaY: -20.0,
		},
	}

	if ev.WindowID != 42 {
		t.Errorf("WindowID = %d, want 42", ev.WindowID)
	}
	if ev.Key != gpucontext.KeyA {
		t.Errorf("Key = %v, want KeyA", ev.Key)
	}
	if ev.Mods != gpucontext.ModShift|gpucontext.ModControl {
		t.Errorf("Mods = %v, want Shift|Control", ev.Mods)
	}
	if ev.Char != 'X' {
		t.Errorf("Char = %c, want X", ev.Char)
	}
	if ev.Pointer.X != 100.5 || ev.Pointer.Y != 200.5 {
		t.Errorf("Pointer = (%f, %f), want (100.5, 200.5)", ev.Pointer.X, ev.Pointer.Y)
	}
	if ev.Scroll.DeltaX != 10.0 || ev.Scroll.DeltaY != -20.0 {
		t.Errorf("Scroll = (%f, %f), want (10.0, -20.0)", ev.Scroll.DeltaX, ev.Scroll.DeltaY)
	}
}

func TestNewWindowID_Unique(t *testing.T) {
	ids := make(map[WindowID]bool, 100)
	for range 100 {
		id := NewWindowID()
		if id == 0 {
			t.Fatal("NewWindowID returned zero (invalid)")
		}
		if ids[id] {
			t.Fatalf("NewWindowID returned duplicate ID %d", id)
		}
		ids[id] = true
	}
}

// mockPlatformWindow is a mock implementation of a platform window used for testing or simulation purposes.
// frameless indicates whether the window is displayed without system decorations such as borders or title bars.
// hitTestCallback is a function used for hit testing, determining interactions based on x and y coordinates.
type mockPlatformWindow struct {
	frameless       bool
	hitTestCallback func(x, y float64) gpucontext.HitTestResult
}

func (m *mockPlatformWindow) ID() WindowID                     { return 0 }
func (m *mockPlatformWindow) GetHandle() (uintptr, uintptr)    { return 0, 0 }
func (m *mockPlatformWindow) LogicalSize() (int, int)          { return 0, 0 }
func (m *mockPlatformWindow) PhysicalSize() (int, int)         { return 0, 0 }
func (m *mockPlatformWindow) ScaleFactor() float64             { return 1.0 }
func (m *mockPlatformWindow) ShouldClose() bool                { return false }
func (m *mockPlatformWindow) InSizeMove() bool                 { return false }
func (m *mockPlatformWindow) SetTitle(string)                  {}
func (m *mockPlatformWindow) PrepareFrame() PrepareFrameResult { return PrepareFrameResult{} }
func (m *mockPlatformWindow) SetCursor(int)                    {}
func (m *mockPlatformWindow) SetCursorMode(int)                {}
func (m *mockPlatformWindow) CursorMode() int                  { return 0 }
func (m *mockPlatformWindow) SyncFrame()                       {}
func (m *mockPlatformWindow) Minimize()                        {}
func (m *mockPlatformWindow) Maximize()                        {}
func (m *mockPlatformWindow) IsMaximized() bool                { return false }
func (m *mockPlatformWindow) Close()                           {}
func (m *mockPlatformWindow) Show()                            {}
func (m *mockPlatformWindow) SetFullscreen(bool)               {}
func (m *mockPlatformWindow) IsFullscreen() bool               { return false }
func (m *mockPlatformWindow) SetModalFrameCallback(func())     {}
func (m *mockPlatformWindow) Destroy()                         {}
func (m *mockPlatformWindow) SetOnClose(func() bool)           {}

func (m *mockPlatformWindow) SetFrameless(v bool) { m.frameless = v }
func (m *mockPlatformWindow) IsFrameless() bool   { return m.frameless }
func (m *mockPlatformWindow) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	m.hitTestCallback = fn
}

// Ensure mockPlatformWindow satisfies PlatformWindow at compile time.
var _ PlatformWindow = (*mockPlatformWindow)(nil)

// TestPlatformWindow_Frameless verifies that SetFrameless correctly sets and retrieves the frameless state
func TestPlatformWindow_Frameless(t *testing.T) {
	var pw PlatformWindow = &mockPlatformWindow{}

	if pw.IsFrameless() {
		t.Error("IsFrameless should be false by default")
	}

	pw.SetFrameless(true)
	if !pw.IsFrameless() {
		t.Error("IsFrameless should be true after SetFrameless(true)")
	}

	pw.SetFrameless(false)
	if pw.IsFrameless() {
		t.Error("IsFrameless should be false after SetFrameless(false)")
	}
}

// TestPlatformWindow_HitTestCallback verifies that SetHitTestCallback correctly stores the callback
func TestPlatformWindow_HitTestCallback(t *testing.T) {
	var pw PlatformWindow = &mockPlatformWindow{}

	called := false
	cb := func(x, y float64) gpucontext.HitTestResult {
		called = true
		if x == 10.0 && y == 20.0 {
			return gpucontext.HitTestClient
		}
		return gpucontext.HitTestCaption
	}

	pw.SetHitTestCallback(cb)

	mock, ok := pw.(*mockPlatformWindow)
	if !ok {
		t.Fatal("cannot type-assert to *mockPlatformWindow")
	}
	if mock.hitTestCallback == nil {
		t.Fatal("hitTestCallback should be non-nil after SetHitTestCallback")
	}

	result := mock.hitTestCallback(10.0, 20.0)
	if !called {
		t.Error("hitTestCallback was not called")
	}
	if result != gpucontext.HitTestClient {
		t.Errorf("unexpected HitTestResult: got %v, want %v", result, gpucontext.HitTestClient)
	}

	called2 := false
	cb2 := func(x, y float64) gpucontext.HitTestResult {
		called2 = true
		return gpucontext.HitTestCaption
	}
	pw.SetHitTestCallback(cb2)
	if mock.hitTestCallback == nil {
		t.Fatal("hitTestCallback should be non-nil after second SetHitTestCallback")
	}
	mock.hitTestCallback(0, 0)
	if !called2 {
		t.Error("second callback should have been called")
	}
}
