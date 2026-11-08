package gogpu

import (
	"reflect"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// imeTestWindow records controller calls. It embeds the PlatformWindow
// interface so every method the test does not care about stays unimplemented.
type imeTestWindow struct {
	platform.PlatformWindow
	calls []string
	caps  gpucontext.IMECapabilities
}

func (w *imeTestWindow) SetIMEEnabled(enabled bool) {
	if enabled {
		w.calls = append(w.calls, "enabled:true")
		return
	}
	w.calls = append(w.calls, "enabled:false")
}

func (w *imeTestWindow) SetIMECursorArea(gpucontext.IMECursorArea) {
	w.calls = append(w.calls, "area")
}

func (w *imeTestWindow) SetIMEContentType(gpucontext.ContentPurpose, gpucontext.ContentHint) {
	w.calls = append(w.calls, "content")
}

func (w *imeTestWindow) SetIMESurroundingText(gpucontext.IMESurroundingText) {
	w.calls = append(w.calls, "surrounding")
}

func (w *imeTestWindow) CancelIME() {
	w.calls = append(w.calls, "cancel")
}

func (w *imeTestWindow) IMECapabilities() gpucontext.IMECapabilities {
	return w.caps
}

// bareIMEWindow exposes no controller, like the platforms whose backend has
// not landed yet.
type bareIMEWindow struct {
	platform.PlatformWindow
}

func TestEventSourceCarriesIMEEventContract(t *testing.T) {
	app := NewApp(DefaultConfig())
	es := app.EventSource()
	events, ok := es.(gpucontext.IMEEventSource)
	if !ok {
		t.Fatal("EventSource does not implement gpucontext.IMEEventSource")
	}

	var order []string
	es.OnIMECompositionStart(func() { order = append(order, "start") })
	es.OnIMECompositionUpdate(func(state gpucontext.IMEState) {
		order = append(order, "update:"+state.CompositionText)
		if !state.Composing || state.Cursor != (gpucontext.IMETextRange{Start: 6, End: 6}) {
			t.Errorf("state = %+v, want composing with the reported cursor", state)
		}
	})
	es.OnIMECompositionEnd(func(committed string) { order = append(order, "end:"+committed) })
	events.OnIMECanceled(func() { order = append(order, "canceled") })
	events.OnIMEDisabled(func() { order = append(order, "disabled") })
	events.OnIMEDeleteSurrounding(func(event gpucontext.IMEDeleteSurroundingEvent) {
		order = append(order, "delete")
	})

	classify := func(event *platform.Event) { app.classifyEvent(event, nil, nil) }
	classify(&platform.Event{WindowID: 1, Type: platform.EventIMECompositionStart})
	classify(&platform.Event{
		WindowID: 1,
		Type:     platform.EventIMECompositionUpdate,
		IME: &platform.IMEEvent{State: gpucontext.IMEState{
			Composing:       true,
			CompositionText: "你好",
			Cursor:          gpucontext.IMETextRange{Start: 6, End: 6},
			Selection:       gpucontext.IMETextRange{Start: 0, End: 3},
		}},
	})
	classify(&platform.Event{
		WindowID: 1,
		Type:     platform.EventIMECompositionEnd,
		IME:      &platform.IMEEvent{Committed: "你好"},
	})
	classify(&platform.Event{WindowID: 1, Type: platform.EventIMECanceled})
	classify(&platform.Event{WindowID: 1, Type: platform.EventIMEDisabled})
	classify(&platform.Event{
		WindowID: 1,
		Type:     platform.EventIMEDeleteSurrounding,
		IME:      &platform.IMEEvent{Delete: gpucontext.IMEDeleteSurroundingEvent{Before: 3}},
	})

	want := []string{"start", "update:你好", "end:你好", "canceled", "disabled", "delete"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("callback order = %#v, want %#v", order, want)
	}
}

func TestAppIMEControllerForwardsAndReplaysConfiguration(t *testing.T) {
	app := NewApp(DefaultConfig())
	active := &imeTestWindow{}
	app.platWindow = active

	app.SetIMEEnabled(true)
	app.SetIMECursorArea(gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16})
	app.SetIMEContentType(gpucontext.ContentPurposeName, gpucontext.ContentHintCompletion)
	app.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "hello", Cursor: 5, Anchor: 5})
	app.CancelIME()
	app.SetIMEEnabled(false)

	want := []string{"enabled:true", "area", "content", "surrounding", "cancel", "enabled:false"}
	if !reflect.DeepEqual(active.calls, want) {
		t.Fatalf("controller calls = %#v, want %#v", active.calls, want)
	}

	// A window created later receives the configuration made before Run.
	recreated := &imeTestWindow{}
	app.applyIMEControllerState(recreated)
	wantReplay := []string{"enabled:false", "area", "content", "surrounding"}
	if !reflect.DeepEqual(recreated.calls, wantReplay) {
		t.Fatalf("replayed calls = %#v, want %#v", recreated.calls, wantReplay)
	}
}

func TestAppIMEWithoutController(t *testing.T) {
	var nilApp *App
	nilApp.SetIMEEnabled(true)
	nilApp.SetIMECursorArea(gpucontext.IMECursorArea{X: 1, Y: 2})
	nilApp.SetIMEContentType(gpucontext.ContentPurposeName, gpucontext.ContentHintNone)
	nilApp.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "x", Cursor: 1, Anchor: 1})
	nilApp.CancelIME()
	nilApp.applyIMEControllerState(nil)
	if got := nilApp.IMECapabilities(); got != platform.DefaultIMECapabilities() {
		t.Fatalf("nil app capabilities = %v, want the platform default", got)
	}

	app := NewApp(DefaultConfig())
	app.platWindow = &bareIMEWindow{}
	app.SetIMEEnabled(true)
	app.SetIMECursorArea(gpucontext.IMECursorArea{X: 1, Y: 2})
	app.SetIMEContentType(gpucontext.ContentPurposeName, gpucontext.ContentHintNone)
	app.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "x", Cursor: 1, Anchor: 1})
	app.CancelIME()
	app.applyIMEControllerState(&bareIMEWindow{})
	if got := app.IMECapabilities(); got != platform.DefaultIMECapabilities() {
		t.Fatalf("capabilities = %v, want the platform default", got)
	}
}

func TestAppIMERejectsInvalidSurroundingText(t *testing.T) {
	app := NewApp(DefaultConfig())
	active := &imeTestWindow{}
	app.platWindow = active

	// An out-of-range cursor is rejected before it reaches the platform. An
	// empty IMESurroundingText stays valid: it clears the surrounding text.
	app.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "ab", Cursor: 3, Anchor: 3})
	if len(active.calls) != 0 {
		t.Fatalf("invalid surrounding text was forwarded: %#v", active.calls)
	}
}

func TestAppIMECapabilitiesReportsWindowBackend(t *testing.T) {
	app := NewApp(DefaultConfig())
	app.platWindow = &imeTestWindow{caps: gpucontext.IMECapabilityCursorArea | gpucontext.IMECapabilityContentType}
	if got, want := app.IMECapabilities(), gpucontext.IMECapabilityCursorArea|gpucontext.IMECapabilityContentType; got != want {
		t.Fatalf("capabilities = %v, want %v", got, want)
	}
}
