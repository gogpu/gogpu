package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

// --- helpers -----------------------------------------------------------------

// mockHeaderAlignerWindow extends mockWindow and implements platform.HeaderAligner.
type mockHeaderAlignerWindow struct {
	mockWindow
	headerAlignment int
	alignmentCalled bool
}

func (m *mockHeaderAlignerWindow) SetHeaderAlignment(alignment int) {
	m.headerAlignment = alignment
	m.alignmentCalled = true
}

// --- HeaderAlignment constants -----------------------------------------------

func TestHeaderAlignmentConstants(t *testing.T) {
	if HeaderAlignCenter != 0 {
		t.Errorf("HeaderAlignCenter = %d, want 0", HeaderAlignCenter)
	}
	if HeaderAlignLeft != 1 {
		t.Errorf("HeaderAlignLeft = %d, want 1", HeaderAlignLeft)
	}
	if HeaderAlignRight != 2 {
		t.Errorf("HeaderAlignRight = %d, want 2", HeaderAlignRight)
	}
	// Verify they are distinct.
	if HeaderAlignCenter == HeaderAlignLeft || HeaderAlignCenter == HeaderAlignRight || HeaderAlignLeft == HeaderAlignRight {
		t.Error("HeaderAlignment constants must be distinct")
	}
}

// --- Window.SetTitle / Title -------------------------------------------------

func TestWindow_SetTitle_NilPlatform(t *testing.T) {
	w := &Window{config: Config{Title: "original"}}

	w.SetTitle("new title")
	if w.Title() != "new title" {
		t.Errorf("Title() = %q, want %q", w.Title(), "new title")
	}
}

func TestWindow_SetTitle_UpdatesConfig(t *testing.T) {
	w := &Window{config: Config{Title: "old"}}
	w.SetTitle("updated")
	if w.config.Title != "updated" {
		t.Errorf("config.Title = %q, want %q", w.config.Title, "updated")
	}
}

func TestWindow_SetTitle_DelegatesToPlatform(t *testing.T) {
	rm := &mockTitleWindow{}
	w := &Window{config: Config{Title: "init"}, platWindow: rm}
	w.SetTitle("hello")

	if rm.lastSetTitle != "hello" {
		t.Errorf("platform.SetTitle called with %q, want %q", rm.lastSetTitle, "hello")
	}
	if w.Title() != "hello" {
		t.Errorf("Title() = %q, want %q", w.Title(), "hello")
	}
}

// mockTitleWindow is a minimal platform.PlatformWindow that records SetTitle calls.
type mockTitleWindow struct {
	mockWindow
	lastSetTitle string
}

func (m *mockTitleWindow) SetTitle(title string) { m.lastSetTitle = title }

func TestWindow_Title_DefaultFromConfig(t *testing.T) {
	w := &Window{config: Config{Title: "My App"}}
	if w.Title() != "My App" {
		t.Errorf("Title() = %q, want %q", w.Title(), "My App")
	}
}

// --- Window.SetHeaderAlignment / HeaderAlignment ----------------------------

func TestWindow_SetHeaderAlignment_NilPlatform(t *testing.T) {
	w := &Window{}

	// Must not panic with nil platWindow.
	w.SetHeaderAlignment(HeaderAlignLeft)

	if w.HeaderAlignment() != HeaderAlignLeft {
		t.Errorf("HeaderAlignment() = %d, want HeaderAlignLeft", w.HeaderAlignment())
	}
}

func TestWindow_SetHeaderAlignment_DefaultIsCenter(t *testing.T) {
	w := &Window{}
	if w.HeaderAlignment() != HeaderAlignCenter {
		t.Errorf("default HeaderAlignment() = %d, want HeaderAlignCenter", w.HeaderAlignment())
	}
}

