//go:build linux

package platform

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gogpu/gogpu/internal/platform/wayland"
	"github.com/gogpu/gogpu/internal/platform/x11"
	"github.com/gogpu/gpucontext"
)

// waylandPlatform implements the Platform interface using Wayland.
type waylandPlatform struct {
	mu sync.Mutex

	// Wakeup pipe for cross-goroutine WakeUp → WaitEvents unblocking.
	// [0]=read, [1]=write. Created with O_NONBLOCK|O_CLOEXEC.
	wakePipe [2]int

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
	touch    *wayland.WlTouch

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

	// Callbacks for pointer, scroll, and keyboard events
	pointerCallback  func(gpucontext.PointerEvent)
	scrollCallback   func(gpucontext.ScrollEvent)
	keyboardCallback func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)
	callbackMu       sync.RWMutex
}

// x11Platform wraps x11.Platform to implement the Platform interface.
type x11Platform struct {
	inner *x11.Platform

	// Wakeup pipe for cross-goroutine WakeUp → WaitEvents unblocking.
	// [0]=read, [1]=write. Created with O_NONBLOCK|O_CLOEXEC.
	wakePipe [2]int
}

// newPlatform creates the platform-specific implementation.
// On Linux, this returns a Wayland platform if available, otherwise X11.
func newPlatform() Platform {
	// Prefer Wayland if WAYLAND_DISPLAY is set
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		logger().Info("platform selected", "type", "wayland", "WAYLAND_DISPLAY", os.Getenv("WAYLAND_DISPLAY"))
		return &waylandPlatform{
			startTime: time.Now(),
		}
	}
	// Fall back to X11 if DISPLAY is set
	if os.Getenv("DISPLAY") != "" {
		logger().Info("platform selected", "type", "x11", "DISPLAY", os.Getenv("DISPLAY"))
		x11.SetLogger(loggerPtr.Load().WithGroup("x11"))
		return &x11Platform{inner: x11.NewPlatform()}
	}
	// Default to Wayland (will fail in Init if not available)
	logger().Info("platform selected", "type", "wayland", "reason", "default (no WAYLAND_DISPLAY or DISPLAY)")
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
	if err := p.inner.Init(x11Config); err != nil {
		return err
	}

	// Create wakeup pipe for WakeUp → WaitEvents unblocking
	if err := unix.Pipe2(p.wakePipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		p.inner.Destroy()
		return fmt.Errorf("x11: wakeup pipe: %w", err)
	}

	return nil
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
	if p.wakePipe[0] != 0 {
		_ = unix.Close(p.wakePipe[0])
		_ = unix.Close(p.wakePipe[1])
		p.wakePipe = [2]int{}
	}
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

// SetKeyCallback registers a callback for keyboard events.
// X11 keyboard events not yet implemented - only Wayland is supported.
func (p *x11Platform) SetKeyCallback(_ func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	// TODO: Implement X11 keyboard events
}

// SetModalFrameCallback is a no-op on X11.
// X11 doesn't have modal resize loops.
func (p *x11Platform) SetModalFrameCallback(_ func()) {}

// WaitEvents blocks until at least one OS event is available.
// Uses unix.Poll on the X11 socket fd and a wakeup pipe to block with 0% CPU.
func (p *x11Platform) WaitEvents() {
	connFd := p.inner.Fd()
	if connFd < 0 {
		return
	}

	fds := []unix.PollFd{
		{Fd: int32(connFd), Events: unix.POLLIN | unix.POLLERR},
		{Fd: int32(p.wakePipe[0]), Events: unix.POLLIN},
	}
	// Block indefinitely until an event arrives or WakeUp is called.
	// EINTR from signal delivery is harmless — returns as spurious wakeup.
	_, _ = unix.Poll(fds, -1)

	// Drain the wakeup pipe so it is ready for the next WakeUp call.
	drainPipe(p.wakePipe[0])
}

