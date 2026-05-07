package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

func TestWindowManager_AddGet(t *testing.T) {
	wm := newWindowManager()

	w := &Window{id: platform.NewWindowID()}
	wm.add(w)

	got := wm.get(w.id)
	if got != w {
		t.Error("get() should return the added window")
	}
}

func TestWindowManager_GetUnknownID(t *testing.T) {
	wm := newWindowManager()

	got := wm.get(platform.NewWindowID())
	if got != nil {
		t.Error("get() should return nil for unknown ID")
	}
}

func TestWindowManager_Remove(t *testing.T) {
	wm := newWindowManager()

	w := &Window{id: platform.NewWindowID()}
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

	w1 := &Window{id: platform.NewWindowID()}
	w2 := &Window{id: platform.NewWindowID()}
	wm.add(w1)
	wm.add(w2)

	if wm.count() != 2 {
		t.Errorf("count() = %d, want 2", wm.count())
	}
}

func TestWindowManager_FocusAutoAssign(t *testing.T) {
	wm := newWindowManager()

	w1 := &Window{id: platform.NewWindowID()}
	wm.add(w1)

	if wm.focused != w1.id {
		t.Error("first added window should receive focus automatically")
	}
}

func TestWindowManager_FocusAfterRemove(t *testing.T) {
	wm := newWindowManager()

	w1 := &Window{id: platform.NewWindowID()}
	w2 := &Window{id: platform.NewWindowID()}
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

	w := &Window{id: platform.NewWindowID()}
	wm.add(w)

	wm.setFocus(platform.NewWindowID())

	if wm.focused != w.id {
		t.Error("setFocus with unknown ID should not change focus")
	}
}

func TestWindow_SetOnKeyPress(t *testing.T) {
	t.Run("fires when set", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		var receivedKey gpucontext.Key
		var receivedMods gpucontext.Modifiers
		w.SetOnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
			receivedKey = key
			receivedMods = mods
		})

		w.onKeyPress(gpucontext.KeyA, gpucontext.ModShift)

		if receivedKey != gpucontext.KeyA {
			t.Errorf("key = %v, want KeyA", receivedKey)
		}
		if receivedMods != gpucontext.ModShift {
			t.Errorf("mods = %v, want ModShift", receivedMods)
		}
	})

	t.Run("nil callback safe", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		if w.onKeyPress != nil {
			t.Error("onKeyPress should be nil by default")
		}
	})

	t.Run("replacement", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		callCount := 0
		w.SetOnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) {
			callCount = 1
		})
		w.SetOnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) {
			callCount = 2
		})

		w.onKeyPress(gpucontext.KeyB, 0)

		if callCount != 2 {
			t.Errorf("callCount = %d, want 2 (replaced callback should fire)", callCount)
		}
	})
}

func TestWindow_SetOnKeyRelease(t *testing.T) {
	t.Run("fires when set", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		var receivedKey gpucontext.Key
		w.SetOnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
			receivedKey = key
		})

		w.onKeyRelease(gpucontext.KeyEscape, 0)

		if receivedKey != gpucontext.KeyEscape {
			t.Errorf("key = %v, want KeyEscape", receivedKey)
		}
	})

	t.Run("nil callback safe", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		if w.onKeyRelease != nil {
			t.Error("onKeyRelease should be nil by default")
		}
	})
}

func TestWindow_SetOnTextInput(t *testing.T) {
	t.Run("fires when set", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		var received string
		w.SetOnTextInput(func(text string) {
			received = text
		})

		w.onTextInput("hello")

		if received != "hello" {
			t.Errorf("text = %q, want %q", received, "hello")
		}
	})

	t.Run("nil callback safe", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		if w.onTextInput != nil {
			t.Error("onTextInput should be nil by default")
		}
	})
}

func TestWindow_SetOnPointer(t *testing.T) {
	t.Run("fires when set", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		var received gpucontext.PointerEvent
		w.SetOnPointer(func(ev gpucontext.PointerEvent) {
			received = ev
		})

		ev := gpucontext.PointerEvent{
			Type: gpucontext.PointerMove,
			X:    42,
			Y:    84,
		}
		w.onPointer(ev)

		if received.X != 42 || received.Y != 84 {
			t.Errorf("pointer = (%f, %f), want (42, 84)", received.X, received.Y)
		}
	})

	t.Run("nil callback safe", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		if w.onPointer != nil {
			t.Error("onPointer should be nil by default")
		}
	})
}

func TestWindow_SetOnScroll(t *testing.T) {
	t.Run("fires when set", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		var received gpucontext.ScrollEvent
		w.SetOnScroll(func(ev gpucontext.ScrollEvent) {
			received = ev
		})

		ev := gpucontext.ScrollEvent{DeltaX: 5.5, DeltaY: -10.0}
		w.onScroll(ev)

		if received.DeltaX != 5.5 || received.DeltaY != -10.0 {
			t.Errorf("scroll = (%f, %f), want (5.5, -10.0)", received.DeltaX, received.DeltaY)
		}
	})

	t.Run("nil callback safe", func(t *testing.T) {
		w := &Window{id: platform.NewWindowID()}
		if w.onScroll != nil {
			t.Error("onScroll should be nil by default")
		}
	})
}

func TestWindow_ID(t *testing.T) {
	id := platform.NewWindowID()
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
