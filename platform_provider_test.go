package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// mockWindow implements platform.PlatformWindow for testing.
// Only the methods needed for WindowProvider/PlatformProvider are functional;
// the rest are no-ops.
type mockWindow struct {
	windowID    platform.WindowID
	width       int
	height      int
	scaleFactor float64
	cursorID    int

	// Frameless window state
	frameless       bool
	fullscreen      bool
	maximized       bool
	minimized       bool
	closed          bool
	hitTestCallback func(float64, float64) gpucontext.HitTestResult
	closeFn         func() bool
}

func (m *mockWindow) ID() platform.WindowID   { return m.windowID }
func (m *mockWindow) ShouldClose() bool       { return false }
func (m *mockWindow) LogicalSize() (int, int) { return m.width, m.height }
func (m *mockWindow) PhysicalSize() (int, int) {
	s := m.scaleFactor
	if s <= 0 {
		s = 1.0
	}
	return int(float64(m.width) * s), int(float64(m.height) * s)
}
func (m *mockWindow) GetHandle() (uintptr, uintptr) { return 0, 0 }
func (m *mockWindow) InSizeMove() bool              { return false }
func (m *mockWindow) SetTitle(_ string)             {}
func (m *mockWindow) SetModalFrameCallback(func())  {}
func (m *mockWindow) Destroy()                      {}
func (m *mockWindow) ScaleFactor() float64          { return m.scaleFactor }
func (m *mockWindow) PrepareFrame() platform.PrepareFrameResult {
	w, h := m.PhysicalSize()
	return platform.PrepareFrameResult{
		ScaleFactor:    m.scaleFactor,
		PhysicalWidth:  uint32(w),
		PhysicalHeight: uint32(h),
	}
}
func (m *mockWindow) SetCursor(cursorID int) { m.cursorID = cursorID }
func (m *mockWindow) SetCursorMode(int)      {}
func (m *mockWindow) CursorMode() int        { return 0 }
func (m *mockWindow) SyncFrame()             {}
func (m *mockWindow) SetFrameless(v bool)    { m.frameless = v }
func (m *mockWindow) IsFrameless() bool      { return m.frameless }
func (m *mockWindow) SetHitTestCallback(fn func(float64, float64) gpucontext.HitTestResult) {
	m.hitTestCallback = fn
}
func (m *mockWindow) Minimize()            { m.minimized = true }
func (m *mockWindow) Maximize()            { m.maximized = !m.maximized }
func (m *mockWindow) IsMaximized() bool    { return m.maximized }
func (m *mockWindow) SetFullscreen(v bool) { m.fullscreen = v }
func (m *mockWindow) IsFullscreen() bool   { return m.fullscreen }
func (m *mockWindow) Close()               { m.closed = true }
func (m *mockWindow) Show()                {}

// mockManager implements platform.PlatformManager for testing.
type mockManager struct {
	clipboardText  string
	darkMode       bool
	reduceMotion   bool
	highContrast   bool
	fontScale      float32
	subpixelLayout gpucontext.SubpixelLayout

	// dialog stubs — set these to control what ShowOpen/SaveFileDialog return
	openDialogPaths []string
	saveDialogPath  string
	// last opts received — inspected in delegation tests
	lastDialogOpts platform.FileDialogOptions
}

