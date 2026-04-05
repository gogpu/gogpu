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

// darwinPlatform implements Platform for macOS using Cocoa/AppKit.
type darwinPlatform struct {
	mu          sync.Mutex
	app         *darwin.Application
	window      *darwin.Window
	surface     *darwin.Surface
	config      Config
	shouldClose bool
	events      []Event

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

	// Timestamp reference for event timing
	startTime time.Time

	// Last known scale factor for change detection in PrepareFrame.
	lastScale float64
}

func newPlatform() Platform {
	return &darwinPlatform{
		startTime: time.Now(),
	}
}

func (p *darwinPlatform) Init(config Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config

	// Initialize NSApplication
	p.app = darwin.GetApplication()
	if err := p.app.Init(); err != nil {
		return err
	}

	p.frameless = config.Frameless

	// Create window
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
		return err
	}
	p.window = window

	// Create Metal surface for GPU rendering.
	// Note: Surface is created before window is shown, but drawable size
	// is set after Show() when window has valid dimensions.
	surface, err := darwin.NewSurface(window)
	if err != nil {
		// Non-fatal: window works without Metal surface
		// This allows the window to still be used with software rendering
		p.surface = nil
	} else {
		p.surface = surface
	}

	// Show window - this makes the window visible and gives it valid dimensions
	p.window.Show()

	// Update surface size now that window is visible.
	// This ensures CAMetalLayer has correct drawable dimensions
	// and avoids "ignoring invalid setDrawableSize" warnings.
	if p.surface != nil {
		p.surface.UpdateSize()
	}

	return nil
}

func (p *darwinPlatform) PollEvents() Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return queued event first (from previous processing).
	if len(p.events) > 0 {
		event := p.events[0]
		p.events = p.events[1:]
		return event
	}

	// Process OS events with our handler — queues pointer/key/scroll events.
	if p.app != nil {
		p.app.PollEventsWithHandler(p.handleEvent)
	}

	// Check if window should close — queue once, not every call.
	if !p.shouldClose && p.window != nil && p.window.ShouldClose() {
		p.shouldClose = true
		p.events = append(p.events, Event{Type: EventClose})
	}

	// Check for resize — queue if size changed.
	// RETINA-002: Do NOT call p.surface.Resize() here. PollEvents runs on the
	// main thread while the render thread operates on wgpu surface. Surface
	// reconfiguration is handled by the render thread via RequestResize.
	if p.window != nil {
		oldWidth, oldHeight := p.config.Width, p.config.Height
		p.window.UpdateSize()
		newWidth, newHeight := p.window.Size() // logical points

		if newWidth != oldWidth || newHeight != oldHeight {
			p.config.Width = newWidth
			p.config.Height = newHeight
			physW, physH := p.window.FramebufferSize()
			p.events = append(p.events, Event{
				Type:           EventResize,
				Width:          newWidth,
				Height:         newHeight,
				PhysicalWidth:  physW,
				PhysicalHeight: physH,
			})
		}
	}

	// Return first queued event, or EventNone.
	if len(p.events) > 0 {
		event := p.events[0]
		p.events = p.events[1:]
		return event
	}

	return Event{Type: EventNone}
}

