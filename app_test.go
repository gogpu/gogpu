package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

func TestNewApp(t *testing.T) {
	cfg := DefaultConfig()
	app := NewApp(cfg)

	if app == nil {
		t.Fatal("NewApp returned nil")
	}
	if app.config.Title != cfg.Title {
		t.Errorf("config.Title = %q, want %q", app.config.Title, cfg.Title)
	}
	if app.config.Width != cfg.Width {
		t.Errorf("config.Width = %d, want %d", app.config.Width, cfg.Width)
	}
}

func TestAppConfig(t *testing.T) {
	cfg := Config{
		Title:  "Test",
		Width:  1024,
		Height: 768,
	}
	app := NewApp(cfg)

	got := app.Config()
	if got.Title != "Test" {
		t.Errorf("Config().Title = %q, want %q", got.Title, "Test")
	}
	if got.Width != 1024 {
		t.Errorf("Config().Width = %d, want 1024", got.Width)
	}
	if got.Height != 768 {
		t.Errorf("Config().Height = %d, want 768", got.Height)
	}
}

func TestAppOnDrawChaining(t *testing.T) {
	app := NewApp(DefaultConfig())

	result := app.OnDraw(func(ctx *Context) {})
	if result != app {
		t.Error("OnDraw should return the same App for chaining")
	}
	if app.onDraw == nil {
		t.Error("OnDraw callback not set")
	}
}

func TestAppOnUpdateChaining(t *testing.T) {
	app := NewApp(DefaultConfig())

	result := app.OnUpdate(func(dt float64) {})
	if result != app {
		t.Error("OnUpdate should return the same App for chaining")
	}
	if app.onUpdate == nil {
		t.Error("OnUpdate callback not set")
	}
}

func TestAppOnResizeChaining(t *testing.T) {
	app := NewApp(DefaultConfig())

	result := app.OnResize(func(w, h int) {})
	if result != app {
		t.Error("OnResize should return the same App for chaining")
	}
	if app.onResize == nil {
		t.Error("OnResize callback not set")
	}
}

func TestAppOnCloseChaining(t *testing.T) {
	app := NewApp(DefaultConfig())

	result := app.OnClose(func() {})
	if result != app {
		t.Error("OnClose should return the same App for chaining")
	}
	if app.onClose == nil {
		t.Error("OnClose callback not set")
	}
}

func TestAppQuit(t *testing.T) {
	app := NewApp(DefaultConfig())

	if app.running {
		t.Error("running should be false initially")
	}

	app.running = true
	app.Quit()

	if app.running {
		t.Error("running should be false after Quit()")
	}
}

func TestAppRequestRedrawNilInvalidator(t *testing.T) {
	app := NewApp(DefaultConfig())

	// Should not panic with nil invalidator
	app.RequestRedraw()
}

func TestAppRequestRedrawWithInvalidator(t *testing.T) {
	app := NewApp(DefaultConfig())
	wokenUp := false
	app.invalidator = newInvalidator(func() { wokenUp = true })

	app.RequestRedraw()

	if !wokenUp {
		t.Error("RequestRedraw should trigger wakeup")
	}
}

func TestAppStartAnimation(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.animations = &AnimationController{}
	app.invalidator = newInvalidator(func() {})

	token := app.StartAnimation()
	if token == nil {
		t.Fatal("StartAnimation returned nil")
	}

	if !app.animations.IsAnimating() {
		t.Error("should be animating after StartAnimation")
	}

	token.Stop()
	if app.animations.IsAnimating() {
		t.Error("should not be animating after Stop")
	}
}

func TestAppSizeNilPlatform(t *testing.T) {
	app := NewApp(Config{Width: 640, Height: 480})

	w, h := app.Size()
	if w != 640 || h != 480 {
		t.Errorf("Size() = (%d, %d), want (640, 480)", w, h)
	}
}

func TestAppSizeWithPlatform(t *testing.T) {
	mock := &mockWindow{width: 1920, height: 1080, scaleFactor: 1.0}
	app := &App{platWindow: mock}

	w, h := app.Size()
	if w != 1920 || h != 1080 {
		t.Errorf("Size() = (%d, %d), want (1920, 1080)", w, h)
	}
}

