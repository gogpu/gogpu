//go:build windows

package platform

import (
	"math"
	"unicode/utf16"
	"unsafe"

	"github.com/gogpu/gpucontext"
)

// IMECapabilities reports the operations implemented by the IMM32 backend.
// IMM32 provides the composition/commit/cancel lifecycle and candidate
// placement, and nothing else: it has no surrounding-text query, no
// delete-surrounding request and no content-hint API.
func (w *win32Window) IMECapabilities() gpucontext.IMECapabilities {
	return defaultIMECapabilities()
}

// initIMEState starts the window with IME disabled: a focused text widget
// enables it. Runs on the window thread right after HWND creation and before
// the message pump can deliver WM_IME_* messages.
func (w *win32Window) initIMEState() {
	if w == nil || w.hwnd == 0 {
		return
	}
	procImmAssociateContextEx.Call(uintptr(w.hwnd), 0, iacChildren)
}

// destroyIMEState severs controller/native callbacks before HWND teardown.
// IMM32 can deliver queued messages after DestroyWindow begins; those messages
// must not resurrect a composition or retain surrounding text.
func (w *win32Window) destroyIMEState() {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	w.imeDestroyed = true
	w.imeCancelPending = false
	w.ime.setEnabled(false)
	w.imeSurrounding = gpucontext.IMESurroundingText{}
	w.imeMu.Unlock()
}

// SetIMEEnabled enables or disables the native input method. It is safe to
// call from any goroutine: the association change happens on the window
// thread, and disabling cancels an active composition first.
func (w *win32Window) SetIMEEnabled(enabled bool) {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	// Password, hidden and sensitive content keeps the native IME out of the
	// session even if a caller enables it; the content type is the only signal
	// IMM32 gives us about the widget's data.
	if enabled && imeContentIsSensitive(w.imePurpose, w.imeHints) {
		enabled = false
	}
	wasEnabled := w.ime.enabled
	canceled := w.ime.setEnabled(enabled)
	if !enabled {
		// Surrounding text is sensitive context. Drop the native copy as soon
		// as input is disabled; App retains only the explicit configuration it
		// may replay when a widget enables IME again.
		w.imeSurrounding = gpucontext.IMESurroundingText{}
	}
	w.imeMu.Unlock()

	if canceled {
		w.notifyIMECancel()
	}
	if !enabled && wasEnabled {
		w.queueIME(&Event{WindowID: w.id, Type: EventIMEDisabled})
	}
	w.postIMEApply()
}

// SetIMECursorArea stores the caret rectangle and, when IME is enabled,
// applies it to the candidate and composition windows on the window thread.
func (w *win32Window) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if !imeAreaIsValid(area) {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	w.imeArea = area
	w.imeAreaSet = true
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	if enabled {
		w.postIMEApply()
	}
}

// SetIMEContentType stores the advisory purpose and hints. IMM32 has no API
// for them, so they only drive the privacy boundary: a session that becomes
// sensitive is terminated at once.
func (w *win32Window) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	w.imePurpose, w.imeHints = purpose, hints
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	if enabled && imeContentIsSensitive(purpose, hints) {
		w.SetIMEEnabled(false)
	}
}

// SetIMESurroundingText retains the surrounding text for the duration of an
// enabled, non-sensitive session. IMM32 cannot receive it, so it is never
// exposed to the native IME; the value is kept only so a backend that does
// support it (Wayland, X11) behaves identically through the same path.
func (w *win32Window) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if !text.IsValid() {
		return
	}
	w.imeMu.Lock()
	if !w.imeDestroyed && w.ime.enabled && !imeContentIsSensitive(w.imePurpose, w.imeHints) {
		w.imeSurrounding = text
	}
	w.imeMu.Unlock()
}

// CancelIME cancels an active composition without committing its preedit.
func (w *win32Window) CancelIME() {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	canceled := w.ime.cancel()
	w.imeMu.Unlock()
	if !canceled {
		return
	}
	w.notifyIMECancel()
}

