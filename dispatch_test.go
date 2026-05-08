package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// newTestApp creates an App with WindowManager, EventSource, and InputState
// initialized for dispatch testing. No GPU or platform resources are needed.
func newTestApp() *App {
	app := NewApp(DefaultConfig())
	app.windowManager = newWindowManager()
	app.eventSource = &eventSourceAdapter{app: app}
	app.inputState = input.New()
	return app
}

// addTestWindow creates a Window with the given ID, registers it in the manager,
// and returns it for callback registration.
func addTestWindow(app *App, platformID platform.WindowID) *Window {
	w := &Window{
		id:         app.windowManager.allocate(),
		platformID: platformID,
	}
	app.windowManager.add(w)
	return w
}

// --- Key event dispatch tests ---

// Regression test: EventSource() called before Run() must preserve callbacks.
// Bug: Run() unconditionally overwrote a.eventSource with a new instance,
// discarding callbacks registered by UI frameworks before Run().
func TestEventSource_CallbacksSurviveInit(t *testing.T) {
	app := NewApp(DefaultConfig())

	var received bool
	es := app.EventSource()
	es.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		received = true
	})

	// Simulate what Run() does — ensure subsystems exist without overwriting.
	_ = app.Input()
	_ = app.EventSource()

	if app.eventSource == nil {
		t.Fatal("eventSource is nil after init")
	}
	app.eventSource.dispatchKeyPress(gpucontext.KeyA, 0)

	if !received {
		t.Error("callback registered before init was lost — eventSource was overwritten")
	}
}

func TestInput_StateSurvivesInit(t *testing.T) {
	app := NewApp(DefaultConfig())

	state1 := app.Input()
	_ = app.EventSource() // triggers init path
	state2 := app.Input()

	if state1 != state2 {
		t.Error("Input() returned different instance after init — inputState was overwritten")
	}
}

func TestDispatchKeyEvent_RoutesToCorrectWindow(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var receivedKey gpucontext.Key
	w.SetOnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		receivedKey = key
	})

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
		Mods:     gpucontext.ModShift,
	}, true)

	if receivedKey != gpucontext.KeyA {
		t.Errorf("window callback key = %v, want KeyA", receivedKey)
	}
}

func TestDispatchKeyEvent_UnknownWindowID(t *testing.T) {
	app := newTestApp()

	// Must not panic when window is not found.
	app.dispatchKeyEvent(&platform.Event{
		WindowID: platform.NewWindowID(),
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
	}, true)
}

func TestDispatchKeyEvent_ReachesEventSource(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	var esKey gpucontext.Key
	app.eventSource.onKeyPress = func(key gpucontext.Key, mods gpucontext.Modifiers) {
		esKey = key
	}

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeySpace,
	}, true)

	if esKey != gpucontext.KeySpace {
		t.Errorf("EventSource key = %v, want KeySpace", esKey)
	}
}

func TestDispatchKeyEvent_ReachesBothWindowAndEventSource(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	windowCalled := false
	esCalled := false

	w.SetOnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) {
		windowCalled = true
	})
	app.eventSource.onKeyPress = func(gpucontext.Key, gpucontext.Modifiers) {
		esCalled = true
	}

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyEnter,
	}, true)

	if !windowCalled {
		t.Error("per-window callback was not called")
	}
	if !esCalled {
		t.Error("EventSource callback was not called")
	}
}

func TestDispatchKeyEvent_UpdatesInputState(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
	}, true)

	if !app.inputState.Keyboard().Pressed(input.KeyA) {
		t.Error("InputState should report KeyA pressed after key down dispatch")
	}

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventKeyUp,
		Key:      gpucontext.KeyA,
	}, false)

	if app.inputState.Keyboard().Pressed(input.KeyA) {
		t.Error("InputState should report KeyA released after key up dispatch")
	}
}

func TestDispatchKeyEvent_ReleaseRoutesToWindow(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var releasedKey gpucontext.Key
	w.SetOnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		releasedKey = key
	})

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventKeyUp,
		Key:      gpucontext.KeyEscape,
	}, false)

	if releasedKey != gpucontext.KeyEscape {
		t.Errorf("release key = %v, want KeyEscape", releasedKey)
	}
}