func TestAppPhysicalSizeNilPlatform(t *testing.T) {
	app := NewApp(Config{Width: 800, Height: 600})

	w, h := app.PhysicalSize()
	if w != 800 || h != 600 {
		t.Errorf("PhysicalSize() = (%d, %d), want (800, 600)", w, h)
	}
}

func TestAppPhysicalSizeWithPlatform(t *testing.T) {
	mock := &mockWindow{width: 800, height: 600, scaleFactor: 2.0}
	app := &App{platWindow: mock}

	w, h := app.PhysicalSize()
	if w != 1600 || h != 1200 {
		t.Errorf("PhysicalSize() = (%d, %d), want (1600, 1200)", w, h)
	}
}

func TestAppDeviceProviderNilBeforeRun(t *testing.T) {
	app := NewApp(DefaultConfig())

	if app.DeviceProvider() != nil {
		t.Error("DeviceProvider should return nil before Run()")
	}
}

func TestAppEventSourceLazy(t *testing.T) {
	app := NewApp(DefaultConfig())

	// Should not be nil
	es := app.EventSource()
	if es == nil {
		t.Fatal("EventSource() returned nil")
	}

	// Should be same instance
	es2 := app.EventSource()
	if es != es2 {
		t.Error("EventSource() should return same instance")
	}
}

func TestAppInputLazy(t *testing.T) {
	app := NewApp(DefaultConfig())

	inp := app.Input()
	if inp == nil {
		t.Fatal("Input() returned nil")
	}

	// Should be same instance
	inp2 := app.Input()
	if inp != inp2 {
		t.Error("Input() should return same instance")
	}
}

