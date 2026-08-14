//go:build linux

package platform

import (
	"math"

	"github.com/gogpu/gpucontext"

	"github.com/gogpu/gogpu/internal/platform/wayland"
)

// IMECapabilities reports the optional operations implemented by the
// zwp_text_input_v3 backend. Support is dynamic because the protocol global
// is optional and some compositors expose only the base wl_keyboard protocol.
func (w *waylandPlatformWindow) IMECapabilities() gpucontext.IMECapabilities {
	if w == nil {
		return gpucontext.IMECapabilities{}
	}
	h := w.libwayland()
	if h == nil || !h.HasTextInput() {
		return gpucontext.IMECapabilities{Version: gpucontext.IMEContractVersion}
	}
	return gpucontext.IMECapabilities{
		Version: gpucontext.IMEContractVersion,
		Features: gpucontext.IMECapabilityComposition |
			gpucontext.IMECapabilityCommit |
			gpucontext.IMECapabilityCancel |
			gpucontext.IMECapabilityDisabled |
			gpucontext.IMECapabilityDeleteSurrounding |
			gpucontext.IMECapabilityCursorArea |
			gpucontext.IMECapabilitySurroundingText |
			gpucontext.IMECapabilityContentPurpose |
			gpucontext.IMECapabilityContentHints,
	}
}

func (w *waylandPlatformWindow) imeWindow() *waylandWindow {
	if w == nil {
		return nil
	}
	if w.secondary != nil {
		return &w.secondary.state
	}
	return w.platform.primary
}

// SetIMEPosition preserves the legacy point-only API by translating it to a
// zero-size logical-DIP cursor rectangle.
func (w *waylandPlatformWindow) SetIMEPosition(x, y int) {
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: float64(x), Y: float64(y)})
}

func (w *waylandPlatformWindow) SetIMEEnabled(enabled bool) {
	wp := w.imeWindow()
	h := w.libwayland()
	if wp == nil || h == nil || !h.HasTextInput() {
		return
	}

	wp.imeMu.Lock()
	wasEnabled := wp.imeEnabled
	canceled := wp.imeComposing && !enabled
	if enabled {
		wp.imeEnabled = true
	} else {
		wp.imeEnabled = false
		wp.imeComposing = false
		wp.imeReplay = false
		wp.imeNeedsDisable = false
		// Surrounding context is sensitive and must not survive disable.
		wp.imeSurrounding = gpucontext.IMESurroundingText{}
		wp.imeSurroundingSet = false
	}
	wp.imeMu.Unlock()

	h.SetTextInputEnabled(enabled)
	if enabled {
		wp.replayWaylandIMEState(h)
	}
	if canceled {
		wp.queueEvent(Event{Type: EventIMECanceled})
	}
	if !enabled && wasEnabled {
		wp.queueEvent(Event{Type: EventIMEDisabled})
	}
}

func (w *waylandPlatformWindow) SetIMECursorArea(area gpucontext.IMECursorArea) {
	wp := w.imeWindow()
	if wp == nil || !validWaylandIMEArea(area) {
		return
	}
	wp.imeMu.Lock()
	wp.imeArea = area
	wp.imeAreaSet = true
	enabled := wp.imeEnabled
	wp.imeMu.Unlock()
	if enabled {
		if h := w.libwayland(); h != nil {
			h.SetTextInputCursorArea(area)
		}
	}
}

func (w *waylandPlatformWindow) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	wp := w.imeWindow()
	if wp == nil {
		return
	}
	wp.imeMu.Lock()
	wp.imePurpose = purpose
	wp.imeHints = hints
	enabled := wp.imeEnabled
	wp.imeMu.Unlock()
	if enabled {
		if h := w.libwayland(); h != nil {
			h.SetTextInputContentType(purpose, hints)
		}
	}
}

func (w *waylandPlatformWindow) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if !text.IsValid() {
		return
	}
	wp := w.imeWindow()
	if wp == nil {
		return
	}
	wp.imeMu.Lock()
	if !wp.imeEnabled {
		wp.imeMu.Unlock()
		return
	}
	wp.imeSurrounding = text
	wp.imeSurroundingSet = true
	wp.imeMu.Unlock()
	if h := w.libwayland(); h != nil {
		h.SetTextInputSurrounding(text)
	}
}

