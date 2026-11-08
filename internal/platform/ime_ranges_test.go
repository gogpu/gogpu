package platform

import "testing"

func TestUTF16IndexToUTF8Offset(t *testing.T) {
	const text = "A😀你"
	tests := []struct {
		index int
		want  int
	}{
		{index: -1, want: 0},
		{index: 0, want: 0},
		{index: 1, want: 1},
		{index: 2, want: 1}, // inside 😀's surrogate pair
		{index: 3, want: 5},
		{index: 4, want: 8},
		{index: 5, want: 8},
		{index: 6, want: len(text)},
	}
	for _, test := range tests {
		if got := utf16IndexToUTF8Offset(text, test.index); got != test.want {
			t.Errorf("index %d = %d, want %d", test.index, got, test.want)
		}
	}
}

func TestIMESelectionRangeConvertsUTF16Attributes(t *testing.T) {
	const text = "a😀你"
	if start, end := imeSelectionRange(text, []byte{0, imeAttrTargetConverted, imeAttrTargetConverted, 0}); start != 1 || end != 5 {
		t.Fatalf("emoji target = (%d, %d), want (1, 5)", start, end)
	}
	if start, end := imeSelectionRange(text, []byte{0, imeAttrTargetNotConverted, imeAttrTargetNotConverted, imeAttrTargetConverted}); start != 1 || end != len(text) {
		t.Fatalf("trailing target = (%d, %d), want (1, %d)", start, end, len(text))
	}
	if start, end := imeSelectionRange(text, []byte{0, 0, 0, 0}); start != 0 || end != 0 {
		t.Fatalf("no target = (%d, %d), want (0, 0)", start, end)
	}
	if start, end := imeSelectionRange("bad\xff", []byte{1, 1, 1, 1}); start != 0 || end != 0 {
		t.Fatalf("invalid UTF-8 target = (%d, %d), want (0, 0)", start, end)
	}
	if start, end := imeSelectionRange(text, []byte{imeAttrTargetConverted}); start != 0 || end != 1 {
		t.Fatalf("short target attributes = (%d, %d), want (0, 1)", start, end)
	}
}
