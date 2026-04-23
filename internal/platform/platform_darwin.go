//go:build darwin

package platform

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gpucontext"
)

// darwinWindow holds all per-window state for a macOS window.
type darwinWindow struct {
	window      *darwin.Window
	surface     *darwin.Surface
	config      Config
	shouldClose bool
	events      []Event
	eventMu     sync.Mutex

	// Mouse state tracking
	pointerX      float64
	pointerY      float64
	buttons       gpucontext.Buttons
	modifiers     gpucontext.Modifiers
	mouseInWindow bool

	// Frameless window state
	frameless       bool
	hitTestCallback func(x, y float64) gpucontext.HitTestResult

	// Callbacks for pointer, scroll, keyboard, and character input events
	pointerCallback  func(gpucontext.PointerEvent)
	scrollCallback   func(gpucontext.ScrollEvent)
	keyboardCallback func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)
	charCallback     func(rune)
	callbackMu       sync.RWMutex

	// Timestamp reference for event timing
	startTime time.Time

	// Last known scale factor for change detection in PrepareFrame.
	lastScale float64
}

// darwinPlatform implements Platform for macOS using Cocoa/AppKit.
// Holds process-level state and a primary window for single-window API.
type darwinPlatform struct {
	mu  sync.Mutex
	app *darwin.Application

	// Primary window for backward-compatible single-window API.
	primary *darwinWindow
}

// newPlatformManager returns a PlatformManager for macOS.
func newPlatformManager() PlatformManager {
	return &darwinPlatform{}
}

// --- PlatformManager implementation on darwinPlatform ---

// Init initializes the macOS platform subsystem (process-level, no window).
func (p *darwinPlatform) Init() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.app = darwin.GetApplication()
	return p.app.Init()
}

// CreateWindow creates a macOS window with the given configuration.
func (p *darwinPlatform) CreateWindow(config Config) (PlatformWindow, error) {
	w := &darwinWindow{
		config:    config,
		frameless: config.Frameless,
		startTime: time.Now(),
	}

	windowConfig := darwin.WindowConfig{
		Title:      config.Title,
		Width:      config.Width,
		Height:     config.Height,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
		Frameless:  config.Frameless,
	}

	window, err := darwin.NewWindow(windowConfig)
	if err != nil {
		return nil, err
	}
	w.window = window

	// Create Metal surface for GPU rendering.
	// Note: Surface is created before window is shown, but drawable size
	// is set after Show() when window has valid dimensions.
	surface, err := darwin.NewSurface(window)
	if err != nil {
		// Non-fatal: window works without Metal surface
		// This allows the window to still be used with software rendering
		w.surface = nil
	} else {
		w.surface = surface
	}

	// Show window - this makes the window visible and gives it valid dimensions
	w.window.Show()

	// Update surface size now that window is visible.
	// This ensures CAMetalLayer has correct drawable dimensions
	// and avoids "ignoring invalid setDrawableSize" warnings.
	if w.surface != nil {
		w.surface.UpdateSize()
	}

	p.primary = w
	return &darwinPlatformWindow{platform: p, id: NewWindowID()}, nil
}

// PollEvents processes pending macOS events.
func (p *darwinPlatform) PollEvents() Event {
	return p.primary.pollEvents(p.app)
}

// WaitEvents blocks until at least one OS event is available, then processes
// all pending events. Uses [NSApp nextEventMatchingMask:untilDate:inMode:dequeue:]
// with distantFuture, which blocks at kernel level via mach_msg for 0% CPU idle.
func (p *darwinPlatform) WaitEvents() {
	w := p.primary
	w.eventMu.Lock()
	defer w.eventMu.Unlock()

	if p.app != nil {
		p.app.WaitEventsWithHandler(w.handleEvent)
	}

	// Check for resize after processing events
	w.checkResize()
}

// WakeUp unblocks WaitEvents from any goroutine by posting a synthetic
// NSEventTypeApplicationDefined event. This is thread-safe per Apple
// documentation and is the standard pattern used by GLFW, winit, SDL, and Qt.
func (p *darwinPlatform) WakeUp() {
	if p.app != nil {
		p.app.PostEmptyEvent()
	}
}

// Destroy closes all windows and releases resources.
func (p *darwinPlatform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary
	if w != nil {
		if w.surface != nil {
			w.surface.Destroy()
			w.surface = nil
		}

		if w.window != nil {
			w.window.Destroy()
			w.window = nil
		}
	}

	if p.app != nil {
		p.app.Destroy()
		p.app = nil
	}
}

// --- darwinPlatformWindow implements PlatformWindow ---

// darwinPlatformWindow wraps darwinPlatform to implement PlatformWindow.
type darwinPlatformWindow struct {
	platform *darwinPlatform
	id       WindowID
}

func (dw *darwinPlatformWindow) ID() WindowID { return dw.id }

func (dw *darwinPlatformWindow) GetHandle() (instance, window uintptr) {
	w := dw.platform.primary
	if w.surface != nil {
		return 0, w.surface.LayerPtr()
	}
	if w.window != nil {
		return 0, w.window.ViewHandle()
	}
	return 0, 0
}