// handleEvent is called for each NSEvent during polling.
// It processes pointer and scroll events and dispatches them to callbacks.
// Returns true to let the event be dispatched to the application.
func (p *darwinPlatform) handleEvent(event darwin.ID, eventType darwin.NSEventType) bool {
	// Get event info
	info := darwin.GetEventInfo(event)

	// RETINA-001: Coordinates are in logical points (Cocoa points / DIP).
	// p.config.Width/Height are now logical, matching NSEvent coordinates.
	// No scaling needed — the coordinate system is consistently logical.

	// Y coordinate flip: macOS uses bottom-left origin, we need top-left.
	y := float64(p.config.Height) - info.LocationY

	// Update modifiers
	p.modifiers = extractModifiers(info.ModifierFlags)

	switch eventType {
	// Mouse button down events
	case darwin.NSEventTypeLeftMouseDown:
		p.buttons |= gpucontext.ButtonsLeft
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonLeft, info, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeRightMouseDown:
		p.buttons |= gpucontext.ButtonsRight
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonRight, info, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeOtherMouseDown:
		btn := buttonFromNumber(info.ButtonNumber)
		p.buttons |= buttonsFromNumber(info.ButtonNumber)
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerDown, btn, info, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse button up events
	case darwin.NSEventTypeLeftMouseUp:
		p.buttons &^= gpucontext.ButtonsLeft
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonLeft, info, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeRightMouseUp:
		p.buttons &^= gpucontext.ButtonsRight
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonRight, info, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeOtherMouseUp:
		btn := buttonFromNumber(info.ButtonNumber)
		p.buttons &^= buttonsFromNumber(info.ButtonNumber)
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerUp, btn, info, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse move events
	case darwin.NSEventTypeMouseMoved:
		wasInWindow := p.mouseInWindow
		p.pointerX = info.LocationX
		p.pointerY = y

		// Detect enter/leave based on position (in logical point coordinates).
		// p.config.Width/Height are in logical points after RETINA-001.
		inWindow := info.LocationX >= 0 && info.LocationX <= float64(p.config.Width) &&
			y >= 0 && y <= float64(p.config.Height)

		if inWindow && !wasInWindow {
			p.mouseInWindow = true
			ev := p.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, info, y)
			p.dispatchPointerEventUnlocked(ev)
		} else if !inWindow && wasInWindow {
			p.mouseInWindow = false
			ev := p.createPointerEvent(gpucontext.PointerLeave, gpucontext.ButtonNone, info, y)
			p.dispatchPointerEventUnlocked(ev)
		}

		// Always send move event
		ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, info, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse drag events (move with button pressed)
	case darwin.NSEventTypeLeftMouseDragged,
		darwin.NSEventTypeRightMouseDragged,
		darwin.NSEventTypeOtherMouseDragged:
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, info, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse enter/exit events (for tracking areas)
	case darwin.NSEventTypeMouseEntered:
		p.mouseInWindow = true
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, info, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeMouseExited:
		p.mouseInWindow = false
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerLeave, gpucontext.ButtonNone, info, y)
		p.dispatchPointerEventUnlocked(ev)

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
			Modifiers: p.modifiers,
			Timestamp: p.eventTimestamp(),
		}
		p.dispatchScrollEventUnlocked(ev)

	// Keyboard events
	case darwin.NSEventTypeKeyDown:
		keyCode := darwin.GetKeyCode(event)
		key := macKeyCodeToKey(keyCode)
		p.dispatchKeyEventUnlocked(key, p.modifiers, true)

		// Dispatch character input from [NSEvent characters].
		// This handles all keyboard layouts, IME, and dead key sequences.
		p.dispatchCharFromEvent(event)

	case darwin.NSEventTypeKeyUp:
		keyCode := darwin.GetKeyCode(event)
		key := macKeyCodeToKey(keyCode)
		p.dispatchKeyEventUnlocked(key, p.modifiers, false)

	case darwin.NSEventTypeFlagsChanged:
		// Modifier key state changed
		// Detect which modifier key was pressed/released by comparing flags
		keyCode := darwin.GetKeyCode(event)
		key, pressed := detectModifierKeyChange(keyCode, info.ModifierFlags)
		if key != gpucontext.KeyUnknown {
			p.dispatchKeyEventUnlocked(key, p.modifiers, pressed)
		}
	}

	// Let all events be dispatched to the application
	return true
}

// dispatchPointerEventUnlocked dispatches without locking (called from handleEvent which is already in lock).
func (p *darwinPlatform) dispatchPointerEventUnlocked(ev gpucontext.PointerEvent) {
	callback := p.pointerCallback
	if callback != nil {
		// Release lock before calling user callback to avoid deadlocks
		p.mu.Unlock()
		callback(ev)
		p.mu.Lock()
	}
}

// dispatchScrollEventUnlocked dispatches without locking (called from handleEvent which is already in lock).
func (p *darwinPlatform) dispatchScrollEventUnlocked(ev gpucontext.ScrollEvent) {
	callback := p.scrollCallback
	if callback != nil {
		// Release lock before calling user callback to avoid deadlocks
		p.mu.Unlock()
		callback(ev)
		p.mu.Lock()
	}
}

