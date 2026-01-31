//go:build linux

package platform

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gogpu/gogpu/internal/platform/wayland"
	"github.com/gogpu/gogpu/internal/platform/x11"
	"github.com/gogpu/gpucontext"
)

// waylandPlatform implements the Platform interface using Wayland.
type waylandPlatform struct {
	mu sync.Mutex

	// Wayland core objects
	display    *wayland.Display
	registry   *wayland.Registry
	compositor *wayland.WlCompositor
	surface    *wayland.WlSurface
	xdgWmBase  *wayland.XdgWmBase
	xdgSurface *wayland.XdgSurface
	toplevel   *wayland.XdgToplevel

	// Input devices
	seat     *wayland.WlSeat
	keyboard *wayland.WlKeyboard
	pointer  *wayland.WlPointer

	// Window state
	width       int
	height      int
	shouldClose bool
	configured  bool

	// Pending resize from configure event
	pendingWidth  int
	pendingHeight int
	hasResize     bool

	// Pointer state tracking
	pointerX  float64
	pointerY  float64
	buttons   gpucontext.Buttons
	modifiers gpucontext.Modifiers
	pointerMu sync.RWMutex
	pointerIn bool // True when pointer is inside our surface
	startTime time.Time

	// Callbacks for pointer and scroll events
	pointerCallback func(gpucontext.PointerEvent)
	scrollCallback  func(gpucontext.ScrollEvent)
	callbackMu      sync.RWMutex
}

// x11Platform wraps x11.Platform to implement the Platform interface.
type x11Platform struct {
	inner *x11.Platform
}

// newPlatform creates the platform-specific implementation.
// On Linux, this returns a Wayland platform if available, otherwise X11.
func newPlatform() Platform {
	// Prefer Wayland if WAYLAND_DISPLAY is set
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return &waylandPlatform{
			startTime: time.Now(),
		}
	}
	// Fall back to X11 if DISPLAY is set
	if os.Getenv("DISPLAY") != "" {
		return &x11Platform{inner: x11.NewPlatform()}
	}
	// Default to Wayland (will fail in Init if not available)
	return &waylandPlatform{
		startTime: time.Now(),
	}
}

// Init creates the X11 window.
func (p *x11Platform) Init(config Config) error {
	x11Config := x11.Config{
		Title:      config.Title,
		Width:      config.Width,
		Height:     config.Height,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
	}
	return p.inner.Init(x11Config)
}

// PollEvents processes pending X11 events.
func (p *x11Platform) PollEvents() Event {
	event := p.inner.PollEvents()
	switch event.Type {
	case x11.EventTypeClose:
		return Event{Type: EventClose}
	case x11.EventTypeResize:
		return Event{Type: EventResize, Width: event.Width, Height: event.Height}
	default:
		return Event{Type: EventNone}
	}
}

// ShouldClose returns true if window close was requested.
func (p *x11Platform) ShouldClose() bool {
	return p.inner.ShouldClose()
}

// GetSize returns current window size in pixels.
func (p *x11Platform) GetSize() (width, height int) {
	return p.inner.GetSize()
}

// GetHandle returns platform-specific handles for Vulkan surface creation.
func (p *x11Platform) GetHandle() (instance, window uintptr) {
	return p.inner.GetHandle()
}

// Destroy closes the window and releases resources.
func (p *x11Platform) Destroy() {
	p.inner.Destroy()
}

// InSizeMove returns true during live resize on X11.
// X11 doesn't have modal resize loops like Windows.
func (p *x11Platform) InSizeMove() bool {
	return false
}

// SetPointerCallback registers a callback for pointer events.
func (p *x11Platform) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	p.inner.SetPointerCallback(fn)
}

// SetScrollCallback registers a callback for scroll events.
func (p *x11Platform) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	p.inner.SetScrollCallback(fn)
}

