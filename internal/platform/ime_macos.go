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

	composition gpucontext.IMEState
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
// range; NSTextInputClient does not expose a portable clause selection range.
func (s *macIMEState) setMarked(text string, selectedLocation, selectedLength uintptr) (started bool, composition gpucontext.IMEState) {
	if s == nil || !s.enabled || !utf8.ValidString(text) {
		return false, gpucontext.IMEState{}
	}
	cursorStart, cursorEnd, hidden, ok := macUTF16RangeToUTF8(text, selectedLocation, selectedLength)
	if !ok {
		return false, gpucontext.IMEState{}
	}
	if hidden {
		cursorStart, cursorEnd = -1, -1
	}
	composition = gpucontext.IMEState{
		Composing:       true,
		CompositionText: text,
		Cursor:          gpucontext.IMETextRange{Start: cursorStart, End: cursorEnd},
	}
	if !composition.IsValid() {
		return false, gpucontext.IMEState{}
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
	s.nativeNeedsUnmark = false
	s.composition = gpucontext.IMEState{}
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
	s.composition = gpucontext.IMEState{}
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
		s.nativeNeedsUnmark = s.nativeNeedsUnmark || canceled
		s.marked = false
		s.composition = gpucontext.IMEState{}
		s.surrounding = gpucontext.IMESurroundingText{}
	}
	return canceled
}
