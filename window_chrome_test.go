package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

// TestWindowChromeInterface verifies App implements gpucontext.WindowChrome.
func TestWindowChromeInterface(t *testing.T) {
	var _ gpucontext.WindowChrome = (*App)(nil)
}

// TestWindowChromeNilPlatform verifies safe defaults when platform is nil.
func TestWindowChromeNilPlatform(t *testing.T) {
	app := NewApp(DefaultConfig())

	t.Run("SetFrameless", func(t *testing.T) {
		app.SetFrameless(true) // must not panic
	})

	t.Run("IsFrameless", func(t *testing.T) {
		if app.IsFrameless() {
			t.Error("IsFrameless() should return false when platform is nil")
		}
	})

	t.Run("SetHitTestCallback", func(t *testing.T) {
		app.SetHitTestCallback(func(x, y float64) gpucontext.HitTestResult {
			return gpucontext.HitTestCaption
		})
		// must not panic
	})

	t.Run("SetHitTestCallbackNil", func(t *testing.T) {
		app.SetHitTestCallback(nil) // must not panic
	})

	t.Run("Minimize", func(t *testing.T) {
		app.Minimize() // must not panic
	})

	t.Run("Maximize", func(t *testing.T) {
		app.Maximize() // must not panic
	})

	t.Run("IsMaximized", func(t *testing.T) {
		if app.IsMaximized() {
			t.Error("IsMaximized() should return false when platform is nil")
		}
	})

	t.Run("Close", func(t *testing.T) {
		app.Close() // must not panic
	})
}

// TestWindowChromeWithPlatform verifies WindowChrome delegates to platform.
func TestWindowChromeWithPlatform(t *testing.T) {
	mock := &mockWindow{width: 800, height: 600, scaleFactor: 1.0}
	app := &App{
		config:     Config{Width: 800, Height: 600},
		platWindow: mock,
	}

	t.Run("SetFramelessAndQuery", func(t *testing.T) {
		if app.IsFrameless() {
			t.Error("IsFrameless() should be false initially")
		}

		app.SetFrameless(true)
		if !mock.frameless {
			t.Error("SetFrameless(true) should set mock.frameless")
		}
		if !app.IsFrameless() {
			t.Error("IsFrameless() should return true after SetFrameless(true)")
		}

		app.SetFrameless(false)
		if mock.frameless {
			t.Error("SetFrameless(false) should clear mock.frameless")
		}
		if app.IsFrameless() {
			t.Error("IsFrameless() should return false after SetFrameless(false)")
		}
	})

	t.Run("Minimize", func(t *testing.T) {
		mock.minimized = false
		app.Minimize()
		if !mock.minimized {
			t.Error("Minimize() should delegate to platform")
		}
	})

	t.Run("MaximizeToggle", func(t *testing.T) {
		mock.maximized = false

		app.Maximize()
		if !app.IsMaximized() {
			t.Error("IsMaximized() should be true after first Maximize()")
		}

		app.Maximize()
		if app.IsMaximized() {
			t.Error("IsMaximized() should be false after second Maximize() (toggle)")
		}
	})

	t.Run("Close", func(t *testing.T) {
		mock.closed = false
		app.Close()
		if !mock.closed {
			t.Error("Close() should delegate to platform")
		}
	})

	t.Run("SetHitTestCallback", func(t *testing.T) {
		called := false
		app.SetHitTestCallback(func(x, y float64) gpucontext.HitTestResult {
			called = true
			if y < 40 {
				return gpucontext.HitTestCaption
			}
			return gpucontext.HitTestClient
		})

		if mock.hitTestCallback == nil {
			t.Fatal("SetHitTestCallback should set platform callback")
		}

		// Test the callback forwards correctly
		result := mock.hitTestCallback(100, 10) // y < 40
		if !called {
			t.Error("HitTestCallback should have been called")
		}
		if result != gpucontext.HitTestCaption {
			t.Errorf("callback(100, 10) = %v, want Caption", result)
		}

		result = mock.hitTestCallback(100, 200) // y >= 40
		if result != gpucontext.HitTestClient {
			t.Errorf("callback(100, 200) = %v, want Client", result)
		}
	})

	t.Run("SetHitTestCallbackNil", func(t *testing.T) {
		app.SetHitTestCallback(nil)

		// Platform callback should still be set (wraps nil safely)
		if mock.hitTestCallback == nil {
			t.Fatal("platform callback should still be set")
		}

		// Nil user callback should return HitTestClient
		result := mock.hitTestCallback(100, 100)
		if result != gpucontext.HitTestClient {
			t.Errorf("nil callback should return Client, got %v", result)
		}
	})
}

