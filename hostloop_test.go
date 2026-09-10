package gogpu

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// queuedEventManager hands out its windows in order from CreateWindow,
// recording each config it was asked for, and
// returns its events from PollEvents in order, then EventNone. Rewind makes
// the same events pending again.
type queuedEventManager struct {
	mockManager
	windows       []platform.PlatformWindow
	windowConfigs []platform.Config
	createdWindow int
	events        []platform.Event
	nextEvent     int
}

func (m *queuedEventManager) CreateWindow(config platform.Config) (platform.PlatformWindow, error) {
	m.windowConfigs = append(m.windowConfigs, config)
	if m.createdWindow == len(m.windows) {
		return nil, errors.New("queuedEventManager: no window left to create")
	}
	window := m.windows[m.createdWindow]
	m.createdWindow++
	return window, nil
}

func (m *queuedEventManager) PollEvents() platform.Event {
	if m.nextEvent == len(m.events) {
		return platform.Event{}
	}
	event := m.events[m.nextEvent]
	m.nextEvent++
	return event
}

func (m *queuedEventManager) rewind() { m.nextEvent = 0 }

// startedHostLoopWithNoWindow starts a HostLoop on the software backend over a
// platform that hands out windows, in order, to NewWindow. It locks the test
// to its OS thread, as HostLoop requires: on macOS a frame's autorelease pool
// must be popped on the thread that pushed it.
func startedHostLoopWithNoWindow(t *testing.T, windows []platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *queuedEventManager) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	manager := &queuedEventManager{windows: windows, events: events}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	t.Cleanup(func() { newPlatformManagerFn = oldManager })

	app := NewApp(DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware))
	loop := NewHostLoop(app)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start on the software backend: %v", err)
	}
	return app, loop, manager
}

// startedHostLoop starts a HostLoop on the software backend, then opens
// window through NewWindow and shows it.
func startedHostLoop(t *testing.T, window platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *Window) {
	t.Helper()
	app, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{window}, events...)
	opened, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	opened.Show()
	return app, loop, opened
}

// mockedRunPlatform makes the next Run open window and get a renderer that
// has no GPU: its surface is sized to the window and never configured, so an
// acquire is attempted and fails.
func mockedRunPlatform(t *testing.T, window platform.PlatformWindow) {
	t.Helper()
	manager := &queuedEventManager{windows: []platform.PlatformWindow{window}}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	oldRenderer := newRendererFn
	newRendererFn = func(platformWindow platform.PlatformWindow, _ types.GraphicsAPI, _ bool, _ gputypes.PowerPreference, _ bool, _ gputypes.Features) (*Renderer, error) {
		renderer := &Renderer{}
		width, height := platformWindow.PhysicalSize()
		renderer.primary = &RenderTarget{
			renderer:   renderer,
			platWindow: platformWindow,
			width:      uint32(width),
			height:     uint32(height),
		}
		return renderer, nil
	}
	t.Cleanup(func() {
		newPlatformManagerFn = oldManager
		newRendererFn = oldRenderer
	})
}

// enabledStats turns frame stats on for the test: a failed acquire records a
// skipped present, which is how a test sees that BeginFrame tried to acquire.
func enabledStats(t *testing.T) {
	t.Helper()
	wasEnabled := StatsEnabled()
	EnableStats()
	t.Cleanup(func() {
		if !wasEnabled {
			DisableStats()
		}
	})
}

// skipWhenRaceInstrumented skips an allocation count in a build instrumented
// by github.com/kolkov/racedetector, the race detector CI runs: the calls it
// adds to the code allocate, so the count no longer measures the code. The
// tool runs go test with GONOSUMCHECK naming its own module, and the test
// binary inherits that environment.
func skipWhenRaceInstrumented(t *testing.T) {
	t.Helper()
	if strings.Contains(os.Getenv("GONOSUMCHECK"), "github.com/kolkov/racedetector") {
		t.Skip("allocation counts include racedetector's instrumentation")
	}
}

