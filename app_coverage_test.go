package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

// mockWindowWithSizeMove wraps mockWindow and allows InSizeMove() to return true.
type mockWindowWithSizeMove struct {
	mockWindow
	inSizeMove bool
}

func (m *mockWindowWithSizeMove) InSizeMove() bool { return m.inSizeMove }

// mockLiveResizePhaserWindow wraps mockWindow and implements platform.LiveResizePhaser.
type mockLiveResizePhaserWindow struct {
	mockWindow
	startHook func()
	endHook   func()
}

func (m *mockLiveResizePhaserWindow) SetLiveResizePhaseHooks(start, end func()) {
	m.startHook = start
	m.endHook = end
}

// mockManagerWithWindow replaces CreateWindow to return a specific platform window.
type mockManagerWithWindow struct {
	mockManager
	win platform.PlatformWindow
}

func (m *mockManagerWithWindow) CreateWindow(_ platform.Config) (platform.PlatformWindow, error) {
	return m.win, nil
}

// singleEventManager returns one event from PollEvents and then EventNone.
type singleEventManager struct {
	mockManager
	event platform.Event
	sent  bool
}

func (m *singleEventManager) PollEvents() platform.Event {
	if !m.sent {
		m.sent = true
		return m.event
	}
	return platform.Event{}
}

// mockRenderLoopResizing returns ok=true from ConsumePendingResize.
type mockRenderLoopResizing struct {
	mockRenderLoop
	w, h uint32
}

func (m *mockRenderLoopResizing) ConsumePendingResize() (uint32, uint32, bool) {
	return m.w, m.h, true
}

// =============================================================================
// App.InSizeMove
// =============================================================================

func TestAppInSizeMove_NilPlatWindow(t *testing.T) {
	app := NewApp(DefaultConfig())
	if app.InSizeMove() {
		t.Error("InSizeMove() should return false when platWindow is nil")
	}
}

func TestAppInSizeMove_ReturnsFalse(t *testing.T) {
	app := &App{platWindow: &mockWindowWithSizeMove{inSizeMove: false}}
	if app.InSizeMove() {
		t.Error("InSizeMove() should return false when platform reports false")
	}
}

func TestAppInSizeMove_ReturnsTrue(t *testing.T) {
	app := &App{platWindow: &mockWindowWithSizeMove{inSizeMove: true}}
	if !app.InSizeMove() {
		t.Error("InSizeMove() should return true when platform reports true")
	}
}

// =============================================================================
// processEventsMultiThread — InSizeMove guard
// =============================================================================

func TestProcessEventsMultiThread_InSizeMoveBlocksResize(t *testing.T) {
	// A resize event that would normally trigger onResize.
	mgr := &singleEventManager{
		event: platform.Event{
			Type:     platform.EventResize,
			WindowID: 0, // primary window (WindowID 0 is always primary)
			Width:    800, Height: 600,
			PhysicalWidth: 1600, PhysicalHeight: 1200,
		},
	}

	app := &App{
		manager:    mgr,
		renderLoop: &mockRenderLoop{},
		platWindow: &mockWindowWithSizeMove{inSizeMove: true},
	}

	var resizeCalled bool
	app.onResize = func(w, h int) { resizeCalled = true }

	app.processEventsMultiThread()

	if resizeCalled {
		t.Error("onResize must not be called while InSizeMove() returns true")
	}
}

func TestProcessEventsMultiThread_InSizeMoveAllowsResize(t *testing.T) {
	mgr := &singleEventManager{
		event: platform.Event{
			Type:     platform.EventResize,
			WindowID: 0,
			Width:    800, Height: 600,
			PhysicalWidth: 1600, PhysicalHeight: 1200,
		},
	}
	invalidateCalled := false
	app := &App{
		manager:     mgr,
		renderLoop:  &mockRenderLoop{},
		platWindow:  &mockWindowWithSizeMove{inSizeMove: false},
		invalidator: newInvalidator(func() { invalidateCalled = true }),
	}

	var resizeW, resizeH int
	app.onResize = func(w, h int) { resizeW, resizeH = w, h }

	app.processEventsMultiThread()

	if resizeW != 800 || resizeH != 600 {
		t.Errorf("onResize got (%d,%d), want (800,600)", resizeW, resizeH)
	}
	if !invalidateCalled {
		t.Error("RequestRedraw should have been called after resize")
	}
}

// =============================================================================
// initPlatform — LiveResizePhaser hooks
// =============================================================================