// Init creates the Wayland window.
func (p *waylandPlatform) Init(config Config) error {
	// Check if Wayland is available
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("wayland: WAYLAND_DISPLAY not set (X11 not yet supported)")
	}

	// Connect to Wayland display
	display, err := wayland.Connect()
	if err != nil {
		return fmt.Errorf("wayland: failed to connect: %w", err)
	}
	p.display = display

	// Get registry
	registry, err := display.GetRegistry()
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to get registry: %w", err)
	}
	p.registry = registry

	// Wait for globals to be advertised
	required := []string{
		wayland.InterfaceWlCompositor,
		wayland.InterfaceXdgWmBase,
	}
	if err := registry.WaitForGlobals(required, 5); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: %w", err)
	}

	// Bind to wl_compositor
	compositorID, err := registry.BindCompositor(4)
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to bind compositor: %w", err)
	}
	p.compositor = wayland.NewWlCompositor(display, compositorID)

	// Bind to xdg_wm_base
	xdgWmBaseID, err := registry.BindXdgWmBase(2)
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to bind xdg_wm_base: %w", err)
	}
	p.xdgWmBase = wayland.NewXdgWmBase(display, xdgWmBaseID)

	// Create wl_surface
	surface, err := p.compositor.CreateSurface()
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to create surface: %w", err)
	}
	p.surface = surface

	// Create xdg_surface
	xdgSurface, err := p.xdgWmBase.GetXdgSurface(surface)
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to create xdg_surface: %w", err)
	}
	p.xdgSurface = xdgSurface

	// Create xdg_toplevel
	toplevel, err := xdgSurface.GetToplevel()
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to create toplevel: %w", err)
	}
	p.toplevel = toplevel

	// Set window properties
	if err := toplevel.SetTitle(config.Title); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to set title: %w", err)
	}
	if err := toplevel.SetAppID("gogpu"); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to set app_id: %w", err)
	}

	// Set initial size
	p.width = config.Width
	p.height = config.Height

	// Set size constraints if not resizable
	if !config.Resizable {
		if err := toplevel.SetMinSize(int32(config.Width), int32(config.Height)); err != nil {
			_ = display.Close()
			return fmt.Errorf("wayland: failed to set min size: %w", err)
		}
		if err := toplevel.SetMaxSize(int32(config.Width), int32(config.Height)); err != nil {
			_ = display.Close()
			return fmt.Errorf("wayland: failed to set max size: %w", err)
		}
	}

	// Set up event handlers
	p.setupEventHandlers()

	// Commit to signal we're ready for configure
	if err := surface.Commit(); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to commit surface: %w", err)
	}

	// Wait for initial configure event
	if err := p.waitForConfigure(); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to wait for configure: %w", err)
	}

	// Optionally bind to seat for input devices
	if registry.HasGlobal(wayland.InterfaceWlSeat) {
		_ = p.bindSeat() // Non-fatal: we can run without input devices
	}

	// Set fullscreen if requested
	if config.Fullscreen {
		_ = toplevel.SetFullscreen(0) // Non-fatal, continue
	}

	return nil
}

// setupEventHandlers sets up Wayland event handlers.
func (p *waylandPlatform) setupEventHandlers() {
	// Handle xdg_surface configure
	p.xdgSurface.SetConfigureHandler(func(serial uint32) {
		p.mu.Lock()
		defer p.mu.Unlock()

		// ACK the configure event
		if err := p.xdgSurface.AckConfigure(serial); err != nil {
			// Log error but continue
			return
		}

		// Commit the surface
		if err := p.surface.Commit(); err != nil {
			// Log error but continue
			return
		}

		p.configured = true
	})

	// Handle toplevel configure (resize)
	p.toplevel.SetConfigureHandler(func(config *wayland.XdgToplevelConfig) {
		p.mu.Lock()
		defer p.mu.Unlock()

		// Width/height of 0 means client can choose
		if config.Width > 0 && config.Height > 0 {
			newWidth := int(config.Width)
			newHeight := int(config.Height)

			if newWidth != p.width || newHeight != p.height {
				p.pendingWidth = newWidth
				p.pendingHeight = newHeight
				p.hasResize = true
			}
		}
	})

	// Handle toplevel close
	p.toplevel.SetCloseHandler(func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.shouldClose = true
	})
}

