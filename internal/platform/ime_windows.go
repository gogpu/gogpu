//go:build windows

package platform

import (
	"math"
	"unicode/utf16"
	"unsafe"

	"github.com/gogpu/gpucontext"
)

// IMECapabilities reports the operations implemented by the IMM32 backend.
// IMM32 provides composition/commit/cancel lifecycle, candidate placement,
// and advisory content metadata. It has no portable delete-surrounding event
// or surrounding-text query, so those capability bits remain unset.
func (w *win32Window) IMECapabilities() gpucontext.IMECapabilities {
	return DefaultIMECapabilities()
}

func (p *windowsPlatform) IMECapabilities() gpucontext.IMECapabilities {
	return DefaultIMECapabilities()
}

func (w *win32Window) SetIMEPosition(x, y int) {
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: float64(x), Y: float64(y)})
}

func (w *win32Window) SetIMEEnabled(enabled bool) {
	w.imeMu.Lock()
	wasEnabled := w.ime.enabled
	canceled := w.ime.setEnabled(enabled)
	if !enabled {
		// Surrounding text is sensitive context. Drop the native copy as soon
		// as input is disabled; App retains only the explicit configuration it
		// may replay when a widget enables IME again.
		w.imeSurrounding = gpucontext.IMESurroundingText{}
	}
	w.imeMu.Unlock()

	if himc := w.imeContext(); himc != 0 {
		procImmSetOpenStatus.Call(himc, boolToUintptr(enabled))
		procImmReleaseContext.Call(uintptr(w.hwnd), himc)
	}
	if enabled {
		w.applyStoredIMECursorArea()
	}
	if canceled {
		w.notifyIMECancel()
	}
	if !enabled && wasEnabled {
		w.queueIME(Event{WindowID: w.id, Type: EventIMEDisabled})
	}
}

func (w *win32Window) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if area.X < 0 || area.Y < 0 || area.Width < 0 || area.Height < 0 ||
		math.IsNaN(area.X) || math.IsNaN(area.Y) || math.IsNaN(area.Width) || math.IsNaN(area.Height) ||
		math.IsInf(area.X, 0) || math.IsInf(area.Y, 0) || math.IsInf(area.Width, 0) || math.IsInf(area.Height, 0) {
		return
	}
	w.imeMu.Lock()
	w.imeArea = area
	w.imeAreaSet = true
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	if enabled {
		w.applyIMECursorArea(area)
	}
}

func (w *win32Window) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	w.imeMu.Lock()
	w.imePurpose, w.imeHints = purpose, hints
	w.imeMu.Unlock()
}

func (w *win32Window) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if !text.IsValid() {
		return
	}
	w.imeMu.Lock()
	if w.ime.enabled {
		w.imeSurrounding = text
	}
	w.imeMu.Unlock()
}

func (w *win32Window) CancelIME() {
	w.imeMu.Lock()
	canceled := w.ime.cancel()
	w.imeMu.Unlock()
	if !canceled {
		return
	}
	w.notifyIMECancel()
}

func (w *win32Window) imeContext() uintptr {
	if w == nil || w.hwnd == 0 {
		return 0
	}
	himc, _, _ := procImmGetContext.Call(uintptr(w.hwnd))
	return himc
}

func (w *win32Window) notifyIMECancel() {
	if himc := w.imeContext(); himc != 0 {
		procImmNotifyIME.Call(himc, immNicompositionstr, immCpsCancel, 0)
		procImmReleaseContext.Call(uintptr(w.hwnd), himc)
	}
	w.queueIME(Event{WindowID: w.id, Type: EventIMECanceled})
}

