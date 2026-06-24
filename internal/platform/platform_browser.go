//go:build js && wasm

package platform

import (
	"fmt"
	"syscall/js"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gpucontext"
)

// browserPlatform implements PlatformManager for browser/WASM.
// The browser manages its own event loop — we integrate via addEventListener
// callbacks and requestAnimationFrame for rendering.
type browserPlatform struct {
	events *eventqueue.Queue[Event]
	window *browserWindow
}

func newPlatformManager() PlatformManager {
	return &browserPlatform{
		events: eventqueue.New[Event](eventqueue.DefaultCapacity),
	}
}

// Init is a no-op on browser — the DOM is always available.
func (p *browserPlatform) Init() error {
	return nil
}

// CreateWindow creates a browserWindow backed by a <canvas> element.
// If no <canvas> exists in the DOM, one is created and appended to the body.
func (p *browserPlatform) CreateWindow(config Config) (PlatformWindow, error) {
	doc := js.Global().Get("document")

	// Find or create a <canvas> element.
	canvas := doc.Call("querySelector", "canvas")
	if canvas.IsNull() || canvas.IsUndefined() {
		canvas = doc.Call("createElement", "canvas")
		doc.Get("body").Call("appendChild", canvas)
	}

	// Apply configuration to the canvas.
	if config.Width > 0 {
		canvas.Set("width", config.Width)
		canvas.Get("style").Set("width", fmt.Sprintf("%dpx", config.Width))
	}
	if config.Height > 0 {
		canvas.Set("height", config.Height)
		canvas.Get("style").Set("height", fmt.Sprintf("%dpx", config.Height))
	}
	if config.Title != "" {
		doc.Set("title", config.Title)
	}

	w := &browserWindow{
		id:     NewWindowID(),
		canvas: canvas,
	}
	w.registerEventListeners(p)

	p.window = w
	return w, nil
}

// PollEvents returns the next pending event, or EventNone if the queue is empty.
func (p *browserPlatform) PollEvents() Event {
	if e, ok := p.events.Pop(); ok {
		return e
	}
	return Event{Type: EventNone}
}

// WaitEvents blocks until at least one event is available.
// On browser this is a no-op — the main loop uses requestAnimationFrame
// callbacks instead of blocking waits.
func (p *browserPlatform) WaitEvents() {
	// Browser event loop is non-blocking. The Go main loop must cooperate
	// with requestAnimationFrame. Blocking here would freeze the page.
	// Instead, we return immediately — the caller should use the
	// requestAnimationFrame-based run loop (see app_browser.go).
}

// WakeUp is a no-op on browser (single-threaded JS environment).
func (p *browserPlatform) WakeUp() {}

// ClipboardRead reads text from the system clipboard via the Clipboard API.
// Note: requires user gesture and Permissions API for async clipboard.
func (p *browserPlatform) ClipboardRead() (string, error) {
	// Synchronous clipboard API is not available in modern browsers.
	// The async clipboard API (navigator.clipboard.readText) requires Promises
	// which we can't easily block on from Go. Return empty for now.
	return "", nil
}

// ClipboardWrite writes text to the system clipboard via the Clipboard API.
func (p *browserPlatform) ClipboardWrite(text string) error {
	clipboard := js.Global().Get("navigator").Get("clipboard")
	if clipboard.IsUndefined() {
		return fmt.Errorf("clipboard API not available")
	}
	clipboard.Call("writeText", text)
	return nil
}

// DarkMode returns true if the user prefers dark color scheme.
func (p *browserPlatform) DarkMode() bool {
	mql := js.Global().Call("matchMedia", "(prefers-color-scheme: dark)")
	if mql.IsUndefined() || mql.IsNull() {
		return false
	}
	return mql.Get("matches").Bool()
}

// ReduceMotion returns true if the user prefers reduced motion.
func (p *browserPlatform) ReduceMotion() bool {
	mql := js.Global().Call("matchMedia", "(prefers-reduced-motion: reduce)")
	if mql.IsUndefined() || mql.IsNull() {
		return false
	}
	return mql.Get("matches").Bool()
}

