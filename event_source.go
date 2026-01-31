package gogpu

import (
	"github.com/gogpu/gpucontext"
)

// eventSourceAdapter bridges gogpu to gpucontext.EventSource interface.
// This enables UI frameworks to receive input events from gogpu.
//
// It also implements PointerEventSource and ScrollEventSource for W3C-compliant
// unified pointer events and detailed scroll events.
type eventSourceAdapter struct {
	app *App

	// Registered callbacks for EventSource
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

	// Registered callbacks for PointerEventSource
	onPointer func(gpucontext.PointerEvent)

	// Registered callbacks for ScrollEventSource
	onScrollEvent func(gpucontext.ScrollEvent)
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

// OnPointer registers a callback for unified pointer events.
// This provides W3C Pointer Events Level 3 compliant input handling,
// unifying mouse, touch, and pen input into a single event stream.
//
// Pointer events are delivered in order:
//
//	PointerEnter -> PointerDown -> PointerMove* -> PointerUp/PointerCancel -> PointerLeave
//
// See gpucontext.PointerEvent for event details.
func (e *eventSourceAdapter) OnPointer(fn func(gpucontext.PointerEvent)) {
	e.onPointer = fn
}

// OnScrollEvent registers a callback for detailed scroll events.
// This provides scroll events with position, delta mode, and timing information
// beyond what the basic OnScroll provides.
//
// Use this when you need:
//   - Pointer position at scroll time
//   - Delta mode (pixels vs lines vs pages)
//   - Timestamps for smooth scrolling
//
// See gpucontext.ScrollEvent for event details.
func (e *eventSourceAdapter) OnScrollEvent(fn func(gpucontext.ScrollEvent)) {
	e.onScrollEvent = fn
}

// Ensure eventSourceAdapter implements gpucontext.EventSource.
var _ gpucontext.EventSource = (*eventSourceAdapter)(nil)

// Ensure eventSourceAdapter implements gpucontext.PointerEventSource.
var _ gpucontext.PointerEventSource = (*eventSourceAdapter)(nil)

// Ensure eventSourceAdapter implements gpucontext.ScrollEventSource.
var _ gpucontext.ScrollEventSource = (*eventSourceAdapter)(nil)

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

// dispatchPointerEvent dispatches a pointer event to registered callbacks.
// It also dispatches to legacy mouse handlers for backward compatibility.
func (e *eventSourceAdapter) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	// Dispatch to new pointer event handler
	if e.onPointer != nil {
		e.onPointer(ev)
	}

	// Also dispatch to legacy mouse handlers for backward compatibility
	// Only dispatch for mouse-type pointers to avoid duplicates from touch/pen
	if ev.PointerType == gpucontext.PointerTypeMouse {
		switch ev.Type {
		case gpucontext.PointerMove:
			if e.onMouseMove != nil {
				e.onMouseMove(ev.X, ev.Y)
			}
		case gpucontext.PointerDown:
			if e.onMousePress != nil {
				button := buttonToMouseButton(ev.Button)
				e.onMousePress(button, ev.X, ev.Y)
			}
		case gpucontext.PointerUp:
			if e.onMouseRelease != nil {
				button := buttonToMouseButton(ev.Button)
				e.onMouseRelease(button, ev.X, ev.Y)
			}
		}
	}
}

// dispatchScrollEventDetailed dispatches a detailed scroll event to registered callbacks.
// It also dispatches to the legacy scroll handler for backward compatibility.
func (e *eventSourceAdapter) dispatchScrollEventDetailed(ev gpucontext.ScrollEvent) {
	// Dispatch to new scroll event handler
	if e.onScrollEvent != nil {
		e.onScrollEvent(ev)
	}

	// Also dispatch to legacy scroll handler for backward compatibility
	if e.onScroll != nil {
		e.onScroll(ev.DeltaX, ev.DeltaY)
	}
}

// buttonToMouseButton converts gpucontext.Button to gpucontext.MouseButton.
// This is used for backward compatibility with legacy mouse handlers.
func buttonToMouseButton(b gpucontext.Button) gpucontext.MouseButton {
	switch b {
	case gpucontext.ButtonLeft:
		return gpucontext.MouseButtonLeft
	case gpucontext.ButtonRight:
		return gpucontext.MouseButtonRight
	case gpucontext.ButtonMiddle:
		return gpucontext.MouseButtonMiddle
	case gpucontext.ButtonX1:
		return gpucontext.MouseButton4
	case gpucontext.ButtonX2:
		return gpucontext.MouseButton5
	default:
		return gpucontext.MouseButtonLeft
	}
}
