package gogpu

import (
	"testing"
	"time"

	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// mockFrameGaterWindow wraps mockWindow and implements platform.FrameGater.
type mockFrameGaterWindow struct {
	mockWindow
	ready bool
}

func (m *mockFrameGaterWindow) FrameCallbackReady() bool { return m.ready }

// =============================================================================
// SetQuitOnLastWindowClosed
// =============================================================================

func TestApp_SetQuitOnLastWindowClosed(t *testing.T) {
	app := NewApp(DefaultConfig())
	ret := app.SetQuitOnLastWindowClosed(false)
	if ret != app {
		t.Error("SetQuitOnLastWindowClosed should return the App for chaining")
	}
	if app.quitOnLastWindowClosed {
		t.Error("quitOnLastWindowClosed should be false after SetQuitOnLastWindowClosed(false)")
	}
}

// =============================================================================
// startRunLoop
// =============================================================================

func TestApp_StartRunLoop(t *testing.T) {
	var wokeUp bool
	app := &App{
		manager: &mockManager{},
	}
	app.invalidator = newInvalidator(func() { wokeUp = true })
	_ = wokeUp

	var surfaceAvailableCalled bool
	app.onSurfaceAvailable = func() { surfaceAvailableCalled = true }
	app.pendingRedraw.Store(true)

	app.startRunLoop()

	if !app.running {
		t.Error("startRunLoop should set running=true")
	}
	if app.lifecycle != AppRunning {
		t.Errorf("lifecycle = %v, want AppRunning", app.lifecycle)
	}
	if !surfaceAvailableCalled {
		t.Error("onSurfaceAvailable should be called")
	}
	if app.animations == nil {
		t.Error("animations should be initialized")
	}
	if app.pendingRedraw.Load() {
		t.Error("pendingRedraw should be consumed into the invalidator")
	}
}

func TestApp_StartRunLoop_NoPendingRedrawNoCallback(t *testing.T) {
	app := &App{manager: &mockManager{}}
	app.invalidator = newInvalidator(func() {})

	app.startRunLoop()

	if !app.running {
		t.Error("startRunLoop should set running=true")
	}
}

// =============================================================================
// runFrame
// =============================================================================

func TestApp_RunFrame_IdleThenResizeInvalidates(t *testing.T) {
	mgr := &singleEventManager{
		event: platform.Event{
			Type:           platform.EventResize,
			WindowID:       0,
			Width:          800,
			Height:         600,
			PhysicalWidth:  1600,
			PhysicalHeight: 1200,
		},
	}
	app := &App{
		manager:       mgr,
		renderLoop:    &mockRenderLoop{},
		platWindow:    &mockWindow{},
		windowManager: newWindowManager(),
		animations:    &AnimationController{},
		lastFrame:     time.Now(),
	}
	app.invalidator = newInvalidator(func() {})

	app.runFrame()
}

func TestApp_RunFrame_ContinuousWithUpdateAndInput(t *testing.T) {
	app := &App{
		manager:       &mockManager{},
		renderLoop:    &mockRenderLoop{},
		windowManager: newWindowManager(),
		animations:    &AnimationController{},
		inputState:    input.New(),
		config:        Config{ContinuousRender: true},
	}
	app.invalidator = newInvalidator(func() {})

	var updateCalled bool
	app.onUpdate = func(dt float64) {
		updateCalled = true
		app.RequestRedraw()
	}

	app.runFrame()

	if !updateCalled {
		t.Error("onUpdate should have been called")
	}
}

// =============================================================================
// registerPrimaryWindow
// =============================================================================

func TestApp_RegisterPrimaryWindow(t *testing.T) {
	win := &mockWindow{windowID: 42}
	app := &App{
		config:   DefaultConfig(),
		renderer: &Renderer{},
	}

	app.registerPrimaryWindow(win)

	if app.windowManager == nil {
		t.Fatal("windowManager should be initialized")
	}
	if app.primaryWindow == nil {
		t.Fatal("primaryWindow should be set")
	}
	if app.primaryPlatformID != win.ID() {
		t.Errorf("primaryPlatformID = %v, want %v", app.primaryPlatformID, win.ID())
	}
	if !app.primaryWindow.visible {
		t.Error("primaryWindow should be visible")
	}
}

// =============================================================================
// modalFrameTick
// =============================================================================

func TestApp_ModalFrameTick_NilPlatWindowReturnsEarly(t *testing.T) {
	app := &App{
		lastFrame: time.Now(),
	}
	var updateCalled bool
	app.onUpdate = func(dt float64) { updateCalled = true }

	app.modalFrameTick()

	if !updateCalled {
		t.Error("onUpdate should run before the nil-platWindow early return")
	}
}

func TestApp_ModalFrameTick_FullCycle(t *testing.T) {
	app := &App{
		lastFrame:     time.Now(),
		platWindow:    &mockWindow{width: 100, height: 100},
		renderLoop:    &mockRenderLoop{},
		renderer:      &Renderer{},
		windowManager: newWindowManager(),
		inputState:    input.New(),
	}

	app.modalFrameTick()
}

// =============================================================================
// resizeSecondaryWindowsDuringModal
// =============================================================================

func TestApp_ResizeSecondaryWindowsDuringModal_NilGuards(t *testing.T) {
	app := &App{}
	app.resizeSecondaryWindowsDuringModal() // windowManager and renderer nil

	app.windowManager = newWindowManager()
	app.resizeSecondaryWindowsDuringModal() // renderer still nil
}

func TestApp_ResizeSecondaryWindowsDuringModal_NoEligibleWindows(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderer:      &Renderer{},
		renderLoop:    &mockRenderLoop{},
	}
	id := app.windowManager.allocate()
	// Zero physical size -- excluded from the resize set.
	w := &Window{id: id, platWindow: &mockWindow{width: 0, height: 0}, surface: &RenderTarget{}}
	app.windowManager.add(w)
	app.primaryWindow = w

	app.resizeSecondaryWindowsDuringModal()
}