func (m *mockManager) Init() error { return nil }
func (m *mockManager) CreateWindow(platform.Config) (platform.PlatformWindow, error) {
	return &mockWindow{}, nil
}
func (m *mockManager) PollEvents() platform.Event                { return platform.Event{} }
func (m *mockManager) WaitEvents()                               {}
func (m *mockManager) WakeUp()                                   {}
func (m *mockManager) ClipboardRead() (string, error)            { return m.clipboardText, nil }
func (m *mockManager) ClipboardWrite(text string) error          { m.clipboardText = text; return nil }
func (m *mockManager) DarkMode() bool                            { return m.darkMode }
func (m *mockManager) ReduceMotion() bool                        { return m.reduceMotion }
func (m *mockManager) HighContrast() bool                        { return m.highContrast }
func (m *mockManager) FontScale() float32                        { return m.fontScale }
func (m *mockManager) SubpixelLayout() gpucontext.SubpixelLayout { return m.subpixelLayout }
func (m *mockManager) SetAppName(name string)                    {}
func (m *mockManager) ShowOpenFileDialog(opts platform.FileDialogOptions) ([]string, error) {
	m.lastDialogOpts = opts
	return m.openDialogPaths, nil
}
func (m *mockManager) ShowSaveFileDialog(opts platform.FileDialogOptions) (string, error) {
	m.lastDialogOpts = opts
	return m.saveDialogPath, nil
}
func (m *mockManager) Destroy()                 {}
func (m *mockWindow) SetOnClose(fn func() bool) { m.closeFn = fn }

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

	t.Run("SubpixelLayout", func(t *testing.T) {
		sl := app.SubpixelLayout()
		if sl != gpucontext.SubpixelNone {
			t.Errorf("SubpixelLayout() = %v, want SubpixelNone", sl)
		}
	})
}