// imeContext returns the window thread's IME context and the HWND it was
// taken from. It must be called on the window thread, and the caller releases
// the context with ImmReleaseContext(hwnd, himc). The HWND is captured under
// imeMu because IMEController calls from other goroutines reach this function
// while Destroy may be zeroing the handle.
func (w *win32Window) imeContext() (himc, hwnd uintptr) {
	if w == nil {
		return 0, 0
	}
	w.imeMu.Lock()
	hwnd = uintptr(w.hwnd)
	destroyed := w.imeDestroyed
	w.imeMu.Unlock()
	if destroyed || hwnd == 0 {
		return 0, 0
	}
	himc, _, _ = procImmGetContext.Call(hwnd)
	return himc, hwnd
}

// notifyIMECancel asks the native IME to drop the composition on the window
// thread and tells consumers the preedit was not committed.
func (w *win32Window) notifyIMECancel() {
	w.imeMu.Lock()
	w.imeCancelPending = true
	w.imeMu.Unlock()
	w.postIMEApply()
	w.queueIME(&Event{WindowID: w.id, Type: EventIMECanceled})
}

// postIMEApply schedules applyIMENativeState on the window thread, coalescing
// repeated calls made before that message runs.
func (w *win32Window) postIMEApply() {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if w.imeDestroyed || w.hwnd == 0 || w.imeApplyPosted {
		w.imeMu.Unlock()
		return
	}
	w.imeApplyPosted = true
	hwnd := uintptr(w.hwnd)
	w.imeMu.Unlock()

	if ret, _, _ := procPostMessageW.Call(hwnd, wmIMEApply, 0, 0); ret == 0 {
		w.imeMu.Lock()
		w.imeApplyPosted = false
		w.imeMu.Unlock()
	}
}

// applyIMENativeState applies the desired IME state with IMM32. It runs only
// from wndProc, on the thread that owns the window.
func (w *win32Window) applyIMENativeState() {
	w.imeMu.Lock()
	if w.imeDestroyed || w.hwnd == 0 {
		w.imeApplyPosted = false
		w.imeMu.Unlock()
		return
	}
	w.imeApplyPosted = false
	open := w.ime.enabled
	openChanged := open != w.imeOpenApplied
	cancel := w.imeCancelPending
	area, areaSet := w.imeArea, w.imeAreaSet
	w.imeCancelPending = false
	w.imeMu.Unlock()

	if cancel {
		if himc, hwnd := w.imeContext(); himc != 0 {
			procImmNotifyIME.Call(himc, immNicompositionstr, immCpsCancel, 0)
			procImmReleaseContext.Call(hwnd, himc)
		}
	}
	if openChanged {
		style := uintptr(iacChildren)
		if open {
			style = iacDefault
		}
		procImmAssociateContextEx.Call(uintptr(w.hwnd), 0, style)
		w.imeMu.Lock()
		w.imeOpenApplied = open
		w.imeMu.Unlock()
	}
	if areaSet {
		w.applyIMECursorAreaNative(area)
	}
}

// applyIMECursorAreaNative positions the candidate window and the (hidden)
// composition window. Runs on the window thread.
func (w *win32Window) applyIMECursorAreaNative(area gpucontext.IMECursorArea) {
	himc, hwnd := w.imeContext()
	if himc == 0 {
		return
	}
	defer procImmReleaseContext.Call(hwnd, himc)

	scale := w.scaleFactor()
	if scale <= 0 {
		scale = 1
	}
	pt := point{
		x: int32(math.Round(area.X * scale)),
		y: int32(math.Round(area.Y * scale)),
	}
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
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
	composition := compositionForm{
		dwStyle:      cfsPoint,
		ptCurrentPos: point{x: pt.x, y: pt.y + int32(math.Round(area.Height*scale))},
		rcArea:       form.rcArea,
	}
	procImmSetCompositionWindow.Call(himc, uintptr(unsafe.Pointer(&composition)))
}

