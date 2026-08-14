package platform

import (
	"math"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gogpu/gpucontext"
)

func validBrowserIMEArea(area gpucontext.IMECursorArea) bool {
	return area.X >= 0 && area.Y >= 0 && area.Width >= 0 && area.Height >= 0 &&
		!math.IsNaN(area.X) && !math.IsNaN(area.Y) && !math.IsNaN(area.Width) && !math.IsNaN(area.Height) &&
		!math.IsInf(area.X, 0) && !math.IsInf(area.Y, 0) && !math.IsInf(area.Width, 0) && !math.IsInf(area.Height, 0)
}

func browserIMEComposition(data string) gpucontext.IMEComposition {
	end := len(data)
	return gpucontext.IMEComposition{
		CompositionText: data,
		CursorBegin:     end,
		CursorEnd:       end,
		SelectionStart:  0,
		SelectionEnd:    end,
	}
}

func browserInputMode(purpose gpucontext.ContentPurpose) string {
	switch purpose {
	case gpucontext.ContentPurposeDigits, gpucontext.ContentPurposePin:
		return "numeric"
	case gpucontext.ContentPurposeNumber:
		return "decimal"
	case gpucontext.ContentPurposePhone:
		return "tel"
	case gpucontext.ContentPurposeURL:
		return "url"
	case gpucontext.ContentPurposeEmail:
		return "email"
	case gpucontext.ContentPurposeDate:
		return "numeric"
	case gpucontext.ContentPurposeTime:
		return "numeric"
	case gpucontext.ContentPurposeDateTime:
		return "datetime"
	default:
		return "text"
	}
}

func browserAutoCapitalize(hints gpucontext.ContentHint) string {
	switch {
	case hints.Has(gpucontext.ContentHintLowercase):
		return "none"
	case hints.Has(gpucontext.ContentHintUppercase):
		return "characters"
	case hints.Has(gpucontext.ContentHintTitlecase):
		return "words"
	case hints.Has(gpucontext.ContentHintAutoCapitalization):
		return "sentences"
	default:
		return "none"
	}
}

func utf8OffsetToUTF16(text string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(text) {
		return len(utf16.Encode([]rune(text)))
	}
	return len(utf16.Encode([]rune(text[:offset])))
}

func utf16OffsetToUTF8(text string, offset int) int {
	if offset <= 0 {
		return 0
	}
	units := 0
	for byteIndex, r := range text {
		runeUnits := len(utf16.Encode([]rune{r}))
		if units+runeUnits > offset {
			return byteIndex
		}
		units += runeUnits
	}
	return len(text)
}

func previousRuneBytes(text string, cursor int) int {
	if cursor <= 0 || cursor > len(text) {
		return 0
	}
	_, size := utf8.DecodeLastRuneInString(text[:cursor])
	if size <= 0 {
		return 0
	}
	return size
}

func nextRuneBytes(text string, cursor int) int {
	if cursor < 0 || cursor >= len(text) {
		return 0
	}
	_, size := utf8.DecodeRuneInString(text[cursor:])
	if size <= 0 {
		return 0
	}
	return size
}

func previousWordStart(text string, cursor int) int {
	for cursor > 0 {
		start := cursor - previousRuneBytes(text, cursor)
		_, size := utf8.DecodeRuneInString(text[start:cursor])
		if size <= 0 {
			return cursor
		}
		r, _ := utf8.DecodeRuneInString(text[start:cursor])
		if !unicode.IsSpace(r) {
			break
		}
		cursor = start
	}
	for cursor > 0 {
		start := cursor - previousRuneBytes(text, cursor)
		r, _ := utf8.DecodeRuneInString(text[start:cursor])
		if unicode.IsSpace(r) {
			break
		}
		cursor = start
	}
	return cursor
}

func nextWordEnd(text string, cursor int) int {
	for cursor < len(text) {
		r, size := utf8.DecodeRuneInString(text[cursor:])
		if !unicode.IsSpace(r) {
			break
		}
		cursor += size
	}
	for cursor < len(text) {
		r, size := utf8.DecodeRuneInString(text[cursor:])
		if unicode.IsSpace(r) {
			break
		}
		cursor += size
	}
	return cursor
}
