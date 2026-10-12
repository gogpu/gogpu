package gogpu

import (
	"testing"
	"time"

	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

type pacingManager struct {
	mockManager
	events []platform.Event
	wait   func()
	wake   chan struct{}
	waits  int
}

func (m *pacingManager) PollEvents() platform.Event {
	if len(m.events) == 0 {
		return platform.Event{}
	}
	e := m.events[0]
	m.events = m.events[1:]
	return e
}
func (m *pacingManager) WaitEvents() {
	m.waits++
	if m.wait != nil {
		m.wait()
		return
	}
	<-m.wake
}
func (m *pacingManager) WakeUp() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func newPacingTestApp() (*App, *pacingManager, *mockFrameGaterWindow) {
	manager := &pacingManager{wake: make(chan struct{}, 1)}
	window := &mockFrameGaterWindow{ready: true}
	window.scaleFactor = 1
	app := &App{manager: manager, platWindow: window, renderLoop: &mockRenderLoop{}, renderer: &Renderer{}, windowManager: newWindowManager(), animations: &AnimationController{}, inputState: input.New(), config: Config{ContinuousRender: true}, lastFrame: time.Now()}
	app.running.Store(true)
	app.invalidator = newInvalidator(manager.WakeUp)
	target := &RenderTarget{renderer: app.renderer}
	app.renderer.primary = target
	app.windowManager.add(&Window{id: 1, platWindow: window, visible: true, surface: target, onDraw: func(*Context) { target.pixelPresented = true }})
	return app, manager, window
}

func TestContinuousWaitPreservesInputUntilUpdate(t *testing.T) {
	app, manager, window := newPacingTestApp()
	window.ready = false
	manager.events = []platform.Event{{Type: platform.EventKeyDown, Key: gpucontext.KeyA}}
	manager.wait = func() {
		if manager.waits == 1 {
			manager.events = append(manager.events, platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyB})
		} else {
			window.ready = true
		}
	}
	updates := 0
	app.onUpdate = func(float64) {
		updates++
		for _, key := range []input.Key{input.KeyA, input.KeyB} {
			if !app.inputState.Keyboard().JustPressed(key) {
				t.Errorf("lost key edge %v", key)
			}
		}
		for _, key := range []gpucontext.Key{gpucontext.KeyA, gpucontext.KeyB} {
			event, ok := app.PollInputEvent()
			if !ok {
				t.Fatal("lost queued input")
			}
			if got := event.(gpucontext.KeyEvent).Key; got != key {
				t.Errorf("key=%v want %v", got, key)
			}
		}
	}
	app.runFrame()
	if updates != 0 {
		t.Fatal("input wake updated the game before frame readiness")
	}
	app.runFrame()
	if updates != 1 {
		t.Fatalf("updates=%d want 1", updates)
	}
	if app.inputState.Keyboard().JustPressed(input.KeyA) {
		t.Fatal("update did not consume input edge")
	}
	if _, ok := app.PollInputEvent(); ok {
		t.Fatal("update left stale queued input")
	}
}

func TestContinuousSuccessfulFramesHaveNoArtificialLimit(t *testing.T) {
	for _, vsync := range []bool{true, false} {
		app, manager, _ := newPacingTestApp()
		app.config.VSync = vsync
		manager.wait = func() { t.Fatal("successful frame waited on an application timer") }
		updates := 0
		app.onUpdate = func(float64) { updates++ }
		for i := 0; i < 300; i++ {
			app.runFrame()
		}
		if updates != 300 {
			t.Fatalf("updates=%d want 300", updates)
		}
	}
}

func TestContinuousUnavailableFrameKeepsBackgroundUpdates(t *testing.T) {
	app, manager, window := newPacingTestApp()
	window.ready = false
	updates := 0
	app.onUpdate = func(float64) { updates++ }
	for i := 0; i < 3; i++ {
		app.runFrame()
	}
	if updates != 3 || manager.waits != 3 {
		t.Fatalf("updates=%d waits=%d", updates, manager.waits)
	}
}