// WakeUp unblocks WaitEvents from any goroutine.
// Writing a single byte to the pipe wakes up unix.Poll immediately.
// Safe from any goroutine — pipe writes <= PIPE_BUF (4096 on Linux) are atomic.
func (p *x11Platform) WakeUp() {
	_, _ = unix.Write(p.wakePipe[1], []byte{0})
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

	// Create wakeup pipe for WakeUp → WaitEvents unblocking
	if err := unix.Pipe2(p.wakePipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: wakeup pipe: %w", err)
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
			p.setupKeyboardHandlers()
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

	// Get touch if available
	if p.seat.HasTouch() {
		touch, err := p.seat.GetTouch()
		if err == nil {
			p.touch = touch
			p.setupTouchHandlers()
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

// setupTouchHandlers configures Wayland touch event handlers.
func (p *waylandPlatform) setupTouchHandlers() {
	if p.touch == nil {
		return
	}

	p.touch.SetDownHandler(func(event *wayland.TouchDownEvent) {
		if p.surface == nil || event.Surface != p.surface.ID() {
			return
		}
		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerDown,
			PointerID:   int(event.ID) + 2, // Touch IDs start at 2 (mouse=1)
			X:           event.X,
			Y:           event.Y,
			Pressure:    0.5,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeTouch,
			IsPrimary:   event.ID == 0,
			Button:      gpucontext.ButtonLeft,
			Buttons:     gpucontext.ButtonsLeft,
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	p.touch.SetUpHandler(func(event *wayland.TouchUpEvent) {
		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerUp,
			PointerID:   int(event.ID) + 2,
			Pressure:    0,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeTouch,
			IsPrimary:   event.ID == 0,
			Button:      gpucontext.ButtonLeft,
			Buttons:     gpucontext.ButtonsNone,
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	p.touch.SetMotionHandler(func(event *wayland.TouchMotionEvent) {
		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerMove,
			PointerID:   int(event.ID) + 2,
			X:           event.X,
			Y:           event.Y,
			Pressure:    0.5,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeTouch,
			IsPrimary:   event.ID == 0,
			Button:      gpucontext.ButtonNone,
			Buttons:     gpucontext.ButtonsLeft,
			Modifiers:   p.getModifiers(),
			Timestamp:   p.eventTimestamp(),
		})
	})

	p.touch.SetCancelHandler(func() {
		// Touch cancel: dispatch PointerLeave to signal compositor took over
		p.dispatchPointerEvent(gpucontext.PointerEvent{
			Type:        gpucontext.PointerLeave,
			PointerID:   2,
			PointerType: gpucontext.PointerTypeTouch,
			IsPrimary:   true,
			Timestamp:   p.eventTimestamp(),
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

// dispatchKeyEvent dispatches a keyboard event to the registered callback.
func (p *waylandPlatform) dispatchKeyEvent(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	p.callbackMu.RLock()
	callback := p.keyboardCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(key, mods, pressed)
	}
}

// setupKeyboardHandlers configures Wayland keyboard event handlers.
func (p *waylandPlatform) setupKeyboardHandlers() {
	if p.keyboard == nil {
		return
	}

	// Handle key events
	p.keyboard.SetKeyHandler(func(event *wayland.KeyboardKeyEvent) {
		// Check if we have keyboard focus on our surface
		if p.keyboard.FocusedSurface() != p.surface.ID() {
			return
		}

		// Convert evdev keycode to gpucontext.Key
		// Note: Wayland uses evdev keycodes, which need +8 offset from X11 keycodes
		key := evdevToKey(event.Key)
		mods := p.getModifiers()
		pressed := event.State == wayland.KeyStatePressed

		p.dispatchKeyEvent(key, mods, pressed)
	})

	// Handle modifier events to update modifier state
	p.keyboard.SetModifiersHandler(func(event *wayland.KeyboardModifiersEvent) {
		p.pointerMu.Lock()
		p.modifiers = evdevModsToModifiers(event.ModsDepressed, event.ModsLocked)
		p.pointerMu.Unlock()
	})
}

// evdevModsToModifiers converts evdev modifier bitmasks to gpucontext.Modifiers.
func evdevModsToModifiers(depressed, locked uint32) gpucontext.Modifiers {
	var mods gpucontext.Modifiers

	// XKB modifier indices (standard layout)
	// These may vary by keymap, but these are common defaults
	const (
		xkbModShift   = 1 << 0
		xkbModLock    = 1 << 1 // Caps Lock
		xkbModControl = 1 << 2
		xkbModMod1    = 1 << 3 // Alt
		xkbModMod2    = 1 << 4 // Num Lock
		xkbModMod4    = 1 << 6 // Super
	)

	if depressed&xkbModShift != 0 {
		mods |= gpucontext.ModShift
	}
	if depressed&xkbModControl != 0 {
		mods |= gpucontext.ModControl
	}
	if depressed&xkbModMod1 != 0 {
		mods |= gpucontext.ModAlt
	}
	if depressed&xkbModMod4 != 0 {
		mods |= gpucontext.ModSuper
	}
	if locked&xkbModLock != 0 {
		mods |= gpucontext.ModCapsLock
	}
	if locked&xkbModMod2 != 0 {
		mods |= gpucontext.ModNumLock
	}

	return mods
}

// evdevToKey converts a Linux evdev keycode to gpucontext.Key.
//
//nolint:maintidx // key mapping requires many cases
func evdevToKey(keycode uint32) gpucontext.Key {
	// Linux evdev keycodes from linux/input-event-codes.h
	const (
		keyEsc        = 1
		key1          = 2
		key2          = 3
		key3          = 4
		key4          = 5
		key5          = 6
		key6          = 7
		key7          = 8
		key8          = 9
		key9          = 10
		key0          = 11
		keyMinus      = 12
		keyEqual      = 13
		keyBackspace  = 14
		keyTab        = 15
		keyQ          = 16
		keyW          = 17
		keyE          = 18
		keyR          = 19
		keyT          = 20
		keyY          = 21
		keyU          = 22
		keyI          = 23
		keyO          = 24
		keyP          = 25
		keyLeftBrace  = 26
		keyRightBrace = 27
		keyEnter      = 28
		keyLeftCtrl   = 29
		keyA          = 30
		keyS          = 31
		keyD          = 32
		keyF          = 33
		keyG          = 34
		keyH          = 35
		keyJ          = 36
		keyK          = 37
		keyL          = 38
		keySemicolon  = 39
		keyApostrophe = 40
		keyGrave      = 41
		keyLeftShift  = 42
		keyBackslash  = 43
		keyZ          = 44
		keyX          = 45
		keyC          = 46
		keyV          = 47
		keyB          = 48
		keyN          = 49
		keyM          = 50
		keyComma      = 51
		keyDot        = 52
		keySlash      = 53
		keyRightShift = 54
		keyKPAsterisk = 55
		keyLeftAlt    = 56
		keySpace      = 57
		keyCapsLock   = 58
		keyF1         = 59
		keyF2         = 60
		keyF3         = 61
		keyF4         = 62
		keyF5         = 63
		keyF6         = 64
		keyF7         = 65
		keyF8         = 66
		keyF9         = 67
		keyF10        = 68
		keyNumLock    = 69
		keyScrollLock = 70
		keyKP7        = 71
		keyKP8        = 72
		keyKP9        = 73
		keyKPMinus    = 74
		keyKP4        = 75
		keyKP5        = 76
		keyKP6        = 77
		keyKPPlus     = 78
		keyKP1        = 79
		keyKP2        = 80
		keyKP3        = 81
		keyKP0        = 82
		keyKPDot      = 83
		keyF11        = 87
		keyF12        = 88
		keyKPEnter    = 96
		keyRightCtrl  = 97
		keyKPSlash    = 98
		keyRightAlt   = 100
		keyHome       = 102
		keyUp         = 103
		keyPageUp     = 104
		keyLeft       = 105
		keyRight      = 106
		keyEnd        = 107
		keyDown       = 108
		keyPageDown   = 109
		keyInsert     = 110
		keyDelete     = 111
		keyPause      = 119
		keyLeftMeta   = 125
		keyRightMeta  = 126
	)

	// Letters
	switch keycode {
	case keyA:
		return gpucontext.KeyA
	case keyB:
		return gpucontext.KeyB
	case keyC:
		return gpucontext.KeyC
	case keyD:
		return gpucontext.KeyD
	case keyE:
		return gpucontext.KeyE
	case keyF:
		return gpucontext.KeyF
	case keyG:
		return gpucontext.KeyG
	case keyH:
		return gpucontext.KeyH
	case keyI:
		return gpucontext.KeyI
	case keyJ:
		return gpucontext.KeyJ
	case keyK:
		return gpucontext.KeyK
	case keyL:
		return gpucontext.KeyL
	case keyM:
		return gpucontext.KeyM
	case keyN:
		return gpucontext.KeyN
	case keyO:
		return gpucontext.KeyO
	case keyP:
		return gpucontext.KeyP
	case keyQ:
		return gpucontext.KeyQ
	case keyR:
		return gpucontext.KeyR
	case keyS:
		return gpucontext.KeyS
	case keyT:
		return gpucontext.KeyT
	case keyU:
		return gpucontext.KeyU
	case keyV:
		return gpucontext.KeyV
	case keyW:
		return gpucontext.KeyW
	case keyX:
		return gpucontext.KeyX
	case keyY:
		return gpucontext.KeyY
	case keyZ:
		return gpucontext.KeyZ

	// Numbers
	case key0:
		return gpucontext.Key0
	case key1:
		return gpucontext.Key1
	case key2:
		return gpucontext.Key2
	case key3:
		return gpucontext.Key3
	case key4:
		return gpucontext.Key4
	case key5:
		return gpucontext.Key5
	case key6:
		return gpucontext.Key6
	case key7:
		return gpucontext.Key7
	case key8:
		return gpucontext.Key8
	case key9:
		return gpucontext.Key9

	// Function keys
	case keyF1:
		return gpucontext.KeyF1
	case keyF2:
		return gpucontext.KeyF2
	case keyF3:
		return gpucontext.KeyF3
	case keyF4:
		return gpucontext.KeyF4
	case keyF5:
		return gpucontext.KeyF5
	case keyF6:
		return gpucontext.KeyF6
	case keyF7:
		return gpucontext.KeyF7
	case keyF8:
		return gpucontext.KeyF8
	case keyF9:
		return gpucontext.KeyF9
	case keyF10:
		return gpucontext.KeyF10
	case keyF11:
		return gpucontext.KeyF11
	case keyF12:
		return gpucontext.KeyF12

	// Navigation
	case keyEsc:
		return gpucontext.KeyEscape
	case keyTab:
		return gpucontext.KeyTab
	case keyBackspace:
		return gpucontext.KeyBackspace
	case keyEnter, keyKPEnter:
		return gpucontext.KeyEnter
	case keySpace:
		return gpucontext.KeySpace
	case keyInsert:
		return gpucontext.KeyInsert
	case keyDelete:
		return gpucontext.KeyDelete
	case keyHome:
		return gpucontext.KeyHome
	case keyEnd:
		return gpucontext.KeyEnd
	case keyPageUp:
		return gpucontext.KeyPageUp
	case keyPageDown:
		return gpucontext.KeyPageDown
	case keyLeft:
		return gpucontext.KeyLeft
	case keyRight:
		return gpucontext.KeyRight
	case keyUp:
		return gpucontext.KeyUp
	case keyDown:
		return gpucontext.KeyDown

	// Modifiers
	case keyLeftShift:
		return gpucontext.KeyLeftShift
	case keyRightShift:
		return gpucontext.KeyRightShift
	case keyLeftCtrl:
		return gpucontext.KeyLeftControl
	case keyRightCtrl:
		return gpucontext.KeyRightControl
	case keyLeftAlt:
		return gpucontext.KeyLeftAlt
	case keyRightAlt:
		return gpucontext.KeyRightAlt
	case keyLeftMeta:
		return gpucontext.KeyLeftSuper
	case keyRightMeta:
		return gpucontext.KeyRightSuper

	// Punctuation
	case keyMinus:
		return gpucontext.KeyMinus
	case keyEqual:
		return gpucontext.KeyEqual
	case keyLeftBrace:
		return gpucontext.KeyLeftBracket
	case keyRightBrace:
		return gpucontext.KeyRightBracket
	case keyBackslash:
		return gpucontext.KeyBackslash
	case keySemicolon:
		return gpucontext.KeySemicolon
	case keyApostrophe:
		return gpucontext.KeyApostrophe
	case keyGrave:
		return gpucontext.KeyGrave
	case keyComma:
		return gpucontext.KeyComma
	case keyDot:
		return gpucontext.KeyPeriod
	case keySlash:
		return gpucontext.KeySlash

	// Numpad
	case keyKP0:
		return gpucontext.KeyNumpad0
	case keyKP1:
		return gpucontext.KeyNumpad1
	case keyKP2:
		return gpucontext.KeyNumpad2
	case keyKP3:
		return gpucontext.KeyNumpad3
	case keyKP4:
		return gpucontext.KeyNumpad4
	case keyKP5:
		return gpucontext.KeyNumpad5
	case keyKP6:
		return gpucontext.KeyNumpad6
	case keyKP7:
		return gpucontext.KeyNumpad7
	case keyKP8:
		return gpucontext.KeyNumpad8
	case keyKP9:
		return gpucontext.KeyNumpad9
	case keyKPDot:
		return gpucontext.KeyNumpadDecimal
	case keyKPSlash:
		return gpucontext.KeyNumpadDivide
	case keyKPAsterisk:
		return gpucontext.KeyNumpadMultiply
	case keyKPMinus:
		return gpucontext.KeyNumpadSubtract
	case keyKPPlus:
		return gpucontext.KeyNumpadAdd

	// Lock keys
	case keyCapsLock:
		return gpucontext.KeyCapsLock
	case keyScrollLock:
		return gpucontext.KeyScrollLock
	case keyNumLock:
		return gpucontext.KeyNumLock
	case keyPause:
		return gpucontext.KeyPause
	}

	return gpucontext.KeyUnknown
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

	// Close wakeup pipe
	if p.wakePipe[0] != 0 {
		_ = unix.Close(p.wakePipe[0])
		_ = unix.Close(p.wakePipe[1])
		p.wakePipe = [2]int{}
	}

	// Destroy in reverse order of creation

	if p.touch != nil {
		_ = p.touch.Release()
		p.touch = nil
	}

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

// SetKeyCallback registers a callback for keyboard events.
func (p *waylandPlatform) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	p.callbackMu.Lock()
	p.keyboardCallback = fn
	p.callbackMu.Unlock()
}

// SetModalFrameCallback is a no-op on Wayland.
// Wayland uses async configure events — resize is never blocking.
func (p *waylandPlatform) SetModalFrameCallback(_ func()) {}

// WaitEvents blocks until at least one OS event is available.
// Uses unix.Poll on the Wayland display fd and a wakeup pipe to block with 0% CPU.
func (p *waylandPlatform) WaitEvents() {
	dispFd := p.display.Fd()
	if dispFd < 0 {
		return
	}

	fds := []unix.PollFd{
		{Fd: int32(dispFd), Events: unix.POLLIN | unix.POLLERR},
		{Fd: int32(p.wakePipe[0]), Events: unix.POLLIN},
	}
	// Block indefinitely until an event arrives or WakeUp is called.
	// EINTR from signal delivery is harmless — returns as spurious wakeup.
	_, _ = unix.Poll(fds, -1)

	// Drain the wakeup pipe so it is ready for the next WakeUp call.
	drainPipe(p.wakePipe[0])
}

// WakeUp unblocks WaitEvents from any goroutine.
// Writing a single byte to the pipe wakes up unix.Poll immediately.
// Safe from any goroutine — pipe writes <= PIPE_BUF (4096 on Linux) are atomic.
func (p *waylandPlatform) WakeUp() {
	_, _ = unix.Write(p.wakePipe[1], []byte{0})
}

// drainPipe reads all pending bytes from a non-blocking pipe fd.
// This ensures the pipe is empty for the next WakeUp call.
func drainPipe(fd int) {
	var buf [64]byte
	for {
		_, err := unix.Read(fd, buf[:])
		if err != nil {
			break
		}
	}
}

// waylandPlatform provider stubs

// ScaleFactor returns the DPI scale factor.
// TODO: Implement using wl_output scale and fractional_scale_v1.
func (p *waylandPlatform) ScaleFactor() float64 { return 1.0 }

// ClipboardRead reads text from the system clipboard.
// TODO: Implement using wl_data_device and wl_data_offer.
func (p *waylandPlatform) ClipboardRead() (string, error) { return "", nil }

// ClipboardWrite writes text to the system clipboard.
// TODO: Implement using wl_data_device and wl_data_source.
func (p *waylandPlatform) ClipboardWrite(string) error { return nil }

// SetCursor changes the mouse cursor shape.
// TODO: Implement using wp_cursor_shape_manager_v1 or cursor theme.
func (p *waylandPlatform) SetCursor(int) {}

// DarkMode returns true if the system dark mode is active.
// TODO: Implement using org.freedesktop.portal.Settings.
func (p *waylandPlatform) DarkMode() bool { return false }

// ReduceMotion returns true if the user prefers reduced animation.
// TODO: Implement using org.freedesktop.portal.Settings.
func (p *waylandPlatform) ReduceMotion() bool { return false }

// HighContrast returns true if high contrast mode is active.
// TODO: Implement using org.freedesktop.portal.Settings.
func (p *waylandPlatform) HighContrast() bool { return false }

// FontScale returns font size preference multiplier.
// TODO: Implement using GSettings text-scaling-factor.
func (p *waylandPlatform) FontScale() float32 { return 1.0 }

// x11Platform provider stubs

// ScaleFactor returns the DPI scale factor.
// TODO: Implement using Xft.dpi or XRandR.
func (p *x11Platform) ScaleFactor() float64 { return 1.0 }

// ClipboardRead reads text from the system clipboard.
// TODO: Implement using X11 selections (XA_CLIPBOARD).
func (p *x11Platform) ClipboardRead() (string, error) { return "", nil }

// ClipboardWrite writes text to the system clipboard.
// TODO: Implement using X11 selections (XA_CLIPBOARD).
func (p *x11Platform) ClipboardWrite(string) error { return nil }

// SetCursor changes the mouse cursor shape.
// TODO: Implement using XCreateFontCursor or Xcursor.
func (p *x11Platform) SetCursor(int) {}

// DarkMode returns true if the system dark mode is active.
// TODO: Implement using org.freedesktop.portal.Settings.
func (p *x11Platform) DarkMode() bool { return false }

// ReduceMotion returns true if the user prefers reduced animation.
// TODO: Implement using org.freedesktop.portal.Settings.
func (p *x11Platform) ReduceMotion() bool { return false }

// HighContrast returns true if high contrast mode is active.
// TODO: Implement using org.freedesktop.portal.Settings.
func (p *x11Platform) HighContrast() bool { return false }

// FontScale returns font size preference multiplier.
// TODO: Implement using Xft.dpi or GSettings text-scaling-factor.
func (p *x11Platform) FontScale() float32 { return 1.0 }