func (w *waylandPlatformWindow) CancelIME() {
	wp := w.imeWindow()
	h := w.libwayland()
	if wp == nil || h == nil || !h.HasTextInput() {
		return
	}
	wp.imeMu.Lock()
	if !wp.imeEnabled {
		wp.imeMu.Unlock()
		return
	}
	wasComposing := wp.imeComposing
	wp.imeComposing = false
	wp.imeReplay = false
	area, areaSet := wp.imeArea, wp.imeAreaSet
	purpose, hints := wp.imePurpose, wp.imeHints
	surrounding, surroundingSet := wp.imeSurrounding, wp.imeSurroundingSet
	wp.imeMu.Unlock()
	if !wasComposing {
		return
	}

	// v3 has no cancel request. Disable/enable is the protocol-defined way to
	// terminate the active state; replay all non-sensitive state immediately.
	h.SetTextInputEnabled(false)
	h.SetTextInputEnabled(true)
	if areaSet {
		h.SetTextInputCursorArea(area)
	}
	h.SetTextInputContentType(purpose, hints)
	if surroundingSet {
		h.SetTextInputSurrounding(surrounding)
	}
	wp.queueEvent(Event{Type: EventIMECanceled})
}

func validWaylandIMEArea(area gpucontext.IMECursorArea) bool {
	for _, value := range []float64{area.X, area.Y, area.Width, area.Height} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) || value > math.MaxInt32 {
			return false
		}
	}
	return true
}

func (w *waylandWindow) imeShouldFilterText() bool {
	if w == nil {
		return false
	}
	w.imeMu.Lock()
	enabled := w.imeEnabled
	w.imeMu.Unlock()
	return enabled
}

// replayWaylandIME is called after dispatching text-input enter callbacks,
// outside the goffi callback. The protocol invalidates state after enter.
func (w *waylandWindow) replayWaylandIME(h *wayland.LibwaylandHandle) {
	if h == nil {
		return
	}
	w.imeMu.Lock()
	needsDisable := w.imeNeedsDisable
	w.imeNeedsDisable = false
	replay := w.imeReplay && w.imeEnabled
	w.imeReplay = false
	w.imeMu.Unlock()
	if needsDisable {
		h.SetTextInputEnabled(false)
	}
	if replay {
		w.replayWaylandIMEState(h)
	}
}

func (w *waylandWindow) replayWaylandIMEState(h *wayland.LibwaylandHandle) {
	if w == nil || h == nil {
		return
	}
	w.imeMu.Lock()
	enabled := w.imeEnabled
	area, areaSet := w.imeArea, w.imeAreaSet
	purpose, hints := w.imePurpose, w.imeHints
	surrounding, surroundingSet := w.imeSurrounding, w.imeSurroundingSet
	w.imeMu.Unlock()
	if !enabled {
		return
	}
	h.SetTextInputEnabled(true)
	if areaSet {
		h.SetTextInputCursorArea(area)
	}
	h.SetTextInputContentType(purpose, hints)
	if surroundingSet {
		h.SetTextInputSurrounding(surrounding)
	}
}

func (w *waylandWindow) handleWaylandTextInputDone(update wayland.TextInputUpdate) {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	if !w.imeEnabled {
		w.imeMu.Unlock()
		return
	}
	composing := w.imeComposing
	var events []Event

	if update.HasPreedit {
		composition := gpucontext.IMEComposition{
			CompositionText: update.PreeditText,
			CursorBegin:     int(update.CursorBegin),
			CursorEnd:       int(update.CursorEnd),
			SelectionStart:  0,
			SelectionEnd:    0,
		}
		if composition.IsValid() && (composition.CompositionText != "" || composing) {
			if !composing {
				events = append(events, Event{Type: EventIMECompositionStart})
				composing = true
			}
			events = append(events, Event{Type: EventIMECompositionUpdate, IMEComposition: composition})
			if composition.CompositionText == "" && !update.HasCommit {
				events = append(events, Event{Type: EventIMECanceled})
				composing = false
			}
		}
	}

	if update.HasDelete {
		deleteEvent := gpucontext.IMEDeleteSurroundingEvent{
			Before: int(update.DeleteBefore),
			After:  int(update.DeleteAfter),
		}
		if deleteEvent.IsValid() {
			events = append(events, Event{Type: EventIMEDeleteSurrounding, IMEDelete: deleteEvent})
		}
	}

	if update.HasCommit && (composing || update.CommitText != "") {
		events = append(events, Event{Type: EventIMECompositionEnd, IMECommitted: update.CommitText})
		composing = false
	}
	w.imeComposing = composing
	w.imeMu.Unlock()

	for _, event := range events {
		w.queueEvent(event)
	}
}

var _ gpucontext.IMEControllerV2 = (*waylandPlatformWindow)(nil)
var _ gpucontext.IMECapabilityProviderV2 = (*waylandPlatformWindow)(nil)
