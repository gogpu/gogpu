package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestMacUTF16RangeToUTF8(t *testing.T) {
	const text = "a😀é"
	tests := []struct {
		name               string
		location, length   uintptr
		wantStart, wantEnd int
		wantHidden, wantOK bool
	}{
		{name: "emoji", location: 1, length: 2, wantStart: 1, wantEnd: 5, wantOK: true},
		{name: "accent", location: 3, length: 1, wantStart: 5, wantEnd: 7, wantOK: true},
		{name: "hidden", location: ^uintptr(0), length: ^uintptr(0), wantStart: -1, wantEnd: -1, wantHidden: true, wantOK: true},
		{name: "out of range", location: 5, length: 1, wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, hidden, ok := macUTF16RangeToUTF8(text, tc.location, tc.length)
			if start != tc.wantStart || end != tc.wantEnd || hidden != tc.wantHidden || ok != tc.wantOK {
				t.Fatalf("range = (%d,%d,hidden=%v,ok=%v), want (%d,%d,hidden=%v,ok=%v)",
					start, end, hidden, ok, tc.wantStart, tc.wantEnd, tc.wantHidden, tc.wantOK)
			}
		})
	}
}

func TestMacIMEStateLifecycle(t *testing.T) {
	var state macIMEState
	if started, _ := state.setMarked("", 0, 0); started {
		t.Fatal("disabled state started composition")
	}
	state.enabled = true
	started, composition := state.setMarked("你😀", 1, 2)
	if !started || !composition.IsValid() {
		t.Fatalf("first composition = %+v, started=%v", composition, started)
	}
	if composition.CursorBegin != 3 || composition.CursorEnd != 7 ||
		composition.SelectionStart != 0 || composition.SelectionEnd != len("你😀") {
		t.Fatalf("composition ranges = %+v", composition)
	}
	if started, _ := state.setMarked("你好", ^uintptr(0), ^uintptr(0)); started {
		t.Fatal("second update restarted composition")
	}
	if !state.composition.IsValid() || state.composition.CursorBegin != -1 {
		t.Fatalf("hidden cursor update = %+v", state.composition)
	}
	if !state.insert("你好") {
		t.Fatal("insert did not close composition")
	}
	if state.marked || state.nativeNeedsUnmark {
		t.Fatalf("composition remained native-marked after insert: marked=%v reset=%v", state.marked, state.nativeNeedsUnmark)
	}
	if state.unmark() {
		t.Fatal("unmark reported an inactive composition")
	}
	state.setMarked("x", 1, 0)
	if !state.unmark() || !state.nativeNeedsUnmark {
		t.Fatal("external unmark did not request native reset")
	}
	state.setMarked("x", 1, 0)
	if !state.setEnabled(false) {
		t.Fatal("disabling active composition did not report cancellation")
	}
	if state.marked || state.composition != (gpucontext.IMEComposition{}) {
		t.Fatalf("disabled state retained composition: %+v", state.composition)
	}
}
