//go:build js && wasm

package platform

import (
	"fmt"
	"math"
	"syscall/js"
	"unicode/utf8"

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
	// TODO(#361, ADR-060): Browser transparency — CSS transparency (e.g.
	// canvas background) is not implemented yet; Config.Transparent is
	// silently ignored.
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
		id:       NewWindowID(),
		canvas:   canvas,
		platform: p,
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

// FontSmoothing returns FontSmoothingGrayscale on browser — text rendering
// is controlled by the browser engine, not the application.
func (p *browserPlatform) FontSmoothing() gpucontext.FontSmoothing {
	return gpucontext.FontSmoothingGrayscale
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

// Destroy removes the hidden IME input and releases DOM callbacks.
func (p *browserPlatform) Destroy() {
	if p != nil && p.window != nil {
		p.window.Destroy()
		p.window = nil
	}
}

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
	platform    *browserPlatform
	destroyed   bool
	shouldClose bool
	lastScale   float64 // DPI scale change detection (ADR-059)

	// Browser IME input is delivered through a focusable, visually hidden
	// textarea. It must remain in the DOM (display:none breaks composition),
	// while all text is routed through the platform event queue.
	imeInput          js.Value
	imeTracker        browserIMETracker
	imeArea           gpucontext.IMECursorArea
	imeAreaSet        bool
	imePurpose        gpucontext.ContentPurpose
	imeHints          gpucontext.ContentHint
	imeSensitive      bool
	imeSurrounding    gpucontext.IMESurroundingText
	imeSurroundingSet bool
	imeEnabled        bool
	focused           bool
	suppressBlur      bool

	// JS callbacks stored for cleanup.
	jsCallbacks []js.Func
}

// registerEventListeners sets up DOM event listeners on the canvas.
func (w *browserWindow) registerEventListeners(p *browserPlatform) {
	// Keyboard events — listen on document since canvas doesn't receive
	// key events without tabindex.
	w.canvas.Call("setAttribute", "tabindex", "0")
	w.registerIMEInput(p)

	// A canvas can lose focus when the hidden IME input takes focus. The
	// hidden-input handlers below coalesce that transition into one window
	// focus state instead of exposing a false blur to consumers.
	w.addEventListener(w.canvas, "focus", func(_ js.Value, _ []js.Value) any {
		w.setBrowserFocus(true)
		return nil
	})
	w.addEventListener(w.canvas, "blur", func(_ js.Value, _ []js.Value) any {
		w.handleCanvasBlur()
		return nil
	})

	w.addEventListener(w.canvas, "keydown", func(_ js.Value, args []js.Value) any {
		if w.destroyed {
			return nil
		}
		ev := args[0]
		// When IME is enabled, the hidden input owns text editing and its
		// beforeinput/input events provide the text payload. The canvas may
		// still receive a key event in browsers that do not move focus
		// synchronously, so do not synthesize a second EventChar here.
		if !w.imeEnabled {
			ev.Call("preventDefault")
		}
		key, mods := translateKeyEvent(ev)
		p.enqueueEvent(Event{
			WindowID: w.id,
			Type:     EventKeyDown,
			Key:      key,
			Mods:     mods,
		})
		// Also generate EventChar for printable characters when the native
		// input method is disabled. IME-enabled text is delivered by the
		// hidden input's input event, avoiding keydown/text double-dispatch.
		if !w.imeEnabled {
			if keyText := ev.Get("key").String(); len([]rune(keyText)) == 1 {
				w.enqueueText(keyText)
			}
		}
		return nil
	})

	w.addEventListener(w.canvas, "keyup", func(_ js.Value, args []js.Value) any {
		if w.destroyed {
			return nil
		}
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
				X:      ev.Get("offsetX").Float(),
				Y:      ev.Get("offsetY").Float(),
				DeltaX: ev.Get("deltaX").Float(),
				DeltaY: ev.Get("deltaY").Float(),
			},
		})
		return nil
	})

	// Resize: watch window resize and update canvas dimensions.
	w.addEventListener(js.Global(), "resize", func(_ js.Value, _ []js.Value) any {
		w.applyIMEInputArea()
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
	w.addEventListener(js.Global(), "scroll", func(_ js.Value, _ []js.Value) any {
		w.applyIMEInputArea()
		return nil
	})

	// Context menu suppression (right-click).
	w.addEventListener(w.canvas, "contextmenu", func(_ js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		return nil
	})
}

// registerIMEInput creates the hidden DOM control used by browser IMEs. A
// canvas is not a text-editing target, and CompositionEvent is only delivered
// reliably to an input/textarea that owns focus. The control remains attached
// and focusable while its pixels are transparent and it is positioned at the
// latest candidate rectangle.
func (w *browserWindow) registerIMEInput(p *browserPlatform) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "textarea")
	input.Set("rows", 1)
	input.Set("cols", 1)
	input.Set("wrap", "off")
	input.Set("tabIndex", -1)
	input.Call("setAttribute", "aria-hidden", "true")
	input.Set("autocomplete", "off")
	input.Set("autocorrect", "off")
	input.Set("spellcheck", false)
	style := input.Get("style")
	style.Set("position", "fixed")
	style.Set("left", "0px")
	style.Set("top", "0px")
	style.Set("width", "1px")
	style.Set("height", "1px")
	style.Set("padding", "0")
	style.Set("margin", "0")
	style.Set("border", "0")
	style.Set("outline", "none")
	style.Set("background", "transparent")
	style.Set("color", "transparent")
	style.Set("caretColor", "transparent")
	style.Set("opacity", "0")
	style.Set("pointerEvents", "none")
	// Keep the control in the hit-test/render tree. Some mobile browsers do
	// not open an IME for off-screen or negative-z-index controls; opacity keeps
	// it invisible while the rect remains available for candidate placement.
	style.Set("zIndex", "2147483647")
	doc.Get("body").Call("appendChild", input)
	w.imeInput = input
	w.imeTracker.setEnabled(false)

	w.addEventListener(input, "focus", func(_ js.Value, _ []js.Value) any {
		w.setBrowserFocus(true)
		return nil
	})
	w.addEventListener(input, "blur", func(_ js.Value, _ []js.Value) any {
		w.handleIMEInputBlur()
		return nil
	})
	w.addEventListener(input, "compositionstart", func(_ js.Value, _ []js.Value) any {
		w.handleIMECompositionStart()
		return nil
	})
	w.addEventListener(input, "compositionupdate", func(_ js.Value, args []js.Value) any {
		w.handleIMECompositionUpdate(jsStringProperty(args[0], "data"))
		return nil
	})
	w.addEventListener(input, "compositionend", func(_ js.Value, args []js.Value) any {
		w.handleIMECompositionEnd(jsStringProperty(args[0], "data"))
		return nil
	})
	w.addEventListener(input, "beforeinput", func(_ js.Value, args []js.Value) any {
		w.handleIMEBeforeInput(args[0])
		return nil
	})
	w.addEventListener(input, "input", func(_ js.Value, args []js.Value) any {
		w.handleIMEInput(args[0])
		return nil
	})
	w.addEventListener(input, "keydown", func(_ js.Value, args []js.Value) any {
		w.handleIMEKeyDown(args[0], p)
		return nil
	})
	w.addEventListener(input, "keyup", func(_ js.Value, args []js.Value) any {
		w.handleIMEKeyUp(args[0], p)
		return nil
	})
}