// HighContrast returns true if the user prefers high contrast.
func (p *browserPlatform) HighContrast() bool {
	mql := js.Global().Call("matchMedia", "(prefers-contrast: more)")
	if mql.IsUndefined() || mql.IsNull() {
		return false
	}
	return mql.Get("matches").Bool()
}

// FontScale returns 1.0 on browser — font scaling is handled by CSS.
func (p *browserPlatform) FontScale() float32 {
	return 1.0
}

// SubpixelLayout returns SubpixelNone on browser — subpixel text rendering
// is controlled by the browser engine, not the application.
func (p *browserPlatform) SubpixelLayout() gpucontext.SubpixelLayout {
	return gpucontext.SubpixelNone
}

// SetAppName is a no-op on browser — the page title is set via document.title.
func (p *browserPlatform) SetAppName(_ string) {}

// ShowOpenFileDialog is not yet implemented on browser.
// Future: could use HTML <input type="file"> via syscall/js.
func (p *browserPlatform) ShowOpenFileDialog(_ FileDialogOptions) ([]string, error) {
	return nil, fmt.Errorf("file dialog: not yet implemented in browser")
}

// ShowSaveFileDialog is not yet implemented on browser.
// Future: could use File System Access API (showSaveFilePicker) via syscall/js.
func (p *browserPlatform) ShowSaveFileDialog(_ FileDialogOptions) (string, error) {
	return "", fmt.Errorf("file dialog: not yet implemented in browser")
}

// Destroy is a no-op on browser.
func (p *browserPlatform) Destroy() {}

// enqueueEvent adds an event to the platform event queue.
func (p *browserPlatform) enqueueEvent(ev Event) {
	p.events.Push(ev)
}

// --------------------------------------------------------------------------
// browserWindow implements PlatformWindow for a single HTML <canvas>.
// --------------------------------------------------------------------------

type browserWindow struct {
	id          WindowID
	canvas      js.Value
	shouldClose bool

	// JS callbacks stored for cleanup.
	jsCallbacks []js.Func
}

// registerEventListeners sets up DOM event listeners on the canvas.
func (w *browserWindow) registerEventListeners(p *browserPlatform) {
	// Keyboard events — listen on document since canvas doesn't receive
	// key events without tabindex.
	w.canvas.Call("setAttribute", "tabindex", "0")

	w.addEventListener(w.canvas, "keydown", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		key, mods := translateKeyEvent(ev)
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventKeyDown,
			Key:      key,
			Mods:     mods,
		})
		// Also generate EventChar for printable characters.
		if keyStr := ev.Get("key").String(); len(keyStr) == 1 {
			r := []rune(keyStr)
			p.enqueueEvent(Event{
				WindowID: w.id,
				Type:     EventChar,
				Char:     r[0],
			})
		}
		return nil
	})

	w.addEventListener(w.canvas, "keyup", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		key, mods := translateKeyEvent(ev)
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventKeyUp,
			Key:      key,
			Mods:     mods,
		})
		return nil
	})

	// Pointer events (mouse + touch unified).
	w.addEventListener(w.canvas, "pointerdown", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventPointerDown,
			Pointer:  translatePointerEvent(ev, gpucontext.PointerDown),
		})
		return nil
	})

	w.addEventListener(w.canvas, "pointerup", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventPointerUp,
			Pointer:  translatePointerEvent(ev, gpucontext.PointerUp),
		})
		return nil
	})

	w.addEventListener(w.canvas, "pointermove", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventPointerMove,
			Pointer:  translatePointerEvent(ev, gpucontext.PointerMove),
		})
		return nil
	})

	w.addEventListener(w.canvas, "pointerenter", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventPointerEnter,
			Pointer:  translatePointerEvent(ev, gpucontext.PointerMove),
		})
		return nil
	})

	w.addEventListener(w.canvas, "pointerleave", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventPointerLeave,
			Pointer:  translatePointerEvent(ev, gpucontext.PointerMove),
		})
		return nil
	})

	// Wheel/scroll events.
	w.addEventListener(w.canvas, "wheel", func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventScroll,
			Scroll: gpucontext.ScrollEvent{
				DeltaX: ev.Get("deltaX").Float(),
				DeltaY: ev.Get("deltaY").Float(),
			},
		})
		return nil
	})

	// Resize: watch window resize and update canvas dimensions.
	w.addEventListener(js.Global(), "resize", func(_ js.Value, _ []js.Value) any {
		logW, logH := w.LogicalSize()
		physW, physH := w.PhysicalSize()
		p.enqueueEvent(Event{
			WindowID:       w.id,
			Type:           EventResize,
			Width:          logW,
			Height:         logH,
			PhysicalWidth:  physW,
			PhysicalHeight: physH,
		})
		return nil
	})

	// Context menu suppression (right-click).
	w.addEventListener(w.canvas, "contextmenu", func(_ js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		return nil
	})
}