func TestGpucontextKeyToInputKey(t *testing.T) {
	tests := []struct {
		name   string
		input  gpucontext.Key
		output input.Key
	}{
		// All letters
		{"KeyA", gpucontext.KeyA, input.KeyA},
		{"KeyB", gpucontext.KeyB, input.KeyB},
		{"KeyC", gpucontext.KeyC, input.KeyC},
		{"KeyD", gpucontext.KeyD, input.KeyD},
		{"KeyE", gpucontext.KeyE, input.KeyE},
		{"KeyF", gpucontext.KeyF, input.KeyF},
		{"KeyG", gpucontext.KeyG, input.KeyG},
		{"KeyH", gpucontext.KeyH, input.KeyH},
		{"KeyI", gpucontext.KeyI, input.KeyI},
		{"KeyJ", gpucontext.KeyJ, input.KeyJ},
		{"KeyK", gpucontext.KeyK, input.KeyK},
		{"KeyL", gpucontext.KeyL, input.KeyL},
		{"KeyM", gpucontext.KeyM, input.KeyM},
		{"KeyN", gpucontext.KeyN, input.KeyN},
		{"KeyO", gpucontext.KeyO, input.KeyO},
		{"KeyP", gpucontext.KeyP, input.KeyP},
		{"KeyQ", gpucontext.KeyQ, input.KeyQ},
		{"KeyR", gpucontext.KeyR, input.KeyR},
		{"KeyS", gpucontext.KeyS, input.KeyS},
		{"KeyT", gpucontext.KeyT, input.KeyT},
		{"KeyU", gpucontext.KeyU, input.KeyU},
		{"KeyV", gpucontext.KeyV, input.KeyV},
		{"KeyW", gpucontext.KeyW, input.KeyW},
		{"KeyX", gpucontext.KeyX, input.KeyX},
		{"KeyY", gpucontext.KeyY, input.KeyY},
		{"KeyZ", gpucontext.KeyZ, input.KeyZ},
		// All numbers
		{"Key0", gpucontext.Key0, input.Key0},
		{"Key1", gpucontext.Key1, input.Key1},
		{"Key2", gpucontext.Key2, input.Key2},
		{"Key3", gpucontext.Key3, input.Key3},
		{"Key4", gpucontext.Key4, input.Key4},
		{"Key5", gpucontext.Key5, input.Key5},
		{"Key6", gpucontext.Key6, input.Key6},
		{"Key7", gpucontext.Key7, input.Key7},
		{"Key8", gpucontext.Key8, input.Key8},
		{"Key9", gpucontext.Key9, input.Key9},
		// All function keys
		{"F1", gpucontext.KeyF1, input.KeyF1},
		{"F2", gpucontext.KeyF2, input.KeyF2},
		{"F3", gpucontext.KeyF3, input.KeyF3},
		{"F4", gpucontext.KeyF4, input.KeyF4},
		{"F5", gpucontext.KeyF5, input.KeyF5},
		{"F6", gpucontext.KeyF6, input.KeyF6},
		{"F7", gpucontext.KeyF7, input.KeyF7},
		{"F8", gpucontext.KeyF8, input.KeyF8},
		{"F9", gpucontext.KeyF9, input.KeyF9},
		{"F10", gpucontext.KeyF10, input.KeyF10},
		{"F11", gpucontext.KeyF11, input.KeyF11},
		{"F12", gpucontext.KeyF12, input.KeyF12},
		// All numpad keys
		{"Numpad1", gpucontext.KeyNumpad1, input.KeyNumpad1},
		{"Numpad2", gpucontext.KeyNumpad2, input.KeyNumpad2},
		{"Numpad3", gpucontext.KeyNumpad3, input.KeyNumpad3},
		{"Numpad4", gpucontext.KeyNumpad4, input.KeyNumpad4},
		{"Numpad5", gpucontext.KeyNumpad5, input.KeyNumpad5},
		{"Numpad6", gpucontext.KeyNumpad6, input.KeyNumpad6},
		{"Numpad7", gpucontext.KeyNumpad7, input.KeyNumpad7},
		{"Numpad8", gpucontext.KeyNumpad8, input.KeyNumpad8},
		{"Escape", gpucontext.KeyEscape, input.KeyEscape},
		{"Tab", gpucontext.KeyTab, input.KeyTab},
		{"Backspace", gpucontext.KeyBackspace, input.KeyBackspace},
		{"Enter", gpucontext.KeyEnter, input.KeyEnter},
		{"Space", gpucontext.KeySpace, input.KeySpace},
		{"Insert", gpucontext.KeyInsert, input.KeyInsert},
		{"Delete", gpucontext.KeyDelete, input.KeyDelete},
		{"Home", gpucontext.KeyHome, input.KeyHome},
		{"End", gpucontext.KeyEnd, input.KeyEnd},
		{"PageUp", gpucontext.KeyPageUp, input.KeyPageUp},
		{"PageDown", gpucontext.KeyPageDown, input.KeyPageDown},
		{"Left", gpucontext.KeyLeft, input.KeyLeft},
		{"Right", gpucontext.KeyRight, input.KeyRight},
		{"Up", gpucontext.KeyUp, input.KeyUp},
		{"Down", gpucontext.KeyDown, input.KeyDown},
		{"LeftShift", gpucontext.KeyLeftShift, input.KeyShiftLeft},
		{"RightShift", gpucontext.KeyRightShift, input.KeyShiftRight},
		{"LeftControl", gpucontext.KeyLeftControl, input.KeyControlLeft},
		{"RightControl", gpucontext.KeyRightControl, input.KeyControlRight},
		{"LeftAlt", gpucontext.KeyLeftAlt, input.KeyAltLeft},
		{"RightAlt", gpucontext.KeyRightAlt, input.KeyAltRight},
		{"LeftSuper", gpucontext.KeyLeftSuper, input.KeySuperLeft},
		{"RightSuper", gpucontext.KeyRightSuper, input.KeySuperRight},
		{"Minus", gpucontext.KeyMinus, input.KeyMinus},
		{"Equal", gpucontext.KeyEqual, input.KeyEqual},
		{"LeftBracket", gpucontext.KeyLeftBracket, input.KeyLeftBracket},
		{"RightBracket", gpucontext.KeyRightBracket, input.KeyRightBracket},
		{"Backslash", gpucontext.KeyBackslash, input.KeyBackslash},
		{"Semicolon", gpucontext.KeySemicolon, input.KeySemicolon},
		{"Apostrophe", gpucontext.KeyApostrophe, input.KeyApostrophe},
		{"Grave", gpucontext.KeyGrave, input.KeyGrave},
		{"Comma", gpucontext.KeyComma, input.KeyComma},
		{"Period", gpucontext.KeyPeriod, input.KeyPeriod},
		{"Slash", gpucontext.KeySlash, input.KeySlash},
		{"Numpad0", gpucontext.KeyNumpad0, input.KeyNumpad0},
		{"Numpad9", gpucontext.KeyNumpad9, input.KeyNumpad9},
		{"NumpadDecimal", gpucontext.KeyNumpadDecimal, input.KeyNumpadDecimal},
		{"NumpadDivide", gpucontext.KeyNumpadDivide, input.KeyNumpadDivide},
		{"NumpadMultiply", gpucontext.KeyNumpadMultiply, input.KeyNumpadMultiply},
		{"NumpadSubtract", gpucontext.KeyNumpadSubtract, input.KeyNumpadSubtract},
		{"NumpadAdd", gpucontext.KeyNumpadAdd, input.KeyNumpadAdd},
		{"NumpadEnter", gpucontext.KeyNumpadEnter, input.KeyNumpadEnter},
		{"CapsLock", gpucontext.KeyCapsLock, input.KeyCapsLock},
		{"ScrollLock", gpucontext.KeyScrollLock, input.KeyScrollLock},
		{"NumLock", gpucontext.KeyNumLock, input.KeyNumLock},
		{"Pause", gpucontext.KeyPause, input.KeyPause},
		{"Unknown", gpucontext.Key(9999), input.KeyUnknown},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := gpucontextKeyToInputKey(tt.input)
				if got != tt.output {
					t.Errorf("gpucontextKeyToInputKey(%v) = %v, want %v", tt.input, got, tt.output)
				}
			},
		)
	}
}