func (w *browserWindow) setBrowserFocus(focused bool) {
	if w.destroyed {
		return
	}
	if w.focused == focused {
		return
	}
	w.focused = focused
	w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventFocus, Focused: focused})
}

func (w *browserWindow) handleCanvasBlur() {
	if w.destroyed {
		return
	}
	if w.suppressBlur || w.activeElementIsIMEInput() {
		return
	}
	if w.imeEnabled {
		w.SetIMEEnabled(false)
	}
	w.setBrowserFocus(false)
}

func (w *browserWindow) handleIMEInputBlur() {
	if w.destroyed {
		return
	}
	if w.suppressBlur {
		w.suppressBlur = false
		return
	}
	if !w.imeEnabled {
		return
	}
	// Focus moved outside both the canvas and hidden input. Match native
	// backends: cancel preedit, drop surrounding text, then report disabled.
	w.SetIMEEnabled(false)
	if !w.activeElementIsCanvas() {
		w.setBrowserFocus(false)
	}
}

func (w *browserWindow) activeElementIsIMEInput() bool {
	if w.imeInput.IsNull() || w.imeInput.IsUndefined() {
		return false
	}
	active := js.Global().Get("document").Get("activeElement")
	return active.Equal(w.imeInput)
}

func (w *browserWindow) activeElementIsCanvas() bool {
	if w.canvas.IsNull() || w.canvas.IsUndefined() {
		return false
	}
	active := js.Global().Get("document").Get("activeElement")
	return active.Equal(w.canvas)
}