// addEventListener registers a JS event listener and tracks the callback for cleanup.
func (w *browserWindow) addEventListener(target js.Value, event string, fn func(js.Value, []js.Value) any) {
	cb := js.FuncOf(fn)
	w.jsCallbacks = append(w.jsCallbacks, cb)
	target.Call("addEventListener", event, cb)
}

// ID returns the unique window identifier.
func (w *browserWindow) ID() WindowID { return w.id }

// GetHandle returns (0, 0) — on browser, wgpu finds the canvas via DOM query.
func (w *browserWindow) GetHandle() (instance, window uintptr) { return 0, 0 }

// LogicalSize returns the CSS pixel dimensions of the canvas.
func (w *browserWindow) LogicalSize() (width, height int) {
	return w.canvas.Get("clientWidth").Int(), w.canvas.Get("clientHeight").Int()
}

// PhysicalSize returns the device pixel dimensions of the canvas.
func (w *browserWindow) PhysicalSize() (width, height int) {
	return w.canvas.Get("width").Int(), w.canvas.Get("height").Int()
}

// ScaleFactor returns window.devicePixelRatio (e.g. 2.0 on Retina displays).
func (w *browserWindow) ScaleFactor() float64 {
	dpr := js.Global().Get("devicePixelRatio")
	if dpr.IsUndefined() || dpr.IsNull() {
		return 1.0
	}
	return dpr.Float()
}

// PrepareFrame updates canvas backing store to match devicePixelRatio.
func (w *browserWindow) PrepareFrame() PrepareFrameResult {
	dpr := w.ScaleFactor()
	clientW := w.canvas.Get("clientWidth").Int()
	clientH := w.canvas.Get("clientHeight").Int()
	physW := int(float64(clientW) * dpr)
	physH := int(float64(clientH) * dpr)

	// Update canvas backing store if needed.
	curW := w.canvas.Get("width").Int()
	curH := w.canvas.Get("height").Int()
	changed := physW != curW || physH != curH
	if changed {
		w.canvas.Set("width", physW)
		w.canvas.Set("height", physH)
	}

	return PrepareFrameResult{
		ScaleChanged:   changed,
		ScaleFactor:    dpr,
		PhysicalWidth:  uint32(physW),
		PhysicalHeight: uint32(physH),
	}
}

// InSizeMove returns false — browser has no modal resize loop.
func (w *browserWindow) InSizeMove() bool { return false }

// ShouldClose returns true if close was requested (tab/window close).
func (w *browserWindow) ShouldClose() bool { return w.shouldClose }

// SetTitle sets the document title.
func (w *browserWindow) SetTitle(title string) {
	js.Global().Get("document").Set("title", title)
}

// SetCursor sets the CSS cursor style on the canvas.
func (w *browserWindow) SetCursor(cursorID int) {
	css := cursorIDToCSS(cursorID)
	w.canvas.Get("style").Set("cursor", css)
}

// SetMinSize is a no-op on browser — window sizing is controlled by the page.
func (w *browserWindow) SetMinSize(_, _ int) {}