func (dw *darwinPlatformWindow) LogicalSize() (int, int) {
	w := dw.platform.primary
	if w.window != nil {
		return w.window.Size()
	}
	return w.config.Width, w.config.Height
}

func (dw *darwinPlatformWindow) PhysicalSize() (int, int) {
	w := dw.platform.primary
	if w.window != nil {
		return w.window.FramebufferSize()
	}
	return w.config.Width, w.config.Height
}

func (dw *darwinPlatformWindow) ScaleFactor() float64 {
	w := dw.platform.primary
	if w.window == nil {
		return 1.0
	}
	return w.window.BackingScaleFactor()
}

func (dw *darwinPlatformWindow) ShouldClose() bool {
	w := dw.platform.primary
	if w.window != nil {
		return w.window.ShouldClose() || w.shouldClose
	}
	return w.shouldClose
}

func (dw *darwinPlatformWindow) InSizeMove() bool  { return false }
func (dw *darwinPlatformWindow) SetTitle(_ string) {}

func (dw *darwinPlatformWindow) PrepareFrame() PrepareFrameResult {
	w := dw.platform.primary
	if w.window == nil {
		return PrepareFrameResult{ScaleFactor: 1.0}
	}

	scale := w.window.BackingScaleFactor()
	physW, physH := w.window.FramebufferSize()

	scaleChanged := w.lastScale != 0 && w.lastScale != scale
	w.lastScale = scale

	if w.surface != nil && scale > 0 {
		w.surface.Layer().SetContentsScale(scale)
	}

	return PrepareFrameResult{
		ScaleChanged:   scaleChanged,
		ScaleFactor:    scale,
		PhysicalWidth:  uint32(physW),
		PhysicalHeight: uint32(physH),
	}
}

func (dw *darwinPlatformWindow) SetCursor(cursorID int) {
	dw.platform.setCursorImpl(cursorID)
}

func (dw *darwinPlatformWindow) SetCursorMode(int) {}
func (dw *darwinPlatformWindow) CursorMode() int   { return 0 }
func (dw *darwinPlatformWindow) SyncFrame()        {}

func (dw *darwinPlatformWindow) SetFrameless(frameless bool) {
	w := dw.platform.primary
	w.callbackMu.Lock()
	w.frameless = frameless
	w.callbackMu.Unlock()

	if w.window != nil {
		if frameless {
			w.window.SetStyleMask(darwin.NSWindowStyleMaskBorderless | darwin.NSWindowStyleMaskResizable)
		} else {
			w.window.SetStyleMask(
				darwin.NSWindowStyleMaskTitled | darwin.NSWindowStyleMaskClosable |
					darwin.NSWindowStyleMaskMiniaturizable | darwin.NSWindowStyleMaskResizable)
		}
	}
}

func (dw *darwinPlatformWindow) IsFrameless() bool {
	w := dw.platform.primary
	w.callbackMu.RLock()
	defer w.callbackMu.RUnlock()
	return w.frameless
}

func (dw *darwinPlatformWindow) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	w := dw.platform.primary
	w.callbackMu.Lock()
	defer w.callbackMu.Unlock()
	w.hitTestCallback = fn
}

func (dw *darwinPlatformWindow) Minimize() {
	w := dw.platform.primary
	if w.window != nil {
		w.window.Miniaturize()
	}
}

func (dw *darwinPlatformWindow) Maximize() {
	w := dw.platform.primary
	if w.window != nil {
		w.window.Zoom()
	}
}

func (dw *darwinPlatformWindow) IsMaximized() bool {
	w := dw.platform.primary
	if w.window != nil {
		return w.window.IsZoomed()
	}
	return false
}

func (dw *darwinPlatformWindow) Close() {
	w := dw.platform.primary
	if w.window != nil {
		w.window.Close()
	}
}

func (dw *darwinPlatformWindow) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	w := dw.platform.primary
	w.callbackMu.Lock()
	w.pointerCallback = fn
	w.callbackMu.Unlock()
}

func (dw *darwinPlatformWindow) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	w := dw.platform.primary
	w.callbackMu.Lock()
	w.scrollCallback = fn
	w.callbackMu.Unlock()
}

func (dw *darwinPlatformWindow) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	w := dw.platform.primary
	w.callbackMu.Lock()
	w.keyboardCallback = fn
	w.callbackMu.Unlock()
}

func (dw *darwinPlatformWindow) SetCharCallback(fn func(rune)) {
	w := dw.platform.primary
	w.callbackMu.Lock()
	w.charCallback = fn
	w.callbackMu.Unlock()
}

func (dw *darwinPlatformWindow) SetModalFrameCallback(_ func()) {}

