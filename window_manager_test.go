package gogpu

import (
	"fmt"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

func TestWindowManager_AddGet(t *testing.T) {
	wm := newWindowManager()
	w := &Window{id: wm.allocate()}
	wm.add(w)

	got := wm.get(w.id)
	if got != w {
		t.Error("get() should return the added window")
	}
}

func TestWindowManager_GetUnknownID(t *testing.T) {
	wm := newWindowManager()
	w := &Window{id: wm.allocate()}
	wm.add(w)

	unknownID := wm.allocate()
	got := wm.get(unknownID)
	if got != nil {
		t.Error("get() should return nil for unknown ID")
	}
}

func TestWindowManager_Remove(t *testing.T) {
	wm := newWindowManager()

	w := &Window{id: wm.allocate()}
	wm.add(w)
	wm.remove(w.id)

	if wm.get(w.id) != nil {
		t.Error("get() should return nil after remove")
	}
	if wm.count() != 0 {
		t.Errorf("count() = %d, want 0", wm.count())
	}
}

func TestWindowManager_Count(t *testing.T) {
	wm := newWindowManager()

	if wm.count() != 0 {
		t.Errorf("count() = %d, want 0 for empty manager", wm.count())
	}

	w1 := &Window{id: wm.allocate()}
	w2 := &Window{id: wm.allocate()}
	wm.add(w1)
	wm.add(w2)

	if wm.count() != 2 {
		t.Errorf("count() = %d, want 2", wm.count())
	}
}

func TestWindowManager_FocusAutoAssign(t *testing.T) {
	wm := newWindowManager()

	w1 := &Window{id: wm.allocate()}
	wm.add(w1)

	if wm.focused != w1.id {
		t.Error("first added window should receive focus automatically")
	}
}

func TestWindowManager_FocusAfterRemove(t *testing.T) {
	wm := newWindowManager()

	w1 := &Window{id: wm.allocate()}
	w2 := &Window{id: wm.allocate()}
	wm.add(w1)
	wm.add(w2)

	wm.setFocus(w2.id)
	wm.remove(w2.id)

	if wm.focused != w1.id {
		t.Errorf("focused = %d, want %d (first remaining window)", wm.focused, w1.id)
	}
}

func TestWindowManager_SetFocusInvalidID(t *testing.T) {
	wm := newWindowManager()

	w := &Window{id: wm.allocate()}
	wm.add(w)

	wm.setFocus(wm.allocate())

	if wm.focused != w.id {
		t.Error("setFocus with unknown ID should not change focus")
	}
}

func TestWindow_SetOnKeyPress(t *testing.T) {
	t.Run(
		"fires when set", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			var receivedKey gpucontext.Key
			var receivedMods gpucontext.Modifiers
			w.SetOnKeyPress(
				func(key gpucontext.Key, mods gpucontext.Modifiers) {
					receivedKey = key
					receivedMods = mods
				},
			)

			w.onKeyPress(gpucontext.KeyA, gpucontext.ModShift)

			if receivedKey != gpucontext.KeyA {
				t.Errorf("key = %v, want KeyA", receivedKey)
			}
			if receivedMods != gpucontext.ModShift {
				t.Errorf("mods = %v, want ModShift", receivedMods)
			}
		},
	)

	t.Run(
		"nil callback safe", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			if w.onKeyPress != nil {
				t.Error("onKeyPress should be nil by default")
			}
		},
	)

	t.Run(
		"replacement", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			callCount := 0
			w.SetOnKeyPress(
				func(gpucontext.Key, gpucontext.Modifiers) {
					callCount = 1
				},
			)
			w.SetOnKeyPress(
				func(gpucontext.Key, gpucontext.Modifiers) {
					callCount = 2
				},
			)

			w.onKeyPress(gpucontext.KeyB, 0)

			if callCount != 2 {
				t.Errorf("callCount = %d, want 2 (replaced callback should fire)", callCount)
			}
		},
	)
}

func TestWindow_SetOnKeyRelease(t *testing.T) {
	t.Run(
		"fires when set", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			var receivedKey gpucontext.Key
			w.SetOnKeyRelease(
				func(key gpucontext.Key, mods gpucontext.Modifiers) {
					receivedKey = key
				},
			)

			w.onKeyRelease(gpucontext.KeyEscape, 0)

			if receivedKey != gpucontext.KeyEscape {
				t.Errorf("key = %v, want KeyEscape", receivedKey)
			}
		},
	)

	t.Run(
		"nil callback safe", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			if w.onKeyRelease != nil {
				t.Error("onKeyRelease should be nil by default")
			}
		},
	)
}

