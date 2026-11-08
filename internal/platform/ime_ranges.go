package platform

import (
	"unicode/utf16"
	"unicode/utf8"
)

// utf16IndexToUTF8Offset converts the UTF-16 code-unit index used by IMM32
// into a UTF-8 byte offset for gpucontext. Indexes inside a surrogate pair are
// rounded to the start of that rune so the returned offset remains valid.
func utf16IndexToUTF8Offset(text string, index int) int {
	if index <= 0 || text == "" {
		return 0
	}
	units := utf16.Encode([]rune(text))
	if index >= len(units) {
		return len(text)
	}
	usedUnits := 0
	for byteOffset, r := range text {
		width := 1
		if r >= 0x10000 {
			width = 2
		}
		if usedUnits+width > index {
			return byteOffset
		}
		usedUnits += width
		if usedUnits == index {
			return byteOffset + len(string(r))
		}
	}
	return len(text)
}

const (
	// ATTR_TARGET_CONVERTED and ATTR_TARGET_NOTCONVERTED identify the marked
	// segment in the GCS_COMPATTR byte array returned by IMM32.
	imeAttrTargetConverted    byte = 1
	imeAttrTargetNotConverted byte = 3
)

// imeSelectionRange converts IMM32's target attributes (one byte per UTF-16
// code unit) into the UTF-8 byte range expected by gpucontext. IMM32 may omit
// attributes for a provider that has no marked segment; the zero range is the
// valid and documented fallback.
func imeSelectionRange(text string, attributes []byte) (start, end int) {
	if text == "" || len(attributes) == 0 || !utf8.ValidString(text) {
		return 0, 0
	}
	units := utf16.Encode([]rune(text))
	limit := len(units)
	if len(attributes) < limit {
		limit = len(attributes)
	}
	targetStart, targetEnd := -1, -1
	unitIndex := 0
	for byteOffset, r := range text {
		width := 1
		if r >= 0x10000 {
			width = 2
		}
		for i := 0; i < width && unitIndex+i < limit; i++ {
			attribute := attributes[unitIndex+i]
			if attribute == imeAttrTargetConverted || attribute == imeAttrTargetNotConverted {
				if targetStart < 0 {
					targetStart = byteOffset
				}
				targetEnd = byteOffset + utf8.RuneLen(r)
			} else if targetStart >= 0 {
				return targetStart, targetEnd
			}
		}
		unitIndex += width
		if unitIndex >= limit && targetStart >= 0 {
			return targetStart, targetEnd
		}
	}
	if targetStart < 0 {
		return 0, 0
	}
	return targetStart, targetEnd
}