func TestHostLoop_StartTwice(t *testing.T) {
	_, loop, _ := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if err := loop.Start(); !errors.Is(err, ErrHostLoopStarted) {
		t.Errorf("second Start: got %v, want ErrHostLoopStarted", err)
	}
}

func TestHostLoop_RunRefusesWhileStarted(t *testing.T) {
	app, loop, _ := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if err := app.Run(); !errors.Is(err, ErrHostLoopStarted) {
		t.Errorf("Run: got %v, want ErrHostLoopStarted", err)
	}
}

func TestHostLoop_StartOpensNoWindow(t *testing.T) {
	app, loop, manager := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if manager.createdWindow != 0 {
		t.Errorf("Start created %d platform windows, want 0", manager.createdWindow)
	}
	if count := app.WindowCount(); count != 0 {
		t.Errorf("WindowCount after Start: got %d, want 0", count)
	}
	if !loop.Running() {
		t.Error("Running after Start with no window: got false, want true")
	}
	if app.renderer.device == nil {
		t.Error("Start with no window brought up no device")
	}
}

func TestHostLoop_NewWindowBeforeStartFails(t *testing.T) {
	loop := NewHostLoop(NewApp(DefaultConfig()))
	if _, err := loop.NewWindow(DefaultConfig()); err == nil {
		t.Error("NewWindow before Start: got nil error")
	}
}

func TestHostLoop_NewWindowOpensHidden(t *testing.T) {
	platformWindow := &mockWindow{width: 800, height: 600}
	_, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{platformWindow})
	defer loop.Stop()

	window, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if platformWindow.visible || window.Visible() {
		t.Error("NewWindow showed the window")
	}
	if _, ok := loop.BeginFrame(window); ok {
		t.Error("BeginFrame before Show: got ok=true, want false")
	}
	window.Show()
	if !platformWindow.visible || !window.Visible() {
		t.Error("Show left the window hidden")
	}
}

// macOS and Windows apply a window's fullscreen themselves, from the
// platform config NewWindow passes on.
func TestHostLoop_NewWindowAsksThePlatformForFullscreen(t *testing.T) {
	_, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{&mockWindow{width: 800, height: 600}})
	defer loop.Stop()

	if _, err := loop.NewWindow(DefaultConfig().WithFullscreen()); err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if len(manager.windowConfigs) != 1 || !manager.windowConfigs[0].Fullscreen {
		t.Errorf("platform configs: got %+v, want one asking fullscreen", manager.windowConfigs)
	}
}

func TestHostLoop_NewWindowPresentsWithConfigVSync(t *testing.T) {
	_, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{
		&mockWindow{windowID: 1, width: 800, height: 600},
		&mockWindow{windowID: 2, width: 800, height: 600},
	})
	defer loop.Stop()

	withVSync, err := loop.NewWindow(DefaultConfig().WithVSync(true))
	if err != nil {
		t.Fatalf("NewWindow with vsync: %v", err)
	}
	withoutVSync, err := loop.NewWindow(DefaultConfig().WithVSync(false))
	if err != nil {
		t.Fatalf("NewWindow without vsync: %v", err)
	}
	if !withVSync.surface.vsync || withoutVSync.surface.vsync {
		t.Errorf("surface vsync: got %v and %v, want true and false", withVSync.surface.vsync, withoutVSync.surface.vsync)
	}
}

