//go:build darwin

package platform

import (
	"math"
	"testing"
	"unsafe"

	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gpucontext"
)

func newMacIMEWindow() *darwinWindow {
	return &darwinWindow{id: 7, events: eventqueue.New[Event](16), ime: macIMEState{enabled: true}}
}

func macIMEString(t *testing.T, value string) platformdarwin.ID {
	t.Helper()
	text := platformdarwin.NewNSString(value)
	if text == nil || text.ID() == 0 {
		t.Fatalf("NewNSString(%q) failed", value)
	}
	t.Cleanup(text.Release)
	return text.ID()
}

func popMacIMEEvents(w *darwinWindow) []Event {
	var events []Event
	for {
		event, ok := w.events.Pop()
		if !ok {
			return events
		}
		events = append(events, event)
	}
}

func TestMacIMECompositionCommitAndCancelEvents(t *testing.T) {
	w := newMacIMEWindow()
	w.handleMacSetMarkedText(macIMEString(t, "你😀"), 1, 2, 0, 0)
	w.handleMacSetMarkedText(macIMEString(t, "你好"), 2, 0, 0, 0)
	got := popMacIMEEvents(w)
	if len(got) != 3 || got[0].Type != EventIMECompositionStart ||
		got[1].Type != EventIMECompositionUpdate || got[2].Type != EventIMECompositionUpdate {
		t.Fatalf("composition events = %#v, want start and two updates", got)
	}
	if got[1].IME == nil || got[1].IME.State.CompositionText != "你😀" ||
		got[1].IME.State.Cursor != (gpucontext.IMETextRange{Start: 3, End: 7}) {
		t.Fatalf("first composition update = %+v", got[1].IME)
	}

	w.handleMacInsertText(macIMEString(t, "你好"), ^uintptr(0), ^uintptr(0))
	got = popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMECompositionEnd || got[0].IME == nil || got[0].IME.Committed != "你好" {
		t.Fatalf("commit events = %#v, want one composition end with committed text", got)
	}

	w.handleMacSetMarkedText(macIMEString(t, "かな"), 2, 0, 0, 0)
	_ = popMacIMEEvents(w)
	w.handleMacUnmarkText()
	got = popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMECanceled {
		t.Fatalf("unmark events = %#v, want one cancellation", got)
	}
}

func TestMacIMEPrivacyAndOrdinaryTextDispatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		purpose gpucontext.ContentPurpose
		hints   gpucontext.ContentHint
	}{
		{name: "password purpose", purpose: gpucontext.ContentPurposePassword},
		{name: "hidden hint", hints: gpucontext.ContentHintHiddenText},
		{name: "sensitive hint", hints: gpucontext.ContentHintSensitiveData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newMacIMEWindow()
			w.ime.purpose, w.ime.hints = tc.purpose, tc.hints
			w.ime.surrounding = gpucontext.IMESurroundingText{Text: "secret", Cursor: 6, Anchor: 6}
			w.handleMacSetMarkedText(macIMEString(t, "secret"), 6, 0, 0, 0)
			w.handleMacInsertText(macIMEString(t, "secret"), 0, 0)
			if events := popMacIMEEvents(w); len(events) != 0 {
				t.Fatalf("sensitive input emitted events: %#v", events)
			}
		})
	}

	w := newMacIMEWindow()
	w.handleMacInsertText(macIMEString(t, "a\n😀\x01"), 0, 0)
	got := popMacIMEEvents(w)
	if len(got) != 2 || got[0].Type != EventChar || got[0].Char != 'a' || got[1].Type != EventChar || got[1].Char != '😀' {
		t.Fatalf("ordinary insert events = %#v, want printable runes only", got)
	}
}

