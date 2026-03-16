package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

// TestEventSourceAdapterInterface verifies eventSourceAdapter implements gpucontext.EventSource.
func TestEventSourceAdapterInterface(t *testing.T) {
	var _ gpucontext.EventSource = (*eventSourceAdapter)(nil)
}

// TestEventSourceReturnsConsistentInstance verifies EventSource returns the same instance.
func TestEventSourceReturnsConsistentInstance(t *testing.T) {
	app := NewApp(DefaultConfig())

	es1 := app.EventSource()
	es2 := app.EventSource()

	if es1 != es2 {
		t.Error("EventSource() should return the same instance on multiple calls")
	}
}

// TestEventSourceCallbackRegistration tests callback registration.
func TestEventSourceCallbackRegistration(t *testing.T) {
	app := NewApp(DefaultConfig())
	es := app.EventSource()

	t.Run("OnKeyPress", func(t *testing.T) {
		called := false
		es.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchKeyPress(gpucontext.KeyA, gpucontext.ModShift)
		if !called {
			t.Error("OnKeyPress callback was not called")
		}
	})

	t.Run("OnKeyRelease", func(t *testing.T) {
		called := false
		es.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchKeyRelease(gpucontext.KeyA, gpucontext.ModShift)
		if !called {
			t.Error("OnKeyRelease callback was not called")
		}
	})

	t.Run("OnTextInput", func(t *testing.T) {
		called := false
		var receivedText string
		es.OnTextInput(func(text string) {
			called = true
			receivedText = text
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchTextInput("Hello")
		if !called {
			t.Error("OnTextInput callback was not called")
		}
		if receivedText != "Hello" {
			t.Errorf("OnTextInput received %q, want %q", receivedText, "Hello")
		}
	})

	t.Run("OnMouseMove", func(t *testing.T) {
		called := false
		es.OnMouseMove(func(x, y float64) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchMouseMove(100, 200)
		if !called {
			t.Error("OnMouseMove callback was not called")
		}
	})

	t.Run("OnMousePress", func(t *testing.T) {
		called := false
		es.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchMousePress(gpucontext.MouseButtonLeft, 100, 200)
		if !called {
			t.Error("OnMousePress callback was not called")
		}
	})

	t.Run("OnMouseRelease", func(t *testing.T) {
		called := false
		es.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchMouseRelease(gpucontext.MouseButtonLeft, 100, 200)
		if !called {
			t.Error("OnMouseRelease callback was not called")
		}
	})

	t.Run("OnScroll", func(t *testing.T) {
		called := false
		es.OnScroll(func(dx, dy float64) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchScroll(10, 20)
		if !called {
			t.Error("OnScroll callback was not called")
		}
	})

	t.Run("OnResize", func(t *testing.T) {
		called := false
		es.OnResize(func(width, height int) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchResize(800, 600)
		if !called {
			t.Error("OnResize callback was not called")
		}
	})

	t.Run("OnFocus", func(t *testing.T) {
		called := false
		es.OnFocus(func(focused bool) {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		adapter.dispatchFocus(true)
		if !called {
			t.Error("OnFocus callback was not called")
		}
	})
}

// TestEventSourceNilCallbacks tests that dispatch methods handle nil callbacks safely.
func TestEventSourceNilCallbacks(t *testing.T) {
	adapter := &eventSourceAdapter{}

	// These should not panic
	t.Run("NilCallbacks", func(t *testing.T) {
		adapter.dispatchKeyPress(gpucontext.KeyA, 0)
		adapter.dispatchKeyRelease(gpucontext.KeyA, 0)
		adapter.dispatchTextInput("test")
		adapter.dispatchMouseMove(0, 0)
		adapter.dispatchMousePress(gpucontext.MouseButtonLeft, 0, 0)
		adapter.dispatchMouseRelease(gpucontext.MouseButtonLeft, 0, 0)
		adapter.dispatchScroll(0, 0)
		adapter.dispatchResize(800, 600)
		adapter.dispatchFocus(true)
	})
}

// TestIMECallbackRegistration tests IME callback registration.
func TestIMECallbackRegistration(t *testing.T) {
	app := NewApp(DefaultConfig())
	es := app.EventSource()

	t.Run("OnIMECompositionStart", func(t *testing.T) {
		called := false
		es.OnIMECompositionStart(func() {
			called = true
		})
		adapter := es.(*eventSourceAdapter)
		if adapter.onIMECompositionStart == nil {
			t.Error("OnIMECompositionStart callback was not registered")
		}
		_ = called
	})

	t.Run("OnIMECompositionUpdate", func(t *testing.T) {
		es.OnIMECompositionUpdate(func(state gpucontext.IMEState) {})
		adapter := es.(*eventSourceAdapter)
		if adapter.onIMECompositionUpdate == nil {
			t.Error("OnIMECompositionUpdate callback was not registered")
		}
	})

	t.Run("OnIMECompositionEnd", func(t *testing.T) {
		es.OnIMECompositionEnd(func(committed string) {})
		adapter := es.(*eventSourceAdapter)
		if adapter.onIMECompositionEnd == nil {
			t.Error("OnIMECompositionEnd callback was not registered")
		}
	})
}

// TestPointerEventSource tests unified pointer event dispatch.
func TestPointerEventSource(t *testing.T) {
	adapter := &eventSourceAdapter{}

	t.Run("OnPointer registration and dispatch", func(t *testing.T) {
		var received gpucontext.PointerEvent
		adapter.OnPointer(func(ev gpucontext.PointerEvent) {
			received = ev
		})

		ev := gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerID:   1,
			PointerType: gpucontext.PointerTypeMouse,
			X:           150,
			Y:           250,
		}
		adapter.dispatchPointerEvent(ev)

		if received.X != 150 || received.Y != 250 {
			t.Errorf("Pointer event position = (%f, %f), want (150, 250)", received.X, received.Y)
		}
	})

	t.Run("legacy mouse move from pointer", func(t *testing.T) {
		var moveX, moveY float64
		adapter.onMouseMove = func(x, y float64) {
			moveX = x
			moveY = y
		}

		adapter.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerType: gpucontext.PointerTypeMouse,
			X:           42,
			Y:           84,
		})

		if moveX != 42 || moveY != 84 {
			t.Errorf("Legacy mouse move = (%f, %f), want (42, 84)", moveX, moveY)
		}
	})

	t.Run("legacy mouse press from pointer", func(t *testing.T) {
		var pressButton gpucontext.MouseButton
		adapter.onMousePress = func(b gpucontext.MouseButton, x, y float64) {
			pressButton = b
		}

		adapter.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerType: gpucontext.PointerTypeMouse,
			Button:      gpucontext.ButtonRight,
		})

		if pressButton != gpucontext.MouseButtonRight {
			t.Errorf("Legacy press button = %v, want MouseButtonRight", pressButton)
		}
	})

	t.Run("legacy mouse release from pointer", func(t *testing.T) {
		var releaseButton gpucontext.MouseButton
		adapter.onMouseRelease = func(b gpucontext.MouseButton, x, y float64) {
			releaseButton = b
		}

		adapter.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerUp,
			PointerType: gpucontext.PointerTypeMouse,
			Button:      gpucontext.ButtonMiddle,
		})

		if releaseButton != gpucontext.MouseButtonMiddle {
			t.Errorf("Legacy release button = %v, want MouseButtonMiddle", releaseButton)
		}
	})

	t.Run("touch events dont dispatch legacy mouse", func(t *testing.T) {
		moveCalled := false
		adapter.onMouseMove = func(x, y float64) {
			moveCalled = true
		}
		pressCalled := false
		adapter.onMousePress = func(b gpucontext.MouseButton, x, y float64) {
			pressCalled = true
		}

		adapter.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerType: gpucontext.PointerTypeTouch,
		})

		if moveCalled || pressCalled {
			t.Error("Touch events should not dispatch legacy mouse handlers")
		}
	})
}