// dispatchKeyEventUnlocked dispatches without locking (called from handleEvent which is already in lock).
func (p *darwinPlatform) dispatchKeyEventUnlocked(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	callback := p.keyboardCallback
	if callback != nil {
		// Release lock before calling user callback to avoid deadlocks
		p.mu.Unlock()
		callback(key, mods, pressed)
		p.mu.Lock()
	}
}

// dispatchCharFromEvent extracts characters from an NSEvent and dispatches them.
// Called from handleEvent under p.mu lock.
func (p *darwinPlatform) dispatchCharFromEvent(event darwin.ID) {
	callback := p.charCallback
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
	p.mu.Unlock()

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

	p.mu.Lock()
}

func (p *darwinPlatform) ShouldClose() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.ShouldClose() || p.shouldClose
	}
	return p.shouldClose
}

// LogicalSize returns the window size in Cocoa points (DIP).
func (p *darwinPlatform) LogicalSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.Size()
	}
	return p.config.Width, p.config.Height
}

// PhysicalSize returns the GPU framebuffer size in device pixels.
// On Retina displays this is LogicalSize * BackingScaleFactor.
func (p *darwinPlatform) PhysicalSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.FramebufferSize()
	}
	return p.config.Width, p.config.Height
}

func (p *darwinPlatform) GetHandle() (instance, window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// On macOS:
	// - instance: 0 (not used)
	// - window: CAMetalLayer pointer for surface creation
	if p.surface != nil {
		return 0, p.surface.LayerPtr()
	}

	// Fallback to content view if no surface
	if p.window != nil {
		return 0, p.window.ViewHandle()
	}

	return 0, 0
}

// InSizeMove returns true during live resize on macOS.
// macOS handles live resize smoothly via CAMetalLayer, so this
// returns false. The window remains responsive during resize.
func (p *darwinPlatform) InSizeMove() bool {
	// macOS doesn't have the same modal resize loop problem as Windows.
	// CAMetalLayer handles resize smoothly without blocking.
	return false
}

func (p *darwinPlatform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.surface != nil {
		p.surface.Destroy()
		p.surface = nil
	}

	if p.window != nil {
		p.window.Destroy()
		p.window = nil
	}

	if p.app != nil {
		p.app.Destroy()
		p.app = nil
	}
}

// queueEvent adds an event to the event queue.
func (p *darwinPlatform) queueEvent(event Event) {
	p.events = append(p.events, event)
}

// SetPointerCallback registers a callback for pointer events.
func (p *darwinPlatform) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	p.mu.Lock()
	p.pointerCallback = fn
	p.mu.Unlock()
}

// SetScrollCallback registers a callback for scroll events.
func (p *darwinPlatform) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	p.mu.Lock()
	p.scrollCallback = fn
	p.mu.Unlock()
}

// SetKeyCallback registers a callback for keyboard events.
func (p *darwinPlatform) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	p.mu.Lock()
	p.keyboardCallback = fn
	p.mu.Unlock()
}

// SetCharCallback registers a callback for Unicode character input.
func (p *darwinPlatform) SetCharCallback(fn func(rune)) {
	p.mu.Lock()
	p.charCallback = fn
	p.mu.Unlock()
}

// SetModalFrameCallback is a no-op on macOS.
// macOS doesn't have modal resize loops — CAMetalLayer handles live resize smoothly.
func (p *darwinPlatform) SetModalFrameCallback(_ func()) {}

// WaitEvents blocks until at least one OS event is available, then processes
// all pending events. Uses [NSApp nextEventMatchingMask:untilDate:inMode:dequeue:]
// with distantFuture, which blocks at kernel level via mach_msg for 0% CPU idle.
func (p *darwinPlatform) WaitEvents() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.app != nil {
		p.app.WaitEventsWithHandler(p.handleEvent)
	}

	// Check for resize after processing events
	p.checkResize()
}

// WakeUp unblocks WaitEvents from any goroutine by posting a synthetic
// NSEventTypeApplicationDefined event. This is thread-safe per Apple
// documentation and is the standard pattern used by GLFW, winit, SDL, and Qt.
func (p *darwinPlatform) WakeUp() {
	// No lock needed: PostEmptyEvent only reads a.initialized (set once at init)
	// and calls postEvent:atStart: which is documented as thread-safe.
	if p.app != nil {
		p.app.PostEmptyEvent()
	}
}

