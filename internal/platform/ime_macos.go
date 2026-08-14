package platform

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gogpu/gpucontext"
)

// macIMEState is the platform-independent state machine behind the AppKit
// bridge. AppKit reports NSRange values in UTF-16 code units; the state that
// crosses into gpucontext always uses UTF-8 byte offsets.
type macIMEState struct {
	enabled bool
	marked  bool
	// nativeNeedsUnmark is set when Go cancels state outside AppKit's callback
	// stack. The next main-thread event flushes the corresponding NSTextView
	// native marked range without emitting a second public cancellation event.
	nativeNeedsUnmark bool

	composition gpucontext.IMEComposition
	surrounding gpucontext.IMESurroundingText
	area        gpucontext.IMECursorArea
	purpose     gpucontext.ContentPurpose
	hints       gpucontext.ContentHint
}

// macUTF16RangeToUTF8 converts one Cocoa NSRange into a UTF-8 byte range.
// NSNotFound (NSUInteger max) denotes a hidden cursor in the selected range.
func macUTF16RangeToUTF8(text string, location, length uintptr) (start, end int, hidden, ok bool) {
	if !utf8.ValidString(text) {
		return 0, 0, false, false
	}
	if location == ^uintptr(0) || length == ^uintptr(0) {
		return -1, -1, true, true
	}
	units := len(utf16.Encode([]rune(text)))
	if location > uintptr(units) || length > uintptr(units)-location {
		return 0, 0, false, false
	}
	start = utf16IndexToUTF8Offset(text, int(location))
	end = utf16IndexToUTF8Offset(text, int(location+length))
	if start < 0 || end < start || end > len(text) {
		return 0, 0, false, false
	}
	return start, end, false, true
}

// setMarked stores a preedit update and returns whether this is the first
// update in a composition session. Cocoa's selectedRange is the active cursor
// range; the complete preedit is the marked/selection range.
func (s *macIMEState) setMarked(text string, selectedLocation, selectedLength uintptr) (started bool, composition gpucontext.IMEComposition) {
	if s == nil || !s.enabled || !utf8.ValidString(text) {
		return false, gpucontext.IMEComposition{}
	}
	cursorStart, cursorEnd, hidden, ok := macUTF16RangeToUTF8(text, selectedLocation, selectedLength)
	if !ok {
		return false, gpucontext.IMEComposition{}
	}
	if hidden {
		cursorStart, cursorEnd = -1, -1
	}
	composition = gpucontext.IMEComposition{
		CompositionText: text,
		CursorBegin:     cursorStart,
		CursorEnd:       cursorEnd,
		SelectionStart:  0,
		SelectionEnd:    len(text),
	}
	if !composition.IsValid() {
		return false, gpucontext.IMEComposition{}
	}
	started = !s.marked
	s.marked = true
	s.nativeNeedsUnmark = false
	s.composition = composition
	return started, composition
}

// insert clears the preedit and reports whether the committed text closes an
// active composition. A non-composition insert is delivered as ordinary text.
func (s *macIMEState) insert(text string) (wasMarked bool) {
	if s == nil {
		return false
	}
	wasMarked = s.marked
	s.marked = false
	s.composition = gpucontext.IMEComposition{}
	return wasMarked
}

// unmark clears a preedit without committing it. NSTextInputClient uses this
// callback for cancellation and focus transitions.
func (s *macIMEState) unmark() (wasMarked bool) {
	if s == nil {
		return false
	}
	wasMarked = s.marked
	s.marked = false
	if wasMarked {
		s.nativeNeedsUnmark = true
	}
	s.composition = gpucontext.IMEComposition{}
	return wasMarked
}

// setEnabled changes the native input state and reports whether disabling it
// canceled an active composition. Sensitive surrounding text is cleared by
// the platform caller at the same transition.
func (s *macIMEState) setEnabled(enabled bool) (canceled bool) {
	if s == nil {
		return false
	}
	canceled = !enabled && s.marked
	s.enabled = enabled
	if !enabled {
		s.nativeNeedsUnmark = canceled
		s.marked = false
		s.composition = gpucontext.IMEComposition{}
		s.surrounding = gpucontext.IMESurroundingText{}
	}
	return canceled
}