func TestWindow_SetHeaderAlignment_WithHeaderAligner(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}
	w := &Window{platWindow: mock}

	tests := []struct {
		alignment HeaderAlignment
		wantInt   int
	}{
		{HeaderAlignCenter, 0},
		{HeaderAlignLeft, 1},
		{HeaderAlignRight, 2},
	}

	for _, tt := range tests {
		mock.alignmentCalled = false
		w.SetHeaderAlignment(tt.alignment)

		if !mock.alignmentCalled {
			t.Errorf("alignment %d: SetHeaderAlignment was not called on platform", tt.alignment)
		}
		if mock.headerAlignment != tt.wantInt {
			t.Errorf("alignment %d: platform got %d, want %d", tt.alignment, mock.headerAlignment, tt.wantInt)
		}
		if w.HeaderAlignment() != tt.alignment {
			t.Errorf("HeaderAlignment() = %d, want %d", w.HeaderAlignment(), tt.alignment)
		}
	}
}

func TestWindow_SetHeaderAlignment_NonAlignerPlatform(t *testing.T) {
	// mockWindow does not implement HeaderAligner — must not panic.
	mock := &mockWindow{}
	w := &Window{platWindow: mock}

	w.SetHeaderAlignment(HeaderAlignLeft) // must not panic
	if w.HeaderAlignment() != HeaderAlignLeft {
		t.Errorf("HeaderAlignment() = %d, want HeaderAlignLeft", w.HeaderAlignment())
	}
}

func TestWindow_HeaderAlignment_RoundTrip(t *testing.T) {
	w := &Window{}

	for _, a := range []HeaderAlignment{HeaderAlignCenter, HeaderAlignLeft, HeaderAlignRight} {
		w.SetHeaderAlignment(a)
		if got := w.HeaderAlignment(); got != a {
			t.Errorf("HeaderAlignment() = %d after Set(%d)", got, a)
		}
	}
}

// --- App.SetHeaderAlignment / HeaderAlignment --------------------------------

func TestApp_SetHeaderAlignment_NilPlatform(t *testing.T) {
	app := NewApp(DefaultConfig())

	app.SetHeaderAlignment(HeaderAlignLeft) // must not panic
	if app.HeaderAlignment() != HeaderAlignLeft {
		t.Errorf("HeaderAlignment() = %d, want HeaderAlignLeft", app.HeaderAlignment())
	}
}

func TestApp_SetHeaderAlignment_DefaultIsCenter(t *testing.T) {
	app := NewApp(DefaultConfig())
	if app.HeaderAlignment() != HeaderAlignCenter {
		t.Errorf("default HeaderAlignment() = %d, want HeaderAlignCenter", app.HeaderAlignment())
	}
}

func TestApp_SetHeaderAlignment_WithPlatformWindow(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}
	app := &App{platWindow: mock, config: DefaultConfig()}

	app.SetHeaderAlignment(HeaderAlignRight)

	if !mock.alignmentCalled {
		t.Error("SetHeaderAlignment was not delegated to platform window")
	}
	if mock.headerAlignment != 2 {
		t.Errorf("platform got alignment=%d, want 2 (Right)", mock.headerAlignment)
	}
	if app.HeaderAlignment() != HeaderAlignRight {
		t.Errorf("HeaderAlignment() = %d, want HeaderAlignRight", app.HeaderAlignment())
	}
}

func TestApp_SetHeaderAlignment_DelegatesToPrimaryWindow(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}
	wm := newWindowManager()
	id := wm.allocate()
	primary := &Window{id: id, platWindow: mock}
	wm.add(primary)

	app := &App{
		config:        DefaultConfig(),
		primaryWindow: primary,
		windowManager: wm,
	}

	app.SetHeaderAlignment(HeaderAlignLeft)

	if !mock.alignmentCalled {
		t.Error("SetHeaderAlignment not delegated to primary window platform")
	}
	if primary.HeaderAlignment() != HeaderAlignLeft {
		t.Errorf("primary window HeaderAlignment() = %d, want HeaderAlignLeft", primary.HeaderAlignment())
	}
	if app.HeaderAlignment() != HeaderAlignLeft {
		t.Errorf("App.HeaderAlignment() = %d, want HeaderAlignLeft", app.HeaderAlignment())
	}
}