func TestHostLoop_SeveralWindowsEachDrawAFrame(t *testing.T) {
	app, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{
		&mockWindow{windowID: 1, width: 800, height: 600},
		&mockWindow{windowID: 2, width: 640, height: 480},
		&mockWindow{windowID: 3, width: 320, height: 240},
	})
	defer loop.Stop()

	windows := make([]*Window, 0, 3)
	for range 3 {
		window, err := loop.NewWindow(DefaultConfig())
		if err != nil {
			t.Fatalf("NewWindow: %v", err)
		}
		window.Show()
		windows = append(windows, window)
	}
	if count := app.WindowCount(); count != 3 {
		t.Errorf("WindowCount: got %d, want 3", count)
	}
	for i, window := range windows {
		context, ok := loop.BeginFrame(window)
		if !ok {
			t.Fatalf("BeginFrame on window %d: got ok=false", i)
		}
		wantWidth, wantHeight := window.PhysicalSize()
		if width, height := context.FramebufferSize(); width != wantWidth || height != wantHeight {
			t.Errorf("window %d framebuffer: got %dx%d, want %dx%d", i, width, height, wantWidth, wantHeight)
		}
		context.Clear(0.1, 0.2, 0.3, 1)
		loop.EndFrame(context)
	}
}

func TestHostLoop_StopClosesOpenWindows(t *testing.T) {
	var closedWindows []WindowID
	app, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{
		&mockWindow{windowID: 1, width: 800, height: 600},
		&mockWindow{windowID: 2, width: 800, height: 600},
	})
	app.OnAnyWindowClosed(func(window WindowID) { closedWindows = append(closedWindows, window) })
	first, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	second, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}

	loop.Stop()

	if count := app.WindowCount(); count != 0 {
		t.Errorf("WindowCount after Stop: got %d, want 0", count)
	}
	if first.surface.surface != nil || second.surface.surface != nil {
		t.Error("Stop left a window's surface unreleased")
	}
	if len(closedWindows) != 2 {
		t.Errorf("windows reported closed by Stop: got %v, want both", closedWindows)
	}
}

func TestHostLoop_RunningAfterStart(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	if !loop.Running() {
		t.Error("Running after Start: got false, want true")
	}
}

func TestHostLoop_NotRunningAfterQuit(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	app.Quit()
	if loop.Running() {
		t.Error("Running after Quit: got true, want false")
	}
}

func TestHostLoop_NotRunningAfterLastWindowCloses(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{windowID: 1, width: 800, height: 600},
		platform.Event{Type: platform.EventClose, WindowID: 1})

	loop.PumpEvents()
	if loop.Running() {
		t.Error("Running after the last window closed: got true, want false")
	}
	loop.Stop() // the close destroyed the window; Stop must not panic
}

func TestHostLoop_StopBeforeStartDoesNothing(t *testing.T) {
	loop := NewHostLoop(NewApp(DefaultConfig()))

	loop.Stop() // nothing was created; Stop must not panic
}

func TestHostLoop_StopAfterFailedStartDoesNothing(t *testing.T) {
	manager := &queuedEventManager{}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	errNoGPU := errors.New("no GPU")
	oldRenderer := newWindowlessRendererFn
	newWindowlessRendererFn = func(types.GraphicsAPI, gputypes.PowerPreference) (*Renderer, error) {
		return nil, errNoGPU
	}
	t.Cleanup(func() {
		newPlatformManagerFn = oldManager
		newWindowlessRendererFn = oldRenderer
	})

	loop := NewHostLoop(NewApp(DefaultConfig()))
	if err := loop.Start(); !errors.Is(err, errNoGPU) {
		t.Fatalf("Start without a renderer: got %v, want %v", err, errNoGPU)
	}
	loop.Stop() // Start destroyed what it created; Stop must not panic
}

func TestHostLoop_StopTwiceTearsDownOnce(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	closeCount := 0
	app.OnClose(func() { closeCount++ })

	loop.Stop()
	loop.Stop()
	if closeCount != 1 {
		t.Errorf("onClose after two Stops: called %d times, want 1", closeCount)
	}
}

// windowlessManager starts, then fails to create any window, and counts how
// often it is destroyed.
type windowlessManager struct {
	mockManager
	destroyCount int
}

var errNoWindow = errors.New("no window")