func (dw *darwinPlatformWindow) BlitPixels(pixels []byte, width, height int) error {
	w := dw.platform.primary
	if w.window == nil {
		return fmt.Errorf("gogpu: darwin BlitPixels: no window")
	}

	cgImage, err := darwin.CreateCGImageFromRGBA(pixels, width, height)
	if err != nil {
		return fmt.Errorf("gogpu: darwin BlitPixels: %w", err)
	}
	defer darwin.ReleaseCGImage(cgImage)

	contentView := w.window.ContentView()
	if contentView.IsNil() {
		return fmt.Errorf("gogpu: darwin BlitPixels: no content view")
	}

	contentView.SendBool(darwin.RegisterSelector("setWantsLayer:"), true)

	layerID := contentView.Send(darwin.RegisterSelector("layer"))
	if layerID.IsNil() {
		return fmt.Errorf("gogpu: darwin BlitPixels: no layer")
	}

	layerID.SendPtr(darwin.RegisterSelector("setContents:"), cgImage)
	layerID.SendBool(darwin.RegisterSelector("setNeedsDisplay:"), true)
	contentView.SendBool(darwin.RegisterSelector("setNeedsDisplay:"), true)

	return nil
}

func (dw *darwinPlatformWindow) Destroy() {
	// Destruction handled by platform.Destroy()
}

func (w *darwinWindow) pollEvents(app *darwin.Application) Event {
	w.eventMu.Lock()
	defer w.eventMu.Unlock()

	// Return queued event first (from previous processing).
	if len(w.events) > 0 {
		event := w.events[0]
		w.events = w.events[1:]
		return event
	}

	// Process OS events with our handler — queues pointer/key/scroll events.
	if app != nil {
		app.PollEventsWithHandler(w.handleEvent)
	}

	// Check if window should close — queue once, not every call.
	if !w.shouldClose && w.window != nil && w.window.ShouldClose() {
		w.shouldClose = true
		w.events = append(w.events, Event{Type: EventClose})
	}

	// Check for resize — queue if size changed.
	// RETINA-002: Do NOT call w.surface.Resize() here. PollEvents runs on the
	// main thread while the render thread operates on wgpu surface. Surface
	// reconfiguration is handled by the render thread via RequestResize.
	if w.window != nil {
		oldWidth, oldHeight := w.config.Width, w.config.Height
		w.window.UpdateSize()
		newWidth, newHeight := w.window.Size() // logical points

		if newWidth != oldWidth || newHeight != oldHeight {
			w.config.Width = newWidth
			w.config.Height = newHeight
			physW, physH := w.window.FramebufferSize()
			w.events = append(w.events, Event{
				Type:           EventResize,
				Width:          newWidth,
				Height:         newHeight,
				PhysicalWidth:  physW,
				PhysicalHeight: physH,
			})
		}
	}

	// Return first queued event, or EventNone.
	if len(w.events) > 0 {
		event := w.events[0]
		w.events = w.events[1:]
		return event
	}

	return Event{Type: EventNone}
}