func TestDispatchKeyEvent_NilEventSourceSafe(t *testing.T) {
	app := newTestApp()
	app.eventSource = nil

	// Must not panic when eventSource is nil.
	app.dispatchKeyEvent(&platform.Event{
		WindowID: platform.NewWindowID(),
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
	}, true)
}

func TestDispatchKeyEvent_NilInputStateSafe(t *testing.T) {
	app := newTestApp()
	app.inputState = nil

	// Must not panic when inputState is nil.
	app.dispatchKeyEvent(&platform.Event{
		WindowID: platform.NewWindowID(),
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
	}, true)
}

// --- Char event dispatch tests ---

func TestDispatchCharEvent_RoutesToCorrectWindow(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var received string
	w.SetOnTextInput(func(text string) {
		received = text
	})

	app.dispatchCharEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventChar,
		Char:     'W',
	})

	if received != "W" {
		t.Errorf("text = %q, want %q", received, "W")
	}
}

func TestDispatchCharEvent_UnknownWindowID(t *testing.T) {
	app := newTestApp()

	// Must not panic.
	app.dispatchCharEvent(&platform.Event{
		WindowID: platform.NewWindowID(),
		Type:     platform.EventChar,
		Char:     'A',
	})
}

func TestDispatchCharEvent_ReachesEventSource(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	var esText string
	app.eventSource.onTextInput = func(text string) {
		esText = text
	}

	app.dispatchCharEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventChar,
		Char:     'Z',
	})

	if esText != "Z" {
		t.Errorf("EventSource text = %q, want %q", esText, "Z")
	}
}

func TestDispatchCharEvent_Unicode(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var received string
	w.SetOnTextInput(func(text string) {
		received = text
	})

	app.dispatchCharEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventChar,
		Char:     0x4E16, // '世' in Chinese
	})

	if received != "世" {
		t.Errorf("text = %q, want %q", received, "世")
	}
}

// --- Pointer event dispatch tests ---

func TestDispatchPointerEvent_RoutesToCorrectWindow(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var received gpucontext.PointerEvent
	w.SetOnPointer(func(ev gpucontext.PointerEvent) {
		received = ev
	})

	app.dispatchPointerEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventPointerMove,
		Pointer: gpucontext.PointerEvent{
			Type: gpucontext.PointerMove,
			X:    150,
			Y:    250,
		},
	})

	if received.X != 150 || received.Y != 250 {
		t.Errorf("pointer = (%f, %f), want (150, 250)", received.X, received.Y)
	}
}

func TestDispatchPointerEvent_UnknownWindowID(t *testing.T) {
	app := newTestApp()

	// Must not panic.
	app.dispatchPointerEvent(&platform.Event{
		WindowID: platform.NewWindowID(),
		Type:     platform.EventPointerDown,
		Pointer: gpucontext.PointerEvent{
			Type: gpucontext.PointerDown,
		},
	})
}

func TestDispatchPointerEvent_ReachesEventSource(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	var esReceived gpucontext.PointerEvent
	app.eventSource.onPointer = func(ev gpucontext.PointerEvent) {
		esReceived = ev
	}

	app.dispatchPointerEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventPointerDown,
		Pointer: gpucontext.PointerEvent{
			Type:   gpucontext.PointerDown,
			Button: gpucontext.ButtonLeft,
			X:      10,
			Y:      20,
		},
	})

	if esReceived.X != 10 || esReceived.Y != 20 {
		t.Errorf("EventSource pointer = (%f, %f), want (10, 20)", esReceived.X, esReceived.Y)
	}
}

func TestDispatchPointerEvent_UpdatesMouseState(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	app.dispatchPointerEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventPointerMove,
		Pointer: gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerType: gpucontext.PointerTypeMouse,
			X:           300,
			Y:           400,
		},
	})

	mx, my := app.inputState.Mouse().Position()
	if mx != 300 || my != 400 {
		t.Errorf("mouse position = (%f, %f), want (300, 400)", mx, my)
	}
}

// --- Scroll event dispatch tests ---