// TestWindowProviderDelegation verifies App delegates to platform correctly.
func TestWindowProviderDelegation(t *testing.T) {
	mock := &mockWindow{
		width:       1920,
		height:      1080,
		scaleFactor: 2.0,
	}
	app := &App{platWindow: mock}

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
	mockMgr := &mockManager{
		clipboardText:  "hello from clipboard",
		darkMode:       true,
		reduceMotion:   true,
		highContrast:   true,
		fontScale:      1.5,
		subpixelLayout: gpucontext.SubpixelBGR,
	}
	mockWin := &mockWindow{
		width:       800,
		height:      600,
		scaleFactor: 1.0,
	}
	app := &App{manager: mockMgr, platWindow: mockWin}

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
		if mockMgr.clipboardText != "new text" {
			t.Errorf("clipboard = %q, want %q", mockMgr.clipboardText, "new text")
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
			if mockWin.cursorID != tc.id {
				t.Errorf("SetCursor(%v): platform got cursorID=%d, want %d", tc.shape, mockWin.cursorID, tc.id)
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

	t.Run("SubpixelLayout", func(t *testing.T) {
		sl := app.SubpixelLayout()
		if sl != gpucontext.SubpixelBGR {
			t.Errorf("SubpixelLayout() = %v, want SubpixelBGR", sl)
		}
	})
}

// TestFileDialogNilPlatform verifies ShowOpen/SaveFileDialog are safe when manager is nil.
func TestFileDialogNilPlatform(t *testing.T) {
	app := NewApp(DefaultConfig())

	t.Run("ShowOpenFileDialog", func(t *testing.T) {
		paths, err := app.ShowOpenFileDialog(FileDialogOptions{Title: "Open"})
		if err != nil {
			t.Errorf("ShowOpenFileDialog() error = %v, want nil", err)
		}
		if paths != nil {
			t.Errorf("ShowOpenFileDialog() = %v, want nil", paths)
		}
	})

	t.Run("ShowSaveFileDialog", func(t *testing.T) {
		path, err := app.ShowSaveFileDialog(FileDialogOptions{Title: "Save"})
		if err != nil {
			t.Errorf("ShowSaveFileDialog() error = %v, want nil", err)
		}
		if path != "" {
			t.Errorf("ShowSaveFileDialog() = %q, want empty", path)
		}
	})
}

// TestFileDialogDelegation verifies App forwards options and returns results from the manager.
func TestFileDialogDelegation(t *testing.T) {
	mgr := &mockManager{
		openDialogPaths: []string{"/tmp/a.png", "/tmp/b.png"},
		saveDialogPath:  "/tmp/out.txt",
	}
	app := &App{manager: mgr}

	openOpts := FileDialogOptions{
		Title:    "Pick images",
		Multiple: true,
		Filters: []FileTypeFilter{
			{Name: "Images", Extensions: []string{"*.png", "*.jpg"}},
		},
	}

	t.Run("ShowOpenFileDialog delegates opts and returns paths", func(t *testing.T) {
		paths, err := app.ShowOpenFileDialog(openOpts)
		if err != nil {
			t.Fatalf("ShowOpenFileDialog() error = %v", err)
		}
		if len(paths) != 2 || paths[0] != "/tmp/a.png" || paths[1] != "/tmp/b.png" {
			t.Errorf("ShowOpenFileDialog() = %v, want [/tmp/a.png /tmp/b.png]", paths)
		}
		if mgr.lastDialogOpts.Title != "Pick images" {
			t.Errorf("delegated Title = %q, want %q", mgr.lastDialogOpts.Title, "Pick images")
		}
		if !mgr.lastDialogOpts.Multiple {
			t.Error("delegated Multiple should be true")
		}
		if len(mgr.lastDialogOpts.Filters) != 1 {
			t.Errorf("delegated Filters len = %d, want 1", len(mgr.lastDialogOpts.Filters))
		}
	})

	saveOpts := FileDialogOptions{
		Title:           "Save log",
		DefaultFilename: "out.txt",
	}

	t.Run("ShowSaveFileDialog delegates opts and returns path", func(t *testing.T) {
		path, err := app.ShowSaveFileDialog(saveOpts)
		if err != nil {
			t.Fatalf("ShowSaveFileDialog() error = %v", err)
		}
		if path != "/tmp/out.txt" {
			t.Errorf("ShowSaveFileDialog() = %q, want %q", path, "/tmp/out.txt")
		}
		if mgr.lastDialogOpts.Title != "Save log" {
			t.Errorf("delegated Title = %q, want %q", mgr.lastDialogOpts.Title, "Save log")
		}
		if mgr.lastDialogOpts.DefaultFilename != "out.txt" {
			t.Errorf("delegated DefaultFilename = %q, want %q", mgr.lastDialogOpts.DefaultFilename, "out.txt")
		}
	})

	t.Run("ShowOpenFileDialog cancel returns nil nil", func(t *testing.T) {
		mgr.openDialogPaths = nil
		paths, err := app.ShowOpenFileDialog(FileDialogOptions{})
		if err != nil || paths != nil {
			t.Errorf("ShowOpenFileDialog() = (%v, %v), want (nil, nil)", paths, err)
		}
	})

	t.Run("ShowSaveFileDialog cancel returns empty nil", func(t *testing.T) {
		mgr.saveDialogPath = ""
		path, err := app.ShowSaveFileDialog(FileDialogOptions{})
		if err != nil || path != "" {
			t.Errorf("ShowSaveFileDialog() = (%q, %v), want (\"\", nil)", path, err)
		}
	})
}

// TestFileDialogOptions verifies struct fields round-trip correctly.
func TestFileDialogOptions(t *testing.T) {
	opts := FileDialogOptions{
		Title:            "My Dialog",
		Directory:        true,
		Multiple:         true,
		InitialDirectory: "/home/user",
		DefaultFilename:  "report.pdf",
		Filters: []FileTypeFilter{
			{Name: "PDF", Extensions: []string{"*.pdf"}},
			{Name: "All", Extensions: []string{"*"}},
		},
	}

	if opts.Title != "My Dialog" {
		t.Errorf("Title = %q", opts.Title)
	}
	if !opts.Directory {
		t.Error("Directory should be true")
	}
	if !opts.Multiple {
		t.Error("Multiple should be true")
	}
	if opts.InitialDirectory != "/home/user" {
		t.Errorf("InitialDirectory = %q", opts.InitialDirectory)
	}
	if opts.DefaultFilename != "report.pdf" {
		t.Errorf("DefaultFilename = %q", opts.DefaultFilename)
	}
	if len(opts.Filters) != 2 || opts.Filters[0].Name != "PDF" {
		t.Errorf("Filters = %v", opts.Filters)
	}
}

// TestPlatformProviderFalseValues verifies delegation when platform returns false/default values.
func TestPlatformProviderFalseValues(t *testing.T) {
	mockMgr := &mockManager{fontScale: 1.0}
	app := &App{manager: mockMgr, platWindow: &mockWindow{scaleFactor: 1.0}}

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

	sl := app.SubpixelLayout()
	if sl != gpucontext.SubpixelNone {
		t.Errorf("SubpixelLayout() = %v, want SubpixelNone (default)", sl)
	}
}
