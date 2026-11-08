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
	"github.com/gogpu/wgpu"
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
// platform that hands out windows, in order, to NewWindow.
func startedHostLoopWithNoWindow(t *testing.T, windows []platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *queuedEventManager) {
	t.Helper()
	app, loop, manager := hostLoopOver(t, DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware), windows, events...)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return app, loop, manager
}

// hostLoopOver returns a HostLoop, not started, over an app with config and a
// platform that hands out windows, in order, to NewWindow. It locks the test
// to its OS thread, as HostLoop requires: on macOS a frame's autorelease pool
// must be popped on the thread that pushed it.
func hostLoopOver(t *testing.T, config Config, windows []platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *queuedEventManager) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	manager := &queuedEventManager{windows: windows, events: events}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	t.Cleanup(func() { newPlatformManagerFn = oldManager })

	app := NewApp(config)
	return app, NewHostLoop(app), manager
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
	if app.DeviceProvider().Device() == nil {
		t.Error("Start with no window brought up no device")
	}
}

func TestHostLoop_StartRequestsTheFallbackAdapter(t *testing.T) {
	config := DefaultConfig().WithGraphicsAPI(types.GraphicsAPIAuto)
	config.ForceFallbackAdapter = true
	app, loop, _ := hostLoopOver(t, config, nil)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer loop.Stop()

	if got := app.renderer.adapter.Info().DeviceType; got != gputypes.DeviceTypeCPU {
		t.Errorf("adapter type with ForceFallbackAdapter: got %v, want %v", got, gputypes.DeviceTypeCPU)
	}
}

func TestHostLoop_StartKeepsTheRequiredLimits(t *testing.T) {
	limits := gputypes.DefaultLimits()
	limits.MaxTextureDimension2D = 1024
	config := DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware)
	config.RequiredLimits = &limits
	app, loop, _ := hostLoopOver(t, config, nil)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer loop.Stop()

	device := app.renderer.device
	if got := device.Limits(); got != limits {
		t.Errorf("device limits: got %+v, want %+v", got, limits)
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: 2048, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding,
	})
	if err == nil {
		texture.Release()
		t.Error("CreateTexture beyond the required MaxTextureDimension2D: got no error")
	}
}

func TestHostLoop_StartFailsNamingWhatTheAdapterLacks(t *testing.T) {
	beyondBindGroups := gputypes.DefaultLimits()
	beyondBindGroups.MaxBindGroups = 1 << 20
	belowAlignment := gputypes.DefaultLimits()
	belowAlignment.MinUniformBufferOffsetAlignment = 1
	tests := []struct {
		name     string
		features gputypes.Features
		limits   *gputypes.Limits
		want     []string
	}{
		{"feature", gputypes.Features(gputypes.FeatureTimestampQuery), nil, []string{"TimestampQuery"}},
		{"maximum beyond", 0, &beyondBindGroups, []string{"MaxBindGroups"}},
		{"alignment below", 0, &belowAlignment, []string{"MinUniformBufferOffsetAlignment"}},
		{"feature and limit", gputypes.Features(gputypes.FeatureTimestampQuery), &beyondBindGroups, []string{"TimestampQuery", "MaxBindGroups"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware).WithRequiredFeatures(test.features)
			config.RequiredLimits = test.limits
			_, loop, _ := hostLoopOver(t, config, nil)
			err := loop.Start()
			if err == nil {
				loop.Stop()
				t.Fatal("Start: got no error")
			}
			for _, name := range test.want {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("Start error %q does not name %s", err, name)
				}
			}
		})
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