// checkResize checks for window size changes and queues a resize event.
// RETINA-002: Does NOT call p.surface.Resize(). Surface reconfiguration
// is handled by the render thread via RequestResize to avoid race conditions.
// Must be called with p.mu held.
func (p *darwinPlatform) checkResize() {
	if p.window == nil {
		return
	}

	oldWidth, oldHeight := p.config.Width, p.config.Height
	p.window.UpdateSize()
	newWidth, newHeight := p.window.Size() // logical points

	if newWidth != oldWidth || newHeight != oldHeight {
		p.config.Width = newWidth
		p.config.Height = newHeight

		physW, physH := p.window.FramebufferSize()

		p.queueEvent(Event{
			Type:           EventResize,
			Width:          newWidth,
			Height:         newHeight,
			PhysicalWidth:  physW,
			PhysicalHeight: physH,
		})
	}
}

// dispatchPointerEvent dispatches a pointer event to the registered callback.
func (p *darwinPlatform) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	// Callback is read under lock, but called without lock to avoid deadlocks.
	p.mu.Lock()
	callback := p.pointerCallback
	p.mu.Unlock()

	if callback != nil {
		callback(ev)
	}
}

// dispatchScrollEvent dispatches a scroll event to the registered callback.
func (p *darwinPlatform) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	p.mu.Lock()
	callback := p.scrollCallback
	p.mu.Unlock()

	if callback != nil {
		callback(ev)
	}
}

