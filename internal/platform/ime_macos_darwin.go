//go:build darwin

package platform

import (
	"math"
	"unicode/utf8"
	"unsafe"

	"github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gpucontext"
)

// IMECapabilities reports the operations implemented by the AppKit backend.
func (w *darwinWindow) IMECapabilities() gpucontext.IMECapabilities {
	return DefaultIMECapabilities()
}

func (w *darwinWindow) imeEnabled() bool {
	if w == nil {
		return false
	}
	w.imeMu.Lock()
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	return enabled
}

func (w *darwinWindow) SetIMEPosition(x, y int) {
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: float64(x), Y: float64(y)})
}

func (w *darwinWindow) SetIMEEnabled(enabled bool) {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	wasEnabled := w.ime.enabled
	canceled := w.ime.setEnabled(enabled)
	w.imeMu.Unlock()

	if enabled {
		return
	}
	if canceled {
		w.queueEvent(Event{WindowID: w.id, Type: EventIMECanceled})
	}
	if wasEnabled {
		w.queueEvent(Event{WindowID: w.id, Type: EventIMEDisabled})
	}
}

func (w *darwinWindow) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if w == nil || !validIMECursorArea(area) {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	w.ime.area = area
	w.imeMu.Unlock()
}

func (w *darwinWindow) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	w.ime.purpose = purpose
	w.ime.hints = hints
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	// AppKit can query the surrounding text through
	// attributedSubstringForProposedRange:. Do not leave a native marked range
	// or surrounding context alive when a password/hidden/sensitive field is
	// selected; the next explicit enable starts a fresh session.
	if enabled && imeContentIsSensitive(purpose, hints) {
		w.SetIMEEnabled(false)
	}
}

func (w *darwinWindow) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if w == nil || !text.IsValid() {
		return
	}
	w.imeMu.Lock()
	if !w.imeDestroyed && w.ime.enabled && !imeContentIsSensitive(w.ime.purpose, w.ime.hints) {
		w.ime.surrounding = text
	}
	w.imeMu.Unlock()
}

func (w *darwinWindow) CancelIME() {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	canceled := w.ime.unmark()
	w.imeMu.Unlock()
	if !canceled {
		return
	}
	w.queueEvent(Event{WindowID: w.id, Type: EventIMECanceled})
}

func (w *darwinWindow) handleIMEKeyDown(view darwin.ID, event darwin.ID) {
	if w == nil || event == 0 {
		return
	}
	if w.imeEnabled() {
		// This callback is reached on AppKit's main thread. Deferring the
		// input-context invalidation to this boundary keeps controller methods
		// safe when widgets update IME state from a render/worker goroutine.
		w.flushNativeIMEReset(view)
		w.applyMacIMEArea(view)
		darwin.InvalidateInputContext(view)
		darwin.InterpretKeyEvent(view, event)
	}
}

func (w *darwinWindow) flushNativeIMEReset(view darwin.ID) {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	reset := w.ime.nativeNeedsUnmark
	w.ime.nativeNeedsUnmark = false
	w.imeMu.Unlock()
	if reset {
		darwin.UnmarkText(view)
	}
}

func (w *darwinWindow) applyMacIMEArea(view darwin.ID) {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	area := w.ime.area
	w.imeMu.Unlock()
	darwin.SetTextInputCursorArea(view, area.X, area.Y)
}

func (w *darwinWindow) handleMacSetMarkedText(text darwin.ID, selectedLocation, selectedLength,
	_, _ uintptr) {
	if w == nil {
		return
	}
	value := darwin.ObjectString(text)
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	started, composition := w.ime.setMarked(value, selectedLocation, selectedLength)
	w.imeMu.Unlock()
	w.applyMacIMEAreaForWindow()
	if !composition.IsValid() {
		return
	}
	if started {
		w.queueEvent(Event{WindowID: w.id, Type: EventIMECompositionStart})
	}
	w.queueEvent(Event{
		WindowID:       w.id,
		Type:           EventIMECompositionUpdate,
		IMEComposition: composition,
	})
}

func (w *darwinWindow) applyMacIMEAreaForWindow() {
	if w == nil || w.window == nil {
		return
	}
	w.applyMacIMEArea(w.window.ContentView())
}

func (w *darwinWindow) handleMacInsertText(text darwin.ID, _, _ uintptr) {
	if w == nil {
		return
	}
	value := darwin.ObjectString(text)
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	if !w.ime.enabled {
		w.imeMu.Unlock()
		return
	}
	wasMarked := w.ime.insert(value)
	w.imeMu.Unlock()

	if wasMarked {
		// NSTextInputClient's insertText: is the one committed-text path. It is
		// delivered exactly once and is never duplicated as EventChar.
		w.queueEvent(Event{
			WindowID:     w.id,
			Type:         EventIMECompositionEnd,
			IMECommitted: value,
		})
		return
	}
	// AppKit also uses insertText: for ordinary text while IME is enabled.
	// Preserve the public EventChar stream for that non-composition path.
	w.dispatchMacInsertedText(value)
}

