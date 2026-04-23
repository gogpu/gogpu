//go:build linux

package platform

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gogpu/gogpu/internal/platform/wayland"
	"github.com/gogpu/gogpu/internal/platform/x11"
	"github.com/gogpu/gpucontext"
)

// waylandWindow holds all per-window state for a Wayland surface.
type waylandWindow struct {
	// Frameless window state
	frameless       bool
	maximized       bool
	hitTestCallback func(x, y float64) gpucontext.HitTestResult

	// Window state
	width       int
	height      int
	shouldClose bool
	configured  bool

	// Event queue (same pattern as X11 and Windows platforms)
	events  []Event
	eventMu sync.Mutex

	savedWidth  int // pre-maximize size for restore
	savedHeight int

	// Pointer state tracking
	pointerX  float64
	pointerY  float64
	buttons   gpucontext.Buttons
	modifiers gpucontext.Modifiers
	pointerMu sync.RWMutex
	pointerIn bool // True when pointer is inside our surface
	startTime time.Time

	// Cursor mode (0=normal, 1=locked, 2=confined)
	cursorMode int

	// Keyboard focus tracking
	keyboardFocused bool

	// Callbacks for pointer, scroll, and keyboard events
	pointerCallback  func(gpucontext.PointerEvent)
	scrollCallback   func(gpucontext.ScrollEvent)
	keyboardCallback func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)
	charCallback     func(rune)
	callbackMu       sync.RWMutex
}

// waylandPlatform implements the Platform interface using Wayland.
// Uses a single libwayland-client C connection for everything:
// display, registry, compositor, surface, xdg-shell, input, CSD.
// Holds process-level state and delegates per-window operations to primary.
type waylandPlatform struct {
	mu sync.Mutex

	// Wakeup pipe for cross-goroutine WakeUp → WaitEvents unblocking.
	// [0]=read, [1]=write. Created with O_NONBLOCK|O_CLOEXEC.
	wakePipe [2]int

	// Single C libwayland connection — owns everything.
	libwl *wayland.LibwaylandHandle

	// Pure Go protocol objects — kept for registry global discovery only.
	// The Pure Go display is used during init to discover global names,
	// then those names are used to bind on the C connection.
	// After init, only libwl is used for event dispatch.
	display  *wayland.Display
	registry *wayland.Registry

	// Scale factor from environment variables (fallback)
	envScaleFactor float64

	// Primary window for backward-compatible single-window API.
	primary *waylandWindow
}

// x11Platform wraps x11.Platform to implement the Platform interface.
type x11Platform struct {
	inner *x11.Platform

	// Channel-based wakeup for cross-goroutine WakeUp → WaitEvents unblocking.
	// Replaces the wakePipe+unix.Poll pattern to avoid dual-poller race with
	// net.Conn (Go runtime netpoller vs kernel poll on dup'd fd).
	wakeCh chan struct{}
}

// newPlatformManager returns a PlatformManager for Linux.
// Detects Wayland vs X11 from environment variables.
func newPlatformManager() PlatformManager {
	// Prefer Wayland if WAYLAND_DISPLAY is set
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		logger().Info("platform selected", "type", "wayland", "WAYLAND_DISPLAY", os.Getenv("WAYLAND_DISPLAY"))
		return &waylandPlatform{
			primary: &waylandWindow{startTime: time.Now()},
		}
	}
	// Fall back to X11 if DISPLAY is set
	if os.Getenv("DISPLAY") != "" {
		logger().Info("platform selected", "type", "x11", "DISPLAY", os.Getenv("DISPLAY"))
		x11.SetLogger(loggerPtr.Load().WithGroup("x11"))
		return &x11Platform{}
	}
	// Default to Wayland (will fail in Init if not available)
	logger().Info("platform selected", "type", "wayland", "reason", "default (no WAYLAND_DISPLAY or DISPLAY)")
	return &waylandPlatform{
		primary: &waylandWindow{startTime: time.Now()},
	}
}

// --- PlatformManager implementation on x11Platform ---

// Init initializes the X11 platform subsystem (process-level, no window).
func (p *x11Platform) Init() error {
	p.inner = x11.NewPlatform()
	p.wakeCh = make(chan struct{}, 1)
	return nil
}

// CreateWindow creates an X11 window.
func (p *x11Platform) CreateWindow(config Config) (PlatformWindow, error) {
	x11Config := x11.Config{
		Title:      config.Title,
		Width:      config.Width,
		Height:     config.Height,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
		Frameless:  config.Frameless,
	}
	if err := p.inner.Init(x11Config); err != nil {
		return nil, err
	}
	return &x11PlatformWindow{platform: p, id: NewWindowID()}, nil
}

// PollEvents processes pending X11 events.
func (p *x11Platform) PollEvents() Event {
	event := p.inner.PollEvents()
	switch event.Type {
	case x11.EventTypeClose:
		return Event{Type: EventClose}
	case x11.EventTypeResize:
		// X11: scale=1.0 baseline, logical == physical
		return Event{
			Type:           EventResize,
			Width:          event.Width,
			Height:         event.Height,
			PhysicalWidth:  event.Width,
			PhysicalHeight: event.Height,
		}
	default:
		return Event{Type: EventNone}
	}
}

// WaitEvents blocks until at least one OS event is available or WakeUp is called.
// Uses PollEventTimeout on the X11 net.Conn (Go runtime netpoller) with periodic
// wake channel checks. This avoids the dual-poller race between unix.Poll on a
// dup'd fd and Go's runtime netpoller on the original net.Conn.
func (p *x11Platform) WaitEvents() {
	for {
		select {
		case <-p.wakeCh:
			return
		default:
		}

		event, err := p.inner.PollEventTimeout(100 * time.Millisecond)
		if err != nil {
			return
		}
		if event != nil {
			if pe := p.inner.HandleEvent(event); pe.Type != x11.EventTypeNone {
				p.inner.QueueEvent(pe)
			}
			return
		}
	}
}

// WakeUp unblocks WaitEvents from any goroutine.
// Non-blocking channel send ensures at most one pending signal.
func (p *x11Platform) WakeUp() {
	select {
	case p.wakeCh <- struct{}{}:
	default:
	}
}

// ClipboardRead reads text from the system clipboard.
// TODO(PLAT-008): Implement using X11 selections (XA_CLIPBOARD).
func (p *x11Platform) ClipboardRead() (string, error) { return "", nil }

