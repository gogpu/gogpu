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
//
// The window the controller drives is a.imeTarget, published and cleared under
// imeMu. It is deliberately not a.platWindow: that field is written by the
// main thread without a lock, and the IME methods are called from any
// goroutine (LEAD-023 F3).

// SetIMEEnabled enables or disables the native input method for the active
// window. Until the first call the window keeps the platform's own IME
// handling, where the IME draws its composition and the committed text
// arrives as ordinary text input; the first call hands composition to the app,
// which then receives the IME lifecycle events. A focused text widget
// normally enables it and a non-text one disables it. Disabling cancels an
// active composition first and clears the surrounding text retained for the
// IME.
func (a *App) SetIMEEnabled(enabled bool) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imeEnabled = enabled
	a.imeConfigured = true
	window := a.imeTarget
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
	window := a.imeTarget
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
	window := a.imeTarget
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
	window := a.imeTarget
	a.imeMu.Unlock()

	if controller, ok := window.(gpucontext.IMEController); ok {
		controller.SetIMESurroundingText(text)
	}
}

// CancelIME cancels an active composition without committing its preedit.
func (a *App) CancelIME() {
	if controller, ok := a.imeController(); ok {
		controller.CancelIME()
	}
}

// IMECapabilities reports which optional IME operations the active platform
// performs. Before Run it returns the build target's default.
func (a *App) IMECapabilities() gpucontext.IMECapabilities {
	if controller, ok := a.imeController(); ok {
		return controller.IMECapabilities()
	}
	return platform.DefaultIMECapabilities()
}

// imeController returns the current IME target's controller, read under
// imeMu.
func (a *App) imeController() (gpucontext.IMEController, bool) {
	if a == nil {
		return nil, false
	}
	a.imeMu.Lock()
	window := a.imeTarget
	a.imeMu.Unlock()
	controller, ok := window.(gpucontext.IMEController)
	return controller, ok
}

// attachIMEWindow makes window the IME target and replays the configuration
// made so far onto it. Publishing the target and replaying happen under one
// hold of imeMu, so a concurrent IME call either lands in the replayed state
// or reaches the window itself after it, never neither. A window's controller
// methods only post to its thread and never call back into the App, so
// holding the App's imeMu here cannot deadlock. An untouched App is left alone: the window keeps the
// platform's own IME handling and must not get a spurious disable.
func (a *App) attachIMEWindow(window platform.PlatformWindow) {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	defer a.imeMu.Unlock()
	a.imeTarget = window

	controller, ok := window.(gpucontext.IMEController)
	if !ok {
		return
	}
	if a.imeConfigured {
		controller.SetIMEEnabled(a.imeEnabled)
	}
	if a.imeAreaSet {
		controller.SetIMECursorArea(a.imeCursorArea)
	}
	if a.imeContentSet {
		controller.SetIMEContentType(a.imePurpose, a.imeHints)
	}
	if a.imeSurroundingSet {
		controller.SetIMESurroundingText(a.imeSurrounding)
	}
}

// detachIMEWindow clears the IME target when its window closes. Calls that
// already read the old target reach a destroyed window, which ignores them.
func (a *App) detachIMEWindow() {
	if a == nil {
		return
	}
	a.imeMu.Lock()
	a.imeTarget = nil
	a.imeMu.Unlock()
}

var _ gpucontext.IMEController = (*App)(nil)