func TestGpucontextButtonToInputButton(t *testing.T) {
	tests := []struct {
		name   string
		input  gpucontext.Button
		output input.MouseButton
	}{
		{"Left", gpucontext.ButtonLeft, input.MouseButtonLeft},
		{"Right", gpucontext.ButtonRight, input.MouseButtonRight},
		{"Middle", gpucontext.ButtonMiddle, input.MouseButtonMiddle},
		{"X1", gpucontext.ButtonX1, input.MouseButton4},
		{"X2", gpucontext.ButtonX2, input.MouseButton5},
		{"Unknown", gpucontext.Button(99), input.MouseButtonLeft},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got := gpucontextButtonToInputButton(tt.input)
				if got != tt.output {
					t.Errorf("gpucontextButtonToInputButton(%v) = %v, want %v", tt.input, got, tt.output)
				}
			},
		)
	}
}

func TestAppTrackResourceLazyInit(t *testing.T) {
	app := NewApp(DefaultConfig())

	if app.tracker != nil {
		t.Error("tracker should be nil initially")
	}

	app.TrackResource(newMockCloser("test"))

	if app.tracker == nil {
		t.Error("tracker should be initialized after TrackResource")
	}
}

func TestAppUntrackResourceNilTracker(t *testing.T) {
	app := NewApp(DefaultConfig())

	// Should not panic
	app.UntrackResource(newMockCloser("test"))
}

func TestAppGPUContextProviderNilRenderer(t *testing.T) {
	app := NewApp(DefaultConfig())

	provider := app.GPUContextProvider()
	if provider != nil {
		t.Error("GPUContextProvider should return nil before Run()")
	}
}

func TestAppGPUContextProviderLazyTracker(t *testing.T) {
	// With a renderer set, tracker should be lazily initialized
	app := &App{
		renderer: &Renderer{},
	}

	provider := app.GPUContextProvider()
	if provider == nil {
		t.Fatal("GPUContextProvider should return non-nil when renderer exists")
	}
	if app.tracker == nil {
		t.Error("tracker should be initialized by GPUContextProvider")
	}
}

func TestAppCallbackChaining(t *testing.T) {
	// Verify full chaining pattern
	app := NewApp(DefaultConfig()).
		OnDraw(func(ctx *Context) {}).
		OnUpdate(func(dt float64) {}).
		OnResize(func(w, h int) {}).
		OnClose(func() {})

	if app.onDraw == nil {
		t.Error("OnDraw not set after chaining")
	}
	if app.onUpdate == nil {
		t.Error("OnUpdate not set after chaining")
	}
	if app.onResize == nil {
		t.Error("OnResize not set after chaining")
	}
	if app.onClose == nil {
		t.Error("OnClose not set after chaining")
	}
}