// ClipboardWrite writes text to the system clipboard.
// TODO(PLAT-008): Implement using X11 selections (XA_CLIPBOARD).
func (p *x11Platform) ClipboardWrite(string) error { return nil }

// DarkMode returns true if the system dark mode is active.
func (p *x11Platform) DarkMode() bool { return detectDarkMode() }

// ReduceMotion returns true if the user prefers reduced animation.
func (p *x11Platform) ReduceMotion() bool { return detectReduceMotion() }

// HighContrast returns true if high contrast mode is active.
func (p *x11Platform) HighContrast() bool { return detectHighContrast() }

// FontScale returns font size preference multiplier.
func (p *x11Platform) FontScale() float32 { return detectFontScale() }

// Destroy closes all windows and releases resources.
func (p *x11Platform) Destroy() {
	p.inner.Destroy()
}

// --- x11PlatformWindow implements PlatformWindow ---

// x11PlatformWindow wraps x11Platform to implement PlatformWindow.
type x11PlatformWindow struct {
	platform *x11Platform
	id       WindowID
}

func (w *x11PlatformWindow) ID() WindowID                   { return w.id }
func (w *x11PlatformWindow) GetHandle() (uintptr, uintptr)  { return w.platform.inner.GetHandle() }
func (w *x11PlatformWindow) LogicalSize() (int, int)        { return w.platform.inner.GetSize() }
func (w *x11PlatformWindow) PhysicalSize() (int, int)       { return w.platform.inner.GetSize() }
func (w *x11PlatformWindow) ScaleFactor() float64           { return w.platform.inner.ScaleFactor() }
func (w *x11PlatformWindow) ShouldClose() bool              { return w.platform.inner.ShouldClose() }
func (w *x11PlatformWindow) InSizeMove() bool               { return false }
func (w *x11PlatformWindow) SetTitle(_ string)              {}
func (w *x11PlatformWindow) SetCursor(cursorID int)         { w.platform.inner.SetCursor(cursorID) }
func (w *x11PlatformWindow) SetCursorMode(mode int)         { w.platform.inner.SetCursorMode(mode) }
func (w *x11PlatformWindow) CursorMode() int                { return w.platform.inner.GetCursorMode() }
func (w *x11PlatformWindow) SyncFrame()                     {}
func (w *x11PlatformWindow) SetModalFrameCallback(_ func()) {}

func (w *x11PlatformWindow) PrepareFrame() PrepareFrameResult {
	pw, ph := w.platform.inner.GetSize()
	return PrepareFrameResult{
		ScaleFactor:    w.platform.inner.ScaleFactor(),
		PhysicalWidth:  uint32(pw),
		PhysicalHeight: uint32(ph),
	}
}

func (w *x11PlatformWindow) SetFrameless(frameless bool) {
	w.platform.inner.SetFrameless(frameless)
}

func (w *x11PlatformWindow) IsFrameless() bool {
	return w.platform.inner.IsFrameless()
}

func (w *x11PlatformWindow) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	w.platform.inner.SetHitTestCallback(fn)
}

func (w *x11PlatformWindow) Minimize()         { w.platform.inner.Minimize() }
func (w *x11PlatformWindow) Maximize()         { w.platform.inner.Maximize() }
func (w *x11PlatformWindow) IsMaximized() bool { return w.platform.inner.IsMaximized() }
func (w *x11PlatformWindow) Close()            { w.platform.inner.CloseWindow() }

func (w *x11PlatformWindow) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	w.platform.inner.SetPointerCallback(fn)
}

func (w *x11PlatformWindow) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	w.platform.inner.SetScrollCallback(fn)
}

func (w *x11PlatformWindow) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	w.platform.inner.SetKeyCallback(fn)
}

func (w *x11PlatformWindow) SetCharCallback(fn func(rune)) {
	w.platform.inner.SetCharCallback(fn)
}

// BlitPixels copies RGBA pixel data to the window using X11 PutImage.
func (w *x11PlatformWindow) BlitPixels(pixels []byte, width, height int) error {
	return w.platform.inner.BlitPixels(pixels, width, height)
}

func (w *x11PlatformWindow) Destroy() {
	// Destruction handled by platform.Destroy()
}

// --- waylandPlatformWindow implements PlatformWindow ---

// waylandPlatformWindow wraps waylandPlatform to implement PlatformWindow.
type waylandPlatformWindow struct {
	platform *waylandPlatform
	id       WindowID
}

func (w *waylandPlatformWindow) ID() WindowID { return w.id }

func (w *waylandPlatformWindow) GetHandle() (uintptr, uintptr) {
	if w.platform.libwl != nil {
		return w.platform.libwl.Display(), w.platform.libwl.Surface()
	}
	return 0, 0
}

func (w *waylandPlatformWindow) LogicalSize() (int, int) {
	wp := w.platform.primary
	wp.eventMu.Lock()
	defer wp.eventMu.Unlock()
	return wp.width, wp.height
}

func (w *waylandPlatformWindow) PhysicalSize() (int, int) {
	return w.LogicalSize() // Wayland: scale tracking TODO
}

func (w *waylandPlatformWindow) ScaleFactor() float64 {
	return w.platform.ScaleFactor()
}

func (w *waylandPlatformWindow) ShouldClose() bool {
	wp := w.platform.primary
	wp.eventMu.Lock()
	defer wp.eventMu.Unlock()
	return wp.shouldClose
}

func (w *waylandPlatformWindow) InSizeMove() bool { return false }
func (w *waylandPlatformWindow) SetTitle(_ string) {
	// TODO: libwl.SetTitle for runtime title change
}

func (w *waylandPlatformWindow) PrepareFrame() PrepareFrameResult {
	pw, ph := w.PhysicalSize()
	return PrepareFrameResult{
		ScaleFactor:    w.ScaleFactor(),
		PhysicalWidth:  uint32(pw),
		PhysicalHeight: uint32(ph),
	}
}

func (w *waylandPlatformWindow) SetCursor(cursorID int) {
	w.platform.SetCursor(cursorID)
}

func (w *waylandPlatformWindow) SetCursorMode(mode int) {
	w.platform.SetCursorMode(mode)
}

func (w *waylandPlatformWindow) CursorMode() int {
	return w.platform.CursorMode()
}

func (w *waylandPlatformWindow) SyncFrame() {}

func (w *waylandPlatformWindow) SetFrameless(frameless bool) {
	w.platform.SetFrameless(frameless)
}

