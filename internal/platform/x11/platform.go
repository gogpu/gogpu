//go:build linux

package x11

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"github.com/gogpu/gpucontext"
)

// Config holds configuration for creating a platform window.
// This mirrors platform.Config to avoid import cycles.
type Config struct {
	Title      string
	Width      int
	Height     int
	Resizable  bool
	Fullscreen bool
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventTypeNone EventType = iota
	EventTypeClose
	EventTypeResize
)

// PlatformEvent represents a platform event.
// This mirrors platform.Event to avoid import cycles.
type PlatformEvent struct {
	Type   EventType
	Width  int
	Height int
}

// xlibHandle holds the Xlib Display* pointer required for Vulkan surface creation.
// VK_KHR_xlib_surface expects a real Display* from XOpenDisplay(), not a socket FD.
// We load libX11 dynamically via goffi (no CGO) and open a parallel Xlib connection.
// The window ID is shared between our pure Go X11 wire protocol and Xlib because
// X11 window IDs are server-side resources visible to all connections.
type xlibHandle struct {
	lib           unsafe.Pointer // libX11.so.6 handle
	display       uintptr        // Display* from XOpenDisplay
	xCloseDisplay unsafe.Pointer // XCloseDisplay symbol
	cifClose      *types.CallInterface
}

// Platform implements X11 windowing support.
type Platform struct {
	mu sync.Mutex

	// X11 connection (pure Go wire protocol for events)
	conn *Connection

	// Xlib Display* for Vulkan surface creation
	xlib *xlibHandle

	// Standard atoms
	atoms *StandardAtoms

	// Window
	window ResourceID

	// Keyboard mapping
	keymap *KeyboardMapping

	// Window state
	width       int
	height      int
	shouldClose bool
	configured  bool

	// Pending resize
	pendingWidth  int
	pendingHeight int
	hasResize     bool

	// Mouse state tracking
	mouseX        float64
	mouseY        float64
	buttons       gpucontext.Buttons
	modifiers     gpucontext.Modifiers
	mouseInWindow bool

	// Callbacks for pointer and scroll events
	pointerCallback func(gpucontext.PointerEvent)
	scrollCallback  func(gpucontext.ScrollEvent)
	callbackMu      sync.RWMutex

	// Timestamp reference for event timing
	startTime time.Time
}

// NewPlatform creates a new X11 platform instance.
func NewPlatform() *Platform {
	return &Platform{
		startTime: time.Now(),
	}
}

// openXlibDisplay loads libX11.so.6 via goffi and calls XOpenDisplay to obtain
// a real Display* pointer for Vulkan surface creation (VK_KHR_xlib_surface).
// Returns nil if libX11 is not available (software-only fallback).
func openXlibDisplay() (*xlibHandle, error) {
	lib, err := ffi.LoadLibrary("libX11.so.6")
	if err != nil {
		return nil, fmt.Errorf("x11: failed to load libX11.so.6: %w", err)
	}

	xOpenDisplay, err := ffi.GetSymbol(lib, "XOpenDisplay")
	if err != nil {
		return nil, fmt.Errorf("x11: XOpenDisplay symbol not found: %w", err)
	}

	xCloseDisplay, err := ffi.GetSymbol(lib, "XCloseDisplay")
	if err != nil {
		return nil, fmt.Errorf("x11: XCloseDisplay symbol not found: %w", err)
	}

	// XOpenDisplay(const char* display_name) -> Display*
	cifOpen := &types.CallInterface{}
	err = ffi.PrepareCallInterface(cifOpen, types.DefaultCall, types.PointerTypeDescriptor, []*types.TypeDescriptor{
		types.PointerTypeDescriptor,
	})
	if err != nil {
		return nil, fmt.Errorf("x11: failed to prepare XOpenDisplay CIF: %w", err)
	}

	// XCloseDisplay(Display*) -> int
	cifClose := &types.CallInterface{}
	err = ffi.PrepareCallInterface(cifClose, types.DefaultCall, types.SInt32TypeDescriptor, []*types.TypeDescriptor{
		types.PointerTypeDescriptor,
	})
	if err != nil {
		return nil, fmt.Errorf("x11: failed to prepare XCloseDisplay CIF: %w", err)
	}

	// Pass DISPLAY env var to XOpenDisplay (NULL uses $DISPLAY automatically,
	// but we pass it explicitly for clarity in error messages).
	displayEnv := os.Getenv("DISPLAY")
	var displayArg uintptr
	if displayEnv != "" {
		// Convert Go string to null-terminated C string on the stack.
		cstr := append([]byte(displayEnv), 0)
		displayArg = uintptr(unsafe.Pointer(&cstr[0]))
	}

	var display uintptr
	args := [1]unsafe.Pointer{unsafe.Pointer(&displayArg)}
	ffi.CallFunction(cifOpen, xOpenDisplay, unsafe.Pointer(&display), args[:])

	if display == 0 {
		return nil, fmt.Errorf("x11: XOpenDisplay(%q) returned NULL", displayEnv)
	}

	logger().Info("XOpenDisplay succeeded", "DISPLAY", displayEnv, "display_ptr", fmt.Sprintf("%#x", display))

	return &xlibHandle{
		lib:           lib,
		display:       display,
		xCloseDisplay: xCloseDisplay,
		cifClose:      cifClose,
	}, nil
}