func (w *browserWindow) enqueueText(text string) {
	if text == "" {
		return
	}
	for _, r := range text {
		if r < 32 || r == 127 {
			continue
		}
		w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventChar, Char: r})
	}
}

func (w *browserWindow) handleIMEKeyDown(ev js.Value, p *browserPlatform) {
	if w.destroyed {
		return
	}
	key, mods := translateKeyEvent(ev)
	p.enqueueEvent(Event{WindowID: w.id, Type: EventKeyDown, Key: key, Mods: mods})
}

func (w *browserWindow) handleIMEKeyUp(ev js.Value, p *browserPlatform) {
	if w.destroyed {
		return
	}
	key, mods := translateKeyEvent(ev)
	p.enqueueEvent(Event{WindowID: w.id, Type: EventKeyUp, Key: key, Mods: mods})
}

func (w *browserWindow) handleIMECompositionStart() {
	if w.destroyed {
		return
	}
	if !w.imeEnabled || !w.imeTracker.start() {
		return
	}
	w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventIMECompositionStart})
}

func (w *browserWindow) handleIMECompositionUpdate(data string) {
	if w.destroyed || !w.imeEnabled {
		return
	}
	if w.imeTracker.ensureActive() {
		w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventIMECompositionStart})
	}
	composition := browserIMEComposition(data)
	if !composition.IsValid() {
		return
	}
	w.platform.enqueueEvent(Event{
		WindowID:       w.id,
		Type:           EventIMECompositionUpdate,
		IMEComposition: composition,
	})
}

func (w *browserWindow) handleIMECompositionEnd(data string) {
	if w.destroyed || !w.imeEnabled {
		return
	}
	committed, canceled, ok := w.imeTracker.end(data)
	if !ok {
		return
	}
	if canceled {
		w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventIMECanceled})
		return
	}
	w.platform.enqueueEvent(Event{
		WindowID:     w.id,
		Type:         EventIMECompositionEnd,
		IMECommitted: committed,
	})
}

func (w *browserWindow) handleIMEBeforeInput(ev js.Value) {
	if w.destroyed || !w.imeEnabled {
		return
	}
	inputType := jsStringProperty(ev, "inputType")
	deletion, ok := w.browserDeleteSurrounding(ev, inputType)
	if !ok {
		return
	}
	ev.Call("preventDefault")
	w.platform.enqueueEvent(Event{
		WindowID:  w.id,
		Type:      EventIMEDeleteSurrounding,
		IMEDelete: deletion,
	})
}

func (w *browserWindow) handleIMEInput(ev js.Value) {
	if w.destroyed || !w.imeEnabled {
		return
	}
	inputType := jsStringProperty(ev, "inputType")
	data := jsStringProperty(ev, "data")
	text, _ := w.imeTracker.input(inputType, data)
	w.enqueueText(text)
}

// SetIMEPosition implements the legacy controller in terms of the richer
// logical-DIP cursor area.
func (w *browserWindow) SetIMEPosition(x, y int) {
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: float64(x), Y: float64(y)})
}

