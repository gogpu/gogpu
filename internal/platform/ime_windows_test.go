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
	if !ok || event.Type != EventIMECompositionEnd || event.IMECommitted != "你" {
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
	if w.ime.enabled || w.imeSurrounding.IsValid() {
		t.Fatalf("sensitive state retained: enabled=%v surrounding=%+v", w.ime.enabled, w.imeSurrounding)
	}
}
