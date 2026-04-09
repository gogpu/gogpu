package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// mockPlatform implements platform.Platform for testing.
// Only the methods needed for WindowProvider/PlatformProvider are functional;
// the rest are no-ops.
type mockPlatform struct {
	width, height int
	scaleFactor   float64
	clipboardText string
	cursorID      int
	darkMode      bool
	reduceMotion  bool
	highContrast  bool
	fontScale     float32

	// Frameless window state
	frameless       bool
	maximized       bool
	minimized       bool
	closed          bool
	hitTestCallback func(float64, float64) gpucontext.HitTestResult
}

func (m *mockPlatform) Init(platform.Config) error { return nil }
func (m *mockPlatform) PollEvents() platform.Event { return platform.Event{} }
func (m *mockPlatform) ShouldClose() bool          { return false }
func (m *mockPlatform) LogicalSize() (int, int)    { return m.width, m.height }
func (m *mockPlatform) PhysicalSize() (int, int) {
	s := m.scaleFactor
	if s <= 0 {
		s = 1.0
	}
	return int(float64(m.width) * s), int(float64(m.height) * s)
}
func (m *mockPlatform) GetHandle() (uintptr, uintptr)                                   { return 0, 0 }
func (m *mockPlatform) InSizeMove() bool                                                { return false }
func (m *mockPlatform) SetPointerCallback(func(gpucontext.PointerEvent))                {}
func (m *mockPlatform) SetScrollCallback(func(gpucontext.ScrollEvent))                  {}
func (m *mockPlatform) SetKeyCallback(func(gpucontext.Key, gpucontext.Modifiers, bool)) {}
func (m *mockPlatform) SetCharCallback(func(rune))                                      {}
func (m *mockPlatform) SetModalFrameCallback(func())                                    {}
func (m *mockPlatform) WaitEvents()                                                     {}
func (m *mockPlatform) WakeUp()                                                         {}
func (m *mockPlatform) Destroy()                                                        {}
func (m *mockPlatform) ScaleFactor() float64                                            { return m.scaleFactor }
func (m *mockPlatform) PrepareFrame() platform.PrepareFrameResult {
	w, h := m.PhysicalSize()
	return platform.PrepareFrameResult{
		ScaleFactor:    m.scaleFactor,
		PhysicalWidth:  uint32(w),
		PhysicalHeight: uint32(h),
	}
}
func (m *mockPlatform) ClipboardRead() (string, error)   { return m.clipboardText, nil }
func (m *mockPlatform) ClipboardWrite(text string) error { m.clipboardText = text; return nil }
func (m *mockPlatform) SetCursor(cursorID int)           { m.cursorID = cursorID }
func (m *mockPlatform) SetCursorMode(int)                {}
func (m *mockPlatform) CursorMode() int                  { return 0 }
func (m *mockPlatform) DarkMode() bool                   { return m.darkMode }
func (m *mockPlatform) ReduceMotion() bool               { return m.reduceMotion }
func (m *mockPlatform) HighContrast() bool               { return m.highContrast }
func (m *mockPlatform) FontScale() float32               { return m.fontScale }

func (m *mockPlatform) SyncFrame()          {}
func (m *mockPlatform) SetFrameless(v bool) { m.frameless = v }
func (m *mockPlatform) IsFrameless() bool   { return m.frameless }
func (m *mockPlatform) SetHitTestCallback(fn func(float64, float64) gpucontext.HitTestResult) {
	m.hitTestCallback = fn
}
func (m *mockPlatform) Minimize()         { m.minimized = true }
func (m *mockPlatform) Maximize()         { m.maximized = !m.maximized }
func (m *mockPlatform) IsMaximized() bool { return m.maximized }
func (m *mockPlatform) CloseWindow()      { m.closed = true }

// TestWindowProviderInterface verifies App implements gpucontext.WindowProvider.
func TestWindowProviderInterface(t *testing.T) {
	var _ gpucontext.WindowProvider = (*App)(nil)
}

// TestPlatformProviderInterface verifies App implements gpucontext.PlatformProvider.
func TestPlatformProviderInterface(t *testing.T) {
	var _ gpucontext.PlatformProvider = (*App)(nil)
}

// TestWindowProviderNilPlatform verifies safe defaults when platform is nil.
func TestWindowProviderNilPlatform(t *testing.T) {
	app := NewApp(Config{Width: 800, Height: 600})

	t.Run("Size", func(t *testing.T) {
		w, h := app.Size()
		if w != 800 || h != 600 {
			t.Errorf("Size() = (%d, %d), want (800, 600)", w, h)
		}
	})

	t.Run("ScaleFactor", func(t *testing.T) {
		sf := app.ScaleFactor()
		if sf != 1.0 {
			t.Errorf("ScaleFactor() = %f, want 1.0", sf)
		}
	})

	t.Run("RequestRedraw", func(t *testing.T) {
		app.RequestRedraw() // must not panic with nil invalidator
	})
}