// SetMaxSize is a no-op on browser — window sizing is controlled by the page.
func (w *browserWindow) SetMaxSize(_, _ int) {}

// SetFrameless is a no-op on browser — there's no OS window chrome.
func (w *browserWindow) SetFrameless(_ bool) {}

// IsFrameless returns true — browser canvas has no window chrome.
func (w *browserWindow) IsFrameless() bool { return true }

// SetFullscreen enters/exits browser fullscreen via the Fullscreen API.
func (w *browserWindow) SetFullscreen(fullscreen bool) {
	if fullscreen {
		w.canvas.Call("requestFullscreen")
	} else {
		js.Global().Get("document").Call("exitFullscreen")
	}
}

// IsFullscreen returns true if the document is in fullscreen mode.
func (w *browserWindow) IsFullscreen() bool {
	fse := js.Global().Get("document").Get("fullscreenElement")
	return !fse.IsNull() && !fse.IsUndefined()
}

// SetHitTestCallback is a no-op on browser — hit testing is not applicable.
func (w *browserWindow) SetHitTestCallback(_ func(x, y float64) gpucontext.HitTestResult) {}

// Minimize is a no-op — browser windows can't be minimized from JS.
func (w *browserWindow) Minimize() {}

// Maximize is a no-op — use Fullscreen API instead.
func (w *browserWindow) Maximize() {}

// IsMaximized returns false — not applicable for browser canvas.
func (w *browserWindow) IsMaximized() bool { return false }

// Close marks the window as should-close. On browser, the user closes tabs directly.
func (w *browserWindow) Close() { w.shouldClose = true }

// Show is a no-op on browser -- the canvas is always visible.
func (w *browserWindow) Show() {}

// SyncFrame is a no-op — browser compositing is handled by requestAnimationFrame.
func (w *browserWindow) SyncFrame() {}

// SetCursorMode is a no-op on browser (pointer lock requires Pointer Lock API).
func (w *browserWindow) SetCursorMode(_ int) {}

// CursorMode returns 0 (normal mode).
func (w *browserWindow) CursorMode() int { return 0 }

// SetModalFrameCallback is a no-op — browser has no modal resize loops.
func (w *browserWindow) SetModalFrameCallback(_ func()) {}

// Destroy releases JS callbacks.
func (w *browserWindow) Destroy() {
	for _, cb := range w.jsCallbacks {
		cb.Release()
	}
	w.jsCallbacks = nil
}

// --------------------------------------------------------------------------
// Key and pointer event translation helpers
// --------------------------------------------------------------------------

// translateKeyEvent converts a JS KeyboardEvent to gpucontext Key + Modifiers.
func translateKeyEvent(ev js.Value) (gpucontext.Key, gpucontext.Modifiers) {
	code := ev.Get("code").String()
	key := jsCodeToKey(code)

	var mods gpucontext.Modifiers
	if ev.Get("shiftKey").Bool() {
		mods |= gpucontext.ModShift
	}
	if ev.Get("ctrlKey").Bool() {
		mods |= gpucontext.ModControl
	}
	if ev.Get("altKey").Bool() {
		mods |= gpucontext.ModAlt
	}
	if ev.Get("metaKey").Bool() {
		mods |= gpucontext.ModSuper
	}
	return key, mods
}

// translatePointerEvent converts a JS PointerEvent to gpucontext.PointerEvent.
func translatePointerEvent(ev js.Value, eventType gpucontext.PointerEventType) gpucontext.PointerEvent {
	pe := gpucontext.PointerEvent{
		Type: eventType,
		X:    ev.Get("offsetX").Float(),
		Y:    ev.Get("offsetY").Float(),
	}

	// Pointer type.
	switch ev.Get("pointerType").String() {
	case "mouse":
		pe.PointerType = gpucontext.PointerTypeMouse
	case "pen":
		pe.PointerType = gpucontext.PointerTypePen
	case "touch":
		pe.PointerType = gpucontext.PointerTypeTouch
	default:
		pe.PointerType = gpucontext.PointerTypeMouse
	}

	// Mouse button.
	switch ev.Get("button").Int() {
	case 0:
		pe.Button = gpucontext.ButtonLeft
	case 1:
		pe.Button = gpucontext.ButtonMiddle
	case 2:
		pe.Button = gpucontext.ButtonRight
	case 3:
		pe.Button = gpucontext.ButtonX1
	case 4:
		pe.Button = gpucontext.ButtonX2
	}

	// Movement delta for pointer lock.
	pe.DeltaX = ev.Get("movementX").Float()
	pe.DeltaY = ev.Get("movementY").Float()

	return pe
}