func (w *win32Window) applyIMECursorArea(area gpucontext.IMECursorArea) {
	himc := w.imeContext()
	if himc == 0 {
		return
	}
	defer procImmReleaseContext.Call(uintptr(w.hwnd), himc)

	scale := w.scaleFactor()
	if scale <= 0 {
		scale = 1
	}
	pt := point{
		x: int32(math.Round(area.X * scale)),
		y: int32(math.Round(area.Y * scale)),
	}
	procClientToScreen.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt)))
	form := candidateForm{
		dwStyle:      immCFSCandidatePos,
		ptCurrentPos: pt,
	}
	if area.Width > 0 || area.Height > 0 {
		form.dwStyle = immCFSExclude
		form.rcArea = rect{
			left:   pt.x,
			top:    pt.y,
			right:  pt.x + int32(math.Round(area.Width*scale)),
			bottom: pt.y + int32(math.Round(area.Height*scale)),
		}
	}
	procImmSetCandidateWindow.Call(himc, uintptr(unsafe.Pointer(&form)))
}

func (w *win32Window) applyStoredIMECursorArea() {
	w.imeMu.Lock()
	area, set, enabled := w.imeArea, w.imeAreaSet, w.ime.enabled
	w.imeMu.Unlock()
	if set && enabled {
		w.applyIMECursorArea(area)
	}
}

// handleIMEStart is called from wndProc on WM_IME_STARTCOMPOSITION.
func (w *win32Window) handleIMEStart() {
	w.imeMu.Lock()
	started := w.ime.start()
	w.imeMu.Unlock()
	if started {
		w.queueIME(Event{WindowID: w.id, Type: EventIMECompositionStart})
	}
}

// handleIMEComposition reads the IMM32 preedit/result payload and queues one
// update event. Result chunks are held until WM_IME_ENDCOMPOSITION so the
// legacy end callback receives one complete committed string.
func (w *win32Window) handleIMEComposition(flags uintptr) {
	w.imeMu.Lock()
	started := w.ime.ensureActive()
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	if !enabled {
		return
	}
	if started {
		w.queueIME(Event{WindowID: w.id, Type: EventIMECompositionStart})
	}

	himc := w.imeContext()
	if himc == 0 {
		return
	}
	defer procImmReleaseContext.Call(uintptr(w.hwnd), himc)

	compositionText := ""
	if flags&gcsCompStr != 0 {
		compositionText = w.immString(himc, gcsCompStr)
	}
	if flags&gcsResultStr != 0 {
		result := w.immString(himc, gcsResultStr)
		w.imeMu.Lock()
		w.ime.addResult(result)
		w.imeMu.Unlock()
	}

	// IMM32 reports the cursor as a UTF-16 code-unit index. Convert it to a
	// UTF-8 byte offset before crossing the gpucontext boundary.
	cursorUnits := 0
	if flags&gcsCompStr != 0 {
		pos, _, _ := procImmGetCompositionStringW.Call(himc, gcsCursorPos, 0, 0)
		if pos != ^uintptr(0) && pos > 0 && pos <= uintptr(len(compositionText)*2) {
			cursorUnits = int(pos)
		}
	}
	cursorByte := utf16IndexToUTF8Offset(compositionText, cursorUnits)
	selectionStart, selectionEnd := imeSelectionRange(compositionText, w.immBytes(himc, gcsCompAttr))
	composition := gpucontext.IMEComposition{
		CompositionText: compositionText,
		CursorBegin:     cursorByte,
		CursorEnd:       cursorByte,
		SelectionStart:  selectionStart,
		SelectionEnd:    selectionEnd,
	}
	if !composition.IsValid() {
		return
	}
	w.queueIME(Event{
		WindowID:       w.id,
		Type:           EventIMECompositionUpdate,
		IMEComposition: composition,
	})
}

func (w *win32Window) handleIMEEnd() {
	w.imeMu.Lock()
	committed, ok := w.ime.end()
	if ok && committed == "" {
		// Legacy IMM32 providers may deliver the result as WM_CHAR after END
		// instead of exposing GCS_RESULTSTR. Defer the end event until those
		// UTF-16 units have been collected (or PollEvents flushes an empty one).
		w.ime.beginCharResult()
	}
	w.imeMu.Unlock()
	if !ok {
		return
	}
	if committed == "" {
		return
	}
	w.queueIME(Event{
		WindowID:     w.id,
		Type:         EventIMECompositionEnd,
		IMECommitted: committed,
	})
}

