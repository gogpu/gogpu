package gogpu

import (
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// SetIMEPosition updates the legacy IME candidate position. Coordinates use
// the legacy gpucontext contract (screen pixels relative to the window).
// Widgets should prefer SetIMECursorArea when the v2 contract is available.
func (a *App) SetIMEPosition(x, y int) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imePosition.x, a.imePosition.y = x, y
	a.imePositionSet = true
	window := a.platWindow
	a.imeMu.Unlock()
	if controller, ok := window.(gpucontext.IMEController); ok {
		controller.SetIMEPosition(x, y)
	}
}

// SetIMEEnabled enables or disables the native input method for the active
// window. IME starts disabled by default and is normally enabled by a focused
// text widget. Disabling cancels an active composition before notifying
// consumers through the optional IME event callbacks.
func (a *App) SetIMEEnabled(enabled bool) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imeEnabled = enabled
	a.imeConfigured = true
	window := a.platWindow
	surroundingSet := a.imeSurroundingSet
	surrounding := a.imeSurrounding
	a.imeMu.Unlock()
	if controller, ok := window.(gpucontext.IMEController); ok {
		controller.SetIMEEnabled(enabled)
	}
	if enabled && surroundingSet {
		if controller, ok := window.(gpucontext.IMEControllerV2); ok {
			// The native backend intentionally drops surrounding text while IME
			// is disabled, so restore the explicit app configuration only after
			// enabling it again.
			controller.SetIMESurroundingText(surrounding)
		}
	}
}

// SetIMECursorArea updates the logical-DIP caret rectangle used to place the
// native candidate window. The active backend converts this rectangle to its
// native coordinate system.
func (a *App) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imeCursorArea = area
	a.imeAreaSet = true
	window := a.platWindow
	a.imeMu.Unlock()
	if controller, ok := window.(gpucontext.IMEControllerV2); ok {
		controller.SetIMECursorArea(area)
	}
}

// SetIMEContentType supplies advisory content purpose and input hints to the
// native IME. Unsupported hints are ignored by the platform backend.
func (a *App) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imePurpose, a.imeHints = purpose, hints
	a.imeContentSet = true
	window := a.platWindow
	a.imeMu.Unlock()
	if controller, ok := window.(gpucontext.IMEControllerV2); ok {
		controller.SetIMEContentType(purpose, hints)
	}
}

// SetIMESurroundingText updates the UTF-8 context available to an IME. The
// native backend must not retain or expose this value while IME is disabled.
func (a *App) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if a == nil || !text.IsValid() {
		return
	}
	a.imeMu.Lock()
	a.imeSurrounding = text
	a.imeSurroundingSet = true
	window := a.platWindow
	a.imeMu.Unlock()
	if controller, ok := window.(gpucontext.IMEControllerV2); ok {
		controller.SetIMESurroundingText(text)
	}
}

// CancelIME cancels an active composition without committing its preedit.
func (a *App) CancelIME() {
	if a == nil {
		return
	}
	if controller, ok := a.platWindow.(gpucontext.IMEControllerV2); ok {
		controller.CancelIME()
	}
}

// IMEController returns the application's legacy IME controller. App itself
// implements both the original and versioned controller contracts, so callers
// may retain this value across window initialization.
func (a *App) IMEController() gpucontext.IMEController {
	return a
}

// OnIMECompositionUpdateV2 registers a full-fidelity composition callback.
func (a *App) OnIMECompositionUpdateV2(fn func(gpucontext.IMEComposition)) {
	a.EventSource().(*eventSourceAdapter).OnIMECompositionUpdateV2(fn)
}

// OnIMECanceled registers an IME cancellation callback.
func (a *App) OnIMECanceled(fn func()) {
	a.EventSource().(*eventSourceAdapter).OnIMECanceled(fn)
}

// OnIMEDisabled registers a callback delivered when text input is disabled.
func (a *App) OnIMEDisabled(fn func()) {
	a.EventSource().(*eventSourceAdapter).OnIMEDisabled(fn)
}

// OnIMEDeleteSurrounding registers a callback for delete-surrounding requests.
func (a *App) OnIMEDeleteSurrounding(fn func(gpucontext.IMEDeleteSurroundingEvent)) {
	a.EventSource().(*eventSourceAdapter).OnIMEDeleteSurrounding(fn)
}

// IMECapabilities advertises the capabilities of the active platform. Before
// Run, the platform package returns a deterministic build-target default.
func (a *App) IMECapabilities() gpucontext.IMECapabilities {
	if a != nil {
		if provider, ok := a.platWindow.(gpucontext.IMECapabilityProviderV2); ok {
			return provider.IMECapabilities()
		}
	}
	return platform.DefaultIMECapabilities()
}

// applyIMEControllerState replays configuration made before Run onto the
// freshly-created native window. It intentionally does not call
// SetIMEEnabled(false) for an untouched App: native IME is already disabled
// by the backend and no spurious OnIMEDisabled event should be emitted.
func (a *App) applyIMEControllerState(window interface{}) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	configured := a.imeConfigured
	enabled := a.imeEnabled
	positionSet := a.imePositionSet
	position := a.imePosition
	area := a.imeCursorArea
	areaSet := a.imeAreaSet
	purpose, hints := a.imePurpose, a.imeHints
	contentSet := a.imeContentSet
	surrounding := a.imeSurrounding
	surroundingSet := a.imeSurroundingSet
	a.imeMu.Unlock()

	legacy, _ := window.(gpucontext.IMEController)
	v2, _ := window.(gpucontext.IMEControllerV2)
	if legacy != nil && configured {
		legacy.SetIMEEnabled(enabled)
	}
	if legacy != nil && positionSet {
		legacy.SetIMEPosition(position.x, position.y)
	}
	if v2 != nil {
		if areaSet {
			v2.SetIMECursorArea(area)
		}
		if contentSet {
			v2.SetIMEContentType(purpose, hints)
		}
		if surroundingSet {
			v2.SetIMESurroundingText(surrounding)
		}
	}
}

var (
	_ gpucontext.IMEControllerV2         = (*App)(nil)
	_ gpucontext.IMEEventSourceV2        = (*App)(nil)
	_ gpucontext.IMECapabilityProviderV2 = (*App)(nil)
)
