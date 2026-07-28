//go:build linux

package x11

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gpucontext"
)

func TestShouldPreferServerKeymap(t *testing.T) {
	tests := []struct {
		name     string
		display  string
		vendor   string
		xwayland bool
		want     bool
	}{
		{name: "local Xorg", display: ":0", vendor: "The X.Org Foundation"},
		{name: "explicit local unix", display: "unix:0", vendor: "The X.Org Foundation"},
		{name: "SSH forwarding", display: "localhost:10.0", vendor: "The X.Org Foundation", want: true},
		{name: "remote TCP", display: "workstation.example:0", vendor: "The X.Org Foundation", want: true},
		{name: "XQuartz vendor", display: ":0", vendor: "The XQuartz Project", want: true},
		{name: "legacy Apple vendor", display: ":0", vendor: "Apple Computer, Inc.", want: true},
		{name: "XWayland", display: ":1", vendor: "The X.Org Foundation", xwayland: true, want: true},
		{name: "invalid display", display: "not-a-display", vendor: "The X.Org Foundation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPreferServerKeymap(tt.display, tt.vendor, tt.xwayland); got != tt.want {
				t.Fatalf("shouldPreferServerKeymap(%q, %q, %v) = %v, want %v",
					tt.display, tt.vendor, tt.xwayland, got, tt.want)
			}
		})
	}
}

func TestDispatchServerKeymapText(t *testing.T) {
	keymap := &KeyboardMapping{
		MinKeycode:     38,
		MaxKeycode:     38,
		KeysymsPerCode: 2,
		Keysyms:        []Keysym{'a', 'A'},
	}
	w := &x11Window{events: eventqueue.New[PlatformEvent](eventqueue.DefaultCapacity)}

	if !dispatchServerKeymapText(w, keymap, 38, gpucontext.ModShift, 0) {
		t.Fatal("dispatchServerKeymapText did not dispatch a printable keysym")
	}
	event, ok := w.events.Pop()
	if !ok {
		t.Fatal("character event was not queued")
	}
	if event.Type != EventTypeChar || event.Char != 'A' {
		t.Fatalf("event = {Type: %v, Char: %q}, want EventTypeChar 'A'", event.Type, event.Char)
	}
}

func TestDispatchServerKeymapTextRejectsNonPrintable(t *testing.T) {
	keymap := &KeyboardMapping{
		MinKeycode:     67,
		MaxKeycode:     67,
		KeysymsPerCode: 1,
		Keysyms:        []Keysym{KeysymF1},
	}
	w := &x11Window{events: eventqueue.New[PlatformEvent](eventqueue.DefaultCapacity)}

	if dispatchServerKeymapText(w, keymap, 67, 0, 0) {
		t.Fatal("dispatchServerKeymapText dispatched a non-printable keysym")
	}
	if _, ok := w.events.Pop(); ok {
		t.Fatal("non-printable keysym queued an event")
	}
}
