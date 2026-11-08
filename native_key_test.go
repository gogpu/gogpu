package gogpu

import (
	"reflect"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

func TestNativeKeyCallbackKeepsDispatchOrderAndLegacyAPI(t *testing.T) {
	app := newTestApp()
	var order []string
	var received NativeKeyEvent
	if !app.OnNativeKey(func(event NativeKeyEvent) { order = append(order, "native"); received = event }) {
		t.Skip("native keyboard unavailable on this platform")
	}
	app.eventSource.OnFocus(func(bool) { order = append(order, "focus") })
	app.eventSource.OnTextInput(func(string) { order = append(order, "legacy") })
	native := platform.NativeKeyEvent{Kind: NativeTextCommit, Char: '@', VirtualKeyCode: 81,
		VirtualScanCode: 16, ControlKeyState: 9, RepeatCount: 2, TextInput: true}
	for _, event := range []platform.Event{
		{Type: platform.EventFocus, Focused: true},
		{Type: platform.EventChar, WindowID: 1, Char: '@', NativeKey: &native},
		{Type: platform.EventFocus, Focused: false},
	} {
		app.classifyEvent(&event, nil, nil)
	}
	if !reflect.DeepEqual(order, []string{"focus", "native", "legacy", "focus"}) {
		t.Fatalf("order = %v", order)
	}
	if received.VirtualKeyCode != 81 || received.ControlKeyState != 9 || received.RepeatCount != 2 || received.WindowID != 1 {
		t.Fatalf("native metadata = %+v", received)
	}
}