// close calls XCloseDisplay and releases the Xlib resources.
func (h *xlibHandle) close() {
	if h == nil || h.display == 0 {
		return
	}
	var result int
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	ffi.CallFunction(h.cifClose, h.xCloseDisplay, unsafe.Pointer(&result), args[:])
	h.display = 0
}

// Init creates the X11 window.
func (p *Platform) Init(config Config) error {
	// Connect to X server
	conn, err := Connect()
	if err != nil {
		return fmt.Errorf("x11: failed to connect: %w", err)
	}
	p.conn = conn

	// Intern standard atoms
	atoms, err := conn.InternStandardAtoms()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to intern atoms: %w", err)
	}
	p.atoms = atoms

	// Create window
	windowConfig := WindowConfig{
		Title:      config.Title,
		Width:      uint16(config.Width),
		Height:     uint16(config.Height),
		X:          0,
		Y:          0,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
	}

	window, err := conn.CreateWindow(windowConfig)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to create window: %w", err)
	}
	p.window = window

	// Set window properties
	if err := conn.SetWindowTitle(window, config.Title, atoms); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to set title: %w", err)
	}

	// Set WM protocols (for close button)
	if err := conn.SetWMProtocols(window, atoms); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to set WM protocols: %w", err)
	}

	// Set WM class
	if err := conn.SetWMClass(window, "gogpu", "GoGPU"); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to set WM class: %w", err)
	}

	// Set PID (non-fatal, some WMs don't support this)
	_ = conn.SetWMPID(window, atoms)

	// Set window type (non-fatal, some WMs don't support this)
	_ = conn.SetNetWMWindowType(window, atoms.NetWMWindowTypeNormal, atoms)

	// Handle non-resizable windows via Motif hints
	if !config.Resizable {
		hints := &MotifWMHints{
			Flags:       MotifHintsDecorations | MotifHintsFunctions,
			Decorations: MotifDecorBorder | MotifDecorTitle | MotifDecorMenu | MotifDecorMinimize,
			Functions:   1 | 2 | 8, // Move | Minimize | Close (no Resize or Maximize)
		}
		// Non-fatal, some WMs don't support Motif hints
		_ = conn.SetMotifWMHints(window, hints, atoms)
	}

	// Map (show) the window
	if err := conn.MapWindow(window); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to map window: %w", err)
	}

	// Get keyboard mapping (non-fatal - keyboard input may not work correctly without it)
	keymap, _ := conn.GetKeyboardMapping()
	p.keymap = keymap

	// Set fullscreen if requested (non-fatal, will fail if WM doesn't support EWMH)
	if config.Fullscreen {
		_ = conn.SetFullscreen(window, true, atoms)
	}

	// Store initial size
	p.width = config.Width
	p.height = config.Height
	p.configured = true

	// Flush to ensure all requests are sent
	_ = conn.Flush()

	// Sync to ensure window is created
	_ = conn.Sync()

	// Open Xlib Display* for Vulkan surface creation.
	// VK_KHR_xlib_surface requires a real Display* pointer, not a socket FD.
	// Non-fatal: if libX11 is unavailable, GPU rendering won't work but
	// the software backend can still function.
	xlib, err := openXlibDisplay()
	if err != nil {
		// Log but continue — software backend doesn't need Display*
		fmt.Fprintf(os.Stderr, "gogpu: warning: %v (GPU rendering unavailable)\n", err)
	}
	p.xlib = xlib

	if xlib != nil {
		logger().Info("x11 init complete", "window", fmt.Sprintf("%#x", p.window), "display", fmt.Sprintf("%#x", xlib.display))
	} else {
		logger().Warn("x11 init without xlib", "window", fmt.Sprintf("%#x", p.window))
	}

	return nil
}