// handleEvent is called for each NSEvent during polling.
// It processes pointer and scroll events and dispatches them to callbacks.
// Returns true to let the event be dispatched to the application.
// Called with w.eventMu held.
func (w *darwinWindow) handleEvent(event darwin.ID, eventType darwin.NSEventType) bool {
	// Get event info
	info := darwin.GetEventInfo(event)

	// RETINA-001: Coordinates are in logical points (Cocoa points / DIP).
	// w.config.Width/Height are now logical, matching NSEvent coordinates.
	// No scaling needed — the coordinate system is consistently logical.

	// Y coordinate flip: macOS uses bottom-left origin, we need top-left.
	y := float64(w.config.Height) - info.LocationY

	// Update modifiers
	w.modifiers = extractModifiers(info.ModifierFlags)

	switch eventType {
	// Mouse button down events
	case darwin.NSEventTypeLeftMouseDown:
		w.buttons |= gpucontext.ButtonsLeft
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonLeft, info, y)
		w.dispatchPointerEvent(ev)

	case darwin.NSEventTypeRightMouseDown:
		w.buttons |= gpucontext.ButtonsRight
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonRight, info, y)
		w.dispatchPointerEvent(ev)

	case darwin.NSEventTypeOtherMouseDown:
		btn := buttonFromNumber(info.ButtonNumber)
		w.buttons |= buttonsFromNumber(info.ButtonNumber)
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerDown, btn, info, y)
		w.dispatchPointerEvent(ev)

	// Mouse button up events
	case darwin.NSEventTypeLeftMouseUp:
		w.buttons &^= gpucontext.ButtonsLeft
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonLeft, info, y)
		w.dispatchPointerEvent(ev)

	case darwin.NSEventTypeRightMouseUp:
		w.buttons &^= gpucontext.ButtonsRight
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonRight, info, y)
		w.dispatchPointerEvent(ev)

	case darwin.NSEventTypeOtherMouseUp:
		btn := buttonFromNumber(info.ButtonNumber)
		w.buttons &^= buttonsFromNumber(info.ButtonNumber)
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerUp, btn, info, y)
		w.dispatchPointerEvent(ev)

	// Mouse move events
	case darwin.NSEventTypeMouseMoved:
		wasInWindow := w.mouseInWindow
		w.pointerX = info.LocationX
		w.pointerY = y

		// Detect enter/leave based on position (in logical point coordinates).
		// w.config.Width/Height are in logical points after RETINA-001.
		inWindow := info.LocationX >= 0 && info.LocationX <= float64(w.config.Width) &&
			y >= 0 && y <= float64(w.config.Height)

		if inWindow && !wasInWindow {
			w.mouseInWindow = true
			ev := w.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, info, y)
			w.dispatchPointerEvent(ev)
		} else if !inWindow && wasInWindow {
			w.mouseInWindow = false
			ev := w.createPointerEvent(gpucontext.PointerLeave, gpucontext.ButtonNone, info, y)
			w.dispatchPointerEvent(ev)
		}

		// Always send move event
		ev := w.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, info, y)
		w.dispatchPointerEvent(ev)

	// Mouse drag events (move with button pressed)
	case darwin.NSEventTypeLeftMouseDragged,
		darwin.NSEventTypeRightMouseDragged,
		darwin.NSEventTypeOtherMouseDragged:
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, info, y)
		w.dispatchPointerEvent(ev)

	// Mouse enter/exit events (for tracking areas)
	case darwin.NSEventTypeMouseEntered:
		w.mouseInWindow = true
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, info, y)
		w.dispatchPointerEvent(ev)

	case darwin.NSEventTypeMouseExited:
		w.mouseInWindow = false
		w.pointerX = info.LocationX
		w.pointerY = y
		ev := w.createPointerEvent(gpucontext.PointerLeave, gpucontext.ButtonNone, info, y)
		w.dispatchPointerEvent(ev)

	// Scroll wheel
	case darwin.NSEventTypeScrollWheel:
		// Determine delta mode based on precision
		deltaMode := gpucontext.ScrollDeltaLine
		deltaX := info.ScrollDeltaX
		deltaY := -info.ScrollDeltaY // Invert Y: natural scrolling convention
		if info.IsPrecise {
			deltaMode = gpucontext.ScrollDeltaPixel
			// Precise (trackpad) deltas are in logical points, matching our
			// logical coordinate system. No scaling needed.
		}

		ev := gpucontext.ScrollEvent{
			X:         info.LocationX,
			Y:         y,
			DeltaX:    deltaX,
			DeltaY:    deltaY,
			DeltaMode: deltaMode,
			Modifiers: w.modifiers,
			Timestamp: w.eventTimestamp(),
		}
		w.dispatchScrollEvent(ev)

	// Keyboard events
	case darwin.NSEventTypeKeyDown:
		keyCode := darwin.GetKeyCode(event)
		key := macKeyCodeToKey(keyCode)
		w.dispatchKeyEvent(key, w.modifiers, true)

		// Dispatch character input from [NSEvent characters].
		// This handles all keyboard layouts, IME, and dead key sequences.
		w.dispatchCharFromEvent(event)

	case darwin.NSEventTypeKeyUp:
		keyCode := darwin.GetKeyCode(event)
		key := macKeyCodeToKey(keyCode)
		w.dispatchKeyEvent(key, w.modifiers, false)

	case darwin.NSEventTypeFlagsChanged:
		// Modifier key state changed
		// Detect which modifier key was pressed/released by comparing flags
		keyCode := darwin.GetKeyCode(event)
		key, pressed := detectModifierKeyChange(keyCode, info.ModifierFlags)
		if key != gpucontext.KeyUnknown {
			w.dispatchKeyEvent(key, w.modifiers, pressed)
		}
	}

	// Let all events be dispatched to the application
	return true
}

// dispatchPointerEvent dispatches a pointer event to the registered callback.
// Called from handleEvent with w.eventMu held. Releases eventMu during callback
// to avoid deadlocks if the callback calls back into platform methods.
func (w *darwinWindow) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	w.callbackMu.RLock()
	callback := w.pointerCallback
	w.callbackMu.RUnlock()

	if callback != nil {
		w.eventMu.Unlock()
		callback(ev)
		w.eventMu.Lock()
	}
}

// dispatchScrollEvent dispatches a scroll event to the registered callback.
// Called from handleEvent with w.eventMu held.
func (w *darwinWindow) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	w.callbackMu.RLock()
	callback := w.scrollCallback
	w.callbackMu.RUnlock()

	if callback != nil {
		w.eventMu.Unlock()
		callback(ev)
		w.eventMu.Lock()
	}
}

// dispatchKeyEvent dispatches a keyboard event to the registered callback.
// Called from handleEvent with w.eventMu held.
func (w *darwinWindow) dispatchKeyEvent(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	w.callbackMu.RLock()
	callback := w.keyboardCallback
	w.callbackMu.RUnlock()

	if callback != nil {
		w.eventMu.Unlock()
		callback(key, mods, pressed)
		w.eventMu.Lock()
	}
}