func TestInitPlatform_LiveResizePhaserHooks(t *testing.T) {
	mockWin := &mockLiveResizePhaserWindow{}

	old := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager {
		return &mockManagerWithWindow{win: mockWin}
	}
	defer func() { newPlatformManagerFn = old }()

	app := NewApp(DefaultConfig())
	platWin, err := app.initPlatform()
	if err != nil {
		t.Fatalf("initPlatform: %v", err)
	}
	defer platWin.Destroy()

	if mockWin.startHook == nil || mockWin.endHook == nil {
		t.Skip("window did not implement LiveResizePhaser — hooks not registered")
	}

	// Case 1: renderLoop == nil → both hooks return early.
	mockWin.startHook()
	mockWin.endHook()

	// Case 2: renderLoop set, renderer == nil → both hooks return early.
	app.renderLoop = &mockRenderLoop{}
	mockWin.startHook()
	mockWin.endHook()

	// Case 3: renderLoop + renderer set, primary == nil → RunOnRenderThread called
	// but inner setTransactionPresent guard prevents GPU access.
	app.renderer = &Renderer{}
	mockWin.startHook()
	mockWin.endHook()

	// Case 4: primary non-nil (surface nil) → setTransactionPresent is a no-op
	// (ws.surface == nil early-return).
	app.renderer.primary = &RenderTarget{}
	mockWin.startHook()
	mockWin.endHook()
}

// =============================================================================
// renderFrameGPU — targeted paths without GPU
// =============================================================================

// TestRenderFrameGPU_NilSurfaces exercises the ws==nil continue path.
func TestRenderFrameGPU_NilSurfaces(t *testing.T) {
	app := &App{
		renderLoop: &mockRenderLoop{},
		renderer:   &Renderer{},
		platWindow: nil,
	}
	frames := []windowFrame{
		{window: &Window{surface: nil}, onDraw: func(*Context) {}, scale: 1.0},
		{window: &Window{surface: nil}, onDraw: func(*Context) {}, scale: 1.0},
	}
	app.renderFrameGPU(frames)
}

// TestRenderFrameGPU_UnconfiguredSurface exercises ws!=nil but CanRender()==false
// with no platWindow set on the RenderTarget.
func TestRenderFrameGPU_UnconfiguredSurface(t *testing.T) {
	ws := &RenderTarget{} // CanRender()==false, platWindow==nil

	app := &App{
		renderLoop: &mockRenderLoop{},
		renderer:   &Renderer{},
		platWindow: nil,
	}
	frames := []windowFrame{
		{window: &Window{surface: ws}, onDraw: func(*Context) {}, scale: 1.0},
	}
	app.renderFrameGPU(frames)

	// The frame had no GPU work, so RequestRedraw must have been queued.
	if !app.pendingRedraw.Load() {
		t.Error("renderFrameGPU should queue a redraw when frameStarted==false")
	}
}

// TestRenderFrameGPU_UnconfiguredSurfaceWithWindow exercises the !CanRender &&
// platWindow!=nil path, where PhysicalSize()==(0,0) so ws.resize is skipped.
func TestRenderFrameGPU_UnconfiguredSurfaceWithWindow(t *testing.T) {
	ws := &RenderTarget{
		platWindow: &mockWindow{width: 0, height: 0},
	}

	app := &App{
		renderLoop: &mockRenderLoop{},
		renderer:   &Renderer{},
		platWindow: nil,
	}
	frames := []windowFrame{
		{window: &Window{surface: ws}, onDraw: func(*Context) {}, scale: 1.0},
	}
	app.renderFrameGPU(frames)
}

// TestRenderFrameGPU_PendingResizeNoPrimary exercises ConsumePendingResize
// returning ok=true when renderer.primary is nil (Resize skipped).
func TestRenderFrameGPU_PendingResizeNoPrimary(t *testing.T) {
	app := &App{
		renderLoop: &mockRenderLoopResizing{w: 100, h: 200},
		renderer:   &Renderer{primary: nil},
		platWindow: nil,
	}
	app.renderFrameGPU(nil)
}

// TestRenderFrameGPU_PlatWindowSyncSkipsResizeAtZeroSize exercises the
// platWindow sync block (platWindow!=nil, primary!=nil) when PhysicalSize==(0,0),
// so the inner Resize call is skipped.
func TestRenderFrameGPU_PlatWindowSyncSkipsResizeAtZeroSize(t *testing.T) {
	primary := &RenderTarget{}
	app := &App{
		renderLoop: &mockRenderLoop{},
		renderer:   &Renderer{primary: primary},
		platWindow: &mockWindow{width: 0, height: 0},
	}
	app.renderFrameGPU(nil)
}

// =============================================================================
// renderFrameMultiThread — exercises the runInFramePool wrapper.
// On Linux CI (!darwin) this also covers framepool_other.go:runInFramePool.
// =============================================================================

func TestRenderFrameMultiThread_InvokesFramePool(t *testing.T) {
	app := &App{
		renderLoop:    &mockRenderLoop{},
		renderer:      &Renderer{},
		windowManager: newWindowManager(),
	}

	id := app.windowManager.allocate()
	w := &Window{
		id:         id,
		visible:    true,
		platWindow: &mockWindow{},
		onDraw:     func(*Context) {}, // no GPU work
		surface:    nil,               // ws==nil → continue in renderFrameGPU
	}
	app.windowManager.add(w)

	app.renderFrameMultiThread()
}