func TestWindow_SetOnTextInput(t *testing.T) {
	t.Run(
		"fires when set", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			var received string
			w.SetOnTextInput(
				func(text string) {
					received = text
				},
			)

			w.onTextInput("hello")

			if received != "hello" {
				t.Errorf("text = %q, want %q", received, "hello")
			}
		},
	)

	t.Run(
		"nil callback safe", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			if w.onTextInput != nil {
				t.Error("onTextInput should be nil by default")
			}
		},
	)
}

func TestWindow_SetOnPointer(t *testing.T) {
	t.Run(
		"fires when set", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			var received gpucontext.PointerEvent
			w.SetOnPointer(
				func(ev gpucontext.PointerEvent) {
					received = ev
				},
			)

			ev := gpucontext.PointerEvent{
				Type: gpucontext.PointerMove,
				X:    42,
				Y:    84,
			}
			w.onPointer(ev)

			if received.X != 42 || received.Y != 84 {
				t.Errorf("pointer = (%f, %f), want (42, 84)", received.X, received.Y)
			}
		},
	)

	t.Run(
		"nil callback safe", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			if w.onPointer != nil {
				t.Error("onPointer should be nil by default")
			}
		},
	)
}

func TestWindow_SetOnScroll(t *testing.T) {
	t.Run(
		"fires when set", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			var received gpucontext.ScrollEvent
			w.SetOnScroll(
				func(ev gpucontext.ScrollEvent) {
					received = ev
				},
			)

			ev := gpucontext.ScrollEvent{DeltaX: 5.5, DeltaY: -10.0}
			w.onScroll(ev)

			if received.DeltaX != 5.5 || received.DeltaY != -10.0 {
				t.Errorf("scroll = (%f, %f), want (5.5, -10.0)", received.DeltaX, received.DeltaY)
			}
		},
	)

	t.Run(
		"nil callback safe", func(t *testing.T) {
			w := &Window{id: WindowID(1)}
			if w.onScroll != nil {
				t.Error("onScroll should be nil by default")
			}
		},
	)
}

func TestWindow_ID(t *testing.T) {
	id := WindowID(1)
	w := &Window{id: id}

	if w.ID() != id {
		t.Errorf("ID() = %d, want %d", w.ID(), id)
	}
}

func TestWindow_SizeNilPlatform(t *testing.T) {
	w := &Window{
		config: Config{Width: 640, Height: 480},
	}

	width, height := w.Size()
	if width != 640 || height != 480 {
		t.Errorf("Size() = (%d, %d), want (640, 480)", width, height)
	}
}

func TestWindow_PhysicalSizeNilPlatform(t *testing.T) {
	w := &Window{
		config: Config{Width: 800, Height: 600},
	}

	width, height := w.PhysicalSize()
	if width != 800 || height != 600 {
		t.Errorf("PhysicalSize() = (%d, %d), want (800, 600)", width, height)
	}
}

func TestWindow_Visible(t *testing.T) {
	w := &Window{visible: true}
	if !w.Visible() {
		t.Error("Visible() should return true")
	}

	w.visible = false
	if w.Visible() {
		t.Error("Visible() should return false")
	}
}

func TestWindowManager_AllocateSequential(t *testing.T) {
	wm := newWindowManager()

	id1 := wm.allocate()
	id2 := wm.allocate()
	id3 := wm.allocate()

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Errorf("allocated IDs = %d, %d, %d; want 1, 2, 3", id1, id2, id3)
	}
}

func TestWindowManager_AllocateReuseAfterRelease(t *testing.T) {
	wm := newWindowManager()

	id1 := wm.allocate()
	id2 := wm.allocate()

	// Release first ID
	wm.release(id1)

	// Next allocation should reuse id1
	id3 := wm.allocate()
	if id3 != id1 {
		t.Errorf("reused ID = %d, want %d", id3, id1)
	}

	// Next allocation should be new (id2 still in use)
	id4 := wm.allocate()
	if id4 != id2+1 { // next sequential after id2
		t.Errorf("expected next sequential ID after %d, got %d", id2, id4)
	}
}