func (w *waylandPlatformWindow) IsFrameless() bool {
	return w.platform.IsFrameless()
}

func (w *waylandPlatformWindow) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	w.platform.SetHitTestCallback(fn)
}

func (w *waylandPlatformWindow) Minimize()         { w.platform.Minimize() }
func (w *waylandPlatformWindow) Maximize()         { w.platform.Maximize() }
func (w *waylandPlatformWindow) IsMaximized() bool { return w.platform.IsMaximized() }
func (w *waylandPlatformWindow) Close()            { w.platform.CloseWindow() }

func (w *waylandPlatformWindow) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	w.platform.SetPointerCallback(fn)
}

func (w *waylandPlatformWindow) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	w.platform.SetScrollCallback(fn)
}

func (w *waylandPlatformWindow) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	w.platform.SetKeyCallback(fn)
}

func (w *waylandPlatformWindow) SetCharCallback(fn func(rune)) {
	w.platform.SetCharCallback(fn)
}

func (w *waylandPlatformWindow) SetModalFrameCallback(_ func()) {}

func (w *waylandPlatformWindow) Destroy() {
	// Destruction handled by platform.Destroy()
}

// --- PlatformManager implementation on waylandPlatform ---

// Init initializes the Wayland platform subsystem (process-level).
// Creates the wake pipe for cross-goroutine event unblocking.
// Does NOT create any windows — that is deferred to CreateWindow.
func (p *waylandPlatform) Init() error {
	// Check if Wayland is available
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("wayland: WAYLAND_DISPLAY not set")
	}

	// Create wakeup pipe for WakeUp → WaitEvents unblocking
	if err := unix.Pipe2(p.wakePipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		return fmt.Errorf("wayland: wakeup pipe: %w", err)
	}

	return nil
}

// CreateWindow creates a Wayland window with the given configuration.
func (p *waylandPlatform) CreateWindow(config Config) (PlatformWindow, error) {
	if err := p.initSingleConnection(config); err != nil {
		return nil, err
	}
	return &waylandPlatformWindow{platform: p, id: NewWindowID()}, nil
}

// initSingleConnection initializes using a single C libwayland connection.
// Uses Pure Go wire protocol ONLY for registry global discovery, then
// creates all objects on the C connection via goffi.
func (p *waylandPlatform) initSingleConnection(config Config) error {
	// Step 1: Use Pure Go protocol to discover registry globals.
	// This is lightweight (just reads global names/versions), then we disconnect.
	display, err := wayland.Connect()
	if err != nil {
		return fmt.Errorf("wayland: failed to connect (Go): %w", err)
	}
	p.display = display

	registry, err := display.GetRegistry()
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to get registry: %w", err)
	}
	p.registry = registry

	required := []string{
		wayland.InterfaceWlCompositor,
		wayland.InterfaceXdgWmBase,
	}
	if err := registry.WaitForGlobals(required, 5); err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: %w", err)
	}

	// Collect global names/versions for C-side binding
	compGlobal := registry.GetGlobalByInterface(wayland.InterfaceWlCompositor)
	xdgGlobal := registry.GetGlobalByInterface(wayland.InterfaceXdgWmBase)
	if compGlobal == nil || xdgGlobal == nil {
		_ = display.Close()
		return fmt.Errorf("wayland: wl_compositor or xdg_wm_base not found")
	}

	var decorName, decorVersion uint32
	decorGlobal := registry.GetGlobalByInterface(wayland.InterfaceZxdgDecorationManagerV1)
	if decorGlobal != nil {
		decorName = decorGlobal.Name
		decorVersion = decorGlobal.Version
	}

	// Step 2: Open C libwayland connection — this is the SINGLE connection
	// that owns everything: surface, xdg-shell, input, Vulkan.
	libwl, err := wayland.OpenLibwayland(
		compGlobal.Name, compGlobal.Version,
		xdgGlobal.Name, xdgGlobal.Version,
		decorName, decorVersion,
	)
	if err != nil {
		_ = display.Close()
		return fmt.Errorf("wayland: failed to open libwayland: %w", err)
	}
	p.libwl = libwl

	// Set initial size on the primary window
	p.primary.width = config.Width
	p.primary.height = config.Height

	// Set window properties on C xdg_toplevel
	libwl.SetTitle(config.Title)
	libwl.SetAppID("gogpu")

	// Set size constraints if not resizable
	if !config.Resizable {
		libwl.SetMinSize(int32(config.Width), int32(config.Height))
		libwl.SetMaxSize(int32(config.Width), int32(config.Height))
	}

	// Set fullscreen if requested
	if config.Fullscreen {
		libwl.SetFullscreen()
	}

	// Register input callbacks BEFORE setting up input
	p.setupInputCallbacks()
	libwl.SetAsInputHandler()

	// Set up xdg_toplevel listeners (configure, close)
	if err := libwl.SetupToplevelListeners(); err != nil {
		logger().Warn("xdg_toplevel listener setup failed", "err", err)
	}

	// Flush + roundtrip to process initial events
	if err := libwl.Flush(); err != nil {
		libwl.Close()
		_ = display.Close()
		return fmt.Errorf("wayland: flush failed: %w", err)
	}
	if err := libwl.Roundtrip(); err != nil {
		libwl.Close()
		_ = display.Close()
		return fmt.Errorf("wayland: roundtrip failed: %w", err)
	}

	p.primary.configured = true

	// Detect env-based scale factor as fallback
	p.envScaleFactor = detectEnvScaleFactor()

	// Set up input devices (pointer, keyboard, touch) on C display
	seatGlobal := registry.GetGlobalByInterface(wayland.InterfaceWlSeat)
	if seatGlobal != nil {
		if err := libwl.SetupInput(seatGlobal.Name, seatGlobal.Version); err != nil {
			logger().Warn("input setup failed", "err", err)
		}
	}

	// Bind pointer constraints and relative pointer protocols (optional, for mouse grab)
	ptrConstraintsGlobal := registry.GetGlobalByInterface(wayland.InterfaceZwpPointerConstraintsV1)
	if ptrConstraintsGlobal != nil {
		if err := libwl.SetupPointerConstraints(ptrConstraintsGlobal.Name, ptrConstraintsGlobal.Version); err != nil {
			logger().Warn("pointer constraints setup failed (mouse grab unavailable)", "err", err)
		} else {
			logger().Debug("pointer constraints protocol bound")
		}
	}
	relPointerGlobal := registry.GetGlobalByInterface(wayland.InterfaceZwpRelativePointerManagerV1)
	if relPointerGlobal != nil {
		if err := libwl.SetupRelativePointerManager(relPointerGlobal.Name, relPointerGlobal.Version); err != nil {
			logger().Warn("relative pointer setup failed", "err", err)
		} else {
			logger().Debug("relative pointer manager bound")
		}
	}

	// Activate CSD if SSD was not available and window is not frameless
	if decorGlobal == nil && !config.Frameless {
		if err := p.initCSD(config); err != nil {
			logger().Warn("CSD initialization failed, running without decorations", "err", err)
		}
	}

	logger().Info("Wayland initialized (single C connection)",
		"display", fmt.Sprintf("%#x", libwl.Display()),
		"surface", fmt.Sprintf("%#x", libwl.Surface()))

	return nil
}

