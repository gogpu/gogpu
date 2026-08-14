package gogpu

import (
	"reflect"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

type imeTestWindow struct {
	platform.PlatformWindow
	calls []string
}

func (w *imeTestWindow) SetIMEPosition(x, y int) {
	w.calls = append(w.calls, "position")
}

func (w *imeTestWindow) SetIMEEnabled(enabled bool) {
	if enabled {
		w.calls = append(w.calls, "enabled:true")
	} else {
		w.calls = append(w.calls, "enabled:false")
	}
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

func TestIMEEventSourceV2OrderingAndLegacyConversion(t *testing.T) {
	app := NewApp(DefaultConfig())
	es := app.EventSource()
	v2, ok := es.(gpucontext.IMEEventSourceV2)
	if !ok {
		t.Fatal("EventSource does not expose the optional v2 IME event contract")
	}

	var order []string
	es.OnIMECompositionStart(func() { order = append(order, "start") })
	es.OnIMECompositionUpdate(func(state gpucontext.IMEState) {
		order = append(order, "legacy-update:"+state.CompositionText)
		if state.CursorPos != 3 || state.CursorBegin != 3 || state.CursorEnd != 6 {
			t.Errorf("legacy range = (%d,%d,%d), want (3,3,6)", state.CursorPos, state.CursorBegin, state.CursorEnd)
		}
	})
	v2.OnIMECompositionUpdateV2(func(state gpucontext.IMEComposition) {
		order = append(order, "v2-update:"+state.CompositionText)
	})
	es.OnIMECompositionEnd(func(committed string) { order = append(order, "end:"+committed) })
	v2.OnIMECanceled(func() { order = append(order, "canceled") })
	v2.OnIMEDisabled(func() { order = append(order, "disabled") })
	v2.OnIMEDeleteSurrounding(func(event gpucontext.IMEDeleteSurroundingEvent) {
		order = append(order, "delete")
	})

	classify := func(event *platform.Event) {
		app.classifyEvent(event, nil, nil)
	}
	classify(&platform.Event{WindowID: 1, Type: platform.EventIMECompositionStart})
	composition := gpucontext.IMEComposition{
		CompositionText: "你好",
		CursorBegin:     3,
		CursorEnd:       6,
		SelectionStart:  0,
		SelectionEnd:    3,
	}
	classify(&platform.Event{
		WindowID:       1,
		Type:           platform.EventIMECompositionUpdate,
		IMEComposition: composition,
	})
	classify(&platform.Event{
		WindowID:     1,
		Type:         platform.EventIMECompositionEnd,
		IMECommitted: "你好",
	})
	classify(&platform.Event{WindowID: 1, Type: platform.EventIMECanceled})
	classify(&platform.Event{WindowID: 1, Type: platform.EventIMEDisabled})
	classify(&platform.Event{
		WindowID: 1,
		Type:     platform.EventIMEDeleteSurrounding,
		IMEDelete: gpucontext.IMEDeleteSurroundingEvent{
			Before: 3,
			After:  0,
		},
	})

	want := []string{
		"start",
		"v2-update:你好",
		"legacy-update:你好",
		"end:你好",
		"canceled",
		"disabled",
		"delete",
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("callback order = %#v, want %#v", order, want)
	}
}

func TestAppIMECapabilityDiscovery(t *testing.T) {
	app := NewApp(DefaultConfig())
	caps, ok := gpucontext.DiscoverIMECapabilities(app)
	if !ok || caps.Version != gpucontext.IMEContractVersion {
		t.Fatalf("capabilities = (%+v, %v), want version %d", caps, ok, gpucontext.IMEContractVersion)
	}
	if _, ok := gpucontext.DiscoverIMECapabilities(app.EventSource()); !ok {
		t.Fatal("EventSource capability discovery failed")
	}
}

func TestAppIMEControllerStateReplayAndPrivacyLifecycle(t *testing.T) {
	app := NewApp(DefaultConfig())
	active := &imeTestWindow{}
	app.platWindow = active

	app.SetIMEPosition(10, 20)
	app.SetIMEEnabled(true)
	app.SetIMECursorArea(gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16})
	app.SetIMEContentType(gpucontext.ContentPurposeName, gpucontext.ContentHintCompletion)
	app.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "hello", Cursor: 5, Anchor: 5})
	app.SetIMEEnabled(false)
	if got, want := active.calls, []string{"position", "enabled:true", "area", "content", "surrounding", "enabled:false"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("active controller calls = %#v, want %#v", got, want)
	}

	recreated := &imeTestWindow{}
	app.applyIMEControllerState(recreated)
	if got, want := recreated.calls, []string{"enabled:false", "position", "area", "content", "surrounding"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("replayed controller calls = %#v, want %#v", got, want)
	}
}