func (m *windowlessManager) CreateWindow(platform.Config) (platform.PlatformWindow, error) {
	return nil, errNoWindow
}

func (m *windowlessManager) Destroy() { m.destroyCount++ }

func TestRun_DestroysPlatformWhenWindowFails(t *testing.T) {
	manager := &windowlessManager{}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	t.Cleanup(func() { newPlatformManagerFn = oldManager })

	if err := NewApp(DefaultConfig()).Run(); !errors.Is(err, errNoWindow) {
		t.Fatalf("Run without a window: got %v, want %v", err, errNoWindow)
	}
	if manager.destroyCount != 1 {
		t.Errorf("platform after a Run whose window failed: destroyed %d times, want 1", manager.destroyCount)
	}
}

// liveResizeWindow records the live-resize hooks installed on it.
type liveResizeWindow struct {
	mockWindow
	resizeHook     func()
	phaseStartHook func()
	phaseEndHook   func()
}

func (w *liveResizeWindow) SetLiveResizeHook(hook func()) { w.resizeHook = hook }

func (w *liveResizeWindow) SetLiveResizePhaseHooks(start, end func()) {
	w.phaseStartHook = start
	w.phaseEndHook = end
}

func TestHostLoop_NewWindowInstallsNoLiveResizeHooks(t *testing.T) {
	window := &liveResizeWindow{mockWindow: mockWindow{width: 800, height: 600}}
	_, loop, _ := startedHostLoop(t, window)
	defer loop.Stop()

	if window.resizeHook != nil {
		t.Error("live-resize hook after NewWindow: installed, want none, so nothing draws inside PumpEvents")
	}
	if window.phaseStartHook != nil || window.phaseEndHook != nil {
		t.Error("live-resize phase hooks after NewWindow: installed, want none")
	}
}

func TestRun_InstallsLiveResizeHooks(t *testing.T) {
	window := &liveResizeWindow{mockWindow: mockWindow{width: 800, height: 600}}
	mockedRunPlatform(t, window)
	app := NewApp(DefaultConfig())
	app.OnUpdate(func(float64) { app.Quit() })

	if err := app.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if window.resizeHook == nil {
		t.Error("live-resize hook after Run: none, want Run's, which draws during a macOS live resize")
	}
	if window.phaseStartHook == nil || window.phaseEndHook == nil {
		t.Error("live-resize phase hooks after Run: none, want Run's")
	}
}

func TestHostLoop_BeginFrameAcquires(t *testing.T) {
	_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	context, ok := loop.BeginFrame(window)
	if !ok {
		t.Fatal("BeginFrame on a shown window: got ok=false, want true")
	}
	if width, height := context.FramebufferSize(); width != 800 || height != 600 {
		t.Errorf("framebuffer: got %dx%d, want 800x600", width, height)
	}
	loop.EndFrame(context)
}

func TestHostLoop_BeginFrameSkipsHiddenWindow(t *testing.T) {
	enabledStats(t)
	app, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	window.Hide()
	if _, ok := loop.BeginFrame(window); ok {
		t.Error("BeginFrame on a hidden window: got ok=true, want false")
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame on a hidden window attempted an acquire")
	}
}

func TestHostLoop_BeginFrameSkipsZeroSizeWindow(t *testing.T) {
	enabledStats(t)
	app, loop, window := startedHostLoop(t, &mockWindow{width: 0, height: 0})
	defer loop.Stop()

	if _, ok := loop.BeginFrame(window); ok {
		t.Error("BeginFrame on a zero-size window: got ok=true, want false")
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame on a zero-size window attempted an acquire")
	}
}

func TestHostLoop_BeginFrameSkipsClosedFrameGate(t *testing.T) {
	enabledStats(t)
	app, loop, window := startedHostLoop(t, &mockFrameGaterWindow{mockWindow: mockWindow{width: 800, height: 600}, ready: false})
	defer loop.Stop()

	if _, ok := loop.BeginFrame(window); ok {
		t.Error("BeginFrame with the frame gate closed: got ok=true, want false")
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame with the frame gate closed attempted an acquire")
	}
}