func (w *win32Window) consumeIMECharResult(codeUnit uint16) bool {
	w.imeMu.Lock()
	consumed := w.ime.consumeCharResultUnit(codeUnit)
	w.imeMu.Unlock()
	if !consumed {
		return false
	}
	if !w.hasNextIMECharMessage() {
		w.finishIMECharResult()
	}
	return true
}

func (w *win32Window) consumeIMECharResultUnits(units []uint16) bool {
	w.imeMu.Lock()
	consumed := w.ime.consumeCharResultUnits(units)
	w.imeMu.Unlock()
	if !consumed {
		return false
	}
	w.finishIMECharResult()
	return true
}

func (w *win32Window) finishIMECharResult() {
	w.imeMu.Lock()
	committed, ok := w.ime.finishCharResult()
	w.imeMu.Unlock()
	if ok {
		w.queueIME(Event{
			WindowID:     w.id,
			Type:         EventIMECompositionEnd,
			IMECommitted: committed,
		})
	}
}

func (w *win32Window) hasNextIMECharMessage() bool {
	var message msgStruct
	ret, _, _ := procPeekMessageW.Call(
		uintptr(unsafe.Pointer(&message)),
		0, 0, 0,
		pmNoRemove,
	)
	if ret == 0 {
		return false
	}
	return message.message == wmChar || message.message == wmSysChar || message.message == wmIMEChar
}

func (w *win32Window) immString(himc uintptr, index uintptr) string {
	length, _, _ := procImmGetCompositionStringW.Call(himc, index, 0, 0)
	// The API returns -1 on failure. Syscall wrappers expose that LONG as a
	// max-sized uintptr, which must not be used as an allocation length.
	if length == ^uintptr(0) || length == 0 || length > 1<<20 {
		return ""
	}
	units := make([]uint16, (length+1)/2)
	read, _, _ := procImmGetCompositionStringW.Call(
		himc,
		index,
		uintptr(unsafe.Pointer(&units[0])),
		length,
	)
	if read == ^uintptr(0) || read == 0 || read > length {
		return ""
	}
	return string(utf16.Decode(units[:read/2]))
}

// immBytes reads an IMM32 payload whose size is expressed directly in bytes,
// such as GCS_COMPATTR. The API uses a signed LONG for its result; the syscall
// wrapper exposes that value as uintptr, so -1 must be rejected before any
// allocation.
func (w *win32Window) immBytes(himc uintptr, index uintptr) []byte {
	length, _, _ := procImmGetCompositionStringW.Call(himc, index, 0, 0)
	if length == ^uintptr(0) || length == 0 || length > 1<<20 {
		return nil
	}
	data := make([]byte, length)
	read, _, _ := procImmGetCompositionStringW.Call(
		himc,
		index,
		uintptr(unsafe.Pointer(&data[0])),
		length,
	)
	if read == ^uintptr(0) || read == 0 || read > length {
		return nil
	}
	if read < length {
		data = data[:read]
	}
	return data
}

func (w *win32Window) consumeIMECharUnit(codeUnit uint16) bool {
	w.imeMu.Lock()
	defer w.imeMu.Unlock()
	return w.ime.consumeCharUnit(codeUnit)
}

func (w *win32Window) beforeIMEKeyDown() {
	w.imeMu.Lock()
	pending := w.ime.hasPendingCharResult()
	w.ime.beforeKeyDown()
	w.imeMu.Unlock()
	if pending {
		// A new key gesture after WM_IME_END means no post-END WM_CHAR result
		// followed. Close the old lifecycle before routing this key.
		w.finishIMECharResult()
	}
}

func (w *win32Window) queueIME(event Event) {
	if w != nil && w.platform != nil {
		w.platform.queueEvent(event)
	}
}

func boolToUintptr(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}

var _ gpucontext.IMEControllerV2 = (*win32Window)(nil)
var _ gpucontext.IMECapabilityProviderV2 = (*win32Window)(nil)