// dispatchCharFromEvent extracts characters from an NSEvent and dispatches them.
// Called from handleEvent with w.eventMu held.
func (w *darwinWindow) dispatchCharFromEvent(event darwin.ID) {
	w.callbackMu.RLock()
	callback := w.charCallback
	w.callbackMu.RUnlock()

	if callback == nil {
		return
	}

	// Get [NSEvent characters] → NSString
	nsstr := darwin.GetCharacters(event)
	if nsstr.IsNil() {
		return
	}

	// Get UTF-8 C string pointer
	utf8Ptr := darwin.NSStringUTF8Ptr(nsstr)
	if utf8Ptr == 0 {
		return
	}

	// Read C string into Go string
	length := darwin.NSStringLength(nsstr)
	if length == 0 {
		return
	}

	// Convert to Go byte slice (safe: pointer valid within this autorelease pool scope)
	data := unsafe.Slice((*byte)(unsafe.Pointer(utf8Ptr)), length*4) //nolint:govet // ObjC UTF8String pointer, bounded by NSString length

	// Release lock before calling user callback to avoid deadlocks
	w.eventMu.Unlock()

	// Decode UTF-8 runes and dispatch each non-control character
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			break // end of valid UTF-8
		}
		if r >= 32 && r != 127 {
			callback(r)
		}
		i += size
	}

	w.eventMu.Lock()
}

// setCursorImpl changes the mouse cursor shape using NSCursor.
// cursorID maps to gpucontext.CursorShape values (0-11).
func (p *darwinPlatform) setCursorImpl(cursorID int) {
	cursorClass := darwin.GetClass("NSCursor")
	if cursorClass == 0 {
		return
	}

	var cursor darwin.ID
	switch cursorID {
	case 0: // CursorDefault
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 1: // CursorPointer
		cursor = cursorClass.Send(darwin.RegisterSelector("pointingHandCursor"))
	case 2: // CursorText
		cursor = cursorClass.Send(darwin.RegisterSelector("IBeamCursor"))
	case 3: // CursorCrosshair
		cursor = cursorClass.Send(darwin.RegisterSelector("crosshairCursor"))
	case 4: // CursorMove
		cursor = cursorClass.Send(darwin.RegisterSelector("openHandCursor"))
	case 5: // CursorResizeNS
		cursor = cursorClass.Send(darwin.RegisterSelector("resizeUpDownCursor"))
	case 6: // CursorResizeEW
		cursor = cursorClass.Send(darwin.RegisterSelector("resizeLeftRightCursor"))
	case 7: // CursorResizeNWSE
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 8: // CursorResizeNESW
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 9: // CursorNotAllowed
		cursor = cursorClass.Send(darwin.RegisterSelector("operationNotAllowedCursor"))
	case 10: // CursorWait
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 11: // CursorNone
		cursorClass.Send(darwin.RegisterSelector("hide"))
		return
	default:
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	}

	if !cursor.IsNil() {
		cursor.Send(darwin.RegisterSelector("set"))
	}
}

// queueEvent adds an event to the event queue.
func (w *darwinWindow) queueEvent(event Event) {
	w.events = append(w.events, event)
}

// checkResize checks for window size changes and queues a resize event.
// RETINA-002: Does NOT call w.surface.Resize(). Surface reconfiguration
// is handled by the render thread via RequestResize to avoid race conditions.
// Must be called with w.eventMu held.
func (w *darwinWindow) checkResize() {
	if w.window == nil {
		return
	}

	oldWidth, oldHeight := w.config.Width, w.config.Height
	w.window.UpdateSize()
	newWidth, newHeight := w.window.Size() // logical points

	if newWidth != oldWidth || newHeight != oldHeight {
		w.config.Width = newWidth
		w.config.Height = newHeight

		physW, physH := w.window.FramebufferSize()

		w.queueEvent(Event{
			Type:           EventResize,
			Width:          newWidth,
			Height:         newHeight,
			PhysicalWidth:  physW,
			PhysicalHeight: physH,
		})
	}
}

// eventTimestamp returns the event timestamp as duration since start.
func (w *darwinWindow) eventTimestamp() time.Duration {
	return time.Since(w.startTime)
}

// extractModifiers converts NSEventModifierFlags to gpucontext.Modifiers.
func extractModifiers(flags darwin.NSEventModifierFlags) gpucontext.Modifiers {
	var mods gpucontext.Modifiers
	if flags&darwin.NSEventModifierFlagShift != 0 {
		mods |= gpucontext.ModShift
	}
	if flags&darwin.NSEventModifierFlagControl != 0 {
		mods |= gpucontext.ModControl
	}
	if flags&darwin.NSEventModifierFlagOption != 0 {
		mods |= gpucontext.ModAlt
	}
	if flags&darwin.NSEventModifierFlagCommand != 0 {
		mods |= gpucontext.ModSuper
	}
	return mods
}

// buttonFromNumber converts NSEvent buttonNumber to gpucontext.Button.
func buttonFromNumber(buttonNumber int64) gpucontext.Button {
	switch buttonNumber {
	case 0:
		return gpucontext.ButtonLeft
	case 1:
		return gpucontext.ButtonRight
	case 2:
		return gpucontext.ButtonMiddle
	case 3:
		return gpucontext.ButtonX1
	case 4:
		return gpucontext.ButtonX2
	default:
		return gpucontext.ButtonNone
	}
}