func TestApp_SetHeaderAlignment_StoresWhenNoPlatformWindow(t *testing.T) {
	app := &App{config: DefaultConfig()}

	app.SetHeaderAlignment(HeaderAlignRight)

	if app.config.HeaderAlignment != HeaderAlignRight {
		t.Errorf("config.HeaderAlignment = %d, want HeaderAlignRight", app.config.HeaderAlignment)
	}
	if app.HeaderAlignment() != HeaderAlignRight {
		t.Errorf("HeaderAlignment() = %d, want HeaderAlignRight", app.HeaderAlignment())
	}
}

func TestApp_HeaderAlignment_ReadsFromPrimaryWindowWhenSet(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}
	wm := newWindowManager()
	id := wm.allocate()
	primary := &Window{id: id, platWindow: mock, headerAlignment: HeaderAlignRight}
	wm.add(primary)

	app := &App{
		config:        DefaultConfig(),
		primaryWindow: primary,
		windowManager: wm,
	}

	if app.HeaderAlignment() != HeaderAlignRight {
		t.Errorf("HeaderAlignment() = %d, want HeaderAlignRight (from primary window)", app.HeaderAlignment())
	}
}

// --- Config.WithHeaderAlignment ----------------------------------------------

func TestConfigWithHeaderAlignment_Default(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.HeaderAlignment != HeaderAlignCenter {
		t.Errorf("DefaultConfig().HeaderAlignment = %d, want HeaderAlignCenter", cfg.HeaderAlignment)
	}
}

func TestConfigWithHeaderAlignment_Left(t *testing.T) {
	cfg := DefaultConfig().WithHeaderAlignment(HeaderAlignLeft)
	if cfg.HeaderAlignment != HeaderAlignLeft {
		t.Errorf("HeaderAlignment = %d, want HeaderAlignLeft", cfg.HeaderAlignment)
	}
}

func TestConfigWithHeaderAlignment_Right(t *testing.T) {
	cfg := DefaultConfig().WithHeaderAlignment(HeaderAlignRight)
	if cfg.HeaderAlignment != HeaderAlignRight {
		t.Errorf("HeaderAlignment = %d, want HeaderAlignRight", cfg.HeaderAlignment)
	}
}

func TestConfigWithHeaderAlignment_Center(t *testing.T) {
	cfg := DefaultConfig().
		WithHeaderAlignment(HeaderAlignLeft).
		WithHeaderAlignment(HeaderAlignCenter)
	if cfg.HeaderAlignment != HeaderAlignCenter {
		t.Errorf("HeaderAlignment = %d, want HeaderAlignCenter after reset", cfg.HeaderAlignment)
	}
}

func TestConfigWithHeaderAlignment_DoesNotAffectOtherFields(t *testing.T) {
	base := DefaultConfig().WithTitle("Test").WithSize(1024, 768)
	cfg := base.WithHeaderAlignment(HeaderAlignLeft)

	if cfg.Title != "Test" {
		t.Errorf("Title = %q, want %q", cfg.Title, "Test")
	}
	if cfg.Width != 1024 || cfg.Height != 768 {
		t.Errorf("Size = (%d, %d), want (1024, 768)", cfg.Width, cfg.Height)
	}
	if cfg.HeaderAlignment != HeaderAlignLeft {
		t.Errorf("HeaderAlignment = %d, want HeaderAlignLeft", cfg.HeaderAlignment)
	}
}

// --- applyHeaderAlignment helper ---------------------------------------------

func TestApplyHeaderAlignment_SkipsNonAligner(t *testing.T) {
	// mockWindow does not implement HeaderAligner — must not panic.
	mock := &mockWindow{}
	applyHeaderAlignment(mock, HeaderAlignLeft) // must not panic
}