func TestContinuousFailedFrameDoesNotRetryOnEveryInputEvent(t *testing.T) {
	app, manager, _ := newPacingTestApp()
	// No visible surfaces: this must be throttled on platforms without a frame gate too.
	app.platWindow = &mockWindow{}
	app.windowManager = newWindowManager()
	updates := 0
	app.onUpdate = func(float64) { updates++ }
	manager.wait = func() {
		manager.events = append(manager.events, platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA})
	}
	app.runFrame()
	if app.retryFrameAt.IsZero() {
		t.Fatal("skipped rendering did not schedule a retry")
	}
	for i := 0; i < 10; i++ {
		app.runFrame()
	}
	if updates != 1 {
		t.Fatalf("input wakes retried unavailable rendering: updates=%d", updates)
	}
	if manager.waits != 11 {
		t.Fatalf("loop did not wait on every skipped attempt: waits=%d", manager.waits)
	}
}

func TestContinuousWaitHandlesQuitWithoutUpdating(t *testing.T) {
	app, manager, window := newPacingTestApp()
	window.ready = false
	app.onUpdate = func(float64) { t.Fatal("updated after quit") }
	manager.wait = func() { app.Quit() }
	app.runFrame()
	if app.running.Load() {
		t.Fatal("still running after quit")
	}
}

func TestFrameWaitJoinsExpiredWakeBeforeReturning(t *testing.T) {
	// A timer that has started must finish touching the platform before shutdown.
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	manager := &joiningPacingManager{entered: entered, release: release}
	app := &App{manager: manager}
	go func() { app.waitForEventsUntil(time.Now().Add(time.Millisecond)); close(returned) }()
	<-entered
	select {
	case <-returned:
		t.Fatal("returned while timer still uses the platform")
	default:
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("wait did not finish")
	}
}

type joiningPacingManager struct {
	mockManager
	entered, release chan struct{}
}

func (m *joiningPacingManager) WakeUp()     { close(m.entered); <-m.release }
func (m *joiningPacingManager) WaitEvents() { <-m.entered }

func TestPresentationPacingRespectsBackendVSyncAndExplicitRequests(t *testing.T) {
	for _, tt := range []struct {
		name               string
		backend            gputypes.Backend
		vsync, force, want bool
	}{
		{"vulkan-vsync", gputypes.BackendVulkan, true, false, false},
		{"vulkan-uncapped", gputypes.BackendVulkan, false, false, false},
		{"gles-vsync", gputypes.BackendGL, true, false, true},
		{"gles-uncapped", gputypes.BackendGL, false, false, false},
		{"explicit-vulkan", gputypes.BackendVulkan, true, true, true},
		{"explicit-uncapped", gputypes.BackendVulkan, false, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			window := &countingPresentationWindow{prepareOK: true}
			target := &RenderTarget{renderer: &Renderer{backend: tt.backend}, platWindow: window, vsync: tt.vsync, presentationSyncRequested: tt.force}
			syncFrameForPresent(target)
			if got := len(window.prepareCalls) > 0; got != tt.want {
				t.Fatalf("prepared=%v want %v", got, tt.want)
			}
		})
	}
}

func TestContinuousInputRemainsAvailableDuringDraw(t *testing.T) {
	app, manager, _ := newPacingTestApp()
	manager.events = []platform.Event{{Type: platform.EventKeyDown, Key: gpucontext.KeyA}}
	window := app.windowManager.get(1)
	window.onDraw = func(*Context) {
		event, ok := app.PollInputEvent()
		if !ok || event.(gpucontext.KeyEvent).Key != gpucontext.KeyA {
			t.Fatal("update cleared input before draw")
		}
		window.surface.pixelPresented = true
	}
	app.runFrame()
}

func TestContinuousFailedAcquireReportsNoPresentation(t *testing.T) {
	app, manager, _ := newPacingTestApp()
	window := app.windowManager.get(1)
	window.onDraw = func(ctx *Context) { ctx.MarkPreserveContent() }
	// This test's surface is unconfigured, so acquisition must fail.
	manager.wait = func() {}
	app.runFrame()
	if app.retryFrameAt.IsZero() {
		t.Fatal("failed acquisition was counted as a presented frame")
	}
}