// TestPlatformProviderNilPlatform verifies safe defaults when platform is nil.
func TestPlatformProviderNilPlatform(t *testing.T) {
	app := NewApp(DefaultConfig())

	t.Run("ClipboardRead", func(t *testing.T) {
		text, err := app.ClipboardRead()
		if text != "" || err != nil {
			t.Errorf("ClipboardRead() = (%q, %v), want (\"\", nil)", text, err)
		}
	})

	t.Run("ClipboardWrite", func(t *testing.T) {
		err := app.ClipboardWrite("test")
		if err != nil {
			t.Errorf("ClipboardWrite() = %v, want nil", err)
		}
	})

	t.Run("SetCursor", func(t *testing.T) {
		app.SetCursor(gpucontext.CursorPointer) // must not panic
	})

	t.Run("DarkMode", func(t *testing.T) {
		if app.DarkMode() {
			t.Error("DarkMode() should return false when platform is nil")
		}
	})

	t.Run("ReduceMotion", func(t *testing.T) {
		if app.ReduceMotion() {
			t.Error("ReduceMotion() should return false when platform is nil")
		}
	})

	t.Run("HighContrast", func(t *testing.T) {
		if app.HighContrast() {
			t.Error("HighContrast() should return false when platform is nil")
		}
	})

	t.Run("FontScale", func(t *testing.T) {
		fs := app.FontScale()
		if fs != 1.0 {
			t.Errorf("FontScale() = %f, want 1.0", fs)
		}
	})
}

// TestWindowProviderDelegation verifies App delegates to platform correctly.
func TestWindowProviderDelegation(t *testing.T) {
	mock := &mockPlatform{
		width:       1920,
		height:      1080,
		scaleFactor: 2.0,
	}
	app := &App{platform: mock}

	t.Run("Size", func(t *testing.T) {
		w, h := app.Size()
		if w != 1920 || h != 1080 {
			t.Errorf("Size() = (%d, %d), want (1920, 1080)", w, h)
		}
	})

	t.Run("ScaleFactor", func(t *testing.T) {
		sf := app.ScaleFactor()
		if sf != 2.0 {
			t.Errorf("ScaleFactor() = %f, want 2.0", sf)
		}
	})
}

// TestPlatformProviderDelegation verifies App delegates PlatformProvider to platform.
func TestPlatformProviderDelegation(t *testing.T) {
	mock := &mockPlatform{
		clipboardText: "hello from clipboard",
		darkMode:      true,
		reduceMotion:  true,
		highContrast:  true,
		fontScale:     1.5,
	}
	app := &App{platform: mock}

	t.Run("ClipboardRead", func(t *testing.T) {
		text, err := app.ClipboardRead()
		if err != nil {
			t.Fatalf("ClipboardRead() error = %v", err)
		}
		if text != "hello from clipboard" {
			t.Errorf("ClipboardRead() = %q, want %q", text, "hello from clipboard")
		}
	})

	t.Run("ClipboardWrite", func(t *testing.T) {
		err := app.ClipboardWrite("new text")
		if err != nil {
			t.Fatalf("ClipboardWrite() error = %v", err)
		}
		if mock.clipboardText != "new text" {
			t.Errorf("clipboard = %q, want %q", mock.clipboardText, "new text")
		}
	})

	t.Run("ClipboardRoundTrip", func(t *testing.T) {
		err := app.ClipboardWrite("round trip")
		if err != nil {
			t.Fatalf("ClipboardWrite() error = %v", err)
		}
		text, err := app.ClipboardRead()
		if err != nil {
			t.Fatalf("ClipboardRead() error = %v", err)
		}
		if text != "round trip" {
			t.Errorf("round trip = %q, want %q", text, "round trip")
		}
	})

	t.Run("SetCursor", func(t *testing.T) {
		cursors := []struct {
			shape gpucontext.CursorShape
			id    int
		}{
			{gpucontext.CursorDefault, 0},
			{gpucontext.CursorPointer, 1},
			{gpucontext.CursorText, 2},
			{gpucontext.CursorCrosshair, 3},
			{gpucontext.CursorMove, 4},
			{gpucontext.CursorResizeNS, 5},
			{gpucontext.CursorResizeEW, 6},
			{gpucontext.CursorResizeNWSE, 7},
			{gpucontext.CursorResizeNESW, 8},
			{gpucontext.CursorNotAllowed, 9},
			{gpucontext.CursorWait, 10},
			{gpucontext.CursorNone, 11},
		}
		for _, tc := range cursors {
			app.SetCursor(tc.shape)
			if mock.cursorID != tc.id {
				t.Errorf("SetCursor(%v): platform got cursorID=%d, want %d", tc.shape, mock.cursorID, tc.id)
			}
		}
	})

	t.Run("DarkMode", func(t *testing.T) {
		if !app.DarkMode() {
			t.Error("DarkMode() = false, want true")
		}
	})

	t.Run("ReduceMotion", func(t *testing.T) {
		if !app.ReduceMotion() {
			t.Error("ReduceMotion() = false, want true")
		}
	})

	t.Run("HighContrast", func(t *testing.T) {
		if !app.HighContrast() {
			t.Error("HighContrast() = false, want true")
		}
	})

	t.Run("FontScale", func(t *testing.T) {
		fs := app.FontScale()
		if fs != 1.5 {
			t.Errorf("FontScale() = %f, want 1.5", fs)
		}
	})
}

// TestPlatformProviderFalseValues verifies delegation when platform returns false/default values.
func TestPlatformProviderFalseValues(t *testing.T) {
	mock := &mockPlatform{
		scaleFactor: 1.0,
		fontScale:   1.0,
	}
	app := &App{platform: mock}

	if app.DarkMode() {
		t.Error("DarkMode() should be false")
	}
	if app.ReduceMotion() {
		t.Error("ReduceMotion() should be false")
	}
	if app.HighContrast() {
		t.Error("HighContrast() should be false")
	}

	text, _ := app.ClipboardRead()
	if text != "" {
		t.Errorf("ClipboardRead() = %q, want empty", text)
	}
}