// buttonsFromNumber returns the Buttons bitmask for a button number.
func buttonsFromNumber(buttonNumber int64) gpucontext.Buttons {
	switch buttonNumber {
	case 0:
		return gpucontext.ButtonsLeft
	case 1:
		return gpucontext.ButtonsRight
	case 2:
		return gpucontext.ButtonsMiddle
	case 3:
		return gpucontext.ButtonsX1
	case 4:
		return gpucontext.ButtonsX2
	default:
		return gpucontext.ButtonsNone
	}
}

// createPointerEvent creates a PointerEvent with common fields filled in.
// Detects pen/tablet input from NSEvent subtype and sets PointerType,
// Pressure, TiltX, TiltY, and Twist accordingly.
func (w *darwinWindow) createPointerEvent(
	eventType gpucontext.PointerEventType,
	button gpucontext.Button,
	info darwin.EventInfo,
	y float64,
) gpucontext.PointerEvent {
	pointerType := gpucontext.PointerTypeMouse
	var pressure float32
	var tiltX, tiltY float32
	var twist float32

	// Detect pen/tablet input from NSEvent subtype
	if info.Subtype == darwin.NSEventSubtypeTabletPoint {
		pointerType = gpucontext.PointerTypePen
		pressure = float32(info.Pressure)
		// NSEvent tilt is -1.0 to 1.0, PointerEvent tiltX/Y is degrees -90 to 90
		tiltX = float32(info.TiltX * 90.0)
		tiltY = float32(info.TiltY * 90.0)
		twist = float32(info.Rotation)
	} else if eventType == gpucontext.PointerDown || w.buttons != gpucontext.ButtonsNone {
		// Regular mouse: default pressure when buttons are active
		pressure = 0.5
	}

	return gpucontext.PointerEvent{
		Type:        eventType,
		PointerID:   1,
		X:           info.LocationX,
		Y:           y,
		Pressure:    pressure,
		TiltX:       tiltX,
		TiltY:       tiltY,
		Twist:       twist,
		Width:       1,
		Height:      1,
		PointerType: pointerType,
		IsPrimary:   true,
		Button:      button,
		Buttons:     w.buttons,
		Modifiers:   w.modifiers,
		Timestamp:   w.eventTimestamp(),
	}
}

