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
	for _, test := range []struct {
		purpose gpucontext.ContentPurpose
		want    string
	}{
		{gpucontext.ContentPurposeDigits, browserInputModeNumeric},
		{gpucontext.ContentPurposePin, browserInputModeNumeric},
		{gpucontext.ContentPurposeNumber, "decimal"},
		{gpucontext.ContentPurposePhone, "tel"},
		{gpucontext.ContentPurposeURL, "url"},
		{gpucontext.ContentPurposeEmail, "email"},
		{gpucontext.ContentPurposeDate, browserInputModeNumeric},
		{gpucontext.ContentPurposeTime, browserInputModeNumeric},
		{gpucontext.ContentPurposeDateTime, "datetime"},
		{gpucontext.ContentPurposeName, "text"},
	} {
		if got := browserInputMode(test.purpose); got != test.want {
			t.Errorf("purpose %v input mode = %q, want %q", test.purpose, got, test.want)
		}
	}
	for _, test := range []struct {
		hints gpucontext.ContentHint
		want  string
	}{
		{gpucontext.ContentHintLowercase, browserAutoCapitalizeNone},
		{gpucontext.ContentHintUppercase, "characters"},
		{gpucontext.ContentHintTitlecase, "words"},
		{gpucontext.ContentHintAutoCapitalization, "sentences"},
		{gpucontext.ContentHintNone, browserAutoCapitalizeNone},
	} {
		if got := browserAutoCapitalize(test.hints); got != test.want {
			t.Errorf("hints %v autocapitalize = %q, want %q", test.hints, got, test.want)
		}
	}
	if !validBrowserIMEArea(gpucontext.IMECursorArea{X: 1, Y: 2, Width: 3, Height: 4}) {
		t.Fatal("valid cursor area rejected")
	}
	for _, area := range []gpucontext.IMECursorArea{
		{X: -1}, {Y: -1}, {Width: -1}, {Height: -1},
		{X: math.NaN()}, {Y: math.NaN()}, {Width: math.NaN()}, {Height: math.NaN()},
		{X: math.Inf(1)}, {Y: math.Inf(-1)}, {Width: math.Inf(1)}, {Height: math.Inf(-1)},
	} {
		if validBrowserIMEArea(area) {
			t.Errorf("invalid cursor area accepted: %+v", area)
		}
	}
}

func TestBrowserIMEHelperBoundaryCases(t *testing.T) {
	if !browserIMEComposition("").IsValid() {
		t.Fatal("empty composition should be valid")
	}
	text := "a😀中"
	if got := utf8OffsetToUTF16(text, -1); got != 0 {
		t.Fatalf("negative UTF-8 offset = %d, want 0", got)
	}
	if got := utf8OffsetToUTF16(text, len(text)); got != len([]rune(text))+1 {
		t.Fatalf("terminal UTF-8 offset = %d, want UTF-16 length %d", got, len([]rune(text))+1)
	}
	if got := utf16OffsetToUTF8(text, -1); got != 0 {
		t.Fatalf("negative UTF-16 offset = %d, want 0", got)
	}
	if got := utf16OffsetToUTF8(text, 2); got != 1 {
		t.Fatalf("surrogate UTF-16 offset = %d, want 1", got)
	}
	if got := utf16OffsetToUTF8(text, 99); got != len(text) {
		t.Fatalf("terminal UTF-16 offset = %d, want %d", got, len(text))
	}
	if previousRuneBytes(text, 0) != 0 || previousRuneBytes(text, len(text)+1) != 0 {
		t.Fatal("invalid previous-rune cursors were not rejected")
	}
	if nextRuneBytes(text, -1) != 0 || nextRuneBytes(text, len(text)) != 0 {
		t.Fatal("invalid next-rune cursors were not rejected")
	}
	if got := previousWordStart("   hello", len("   hello")); got != len("   ") {
		t.Fatalf("leading-space previous word start = %d, want %d", got, len("   "))
	}
	if got := nextWordEnd("   hello", 0); got != len("   ")+len("hello") {
		t.Fatalf("leading-space next word end = %d, want %d", got, len("   hello"))
	}
}