func (w *browserWindow) IMECapabilities() gpucontext.IMECapabilities {
	return DefaultIMECapabilities()
}

func (w *browserWindow) SetIMEEnabled(enabled bool) {
	if w.destroyed {
		return
	}
	if enabled {
		if w.imeEnabled {
			w.applyIMEInputArea()
			w.focusIMEInput()
			return
		}
		w.imeEnabled = true
		w.imeTracker.setEnabled(true)
		w.applyIMEInputAttributes()
		w.applyIMEInputValue()
		w.applyIMEInputArea()
		w.focusIMEInput()
		return
	}
	w.disableIME()
}

func (w *browserWindow) disableIME() {
	wasEnabled := w.imeEnabled
	canceled := w.imeTracker.cancel()
	w.imeTracker.setEnabled(false)
	w.imeEnabled = false
	// Do not retain sensitive surrounding text in a DOM control after IME is
	// disabled. App keeps an explicit copy for replay on the next enable.
	w.imeSurrounding = gpucontext.IMESurroundingText{}
	w.imeSurroundingSet = false
	w.clearIMEInput()
	if w.activeElementIsIMEInput() {
		w.suppressBlur = true
		w.imeInput.Call("blur")
	}
	if canceled {
		w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventIMECanceled})
	}
	if wasEnabled {
		w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventIMEDisabled})
	}
}

func (w *browserWindow) SetIMECursorArea(area gpucontext.IMECursorArea) {
	if w.destroyed || !validBrowserIMEArea(area) {
		return
	}
	w.imeArea = area
	w.imeAreaSet = true
	if w.imeEnabled {
		w.applyIMEInputArea()
	}
}

func (w *browserWindow) SetIMEContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	if w.destroyed {
		return
	}
	w.imePurpose, w.imeHints = purpose, hints
	w.imeSensitive = purpose == gpucontext.ContentPurposePassword ||
		hints.Has(gpucontext.ContentHintHiddenText) || hints.Has(gpucontext.ContentHintSensitiveData)
	if w.imeSensitive && w.imeEnabled {
		// Password/sensitive fields must not leave a focused DOM editor or
		// an active preedit that could be observed by browser suggestions.
		w.SetIMEEnabled(false)
	}
	w.applyIMEInputAttributes()
	if w.imeSensitive {
		w.clearIMEInput()
	}
}

func (w *browserWindow) SetIMESurroundingText(text gpucontext.IMESurroundingText) {
	if w.destroyed || !text.IsValid() || !w.imeEnabled || w.imeSensitive {
		return
	}
	w.imeSurrounding = text
	w.imeSurroundingSet = true
	w.applyIMEInputValue()
}

func (w *browserWindow) CancelIME() {
	if w.destroyed {
		return
	}
	if !w.imeTracker.cancel() {
		return
	}
	w.platform.enqueueEvent(Event{WindowID: w.id, Type: EventIMECanceled})
	w.applyIMEInputValue()
	if w.imeEnabled {
		w.focusIMEInput()
	}
}

func (w *browserWindow) focusIMEInput() {
	if w.imeInput.IsNull() || w.imeInput.IsUndefined() {
		return
	}
	w.imeInput.Call("focus")
}

func (w *browserWindow) clearIMEInput() {
	if w.imeInput.IsNull() || w.imeInput.IsUndefined() {
		return
	}
	w.imeInput.Set("value", "")
	w.imeInput.Call("setSelectionRange", 0, 0)
}

func (w *browserWindow) applyIMEInputValue() {
	if w.imeInput.IsNull() || w.imeInput.IsUndefined() {
		return
	}
	if !w.imeEnabled || w.imeSensitive || !w.imeSurroundingSet {
		w.clearIMEInput()
		return
	}
	text := w.imeSurrounding.Text
	w.imeInput.Set("value", text)
	start := utf8OffsetToUTF16(text, w.imeSurrounding.Cursor)
	end := utf8OffsetToUTF16(text, w.imeSurrounding.Anchor)
	w.imeInput.Call("setSelectionRange", start, end)
}

