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