// PollEvents processes pending X11 events.
func (p *Platform) PollEvents() PlatformEvent {
	p.mu.Lock()

	// Check for pending resize
	if p.hasResize {
		p.width = p.pendingWidth
		p.height = p.pendingHeight
		p.hasResize = false
		p.mu.Unlock()

		return PlatformEvent{
			Type:   EventTypeResize,
			Width:  p.pendingWidth,
			Height: p.pendingHeight,
		}
	}

	// Check for close
	if p.shouldClose {
		p.mu.Unlock()
		return PlatformEvent{Type: EventTypeClose}
	}

	p.mu.Unlock()

	// Process pending events
	for {
		event, err := p.conn.PollEvent()
		if err != nil {
			p.mu.Lock()
			p.shouldClose = true
			p.mu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

		if event == nil {
			break
		}

		if platformEvent := p.handleEvent(event); platformEvent.Type != EventTypeNone {
			return platformEvent
		}
	}

	// Check again after processing
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.hasResize {
		p.width = p.pendingWidth
		p.height = p.pendingHeight
		p.hasResize = false
		return PlatformEvent{
			Type:   EventTypeResize,
			Width:  p.pendingWidth,
			Height: p.pendingHeight,
		}
	}

	if p.shouldClose {
		return PlatformEvent{Type: EventTypeClose}
	}

	return PlatformEvent{Type: EventTypeNone}
}

// handleEvent processes a single X11 event.
func (p *Platform) handleEvent(event Event) PlatformEvent {
	switch e := event.(type) {
	case *ConfigureNotifyEvent:
		if e.Window == p.window {
			p.mu.Lock()
			newWidth := int(e.Width)
			newHeight := int(e.Height)
			if newWidth != p.width || newHeight != p.height {
				p.pendingWidth = newWidth
				p.pendingHeight = newHeight
				p.hasResize = true
			}
			p.mu.Unlock()

			if p.hasResize {
				return PlatformEvent{
					Type:   EventTypeResize,
					Width:  newWidth,
					Height: newHeight,
				}
			}
		}

	case *ClientMessageEvent:
		if e.IsDeleteWindow(p.atoms) {
			p.mu.Lock()
			p.shouldClose = true
			p.mu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

	case *DestroyNotifyEvent:
		if e.Window == p.window {
			p.mu.Lock()
			p.shouldClose = true
			p.mu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

	case *ExposeEvent:
		// Could trigger redraw, but for now we just ignore
		// The main render loop should handle this

	case *MapNotifyEvent:
		p.mu.Lock()
		p.configured = true
		p.mu.Unlock()

	case *MotionNotifyEvent:
		p.handleMotionNotify(e)

	case *ButtonPressEvent:
		p.handleButtonPress(e)

	case *ButtonReleaseEvent:
		p.handleButtonRelease(e)

	case *EnterNotifyEvent:
		p.handleEnterNotify(e)

	case *LeaveNotifyEvent:
		p.handleLeaveNotify(e)
	}

	return PlatformEvent{Type: EventTypeNone}
}

// handleMotionNotify processes mouse movement events.
func (p *Platform) handleMotionNotify(e *MotionNotifyEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	p.mu.Lock()
	p.mouseX = x
	p.mouseY = y
	p.buttons = extractButtons(e.State)
	p.modifiers = extractModifiers(e.State)
	p.mu.Unlock()

	ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, x, y, e.State)
	p.dispatchPointerEvent(ev)
}

// handleButtonPress processes mouse button press events.
func (p *Platform) handleButtonPress(e *ButtonPressEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	// Scroll buttons (4-7) are emulated as button presses in X11
	if isScrollButton(e.Detail) {
		p.handleScrollButton(e.Detail, x, y, e.State)
		return
	}

	// Regular button press
	button := x11ButtonToButton(e.Detail)
	if button == gpucontext.ButtonNone {
		return // Unknown button
	}

	p.mu.Lock()
	p.mouseX = x
	p.mouseY = y
	// Update button state - button is now pressed
	switch button {
	case gpucontext.ButtonLeft:
		p.buttons |= gpucontext.ButtonsLeft
	case gpucontext.ButtonMiddle:
		p.buttons |= gpucontext.ButtonsMiddle
	case gpucontext.ButtonRight:
		p.buttons |= gpucontext.ButtonsRight
	case gpucontext.ButtonX1:
		p.buttons |= gpucontext.ButtonsX1
	case gpucontext.ButtonX2:
		p.buttons |= gpucontext.ButtonsX2
	}
	p.modifiers = extractModifiers(e.State)
	p.mu.Unlock()

	ev := p.createPointerEvent(gpucontext.PointerDown, button, x, y, e.State)
	// Add the pressed button to the buttons mask for PointerDown
	switch button {
	case gpucontext.ButtonLeft:
		ev.Buttons |= gpucontext.ButtonsLeft
	case gpucontext.ButtonMiddle:
		ev.Buttons |= gpucontext.ButtonsMiddle
	case gpucontext.ButtonRight:
		ev.Buttons |= gpucontext.ButtonsRight
	case gpucontext.ButtonX1:
		ev.Buttons |= gpucontext.ButtonsX1
	case gpucontext.ButtonX2:
		ev.Buttons |= gpucontext.ButtonsX2
	}
	p.dispatchPointerEvent(ev)
}

// handleButtonRelease processes mouse button release events.
func (p *Platform) handleButtonRelease(e *ButtonReleaseEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	// Scroll button releases are ignored (scroll is handled on press)
	if isScrollButton(e.Detail) {
		return
	}

	// Regular button release
	button := x11ButtonToButton(e.Detail)
	if button == gpucontext.ButtonNone {
		return // Unknown button
	}

	p.mu.Lock()
	p.mouseX = x
	p.mouseY = y
	// Update button state - button is now released
	switch button {
	case gpucontext.ButtonLeft:
		p.buttons &^= gpucontext.ButtonsLeft
	case gpucontext.ButtonMiddle:
		p.buttons &^= gpucontext.ButtonsMiddle
	case gpucontext.ButtonRight:
		p.buttons &^= gpucontext.ButtonsRight
	case gpucontext.ButtonX1:
		p.buttons &^= gpucontext.ButtonsX1
	case gpucontext.ButtonX2:
		p.buttons &^= gpucontext.ButtonsX2
	}
	p.modifiers = extractModifiers(e.State)
	p.mu.Unlock()

	ev := p.createPointerEvent(gpucontext.PointerUp, button, x, y, e.State)
	p.dispatchPointerEvent(ev)
}

// handleScrollButton processes X11 scroll button events (buttons 4-7).
func (p *Platform) handleScrollButton(detail uint8, x, y float64, state uint16) {
	var deltaX, deltaY float64

	switch detail {
	case x11ButtonScrollUp:
		deltaY = -1.0 // Scroll up = negative deltaY (content moves up)
	case x11ButtonScrollDown:
		deltaY = 1.0 // Scroll down = positive deltaY (content moves down)
	case x11ButtonScrollLeft:
		deltaX = -1.0 // Scroll left = negative deltaX
	case x11ButtonScrollRight:
		deltaX = 1.0 // Scroll right = positive deltaX
	default:
		return
	}

	ev := gpucontext.ScrollEvent{
		X:         x,
		Y:         y,
		DeltaX:    deltaX,
		DeltaY:    deltaY,
		DeltaMode: gpucontext.ScrollDeltaLine,
		Modifiers: extractModifiers(state),
		Timestamp: p.eventTimestamp(),
	}
	p.dispatchScrollEvent(ev)
}

// handleEnterNotify processes pointer enter events.
func (p *Platform) handleEnterNotify(e *EnterNotifyEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	p.mu.Lock()
	p.mouseX = x
	p.mouseY = y
	p.buttons = extractButtons(e.State)
	p.modifiers = extractModifiers(e.State)
	p.mouseInWindow = true
	p.mu.Unlock()

	ev := p.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, x, y, e.State)
	p.dispatchPointerEvent(ev)
}

// handleLeaveNotify processes pointer leave events.
func (p *Platform) handleLeaveNotify(e *LeaveNotifyEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	p.mu.Lock()
	p.mouseX = x
	p.mouseY = y
	p.buttons = extractButtons(e.State)
	p.modifiers = extractModifiers(e.State)
	p.mouseInWindow = false
	p.mu.Unlock()

	ev := gpucontext.PointerEvent{
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
		Buttons:     extractButtons(e.State),
		Modifiers:   extractModifiers(e.State),
		Timestamp:   p.eventTimestamp(),
	}
	p.dispatchPointerEvent(ev)
}

// ShouldClose returns true if window close was requested.
func (p *Platform) ShouldClose() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shouldClose
}