// TestScrollEventSource tests detailed scroll event dispatch.
func TestScrollEventSource(t *testing.T) {
	adapter := &eventSourceAdapter{}

	t.Run("OnScrollEvent registration and dispatch", func(t *testing.T) {
		var received gpucontext.ScrollEvent
		adapter.OnScrollEvent(func(ev gpucontext.ScrollEvent) {
			received = ev
		})

		ev := gpucontext.ScrollEvent{
			DeltaX: 10.5,
			DeltaY: -20.3,
		}
		adapter.dispatchScrollEventDetailed(ev)

		if received.DeltaX != 10.5 || received.DeltaY != -20.3 {
			t.Errorf("Scroll event delta = (%f, %f), want (10.5, -20.3)", received.DeltaX, received.DeltaY)
		}
	})

	t.Run("legacy scroll from detailed", func(t *testing.T) {
		var dx, dy float64
		adapter.onScroll = func(x, y float64) {
			dx = x
			dy = y
		}

		adapter.dispatchScrollEventDetailed(gpucontext.ScrollEvent{
			DeltaX: 5.0,
			DeltaY: -3.0,
		})

		if dx != 5.0 || dy != -3.0 {
			t.Errorf("Legacy scroll delta = (%f, %f), want (5.0, -3.0)", dx, dy)
		}
	})

	t.Run("nil scroll handlers safe", func(t *testing.T) {
		empty := &eventSourceAdapter{}
		// Should not panic
		empty.dispatchScrollEventDetailed(gpucontext.ScrollEvent{})
	})
}