// macKeyCodeToKey converts macOS virtual key codes to gpucontext.Key.
func macKeyCodeToKey(keyCode uint16) gpucontext.Key { //nolint:maintidx // key mapping tables are inherently large
	switch keyCode {
	// Letters (QWERTY layout)
	case 0x00:
		return gpucontext.KeyA
	case 0x01:
		return gpucontext.KeyS
	case 0x02:
		return gpucontext.KeyD
	case 0x03:
		return gpucontext.KeyF
	case 0x04:
		return gpucontext.KeyH
	case 0x05:
		return gpucontext.KeyG
	case 0x06:
		return gpucontext.KeyZ
	case 0x07:
		return gpucontext.KeyX
	case 0x08:
		return gpucontext.KeyC
	case 0x09:
		return gpucontext.KeyV
	case 0x0B:
		return gpucontext.KeyB
	case 0x0C:
		return gpucontext.KeyQ
	case 0x0D:
		return gpucontext.KeyW
	case 0x0E:
		return gpucontext.KeyE
	case 0x0F:
		return gpucontext.KeyR
	case 0x10:
		return gpucontext.KeyY
	case 0x11:
		return gpucontext.KeyT
	case 0x12:
		return gpucontext.Key1
	case 0x13:
		return gpucontext.Key2
	case 0x14:
		return gpucontext.Key3
	case 0x15:
		return gpucontext.Key4
	case 0x16:
		return gpucontext.Key6
	case 0x17:
		return gpucontext.Key5
	case 0x18:
		return gpucontext.KeyEqual
	case 0x19:
		return gpucontext.Key9
	case 0x1A:
		return gpucontext.Key7
	case 0x1B:
		return gpucontext.KeyMinus
	case 0x1C:
		return gpucontext.Key8
	case 0x1D:
		return gpucontext.Key0
	case 0x1E:
		return gpucontext.KeyRightBracket
	case 0x1F:
		return gpucontext.KeyO
	case 0x20:
		return gpucontext.KeyU
	case 0x21:
		return gpucontext.KeyLeftBracket
	case 0x22:
		return gpucontext.KeyI
	case 0x23:
		return gpucontext.KeyP
	case 0x25:
		return gpucontext.KeyL
	case 0x26:
		return gpucontext.KeyJ
	case 0x27:
		return gpucontext.KeyApostrophe
	case 0x28:
		return gpucontext.KeyK
	case 0x29:
		return gpucontext.KeySemicolon
	case 0x2A:
		return gpucontext.KeyBackslash
	case 0x2B:
		return gpucontext.KeyComma
	case 0x2C:
		return gpucontext.KeySlash
	case 0x2D:
		return gpucontext.KeyN
	case 0x2E:
		return gpucontext.KeyM
	case 0x2F:
		return gpucontext.KeyPeriod
	case 0x32:
		return gpucontext.KeyGrave

	// Special keys
	case 0x24:
		return gpucontext.KeyEnter
	case 0x30:
		return gpucontext.KeyTab
	case 0x31:
		return gpucontext.KeySpace
	case 0x33:
		return gpucontext.KeyBackspace
	case 0x35:
		return gpucontext.KeyEscape
	case 0x37:
		return gpucontext.KeyLeftSuper // Command
	case 0x38:
		return gpucontext.KeyLeftShift
	case 0x39:
		return gpucontext.KeyCapsLock
	case 0x3A:
		return gpucontext.KeyLeftAlt // Option
	case 0x3B:
		return gpucontext.KeyLeftControl
	case 0x3C:
		return gpucontext.KeyRightShift
	case 0x3D:
		return gpucontext.KeyRightAlt
	case 0x3E:
		return gpucontext.KeyRightControl
	case 0x36:
		return gpucontext.KeyRightSuper

	// Function keys
	case 0x7A:
		return gpucontext.KeyF1
	case 0x78:
		return gpucontext.KeyF2
	case 0x63:
		return gpucontext.KeyF3
	case 0x76:
		return gpucontext.KeyF4
	case 0x60:
		return gpucontext.KeyF5
	case 0x61:
		return gpucontext.KeyF6
	case 0x62:
		return gpucontext.KeyF7
	case 0x64:
		return gpucontext.KeyF8
	case 0x65:
		return gpucontext.KeyF9
	case 0x6D:
		return gpucontext.KeyF10
	case 0x67:
		return gpucontext.KeyF11
	case 0x6F:
		return gpucontext.KeyF12

	// Navigation
	case 0x73:
		return gpucontext.KeyHome
	case 0x77:
		return gpucontext.KeyEnd
	case 0x74:
		return gpucontext.KeyPageUp
	case 0x79:
		return gpucontext.KeyPageDown
	case 0x75:
		return gpucontext.KeyDelete
	case 0x72:
		return gpucontext.KeyInsert

	// Arrow keys
	case 0x7B:
		return gpucontext.KeyLeft
	case 0x7C:
		return gpucontext.KeyRight
	case 0x7D:
		return gpucontext.KeyDown
	case 0x7E:
		return gpucontext.KeyUp

	// Numpad
	case 0x52:
		return gpucontext.KeyNumpad0
	case 0x53:
		return gpucontext.KeyNumpad1
	case 0x54:
		return gpucontext.KeyNumpad2
	case 0x55:
		return gpucontext.KeyNumpad3
	case 0x56:
		return gpucontext.KeyNumpad4
	case 0x57:
		return gpucontext.KeyNumpad5
	case 0x58:
		return gpucontext.KeyNumpad6
	case 0x59:
		return gpucontext.KeyNumpad7
	case 0x5B:
		return gpucontext.KeyNumpad8
	case 0x5C:
		return gpucontext.KeyNumpad9
	case 0x41:
		return gpucontext.KeyNumpadDecimal
	case 0x43:
		return gpucontext.KeyNumpadMultiply
	case 0x45:
		return gpucontext.KeyNumpadAdd
	case 0x47:
		return gpucontext.KeyNumLock
	case 0x4B:
		return gpucontext.KeyNumpadDivide
	case 0x4C:
		return gpucontext.KeyNumpadEnter
	case 0x4E:
		return gpucontext.KeyNumpadSubtract

	default:
		return gpucontext.KeyUnknown
	}
}

// ClipboardRead reads text from the system clipboard using NSPasteboard.
func (p *darwinPlatform) ClipboardRead() (string, error) {
	pb := darwin.GetClass("NSPasteboard").Send(darwin.RegisterSelector("generalPasteboard"))
	if pb.IsNil() {
		return "", nil
	}

	// Request public.utf8-plain-text type
	typeStr := darwin.NewNSString("public.utf8-plain-text")
	if typeStr == nil {
		return "", nil
	}
	defer typeStr.Release()

	nsstr := pb.SendPtr(darwin.RegisterSelector("stringForType:"), uintptr(typeStr.ID()))
	if nsstr.IsNil() {
		return "", nil
	}

	// Convert NSString to Go string
	utf8Ptr := darwin.NSStringUTF8Ptr(nsstr)
	if utf8Ptr == 0 {
		return "", nil
	}

	length := darwin.NSStringLength(nsstr)
	if length == 0 {
		return "", nil
	}

	// Read UTF-8 bytes (length is character count; UTF-8 may use up to 4 bytes per char)
	data := unsafe.Slice((*byte)(unsafe.Pointer(utf8Ptr)), length*4) //nolint:govet // ObjC UTF8String pointer, bounded by NSString length

	// Find actual end of the C string
	end := 0
	for end < len(data) && data[end] != 0 {
		end++
	}

	return string(data[:end]), nil
}