// GetSize returns current window size in pixels.
func (p *Platform) GetSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width, p.height
}

// GetHandle returns platform-specific handles for Vulkan surface creation.
// Returns (Display* pointer, X11 Window ID) for use with VK_KHR_xlib_surface.
// The Display* comes from XOpenDisplay (loaded via goffi), not from our pure Go
// X11 wire protocol connection. Window IDs are server-side resources shared
// across all connections to the same X server.
func (p *Platform) GetHandle() (display, window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.xlib == nil || p.xlib.display == 0 {
		logger().Warn("GetHandle returning zero handles", "reason", "no xlib display")
		return 0, 0
	}

	logger().Debug("GetHandle", "display", fmt.Sprintf("%#x", p.xlib.display), "window", fmt.Sprintf("%#x", uintptr(p.window)))
	return p.xlib.display, uintptr(p.window)
}

// Fd returns the X11 connection file descriptor.
// This can be used with poll/epoll for event loop integration.
func (p *Platform) Fd() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		return -1
	}
	return p.conn.Fd()
}

// Destroy closes the window and releases resources.
func (p *Platform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close Xlib Display* (Vulkan surface handle)
	if p.xlib != nil {
		p.xlib.close()
		p.xlib = nil
	}

	if p.conn != nil {
		if p.window != 0 {
			_ = p.conn.DestroyWindow(p.window)
			p.window = 0
		}
		_ = p.conn.Close()
		p.conn = nil
	}

	p.atoms = nil
	p.keymap = nil
}