// TestGestureEventSource tests gesture event dispatch.
func TestGestureEventSource(t *testing.T) {
	adapter := &eventSourceAdapter{}

	t.Run("OnGesture initializes recognizer", func(t *testing.T) {
		if adapter.gestureRecognizer != nil {
			t.Error("gestureRecognizer should be nil initially")
		}

		adapter.OnGesture(func(ev gpucontext.GestureEvent) {})

		if adapter.gestureRecognizer == nil {
			t.Error("gestureRecognizer should be initialized after OnGesture")
		}
	})

	t.Run("gesture recognizer receives pointer events", func(t *testing.T) {
		adapter2 := &eventSourceAdapter{}
		adapter2.OnGesture(func(ev gpucontext.GestureEvent) {})

		adapter2.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:      gpucontext.PointerDown,
			PointerID: 1,
			X:         0,
			Y:         0,
		})

		if adapter2.gestureRecognizer.NumActivePointers() != 1 {
			t.Errorf("NumActivePointers = %d, want 1", adapter2.gestureRecognizer.NumActivePointers())
		}
	})
}

// TestDispatchEndFrame tests end-of-frame gesture dispatch.
func TestDispatchEndFrame(t *testing.T) {
	t.Run("no recognizer", func(t *testing.T) {
		adapter := &eventSourceAdapter{}
		// Should not panic
		adapter.dispatchEndFrame()
	})

	t.Run("recognizer but no gesture callback", func(t *testing.T) {
		adapter := &eventSourceAdapter{
			gestureRecognizer: NewGestureRecognizer(),
		}
		// Should not panic
		adapter.dispatchEndFrame()
	})

	t.Run("gesture dispatched with 2+ pointers", func(t *testing.T) {
		adapter := &eventSourceAdapter{}
		var gestureReceived bool
		adapter.OnGesture(func(ev gpucontext.GestureEvent) {
			gestureReceived = true
		})

		// Add two pointers
		adapter.gestureRecognizer.HandlePointer(gpucontext.PointerEvent{
			Type:      gpucontext.PointerDown,
			PointerID: 1,
			X:         0, Y: 0,
		})
		adapter.gestureRecognizer.HandlePointer(gpucontext.PointerEvent{
			Type:      gpucontext.PointerDown,
			PointerID: 2,
			X:         100, Y: 0,
		})

		adapter.dispatchEndFrame()

		if !gestureReceived {
			t.Error("Gesture should be dispatched with 2 pointers")
		}
	})

	t.Run("gesture not dispatched with <2 pointers", func(t *testing.T) {
		adapter := &eventSourceAdapter{}
		gestureCalled := false
		adapter.OnGesture(func(ev gpucontext.GestureEvent) {
			gestureCalled = true
		})

		// Only one pointer
		adapter.gestureRecognizer.HandlePointer(gpucontext.PointerEvent{
			Type:      gpucontext.PointerDown,
			PointerID: 1,
			X:         0, Y: 0,
		})

		adapter.dispatchEndFrame()

		if gestureCalled {
			t.Error("Gesture should not be dispatched with <2 pointers")
		}
	})
}

// TestButtonToMouseButton tests button conversion.
func TestButtonToMouseButton(t *testing.T) {
	tests := []struct {
		input  gpucontext.Button
		output gpucontext.MouseButton
	}{
		{gpucontext.ButtonLeft, gpucontext.MouseButtonLeft},
		{gpucontext.ButtonRight, gpucontext.MouseButtonRight},
		{gpucontext.ButtonMiddle, gpucontext.MouseButtonMiddle},
		{gpucontext.ButtonX1, gpucontext.MouseButton4},
		{gpucontext.ButtonX2, gpucontext.MouseButton5},
		{gpucontext.Button(99), gpucontext.MouseButtonLeft},
	}

	for _, tt := range tests {
		got := buttonToMouseButton(tt.input)
		if got != tt.output {
			t.Errorf("buttonToMouseButton(%v) = %v, want %v", tt.input, got, tt.output)
		}
	}
}