// waitForConfigure waits for the initial configure event.
func (p *waylandPlatform) waitForConfigure() error {
	// Perform roundtrips until we receive a configure event
	for i := 0; i < 10; i++ {
		if err := p.display.Roundtrip(); err != nil {
			return fmt.Errorf("roundtrip failed: %w", err)
		}

		p.mu.Lock()
		configured := p.configured
		p.mu.Unlock()

		if configured {
			return nil
		}
	}

	return fmt.Errorf("timeout waiting for configure")
}

// bindSeat binds to the wl_seat for input devices.
func (p *waylandPlatform) bindSeat() error {
	seatVersion := p.registry.GlobalVersion(wayland.InterfaceWlSeat)
	if seatVersion == 0 {
		return fmt.Errorf("wl_seat not available")
	}

	// Limit to version we support
	if seatVersion > 7 {
		seatVersion = 7
	}

	seatID, err := p.registry.BindSeat(seatVersion)
	if err != nil {
		return fmt.Errorf("failed to bind seat: %w", err)
	}
	p.seat = wayland.NewWlSeat(p.display, seatID, seatVersion)

	// Wait for capabilities
	if err := p.display.Roundtrip(); err != nil {
		return fmt.Errorf("roundtrip failed: %w", err)
	}

	// Get keyboard if available
	if p.seat.HasKeyboard() {
		keyboard, err := p.seat.GetKeyboard()
		if err == nil {
			p.keyboard = keyboard
		}
	}

	// Get pointer if available
	if p.seat.HasPointer() {
		pointer, err := p.seat.GetPointer()
		if err == nil {
			p.pointer = pointer
			p.setupPointerHandlers()
		}
	}

	return nil
}