func TestAppUpdateMouseStateFromPointer(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.inputState = input.New()

	// Mouse press
	app.updateMouseStateFromPointer(
		gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerType: gpucontext.PointerTypeMouse,
			Button:      gpucontext.ButtonLeft,
			X:           100,
			Y:           200,
		},
	)

	mx, my := app.inputState.Mouse().Position()
	if mx != 100 || my != 200 {
		t.Errorf("Mouse position = (%f, %f), want (100, 200)", mx, my)
	}

	// Mouse release
	app.updateMouseStateFromPointer(
		gpucontext.PointerEvent{
			Type:        gpucontext.PointerUp,
			PointerType: gpucontext.PointerTypeMouse,
			Button:      gpucontext.ButtonLeft,
			X:           110,
			Y:           210,
		},
	)

	mx, my = app.inputState.Mouse().Position()
	if mx != 110 || my != 210 {
		t.Errorf("Mouse position after release = (%f, %f), want (110, 210)", mx, my)
	}
}

func TestAppUpdateMouseStateNilInputState(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.inputState = nil

	// Should not panic
	app.updateMouseStateFromPointer(
		gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerType: gpucontext.PointerTypeMouse,
			Button:      gpucontext.ButtonLeft,
		},
	)
}

func TestAppUpdateMouseStateNonMouse(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.inputState = input.New()

	// Touch events should update position but not button state
	app.updateMouseStateFromPointer(
		gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerType: gpucontext.PointerTypeTouch,
			Button:      gpucontext.ButtonLeft,
			X:           50,
			Y:           60,
		},
	)

	mx, my := app.inputState.Mouse().Position()
	if mx != 50 || my != 60 {
		t.Errorf("Mouse position = (%f, %f), want (50, 60) (touch updates position)", mx, my)
	}
}

func TestAppUpdateMouseStateInvalidButton(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.inputState = input.New()

	// Invalid button should not panic
	app.updateMouseStateFromPointer(
		gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerType: gpucontext.PointerTypeMouse,
			Button:      gpucontext.Button(99),
			X:           10,
			Y:           20,
		},
	)
}

func TestAppUpdateMouseStatePointerMove(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.inputState = input.New()

	// Pointer move should update position
	app.updateMouseStateFromPointer(
		gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerType: gpucontext.PointerTypeMouse,
			X:           300,
			Y:           400,
		},
	)

	mx, my := app.inputState.Mouse().Position()
	if mx != 300 || my != 400 {
		t.Errorf("Mouse position = (%f, %f), want (300, 400)", mx, my)
	}
}

type mockRenderLoop struct{}

func (m *mockRenderLoop) RunOnRenderThreadVoid(fn func())              { fn() }
func (m *mockRenderLoop) Stop()                                        {}
func (m *mockRenderLoop) RequestResize(w, h uint32)                    {}
func (m *mockRenderLoop) ConsumePendingResize() (uint32, uint32, bool) { return 0, 0, false }

func TestApp_CloseSecondaryWindowRecyclesID(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		primaryWindow: nil,
		renderLoop:    &mockRenderLoop{},
	}
	secID := app.windowManager.allocate()
	sec := &Window{
		id:         secID,
		platformID: platform.NewWindowID(),
		visible:    true,
		platWindow: &mockWindow{},
	}
	app.windowManager.add(sec)

	app.closeSecondaryWindow(secID)

	if app.windowManager.get(secID) != nil {
		t.Fatal("secondary window should be removed from manager")
	}
	reusedID := app.windowManager.allocate()
	if reusedID != secID {
		t.Errorf("expected recycled ID %d, got %d", secID, reusedID)
	}
}

func TestApp_OnAnyWindowClosed_Chaining(t *testing.T) {
	app := NewApp(DefaultConfig())
	if app.OnAnyWindowClosed(func(id WindowID) {}) != app {
		t.Error("should return *App for chaining")
	}
	if app.onAnyWindowClosed == nil {
		t.Error("callback not set")
	}
}