// =============================================================================
// frameCallbackReady
// =============================================================================

func TestApp_FrameCallbackReady_NoFrameGater(t *testing.T) {
	app := &App{platWindow: &mockWindow{}}
	if !app.frameCallbackReady() {
		t.Error("frameCallbackReady should default to true when platform lacks FrameGater")
	}
}

func TestApp_FrameCallbackReady_NilPlatWindow(t *testing.T) {
	app := &App{}
	if !app.frameCallbackReady() {
		t.Error("frameCallbackReady should default to true when platWindow is nil")
	}
}

func TestApp_FrameCallbackReady_FrameGater(t *testing.T) {
	app := &App{platWindow: &mockFrameGaterWindow{ready: false}}
	if app.frameCallbackReady() {
		t.Error("frameCallbackReady should reflect FrameGater's result")
	}
}

// =============================================================================
// SetAppName
// =============================================================================

func TestApp_SetAppName_NilManager(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.SetAppName("MyApp")
	if app.config.AppName != "MyApp" {
		t.Errorf("config.AppName = %q, want MyApp", app.config.AppName)
	}
}

func TestApp_SetAppName_WithManager(t *testing.T) {
	app := &App{manager: &mockManager{}}
	app.SetAppName("MyApp")
	if app.config.AppName != "MyApp" {
		t.Errorf("config.AppName = %q, want MyApp", app.config.AppName)
	}
}

// =============================================================================
// SetCursorMode / CursorMode
// =============================================================================

func TestApp_SetCursorMode_CursorMode(t *testing.T) {
	app := &App{}
	// Nil platWindow: no-op, default mode.
	app.SetCursorMode(gpucontext.CursorModeLocked)
	if app.CursorMode() != gpucontext.CursorModeNormal {
		t.Error("CursorMode should default to Normal with nil platWindow")
	}

	app.platWindow = &mockWindow{}
	app.SetCursorMode(gpucontext.CursorModeLocked)
	_ = app.CursorMode()
}