// SetPointerCallback registers a callback for pointer events.
func (p *Platform) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	p.callbackMu.Lock()
	p.pointerCallback = fn
	p.callbackMu.Unlock()
}

// SetScrollCallback registers a callback for scroll events.
func (p *Platform) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	p.callbackMu.Lock()
	p.scrollCallback = fn
	p.callbackMu.Unlock()
}

// dispatchPointerEvent dispatches a pointer event to the registered callback.
func (p *Platform) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	p.callbackMu.RLock()
	callback := p.pointerCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// dispatchScrollEvent dispatches a scroll event to the registered callback.
func (p *Platform) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	p.callbackMu.RLock()
	callback := p.scrollCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// eventTimestamp returns the event timestamp as duration since start.
func (p *Platform) eventTimestamp() time.Duration {
	return time.Since(p.startTime)
}

// X11 button constants.
const (
	x11ButtonLeft        = 1
	x11ButtonMiddle      = 2
	x11ButtonRight       = 3
	x11ButtonScrollUp    = 4
	x11ButtonScrollDown  = 5
	x11ButtonScrollLeft  = 6
	x11ButtonScrollRight = 7
	x11ButtonX1          = 8
	x11ButtonX2          = 9
)

// X11 modifier mask constants.
const (
	x11ModShift   = 1 << 0  // Bit 0: Shift
	x11ModLock    = 1 << 1  // Bit 1: Caps Lock
	x11ModControl = 1 << 2  // Bit 2: Control
	x11ModMod1    = 1 << 3  // Bit 3: Mod1 (Alt)
	x11ModMod2    = 1 << 4  // Bit 4: Mod2 (Num Lock)
	x11ModMod3    = 1 << 5  // Bit 5: Mod3
	x11ModMod4    = 1 << 6  // Bit 6: Mod4 (Super/Windows)
	x11ModMod5    = 1 << 7  // Bit 7: Mod5
	x11ModButton1 = 1 << 8  // Button1 (left) pressed
	x11ModButton2 = 1 << 9  // Button2 (middle) pressed
	x11ModButton3 = 1 << 10 // Button3 (right) pressed
)

