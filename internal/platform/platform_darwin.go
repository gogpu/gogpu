//go:build darwin

package platform

import (
	"sync"
	"time"

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

	// Callbacks for pointer and scroll events
	pointerCallback func(gpucontext.PointerEvent)
	scrollCallback  func(gpucontext.ScrollEvent)

	// Timestamp reference for event timing
	startTime time.Time
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

	// Create window
	windowConfig := darwin.WindowConfig{
		Title:      config.Title,
		Width:      config.Width,
		Height:     config.Height,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
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

	// Process OS events with our handler
	if p.app != nil {
		p.app.PollEventsWithHandler(p.handleEvent)
	}

	// Check if window should close
	if p.window != nil && p.window.ShouldClose() {
		p.shouldClose = true
		return Event{Type: EventClose}
	}

	// Update window size and check for resize
	if p.window != nil {
		oldWidth, oldHeight := p.config.Width, p.config.Height
		p.window.UpdateSize()
		newWidth, newHeight := p.window.Size()

		if newWidth != oldWidth || newHeight != oldHeight {
			p.config.Width = newWidth
			p.config.Height = newHeight

			// Update surface size
			if p.surface != nil {
				p.surface.Resize(newWidth, newHeight)
			}

			return Event{
				Type:   EventResize,
				Width:  newWidth,
				Height: newHeight,
			}
		}
	}

	// Return queued event if any
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
//
//nolint:gocyclo // event type switch inherently has many cases
func (p *darwinPlatform) handleEvent(event darwin.ID, eventType darwin.NSEventType) bool {
	// Get event info
	info := darwin.GetEventInfo(event)

	// Get window height for Y coordinate flip
	// macOS uses bottom-left origin, we need top-left
	windowHeight := float64(p.config.Height)
	y := windowHeight - info.LocationY

	// Update modifiers
	p.modifiers = extractModifiers(info.ModifierFlags)

	switch eventType {
	// Mouse button down events
	case darwin.NSEventTypeLeftMouseDown:
		p.buttons |= gpucontext.ButtonsLeft
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonLeft, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeRightMouseDown:
		p.buttons |= gpucontext.ButtonsRight
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonRight, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeOtherMouseDown:
		btn := buttonFromNumber(info.ButtonNumber)
		p.buttons |= buttonsFromNumber(info.ButtonNumber)
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerDown, btn, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse button up events
	case darwin.NSEventTypeLeftMouseUp:
		p.buttons &^= gpucontext.ButtonsLeft
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonLeft, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeRightMouseUp:
		p.buttons &^= gpucontext.ButtonsRight
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonRight, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeOtherMouseUp:
		btn := buttonFromNumber(info.ButtonNumber)
		p.buttons &^= buttonsFromNumber(info.ButtonNumber)
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerUp, btn, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse move events
	case darwin.NSEventTypeMouseMoved:
		wasInWindow := p.mouseInWindow
		p.pointerX = info.LocationX
		p.pointerY = y

		// Detect enter/leave based on position
		inWindow := info.LocationX >= 0 && info.LocationX <= float64(p.config.Width) &&
			y >= 0 && y <= windowHeight

		if inWindow && !wasInWindow {
			p.mouseInWindow = true
			ev := p.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, info.LocationX, y)
			p.dispatchPointerEventUnlocked(ev)
		} else if !inWindow && wasInWindow {
			p.mouseInWindow = false
			ev := p.createPointerEvent(gpucontext.PointerLeave, gpucontext.ButtonNone, info.LocationX, y)
			p.dispatchPointerEventUnlocked(ev)
		}

		// Always send move event
		ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse drag events (move with button pressed)
	case darwin.NSEventTypeLeftMouseDragged,
		darwin.NSEventTypeRightMouseDragged,
		darwin.NSEventTypeOtherMouseDragged:
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	// Mouse enter/exit events (for tracking areas)
	case darwin.NSEventTypeMouseEntered:
		p.mouseInWindow = true
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	case darwin.NSEventTypeMouseExited:
		p.mouseInWindow = false
		p.pointerX = info.LocationX
		p.pointerY = y
		ev := p.createPointerEvent(gpucontext.PointerLeave, gpucontext.ButtonNone, info.LocationX, y)
		p.dispatchPointerEventUnlocked(ev)

	// Scroll wheel
	case darwin.NSEventTypeScrollWheel:
		// Determine delta mode based on precision
		deltaMode := gpucontext.ScrollDeltaLine
		if info.IsPrecise {
			deltaMode = gpucontext.ScrollDeltaPixel
		}

		ev := gpucontext.ScrollEvent{
			X:         info.LocationX,
			Y:         y,
			DeltaX:    info.ScrollDeltaX,
			DeltaY:    -info.ScrollDeltaY, // Invert Y: natural scrolling convention
			DeltaMode: deltaMode,
			Modifiers: p.modifiers,
			Timestamp: p.eventTimestamp(),
		}
		p.dispatchScrollEventUnlocked(ev)
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

func (p *darwinPlatform) ShouldClose() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.ShouldClose() || p.shouldClose
	}
	return p.shouldClose
}

func (p *darwinPlatform) GetSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.Size()
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
func (p *darwinPlatform) createPointerEvent(
	eventType gpucontext.PointerEventType,
	button gpucontext.Button,
	x, y float64,
) gpucontext.PointerEvent {
	// For button down/up, set pressure based on button state
	var pressure float32
	if eventType == gpucontext.PointerDown || p.buttons != gpucontext.ButtonsNone {
		pressure = 0.5 // Default pressure for mouse
	}

	return gpucontext.PointerEvent{
		Type:        eventType,
		PointerID:   1, // Mouse always has ID 1
		X:           x,
		Y:           y,
		Pressure:    pressure,
		TiltX:       0,
		TiltY:       0,
		Twist:       0,
		Width:       1,
		Height:      1,
		PointerType: gpucontext.PointerTypeMouse,
		IsPrimary:   true,
		Button:      button,
		Buttons:     p.buttons,
		Modifiers:   p.modifiers,
		Timestamp:   p.eventTimestamp(),
	}
}