// eventTimestamp returns the event timestamp as duration since start.
func (p *darwinPlatform) eventTimestamp() time.Duration {
	return time.Since(p.startTime)
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
func (p *darwinPlatform) createPointerEvent(
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
	} else if eventType == gpucontext.PointerDown || p.buttons != gpucontext.ButtonsNone {
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
		Buttons:     p.buttons,
		Modifiers:   p.modifiers,
		Timestamp:   p.eventTimestamp(),
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

// ScaleFactor returns the DPI scale factor.
// On Retina displays returns 2.0, on standard displays 1.0.
func (p *darwinPlatform) ScaleFactor() float64 {
	if p.window == nil {
		return 1.0
	}
	return p.window.BackingScaleFactor()
}

// PrepareFrame updates macOS surface state before frame acquisition.
// Refreshes CAMetalLayer.contentsScale from the window's BackingScaleFactor
// every frame. In layer-hosting mode, macOS does not manage the layer and may
// reset contentsScale during layout passes. This matches Gio's approach of
// re-setting contentsScale in displayLayer: every frame.
func (p *darwinPlatform) PrepareFrame() PrepareFrameResult {
	if p.window == nil {
		return PrepareFrameResult{ScaleFactor: 1.0}
	}

	scale := p.window.BackingScaleFactor()
	physW, physH := p.window.FramebufferSize()

	// Detect scale change (skip first frame where lastScale is zero).
	scaleChanged := p.lastScale != 0 && p.lastScale != scale
	p.lastScale = scale

	// Re-set contentsScale every frame (defense-in-depth for Retina drift).
	if p.surface != nil && scale > 0 {
		p.surface.Layer().SetContentsScale(scale)
	}

	return PrepareFrameResult{
		ScaleChanged:   scaleChanged,
		ScaleFactor:    scale,
		PhysicalWidth:  uint32(physW),
		PhysicalHeight: uint32(physH),
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

// SetCursor changes the mouse cursor shape using NSCursor.
// cursorID maps to gpucontext.CursorShape values (0-11).
func (p *darwinPlatform) SetCursor(cursorID int) {
	cursorClass := darwin.GetClass("NSCursor")
	if cursorClass == 0 {
		return
	}

	var cursor darwin.ID
	switch cursorID {
	case 0: // CursorDefault — arrow
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 1: // CursorPointer — pointing hand
		cursor = cursorClass.Send(darwin.RegisterSelector("pointingHandCursor"))
	case 2: // CursorText — I-beam
		cursor = cursorClass.Send(darwin.RegisterSelector("IBeamCursor"))
	case 3: // CursorCrosshair
		cursor = cursorClass.Send(darwin.RegisterSelector("crosshairCursor"))
	case 4: // CursorMove — open hand (closest macOS equivalent)
		cursor = cursorClass.Send(darwin.RegisterSelector("openHandCursor"))
	case 5: // CursorResizeNS
		cursor = cursorClass.Send(darwin.RegisterSelector("resizeUpDownCursor"))
	case 6: // CursorResizeEW
		cursor = cursorClass.Send(darwin.RegisterSelector("resizeLeftRightCursor"))
	case 7: // CursorResizeNWSE — no direct macOS equivalent, use arrow
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 8: // CursorResizeNESW — no direct macOS equivalent, use arrow
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 9: // CursorNotAllowed
		cursor = cursorClass.Send(darwin.RegisterSelector("operationNotAllowedCursor"))
	case 10: // CursorWait — macOS has no wait cursor, use arrow
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	case 11: // CursorNone — hide cursor
		cursorClass.Send(darwin.RegisterSelector("hide"))
		return
	default:
		cursor = cursorClass.Send(darwin.RegisterSelector("arrowCursor"))
	}

	if !cursor.IsNil() {
		// Call [cursor set] to activate it
		cursor.Send(darwin.RegisterSelector("set"))
	}
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

func (p *darwinPlatform) SetFrameless(frameless bool) {
	p.mu.Lock()
	p.frameless = frameless
	p.mu.Unlock()

	if p.window != nil {
		if frameless {
			p.window.SetStyleMask(darwin.NSWindowStyleMaskBorderless | darwin.NSWindowStyleMaskResizable)
		} else {
			p.window.SetStyleMask(
				darwin.NSWindowStyleMaskTitled | darwin.NSWindowStyleMaskClosable |
					darwin.NSWindowStyleMaskMiniaturizable | darwin.NSWindowStyleMaskResizable)
		}
	}
}

func (p *darwinPlatform) IsFrameless() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frameless
}

func (p *darwinPlatform) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hitTestCallback = fn
}

func (p *darwinPlatform) Minimize() {
	if p.window != nil {
		p.window.Miniaturize()
	}
}

func (p *darwinPlatform) Maximize() {
	if p.window != nil {
		p.window.Zoom()
	}
}

func (p *darwinPlatform) IsMaximized() bool {
	if p.window != nil {
		return p.window.IsZoomed()
	}
	return false
}

func (p *darwinPlatform) SyncFrame() {}

// BlitPixels copies RGBA pixel data to the window using CoreGraphics.
// Implements the PixelBlitter interface for software backend presentation.
// Creates a CGImage from the pixel data and sets it as the NSView's layer contents.
func (p *darwinPlatform) BlitPixels(pixels []byte, width, height int) error {
	if p.window == nil {
		return fmt.Errorf("gogpu: darwin BlitPixels: no window")
	}

	// Create CGImage from RGBA pixel data
	cgImage, err := darwin.CreateCGImageFromRGBA(pixels, width, height)
	if err != nil {
		return fmt.Errorf("gogpu: darwin BlitPixels: %w", err)
	}
	defer darwin.ReleaseCGImage(cgImage)

	// Get the view's layer and set the image as its contents.
	// setContents: accepts a CGImageRef (toll-free bridged with id).
	contentView := p.window.ContentView()
	if contentView.IsNil() {
		return fmt.Errorf("gogpu: darwin BlitPixels: no content view")
	}

	// Ensure the view is layer-backed
	contentView.SendBool(darwin.RegisterSelector("setWantsLayer:"), true)

	// Get the layer
	layerID := contentView.Send(darwin.RegisterSelector("layer"))
	if layerID.IsNil() {
		return fmt.Errorf("gogpu: darwin BlitPixels: no layer")
	}

	// Set CGImage as layer contents (toll-free bridged with id)
	layerID.SendPtr(darwin.RegisterSelector("setContents:"), cgImage)

	return nil
}

func (p *darwinPlatform) CloseWindow() {
	if p.window != nil {
		p.window.Close()
	}
}

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