func TestDispatchScrollEvent_RoutesToCorrectWindow(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var received gpucontext.ScrollEvent
	w.SetOnScroll(func(ev gpucontext.ScrollEvent) {
		received = ev
	})

	app.dispatchScrollEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventScroll,
		Scroll: gpucontext.ScrollEvent{
			DeltaX: 3.5,
			DeltaY: -7.0,
		},
	})

	if received.DeltaX != 3.5 || received.DeltaY != -7.0 {
		t.Errorf("scroll = (%f, %f), want (3.5, -7.0)", received.DeltaX, received.DeltaY)
	}
}

func TestDispatchScrollEvent_UnknownWindowID(t *testing.T) {
	app := newTestApp()

	// Must not panic.
	app.dispatchScrollEvent(&platform.Event{
		WindowID: platform.NewWindowID(),
		Type:     platform.EventScroll,
		Scroll: gpucontext.ScrollEvent{
			DeltaX: 1,
			DeltaY: 2,
		},
	})
}

func TestDispatchScrollEvent_ReachesEventSource(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	var esReceived gpucontext.ScrollEvent
	app.eventSource.onScrollEvent = func(ev gpucontext.ScrollEvent) {
		esReceived = ev
	}

	app.dispatchScrollEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventScroll,
		Scroll: gpucontext.ScrollEvent{
			DeltaX: 8.0,
			DeltaY: -4.0,
		},
	})

	if esReceived.DeltaX != 8.0 || esReceived.DeltaY != -4.0 {
		t.Errorf("EventSource scroll = (%f, %f), want (8.0, -4.0)", esReceived.DeltaX, esReceived.DeltaY)
	}
}

func TestDispatchScrollEvent_UpdatesInputState(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	addTestWindow(app, id)

	app.dispatchScrollEvent(&platform.Event{
		WindowID: id,
		Type:     platform.EventScroll,
		Scroll: gpucontext.ScrollEvent{
			DeltaX: 5.5,
			DeltaY: -12.0,
		},
	})

	// Mouse.Scroll() reads frame-latched values; UpdateFrame must be called
	// to transfer accumulated scroll into the frame snapshot (game-loop pattern).
	app.inputState.Mouse().UpdateFrame()

	sx, sy := app.inputState.Mouse().Scroll()
	if sx != 5.5 || sy != -12.0 {
		t.Errorf("scroll delta = (%f, %f), want (5.5, -12.0)", sx, sy)
	}
}

// --- Multi-window routing tests ---

func TestDispatch_MultiWindow_RoutesToCorrectWindow(t *testing.T) {
	app := newTestApp()

	id1 := platform.NewWindowID()
	id2 := platform.NewWindowID()
	w1 := addTestWindow(app, id1)
	w2 := addTestWindow(app, id2)

	var w1Keys []gpucontext.Key
	var w2Keys []gpucontext.Key

	w1.SetOnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		w1Keys = append(w1Keys, key)
	})
	w2.SetOnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		w2Keys = append(w2Keys, key)
	})

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id1,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
	}, true)

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id2,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyB,
	}, true)

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id1,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyC,
	}, true)

	if len(w1Keys) != 2 {
		t.Fatalf("window1 received %d keys, want 2", len(w1Keys))
	}
	if w1Keys[0] != gpucontext.KeyA || w1Keys[1] != gpucontext.KeyC {
		t.Errorf("window1 keys = %v, want [KeyA, KeyC]", w1Keys)
	}

	if len(w2Keys) != 1 {
		t.Fatalf("window2 received %d keys, want 1", len(w2Keys))
	}
	if w2Keys[0] != gpucontext.KeyB {
		t.Errorf("window2 keys = %v, want [KeyB]", w2Keys)
	}
}

func TestDispatch_MultiWindow_EventSourceReceivesAll(t *testing.T) {
	app := newTestApp()

	id1 := platform.NewWindowID()
	id2 := platform.NewWindowID()
	addTestWindow(app, id1)
	addTestWindow(app, id2)

	var esKeys []gpucontext.Key
	app.eventSource.onKeyPress = func(key gpucontext.Key, mods gpucontext.Modifiers) {
		esKeys = append(esKeys, key)
	}

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id1,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyA,
	}, true)

	app.dispatchKeyEvent(&platform.Event{
		WindowID: id2,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyB,
	}, true)

	if len(esKeys) != 2 {
		t.Fatalf("EventSource received %d keys, want 2", len(esKeys))
	}
	if esKeys[0] != gpucontext.KeyA || esKeys[1] != gpucontext.KeyB {
		t.Errorf("EventSource keys = %v, want [KeyA, KeyB]", esKeys)
	}
}