func TestApp_OnAnyWindowClosed_Primary(t *testing.T) {
	app := &App{
		windowManager:          newWindowManager(),
		renderLoop:             &mockRenderLoop{},
		quitOnLastWindowClosed: true,
	}
	pid := platform.NewWindowID()
	primaryID := app.windowManager.allocate()
	app.primaryWindow = &Window{
		id:         primaryID,
		platformID: pid,
		visible:    true,
		platWindow: &mockWindow{},
		surface:    &RenderTarget{},
	}
	app.primaryPlatformID = pid
	app.windowManager.add(app.primaryWindow)

	var closedID WindowID
	app.OnAnyWindowClosed(func(id WindowID) { closedID = id })

	app.classifyEvent(
		&platform.Event{
			Type:     platform.EventClose,
			WindowID: pid,
		}, nil, nil,
	)

	// ADR-026: primary close with no other windows → app exits
	if app.running {
		t.Error("app should be stopped after last window close")
	}
	if app.primaryWindow != nil {
		t.Error("primaryWindow should be nil after close")
	}
	if closedID != primaryID {
		t.Errorf("got ID %d, want %d", closedID, primaryID)
	}
}

func TestApp_OnAnyWindowClosed_PrimaryRejected(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		running:       true,
	}
	pid := platform.NewWindowID()
	app.primaryWindow = &Window{
		id:         app.windowManager.allocate(),
		platformID: pid,
		visible:    true,
		platWindow: &mockWindow{},
		surface:    &RenderTarget{},
	}
	app.primaryWindow.SetOnClose(func() bool { return false })
	app.windowManager.add(app.primaryWindow)

	called := false
	app.OnAnyWindowClosed(func(id WindowID) { called = true })

	app.classifyEvent(
		&platform.Event{
			Type:     platform.EventClose,
			WindowID: pid,
		}, nil, nil,
	)

	if called {
		t.Error("should not be called when onClose rejects")
	}
	if !app.running {
		t.Error("app should still be running")
	}
}

func TestApp_OnAnyWindowClosed_Secondary(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		primaryWindow: nil,
		renderLoop:    &mockRenderLoop{},
	}
	id := app.windowManager.allocate()
	w := &Window{
		id:         id,
		platformID: platform.NewWindowID(),
		visible:    true,
		platWindow: &mockWindow{},
		surface:    nil,
	}
	app.windowManager.add(w)

	var closedID WindowID
	app.OnAnyWindowClosed(func(id WindowID) { closedID = id })

	app.closeSecondaryWindow(id)

	if closedID != id {
		t.Errorf("got ID %d, want %d", closedID, id)
	}
	if app.windowManager.get(id) != nil {
		t.Error("window should be removed")
	}
}

func TestApp_OnAnyWindowClosed_SecondaryRejected(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		primaryWindow: nil,
		renderLoop:    &mockRenderLoop{},
	}
	id := app.windowManager.allocate()
	w := &Window{
		id:         id,
		platformID: platform.NewWindowID(),
		visible:    true,
		platWindow: &mockWindow{},
	}
	w.SetOnClose(func() bool { return false })
	app.windowManager.add(w)

	called := false
	app.OnAnyWindowClosed(func(id WindowID) { called = true })

	app.classifyEvent(
		&platform.Event{
			Type:     platform.EventClose,
			WindowID: w.platformID,
		}, nil, nil,
	)

	if called {
		t.Error("should not be called when onClose rejects")
	}
	if app.windowManager.get(id) == nil {
		t.Error("window should still be present")
	}
}

// --- ADR-026 Lifecycle Tests ---

func TestApp_PrimaryCloseKeepsSecondary(t *testing.T) {
	app := &App{
		windowManager:          newWindowManager(),
		renderLoop:             &mockRenderLoop{},
		running:                true,
		quitOnLastWindowClosed: true,
	}

	// Create primary
	primaryPID := platform.NewWindowID()
	primaryID := app.windowManager.allocate()
	app.primaryWindow = &Window{
		id: primaryID, platformID: primaryPID,
		visible: true, platWindow: &mockWindow{}, surface: &RenderTarget{},
	}
	app.primaryPlatformID = primaryPID
	app.windowManager.add(app.primaryWindow)

	// Create secondary
	secondaryPID := platform.NewWindowID()
	secondaryID := app.windowManager.allocate()
	secondary := &Window{
		id: secondaryID, platformID: secondaryPID,
		visible: true, platWindow: &mockWindow{}, surface: &RenderTarget{},
	}
	app.windowManager.add(secondary)

	// Close primary
	app.classifyEvent(&platform.Event{Type: platform.EventClose, WindowID: primaryPID}, nil, nil)

	if app.primaryWindow != nil {
		t.Error("primaryWindow should be nil after close")
	}
	if !app.running {
		t.Error("app should still be running — secondary window alive")
	}
	if app.windowManager.get(secondaryID) == nil {
		t.Error("secondary window should still exist")
	}
	if app.windowManager.count() != 1 {
		t.Errorf("expected 1 window remaining, got %d", app.windowManager.count())
	}

	// Close secondary — NOW app should exit
	app.classifyEvent(&platform.Event{Type: platform.EventClose, WindowID: secondaryPID}, nil, nil)

	if app.running {
		t.Error("app should stop after last window closed")
	}
	if app.windowManager.count() != 0 {
		t.Errorf("expected 0 windows, got %d", app.windowManager.count())
	}
}