// initCSD initializes Client-Side Decorations when SSD is unavailable.
// Creates subsurfaces on the C display (same connection as main surface).
func (p *waylandPlatform) initCSD(config Config) error {
	if p.libwl == nil {
		return fmt.Errorf("libwayland-client not available for CSD")
	}

	registry := p.registry

	// Check required globals
	subcompGlobal := registry.GetGlobalByInterface(wayland.InterfaceWlSubcompositor)
	shmGlobal := registry.GetGlobalByInterface(wayland.InterfaceWlShm)
	if subcompGlobal == nil || shmGlobal == nil {
		return fmt.Errorf("required CSD globals not found (subcompositor or shm)")
	}

	var seatName, seatVersion uint32
	seatGlobal := registry.GetGlobalByInterface(wayland.InterfaceWlSeat)
	if seatGlobal != nil {
		seatName = seatGlobal.Name
		seatVersion = seatGlobal.Version
	}

	if err := p.libwl.SetupCSD(
		subcompGlobal.Name, subcompGlobal.Version,
		shmGlobal.Name, shmGlobal.Version,
		seatName, seatVersion,
		config.Width, config.Height,
		config.Title,
		nil, // DefaultCSDPainter
		func() {
			logger().Info("CSD close button pressed")
			w := p.primary
			w.eventMu.Lock()
			w.shouldClose = true
			w.eventMu.Unlock()
			w.queueEvent(Event{Type: EventClose})
			p.WakeUp() // unblock WaitEvents so main loop sees shouldClose
		},
	); err != nil {
		return fmt.Errorf("CSD setup: %w", err)
	}

	logger().Info("CSD: client-side decorations activated",
		"titleBarHeight", wayland.DefaultCSDPainter{}.TitleBarHeight(),
		"borderWidth", wayland.DefaultCSDPainter{}.BorderWidth())

	return nil
}

