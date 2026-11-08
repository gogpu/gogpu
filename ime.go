package gogpu

import (
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// The App implements gpucontext.IMEController. Configuration made before Run
// is retained and replayed onto the native window when it is created. Every
// method is safe to call from any goroutine, including from a draw callback:
// the platform marshals native IME calls to the window's own thread and never
// blocks the caller (gpucontext.IMEController threading contract).

// SetIMEEnabled enables or disables the native input method for the active
// window. IME starts disabled and is normally enabled by a focused text
// widget. Disabling cancels an active composition first and clears the
// surrounding text retained for the IME.
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

	controller, ok := window.(gpucontext.IMEController)
	if !ok {
		return
	}
	controller.SetIMEEnabled(enabled)
	if enabled && surroundingSet {
		// Backends drop surrounding text while IME is disabled, so the explicit
		// configuration is restored only after enabling it again.
		controller.SetIMESurroundingText(surrounding)
	}
}

// SetIMECursorArea updates the logical-DIP caret rectangle used to place the
// native candidate window. The backend converts it to native coordinates.
func (a *App) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imeCursorArea = area
	a.imeAreaSet = true
	window := a.platWindow
	a.imeMu.Unlock()

	if controller, ok := window.(gpucontext.IMEController); ok {
		controller.SetIMECursorArea(area)
	}
}

// SetIMEContentType supplies an advisory content purpose and set of hints.
// A backend ignores a value its platform cannot express.
func (a *App) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imePurpose, a.imeHints = purpose, hints
	a.imeContentSet = true
	window := a.platWindow
	a.imeMu.Unlock()

	if controller, ok := window.(gpucontext.IMEController); ok {
		controller.SetIMEContentType(purpose, hints)
	}
}

// SetIMESurroundingText supplies the text around the insertion point for
// context-sensitive candidates. An invalid value is ignored. The backend
// never retains it while IME is disabled.
func (a *App) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if a == nil || !text.IsValid() {
		return
	}
	a.imeMu.Lock()
	a.imeSurrounding = text
	a.imeSurroundingSet = true
	window := a.platWindow
	a.imeMu.Unlock()

	if controller, ok := window.(gpucontext.IMEController); ok {
		controller.SetIMESurroundingText(text)
	}
}

// CancelIME cancels an active composition without committing its preedit.
func (a *App) CancelIME() {
	if a == nil {
		return
	}
	if controller, ok := a.platWindow.(gpucontext.IMEController); ok {
		controller.CancelIME()
	}
}

// IMECapabilities reports which optional IME operations the active platform
// performs. Before Run it returns the build target's default.
func (a *App) IMECapabilities() gpucontext.IMECapabilities {
	if a != nil {
		if controller, ok := a.platWindow.(gpucontext.IMEController); ok {
			return controller.IMECapabilities()
		}
	}
	return platform.DefaultIMECapabilities()
}

// applyIMEControllerState replays configuration made before Run onto the
// freshly created native window. An untouched App is left alone: the backend
// starts with IME disabled and must not emit a spurious disable event.
func (a *App) applyIMEControllerState(window platform.PlatformWindow) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	configured := a.imeConfigured
	enabled := a.imeEnabled
	area := a.imeCursorArea
	areaSet := a.imeAreaSet
	purpose, hints := a.imePurpose, a.imeHints
	contentSet := a.imeContentSet
	surrounding := a.imeSurrounding
	surroundingSet := a.imeSurroundingSet
	a.imeMu.Unlock()

	controller, ok := window.(gpucontext.IMEController)
	if !ok {
		return
	}
	if configured {
		controller.SetIMEEnabled(enabled)
	}
	if areaSet {
		controller.SetIMECursorArea(area)
	}
	if contentSet {
		controller.SetIMEContentType(purpose, hints)
	}
	if surroundingSet {
		controller.SetIMESurroundingText(surrounding)
	}
}

var _ gpucontext.IMEController = (*App)(nil)