// extractModifiers extracts keyboard modifiers from X11 state.
func extractModifiers(state uint16) gpucontext.Modifiers {
	var mods gpucontext.Modifiers
	if state&x11ModShift != 0 {
		mods |= gpucontext.ModShift
	}
	if state&x11ModControl != 0 {
		mods |= gpucontext.ModControl
	}
	if state&x11ModMod1 != 0 {
		mods |= gpucontext.ModAlt
	}
	if state&x11ModMod4 != 0 {
		mods |= gpucontext.ModSuper
	}
	if state&x11ModLock != 0 {
		mods |= gpucontext.ModCapsLock
	}
	if state&x11ModMod2 != 0 {
		mods |= gpucontext.ModNumLock
	}
	return mods
}

// extractButtons extracts button state from X11 state.
func extractButtons(state uint16) gpucontext.Buttons {
	var btns gpucontext.Buttons
	if state&x11ModButton1 != 0 {
		btns |= gpucontext.ButtonsLeft
	}
	if state&x11ModButton2 != 0 {
		btns |= gpucontext.ButtonsMiddle
	}
	if state&x11ModButton3 != 0 {
		btns |= gpucontext.ButtonsRight
	}
	return btns
}

// x11ButtonToButton converts X11 button number to gpucontext.Button.
func x11ButtonToButton(detail uint8) gpucontext.Button {
	switch detail {
	case x11ButtonLeft:
		return gpucontext.ButtonLeft
	case x11ButtonMiddle:
		return gpucontext.ButtonMiddle
	case x11ButtonRight:
		return gpucontext.ButtonRight
	case x11ButtonX1:
		return gpucontext.ButtonX1
	case x11ButtonX2:
		return gpucontext.ButtonX2
	default:
		return gpucontext.ButtonNone
	}
}

// isScrollButton returns true if the X11 button is a scroll button (4-7).
func isScrollButton(detail uint8) bool {
	return detail >= x11ButtonScrollUp && detail <= x11ButtonScrollRight
}

// createPointerEvent creates a PointerEvent with common fields filled in.
func (p *Platform) createPointerEvent(
	eventType gpucontext.PointerEventType,
	button gpucontext.Button,
	x, y float64,
	state uint16,
) gpucontext.PointerEvent {
	buttons := extractButtons(state)
	modifiers := extractModifiers(state)

	// For button down/up, set pressure based on button state
	var pressure float32
	if eventType == gpucontext.PointerDown || buttons != gpucontext.ButtonsNone {
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
		Buttons:     buttons,
		Modifiers:   modifiers,
		Timestamp:   p.eventTimestamp(),
	}
}