func TestApplyHeaderAlignment_DelegatesToAligner(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}

	applyHeaderAlignment(mock, HeaderAlignLeft)
	if mock.headerAlignment != 1 {
		t.Errorf("headerAlignment = %d, want 1 (Left)", mock.headerAlignment)
	}

	applyHeaderAlignment(mock, HeaderAlignRight)
	if mock.headerAlignment != 2 {
		t.Errorf("headerAlignment = %d, want 2 (Right)", mock.headerAlignment)
	}

	applyHeaderAlignment(mock, HeaderAlignCenter)
	if mock.headerAlignment != 0 {
		t.Errorf("headerAlignment = %d, want 0 (Center)", mock.headerAlignment)
	}
}

// --- HeaderAligner interface compliance -------------------------------------

func TestHeaderAlignerInterface(t *testing.T) {
	// Compile-time check: mockHeaderAlignerWindow implements platform.HeaderAligner.
	var _ platform.HeaderAligner = (*mockHeaderAlignerWindow)(nil)
}

// --- Window header alignment propagates through config ----------------------

func TestWindow_HeaderAlignmentFromConfig(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}
	cfg := Config{
		Width:           800,
		Height:          600,
		HeaderAlignment: HeaderAlignLeft,
	}
	w := &Window{
		config:          cfg,
		platWindow:      mock,
		headerAlignment: cfg.HeaderAlignment,
	}
	// Simulate what NewWindow does after creation.
	if cfg.HeaderAlignment != HeaderAlignCenter {
		applyHeaderAlignment(w.platWindow, cfg.HeaderAlignment)
	}

	if !mock.alignmentCalled {
		t.Error("SetHeaderAlignment not called on platform window")
	}
	if mock.headerAlignment != 1 {
		t.Errorf("platform headerAlignment = %d, want 1 (Left)", mock.headerAlignment)
	}
	if w.HeaderAlignment() != HeaderAlignLeft {
		t.Errorf("w.HeaderAlignment() = %d, want HeaderAlignLeft", w.HeaderAlignment())
	}
}

func TestWindow_HeaderAlignmentCenterSkipsApply(t *testing.T) {
	mock := &mockHeaderAlignerWindow{}
	cfg := Config{HeaderAlignment: HeaderAlignCenter}
	// Simulate NewWindow path: Center means no call to applyHeaderAlignment.
	if cfg.HeaderAlignment != HeaderAlignCenter {
		applyHeaderAlignment(mock, cfg.HeaderAlignment)
	}
	if mock.alignmentCalled {
		t.Error("applyHeaderAlignment should not be called for HeaderAlignCenter")
	}
}

// --- Window.SetTitle per-window independence --------------------------------

func TestWindow_SetTitle_IndependentOfApp(t *testing.T) {
	rm1 := &mockTitleWindow{}
	rm2 := &mockTitleWindow{}

	wm := newWindowManager()
	id1, id2 := wm.allocate(), wm.allocate()
	w1 := &Window{id: id1, platWindow: rm1, config: Config{Title: "Window1"}}
	w2 := &Window{id: id2, platWindow: rm2, config: Config{Title: "Window2"}}
	wm.add(w1)
	wm.add(w2)

	w1.SetTitle("New Title 1")
	w2.SetTitle("New Title 2")

	if w1.Title() != "New Title 1" {
		t.Errorf("w1.Title() = %q, want %q", w1.Title(), "New Title 1")
	}
	if w2.Title() != "New Title 2" {
		t.Errorf("w2.Title() = %q, want %q", w2.Title(), "New Title 2")
	}
	if rm1.lastSetTitle != "New Title 1" {
		t.Errorf("rm1.lastSetTitle = %q, want %q", rm1.lastSetTitle, "New Title 1")
	}
	if rm2.lastSetTitle != "New Title 2" {
		t.Errorf("rm2.lastSetTitle = %q, want %q", rm2.lastSetTitle, "New Title 2")
	}
	// Verify windows don't interfere with each other.
	if w1.Title() == w2.Title() {
		t.Error("w1 and w2 titles should be independent")
	}
}