// cursorIDToCSS converts a gpucontext.CursorShape to a CSS cursor value.
func cursorIDToCSS(id int) string {
	switch gpucontext.CursorShape(id) {
	case gpucontext.CursorDefault:
		return "default"
	case gpucontext.CursorText:
		return "text"
	case gpucontext.CursorPointer:
		return "pointer"
	case gpucontext.CursorCrosshair:
		return "crosshair"
	case gpucontext.CursorMove:
		return "move"
	case gpucontext.CursorResizeNS:
		return "ns-resize"
	case gpucontext.CursorResizeEW:
		return "ew-resize"
	case gpucontext.CursorResizeNWSE:
		return "nwse-resize"
	case gpucontext.CursorResizeNESW:
		return "nesw-resize"
	case gpucontext.CursorNotAllowed:
		return "not-allowed"
	case gpucontext.CursorWait:
		return "wait"
	case gpucontext.CursorNone:
		return "none"
	default:
		return "default"
	}
}

// jsCodeToKey maps JS KeyboardEvent.code to gpucontext.Key.
//
//nolint:maintidx // key mapping tables are inherently large
func jsCodeToKey(code string) gpucontext.Key {
	switch code {
	// Letters
	case "KeyA":
		return gpucontext.KeyA
	case "KeyB":
		return gpucontext.KeyB
	case "KeyC":
		return gpucontext.KeyC
	case "KeyD":
		return gpucontext.KeyD
	case "KeyE":
		return gpucontext.KeyE
	case "KeyF":
		return gpucontext.KeyF
	case "KeyG":
		return gpucontext.KeyG
	case "KeyH":
		return gpucontext.KeyH
	case "KeyI":
		return gpucontext.KeyI
	case "KeyJ":
		return gpucontext.KeyJ
	case "KeyK":
		return gpucontext.KeyK
	case "KeyL":
		return gpucontext.KeyL
	case "KeyM":
		return gpucontext.KeyM
	case "KeyN":
		return gpucontext.KeyN
	case "KeyO":
		return gpucontext.KeyO
	case "KeyP":
		return gpucontext.KeyP
	case "KeyQ":
		return gpucontext.KeyQ
	case "KeyR":
		return gpucontext.KeyR
	case "KeyS":
		return gpucontext.KeyS
	case "KeyT":
		return gpucontext.KeyT
	case "KeyU":
		return gpucontext.KeyU
	case "KeyV":
		return gpucontext.KeyV
	case "KeyW":
		return gpucontext.KeyW
	case "KeyX":
		return gpucontext.KeyX
	case "KeyY":
		return gpucontext.KeyY
	case "KeyZ":
		return gpucontext.KeyZ

	// Digits
	case "Digit0":
		return gpucontext.Key0
	case "Digit1":
		return gpucontext.Key1
	case "Digit2":
		return gpucontext.Key2
	case "Digit3":
		return gpucontext.Key3
	case "Digit4":
		return gpucontext.Key4
	case "Digit5":
		return gpucontext.Key5
	case "Digit6":
		return gpucontext.Key6
	case "Digit7":
		return gpucontext.Key7
	case "Digit8":
		return gpucontext.Key8
	case "Digit9":
		return gpucontext.Key9

	// Function keys
	case "F1":
		return gpucontext.KeyF1
	case "F2":
		return gpucontext.KeyF2
	case "F3":
		return gpucontext.KeyF3
	case "F4":
		return gpucontext.KeyF4
	case "F5":
		return gpucontext.KeyF5
	case "F6":
		return gpucontext.KeyF6
	case "F7":
		return gpucontext.KeyF7
	case "F8":
		return gpucontext.KeyF8
	case "F9":
		return gpucontext.KeyF9
	case "F10":
		return gpucontext.KeyF10
	case "F11":
		return gpucontext.KeyF11
	case "F12":
		return gpucontext.KeyF12

	// Navigation
	case "Escape":
		return gpucontext.KeyEscape
	case "Tab":
		return gpucontext.KeyTab
	case "Backspace":
		return gpucontext.KeyBackspace
	case "Enter":
		return gpucontext.KeyEnter
	case "Space":
		return gpucontext.KeySpace
	case "Insert":
		return gpucontext.KeyInsert
	case "Delete":
		return gpucontext.KeyDelete
	case "Home":
		return gpucontext.KeyHome
	case "End":
		return gpucontext.KeyEnd
	case "PageUp":
		return gpucontext.KeyPageUp
	case "PageDown":
		return gpucontext.KeyPageDown
	case "ArrowLeft":
		return gpucontext.KeyLeft
	case "ArrowRight":
		return gpucontext.KeyRight
	case "ArrowUp":
		return gpucontext.KeyUp
	case "ArrowDown":
		return gpucontext.KeyDown

	// Modifiers
	case "ShiftLeft":
		return gpucontext.KeyLeftShift
	case "ShiftRight":
		return gpucontext.KeyRightShift
	case "ControlLeft":
		return gpucontext.KeyLeftControl
	case "ControlRight":
		return gpucontext.KeyRightControl
	case "AltLeft":
		return gpucontext.KeyLeftAlt
	case "AltRight":
		return gpucontext.KeyRightAlt
	case "MetaLeft":
		return gpucontext.KeyLeftSuper
	case "MetaRight":
		return gpucontext.KeyRightSuper

	// Punctuation
	case "Minus":
		return gpucontext.KeyMinus
	case "Equal":
		return gpucontext.KeyEqual
	case "BracketLeft":
		return gpucontext.KeyLeftBracket
	case "BracketRight":
		return gpucontext.KeyRightBracket
	case "Backslash":
		return gpucontext.KeyBackslash
	case "Semicolon":
		return gpucontext.KeySemicolon
	case "Quote":
		return gpucontext.KeyApostrophe
	case "Backquote":
		return gpucontext.KeyGrave
	case "Comma":
		return gpucontext.KeyComma
	case "Period":
		return gpucontext.KeyPeriod
	case "Slash":
		return gpucontext.KeySlash

	// Numpad
	case "Numpad0":
		return gpucontext.KeyNumpad0
	case "Numpad1":
		return gpucontext.KeyNumpad1
	case "Numpad2":
		return gpucontext.KeyNumpad2
	case "Numpad3":
		return gpucontext.KeyNumpad3
	case "Numpad4":
		return gpucontext.KeyNumpad4
	case "Numpad5":
		return gpucontext.KeyNumpad5
	case "Numpad6":
		return gpucontext.KeyNumpad6
	case "Numpad7":
		return gpucontext.KeyNumpad7
	case "Numpad8":
		return gpucontext.KeyNumpad8
	case "Numpad9":
		return gpucontext.KeyNumpad9
	case "NumpadDecimal":
		return gpucontext.KeyNumpadDecimal
	case "NumpadDivide":
		return gpucontext.KeyNumpadDivide
	case "NumpadMultiply":
		return gpucontext.KeyNumpadMultiply
	case "NumpadSubtract":
		return gpucontext.KeyNumpadSubtract
	case "NumpadAdd":
		return gpucontext.KeyNumpadAdd
	case "NumpadEnter":
		return gpucontext.KeyNumpadEnter

	// Lock keys
	case "CapsLock":
		return gpucontext.KeyCapsLock
	case "ScrollLock":
		return gpucontext.KeyScrollLock
	case "NumLock":
		return gpucontext.KeyNumLock
	case "Pause":
		return gpucontext.KeyPause

	default:
		return 0
	}
}
