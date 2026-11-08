//go:build windows

package platform

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gpucontext"
)

func TestWin32EmptyPostEndResultIsCancellation(t *testing.T) {
	p := &windowsPlatform{events: eventqueue.New[Event](eventqueue.DefaultCapacity)}
	w := &win32Window{platform: p}
	w.ime.setEnabled(true)
	w.ime.start()
	if _, ok := w.ime.end(); !ok {
		t.Fatal("composition did not end")
	}
	w.ime.beginCharResult()
	w.finishIMECharResult()

	event, ok := p.events.Pop()
	if !ok {
		t.Fatal("empty post-END result did not produce a terminal event")
	}
	if event.Type != EventIMECanceled {
		t.Fatalf("empty post-END result event = %v, want cancellation", event.Type)
	}
}

func TestWin32PostEndCommitRemainsExactlyOnce(t *testing.T) {
	p := &windowsPlatform{events: eventqueue.New[Event](eventqueue.DefaultCapacity)}
	w := &win32Window{platform: p}
	w.ime.setEnabled(true)
	w.ime.start()
	if _, ok := w.ime.end(); !ok {
		t.Fatal("composition did not end")
	}
	w.ime.beginCharResult()
	if !w.ime.consumeCharResultUnits([]uint16{'你'}) {
		t.Fatal("post-END result was not collected")
	}
	w.finishIMECharResult()

	event, ok := p.events.Pop()
	if !ok || event.Type != EventIMECompositionEnd || event.IME == nil || event.IME.Committed != "你" {
		t.Fatalf("post-END result event = %+v (ok=%v), want one committed end", event, ok)
	}
	if _, ok := p.events.Pop(); ok {
		t.Fatal("post-END result produced duplicate terminal event")
	}
}

func TestWin32SensitiveContentDropsSurroundingText(t *testing.T) {
	w := &win32Window{}
	w.ime.setEnabled(true)
	w.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "context", Cursor: 7, Anchor: 7})
	w.imeMu.Lock()
	if !w.imeSurrounding.IsValid() {
		w.imeMu.Unlock()
		t.Fatal("normal surrounding text was not retained")
	}
	w.imeMu.Unlock()

	w.SetIMEContentType(gpucontext.ContentPurposePassword, gpucontext.ContentHintNone)
	w.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: "secret", Cursor: 6, Anchor: 6})
	w.imeMu.Lock()
	defer w.imeMu.Unlock()
	if w.ime.enabled || w.imeSurrounding.Text != "" {
		t.Fatalf("sensitive state retained: enabled=%v surrounding=%+v", w.ime.enabled, w.imeSurrounding)
	}
}

func TestWin32SensitiveContentStaysDisabled(t *testing.T) {
	w := &win32Window{}
	w.SetIMEContentType(gpucontext.ContentPurposePassword, gpucontext.ContentHintNone)
	w.SetIMEEnabled(true)
	w.imeMu.Lock()
	defer w.imeMu.Unlock()
	if w.ime.enabled {
		t.Fatal("IME enabled for password content")
	}
}

func TestWin32CursorAreaValidation(t *testing.T) {
	valid := gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16}
	if !imeAreaIsValid(valid) {
		t.Fatal("a plain caret rectangle was rejected")
	}
	if imeAreaIsValid(gpucontext.IMECursorArea{X: -1}) {
		t.Fatal("a negative origin was accepted")
	}
	if imeAreaIsValid(gpucontext.IMECursorArea{Width: -1}) {
		t.Fatal("a negative width was accepted")
	}
}

// TestWin32LateResultDeliveredOnce covers the Hangul order: END delivers
// nothing and arms the post-END path, the result arrives in a later
// COMPOSITION, and it must reach consumers exactly once with its WM_CHAR echo
// swallowed.
func TestWin32LateResultDeliveredOnce(t *testing.T) {
	p := &windowsPlatform{events: eventqueue.New[Event](eventqueue.DefaultCapacity)}
	w := &win32Window{platform: p}
	w.ime.setEnabled(true)
	w.ime.start()
	if _, ok := w.ime.end(); !ok {
		t.Fatal("composition did not end")
	}
	w.ime.beginCharResult()
	w.processIMEResult("한")

	event, ok := p.events.Pop()
	if !ok || event.Type != EventIMECompositionEnd || event.IME == nil || event.IME.Committed != "한" {
		t.Fatalf("late result event = %+v (ok=%v), want one committed end", event, ok)
	}
	if _, ok := p.events.Pop(); ok {
		t.Fatal("late result produced a duplicate terminal event")
	}
	if !w.ime.consumeCharUnit('한') {
		t.Fatal("WM_CHAR echo of the late result was not suppressed")
	}
	if w.ime.consumeCharUnit('한') {
		t.Fatal("echo suppression survived its single use")
	}
}

// TestWin32ResultWhileDisabledIsDropped: the native detach is posted, so a
// result queued after SetIMEEnabled(false) can still arrive; it must not be
// committed while input is disabled.
func TestWin32ResultWhileDisabledIsDropped(t *testing.T) {
	p := &windowsPlatform{events: eventqueue.New[Event](eventqueue.DefaultCapacity)}
	w := &win32Window{platform: p}
	w.ime.setEnabled(true)
	w.ime.start()
	w.ime.setEnabled(false)
	w.processIMEResult("한")
	if event, ok := p.events.Pop(); ok {
		t.Fatalf("result committed while IME was disabled: %+v", event)
	}
}

// TestWin32IMECompositionRoute locks in how a composition message splits
// between the result path and the preedit path. The both-flags-after-END case
// is the one that used to deliver the committed text twice.
func TestWin32IMECompositionRoute(t *testing.T) {
	both := uintptr(gcsResultStr | gcsCompStr)
	cases := []struct {
		name      string
		flags     uintptr
		armed     bool
		handle    bool
		inSession bool
	}{
		{"preedit only", uintptr(gcsCompStr), false, false, false},
		{"result only", uintptr(gcsResultStr), false, true, false},
		{"result only after end", uintptr(gcsResultStr), true, true, false},
		{"result and preedit in session", both, false, false, true},
		{"result and preedit after end", both, true, true, false},
		{"no flags", 0, true, false, false},
	}
	for _, tc := range cases {
		handle, inSession := imeCompositionRoute(tc.flags, tc.armed)
		if handle != tc.handle || inSession != tc.inSession {
			t.Errorf("%s: route = (%v, %v), want (%v, %v)", tc.name, handle, inSession, tc.handle, tc.inSession)
		}
	}
}

// TestWin32LateResultEchoIsSwallowed covers the other Hangul order: END reads
// the result itself, and the late COMPOSITION only repeats it.
func TestWin32LateResultEchoIsSwallowed(t *testing.T) {
	p := &windowsPlatform{events: eventqueue.New[Event](eventqueue.DefaultCapacity)}
	w := &win32Window{platform: p}
	w.ime.setEnabled(true)
	w.ime.start()
	w.ime.addResult("한")
	if committed, ok := w.ime.end(); !ok || committed != "한" {
		t.Fatalf("end = (%q, %v), want 한", committed, ok)
	}
	w.processIMEResult("한")
	if event, ok := p.events.Pop(); ok {
		t.Fatalf("late echo was delivered as a commit: %+v", event)
	}
}
