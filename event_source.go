package gogpu

import (
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
)

// eventSourceAdapter bridges gogpu to gpucontext.EventSource interface.
// This enables UI frameworks to receive input events from gogpu.
//
// It also implements PointerEventSource, ScrollEventSource, and GestureEventSource
// for W3C-compliant unified pointer events, detailed scroll events, and Vello-style
// gesture events.
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

	// Registered callbacks for GestureEventSource
	onGesture func(gpucontext.GestureEvent)

	// Gesture recognizer for computing gesture deltas from pointer events
	gestureRecognizer *GestureRecognizer
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

// OnGesture registers a callback for gesture events.
// This provides Vello-style per-frame gesture recognition for multi-touch
// interactions like pinch-to-zoom, rotation, and panning.
//
// Gesture events are computed once per frame from accumulated pointer events,
// providing smooth, predictable gesture values without jitter.
//
// Use this when you need:
//   - Pinch-to-zoom (ZoomDelta)
//   - Two-finger rotation (RotationDelta)
//   - Two-finger pan (TranslationDelta)
//   - Pinch type classification (horizontal/vertical/proportional)
//
// See gpucontext.GestureEvent for event details.
func (e *eventSourceAdapter) OnGesture(fn func(gpucontext.GestureEvent)) {
	e.onGesture = fn
	// Initialize gesture recognizer on first registration
	if e.gestureRecognizer == nil {
		e.gestureRecognizer = NewGestureRecognizer()
	}
}

// Ensure eventSourceAdapter implements gpucontext.EventSource.
var _ gpucontext.EventSource = (*eventSourceAdapter)(nil)

// Ensure eventSourceAdapter implements gpucontext.PointerEventSource.
var _ gpucontext.PointerEventSource = (*eventSourceAdapter)(nil)

// Ensure eventSourceAdapter implements gpucontext.ScrollEventSource.
var _ gpucontext.ScrollEventSource = (*eventSourceAdapter)(nil)

// Ensure eventSourceAdapter implements gpucontext.GestureEventSource.
var _ gpucontext.GestureEventSource = (*eventSourceAdapter)(nil)

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

// Input returns the input state for Ebiten-style polling.
// This enables game-loop style input handling with KeyPressed, KeyJustPressed, etc.
//
// Example:
//
//	app := gogpu.NewApp(gogpu.Config{Title: "My Game"})
//
//	app.OnUpdate(func(dt float64) {
//	    inp := app.Input()
//
//	    // Check if Space was just pressed this frame
//	    if inp.Keyboard().JustPressed(input.KeySpace) {
//	        player.Jump()
//	    }
//
//	    // Check if Left is held down
//	    if inp.Keyboard().Pressed(input.KeyLeft) {
//	        player.MoveLeft(dt)
//	    }
//	})
//
// Note: Input state is automatically updated each frame. The "JustPressed"
// and "JustReleased" methods work correctly across frames.
// All methods are thread-safe.
func (a *App) Input() *input.State {
	if a.inputState == nil {
		a.inputState = input.New()
	}
	return a.inputState
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
// It also dispatches to legacy mouse handlers for backward compatibility,
// and feeds the gesture recognizer for multi-touch gesture computation.
func (e *eventSourceAdapter) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	// Dispatch to new pointer event handler
	if e.onPointer != nil {
		e.onPointer(ev)
	}

	// Feed gesture recognizer if initialized
	if e.gestureRecognizer != nil {
		e.gestureRecognizer.HandlePointer(ev)
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

// dispatchEndFrame dispatches end-of-frame events like gestures.
// This should be called at the end of each frame after all pointer events
// have been processed.
func (e *eventSourceAdapter) dispatchEndFrame() {
	// Compute and dispatch gesture event if recognizer is active
	if e.gestureRecognizer != nil && e.onGesture != nil {
		gesture := e.gestureRecognizer.EndFrame()
		// Only dispatch if there are enough pointers for a gesture
		if gesture.NumPointers >= 2 {
			e.onGesture(gesture)
		}
	}
}

// resetGestureRecognizer resets the gesture recognizer state.
// Call this when gestures should be canceled (e.g., on window blur).
//
//nolint:unused // Will be called by platform handlers in EVENT-006
func (e *eventSourceAdapter) resetGestureRecognizer() {
	if e.gestureRecognizer != nil {
		e.gestureRecognizer.Reset()
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
