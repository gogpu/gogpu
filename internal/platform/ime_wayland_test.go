//go:build linux

package platform

import (
	"testing"
	"time"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gogpu/internal/platform/wayland"
)

func TestWaylandIMEDoneDispatchesAtomicLifecycle(t *testing.T) {
	w := &waylandWindow{
		startTime:  time.Now(),
		events:     eventqueue.New[Event](eventqueue.DefaultCapacity),
		imeEnabled: true,
	}
	w.handleWaylandTextInputDone(wayland.TextInputUpdate{
		HasPreedit:   true,
		PreeditText:  "日本",
		CursorBegin:  0,
		CursorEnd:    6,
		HasDelete:    true,
		DeleteBefore: 3,
		DeleteAfter:  2,
		HasCommit:    true,
		CommitText:   "に",
	})

	var got []Event
	for {
		event, ok := w.events.Pop()
		if !ok {
			break
		}
		got = append(got, event)
	}
	if len(got) != 4 {
		t.Fatalf("got %d events, want start/update/delete/end: %+v", len(got), got)
	}
	if got[0].Type != EventIMECompositionStart || got[1].Type != EventIMECompositionUpdate ||
		got[2].Type != EventIMEDeleteSurrounding || got[3].Type != EventIMECompositionEnd {
		t.Fatalf("unexpected atomic event order: %+v", got)
	}
	if got[1].IMEComposition.CompositionText != "日本" || got[3].IMECommitted != "に" {
		t.Fatalf("unexpected text payloads: %+v", got)
	}
}