// applyStoredIMECursorArea re-applies the caret rectangle after a DPI change.
// Runs on the window thread.
func (w *win32Window) applyStoredIMECursorArea() {
	w.imeMu.Lock()
	area, set := w.imeArea, w.imeAreaSet
	w.imeMu.Unlock()
	if set {
		w.applyIMECursorAreaNative(area)
	}
}

// handleIMEStart is called from wndProc on WM_IME_STARTCOMPOSITION.
func (w *win32Window) handleIMEStart() {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	started := w.ime.start()
	w.imeMu.Unlock()
	if started {
		w.queueIME(&Event{WindowID: w.id, Type: EventIMECompositionStart})
	}
}

// handleIMEComposition reads the IMM32 preedit/result payload and queues one
// update event. Result chunks are held until WM_IME_ENDCOMPOSITION so the end
// callback receives one complete committed string.
func (w *win32Window) handleIMEComposition(flags uintptr) {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	armed := w.ime.hasPendingCharResult()
	w.imeMu.Unlock()

	// The route keeps a result that belongs to the post-END lifecycle or to a
	// standalone commit out of the preedit session: reading it there as well
	// would deliver the same text twice.
	handleResult, resultInSession := imeCompositionRoute(flags, armed)
	if handleResult {
		w.handleIMEResult()
		if flags&gcsCompStr == 0 {
			return
		}
	}

	w.imeMu.Lock()
	started := w.ime.ensureActive()
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	if !enabled {
		return
	}
	if started {
		w.queueIME(&Event{WindowID: w.id, Type: EventIMECompositionStart})
	}

	himc, hwnd := w.imeContext()
	if himc == 0 {
		return
	}
	defer procImmReleaseContext.Call(hwnd, himc)

	compositionText := ""
	if flags&gcsCompStr != 0 {
		compositionText = w.immString(himc, gcsCompStr)
	}
	if flags&gcsResultStr != 0 && resultInSession {
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
	state := gpucontext.IMEState{
		Composing:       true,
		CompositionText: compositionText,
		Cursor:          gpucontext.IMETextRange{Start: cursorByte, End: cursorByte},
		Selection:       gpucontext.IMETextRange{Start: selectionStart, End: selectionEnd},
	}
	if !state.IsValid() {
		return
	}
	w.queueIME(&Event{
		WindowID: w.id,
		Type:     EventIMECompositionUpdate,
		IME:      &IMEEvent{State: state},
	})
}

// handleIMEResult reads the result string of a result-only WM_IME_COMPOSITION
// and hands it to processIMEResult.
func (w *win32Window) handleIMEResult() {
	himc, hwnd := w.imeContext()
	if himc == 0 {
		return
	}
	result := w.immString(himc, gcsResultStr)
	procImmReleaseContext.Call(hwnd, himc)
	w.processIMEResult(result)
}

// processIMEResult commits a result string that arrived in a
// WM_IME_COMPOSITION carrying no preedit, exactly once.
func (w *win32Window) processIMEResult(result string) {
	if result == "" {
		return
	}

	w.imeMu.Lock()
	if w.imeDestroyed || !w.ime.enabled {
		w.imeMu.Unlock()
		return
	}
	if w.ime.consumePostEndResult(result) {
		// END delivered nothing and armed the post-END path; this is the text
		// it was waiting for (Windows Hangul sends it in a late COMPOSITION).
		w.imeMu.Unlock()
		w.finishIMECharResult()
		return
	}
	if w.ime.consumeResultEcho(result) {
		// END already read and delivered this result; the composition message
		// is its echo.
		w.imeMu.Unlock()
		return
	}
	w.ime.addResult(result)
	committed, active := w.ime.end()
	w.imeMu.Unlock()
	if !active {
		// No composition was open (a provider that reports a complete result
		// on its own): the result itself is the commit.
		committed = result
	}
	w.queueIME(&Event{
		WindowID: w.id,
		Type:     EventIMECompositionEnd,
		IME:      &IMEEvent{Committed: committed},
	})
}

// handleIMEEnd ends the composition. IMM32 may expose the committed result
// only here (Hangul sends the result in a COMPOSITION after END), so END reads
// GCS_RESULTSTR itself when nothing was collected for the session, and a late
// COMPOSITION then finds it already delivered.
func (w *win32Window) handleIMEEnd() {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	active := w.ime.active
	haveResult := w.ime.result != ""
	w.imeMu.Unlock()

	if active && !haveResult {
		if himc, hwnd := w.imeContext(); himc != 0 {
			result := w.immString(himc, gcsResultStr)
			procImmReleaseContext.Call(hwnd, himc)
			if result != "" {
				w.imeMu.Lock()
				w.ime.addResult(result)
				w.imeMu.Unlock()
			}
		}
	}

	w.imeMu.Lock()
	committed, ok := w.ime.end()
	if ok && committed == "" && !w.ime.hasPendingCharResult() {
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
	w.queueIME(&Event{
		WindowID: w.id,
		Type:     EventIMECompositionEnd,
		IME:      &IMEEvent{Committed: committed},
	})
}

func (w *win32Window) consumeIMECharResult(codeUnit uint16) bool {
	w.imeMu.Lock()
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return false
	}
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
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return false
	}
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
	if w.imeDestroyed {
		w.imeMu.Unlock()
		return
	}
	committed, ok := w.ime.finishCharResult()
	w.imeMu.Unlock()
	if !ok {
		return
	}
	if committed == "" {
		// WM_IME_ENDCOMPOSITION with no GCS_RESULTSTR and no post-END WM_CHAR
		// is the native cancellation path (Escape/focus loss), not an empty
		// commit: an empty commit has no insertion semantics and would leave
		// the preedit lifecycle un-canceled for consumers.
		w.queueIME(&Event{WindowID: w.id, Type: EventIMECanceled})
		return
	}
	w.queueIME(&Event{
		WindowID: w.id,
		Type:     EventIMECompositionEnd,
		IME:      &IMEEvent{Committed: committed},
	})
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

func (w *win32Window) queueIME(event *Event) {
	if w != nil && w.platform != nil {
		w.platform.queueEvent(*event)
	}
}

// consumeIMEUnichar reports whether a WM_UNICHAR code point is the echo of an
// IME result already delivered, and consumes it as such.
func (w *win32Window) consumeIMEUnichar(char rune) bool {
	resultUnits := utf16.Encode([]rune{char})
	if w.consumeIMECharResultUnits(resultUnits) {
		return true
	}
	for _, unit := range resultUnits {
		if !w.consumeIMECharUnit(unit) {
			return false
		}
	}
	return true
}

// imeCompositionRoute decides how a WM_IME_COMPOSITION splits between the
// result path and the preedit path. armed reports whether WM_IME_ENDCOMPOSITION
// left the post-END lifecycle open.
//
// A result is handled first when it arrives without a preedit (a standalone
// commit) or while the post-END path is armed (Hangul sends the committed text
// after END); in the second case, when the message also carries a preedit, the
// preedit still starts a new composition but the result is not read into it.
// Any other result belongs to the active composition session and is collected
// there until its end.
func imeCompositionRoute(flags uintptr, armed bool) (handleResult, resultInSession bool) {
	if flags&gcsResultStr == 0 {
		return false, false
	}
	if flags&gcsCompStr == 0 || armed {
		return true, false
	}
	return false, true
}

// imeAreaIsValid rejects rectangles that cannot be placed: negative or
// non-finite coordinates, and sub-pixel sizes are allowed (a caret may have a
// zero width).
func imeAreaIsValid(area gpucontext.IMECursorArea) bool {
	return area.X >= 0 && area.Y >= 0 && area.Width >= 0 && area.Height >= 0 &&
		!math.IsNaN(area.X) && !math.IsNaN(area.Y) && !math.IsNaN(area.Width) && !math.IsNaN(area.Height) &&
		!math.IsInf(area.X, 0) && !math.IsInf(area.Y, 0) && !math.IsInf(area.Width, 0) && !math.IsInf(area.Height, 0)
}

var _ gpucontext.IMEController = (*win32Window)(nil)