func (w *browserWindow) applyIMEInputAttributes() {
	if w.imeInput.IsNull() || w.imeInput.IsUndefined() {
		return
	}
	inputMode := browserInputMode(w.imePurpose)
	w.imeInput.Set("inputMode", inputMode)
	w.imeInput.Set("autocapitalize", browserAutoCapitalize(w.imeHints))
	spellcheck := !w.imeSensitive && !w.imeHints.Has(gpucontext.ContentHintLowercase)
	w.imeInput.Set("spellcheck", spellcheck)
	if w.imeSensitive {
		w.imeInput.Set("autocomplete", "new-password")
		w.imeInput.Set("autocorrect", "off")
	} else {
		w.imeInput.Set("autocomplete", "off")
		w.imeInput.Set("autocorrect", "on")
	}
}

func (w *browserWindow) applyIMEInputArea() {
	if !w.imeAreaSet || w.imeInput.IsNull() || w.imeInput.IsUndefined() {
		return
	}
	area := w.imeArea
	canvasRect := w.canvas.Call("getBoundingClientRect")
	left := canvasRect.Get("left").Float() + area.X
	top := canvasRect.Get("top").Float() + area.Y
	style := w.imeInput.Get("style")
	style.Set("left", cssPixels(left))
	style.Set("top", cssPixels(top))
	style.Set("width", cssPixels(math.Max(area.Width, 1)))
	style.Set("height", cssPixels(math.Max(area.Height, 1)))
}

func cssPixels(value float64) string {
	return fmt.Sprintf("%.3fpx", value)
}

func jsStringProperty(value js.Value, property string) string {
	propertyValue := value.Get(property)
	if propertyValue.IsUndefined() || propertyValue.IsNull() {
		return ""
	}
	return propertyValue.String()
}

func (w *browserWindow) browserDeleteSurrounding(ev js.Value, inputType string) (gpucontext.IMEDeleteSurroundingEvent, bool) {
	text := ""
	if !w.imeSensitive && w.imeSurroundingSet {
		text = w.imeSurrounding.Text
	} else if !w.imeInput.IsNull() && !w.imeInput.IsUndefined() {
		text = w.imeInput.Get("value").String()
	}
	if !utf8.ValidString(text) {
		return gpucontext.IMEDeleteSurroundingEvent{}, false
	}
	start16, end16, ok := browserSelection(ev, text, w.imeSurrounding)
	if !ok {
		return gpucontext.IMEDeleteSurroundingEvent{}, false
	}
	start := utf16OffsetToUTF8(text, start16)
	end := utf16OffsetToUTF8(text, end16)
	if start > end {
		start, end = end, start
	}
	if start < 0 || end < start || end > len(text) {
		return gpucontext.IMEDeleteSurroundingEvent{}, false
	}
	if start != end {
		// A selected range is represented as an atomic deletion before the
		// DOM's selection anchor. Consumers apply it as one edit.
		deletion := gpucontext.IMEDeleteSurroundingEvent{Before: end - start}
		return deletion, deletion.IsValid()
	}

	var before, after int
	switch inputType {
	case "deleteContentBackward":
		before = previousRuneBytes(text, start)
	case "deleteWordBackward":
		before = start - previousWordStart(text, start)
	case "deleteContentForward":
		after = nextRuneBytes(text, start)
	case "deleteWordForward":
		after = nextWordEnd(text, start) - start
	default:
		return gpucontext.IMEDeleteSurroundingEvent{}, false
	}
	deletion := gpucontext.IMEDeleteSurroundingEvent{Before: before, After: after}
	if !deletion.IsValid() || before == 0 && after == 0 {
		return gpucontext.IMEDeleteSurroundingEvent{}, false
	}
	return deletion, true
}