// setupInputCallbacks creates Go-side input callbacks and wires them to
// the LibwaylandHandle. These callbacks are invoked by goffi from C context.
// All per-window state access goes through p.primary.
//
//nolint:gocognit,maintidx // callback setup is inherently complex but well-structured per event type
func (p *waylandPlatform) setupInputCallbacks() {
	w := p.primary
	cb := &wayland.InputCallbacks{
		// Pointer events
		OnPointerEnter: func(serial uint32, x, y float64) {
			w.pointerMu.Lock()
			w.pointerX = x
			w.pointerY = y
			w.pointerIn = true
			w.pointerMu.Unlock()

			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerEnter,
				PointerID:   1,
				X:           x,
				Y:           y,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeMouse,
				IsPrimary:   true,
				Button:      gpucontext.ButtonNone,
				Buttons:     w.getButtons(),
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnPointerLeave: func(serial uint32) {
			w.pointerMu.Lock()
			x := w.pointerX
			y := w.pointerY
			w.pointerIn = false
			w.pointerMu.Unlock()

			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerLeave,
				PointerID:   1,
				X:           x,
				Y:           y,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeMouse,
				IsPrimary:   true,
				Button:      gpucontext.ButtonNone,
				Buttons:     w.getButtons(),
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnPointerMotion: func(timeMs uint32, x, y float64) {
			w.pointerMu.Lock()
			if !w.pointerIn {
				w.pointerMu.Unlock()
				return
			}
			w.pointerX = x
			w.pointerY = y
			buttons := w.buttons
			w.pointerMu.Unlock()

			var pressure float32
			if buttons != gpucontext.ButtonsNone {
				pressure = 0.5
			}

			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerMove,
				PointerID:   1,
				X:           x,
				Y:           y,
				Pressure:    pressure,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeMouse,
				IsPrimary:   true,
				Button:      gpucontext.ButtonNone,
				Buttons:     buttons,
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnPointerButton: func(serial, timeMs, button, state uint32) {
			w.pointerMu.Lock()
			if !w.pointerIn {
				w.pointerMu.Unlock()
				return
			}

			btn := mapWaylandButton(button)
			mask := buttonToMask(btn)

			if state == wayland.PointerButtonStatePressed {
				w.buttons |= mask
			} else {
				w.buttons &^= mask
			}

			buttons := w.buttons
			x := w.pointerX
			y := w.pointerY
			w.pointerMu.Unlock()

			var eventType gpucontext.PointerEventType
			if state == wayland.PointerButtonStatePressed {
				eventType = gpucontext.PointerDown
			} else {
				eventType = gpucontext.PointerUp
			}

			var pressure float32
			if eventType == gpucontext.PointerDown || buttons != gpucontext.ButtonsNone {
				pressure = 0.5
			}

			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        eventType,
				PointerID:   1,
				X:           x,
				Y:           y,
				Pressure:    pressure,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeMouse,
				IsPrimary:   true,
				Button:      btn,
				Buttons:     buttons,
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnPointerAxis: func(timeMs, axis uint32, value float64) {
			w.pointerMu.Lock()
			if !w.pointerIn {
				w.pointerMu.Unlock()
				return
			}
			x := w.pointerX
			y := w.pointerY
			w.pointerMu.Unlock()

			var deltaX, deltaY float64
			switch axis {
			case wayland.PointerAxisVerticalScroll:
				deltaY = value
			case wayland.PointerAxisHorizontalScroll:
				deltaX = value
			}

			w.dispatchScrollEvent(gpucontext.ScrollEvent{
				X:         x,
				Y:         y,
				DeltaX:    deltaX,
				DeltaY:    deltaY,
				DeltaMode: gpucontext.ScrollDeltaPixel,
				Modifiers: w.getModifiers(),
				Timestamp: w.eventTimestamp(),
			})
		},

		// Keyboard events
		OnKeyboardKeymap: func(format uint32, fd int, size uint32) {
			// For now, ignore keymap (basic evdev keycode mapping used).
			// Full libxkbcommon integration is a future task.
		},
		OnKeyboardEnter: func(serial uint32, keys []uint32) {
			w.pointerMu.Lock()
			w.keyboardFocused = true
			w.pointerMu.Unlock()
		},
		OnKeyboardLeave: func(serial uint32) {
			w.pointerMu.Lock()
			w.keyboardFocused = false
			w.pointerMu.Unlock()
		},
		OnKeyboardKey: func(serial, timeMs, key, state uint32) {
			w.pointerMu.RLock()
			focused := w.keyboardFocused
			w.pointerMu.RUnlock()
			if !focused {
				return
			}

			gpuKey := evdevToKey(key)
			mods := w.getModifiers()
			pressed := state == wayland.KeyStatePressed

			w.dispatchKeyEvent(gpuKey, mods, pressed)

			// Dispatch character input on key press only.
			if pressed && mods&(gpucontext.ModControl|gpucontext.ModAlt|gpucontext.ModSuper) == 0 {
				shift := mods&gpucontext.ModShift != 0
				capsLock := mods&gpucontext.ModCapsLock != 0
				if r := evdevKeycodeToRune(key, shift, capsLock); r != 0 {
					w.callbackMu.RLock()
					cb := w.charCallback
					w.callbackMu.RUnlock()
					if cb != nil {
						cb(r)
					}
				}
			}
		},
		OnKeyboardModifiers: func(serial, modsDepressed, modsLatched, modsLocked, group uint32) {
			w.pointerMu.Lock()
			w.modifiers = evdevModsToModifiers(modsDepressed, modsLocked)
			w.pointerMu.Unlock()
		},
		OnKeyboardRepeat: func(rate, delay int32) {
			// Stored for future key repeat implementation
		},

		// Touch events
		OnTouchDown: func(serial, timeMs uint32, id int32, x, y float64) {
			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerDown,
				PointerID:   int(id) + 2,
				X:           x,
				Y:           y,
				Pressure:    0.5,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeTouch,
				IsPrimary:   id == 0,
				Button:      gpucontext.ButtonLeft,
				Buttons:     gpucontext.ButtonsLeft,
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnTouchUp: func(serial, timeMs uint32, id int32) {
			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerUp,
				PointerID:   int(id) + 2,
				Pressure:    0,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeTouch,
				IsPrimary:   id == 0,
				Button:      gpucontext.ButtonLeft,
				Buttons:     gpucontext.ButtonsNone,
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnTouchMotion: func(timeMs uint32, id int32, x, y float64) {
			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerMove,
				PointerID:   int(id) + 2,
				X:           x,
				Y:           y,
				Pressure:    0.5,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeTouch,
				IsPrimary:   id == 0,
				Button:      gpucontext.ButtonNone,
				Buttons:     gpucontext.ButtonsLeft,
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},
		OnTouchCancel: func() {
			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerLeave,
				PointerID:   2,
				PointerType: gpucontext.PointerTypeTouch,
				IsPrimary:   true,
				Timestamp:   w.eventTimestamp(),
			})
		},

		// Pointer constraint events
		OnLockedPointerLocked: func() {
			logger().Debug("wayland: pointer lock activated by compositor")
		},
		OnLockedPointerUnlocked: func() {
			logger().Debug("wayland: pointer lock deactivated by compositor")
		},
		OnRelativePointerMotion: func(timeUs uint64, dx, dy, dxUnaccel, dyUnaccel float64) {
			// Read cursor mode under pointerMu (SetCursorMode writes under p.mu).
			w.pointerMu.RLock()
			mode := w.cursorMode
			buttons := w.buttons
			w.pointerMu.RUnlock()

			// Only dispatch relative motion when in locked mode.
			// In normal/confined mode, absolute motion events are used.
			if mode != 1 {
				return
			}

			var pressure float32
			if buttons != gpucontext.ButtonsNone {
				pressure = 0.5
			}

			// In locked mode, X/Y stay at the lock position; only DeltaX/DeltaY matter.
			w.dispatchPointerEvent(gpucontext.PointerEvent{
				Type:        gpucontext.PointerMove,
				PointerID:   1,
				DeltaX:      dx,
				DeltaY:      dy,
				Pressure:    pressure,
				Width:       1,
				Height:      1,
				PointerType: gpucontext.PointerTypeMouse,
				IsPrimary:   true,
				Button:      gpucontext.ButtonNone,
				Buttons:     buttons,
				Modifiers:   w.getModifiers(),
				Timestamp:   w.eventTimestamp(),
			})
		},

		// xdg_toplevel events
		OnClose: func() {
			logger().Info("xdg_toplevel close event from compositor")
			w.eventMu.Lock()
			w.shouldClose = true
			w.eventMu.Unlock()
			w.queueEvent(Event{Type: EventClose})
			p.WakeUp() // unblock WaitEvents so main loop sees shouldClose
		},
		OnConfigure: func(width, height int32) {
			logger().Debug("wayland toplevel.configure", "rawW", width, "rawH", height)
			w.eventMu.Lock()
			defer w.eventMu.Unlock()

			isMaximized := p.libwl != nil && p.libwl.CSDActive() && p.libwl.IsMaximized()

			// Save pre-maximize size ONLY when transitioning TO maximized.
			// Don't overwrite on every configure — restore needs the original size.
			if isMaximized && w.savedWidth == 0 && w.width > 0 {
				w.savedWidth = w.width
				w.savedHeight = w.height
			}
			// Clear saved size when restored (so next maximize saves fresh)
			if !isMaximized && w.savedWidth > 0 && width > 0 {
				w.savedWidth = 0
				w.savedHeight = 0
			}

			// Width/height of 0 means client can choose — restore to saved size.
			if width == 0 && height == 0 && w.savedWidth > 0 {
				width = int32(w.savedWidth)
				height = int32(w.savedHeight)
				w.savedWidth = 0
				w.savedHeight = 0
			}
			if width > 0 && height > 0 {
				newWidth := int(width)
				newHeight := int(height)

				// Geometry = content area (0, 0, contentW, contentH).
				// Configure matches geometry → content size directly.
				// On maximize: compositor sends full screen size, but we need room
				// for the title bar inside the screen. Subtract only tbH.
				vulkanW := newWidth
				vulkanH := newHeight
				if isMaximized && p.libwl != nil && p.libwl.CSDActive() {
					tbH, _ := p.libwl.CSDBorders()
					vulkanH = newHeight - tbH
					if vulkanH < 1 {
						vulkanH = 1
					}
				}
				logger().Debug("wayland configure", "vulkanW", vulkanW, "vulkanH", vulkanH)
				if vulkanW != w.width || vulkanH != w.height {
					w.width = vulkanW
					w.height = vulkanH
					w.events = append(w.events, Event{
						Type:           EventResize,
						Width:          vulkanW,
						Height:         vulkanH,
						PhysicalWidth:  vulkanW,
						PhysicalHeight: vulkanH,
					})
					// Schedule CSD resize for xdgSurfaceConfigureCb (after ack_configure).
					if p.libwl != nil && p.libwl.CSDActive() {
						p.libwl.SetPendingCSDResize(vulkanW, vulkanH)
					}
				}
			}
		},
	}

	p.libwl.SetInputCallbacks(cb)
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
func (w *waylandWindow) getButtons() gpucontext.Buttons {
	w.pointerMu.RLock()
	defer w.pointerMu.RUnlock()
	return w.buttons
}

// getModifiers returns the current modifier state (thread-safe).
func (w *waylandWindow) getModifiers() gpucontext.Modifiers {
	w.pointerMu.RLock()
	defer w.pointerMu.RUnlock()
	return w.modifiers
}

// eventTimestamp returns the event timestamp as duration since start.
func (w *waylandWindow) eventTimestamp() time.Duration {
	return time.Since(w.startTime)
}

// dispatchPointerEvent dispatches a pointer event to the registered callback.
func (w *waylandWindow) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	w.callbackMu.RLock()
	callback := w.pointerCallback
	w.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// dispatchScrollEvent dispatches a scroll event to the registered callback.
func (w *waylandWindow) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	w.callbackMu.RLock()
	callback := w.scrollCallback
	w.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// dispatchKeyEvent dispatches a keyboard event to the registered callback.
func (w *waylandWindow) dispatchKeyEvent(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	w.callbackMu.RLock()
	callback := w.keyboardCallback
	w.callbackMu.RUnlock()

	if callback != nil {
		callback(key, mods, pressed)
	}
}

// queueEvent appends a platform event to the window's event queue.
func (w *waylandWindow) queueEvent(event Event) {
	w.eventMu.Lock()
	defer w.eventMu.Unlock()
	w.events = append(w.events, event)
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

// evdevKeycodeToRune converts a Linux evdev keycode to a printable rune.
// Assumes US QWERTY layout. Returns 0 for non-printable keys.
// This is a basic fallback; full Unicode support requires libxkbcommon.
//
//nolint:gocognit,maintidx // keycode-to-char mapping is inherently a large switch
func evdevKeycodeToRune(keycode uint32, shift, capsLock bool) rune {
	// Letters: apply shift XOR capsLock for case
	upper := shift != capsLock
	switch keycode {
	case 30: // A
		if upper {
			return 'A'
		}
		return 'a'
	case 48: // B
		if upper {
			return 'B'
		}
		return 'b'
	case 46: // C
		if upper {
			return 'C'
		}
		return 'c'
	case 32: // D
		if upper {
			return 'D'
		}
		return 'd'
	case 18: // E
		if upper {
			return 'E'
		}
		return 'e'
	case 33: // F
		if upper {
			return 'F'
		}
		return 'f'
	case 34: // G
		if upper {
			return 'G'
		}
		return 'g'
	case 35: // H
		if upper {
			return 'H'
		}
		return 'h'
	case 23: // I
		if upper {
			return 'I'
		}
		return 'i'
	case 36: // J
		if upper {
			return 'J'
		}
		return 'j'
	case 37: // K
		if upper {
			return 'K'
		}
		return 'k'
	case 38: // L
		if upper {
			return 'L'
		}
		return 'l'
	case 50: // M
		if upper {
			return 'M'
		}
		return 'm'
	case 49: // N
		if upper {
			return 'N'
		}
		return 'n'
	case 24: // O
		if upper {
			return 'O'
		}
		return 'o'
	case 25: // P
		if upper {
			return 'P'
		}
		return 'p'
	case 16: // Q
		if upper {
			return 'Q'
		}
		return 'q'
	case 19: // R
		if upper {
			return 'R'
		}
		return 'r'
	case 31: // S
		if upper {
			return 'S'
		}
		return 's'
	case 20: // T
		if upper {
			return 'T'
		}
		return 't'
	case 22: // U
		if upper {
			return 'U'
		}
		return 'u'
	case 47: // V
		if upper {
			return 'V'
		}
		return 'v'
	case 17: // W
		if upper {
			return 'W'
		}
		return 'w'
	case 45: // X
		if upper {
			return 'X'
		}
		return 'x'
	case 21: // Y
		if upper {
			return 'Y'
		}
		return 'y'
	case 44: // Z
		if upper {
			return 'Z'
		}
		return 'z'
	}

	// Numbers and symbols: shift changes the character
	switch keycode {
	case 2: // 1
		if shift {
			return '!'
		}
		return '1'
	case 3: // 2
		if shift {
			return '@'
		}
		return '2'
	case 4: // 3
		if shift {
			return '#'
		}
		return '3'
	case 5: // 4
		if shift {
			return '$'
		}
		return '4'
	case 6: // 5
		if shift {
			return '%'
		}
		return '5'
	case 7: // 6
		if shift {
			return '^'
		}
		return '6'
	case 8: // 7
		if shift {
			return '&'
		}
		return '7'
	case 9: // 8
		if shift {
			return '*'
		}
		return '8'
	case 10: // 9
		if shift {
			return '('
		}
		return '9'
	case 11: // 0
		if shift {
			return ')'
		}
		return '0'

	// Punctuation
	case 12: // Minus
		if shift {
			return '_'
		}
		return '-'
	case 13: // Equal
		if shift {
			return '+'
		}
		return '='
	case 26: // Left bracket
		if shift {
			return '{'
		}
		return '['
	case 27: // Right bracket
		if shift {
			return '}'
		}
		return ']'
	case 43: // Backslash
		if shift {
			return '|'
		}
		return '\\'
	case 39: // Semicolon
		if shift {
			return ':'
		}
		return ';'
	case 40: // Apostrophe
		if shift {
			return '"'
		}
		return '\''
	case 41: // Grave
		if shift {
			return '~'
		}
		return '`'
	case 51: // Comma
		if shift {
			return '<'
		}
		return ','
	case 52: // Period
		if shift {
			return '>'
		}
		return '.'
	case 53: // Slash
		if shift {
			return '?'
		}
		return '/'
	case 57: // Space
		return ' '

	// Numpad (when NumLock is on, these produce digits)
	case 71: // KP7
		return '7'
	case 72: // KP8
		return '8'
	case 73: // KP9
		return '9'
	case 75: // KP4
		return '4'
	case 76: // KP5
		return '5'
	case 77: // KP6
		return '6'
	case 79: // KP1
		return '1'
	case 80: // KP2
		return '2'
	case 81: // KP3
		return '3'
	case 82: // KP0
		return '0'
	case 83: // KP Decimal
		return '.'
	case 98: // KP Slash
		return '/'
	case 55: // KP Asterisk
		return '*'
	case 74: // KP Minus
		return '-'
	case 78: // KP Plus
		return '+'
	}

	return 0
}

// PollEvents processes pending Wayland events using the event queue pattern.
// Same architecture as X11 and Windows platforms: callbacks queue events,
// PollEvents dequeues one at a time.
func (p *waylandPlatform) PollEvents() Event {
	w := p.primary

	// First, drain queued events (from previous dispatch).
	w.eventMu.Lock()
	if len(w.events) > 0 {
		event := w.events[0]
		w.events = w.events[1:]
		w.eventMu.Unlock()
		return event
	}
	w.eventMu.Unlock()

	// Dispatch all pending events on the C display (single connection).
	// Callbacks will queue events via queueEvent().
	if p.libwl != nil {
		// Read from socket + dispatch default queue (xdg, pointer, keyboard, touch)
		if err := p.libwl.DispatchDefaultQueue(); err != nil {
			logger().Error("wayland dispatch error — closing window", "error", err)
			w.eventMu.Lock()
			w.shouldClose = true
			w.eventMu.Unlock()
			w.queueEvent(Event{Type: EventClose})
		}

		// Dispatch CSD events (separate queue, read by DispatchDefaultQueue above)
		if p.libwl.CSDActive() {
			if err := p.libwl.DispatchCSDEvents(); err != nil {
				logger().Error("CSD dispatch error", "error", err)
			}
		}
	}

	// Return first queued event, or EventNone if empty.
	w.eventMu.Lock()
	defer w.eventMu.Unlock()
	if len(w.events) > 0 {
		event := w.events[0]
		w.events = w.events[1:]
		return event
	}
	return Event{Type: EventNone}
}

// ClipboardRead reads text from the system clipboard.
// TODO(PLAT-008): Implement using wl_data_device and wl_data_offer.
func (p *waylandPlatform) ClipboardRead() (string, error) { return "", nil }

// ClipboardWrite writes text to the system clipboard.
// TODO(PLAT-008): Implement using wl_data_device and wl_data_source.
func (p *waylandPlatform) ClipboardWrite(string) error { return nil }

// DarkMode returns true if the system dark mode is active.
func (p *waylandPlatform) DarkMode() bool { return detectDarkMode() }

// ReduceMotion returns true if the user prefers reduced animation.
func (p *waylandPlatform) ReduceMotion() bool { return detectReduceMotion() }

// HighContrast returns true if high contrast mode is active.
func (p *waylandPlatform) HighContrast() bool { return detectHighContrast() }

// FontScale returns font size preference multiplier.
func (p *waylandPlatform) FontScale() float32 { return detectFontScale() }

// Destroy closes the window and releases resources.
func (p *waylandPlatform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close wakeup pipe (process-level)
	if p.wakePipe[0] != 0 {
		_ = unix.Close(p.wakePipe[0])
		_ = unix.Close(p.wakePipe[1])
		p.wakePipe = [2]int{}
	}

	// Close C libwayland connection (owns all Wayland objects)
	if p.libwl != nil {
		p.libwl.Close()
		p.libwl = nil
	}

	// Close Pure Go display (used only for registry discovery during init)
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
	w := p.primary
	w.callbackMu.Lock()
	w.pointerCallback = fn
	w.callbackMu.Unlock()
}

// SetScrollCallback registers a callback for scroll events.
func (p *waylandPlatform) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	w := p.primary
	w.callbackMu.Lock()
	w.scrollCallback = fn
	w.callbackMu.Unlock()
}

// SetKeyCallback registers a callback for keyboard events.
func (p *waylandPlatform) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	w := p.primary
	w.callbackMu.Lock()
	w.keyboardCallback = fn
	w.callbackMu.Unlock()
}

// SetCharCallback registers a callback for Unicode character input.
// TODO: Implement via libxkbcommon xkb_state_key_get_utf8 for full Unicode support.
func (p *waylandPlatform) SetCharCallback(fn func(rune)) {
	w := p.primary
	w.callbackMu.Lock()
	w.charCallback = fn
	w.callbackMu.Unlock()
}

// SetModalFrameCallback is a no-op on Wayland.
// Wayland uses async configure events — resize is never blocking.
func (p *waylandPlatform) SetModalFrameCallback(_ func()) {}

// WaitEvents blocks until at least one OS event is available.
// Uses unix.Poll on the C display fd and a wakeup pipe to block with 0% CPU.
func (p *waylandPlatform) WaitEvents() {
	if p.libwl == nil {
		return
	}
	dispFd := p.libwl.GetDisplayFD()
	if dispFd < 0 {
		return
	}

	fds := []unix.PollFd{
		{Fd: int32(dispFd), Events: unix.POLLIN | unix.POLLERR},
		{Fd: int32(p.wakePipe[0]), Events: unix.POLLIN},
	}
	// Block indefinitely until an event arrives or WakeUp is called.
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

// detectEnvScaleFactor reads scale factor from environment variables.
// Checks GDK_SCALE (GNOME/GTK) and QT_SCALE_FACTOR (KDE/Qt).
// Returns 0 if no env var is set.
func detectEnvScaleFactor() float64 {
	// GDK_SCALE is integer-only (GNOME/GTK)
	if s := os.Getenv("GDK_SCALE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return float64(v)
		}
	}

	// QT_SCALE_FACTOR supports fractional values (KDE/Qt)
	if s := os.Getenv("QT_SCALE_FACTOR"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
			return v
		}
	}

	return 0
}

// ScaleFactor returns the DPI scale factor.
// Falls back to environment variables (GDK_SCALE, QT_SCALE_FACTOR) or 1.0.
// TODO: Add wl_output scale tracking on C display for proper HiDPI support.
func (p *waylandPlatform) ScaleFactor() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.envScaleFactor > 0 {
		return p.envScaleFactor
	}
	return 1.0
}

// SetCursor changes the mouse cursor shape.
// TODO(PLAT-008): Implement using wp_cursor_shape_manager_v1 or xcursor theme loading.
// wp_cursor_shape_manager_v1 is the modern approach (Wayland protocol extension).
// Fallback: load xcursor theme files from $XCURSOR_PATH, render to wl_buffer,
// attach via wl_pointer.set_cursor. Both approaches are significant effort.
func (p *waylandPlatform) SetCursor(int) {}

// SetCursorMode sets cursor confinement/lock mode on Wayland.
// 0=normal, 1=locked (hidden + pointer lock + relative deltas), 2=confined (visible + confined to surface).
// Uses zwp_pointer_constraints_v1 for lock/confine and zwp_relative_pointer_v1 for relative motion.
func (p *waylandPlatform) SetCursorMode(mode int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary
	w.pointerMu.Lock()
	currentMode := w.cursorMode
	w.pointerMu.Unlock()

	if mode == currentMode {
		return
	}

	if p.libwl == nil {
		return
	}

	surface := p.libwl.Surface()
	pointer := p.libwl.InputPointer()
	if surface == 0 || pointer == 0 {
		logger().Warn("wayland: SetCursorMode requires surface and pointer", "mode", mode)
		return
	}

	// Release any existing constraints before applying new mode
	p.releasePointerConstraints()

	switch mode {
	case 1: // Locked — hide cursor, lock pointer, receive relative deltas
		if !p.libwl.HasPointerConstraints() {
			logger().Warn("wayland: pointer constraints not supported by compositor, cannot lock")
			return
		}

		// Lock pointer to surface (persistent lifetime=1 for auto re-lock on focus)
		if err := p.libwl.LockPointer(surface, pointer, 1); err != nil {
			logger().Warn("wayland: lock_pointer failed", "err", err)
			return
		}

		// Set up relative pointer for motion deltas (used instead of absolute coords)
		if p.libwl.HasRelativePointerManager() {
			if err := p.libwl.GetRelativePointer(pointer); err != nil {
				logger().Warn("wayland: get_relative_pointer failed", "err", err)
			}
		}

		// Hide cursor: set_cursor with NULL surface
		p.libwl.HideCursor(p.libwl.PointerEnterSerial())

		w.pointerMu.Lock()
		w.cursorMode = 1
		w.pointerMu.Unlock()

	case 2: // Confined — cursor visible but confined to surface bounds
		if !p.libwl.HasPointerConstraints() {
			logger().Warn("wayland: pointer constraints not supported by compositor, cannot confine")
			return
		}

		// Confine pointer to surface (persistent lifetime=1)
		if err := p.libwl.ConfinePointer(surface, pointer, 1); err != nil {
			logger().Warn("wayland: confine_pointer failed", "err", err)
			return
		}

		w.pointerMu.Lock()
		w.cursorMode = 2
		w.pointerMu.Unlock()

	default: // Normal (0) — release all constraints, show cursor
		// Constraints already released by releasePointerConstraints above.
		// Cursor restoration happens automatically when the compositor processes
		// the constraint destroy and the pointer re-enters the surface.
		w.pointerMu.Lock()
		w.cursorMode = 0
		w.pointerMu.Unlock()
	}

	// Flush to send all protocol requests immediately
	if err := p.libwl.Flush(); err != nil {
		logger().Warn("wayland: flush failed after SetCursorMode", "err", err)
	}
}

// releasePointerConstraints destroys all active pointer constraints and relative pointer.
// Must be called with p.mu held.
func (p *waylandPlatform) releasePointerConstraints() {
	if p.libwl == nil {
		return
	}
	p.libwl.DestroyLockedPointer()
	p.libwl.DestroyConfinedPointer()
	p.libwl.DestroyRelativePointer()
}

// CursorMode returns the current cursor mode.
func (p *waylandPlatform) CursorMode() int {
	w := p.primary
	w.pointerMu.RLock()
	defer w.pointerMu.RUnlock()
	return w.cursorMode
}

// Frameless window support — waylandPlatform

func (p *waylandPlatform) SetFrameless(frameless bool) {
	w := p.primary
	w.callbackMu.Lock()
	w.frameless = frameless
	w.callbackMu.Unlock()
	// SSD/CSD mode switching on C display is not yet implemented.
	// The decoration mode is set during Init based on config.Frameless.
}

func (p *waylandPlatform) IsFrameless() bool {
	w := p.primary
	w.callbackMu.RLock()
	defer w.callbackMu.RUnlock()
	return w.frameless
}

func (p *waylandPlatform) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	w := p.primary
	w.callbackMu.Lock()
	defer w.callbackMu.Unlock()
	w.hitTestCallback = fn
}

func (p *waylandPlatform) Minimize() {
	if p.libwl != nil && p.libwl.Toplevel() != 0 {
		p.libwl.MarshalVoidOnToplevel(13) // xdg_toplevel.set_minimized = opcode 13
	}
}

func (p *waylandPlatform) Maximize() {
	w := p.primary
	w.eventMu.Lock()
	maximized := w.maximized
	w.eventMu.Unlock()

	if p.libwl != nil && p.libwl.Toplevel() != 0 {
		if maximized {
			p.libwl.MarshalVoidOnToplevel(10) // unset_maximized = opcode 10
		} else {
			p.libwl.MarshalVoidOnToplevel(9) // set_maximized = opcode 9
		}
	}
}

func (p *waylandPlatform) IsMaximized() bool {
	w := p.primary
	w.eventMu.Lock()
	defer w.eventMu.Unlock()
	return w.maximized
}

func (p *waylandPlatform) CloseWindow() {
	w := p.primary
	w.eventMu.Lock()
	w.shouldClose = true
	w.eventMu.Unlock()
	w.queueEvent(Event{Type: EventClose})
}
