package platform

import (
	"math"
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestBrowserIMECompositionUsesUTF8Ranges(t *testing.T) {
	composition := browserIMEComposition("拼音")
	if !composition.IsValid() {
		t.Fatal("composition with multibyte text is invalid")
	}
	if composition.CursorBegin != len("拼音") || composition.CursorEnd != len("拼音") {
		t.Fatalf("cursor = [%d,%d], want [%d,%d]", composition.CursorBegin, composition.CursorEnd, len("拼音"), len("拼音"))
	}
	if composition.SelectionStart != 0 || composition.SelectionEnd != len("拼音") {
		t.Fatalf("selection = [%d,%d], want [0,%d]", composition.SelectionStart, composition.SelectionEnd, len("拼音"))
	}
}

func TestBrowserIMEUTF8UTF16Offsets(t *testing.T) {
	text := "a😀中"
	for _, offset := range []int{0, 1, 5, 8} {
		units := utf8OffsetToUTF16(text, offset)
		if got := utf16OffsetToUTF8(text, units); got != offset {
			t.Fatalf("offset %d -> UTF-16 %d -> %d", offset, units, got)
		}
	}
}

func TestBrowserIMEDeleteLengthsAreUTF8Bytes(t *testing.T) {
	text := "a😀中"
	if got := previousRuneBytes(text, len(text)); got != len("中") {
		t.Fatalf("previous rune bytes = %d, want %d", got, len("中"))
	}
	if got := nextRuneBytes(text, len("a")); got != len("😀") {
		t.Fatalf("next rune bytes = %d, want %d", got, len("😀"))
	}
	if got := previousWordStart("hello world", len("hello world")); got != len("hello ") {
		t.Fatalf("previous word start = %d, want %d", got, len("hello "))
	}
	if got := nextWordEnd("hello world", 0); got != len("hello") {
		t.Fatalf("next word end = %d, want 5", got)
	}
}

func TestBrowserIMEContentMappingAndAreaValidation(t *testing.T) {
	if got := browserInputMode(gpucontext.ContentPurposeEmail); got != "email" {
		t.Fatalf("email input mode = %q", got)
	}
	if got := browserAutoCapitalize(gpucontext.ContentHintUppercase); got != "characters" {
		t.Fatalf("uppercase autocapitalize = %q", got)
	}
	if !validBrowserIMEArea(gpucontext.IMECursorArea{X: 1, Y: 2, Width: 3, Height: 4}) {
		t.Fatal("valid cursor area rejected")
	}
	if validBrowserIMEArea(gpucontext.IMECursorArea{X: math.NaN()}) {
		t.Fatal("NaN cursor area accepted")
	}
}