func TestHostLoop_PumpEventsDispatches(t *testing.T) {
	app, loop, window := startedHostLoop(t, &mockWindow{windowID: 1, width: 800, height: 600},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventResize, WindowID: 1, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768},
	)
	defer loop.Stop()

	var pressedKey, releasedKey gpucontext.Key
	var resizeWidth, resizeHeight int
	app.EventSource().OnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) { pressedKey = key })
	app.EventSource().OnKeyRelease(func(key gpucontext.Key, _ gpucontext.Modifiers) { releasedKey = key })
	window.SetOnResize(func(width, height int) { resizeWidth, resizeHeight = width, height })

	loop.PumpEvents()

	if pressedKey != gpucontext.KeyA {
		t.Errorf("key press: got %v, want KeyA", pressedKey)
	}
	if releasedKey != gpucontext.KeyA {
		t.Errorf("key release: got %v, want KeyA", releasedKey)
	}
	if resizeWidth != 1024 || resizeHeight != 768 {
		t.Errorf("resize: got %dx%d, want 1024x768", resizeWidth, resizeHeight)
	}
	var queuedEventCount int
	for {
		if _, ok := app.PollInputEvent(); !ok {
			break
		}
		queuedEventCount++
	}
	if queuedEventCount != 3 {
		t.Errorf("input event queue: got %d events, want 3", queuedEventCount)
	}
}

func TestHostLoop_FrameAcquiresAndPresents(t *testing.T) {
	app, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	for frame := range 2 {
		context, ok := loop.BeginFrame(window)
		if !ok {
			t.Fatalf("BeginFrame %d on the software backend: got ok=false, want true", frame)
		}
		context.Clear(0.1, 0.2, 0.3, 1)
		loop.EndFrame(context)
		if app.renderer.currentSurface != nil {
			t.Errorf("renderer's current surface after EndFrame %d: set, want none", frame)
		}
	}
}

func TestHostLoop_BeginFrameReconfiguresLostSurface(t *testing.T) {
	_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	surface := window.surface
	surface.state = SurfaceLost
	context, ok := loop.BeginFrame(window)
	if !ok {
		t.Fatal("BeginFrame on a lost surface: got ok=false, want true after the reconfigure")
	}
	loop.EndFrame(context)
	if !surface.CanRender() {
		t.Error("surface after BeginFrame on a lost one: cannot render, want it reconfigured")
	}
}

func TestHostLoop_ConsumeRedrawRequestBeforeStart(t *testing.T) {
	app := NewApp(DefaultConfig())
	loop := NewHostLoop(app)

	app.RequestRedraw()
	if loop.ConsumeRedrawRequest() {
		t.Error("ConsumeRedrawRequest before Start: got true, want false")
	}
}

func TestHostLoop_ConsumeRedrawRequestReportsEachRequestOnce(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	if !loop.ConsumeRedrawRequest() {
		t.Error("first call after Start: got false, want true for the first frame")
	}
	if loop.ConsumeRedrawRequest() {
		t.Error("call with no request since the last: got true, want false")
	}
	app.RequestRedraw()
	app.RequestRedraw()
	if !loop.ConsumeRedrawRequest() {
		t.Error("call after two RequestRedraw: got false, want true")
	}
	if loop.ConsumeRedrawRequest() {
		t.Error("second call after two RequestRedraw: got true, want false, the requests coalesce")
	}
}

func TestHostLoop_ConsumeRedrawRequestAfterPumpedResize(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{windowID: 7, width: 800, height: 600},
		platform.Event{Type: platform.EventResize, WindowID: 7, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768})
	defer loop.Stop()

	loop.ConsumeRedrawRequest() // Start's first frame
	loop.PumpEvents()
	if !loop.ConsumeRedrawRequest() {
		t.Error("call after a pumped resize: got false, want true")
	}
}