func browserSelection(ev js.Value, text string, surrounding gpucontext.IMESurroundingText) (start, end int, ok bool) {
	target := ev.Get("target")
	if !target.IsUndefined() && !target.IsNull() {
		startValue, endValue := target.Get("selectionStart"), target.Get("selectionEnd")
		if !startValue.IsUndefined() && !startValue.IsNull() && !endValue.IsUndefined() && !endValue.IsNull() {
			return startValue.Int(), endValue.Int(), true
		}
	}
	if surrounding.IsValid() {
		return utf8OffsetToUTF16(text, surrounding.Cursor), utf8OffsetToUTF16(text, surrounding.Anchor), true
	}
	return 0, 0, false
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

	scaleChanged := w.lastScale != 0 && w.lastScale != dpr
	w.lastScale = dpr

	return PrepareFrameResult{
		ScaleChanged:   changed || scaleChanged,
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

// RequestSize sets the canvas element dimensions to the given logical size (DIP).
// Adjusts both the CSS layout size and the canvas drawing buffer for HiDPI.
func (w *browserWindow) RequestSize(width, height int) {
	dpr := js.Global().Get("devicePixelRatio").Float()
	if dpr <= 0 {
		dpr = 1.0
	}
	style := w.canvas.Get("style")
	style.Set("width", js.ValueOf(fmt.Sprintf("%dpx", width)))
	style.Set("height", js.ValueOf(fmt.Sprintf("%dpx", height)))
	w.canvas.Set("width", int(float64(width)*dpr))
	w.canvas.Set("height", int(float64(height)*dpr))
}

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

// Hide is a no-op on browser — the canvas is always visible.
func (w *browserWindow) Hide() {}

// SetPosition is a no-op on browser — the page position is fixed.
func (w *browserWindow) SetPosition(_, _ int) {}

// SyncFrame is a no-op — browser compositing is handled by requestAnimationFrame.
func (w *browserWindow) SyncFrame() {}

// SetCursorMode is a no-op on browser (pointer lock requires Pointer Lock API).
func (w *browserWindow) SetCursorMode(_ int) {}

// CursorMode returns 0 (normal mode).
func (w *browserWindow) CursorMode() int { return 0 }

// SetModalFrameCallback is a no-op — browser has no modal resize loops.
func (w *browserWindow) SetModalFrameCallback(_ func()) {}

// StartDrag initiates an outgoing drag via HTML5 Drag and Drop API.
// In the browser, drag operations require user gesture context (dragstart event).
// Since we cannot programmatically initiate a drag without a native dragstart,
// this implementation sets up the data transfer so that the next native dragstart
// on the canvas will carry the file paths. The done callback fires immediately
// with DragCancelled because HTML5 drag requires the browser's own gesture.
func (w *browserWindow) StartDrag(paths []string, done func(DragResult)) {
	// HTML5 Drag and Drop requires the dragstart event to originate from the
	// browser's native event handling. We cannot programmatically start a drag
	// from Go/WASM. The proper pattern is to listen for dragstart on the canvas
	// and populate dataTransfer there.
	//
	// For now, we report cancellation since programmatic drag initiation is not
	// possible in the browser security model.
	if done != nil {
		done(DragCancelled)
	}
}

// Destroy releases JS callbacks.
func (w *browserWindow) Destroy() {
	if w == nil || w.destroyed {
		return
	}
	w.destroyed = true
	w.imeEnabled = false
	w.imeTracker.setEnabled(false)
	w.imeSurrounding = gpucontext.IMESurroundingText{}
	w.imeSurroundingSet = false
	if !w.imeInput.IsNull() && !w.imeInput.IsUndefined() {
		parent := w.imeInput.Get("parentNode")
		if !parent.IsNull() && !parent.IsUndefined() {
			parent.Call("removeChild", w.imeInput)
		}
		w.imeInput = js.Null()
	}
	for _, cb := range w.jsCallbacks {
		cb.Release()
	}
	w.jsCallbacks = nil
}

var _ PlatformWindow = (*browserWindow)(nil)
var _ gpucontext.IMEControllerV2 = (*browserWindow)(nil)
var _ gpucontext.IMECapabilityProviderV2 = (*browserWindow)(nil)

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
