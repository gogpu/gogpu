package gogpu

import (
	"errors"
	"runtime"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

func TestBackgroundSetsPolicyBeforeInit(t *testing.T) {
	for _, background := range []bool{false, true} {
		want := background && runtime.GOOS == "darwin"
		manager := &backgroundStartupManager{}
		original := newPlatformManagerFn
		newPlatformManagerFn = func() platform.PlatformManager { return manager }
		t.Cleanup(func() { newPlatformManagerFn = original })
		app := &App{config: DefaultConfig().WithBackground(background)}
		_, err := app.initPlatform()
		if !errors.Is(err, errStopBackgroundInit) {
			t.Fatalf("startup did not reach the fake manager: %v", err)
		}
		if !manager.configured || manager.backgroundAtInit != want {
			t.Fatalf("background = %t: policy set = %t, policy at init = %t; want %t",
				background, manager.configured, manager.backgroundAtInit, want)
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
	enabled := original.WithBackground(true)
	disabled := enabled.WithBackground(false)
	if original.Background || !enabled.Background || disabled.Background {
		t.Fatal("background should default to false and its builder should return a copy")
	}

	native := &backgroundTestWindow{}
	app := &App{config: disabled, platWindow: native, renderer: &Renderer{}, renderLoop: &startupFrameRecorder{}}
	app.startPrimaryWindow(native)
	if native.shows != 1 || !app.primaryWindow.Visible() {
		t.Fatalf("normal startup: native shows %d, visible %t", native.shows, app.primaryWindow.Visible())
	}
}

func TestBackgroundStartupKeepsWindowHiddenUntilShown(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("hidden startup is specific to macOS")
	}
	window := &backgroundTestWindow{}
	loop := &startupFrameRecorder{}
	app := &App{
		config: DefaultConfig().WithBackground(true), platWindow: window,
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