// TestConfigWithFrameless verifies the WithFrameless builder method.
func TestConfigWithFrameless(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.Frameless {
			t.Error("DefaultConfig().Frameless should be false")
		}
	})

	t.Run("WithFramelessTrue", func(t *testing.T) {
		cfg := DefaultConfig().WithFrameless(true)
		if !cfg.Frameless {
			t.Error("WithFrameless(true) should set Frameless to true")
		}
	})

	t.Run("WithFramelessFalse", func(t *testing.T) {
		cfg := DefaultConfig().WithFrameless(true).WithFrameless(false)
		if cfg.Frameless {
			t.Error("WithFrameless(false) should set Frameless to false")
		}
	})

	t.Run("DoesNotAffectOtherFields", func(t *testing.T) {
		base := DefaultConfig().WithTitle("Test").WithSize(1024, 768)
		cfg := base.WithFrameless(true)

		if cfg.Title != "Test" {
			t.Errorf("Title = %q, want \"Test\"", cfg.Title)
		}
		if cfg.Width != 1024 || cfg.Height != 768 {
			t.Errorf("Size = (%d, %d), want (1024, 768)", cfg.Width, cfg.Height)
		}
		if !cfg.Frameless {
			t.Error("Frameless should be true")
		}
	})
}

// TestHitTestResultMapping verifies all HitTestResult values have correct String().
func TestHitTestResultMapping(t *testing.T) {
	tests := []struct {
		result gpucontext.HitTestResult
		name   string
	}{
		{gpucontext.HitTestClient, "Client"},
		{gpucontext.HitTestCaption, "Caption"},
		{gpucontext.HitTestClose, "Close"},
		{gpucontext.HitTestMaximize, "Maximize"},
		{gpucontext.HitTestMinimize, "Minimize"},
		{gpucontext.HitTestResizeN, "ResizeN"},
		{gpucontext.HitTestResizeS, "ResizeS"},
		{gpucontext.HitTestResizeW, "ResizeW"},
		{gpucontext.HitTestResizeE, "ResizeE"},
		{gpucontext.HitTestResizeNW, "ResizeNW"},
		{gpucontext.HitTestResizeNE, "ResizeNE"},
		{gpucontext.HitTestResizeSW, "ResizeSW"},
		{gpucontext.HitTestResizeSE, "ResizeSE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.String(); got != tt.name {
				t.Errorf("HitTestResult(%d).String() = %q, want %q", tt.result, got, tt.name)
			}
		})
	}
}

// TestHitTestCallbackRegions verifies a typical title bar hit test layout.
func TestHitTestCallbackRegions(t *testing.T) {
	const (
		windowWidth  = 800
		titleHeight  = 40
		buttonWidth  = 46
		resizeBorder = 5
	)

	hitTest := func(x, y float64) gpucontext.HitTestResult {
		// Resize edges
		if y < resizeBorder {
			if x < resizeBorder {
				return gpucontext.HitTestResizeNW
			}
			if x > windowWidth-resizeBorder {
				return gpucontext.HitTestResizeNE
			}
			return gpucontext.HitTestResizeN
		}
		if x < resizeBorder {
			return gpucontext.HitTestResizeW
		}
		if x > windowWidth-resizeBorder {
			return gpucontext.HitTestResizeE
		}

		// Title bar region
		if y < titleHeight {
			// Window control buttons (right side)
			if x > windowWidth-buttonWidth {
				return gpucontext.HitTestClose
			}
			if x > windowWidth-2*buttonWidth {
				return gpucontext.HitTestMaximize
			}
			if x > windowWidth-3*buttonWidth {
				return gpucontext.HitTestMinimize
			}
			return gpucontext.HitTestCaption
		}

		return gpucontext.HitTestClient
	}

	tests := []struct {
		name string
		x, y float64
		want gpucontext.HitTestResult
	}{
		// Client area
		{"center", 400, 300, gpucontext.HitTestClient},
		{"below_titlebar", 400, 50, gpucontext.HitTestClient},

		// Title bar drag area
		{"caption_left", 100, 20, gpucontext.HitTestCaption},
		{"caption_center", 400, 20, gpucontext.HitTestCaption},

		// Window buttons
		{"close_button", 780, 20, gpucontext.HitTestClose},
		{"maximize_button", 730, 20, gpucontext.HitTestMaximize},
		{"minimize_button", 680, 20, gpucontext.HitTestMinimize},

		// Resize edges
		{"resize_top", 400, 2, gpucontext.HitTestResizeN},
		{"resize_left", 2, 300, gpucontext.HitTestResizeW},
		{"resize_right", 798, 300, gpucontext.HitTestResizeE},

		// Resize corners
		{"resize_nw", 2, 2, gpucontext.HitTestResizeNW},
		{"resize_ne", 798, 2, gpucontext.HitTestResizeNE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hitTest(tt.x, tt.y)
			if got != tt.want {
				t.Errorf("hitTest(%.0f, %.0f) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}