func TestDispatch_MultiWindow_PointerIsolation(t *testing.T) {
	app := newTestApp()

	id1 := platform.NewWindowID()
	id2 := platform.NewWindowID()
	w1 := addTestWindow(app, id1)
	w2 := addTestWindow(app, id2)

	var w1Pointers []gpucontext.PointerEvent
	var w2Pointers []gpucontext.PointerEvent

	w1.SetOnPointer(func(ev gpucontext.PointerEvent) {
		w1Pointers = append(w1Pointers, ev)
	})
	w2.SetOnPointer(func(ev gpucontext.PointerEvent) {
		w2Pointers = append(w2Pointers, ev)
	})

	app.dispatchPointerEvent(&platform.Event{
		WindowID: id1,
		Type:     platform.EventPointerMove,
		Pointer:  gpucontext.PointerEvent{Type: gpucontext.PointerMove, X: 10, Y: 20},
	})

	app.dispatchPointerEvent(&platform.Event{
		WindowID: id2,
		Type:     platform.EventPointerDown,
		Pointer:  gpucontext.PointerEvent{Type: gpucontext.PointerDown, X: 30, Y: 40},
	})

	if len(w1Pointers) != 1 {
		t.Fatalf("window1 received %d pointer events, want 1", len(w1Pointers))
	}
	if w1Pointers[0].X != 10 || w1Pointers[0].Y != 20 {
		t.Errorf("window1 pointer = (%f, %f), want (10, 20)", w1Pointers[0].X, w1Pointers[0].Y)
	}

	if len(w2Pointers) != 1 {
		t.Fatalf("window2 received %d pointer events, want 1", len(w2Pointers))
	}
	if w2Pointers[0].X != 30 || w2Pointers[0].Y != 40 {
		t.Errorf("window2 pointer = (%f, %f), want (30, 40)", w2Pointers[0].X, w2Pointers[0].Y)
	}
}

func TestDispatch_MultiWindow_ScrollIsolation(t *testing.T) {
	app := newTestApp()

	id1 := platform.NewWindowID()
	id2 := platform.NewWindowID()
	w1 := addTestWindow(app, id1)
	w2 := addTestWindow(app, id2)

	var w1Scrolls []gpucontext.ScrollEvent
	var w2Scrolls []gpucontext.ScrollEvent

	w1.SetOnScroll(func(ev gpucontext.ScrollEvent) {
		w1Scrolls = append(w1Scrolls, ev)
	})
	w2.SetOnScroll(func(ev gpucontext.ScrollEvent) {
		w2Scrolls = append(w2Scrolls, ev)
	})

	app.dispatchScrollEvent(&platform.Event{
		WindowID: id1,
		Type:     platform.EventScroll,
		Scroll:   gpucontext.ScrollEvent{DeltaX: 1, DeltaY: 2},
	})

	app.dispatchScrollEvent(&platform.Event{
		WindowID: id2,
		Type:     platform.EventScroll,
		Scroll:   gpucontext.ScrollEvent{DeltaX: 3, DeltaY: 4},
	})

	if len(w1Scrolls) != 1 {
		t.Fatalf("window1 received %d scroll events, want 1", len(w1Scrolls))
	}
	if w1Scrolls[0].DeltaY != 2 {
		t.Errorf("window1 scroll DeltaY = %f, want 2", w1Scrolls[0].DeltaY)
	}

	if len(w2Scrolls) != 1 {
		t.Fatalf("window2 received %d scroll events, want 1", len(w2Scrolls))
	}
	if w2Scrolls[0].DeltaY != 4 {
		t.Errorf("window2 scroll DeltaY = %f, want 4", w2Scrolls[0].DeltaY)
	}
}