func TestApp_QuitOnLastWindowClosedFalse(t *testing.T) {
	app := &App{
		windowManager:          newWindowManager(),
		renderLoop:             &mockRenderLoop{},
		running:                true,
		quitOnLastWindowClosed: false,
	}

	pid := platform.NewWindowID()
	id := app.windowManager.allocate()
	app.primaryWindow = &Window{
		id: id, platformID: pid,
		visible: true, platWindow: &mockWindow{}, surface: &RenderTarget{},
	}
	app.primaryPlatformID = pid
	app.windowManager.add(app.primaryWindow)

	// Close only window
	app.classifyEvent(&platform.Event{Type: platform.EventClose, WindowID: pid}, nil, nil)

	if !app.running {
		t.Error("app should still run with quitOnLastWindowClosed=false")
	}
}

func TestApp_FocusEvent_SetsFocusedWindow(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
	}

	platformID := platform.NewWindowID()
	internalID := app.windowManager.allocate()
	w := &Window{id: internalID, platformID: platformID, visible: true}
	app.windowManager.add(w)

	app.classifyEvent(
		&platform.Event{
			Type:     platform.EventFocus,
			WindowID: platformID,
			Focused:  true,
		}, nil, nil,
	)

	if app.windowManager.focused != internalID {
		t.Errorf("focused = %d, want %d", app.windowManager.focused, internalID)
	}
}

func TestApp_HandleSecondaryResize_NilWindow(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		invalidator:   newInvalidator(nil),
	}

	app.handleSecondaryResize(
		platform.Event{
			Type:     platform.EventResize,
			WindowID: platform.NewWindowID(),
			Width:    100, Height: 200,
		},
	)
}

func TestApp_HandleSecondaryResize_NoPhysicalResize(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		invalidator:   newInvalidator(nil),
	}
	platformID := platform.NewWindowID()
	internalID := app.windowManager.allocate()
	surface := &RenderTarget{width: 10, height: 10}
	w := &Window{id: internalID, platformID: platformID, surface: surface, visible: true}
	app.windowManager.add(w)

	var resizeCalled bool
	var newW, newH int
	w.SetOnResize(func(w, h int) { resizeCalled = true; newW, newH = w, h })

	app.handleSecondaryResize(
		platform.Event{
			Type:     platform.EventResize,
			WindowID: platformID,
			Width:    320, Height: 240,
			PhysicalWidth:  0,
			PhysicalHeight: 0,
		},
	)

	if !resizeCalled {
		t.Error("onResize callback should have been called")
	}
	if newW != 320 || newH != 240 {
		t.Errorf("resize dimensions = (%d,%d), want (320,240)", newW, newH)
	}
	if !app.invalidator.Consume() {
		t.Error("expected redraw request after secondary resize")
	}
}

func TestApp_PrimaryWindow_IDs(t *testing.T) {
	app := &App{windowManager: newWindowManager()}

	internalID := app.windowManager.allocate()
	platformID := platform.NewWindowID()
	app.primaryWindow = &Window{
		id:         internalID,
		platformID: platformID,
		visible:    true,
		platWindow: &mockWindow{},
		surface:    &RenderTarget{},
	}
	app.windowManager.add(app.primaryWindow)

	if app.primaryWindow.id != internalID {
		t.Error("primaryWindow.id should be internalID")
	}
	if app.primaryWindow.platformID != platformID {
		t.Error("primaryWindow.platformID should be platformID")
	}
	if app.PrimaryWindow() != app.primaryWindow {
		t.Error("PrimaryWindow() should return primaryWindow")
	}
}