func TestWindowManager_AllocateMultipleReuse(t *testing.T) {
	wm := newWindowManager()
	ids := make([]WindowID, 5)
	for i := range ids {
		ids[i] = wm.allocate()
	}
	// Release them all in reverse order
	for i := 0; i < len(ids); i++ {
		wm.release(ids[i])
	}
	// Now re-allocate; IDs should be reused in LIFO order (stack)
	for i := 0; i < len(ids); i++ {
		reused := wm.allocate()
		expected := ids[len(ids)-1-i]
		if reused != expected {
			t.Errorf("reused ID = %d, want %d", reused, expected)
		}
	}
}

func TestWindowManager_GetByPlatformID(t *testing.T) {
	wm := newWindowManager()

	pid1 := platform.NewWindowID()
	pid2 := platform.NewWindowID()

	// Create windows with internal and platform IDs
	w1 := &Window{id: wm.allocate(), platformID: pid1}
	w2 := &Window{id: wm.allocate(), platformID: pid2}
	wm.add(w1)
	wm.add(w2)

	// Lookup by platform ID
	if got := wm.getByPlatformID(pid1); got != w1 {
		t.Error("getByPlatformID(pid1) should return w1")
	}
	if got := wm.getByPlatformID(pid2); got != w2 {
		t.Error("getByPlatformID(pid2) should return w2")
	}

	// Unknown platform ID
	if got := wm.getByPlatformID(platform.NewWindowID()); got != nil {
		t.Error("getByPlatformID(unknown) should return nil")
	}
}

func TestWindowManager_RemoveDoesNotReleaseAutomatically(t *testing.T) {
	// Verify backward compatibility: remove doesn't recycle ID
	wm := newWindowManager()

	id1 := wm.allocate()
	w := &Window{id: id1, platformID: platform.NewWindowID()}
	wm.add(w)

	wm.remove(id1)
	// ID not recycled; next allocate gives new ID
	id2 := wm.allocate()
	if id2 == id1 {
		t.Error("remove should not automatically recycle ID, but got same ID")
	}
}

func TestWindowManager_AddWithPlatformID(t *testing.T) {
	wm := newWindowManager()

	pid := platform.NewWindowID()
	w := &Window{id: wm.allocate(), platformID: pid}
	wm.add(w)

	if got := wm.get(w.id); got != w {
		t.Error("get(internalID) should return the window")
	}
	if got := wm.getByPlatformID(pid); got != w {
		t.Error("getByPlatformID(platformID) should return the window")
	}
}

func TestWindowManager_GetByPlatformID_AfterRemove(t *testing.T) {
	wm := newWindowManager()
	pid := platform.NewWindowID()
	w := &Window{id: wm.allocate(), platformID: pid}
	wm.add(w)
	wm.remove(w.id)

	if got := wm.getByPlatformID(pid); got != nil {
		t.Error("getByPlatformID should return nil after window removed")
	}
}

func TestWindowManager_RemoveUnknownID(t *testing.T) {
	wm := newWindowManager()
	wm.remove(WindowID(999))
	if wm.count() != 0 {
		t.Error("count should remain 0 after removing unknown ID")
	}
}

type errPlatformManager struct {
	platform.PlatformManager
}

func (e *errPlatformManager) CreateWindow(platform.Config) (platform.PlatformWindow, error) {
	return nil, fmt.Errorf("simulated error")
}

func TestNewWindow_CreateWindowError(t *testing.T) {
	app := &App{
		manager:       &errPlatformManager{},
		renderer:      &Renderer{},
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
	}
	_, err := app.NewWindow(Config{})
	if err == nil {
		t.Error("expected error from CreateWindow")
	}
}

func TestWindow_SetOnClose_PropagatesToPlatform(t *testing.T) {
	mw := &mockWindow{}
	w := &Window{
		platWindow: mw,
	}
	called := false
	cb := func() bool { called = true; return true }
	w.SetOnClose(cb)
	if mw.closeFn == nil {
		t.Fatal("SetOnClose was not propagated to platform window")
	}

	if !mw.closeFn() {
		t.Error("callback returned false")
	}
	if !called {
		t.Error("original callback was not called")
	}
}