func TestDispatch_MultiWindow_CharIsolation(t *testing.T) {
	app := newTestApp()

	id1 := platform.NewWindowID()
	id2 := platform.NewWindowID()
	w1 := addTestWindow(app, id1)
	w2 := addTestWindow(app, id2)

	var w1Texts []string
	var w2Texts []string

	w1.SetOnTextInput(func(text string) {
		w1Texts = append(w1Texts, text)
	})
	w2.SetOnTextInput(func(text string) {
		w2Texts = append(w2Texts, text)
	})

	app.dispatchCharEvent(&platform.Event{
		WindowID: id1,
		Type:     platform.EventChar,
		Char:     'A',
	})

	app.dispatchCharEvent(&platform.Event{
		WindowID: id2,
		Type:     platform.EventChar,
		Char:     'B',
	})

	if len(w1Texts) != 1 || w1Texts[0] != "A" {
		t.Errorf("window1 texts = %v, want [A]", w1Texts)
	}
	if len(w2Texts) != 1 || w2Texts[0] != "B" {
		t.Errorf("window2 texts = %v, want [B]", w2Texts)
	}
}

// --- Integration: full dispatch chain via classifyEvent ---

func TestClassifyEvent_KeyDownIntegration(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	windowCalled := false
	esCalled := false

	w.SetOnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) {
		windowCalled = true
	})
	app.eventSource.onKeyPress = func(gpucontext.Key, gpucontext.Modifiers) {
		esCalled = true
	}

	event := &platform.Event{
		WindowID: id,
		Type:     platform.EventKeyDown,
		Key:      gpucontext.KeyW,
		Mods:     gpucontext.ModControl,
	}
	app.classifyEvent(event, nil, nil)

	if !windowCalled {
		t.Error("per-window key press callback not called via classifyEvent")
	}
	if !esCalled {
		t.Error("EventSource key press callback not called via classifyEvent")
	}
	if !app.inputState.Keyboard().Pressed(input.KeyW) {
		t.Error("InputState should report KeyW pressed via classifyEvent")
	}
}

func TestClassifyEvent_ScrollIntegration(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	windowCalled := false
	w.SetOnScroll(func(ev gpucontext.ScrollEvent) {
		windowCalled = true
	})

	event := &platform.Event{
		WindowID: id,
		Type:     platform.EventScroll,
		Scroll: gpucontext.ScrollEvent{
			DeltaX: 1.0,
			DeltaY: -2.0,
		},
	}
	app.classifyEvent(event, nil, nil)

	if !windowCalled {
		t.Error("per-window scroll callback not called via classifyEvent")
	}

	// Mouse.Scroll() reads frame-latched values; UpdateFrame must be called first.
	app.inputState.Mouse().UpdateFrame()

	sx, sy := app.inputState.Mouse().Scroll()
	if sx != 1.0 || sy != -2.0 {
		t.Errorf("InputState scroll = (%f, %f), want (1.0, -2.0)", sx, sy)
	}
}

func TestClassifyEvent_PointerIntegration(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	windowCalled := false
	esCalled := false

	w.SetOnPointer(func(ev gpucontext.PointerEvent) {
		windowCalled = true
	})
	app.eventSource.onPointer = func(ev gpucontext.PointerEvent) {
		esCalled = true
	}

	event := &platform.Event{
		WindowID: id,
		Type:     platform.EventPointerMove,
		Pointer: gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerType: gpucontext.PointerTypeMouse,
			X:           55,
			Y:           66,
		},
	}
	app.classifyEvent(event, nil, nil)

	if !windowCalled {
		t.Error("per-window pointer callback not called via classifyEvent")
	}
	if !esCalled {
		t.Error("EventSource pointer callback not called via classifyEvent")
	}

	mx, my := app.inputState.Mouse().Position()
	if mx != 55 || my != 66 {
		t.Errorf("InputState mouse = (%f, %f), want (55, 66)", mx, my)
	}
}

func TestClassifyEvent_CharIntegration(t *testing.T) {
	app := newTestApp()
	id := platform.NewWindowID()
	w := addTestWindow(app, id)

	var received string
	w.SetOnTextInput(func(text string) {
		received = text
	})

	var esReceived string
	app.eventSource.onTextInput = func(text string) {
		esReceived = text
	}

	event := &platform.Event{
		WindowID: id,
		Type:     platform.EventChar,
		Char:     'Q',
	}
	app.classifyEvent(event, nil, nil)

	if received != "Q" {
		t.Errorf("window text = %q, want %q", received, "Q")
	}
	if esReceived != "Q" {
		t.Errorf("EventSource text = %q, want %q", esReceived, "Q")
	}
}