// =============================================================================
// SetFullscreen / IsFullscreen / ToggleFullscreen
// =============================================================================

func TestApp_Fullscreen_NilPlatWindow(t *testing.T) {
	app := &App{}
	app.SetFullscreen(true)
	if app.IsFullscreen() {
		t.Error("IsFullscreen should be false with nil platWindow")
	}
	app.ToggleFullscreen()
}

func TestApp_Fullscreen_WithPlatWindow(t *testing.T) {
	win := &mockWindow{}
	app := &App{platWindow: win}

	app.SetFullscreen(true)
	if !app.IsFullscreen() {
		t.Error("IsFullscreen should be true after SetFullscreen(true)")
	}

	app.ToggleFullscreen()
	if app.IsFullscreen() {
		t.Error("ToggleFullscreen should flip fullscreen state to false")
	}
}

// =============================================================================
// GetSystemMenu
// =============================================================================

func TestApp_GetSystemMenu_NilManager(t *testing.T) {
	app := NewApp(DefaultConfig())
	handle := app.GetSystemMenu(SystemMenuApplication)
	if handle == nil {
		t.Fatal("GetSystemMenu should return a handle even before Run()")
	}
	if app.pendingSystemMenuItems == nil {
		t.Error("pendingSystemMenuItems should be initialized")
	}
}

func TestApp_GetSystemMenu_WithMenuManager(t *testing.T) {
	app := &App{manager: &mockMenuManager{}}
	handle := app.GetSystemMenu(SystemMenuApplication)
	if handle == nil {
		t.Fatal("GetSystemMenu should return a non-nil handle")
	}
}

func TestApp_GetSystemMenu_ManagerWithoutMenuSupport(t *testing.T) {
	app := &App{manager: &mockManager{}}
	handle := app.GetSystemMenu(SystemMenuApplication)
	if handle != nil {
		t.Error("GetSystemMenu should return nil when the manager lacks PlatMenuManager")
	}
}

// =============================================================================
// SetCustomMenu
// =============================================================================

func TestApp_SetCustomMenu_UpdatesExistingEntry(t *testing.T) {
	app := &App{manager: &mockMenuManager{}}
	first := &Menu{Title: "Edit"}
	second := &Menu{Title: "Edit2"}

	app.SetCustomMenu("edit", first)
	app.SetCustomMenu("edit", second)

	if len(app.customMenus) != 1 {
		t.Fatalf("expected 1 custom menu entry, got %d", len(app.customMenus))
	}
	if app.customMenus[0].menu != second {
		t.Error("SetCustomMenu should update the existing entry's menu")
	}
}

// =============================================================================
// DeviceProvider
// =============================================================================

func TestApp_DeviceProvider_WithRenderer(t *testing.T) {
	app := &App{renderer: &Renderer{}}
	provider := app.DeviceProvider()
	if provider == nil {
		t.Error("DeviceProvider should be non-nil once the renderer is set")
	}
}

// =============================================================================
// OnDraw / OnResize sync primary window
// =============================================================================

func TestApp_OnDraw_OnResize_SyncsPrimaryWindow(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.primaryWindow = &Window{}

	drawFn := func(*Context) {}
	resizeFn := func(int, int) {}
	app.OnDraw(drawFn)
	app.OnResize(resizeFn)

	if app.primaryWindow.onDraw == nil {
		t.Error("primaryWindow.onDraw should be synced by OnDraw")
	}
	if app.primaryWindow.onResize == nil {
		t.Error("primaryWindow.onResize should be synced by OnResize")
	}
}

// =============================================================================
// UntrackResource
// =============================================================================

func TestApp_UntrackResource_RemovesTrackedResource(t *testing.T) {
	app := NewApp(DefaultConfig())
	closer := newMockCloser("res")
	app.TrackResource(closer)

	app.UntrackResource(closer)
}
