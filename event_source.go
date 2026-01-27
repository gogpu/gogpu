package gogpu

import (
	"github.com/gogpu/gpucontext"
)

// eventSourceAdapter bridges gogpu to gpucontext.EventSource interface.
// This enables UI frameworks to receive input events from gogpu.
type eventSourceAdapter struct {
	app *App

	// Registered callbacks
	onKeyPress             func(gpucontext.Key, gpucontext.Modifiers)
	onKeyRelease           func(gpucontext.Key, gpucontext.Modifiers)
	onTextInput            func(string)
	onMouseMove            func(float64, float64)
	onMousePress           func(gpucontext.MouseButton, float64, float64)
	onMouseRelease         func(gpucontext.MouseButton, float64, float64)
	onScroll               func(float64, float64)
	onResize               func(int, int)
	onFocus                func(bool)
	onIMECompositionStart  func()
	onIMECompositionUpdate func(gpucontext.IMEState)
	onIMECompositionEnd    func(string)
}

// OnKeyPress registers a callback for key press events.
func (e *eventSourceAdapter) OnKeyPress(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	e.onKeyPress = fn
}

// OnKeyRelease registers a callback for key release events.
func (e *eventSourceAdapter) OnKeyRelease(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	e.onKeyRelease = fn
}

// OnTextInput registers a callback for text input events.
func (e *eventSourceAdapter) OnTextInput(fn func(string)) {
	e.onTextInput = fn
}

// OnMouseMove registers a callback for mouse movement.
func (e *eventSourceAdapter) OnMouseMove(fn func(float64, float64)) {
	e.onMouseMove = fn
}

// OnMousePress registers a callback for mouse button press.
func (e *eventSourceAdapter) OnMousePress(fn func(gpucontext.MouseButton, float64, float64)) {
	e.onMousePress = fn
}

// OnMouseRelease registers a callback for mouse button release.
func (e *eventSourceAdapter) OnMouseRelease(fn func(gpucontext.MouseButton, float64, float64)) {
	e.onMouseRelease = fn
}

// OnScroll registers a callback for scroll wheel events.
func (e *eventSourceAdapter) OnScroll(fn func(float64, float64)) {
	e.onScroll = fn
}

// OnResize registers a callback for window resize.
func (e *eventSourceAdapter) OnResize(fn func(int, int)) {
	e.onResize = fn
}

// OnFocus registers a callback for focus change.
func (e *eventSourceAdapter) OnFocus(fn func(bool)) {
	e.onFocus = fn
}

// OnIMECompositionStart registers a callback for IME composition start.
func (e *eventSourceAdapter) OnIMECompositionStart(fn func()) {
	e.onIMECompositionStart = fn
}

// OnIMECompositionUpdate registers a callback for IME composition updates.
func (e *eventSourceAdapter) OnIMECompositionUpdate(fn func(gpucontext.IMEState)) {
	e.onIMECompositionUpdate = fn
}

// OnIMECompositionEnd registers a callback for IME composition end.
func (e *eventSourceAdapter) OnIMECompositionEnd(fn func(string)) {
	e.onIMECompositionEnd = fn
}

// Ensure eventSourceAdapter implements gpucontext.EventSource.
var _ gpucontext.EventSource = (*eventSourceAdapter)(nil)

// EventSource returns a gpucontext.EventSource for use with UI frameworks.
// This enables UI frameworks to receive input events from the gogpu application.
//
// Example:
//
//	app := gogpu.NewApp(gogpu.Config{Title: "My App"})
//
//	app.OnDraw(func(ctx *gogpu.Context) {
//	    // Get event source for UI
//	    events := app.EventSource()
//	    events.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
//	        // Handle key press
//	    })
//	})
//
// Note: EventSource can be called before Run(), but callbacks will only
// be invoked once the main loop starts.
func (a *App) EventSource() gpucontext.EventSource {
	if a.eventSource == nil {
		a.eventSource = &eventSourceAdapter{app: a}
	}
	return a.eventSource
}

// dispatchKeyPress dispatches a key press event to registered callbacks.
func (e *eventSourceAdapter) dispatchKeyPress(key gpucontext.Key, mods gpucontext.Modifiers) {
	if e.onKeyPress != nil {
		e.onKeyPress(key, mods)
	}
}

// dispatchKeyRelease dispatches a key release event to registered callbacks.
func (e *eventSourceAdapter) dispatchKeyRelease(key gpucontext.Key, mods gpucontext.Modifiers) {
	if e.onKeyRelease != nil {
		e.onKeyRelease(key, mods)
	}
}

// dispatchTextInput dispatches a text input event to registered callbacks.
func (e *eventSourceAdapter) dispatchTextInput(text string) {
	if e.onTextInput != nil {
		e.onTextInput(text)
	}
}

// dispatchMouseMove dispatches a mouse move event to registered callbacks.
func (e *eventSourceAdapter) dispatchMouseMove(x, y float64) {
	if e.onMouseMove != nil {
		e.onMouseMove(x, y)
	}
}

// dispatchMousePress dispatches a mouse press event to registered callbacks.
func (e *eventSourceAdapter) dispatchMousePress(button gpucontext.MouseButton, x, y float64) {
	if e.onMousePress != nil {
		e.onMousePress(button, x, y)
	}
}

// dispatchMouseRelease dispatches a mouse release event to registered callbacks.
func (e *eventSourceAdapter) dispatchMouseRelease(button gpucontext.MouseButton, x, y float64) {
	if e.onMouseRelease != nil {
		e.onMouseRelease(button, x, y)
	}
}

// dispatchScroll dispatches a scroll event to registered callbacks.
func (e *eventSourceAdapter) dispatchScroll(dx, dy float64) {
	if e.onScroll != nil {
		e.onScroll(dx, dy)
	}
}

// dispatchResize dispatches a resize event to registered callbacks.
func (e *eventSourceAdapter) dispatchResize(width, height int) {
	if e.onResize != nil {
		e.onResize(width, height)
	}
}

// dispatchFocus dispatches a focus event to registered callbacks.
func (e *eventSourceAdapter) dispatchFocus(focused bool) {
	if e.onFocus != nil {
		e.onFocus(focused)
	}
}