// setupPointerHandlers configures Wayland pointer event handlers.
func (p *waylandPlatform) setupPointerHandlers() {
	if p.pointer == nil {
		return
	}

	// Handle pointer enter (mouse enters our surface)
	p.pointer.SetEnterHandler(func(event *wayland.PointerEnterEvent) {
		// Check if this is our surface
		if p.surface == nil || event.Surface != p.surface.ID() {
			return
		}

		p.pointerMu.Lock()
		p.pointerX = event.SurfaceX
		p.pointerY = event.SurfaceY
		p.pointerIn = true
		p.pointerMu.Unlock()

		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerEnter,
			PointerID:   1, // Mouse always has ID 1
			X:           event.SurfaceX,
			Y:           event.SurfaceY,
			Pressure:    0,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeMouse,
			IsPrimary:   true,
			Button:      gpucontext.ButtonNone,
			Buttons:     p.getButtons(),
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	// Handle pointer leave (mouse leaves our surface)
	p.pointer.SetLeaveHandler(func(event *wayland.PointerLeaveEvent) {
		// Check if this is our surface
		if p.surface == nil || event.Surface != p.surface.ID() {
			return
		}

		p.pointerMu.Lock()
		x := p.pointerX
		y := p.pointerY
		p.pointerIn = false
		p.pointerMu.Unlock()

		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerLeave,
			PointerID:   1,
			X:           x,
			Y:           y,
			Pressure:    0,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeMouse,
			IsPrimary:   true,
			Button:      gpucontext.ButtonNone,
			Buttons:     p.getButtons(),
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	// Handle pointer motion
	p.pointer.SetMotionHandler(func(event *wayland.PointerMotionEvent) {
		p.pointerMu.Lock()
		if !p.pointerIn {
			p.pointerMu.Unlock()
			return
		}
		p.pointerX = event.SurfaceX
		p.pointerY = event.SurfaceY
		buttons := p.buttons
		p.pointerMu.Unlock()

		// Pressure is 0.5 if any button is pressed, 0 otherwise
		var pressure float32
		if buttons != gpucontext.ButtonsNone {
			pressure = 0.5
		}

		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerID:   1,
			X:           event.SurfaceX,
			Y:           event.SurfaceY,
			Pressure:    pressure,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeMouse,
			IsPrimary:   true,
			Button:      gpucontext.ButtonNone,
			Buttons:     buttons,
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	// Handle pointer button events
	p.pointer.SetButtonHandler(func(event *wayland.PointerButtonEvent) {
		p.pointerMu.Lock()
		if !p.pointerIn {
			p.pointerMu.Unlock()
			return
		}

		// Map Linux evdev button code to gpucontext button
		button := mapWaylandButton(event.Button)
		buttonMask := buttonToMask(button)

		// Update button state
		if event.State == wayland.PointerButtonStatePressed {
			p.buttons |= buttonMask
		} else {
			p.buttons &^= buttonMask
		}

		buttons := p.buttons
		x := p.pointerX
		y := p.pointerY
		p.pointerMu.Unlock()

		// Determine event type
		var eventType gpucontext.PointerEventType
		if event.State == wayland.PointerButtonStatePressed {
			eventType = gpucontext.PointerDown
		} else {
			eventType = gpucontext.PointerUp
		}

		// Pressure is 0.5 for button down, based on button state for up
		var pressure float32
		if eventType == gpucontext.PointerDown || buttons != gpucontext.ButtonsNone {
			pressure = 0.5
		}

		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        eventType,
			PointerID:   1,
			X:           x,
			Y:           y,
			Pressure:    pressure,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeMouse,
			IsPrimary:   true,
			Button:      button,
			Buttons:     buttons,
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	// Handle scroll (axis) events
	p.pointer.SetAxisHandler(func(event *wayland.PointerAxisEvent) {
		p.pointerMu.Lock()
		if !p.pointerIn {
			p.pointerMu.Unlock()
			return
		}
		x := p.pointerX
		y := p.pointerY
		p.pointerMu.Unlock()

		var deltaX, deltaY float64

		// Map Wayland axis to scroll delta
		// Axis 0 = vertical scroll, Axis 1 = horizontal scroll
		// Wayland: positive = down/right
		// gpucontext ScrollEvent: positive = down/right (same convention)
		switch event.Axis {
		case wayland.PointerAxisVerticalScroll:
			deltaY = event.Value
		case wayland.PointerAxisHorizontalScroll:
			deltaX = event.Value
		}

		p.dispatchScrollEvent(gpucontext.ScrollEvent{
			X:         x,
			Y:         y,
			DeltaX:    deltaX,
			DeltaY:    deltaY,
			DeltaMode: gpucontext.ScrollDeltaPixel, // Wayland provides pixel values
			Modifiers: p.getModifiers(),
			Timestamp: p.eventTimestamp(),
		})
	})
}

// mapWaylandButton maps a Linux evdev button code to gpucontext.Button.
func mapWaylandButton(button uint32) gpucontext.Button {
	switch button {
	case wayland.ButtonLeft: // 0x110 (BTN_LEFT)
		return gpucontext.ButtonLeft
	case wayland.ButtonRight: // 0x111 (BTN_RIGHT)
		return gpucontext.ButtonRight
	case wayland.ButtonMiddle: // 0x112 (BTN_MIDDLE)
		return gpucontext.ButtonMiddle
	case wayland.ButtonSide: // 0x113 (BTN_SIDE) - maps to X1 (back)
		return gpucontext.ButtonX1
	case wayland.ButtonExtra: // 0x114 (BTN_EXTRA) - maps to X2 (forward)
		return gpucontext.ButtonX2
	default:
		return gpucontext.ButtonNone
	}
}

// buttonToMask converts a Button to its Buttons bitmask.
func buttonToMask(button gpucontext.Button) gpucontext.Buttons {
	switch button {
	case gpucontext.ButtonLeft:
		return gpucontext.ButtonsLeft
	case gpucontext.ButtonRight:
		return gpucontext.ButtonsRight
	case gpucontext.ButtonMiddle:
		return gpucontext.ButtonsMiddle
	case gpucontext.ButtonX1:
		return gpucontext.ButtonsX1
	case gpucontext.ButtonX2:
		return gpucontext.ButtonsX2
	default:
		return gpucontext.ButtonsNone
	}
}

// getButtons returns the current button state (thread-safe).
func (p *waylandPlatform) getButtons() gpucontext.Buttons {
	p.pointerMu.RLock()
	defer p.pointerMu.RUnlock()
	return p.buttons
}

// getModifiers returns the current modifier state (thread-safe).
func (p *waylandPlatform) getModifiers() gpucontext.Modifiers {
	p.pointerMu.RLock()
	defer p.pointerMu.RUnlock()
	return p.modifiers
}

// eventTimestamp returns the event timestamp as duration since start.
func (p *waylandPlatform) eventTimestamp() time.Duration {
	return time.Since(p.startTime)
}

// dispatchPointerEvent dispatches a pointer event to the registered callback.
func (p *waylandPlatform) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	p.callbackMu.RLock()
	callback := p.pointerCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// dispatchScrollEvent dispatches a scroll event to the registered callback.
func (p *waylandPlatform) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	p.callbackMu.RLock()
	callback := p.scrollCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// PollEvents processes pending Wayland events.
func (p *waylandPlatform) PollEvents() Event {
	p.mu.Lock()

	// Check for pending resize
	if p.hasResize {
		p.width = p.pendingWidth
		p.height = p.pendingHeight
		p.hasResize = false
		p.mu.Unlock()

		return Event{
			Type:   EventResize,
			Width:  p.pendingWidth,
			Height: p.pendingHeight,
		}
	}

	// Check for close
	if p.shouldClose {
		p.mu.Unlock()
		return Event{Type: EventClose}
	}

	p.mu.Unlock()

	// Dispatch pending Wayland events (non-blocking)
	if err := p.display.Dispatch(); err != nil {
		// Connection error - treat as close
		p.mu.Lock()
		p.shouldClose = true
		p.mu.Unlock()
		return Event{Type: EventClose}
	}

	// Check again after dispatch
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.hasResize {
		p.width = p.pendingWidth
		p.height = p.pendingHeight
		p.hasResize = false
		return Event{
			Type:   EventResize,
			Width:  p.pendingWidth,
			Height: p.pendingHeight,
		}
	}

	if p.shouldClose {
		return Event{Type: EventClose}
	}

	return Event{Type: EventNone}
}

// ShouldClose returns true if window close was requested.
func (p *waylandPlatform) ShouldClose() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shouldClose
}

// GetSize returns current window size in pixels.
func (p *waylandPlatform) GetSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width, p.height
}

// GetHandle returns platform-specific handles for Vulkan surface creation.
// On Linux/Wayland, returns (wl_display fd, wl_surface id).
// Note: For VK_KHR_wayland_surface, you need the actual C pointers.
// This pure Go implementation provides the underlying IDs/FDs.
func (p *waylandPlatform) GetHandle() (instance, window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.display == nil || p.surface == nil {
		return 0, 0
	}

	return p.display.Ptr(), p.surface.Ptr()
}

// Destroy closes the window and releases resources.
func (p *waylandPlatform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Destroy in reverse order of creation

	if p.pointer != nil {
		_ = p.pointer.Release()
		p.pointer = nil
	}

	if p.keyboard != nil {
		_ = p.keyboard.Release()
		p.keyboard = nil
	}

	if p.seat != nil {
		// Don't call Release() unless we have version 5+
		p.seat = nil
	}

	if p.toplevel != nil {
		_ = p.toplevel.Destroy()
		p.toplevel = nil
	}

	if p.xdgSurface != nil {
		_ = p.xdgSurface.Destroy()
		p.xdgSurface = nil
	}

	if p.surface != nil {
		_ = p.surface.Destroy()
		p.surface = nil
	}

	if p.xdgWmBase != nil {
		_ = p.xdgWmBase.Destroy()
		p.xdgWmBase = nil
	}

	// Note: compositor doesn't have a destroy method

	if p.display != nil {
		_ = p.display.Close()
		p.display = nil
	}
}

// InSizeMove returns true during live resize on Wayland.
// Wayland uses async configure events, so resize is never blocking.
func (p *waylandPlatform) InSizeMove() bool {
	return false
}

// SetPointerCallback registers a callback for pointer events.
func (p *waylandPlatform) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	p.callbackMu.Lock()
	p.pointerCallback = fn
	p.callbackMu.Unlock()
}

// SetScrollCallback registers a callback for scroll events.
func (p *waylandPlatform) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	p.callbackMu.Lock()
	p.scrollCallback = fn
	p.callbackMu.Unlock()
}