// ClipboardWrite writes text to the system clipboard using NSPasteboard.
func (p *darwinPlatform) ClipboardWrite(text string) error {
	pb := darwin.GetClass("NSPasteboard").Send(darwin.RegisterSelector("generalPasteboard"))
	if pb.IsNil() {
		return nil
	}

	// Clear existing contents
	pb.Send(darwin.RegisterSelector("clearContents"))

	// Create NSString with the text
	nsStr := darwin.NewNSString(text)
	if nsStr == nil {
		return nil
	}
	defer nsStr.Release()

	// Create type string
	typeStr := darwin.NewNSString("public.utf8-plain-text")
	if typeStr == nil {
		return nil
	}
	defer typeStr.Release()

	// setString:forType: takes two pointer arguments
	pb.SendUintUint(
		darwin.RegisterSelector("setString:forType:"),
		uint64(nsStr.ID()),
		uint64(typeStr.ID()),
	)

	return nil
}

// DarkMode returns true if the system dark mode is active.
// Checks NSApplication.effectiveAppearance.name for "Dark" substring.
func (p *darwinPlatform) DarkMode() bool {
	app := darwin.GetClass("NSApplication").Send(darwin.RegisterSelector("sharedApplication"))
	if app.IsNil() {
		return false
	}

	appearance := app.Send(darwin.RegisterSelector("effectiveAppearance"))
	if appearance.IsNil() {
		return false
	}

	nameID := appearance.Send(darwin.RegisterSelector("name"))
	if nameID.IsNil() {
		return false
	}

	// Get the UTF-8 string from the appearance name
	utf8Ptr := darwin.NSStringUTF8Ptr(nameID)
	if utf8Ptr == 0 {
		return false
	}

	length := darwin.NSStringLength(nameID)
	if length == 0 {
		return false
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(utf8Ptr)), length*4) //nolint:govet // ObjC UTF8String pointer, bounded by NSString length

	// Find actual string end
	end := 0
	for end < len(data) && data[end] != 0 {
		end++
	}

	name := string(data[:end])
	// macOS dark appearance names contain "Dark" (e.g., "NSAppearanceNameDarkAqua")
	return strings.Contains(name, "Dark")
}

// ReduceMotion returns true if the user prefers reduced animation.
// Uses NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion.
func (p *darwinPlatform) ReduceMotion() bool {
	ws := darwin.GetClass("NSWorkspace").Send(darwin.RegisterSelector("sharedWorkspace"))
	if ws.IsNil() {
		return false
	}
	return ws.GetBool(darwin.RegisterSelector("accessibilityDisplayShouldReduceMotion"))
}

// HighContrast returns true if high contrast mode is active.
// Uses NSWorkspace.sharedWorkspace.accessibilityDisplayShouldIncreaseContrast.
func (p *darwinPlatform) HighContrast() bool {
	ws := darwin.GetClass("NSWorkspace").Send(darwin.RegisterSelector("sharedWorkspace"))
	if ws.IsNil() {
		return false
	}
	return ws.GetBool(darwin.RegisterSelector("accessibilityDisplayShouldIncreaseContrast"))
}

// FontScale returns the font size preference multiplier.
// macOS does not have a system-wide font scale setting like Windows or Android.
// Individual apps control their own text sizing. Returns 1.0 (no scaling).
func (p *darwinPlatform) FontScale() float32 { return 1.0 }

// detectModifierKeyChange detects which modifier key was pressed/released.
// macOS sends NSEventTypeFlagsChanged for modifier keys instead of keyDown/keyUp.
func detectModifierKeyChange(keyCode uint16, flags darwin.NSEventModifierFlags) (gpucontext.Key, bool) {
	var key gpucontext.Key
	var flagMask darwin.NSEventModifierFlags

	switch keyCode {
	case 0x38: // Left Shift
		key = gpucontext.KeyLeftShift
		flagMask = darwin.NSEventModifierFlagShift
	case 0x3C: // Right Shift
		key = gpucontext.KeyRightShift
		flagMask = darwin.NSEventModifierFlagShift
	case 0x3B: // Left Control
		key = gpucontext.KeyLeftControl
		flagMask = darwin.NSEventModifierFlagControl
	case 0x3E: // Right Control
		key = gpucontext.KeyRightControl
		flagMask = darwin.NSEventModifierFlagControl
	case 0x3A: // Left Option (Alt)
		key = gpucontext.KeyLeftAlt
		flagMask = darwin.NSEventModifierFlagOption
	case 0x3D: // Right Option (Alt)
		key = gpucontext.KeyRightAlt
		flagMask = darwin.NSEventModifierFlagOption
	case 0x37: // Left Command (Super)
		key = gpucontext.KeyLeftSuper
		flagMask = darwin.NSEventModifierFlagCommand
	case 0x36: // Right Command (Super)
		key = gpucontext.KeyRightSuper
		flagMask = darwin.NSEventModifierFlagCommand
	case 0x39: // Caps Lock
		key = gpucontext.KeyCapsLock
		flagMask = darwin.NSEventModifierFlagCapsLock
	default:
		return gpucontext.KeyUnknown, false
	}

	// Check if the key is pressed (flag is set) or released (flag is cleared)
	pressed := (flags & flagMask) != 0
	return key, pressed
}
