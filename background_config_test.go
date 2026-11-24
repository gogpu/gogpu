package gogpu

import (
	"errors"
	"runtime"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

// The activation policy is macOS-only: the platform manager takes it before
// Init, so the app never activates.
func TestStartHiddenSetsPolicyBeforeInitOnMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the activation policy is specific to macOS")
	}
	for _, hidden := range []bool{false, true} {
		manager := &backgroundStartupManager{}
		original := newPlatformManagerFn
		newPlatformManagerFn = func() platform.PlatformManager { return manager }
		t.Cleanup(func() { newPlatformManagerFn = original })
		app := &App{config: DefaultConfig().WithStartHidden(hidden)}
		_, err := app.initPlatform()
		if !errors.Is(err, errStopBackgroundInit) {
			t.Fatalf("startup did not reach the fake manager: %v", err)
		}
		if !manager.configured || manager.backgroundAtInit != hidden {
			t.Fatalf("start hidden = %t: policy set = %t, policy at init = %t; want %t",
				hidden, manager.configured, manager.backgroundAtInit, hidden)
		}
		newPlatformManagerFn = original
	}
}

var errStopBackgroundInit = errors.New("stop before creating the window")

type backgroundStartupManager struct {
	platform.PlatformManager
	configured, background, backgroundAtInit bool
}

func (m *backgroundStartupManager) SetBackground(background bool) {
	m.configured = true
	m.background = background
}

func (m *backgroundStartupManager) Init() error {
	m.backgroundAtInit = m.background
	return errStopBackgroundInit
}

func TestNormalStartupShowsPrimaryWindow(t *testing.T) {
	original := DefaultConfig()
	enabled := original.WithStartHidden(true)
	disabled := enabled.WithStartHidden(false)
	if original.StartHidden || !enabled.StartHidden || disabled.StartHidden {
		t.Fatal("start hidden should default to false and its builder should return a copy")
	}

	native := &backgroundTestWindow{}
	app := &App{config: disabled, platWindow: native, renderer: &Renderer{}, renderLoop: &startupFrameRecorder{}}
	app.startPrimaryWindow(native)
	if native.shows != 1 || !app.primaryWindow.Visible() {
		t.Fatalf("normal startup: native shows %d, visible %t", native.shows, app.primaryWindow.Visible())
	}
}

// A window started hidden is not shown, still renders, and is shown by an
// explicit Show, on every platform.
func TestStartHiddenKeepsWindowHiddenUntilShown(t *testing.T) {
	window := &backgroundTestWindow{}
	loop := &startupFrameRecorder{}
	app := &App{
		config: DefaultConfig().WithStartHidden(true), platWindow: window,
		renderer: &Renderer{}, renderLoop: loop,
	}
	app.startPrimaryWindow(window)
	primary := app.primaryWindow
	if window.shows != 0 || primary.Visible() {
		t.Fatalf("startup: native shows %d, visible %t; want no Show and a hidden window",
			window.shows, primary.Visible())
	}

	before := loop.calls
	app.renderFrameMultiThread()
	if loop.calls != before+1 {
		t.Fatal("hidden startup window did not reach the render loop")
	}

	primary.Show()
	if window.shows != 1 || !primary.Visible() {
		t.Fatal("an explicit Show should reach the window and mark it visible")
	}
	primary.Hide()
	before = loop.calls
	app.renderFrameMultiThread()
	if window.hides != 1 || primary.Visible() || loop.calls != before {
		t.Fatal("Hide should hide the window and stop scheduling its frames")
	}
}

type backgroundTestWindow struct {
	mockWindow
	shows, hides int
}

func (w *backgroundTestWindow) Show() { w.shows++ }
func (w *backgroundTestWindow) Hide() { w.hides++ }

type startupFrameRecorder struct {
	mockRenderLoop
	calls int
}

// Count queued work without entering the native frame pool.
func (r *startupFrameRecorder) RunOnRenderThreadVoid(func()) { r.calls++ }
