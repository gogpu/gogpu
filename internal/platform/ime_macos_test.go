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
		{name: "emoji surrogate pair", location: 1, length: 2, wantStart: 1, wantEnd: 5, wantOK: true},
		{name: "multibyte BMP rune", location: 3, length: 1, wantStart: 5, wantEnd: 7, wantOK: true},
		{name: "end of text", location: 4, wantStart: 7, wantEnd: 7, wantOK: true},
		{name: "hidden cursor", location: ^uintptr(0), length: ^uintptr(0), wantStart: -1, wantEnd: -1, wantHidden: true, wantOK: true},
		{name: "range beyond text", location: 5, length: 1},
		{name: "range overflows text", location: 3, length: 2},
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
	if _, _, hidden, ok := macUTF16RangeToUTF8("bad\xff", 0, 0); hidden || ok {
		t.Fatal("invalid UTF-8 text was accepted")
	}
}

func TestMacIMEStateLifecycle(t *testing.T) {
	var state macIMEState
	if started, _ := state.setMarked("ni", 2, 0); started {
		t.Fatal("disabled state started a composition")
	}
	state.enabled = true
	started, first := state.setMarked("你😀", 1, 2)
	if !started || !first.IsValid() || !first.Composing {
		t.Fatalf("first update = %+v, started=%v", first, started)
	}
	if first.Cursor != (gpucontext.IMETextRange{Start: 3, End: 7}) {
		t.Fatalf("cursor = %+v, want UTF-8 byte range [3,7)", first.Cursor)
	}
	started, hidden := state.setMarked("你😀", ^uintptr(0), ^uintptr(0))
	if started || hidden.Cursor != (gpucontext.IMETextRange{Start: -1, End: -1}) {
		t.Fatalf("hidden cursor update = %+v, started=%v", hidden, started)
	}
	if !state.insert("你好") || state.marked || state.composition.Composing {
		t.Fatalf("insert did not close composition: %+v", state)
	}
	if state.unmark() {
		t.Fatal("unmark reported an inactive composition")
	}
	if started, _ := state.setMarked("x", 1, 0); !started {
		t.Fatal("new marked text did not start a composition")
	}
	if !state.unmark() || !state.nativeNeedsUnmark {
		t.Fatal("external unmark did not request native reset")
	}
	if state.setEnabled(false) || !state.nativeNeedsUnmark {
		t.Fatal("disabling after cancellation lost the pending native reset")
	}
	if state.marked || state.composition.Composing {
		t.Fatalf("disabled state retained text input state: %+v", state)
	}
}

func TestMacIMEKeyInterpretation(t *testing.T) {
	tests := []struct {
		name             string
		marked           bool
		callbacks        string
		consumed, cancel bool
	}{
		{"ordinary key", false, "", false, false},
		{"preedit updated", false, "m", true, false},
		{"active preedit consumes", true, "", true, false},
		{"commit then command", true, "ic", true, false},
		{"commit then repeated command", true, "icc", true, false},
		{"unhandled command while marked", true, "c", false, true},
		{"updated preedit then command", false, "mc", false, true},
		{"ordinary command", false, "c", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := macIMEState{enabled: true, marked: tc.marked}
			var key macIMEKeyState
			key.begin(state.marked)
			canceled := false
			for _, callback := range tc.callbacks {
				switch callback {
				case 'm':
					state.setMarked("preedit", 7, 0)
					key.mark()
				case 'i':
					if state.insert("한") {
						key.commit()
					}
				case 'c':
					if key.command() {
						canceled = state.unmark() || canceled
					}
				}
			}
			if key.consumed != tc.consumed || canceled != tc.cancel || (tc.cancel && state.marked) {
				t.Fatalf("consumed=%v canceled=%v marked=%v; want consumed=%v canceled=%v", key.consumed, canceled, state.marked, tc.consumed, tc.cancel)
			}
		})
	}
}

func TestMacIMEControlInsert(t *testing.T) {
	for _, tc := range []struct {
		text    string
		control bool
	}{
		{"\n", true}, {"\x7f", true}, {"\u0085", true}, {"한", false}, {"", false}, {"a\n", false},
	} {
		if got := macIMEControlInsert(tc.text); got != tc.control {
			t.Errorf("macIMEControlInsert(%q) = %v, want %v", tc.text, got, tc.control)
		}
	}
}

func TestMacIMEStateRejectsInvalidMarkedText(t *testing.T) {
	state := macIMEState{enabled: true}
	if started, _ := state.setMarked("bad\xff", 0, 0); started {
		t.Fatal("invalid UTF-8 marked text started a composition")
	}
	if started, _ := state.setMarked("x", 2, 1); started {
		t.Fatal("out-of-range selected range started a composition")
	}
}