func TestMacIMESurroundingSubstringUsesUTF16RangesAndHonorsPrivacy(t *testing.T) {
	w := newMacIMEWindow()
	w.ime.surrounding = gpucontext.IMESurroundingText{Text: "a😀é", Cursor: 7, Anchor: 7}
	var actual struct{ location, length uintptr }
	result := w.handleMacAttributedSubstring(1, 2, uintptr(unsafe.Pointer(&actual)))
	if got := platformdarwin.ObjectString(result); got != "😀" {
		t.Fatalf("substring = %q, want emoji", got)
	}
	if actual != (struct{ location, length uintptr }{1, 2}) {
		t.Fatalf("actual range = %+v, want UTF-16 range {1, 2}", actual)
	}

	result = w.handleMacAttributedSubstring(20, 1, uintptr(unsafe.Pointer(&actual)))
	if got := platformdarwin.ObjectString(result); got != "" || actual != (struct{ location, length uintptr }{}) {
		t.Fatalf("out-of-range result = %q, actual=%+v, want empty range", got, actual)
	}

	w.ime.purpose = gpucontext.ContentPurposePassword
	result = w.handleMacAttributedSubstring(0, 1, uintptr(unsafe.Pointer(&actual)))
	if got := platformdarwin.ObjectString(result); got != "" {
		t.Fatalf("password substring = %q, want empty", got)
	}
}

func TestMacIMEValidCursorArea(t *testing.T) {
	for _, tc := range []struct {
		name string
		area gpucontext.IMECursorArea
		want bool
	}{
		{name: "zero", want: true},
		{name: "valid", area: gpucontext.IMECursorArea{X: 1, Y: 2, Width: 3, Height: 4}, want: true},
		{name: "negative", area: gpucontext.IMECursorArea{X: -1}, want: false},
		{name: "nan", area: gpucontext.IMECursorArea{Y: math.NaN()}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validIMECursorArea(tc.area); got != tc.want {
				t.Fatalf("validIMECursorArea(%+v) = %v, want %v", tc.area, got, tc.want)
			}
		})
	}
}

func TestMacIMEControllerEnablePrivacyAndSurroundingText(t *testing.T) {
	w := newMacIMEWindow()
	w.ime.enabled = false
	if w.imeEnabled() {
		t.Fatal("IME must be disabled by default")
	}
	w.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "ignored", Cursor: 7, Anchor: 7})
	if w.ime.surrounding.Text != "" {
		t.Fatal("surrounding text was retained while IME was disabled")
	}

	// Enabling requires a real NSTextInputClient view; set the state directly
	// here to exercise controller transitions without constructing an NSWindow.
	w.ime.enabled = true
	w.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "hello", Cursor: 5, Anchor: 5})
	if w.ime.surrounding.Text != "hello" {
		t.Fatalf("surrounding text = %+v", w.ime.surrounding)
	}
	w.ime.setMarked("preedit", 7, 0)
	w.SetIMEEnabled(false)
	if w.imeEnabled() || w.ime.surrounding.Text != "" || w.ime.marked {
		t.Fatalf("disable retained IME state: %+v", w.ime)
	}
	got := popMacIMEEvents(w)
	if len(got) != 2 || got[0].Type != EventIMECanceled || got[1].Type != EventIMEDisabled {
		t.Fatalf("disable events = %#v, want canceled then disabled", got)
	}

	w.ime.enabled = true
	w.SetIMEContentType(gpucontext.ContentPurposePassword, gpucontext.ContentHintNone)
	if w.imeEnabled() {
		t.Fatal("password purpose did not disable IME")
	}
	got = popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMEDisabled {
		t.Fatalf("password transition events = %#v, want disabled", got)
	}
	w.SetIMEContentType(gpucontext.ContentPurposeNormal, gpucontext.ContentHintSensitiveData)
	w.ime.enabled = true
	w.SetIMEContentType(gpucontext.ContentPurposeNormal, gpucontext.ContentHintSensitiveData)
	if w.imeEnabled() {
		t.Fatal("sensitive hint did not prevent IME from enabling")
	}
}

func TestMacIMECursorAreaAndCancelController(t *testing.T) {
	w := newMacIMEWindow()
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16})
	if w.ime.area != (gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16}) {
		t.Fatalf("cursor area = %+v", w.ime.area)
	}
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: -1})
	if w.ime.area.X != 10 {
		t.Fatalf("invalid cursor area replaced valid value: %+v", w.ime.area)
	}
	w.ime.setMarked("preedit", 7, 0)
	w.CancelIME()
	if w.ime.marked || !w.ime.nativeNeedsUnmark {
		t.Fatalf("CancelIME did not clear state and defer native unmark: %+v", w.ime)
	}
	got := popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMECanceled {
		t.Fatalf("CancelIME events = %#v, want one cancellation", got)
	}
	w.flushNativeIMEReset(0)
	if w.ime.nativeNeedsUnmark {
		t.Fatal("flushNativeIMEReset did not consume pending reset")
	}
}
