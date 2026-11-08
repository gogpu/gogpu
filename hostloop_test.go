package gogpu

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// queuedEventManager returns its events from PollEvents in order, then
// EventNone. Rewind makes the same events pending again.
type queuedEventManager struct {
	mockManagerWithWindow
	events    []platform.Event
	nextEvent int
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

// startedHostLoop starts a HostLoop over window, with a renderer that has no
// GPU: its surface is sized to the window and never configured, so an
// acquire is attempted and fails.
func startedHostLoop(t *testing.T, window platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *queuedEventManager) {
	t.Helper()
	manager := &queuedEventManager{mockManagerWithWindow: mockManagerWithWindow{win: window}, events: events}

	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	oldRenderer := newRendererFn
	newRendererFn = func(platformWindow platform.PlatformWindow, _ types.GraphicsAPI, _ bool, _ gputypes.PowerPreference, _ bool) (*Renderer, error) {
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

	app := NewApp(DefaultConfig())
	loop := NewHostLoop(app)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return app, loop, manager
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
	_, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	if err := loop.Start(); !errors.Is(err, ErrHostLoopStarted) {
		t.Errorf("second Start: got %v, want ErrHostLoopStarted", err)
	}
}

func TestHostLoop_RunRefusesWhileStarted(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	if err := app.Run(); !errors.Is(err, ErrHostLoopStarted) {
		t.Errorf("Run: got %v, want ErrHostLoopStarted", err)
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

func TestHostLoop_NotRunningAfterWindowCloses(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{windowID: 1, width: 800, height: 600},
		platform.Event{Type: platform.EventClose, WindowID: 1})

	loop.PumpEvents()
	if loop.Running() {
		t.Error("Running after the window closed: got true, want false")
	}
	loop.Stop() // the close cleared the app's window; Stop must not panic
}

func TestHostLoop_StopBeforeStartDoesNothing(t *testing.T) {
	loop := NewHostLoop(NewApp(DefaultConfig()))

	loop.Stop() // nothing was created; Stop must not panic
}

func TestHostLoop_StopAfterFailedStartDoesNothing(t *testing.T) {
	manager := &queuedEventManager{mockManagerWithWindow: mockManagerWithWindow{win: &mockWindow{width: 800, height: 600}}}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	errNoGPU := errors.New("no GPU")
	oldRenderer := newRendererFn
	newRendererFn = func(platform.PlatformWindow, types.GraphicsAPI, bool, gputypes.PowerPreference, bool) (*Renderer, error) {
		return nil, errNoGPU
	}
	t.Cleanup(func() {
		newPlatformManagerFn = oldManager
		newRendererFn = oldRenderer
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

func TestHostLoop_BeginFrameAttemptsAcquire(t *testing.T) {
	enabledStats(t)
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	// The renderer has no GPU, so the acquire fails and is recorded.
	if _, ok := loop.BeginFrame(app.PrimaryWindow()); ok {
		t.Fatal("BeginFrame without a GPU: got ok=true, want false")
	}
	if !app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame with an open window recorded no acquire attempt")
	}
}

func TestHostLoop_BeginFrameSkipsHiddenWindow(t *testing.T) {
	enabledStats(t)
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	window := app.PrimaryWindow()
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
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 0, height: 0})
	defer loop.Stop()

	if _, ok := loop.BeginFrame(app.PrimaryWindow()); ok {
		t.Error("BeginFrame on a zero-size window: got ok=true, want false")
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame on a zero-size window attempted an acquire")
	}
}

func TestHostLoop_BeginFrameSkipsClosedFrameGate(t *testing.T) {
	enabledStats(t)
	app, loop, _ := startedHostLoop(t, &mockFrameGaterWindow{mockWindow: mockWindow{width: 800, height: 600}, ready: false})
	defer loop.Stop()

	if _, ok := loop.BeginFrame(app.PrimaryWindow()); ok {
		t.Error("BeginFrame with the frame gate closed: got ok=true, want false")
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame with the frame gate closed attempted an acquire")
	}
}

func TestHostLoop_PumpEventsDispatches(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventResize, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768},
	)
	defer loop.Stop()

	var pressedKey, releasedKey gpucontext.Key
	var resizeWidth, resizeHeight int
	app.EventSource().OnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) { pressedKey = key })
	app.EventSource().OnKeyRelease(func(key gpucontext.Key, _ gpucontext.Modifiers) { releasedKey = key })
	app.OnResize(func(width, height int) { resizeWidth, resizeHeight = width, height })

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
	_, loop, manager := startedHostLoop(t, &mockWindow{width: 800, height: 600},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventResize, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768},
	)
	defer loop.Stop()

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
	app, loop, manager := startedHostLoop(t, &mockWindow{windowID: 1, width: 800, height: 600},
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
	window := app.PrimaryWindow()
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