func TestApp_ProcessEvents_SecondaryResizeCycle(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		invalidator:   newInvalidator(nil),
	}

	platformID := platform.NewWindowID()
	internalID := app.windowManager.allocate()
	surface := &RenderTarget{width: 10, height: 10}
	w := &Window{
		id:         internalID,
		platformID: platformID,
		visible:    true,
		platWindow: &mockWindow{},
		surface:    surface,
	}
	app.windowManager.add(w)

	var resizeCalled bool
	var resizeW, resizeH int
	w.SetOnResize(func(w, h int) {
		resizeCalled = true
		resizeW, resizeH = w, h
	})

	primaryEv := platform.Event{Type: platform.EventResize, WindowID: 0, Width: 1024, Height: 768}
	secondaryEv := platform.Event{
		Type:           platform.EventResize,
		WindowID:       platformID,
		Width:          400,
		Height:         300,
		PhysicalWidth:  0,
		PhysicalHeight: 0,
	}

	var secResizes []platform.Event
	_, secResizes = app.classifyEvent(&primaryEv, nil, secResizes)
	_, secResizes = app.classifyEvent(&secondaryEv, nil, secResizes)

	for i := range secResizes {
		app.handleSecondaryResize(secResizes[i])
	}

	if !resizeCalled {
		t.Error("secondary window onResize should have been called")
	}
	if resizeW != 400 || resizeH != 300 {
		t.Errorf("resize dimensions = (%d,%d), want (400,300)", resizeW, resizeH)
	}
	if !app.invalidator.Consume() {
		t.Error("expected redraw request after secondary resize")
	}
}

func TestApp_WindowCloseEvent_OnClosePanic(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		running:       true,
	}
	pid := platform.NewWindowID()
	app.primaryWindow = &Window{
		id:         app.windowManager.allocate(),
		platformID: pid,
		visible:    true,
		platWindow: &mockWindow{},
		surface:    &RenderTarget{},
	}
	app.primaryPlatformID = pid
	app.windowManager.add(app.primaryWindow)

	app.primaryWindow.SetOnClose(func() bool {
		panic("unexpected error")
	})

	app.windowCloseEvent(&platform.Event{
		Type:     platform.EventClose,
		WindowID: pid,
	})

	if !app.running {
		t.Error("app should still be running after panicking onClose")
	}
}

func TestAppSetTitle(t *testing.T) {
	app := &App{
		config:     DefaultConfig(),
		platWindow: &mockWindow{},
	}

	app.SetTitle("New Title")

	if app.config.Title != "New Title" {
		t.Errorf("config.Title = %q, want %q", app.config.Title, "New Title")
	}
}

func TestAppTitle(t *testing.T) {
	cfg := DefaultConfig().WithTitle("My App")
	app := NewApp(cfg)

	if app.Title() != "My App" {
		t.Errorf("Title() = %q, want %q", app.Title(), "My App")
	}

	app.SetTitle("Updated")
	if app.Title() != "Updated" {
		t.Errorf("Title() after SetTitle = %q, want %q", app.Title(), "Updated")
	}
}

func TestAppOnFocusChaining(t *testing.T) {
	app := NewApp(DefaultConfig())

	result := app.OnFocus(func(bool) {})
	if result != app {
		t.Error("OnFocus should return the same App for chaining")
	}
	if app.eventSource == nil {
		t.Error("OnFocus should initialize eventSource")
	}
}

func TestAppHasFocus(t *testing.T) {
	app := NewApp(DefaultConfig())

	if app.HasFocus() {
		t.Error("HasFocus() = true before any focus event, want false")
	}

	app.focused = true
	if !app.HasFocus() {
		t.Error("HasFocus() = false after setting focused, want true")
	}
}

func TestAppOnFocusCallback(t *testing.T) {
	app := NewApp(DefaultConfig())

	var got []bool
	app.OnFocus(func(focused bool) {
		got = append(got, focused)
	})

	app.eventSource.dispatchFocus(true)
	app.eventSource.dispatchFocus(false)
	app.eventSource.dispatchFocus(true)

	if len(got) != 3 {
		t.Fatalf("expected 3 focus events, got %d", len(got))
	}
	if got[0] != true || got[1] != false || got[2] != true {
		t.Errorf("focus sequence = %v, want [true false true]", got)
	}
}