func TestHostLoop_PumpEventsAllocatesNothingWhenIdle(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	skipWhenRaceInstrumented(t)
	allocations := testing.AllocsPerRun(100, loop.PumpEvents)
	if allocations != 0 {
		t.Errorf("PumpEvents with no events: %v allocations per pump, want 0", allocations)
	}
}

// Each event entering the input-event queue is boxed into a
// gpucontext.InputEvent, one allocation, as in Run. Nothing else allocates.
func TestHostLoop_PumpEventsAllocatesOnlyQueuedInputEvents(t *testing.T) {
	_, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{&mockWindow{width: 800, height: 600}},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventResize, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768},
	)
	defer loop.Stop()
	window, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	window.Show()

	skipWhenRaceInstrumented(t)
	allocations := testing.AllocsPerRun(100, func() {
		manager.rewind()
		loop.PumpEvents()
	})
	if allocations != float64(len(manager.events)) {
		t.Errorf("PumpEvents with %d events: %v allocations per pump, want %d", len(manager.events), allocations, len(manager.events))
	}
}

// Events routed to a window, with callbacks on the window and on the event
// source, allocate as unrouted ones do: each is boxed once into the
// input-event queue, apart from a character under 256, which the runtime
// boxes without allocating. A typed character outside ASCII also allocates
// its text, once for both callbacks; an ASCII one does not.
func TestHostLoop_PumpEventsAllocatesOncePerRoutedEvent(t *testing.T) {
	app, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{&mockWindow{windowID: 1, width: 800, height: 600}},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventChar, Char: 'a', WindowID: 1},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventPointerMove, WindowID: 1},
		platform.Event{Type: platform.EventPointerDown, WindowID: 1},
		platform.Event{Type: platform.EventPointerUp, WindowID: 1},
		platform.Event{Type: platform.EventScroll, WindowID: 1},
		platform.Event{Type: platform.EventChar, Char: 'ж', WindowID: 1},
	)
	defer loop.Stop()
	window, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	window.Show()
	var keys, pointers, scrolls int
	var text string
	window.SetOnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) { keys++ })
	window.SetOnTextInput(func(typed string) { text = typed })
	window.SetOnPointer(func(gpucontext.PointerEvent) { pointers++ })
	window.SetOnScroll(func(gpucontext.ScrollEvent) { scrolls++ })
	app.EventSource().OnTextInput(func(typed string) { text = typed })

	skipWhenRaceInstrumented(t)
	allocations := testing.AllocsPerRun(100, func() {
		manager.rewind()
		loop.PumpEvents()
	})
	// One boxing for each of the 8 events but 'a', and one text for 'ж'.
	const want = 8
	if allocations != want {
		t.Errorf("PumpEvents with %d routed events: %v allocations per pump, want %d", len(manager.events), allocations, want)
	}
	if keys == 0 || pointers == 0 || scrolls == 0 || text != "ж" {
		t.Errorf("callbacks: %d keys, %d pointer events, %d scrolls, text %q; want each reached and the text \"ж\"", keys, pointers, scrolls, text)
	}
}

// Run's frame pumps through the same step as PumpEvents, which clears the
// previous pump's edges before applying new events: onUpdate sees a key
// press as JustPressed on the frame it arrived, and not on the next.
func TestRunFrame_OnUpdateSeesEachInputEdgeOnce(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA})
	defer loop.Stop()

	var justPressed []bool
	app.OnUpdate(func(float64) {
		justPressed = append(justPressed, app.Input().Keyboard().JustPressed(input.KeyA))
	})

	app.runFrame()
	app.runFrame()

	if len(justPressed) != 2 || !justPressed[0] || justPressed[1] {
		t.Errorf("JustPressed(KeyA) in onUpdate over two frames: got %v, want [true false]", justPressed)
	}
}