func (w *darwinWindow) dispatchMacInsertedText(value string) {
	if !utf8.ValidString(value) {
		return
	}
	for _, r := range value {
		if r >= 0xF700 && r <= 0xF8FF {
			continue
		}
		if r >= 32 && r != 127 {
			w.queueEvent(Event{WindowID: w.id, Type: EventChar, Char: r})
		}
	}
}

func (w *darwinWindow) handleMacUnmarkText() {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	if !w.ime.enabled {
		w.imeMu.Unlock()
		return
	}
	canceled := w.ime.unmark()
	// The native callback itself already unmarked NSTextView; avoid scheduling
	// another native reset while retaining the public cancellation event.
	w.ime.nativeNeedsUnmark = false
	w.imeMu.Unlock()
	if canceled {
		w.queueEvent(Event{WindowID: w.id, Type: EventIMECanceled})
	}
}

func (w *darwinWindow) handleMacAttributedSubstring(location, length, actualRange uintptr) darwin.ID {
	if w == nil {
		return darwin.NewAutoreleasedAttributedString("")
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return darwin.NewAutoreleasedAttributedString("")
	}
	enabled := w.ime.enabled
	sensitive := imeContentIsSensitive(w.ime.purpose, w.ime.hints)
	surrounding := w.ime.surrounding
	w.imeMu.Unlock()
	if !enabled || sensitive || !surrounding.IsValid() {
		return darwin.NewAutoreleasedAttributedString("")
	}
	start, end, hidden, ok := macUTF16RangeToUTF8(surrounding.Text, location, length)
	if !ok || hidden {
		writeNSRange(actualRange, 0, 0)
		return darwin.NewAutoreleasedAttributedString("")
	}
	writeNSRange(actualRange, location, length)
	return darwin.NewAutoreleasedAttributedString(surrounding.Text[start:end])
}

func writeNSRange(pointer, location, length uintptr) {
	if pointer == 0 {
		return
	}
	rangeValue := struct {
		location uintptr
		length   uintptr
	}{location: location, length: length}
	*(*struct {
		location uintptr
		length   uintptr
	})(unsafe.Pointer(pointer)) = rangeValue
}

func validIMECursorArea(area gpucontext.IMECursorArea) bool {
	return area.X >= 0 && area.Y >= 0 && area.Width >= 0 && area.Height >= 0 &&
		!math.IsNaN(area.X) && !math.IsNaN(area.Y) &&
		!math.IsNaN(area.Width) && !math.IsNaN(area.Height) &&
		!math.IsInf(area.X, 0) && !math.IsInf(area.Y, 0) &&
		!math.IsInf(area.Width, 0) && !math.IsInf(area.Height, 0)
}

// Forward the v2 controller/provider methods through PlatformWindow, which
// deliberately remains source-compatible with existing implementations.
func (dw *darwinPlatformWindow) IMECapabilities() gpucontext.IMECapabilities {
	if dw != nil && dw.owner != nil {
		return dw.owner.IMECapabilities()
	}
	return DefaultIMECapabilities()
}

func (dw *darwinPlatformWindow) SetIMEPosition(x, y int) {
	if dw != nil && dw.owner != nil {
		dw.owner.SetIMEPosition(x, y)
	}
}

func (dw *darwinPlatformWindow) SetIMEEnabled(enabled bool) {
	if dw != nil && dw.owner != nil {
		dw.owner.SetIMEEnabled(enabled)
	}
}

func (dw *darwinPlatformWindow) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if dw != nil && dw.owner != nil {
		dw.owner.SetIMECursorArea(area)
	}
}

func (dw *darwinPlatformWindow) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	if dw != nil && dw.owner != nil {
		dw.owner.SetIMEContentType(purpose, hints)
	}
}

func (dw *darwinPlatformWindow) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if dw != nil && dw.owner != nil {
		dw.owner.SetIMESurroundingText(text)
	}
}

func (dw *darwinPlatformWindow) CancelIME() {
	if dw != nil && dw.owner != nil {
		dw.owner.CancelIME()
	}
}

var _ gpucontext.IMEControllerV2 = (*darwinWindow)(nil)
var _ gpucontext.IMECapabilityProviderV2 = (*darwinWindow)(nil)
var _ gpucontext.IMEControllerV2 = (*darwinPlatformWindow)(nil)
var _ gpucontext.IMECapabilityProviderV2 = (*darwinPlatformWindow)(nil)
