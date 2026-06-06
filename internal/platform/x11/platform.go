//go:build linux

package x11

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	xkbcommon "github.com/gogpu/gogpu/internal/platform/xkb"
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
	Frameless  bool
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventTypeNone EventType = iota
	EventTypeClose
	EventTypeResize
	EventTypeFocus
	EventTypeKeyDown
	EventTypeKeyUp
	EventTypeChar
	EventTypePointerDown
	EventTypePointerUp
	EventTypePointerMove
	EventTypePointerEnter
	EventTypePointerLeave
	EventTypeScroll
)

// PlatformEvent represents a platform event.
// This mirrors platform.Event to avoid import cycles.
type PlatformEvent struct {
	Type    EventType
	Width   int
	Height  int
	Focused bool // for focus events: true = gained, false = lost

	// Keyboard (EventTypeKeyDown, EventTypeKeyUp)
	Key  gpucontext.Key
	Mods gpucontext.Modifiers

	// Character input (EventTypeChar)
	Char rune

	// Pointer (EventTypePointer*)
	Pointer gpucontext.PointerEvent

	// Scroll (EventTypeScroll)
	Scroll gpucontext.ScrollEvent
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

// x11Window holds all per-window state.
// In Phase 1 (single-window), there is exactly one instance referenced as primary.
// Phase 2 will add multi-window support via the window registry.
type x11Window struct {
	// X11 window ID
	window ResourceID

	// Window state (guarded by eventMu for thread-safe access from multiple goroutines).
	width       int
	height      int
	shouldClose bool
	configured  bool
	eventMu     sync.Mutex // guards window state fields (width, height, shouldClose, configured)

	// Event queue — ring buffer (ADR-031: fixed capacity, zero allocs, drops oldest).
	// The ring buffer has its own internal mutex — no external lock needed for Push/Pop.
	events *eventqueue.Queue[PlatformEvent]

	// Mouse state tracking
	mouseX        float64
	mouseY        float64
	buttons       gpucontext.Buttons
	modifiers     gpucontext.Modifiers
	mouseInWindow bool

	// Touch tracking: first active touchID is primary
	activeTouches map[uint32]bool
	primaryTouch  uint32
	hasPrimary    bool

	// Frameless window state
	frameless       bool
	fullscreen      bool
	hitTestCallback func(x, y float64) gpucontext.HitTestResult

	callbackMu sync.RWMutex

	// Graphics context for BlitPixels (software backend presentation).
	// Lazily created on first BlitPixels call. Bound to this window's drawable.
	blitGC ResourceID

	// Timestamp reference for event timing
	startTime time.Time

	// Cursor mode state (0=normal, 1=locked, 2=confined)
	cursorMode    int
	savedMouseX   float64 // saved position before locking
	savedMouseY   float64
	cursorCenterX int16 // window center for warp-back in locked mode
	cursorCenterY int16
	cursorGrabbed bool // whether XGrabPointer is active
}

// Platform implements X11 windowing support.
// Holds process-level state shared across all windows and a registry
// of x11Window instances keyed by X11 ResourceID.
type Platform struct {
	mu sync.Mutex

	// X11 connection (pure Go wire protocol for events) — process-level
	conn *Connection

	// Xlib Display* for Vulkan surface creation — process-level
	xlib *xlibHandle

	// Standard atoms — process-level
	atoms *StandardAtoms

	// XInput2 extension (nil if unavailable) — process-level
	xi *XIExtension

	// XKB extension (nil if unavailable) — process-level
	xkb *XkbExtension

	// Current keyboard group from XKB (0-3), 0 = first layout
	xkbGroup int

	// Keyboard mapping — process-level
	keymap *KeyboardMapping

	// Shared xkbcommon handle for proper text input (AltGr, dead keys, etc.).
	// Uses xkb_keymap_new_from_names(NULL) for system default keymap.
	// Nil if libxkbcommon is not available (falls back to manual KeycodeToKeysymGroup).
	xkbState *xkbcommon.Handle

	// XWayland detection: true when running under XWayland (Wayland compositor).
	// _XKB_RULES_NAMES is unreliable under XWayland (freedesktop#612).
	isXWayland bool

	// DPI scale factor (from Xft.dpi or screen physical size) — process-level
	scaleFactor float64

	// Cursor resources — process-level (shared across windows)
	cursorFontID  ResourceID         // "cursor" font ID (0 = not opened yet)
	cursorCache   map[int]ResourceID // cursor shape → X11 cursor resource ID
	blankCursorID ResourceID         // 1x1 transparent cursor for locked mode

	// Clipboard state (ICCCM selection protocol)
	clipboardMu    sync.Mutex // guards clipboard fields below
	clipboardText  string     // locally stored clipboard content
	ownsClipboard  bool       // true if we are the CLIPBOARD selection owner
	clipboardReady bool       // signaled by SelectionNotify handler during read

	// Window registry keyed by X11 ResourceID for event routing.
	windowMu sync.RWMutex
	windows  map[ResourceID]*x11Window

	// Primary window for backward-compatible single-window API.
	primary *x11Window
}

// NewPlatform creates a new X11 platform instance.
func NewPlatform() *Platform {
	return &Platform{
		windows: make(map[ResourceID]*x11Window),
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

	// Detect DPI scale factor BEFORE window creation using Xft.dpi from RESOURCE_MANAGER.
	// This only needs p.conn (no Xlib). The Xlib-based fallback runs later as refinement.
	p.scaleFactor = p.queryScaleFactorFromXftDPI()

	// Scale config dimensions from logical (DIP) to physical pixels for X11 window creation.
	// X11 always works in physical pixels — the window manager does not perform any scaling.
	physWidth := config.Width
	physHeight := config.Height
	if p.scaleFactor > 1.0 {
		physWidth = int(math.Round(float64(config.Width) * p.scaleFactor))
		physHeight = int(math.Round(float64(config.Height) * p.scaleFactor))
	}

	// Create window with physical pixel dimensions
	windowConfig := WindowConfig{
		Title:      config.Title,
		Width:      uint16(physWidth),
		Height:     uint16(physHeight),
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

	// Create per-window state.
	// Store physical pixel dimensions (what the X server sees).
	w := &x11Window{
		window:        window,
		width:         physWidth,
		height:        physHeight,
		startTime:     time.Now(),
		activeTouches: make(map[uint32]bool),
		frameless:     config.Frameless,
		events:        eventqueue.New[PlatformEvent](eventqueue.DefaultCapacity),
	}

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

	// Handle frameless windows via Motif hints
	if config.Frameless {
		_ = conn.SetWindowBorderless(window, atoms)
	}

	// Handle non-resizable windows via Motif hints
	if !config.Resizable && !config.Frameless {
		hints := &MotifWMHints{
			Flags:       MotifHintsDecorations | MotifHintsFunctions,
			Decorations: MotifDecorBorder | MotifDecorTitle | MotifDecorMenu | MotifDecorMinimize,
			Functions:   1 | 2 | 8, // Move | Minimize | Close (no Resize or Maximize)
		}
		// Non-fatal, some WMs don't support Motif hints
		_ = conn.SetMotifWMHints(window, hints, atoms)
	}

	// Get keyboard mapping (non-fatal - keyboard input may not work correctly without it)
	keymap, _ := conn.GetKeyboardMapping()
	p.keymap = keymap

	// Initialize XKB for keyboard layout/group support (non-fatal).
	// When available, XKB tracks the active keyboard group (e.g., EN/RU)
	// and delivers XkbStateNotify events on layout switches.
	xkb, xkbErr := conn.InitXkb()
	if xkbErr != nil {
		logger().Info("XKB not available, using default keyboard layout", "error", xkbErr)
	} else {
		p.xkb = xkb
		p.xkbGroup = xkb.Group
		logger().Info("XKB enabled", "version", fmt.Sprintf("%d.%d", xkb.MajorVer, xkb.MinorVer), "group", xkb.Group)
	}

	// Detect XWayland before initXkbcommon — _XKB_RULES_NAMES is unreliable
	// under XWayland (freedesktop#612). SDL3 uses QueryExtension("XWAYLAND").
	p.isXWayland = p.detectXWayland()
	if p.isXWayland {
		logger().Info("XWayland detected — _XKB_RULES_NAMES may be unreliable")
	}

	// Load libxkbcommon for proper text input (AltGr, dead keys, multi-layout).
	// First try RMLVO from _XKB_RULES_NAMES, then system defaults.
	// Non-fatal: falls back to manual KeycodeToKeysymGroup (no AltGr support).
	p.initXkbcommon()

	// Initialize XInput2 for touch support (non-fatal)
	xi, err := conn.InitXInput2()
	if err != nil {
		logger().Info("XInput2 touch not available", "error", err)
	} else {
		p.xi = xi
		if xiErr := conn.XISelectTouchEvents(xi, window); xiErr != nil {
			logger().Warn("failed to select touch events", "error", xiErr)
			p.xi = nil
		} else {
			logger().Info("XInput2 touch enabled", "version", fmt.Sprintf("%d.%d", xi.MajorVer, xi.MinorVer))
		}
	}

	// Set fullscreen if requested (non-fatal, will fail if WM doesn't support EWMH)
	if config.Fullscreen {
		_ = conn.SetFullscreen(window, true, atoms)
	}

	// Mark window as configured
	w.configured = true

	// Register window in the map and set as primary
	p.windowMu.Lock()
	p.windows[window] = w
	p.windowMu.Unlock()
	p.primary = w

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

	// Refine DPI scale factor using Xlib screen info (fallback for systems without Xft.dpi).
	// queryScaleFactor re-checks Xft.dpi (fast, same result) then tries screen physical dimensions.
	refinedScale := p.queryScaleFactor()
	if refinedScale != p.scaleFactor && refinedScale != 1.0 {
		p.scaleFactor = refinedScale
	}
	if p.scaleFactor != 1.0 {
		logger().Info("x11 DPI scale", "factor", p.scaleFactor)
	}

	// Enable detectable auto-repeat to suppress spurious KeyRelease events
	// during key repeat. Without this, X11 sends KeyRelease+KeyPress pairs
	// for each repeat, making it impossible to distinguish real release from repeat.
	// ADR-033: Key Repeat.
	if xlib != nil {
		p.setDetectableAutoRepeat()
	}

	if xlib != nil {
		logger().Info("x11 init complete", "window", fmt.Sprintf("%#x", w.window), "display", fmt.Sprintf("%#x", xlib.display))
	} else {
		logger().Warn("x11 init without xlib", "window", fmt.Sprintf("%#x", w.window))
	}

	return nil
}

// ScaleFactor returns the DPI scale factor detected during Init.
// Returns 1.0 if DPI information is unavailable.
func (p *Platform) ScaleFactor() float64 {
	if p.scaleFactor <= 0 {
		return 1.0
	}
	return p.scaleFactor
}

// queryScaleFactor determines the DPI scale factor using two methods:
//  1. Xft.dpi from RESOURCE_MANAGER property on root window (most reliable, matches GLFW/Qt/GTK)
//  2. Screen physical dimensions as fallback (pixels / mm * 25.4 / 96)
//
// Returns 1.0 if neither method yields a usable result.
func (p *Platform) queryScaleFactor() float64 {
	if p.conn == nil {
		return 1.0
	}

	// Method 1: Read Xft.dpi from RESOURCE_MANAGER property on root window.
	// This is the standard way desktop environments communicate DPI settings.
	// KDE, GNOME, Xfce all set this. GLFW uses the same approach.
	rootWindow := p.conn.RootWindow()
	if rootWindow != 0 {
		// RESOURCE_MANAGER is a predefined atom (23). Request type AnyPropertyType (0).
		data, _, _, err := p.conn.GetProperty(rootWindow, AtomResourceManager, Atom(0), 0, 8192, false)
		if err == nil && len(data) > 0 {
			if dpi := parseXftDPI(string(data)); dpi > 0 {
				scale := dpi / 96.0
				// Clamp to reasonable range [0.5, 8.0]
				if scale >= 0.5 && scale <= 8.0 {
					return scale
				}
			}
		}
	}

	// Method 2: Compute DPI from screen physical dimensions.
	screen := p.conn.DefaultScreen()
	if screen != nil && screen.WidthInMillimeters > 0 {
		dpi := float64(screen.WidthInPixels) * 25.4 / float64(screen.WidthInMillimeters)
		scale := dpi / 96.0
		// Only use if significantly different from 1.0 and within reasonable range.
		// Screen physical sizes reported by X are often inaccurate (especially on VMs),
		// so we use a wider dead zone than for Xft.dpi.
		if scale >= 0.5 && scale <= 8.0 && math.Abs(scale-1.0) > 0.1 {
			return math.Round(scale*4) / 4 // Round to nearest 0.25
		}
	}

	return 1.0
}

// queryScaleFactorFromXftDPI detects scale factor using ONLY Xft.dpi from
// the RESOURCE_MANAGER property on the root window. This method requires only
// p.conn (no Xlib display), so it can be called BEFORE window creation.
// Returns 1.0 if Xft.dpi is not set or cannot be parsed.
func (p *Platform) queryScaleFactorFromXftDPI() float64 {
	if p.conn == nil {
		return 1.0
	}

	rootWindow := p.conn.RootWindow()
	if rootWindow == 0 {
		return 1.0
	}

	// RESOURCE_MANAGER is a predefined atom (23). Request type AnyPropertyType (0).
	data, _, _, err := p.conn.GetProperty(rootWindow, AtomResourceManager, Atom(0), 0, 8192, false)
	if err == nil && len(data) > 0 {
		if dpi := parseXftDPI(string(data)); dpi > 0 {
			scale := dpi / 96.0
			// Clamp to reasonable range [0.5, 8.0]
			if scale >= 0.5 && scale <= 8.0 {
				return scale
			}
		}
	}

	// Also check GDK_SCALE and QT_SCALE_FACTOR environment variables.
	// These are set by GNOME/KDE and available before any X resource reads.
	if s := os.Getenv("GDK_SCALE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return float64(v)
		}
	}
	if s := os.Getenv("QT_SCALE_FACTOR"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 && v <= 8.0 {
			return v
		}
	}

	return 1.0
}

// parseXftDPI parses the Xft.dpi value from an X RESOURCE_MANAGER string.
// The string contains lines like "Xft.dpi:\t96" or "Xft.dpi: 144".
// Returns 0 if Xft.dpi is not found or cannot be parsed.
func parseXftDPI(resources string) float64 {
	for _, line := range strings.Split(resources, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Xft.dpi:") {
			continue
		}
		value := strings.TrimSpace(line[len("Xft.dpi:"):])
		if dpi, err := strconv.ParseFloat(value, 64); err == nil && dpi > 0 {
			return dpi
		}
	}
	return 0
}

// SubpixelLayout returns the display's subpixel arrangement by reading
// Xft.rgba from the X RESOURCE_MANAGER property on the root window.
// Returns SubpixelNone if the X connection is not available, Xft.rgba
// is not set, or the scale factor indicates HiDPI (>= 2.0).
func (p *Platform) SubpixelLayout() gpucontext.SubpixelLayout {
	// HiDPI displays use grayscale AA (subpixels too small to matter).
	if p.scaleFactor >= 2.0 {
		return gpucontext.SubpixelNone
	}

	if p.conn == nil {
		return gpucontext.SubpixelNone
	}

	rootWindow := p.conn.RootWindow()
	if rootWindow == 0 {
		return gpucontext.SubpixelRGB
	}

	data, _, _, err := p.conn.GetProperty(rootWindow, AtomResourceManager, Atom(0), 0, 8192, false)
	if err != nil || len(data) == 0 {
		return gpucontext.SubpixelRGB
	}

	return parseXftRGBA(string(data))
}

// parseXftRGBA parses the Xft.rgba value from an X RESOURCE_MANAGER string.
// The string contains lines like "Xft.rgba:\trgb" or "Xft.rgba: bgr".
// Valid values: "rgb", "bgr", "vrgb", "vbgr", "none".
// Returns SubpixelRGB if Xft.rgba is not found (most common LCD default).
func parseXftRGBA(resources string) gpucontext.SubpixelLayout {
	for _, line := range strings.Split(resources, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Xft.rgba:") {
			continue
		}
		value := strings.TrimSpace(line[len("Xft.rgba:"):])
		value = strings.ToLower(value)
		switch value {
		case "rgb":
			return gpucontext.SubpixelRGB
		case "bgr":
			return gpucontext.SubpixelBGR
		case "vrgb":
			return gpucontext.SubpixelVRGB
		case "vbgr":
			return gpucontext.SubpixelVBGR
		case "none":
			return gpucontext.SubpixelNone
		default:
			return gpucontext.SubpixelRGB
		}
	}
	// Xft.rgba not set — default to RGB (most common LCD layout).
	return gpucontext.SubpixelRGB
}

// initXkbcommon loads libxkbcommon and creates a keymap.
// First tries to read the X server's actual RMLVO configuration from
// _XKB_RULES_NAMES root window property (BUG-INPUT-004: multi-layout support).
// Falls back to xkb_keymap_new_from_names(NULL) which only loads "us".
// Under XWayland, skips _XKB_RULES_NAMES (unreliable per freedesktop#612).
// Non-fatal: if xkbcommon is unavailable, keyboard falls back to manual keysym lookup.
func (p *Platform) initXkbcommon() {
	xkbHandle, xkbcommonErr := xkbcommon.New()
	if xkbcommonErr != nil {
		logger().Info("xkbcommon not available for X11, AltGr may not work", "err", xkbcommonErr)
		return
	}

	// Try to load keymap from X server's actual configuration.
	// BUG-INPUT-005: Skip _XKB_RULES_NAMES under XWayland (unreliable per freedesktop#612).
	loaded := false
	if !p.isXWayland {
		// Native X11: read RMLVO from root window property.
		rules, model, layout, variant, options := p.readXKBRulesNames()
		if layout != "" {
			if err := xkbHandle.SetKeymapFromRMLVO(rules, model, layout, variant, options); err != nil {
				logger().Warn("xkbcommon: RMLVO keymap failed, trying defaults", "err", err, "layout", layout)
			} else {
				loaded = true
				logger().Info("xkbcommon: keymap loaded from _XKB_RULES_NAMES", "layout", layout)
			}
		}
	} else {
		logger().Debug("xkbcommon: skipping _XKB_RULES_NAMES under XWayland (unreliable)")
	}

	// Fallback to system defaults if RMLVO not available or under XWayland
	if !loaded {
		if err := xkbHandle.SetKeymapFromNames(); err != nil {
			logger().Warn("xkbcommon: failed to load system keymap", "err", err)
			xkbHandle.Close()
			return
		}
		logger().Info("xkbcommon: keymap loaded from system defaults")
	}

	p.xkbState = xkbHandle

	// BUG-INPUT-005: Sync xkbcommon state with X server's current state (winit pattern).
	// xkb_state_new starts at group=0 with zero modifiers. The X server may already
	// be at a different group (e.g., Russian) or have modifiers like CapsLock active.
	if p.xkb != nil && p.xkbState != nil && p.xkbState.Ready() {
		if fullState, err := p.conn.xkbGetFullState(p.xkb.MajorOpcode); err == nil {
			p.xkbState.UpdateMask(
				fullState.BaseMods, fullState.LatchedMods, fullState.LockedMods,
				0, 0, uint32(fullState.Group),
			)
			p.mu.Lock()
			p.xkbGroup = fullState.Group
			p.mu.Unlock()
			logger().Debug("xkbcommon: initial state synced with X server", "group", fullState.Group)
		}
	}
}

// readXKBRulesNames reads the _XKB_RULES_NAMES property from the root window.
// Returns RMLVO (rules, model, layout, variant, options).
// The property contains 5 null-terminated strings concatenated:
// "evdev\0pc105\0us,ru,ru\0,,phonetic\0grp:alt_shift_toggle\0"
// If property is missing or empty, all returned strings are empty.
func (p *Platform) readXKBRulesNames() (string, string, string, string, string) {
	if p.conn == nil {
		return "", "", "", "", ""
	}

	// Intern the _XKB_RULES_NAMES atom
	atom, err := p.conn.InternAtom("_XKB_RULES_NAMES", true)
	if err != nil || atom == AtomNone {
		return "", "", "", "", ""
	}

	// Get root window
	root := p.conn.RootWindow()
	if root == 0 {
		return "", "", "", "", ""
	}

	// Read property (type AnyPropertyType=0, up to 256 longs = 1024 bytes)
	data, _, _, err := p.conn.GetProperty(root, atom, Atom(0), 0, 256, false)
	if err != nil || len(data) == 0 {
		return "", "", "", "", ""
	}

	// Parse 5 null-terminated strings
	parts := splitNullTerminated(data, 5)
	var rules, model, layout, variant, options string
	if len(parts) >= 1 {
		rules = parts[0]
	}
	if len(parts) >= 2 {
		model = parts[1]
	}
	if len(parts) >= 3 {
		layout = parts[2]
	}
	if len(parts) >= 4 {
		variant = parts[3]
	}
	if len(parts) >= 5 {
		options = parts[4]
	}

	return rules, model, layout, variant, options
}

// detectXWayland checks if we are running under XWayland by querying the
// XWAYLAND X11 extension. SDL3 uses the same pattern.
// Under XWayland, _XKB_RULES_NAMES is unreliable (freedesktop#612).
func (p *Platform) detectXWayland() bool {
	if p.conn == nil {
		return false
	}
	ext, err := p.conn.QueryExtension("XWAYLAND")
	if err != nil {
		return false
	}
	return ext.Present
}

// splitNullTerminated splits a byte slice by null bytes into up to maxParts strings.
func splitNullTerminated(data []byte, maxParts int) []string {
	var parts []string
	start := 0
	for i, b := range data {
		if b == 0 {
			parts = append(parts, string(data[start:i]))
			start = i + 1
			if len(parts) >= maxParts {
				break
			}
		}
	}
	return parts
}

// PollEvents processes pending X11 events.
func (p *Platform) PollEvents() PlatformEvent {
	w := p.primary

	// First, drain queued events (from previous X11 reads).
	if event, ok := w.dequeueEvent(); ok {
		return event
	}

	// Read and process all available X11 events into the queue.
	for {
		event, err := p.conn.PollEvent()
		if err != nil {
			w.eventMu.Lock()
			w.shouldClose = true
			w.eventMu.Unlock()
			w.queueEvent(PlatformEvent{Type: EventTypeClose})
			break
		}

		if event == nil {
			break // No more X11 data available
		}

		if platformEvent := p.handleEvent(event); platformEvent.Type != EventTypeNone {
			w.queueEvent(platformEvent)
		}
	}

	// Return first queued event, or EventNone if empty.
	if event, ok := w.dequeueEvent(); ok {
		return event
	}
	return PlatformEvent{Type: EventTypeNone}
}

// dequeueEvent removes and returns the first event from the queue.
// Returns false if the queue is empty.
func (w *x11Window) dequeueEvent() (PlatformEvent, bool) {
	return w.events.Pop()
}

// queueEvent pushes a platform event to the window's ring buffer queue.
func (w *x11Window) queueEvent(event PlatformEvent) {
	w.events.Push(event)
}

// QueueEvent is an exported wrapper for queueEvent, used by the platform
// layer's WaitEvents to enqueue events read during idle wait.
func (p *Platform) QueueEvent(event PlatformEvent) {
	p.primary.queueEvent(event)
}

// HandleEvent is an exported wrapper for handleEvent, used by the platform
// layer's WaitEvents to process raw X11 events read during idle wait.
func (p *Platform) HandleEvent(event Event) PlatformEvent {
	return p.handleEvent(event)
}

// handleEvent processes a single X11 event.
// Routes to the appropriate window based on the event's window ID.
func (p *Platform) handleEvent(event Event) PlatformEvent {
	// For Phase 1 (single-window), all events go to primary.
	// Phase 2 will look up window from event's Window field.
	w := p.primary

	switch e := event.(type) {
	case *ConfigureNotifyEvent:
		if e.Window == w.window {
			newWidth := int(e.Width)
			newHeight := int(e.Height)
			w.eventMu.Lock()
			changed := newWidth != w.width || newHeight != w.height
			if changed {
				w.width = newWidth
				w.height = newHeight
			}
			w.eventMu.Unlock()
			if changed {
				return PlatformEvent{
					Type:   EventTypeResize,
					Width:  newWidth,
					Height: newHeight,
				}
			}
		}

	case *ClientMessageEvent:
		if e.IsDeleteWindow(p.atoms) {
			w.eventMu.Lock()
			w.shouldClose = true
			w.eventMu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

	case *DestroyNotifyEvent:
		if e.Window == w.window {
			w.eventMu.Lock()
			w.shouldClose = true
			w.eventMu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

	case *ExposeEvent:
		// Could trigger redraw, but for now we just ignore
		// The main render loop should handle this

	case *MapNotifyEvent:
		w.eventMu.Lock()
		w.configured = true
		w.eventMu.Unlock()

	case *KeyPressEvent:
		p.handleKeyEvent(w, e.Detail, e.State, true)

	case *KeyReleaseEvent:
		p.handleKeyEvent(w, e.Detail, e.State, false)

	case *MotionNotifyEvent:
		p.handleMotionNotify(w, e)

	case *ButtonPressEvent:
		p.handleButtonPress(w, e)

	case *ButtonReleaseEvent:
		p.handleButtonRelease(w, e)

	case *EnterNotifyEvent:
		p.handleEnterNotify(w, e)

	case *LeaveNotifyEvent:
		p.handleLeaveNotify(w, e)

	case *FocusInEvent:
		// Re-grab pointer if cursor mode requires it
		p.handleFocusIn(w)

		// Emit focus event only for meaningful focus changes.
		// Filter out NotifyPointer (5) and NotifyPointerRoot (6) which are noise,
		// and only accept NotifyNormal mode (0) to avoid spurious events from grabs.
		if e.Detail != 5 && e.Detail != 6 && e.Mode == 0 {
			return PlatformEvent{Type: EventTypeFocus, Focused: true}
		}

	case *FocusOutEvent:
		// Release pointer grab on focus loss
		p.handleFocusOut(w)

		// Emit focus event with same filtering as FocusIn.
		if e.Detail != 5 && e.Detail != 6 && e.Mode == 0 {
			return PlatformEvent{Type: EventTypeFocus, Focused: false}
		}

	case *SelectionClearEvent:
		p.handleSelectionClear(e)

	case *SelectionRequestEvent:
		p.handleSelectionRequest(e)

	case *GenericEvent:
		p.handleGenericEvent(w, e)

	case *UnknownEvent:
		p.handleUnknownEvent(e)

	case *MappingNotifyEvent:
		p.handleMappingNotify()
	}

	return PlatformEvent{Type: EventTypeNone}
}

// handleMotionNotify processes mouse movement events.
func (p *Platform) handleMotionNotify(w *x11Window, e *MotionNotifyEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	w.eventMu.Lock()
	cursorMode := w.cursorMode
	centerX := float64(w.cursorCenterX)
	centerY := float64(w.cursorCenterY)
	w.mouseX = x
	w.mouseY = y
	w.buttons = extractButtons(e.State)
	w.modifiers = extractModifiers(e.State)
	w.eventMu.Unlock()

	// In locked mode, compute delta from center and warp back
	if cursorMode == 1 {
		deltaX := x - centerX
		deltaY := y - centerY

		// Skip the warp-back event (delta=0)
		if deltaX == 0 && deltaY == 0 {
			return
		}

		// Warp cursor back to center
		_ = p.conn.WarpPointer(0, w.window, 0, 0, 0, 0,
			int16(centerX), int16(centerY))

		// Emit event with relative deltas
		ev := w.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, x, y, e.State)
		ev.DeltaX = deltaX
		ev.DeltaY = deltaY
		w.dispatchPointerEvent(ev)
		return
	}

	ev := w.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, x, y, e.State)
	w.dispatchPointerEvent(ev)
}

// handleButtonPress processes mouse button press events.
func (p *Platform) handleButtonPress(w *x11Window, e *ButtonPressEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	// Scroll buttons (4-7) are emulated as button presses in X11
	if isScrollButton(e.Detail) {
		p.handleScrollButton(w, e.Detail, x, y, e.State)
		return
	}

	// Check hit test for frameless window move/resize (left button only).
	// When the WM takes over, we must not dispatch PointerDown.
	if e.Detail == 1 {
		w.callbackMu.RLock()
		cb := w.hitTestCallback
		frameless := w.frameless
		w.callbackMu.RUnlock()

		if frameless && cb != nil {
			result := cb(x, y)
			if dir, ok := hitTestToMoveResizeDirection(result); ok {
				// Send _NET_WM_MOVERESIZE to the window manager.
				// data: [x_root, y_root, direction, button, source_indication]
				_ = p.conn.SendClientMessage(w.window, p.conn.RootWindow(),
					p.atoms.NetWMMoveresize,
					uint32(e.RootX), uint32(e.RootY), dir, 1, 1)
				return
			}
		}
	}

	// Regular button press
	button := x11ButtonToButton(e.Detail)
	if button == gpucontext.ButtonNone {
		return // Unknown button
	}

	w.eventMu.Lock()
	w.mouseX = x
	w.mouseY = y
	// Update button state - button is now pressed
	switch button {
	case gpucontext.ButtonLeft:
		w.buttons |= gpucontext.ButtonsLeft
	case gpucontext.ButtonMiddle:
		w.buttons |= gpucontext.ButtonsMiddle
	case gpucontext.ButtonRight:
		w.buttons |= gpucontext.ButtonsRight
	case gpucontext.ButtonX1:
		w.buttons |= gpucontext.ButtonsX1
	case gpucontext.ButtonX2:
		w.buttons |= gpucontext.ButtonsX2
	}
	w.modifiers = extractModifiers(e.State)
	w.eventMu.Unlock()

	ev := w.createPointerEvent(gpucontext.PointerDown, button, x, y, e.State)
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
	w.dispatchPointerEvent(ev)
}

// handleButtonRelease processes mouse button release events.
func (p *Platform) handleButtonRelease(w *x11Window, e *ButtonReleaseEvent) {
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

	w.eventMu.Lock()
	w.mouseX = x
	w.mouseY = y
	// Update button state - button is now released
	switch button {
	case gpucontext.ButtonLeft:
		w.buttons &^= gpucontext.ButtonsLeft
	case gpucontext.ButtonMiddle:
		w.buttons &^= gpucontext.ButtonsMiddle
	case gpucontext.ButtonRight:
		w.buttons &^= gpucontext.ButtonsRight
	case gpucontext.ButtonX1:
		w.buttons &^= gpucontext.ButtonsX1
	case gpucontext.ButtonX2:
		w.buttons &^= gpucontext.ButtonsX2
	}
	w.modifiers = extractModifiers(e.State)
	w.eventMu.Unlock()

	ev := w.createPointerEvent(gpucontext.PointerUp, button, x, y, e.State)
	w.dispatchPointerEvent(ev)
}

// handleScrollButton processes X11 scroll button events (buttons 4-7).
func (p *Platform) handleScrollButton(w *x11Window, detail uint8, x, y float64, state uint16) {
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
		Timestamp: w.eventTimestamp(),
	}
	w.dispatchScrollEvent(ev)
}

// handleEnterNotify processes pointer enter events.
func (p *Platform) handleEnterNotify(w *x11Window, e *EnterNotifyEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	w.eventMu.Lock()
	w.mouseX = x
	w.mouseY = y
	w.buttons = extractButtons(e.State)
	w.modifiers = extractModifiers(e.State)
	w.mouseInWindow = true
	w.eventMu.Unlock()

	ev := w.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, x, y, e.State)
	w.dispatchPointerEvent(ev)
}

// handleLeaveNotify processes pointer leave events.
func (p *Platform) handleLeaveNotify(w *x11Window, e *LeaveNotifyEvent) {
	x := float64(e.EventX)
	y := float64(e.EventY)

	w.eventMu.Lock()
	w.mouseX = x
	w.mouseY = y
	w.buttons = extractButtons(e.State)
	w.modifiers = extractModifiers(e.State)
	w.mouseInWindow = false
	w.eventMu.Unlock()

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
		Timestamp:   w.eventTimestamp(),
	}
	w.dispatchPointerEvent(ev)
}

// handleGenericEvent processes X11 GenericEvent (type 35) for extension events.
func (p *Platform) handleGenericEvent(w *x11Window, ge *GenericEvent) {
	if p.xi == nil || ge.Extension != p.xi.MajorOpcode {
		return
	}

	switch ge.EventType {
	case XITouchBegin, XITouchUpdate, XITouchEnd:
		dev, err := p.conn.ParseXIDeviceEvent(ge)
		if err != nil {
			return
		}
		w.handleXITouchEvent(dev)
	}
}

// handleUnknownEvent checks if an unrecognized X11 event is an XKB extension event.
// XKB events arrive with a dynamic event type code (EventBase assigned by the server),
// which parseEvent cannot recognize statically — they land in UnknownEvent.
func (p *Platform) handleUnknownEvent(e *UnknownEvent) {
	if p.xkb == nil || e.Type != p.xkb.EventBase {
		return
	}

	// XKB events share a single event type code (EventBase).
	// The XKB sub-type is at byte 1 of the raw event (Data[0] in UnknownEvent).
	xkbType := e.Data[0]

	switch xkbType {
	case 0, 1: // XkbNewKeyboardNotify (0) or XkbMapNotify (1)
		// BUG-INPUT-005: Keymap changed (keyboard hot-plug or layout reconfiguration).
		// Reload the keymap and re-sync state.
		logger().Debug("XKB keymap changed, reloading", "type", xkbType)
		p.reloadXkbKeymap()
		return

	case XkbStateNotify: // = 2
		// Fall through to state handling below.

	default:
		return
	}

	// XCB wire format for xcb_xkb_state_notify_event_t (32 bytes):
	//   byte 0:    response_type (= EventBase)
	//   byte 1:    xkb_type (= XkbStateNotify = 2)
	//   bytes 2-3: sequence
	//   bytes 4-7: time
	//   byte 8:    deviceID
	//   byte 9:    mods (effective)
	//   byte 10:   baseMods
	//   byte 11:   latchedMods
	//   byte 12:   lockedMods
	//   byte 13:   group (effective)
	//   bytes 14-15: baseGroup (int16 LE)
	//   bytes 16-17: latchedGroup (int16 LE)
	//   bytes 18-19: lockedGroup (int16 LE)
	//
	// UnknownEvent.Data starts at byte 1 (Data[0] = byte 1 of raw event).
	// So: baseMods=Data[9], latchedMods=Data[10], lockedMods=Data[11],
	//     group=Data[12], baseGroup=Data[13:15], etc.
	if len(e.Data) > 18 {
		newGroup := int(e.Data[12])
		baseMods := uint32(e.Data[9])
		latchedMods := uint32(e.Data[10])
		lockedMods := uint32(e.Data[11])

		p.mu.Lock()
		p.xkbGroup = newGroup
		xkbState := p.xkbState
		p.mu.Unlock()

		// Sync xkbcommon state with X server's effective group.
		// Use Wayland pattern: pass effective group (Data[12]) as layoutLocked,
		// zeros for baseGroup/latchedGroup. This avoids two bugs:
		// 1. lockedGroup is uint8 in XCB wire but we were reading 2 bytes (compatState leak)
		// 2. Decomposed groups can produce different effective group in xkbcommon
		//    due to double-wrapping with potentially different keymap rules.
		// The effective group is already computed by the X server — safe to pass directly.
		if xkbState != nil && xkbState.Ready() {
			xkbState.UpdateMask(baseMods, latchedMods, lockedMods,
				0, 0, uint32(newGroup))
		}

		logger().Debug("XKB state changed", "group", newGroup)
	}
}

// handleMappingNotify re-reads XKB full state when keyboard mapping changes.
// Some X servers send MappingNotify instead of XkbStateNotify on layout switch.
// SDL3 uses the same fallback pattern.
// BUG-INPUT-005: Now uses full state sync (modifiers + group) instead of group-only.
func (p *Platform) handleMappingNotify() {
	if p.xkb == nil {
		return
	}
	fullState, err := p.conn.xkbGetFullState(p.xkb.MajorOpcode)
	if err != nil {
		return
	}

	p.mu.Lock()
	changed := p.xkbGroup != fullState.Group
	p.xkbGroup = fullState.Group
	xkbState := p.xkbState
	p.mu.Unlock()

	// Sync xkbcommon state using effective group (Wayland pattern).
	if xkbState != nil && xkbState.Ready() {
		xkbState.UpdateMask(
			fullState.BaseMods, fullState.LatchedMods, fullState.LockedMods,
			0, 0, uint32(fullState.Group),
		)
	}

	if changed {
		logger().Debug("XKB group changed via MappingNotify", "group", fullState.Group)
	}
}

// reloadXkbKeymap re-reads the keymap and re-syncs xkbcommon state.
// Called on XkbNewKeyboardNotify (keyboard hot-plug) and XkbMapNotify (keymap changed).
// BUG-INPUT-005: Handles layout reconfiguration without restart.
func (p *Platform) reloadXkbKeymap() {
	p.mu.Lock()
	xkbState := p.xkbState
	p.mu.Unlock()

	if xkbState == nil {
		return
	}

	// Re-read keymap (RMLVO or system defaults).
	// Under XWayland, _XKB_RULES_NAMES is unreliable — use system defaults.
	if !p.isXWayland {
		rules, model, layout, variant, options := p.readXKBRulesNames()
		if layout != "" {
			if err := xkbState.SetKeymapFromRMLVO(rules, model, layout, variant, options); err == nil {
				logger().Info("xkbcommon: keymap reloaded from _XKB_RULES_NAMES", "layout", layout)
			}
		}
	} else {
		if err := xkbState.SetKeymapFromNames(); err == nil {
			logger().Info("xkbcommon: keymap reloaded from system defaults (XWayland)")
		}
	}

	// Sync state after keymap reload using effective group (Wayland pattern).
	if p.xkb != nil && xkbState.Ready() {
		if fullState, err := p.conn.xkbGetFullState(p.xkb.MajorOpcode); err == nil {
			xkbState.UpdateMask(
				fullState.BaseMods, fullState.LatchedMods, fullState.LockedMods,
				0, 0, uint32(fullState.Group),
			)
			p.mu.Lock()
			p.xkbGroup = fullState.Group
			p.mu.Unlock()
		}
	}

	// Also re-read keyboard mapping for the fallback path.
	keymap, _ := p.conn.GetKeyboardMapping()
	if keymap != nil {
		p.mu.Lock()
		p.keymap = keymap
		p.mu.Unlock()
	}
}

// handleXITouchEvent processes an XI2 touch event and dispatches it as a PointerEvent.
func (w *x11Window) handleXITouchEvent(e *XIDeviceEvent) {
	var pointerType gpucontext.PointerEventType

	w.eventMu.Lock()
	switch e.EventType {
	case XITouchBegin:
		pointerType = gpucontext.PointerDown
		w.activeTouches[e.Detail] = true
		if !w.hasPrimary {
			w.primaryTouch = e.Detail
			w.hasPrimary = true
		}
	case XITouchUpdate:
		pointerType = gpucontext.PointerMove
	case XITouchEnd:
		pointerType = gpucontext.PointerUp
		delete(w.activeTouches, e.Detail)
		if w.hasPrimary && w.primaryTouch == e.Detail {
			w.hasPrimary = false
		}
	default:
		w.eventMu.Unlock()
		return
	}
	isPrimary := w.hasPrimary && w.primaryTouch == e.Detail
	mods := w.modifiers
	w.eventMu.Unlock()

	var pressure float32
	if e.EventType == XITouchBegin || e.EventType == XITouchUpdate {
		pressure = 0.5 // Default for touch without pressure axis
	}

	ev := gpucontext.PointerEvent{
		Type:        pointerType,
		PointerID:   int(e.Detail),
		X:           e.EventX,
		Y:           e.EventY,
		Pressure:    pressure,
		Width:       1,
		Height:      1,
		PointerType: gpucontext.PointerTypeTouch,
		IsPrimary:   isPrimary,
		Button:      gpucontext.ButtonLeft,
		Buttons:     gpucontext.ButtonsLeft,
		Modifiers:   mods,
		Timestamp:   w.eventTimestamp(),
	}

	// For PointerUp, clear button
	if e.EventType == XITouchEnd {
		ev.Buttons = gpucontext.ButtonsNone
	}

	w.dispatchPointerEvent(ev)
}

// ShouldClose returns true if window close was requested.
func (p *Platform) ShouldClose() bool {
	w := p.primary
	w.eventMu.Lock()
	defer w.eventMu.Unlock()
	return w.shouldClose
}

// GetSize returns current window size in physical pixels (what X11 reports).
func (p *Platform) GetSize() (width, height int) {
	w := p.primary
	w.eventMu.Lock()
	defer w.eventMu.Unlock()
	return w.width, w.height
}

// LogicalSize returns current window size in logical units (DIP).
// On HiDPI, this divides physical pixels by the scale factor.
func (p *Platform) LogicalSize() (width, height int) {
	pw, ph := p.GetSize()
	scale := p.ScaleFactor()
	if scale <= 1.0 {
		return pw, ph
	}
	return int(math.Round(float64(pw) / scale)), int(math.Round(float64(ph) / scale))
}

// GetHandle returns platform-specific handles for Vulkan surface creation.
// Returns (Display* pointer, X11 Window ID) for use with VK_KHR_xlib_surface.
// The Display* comes from XOpenDisplay (loaded via goffi), not from our pure Go
// X11 wire protocol connection. Window IDs are server-side resources shared
// across all connections to the same X server.
func (p *Platform) GetHandle() (display, window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary
	if p.xlib == nil || p.xlib.display == 0 {
		logger().Warn("GetHandle returning zero handles", "reason", "no xlib display")
		return 0, 0
	}

	logger().Debug("GetHandle", "display", fmt.Sprintf("%#x", p.xlib.display), "window", fmt.Sprintf("%#x", uintptr(w.window)))
	return p.xlib.display, uintptr(w.window)
}

// PollEventTimeout checks for a pending X11 event with a configurable timeout.
// Returns nil event if no event is available within the timeout.
func (p *Platform) PollEventTimeout(timeout time.Duration) (Event, error) {
	if p.conn == nil {
		return nil, ErrNotConnected
	}
	return p.conn.PollEventTimeout(timeout)
}

// MapWindow makes the primary window visible by sending MapWindow to the X server.
// Called separately from Init to allow GPU initialization while hidden.
func (p *Platform) MapWindow() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.primary == nil || p.conn == nil {
		return fmt.Errorf("x11: no window to map")
	}
	return p.conn.MapWindow(p.primary.window)
}

// Destroy closes the window and releases resources.
func (p *Platform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary

	// Close xkbcommon handle
	if p.xkbState != nil {
		p.xkbState.Close()
		p.xkbState = nil
	}

	// Close Xlib Display* (Vulkan surface handle)
	if p.xlib != nil {
		p.xlib.close()
		p.xlib = nil
	}

	if p.conn != nil {
		// Free per-window resources
		if w != nil {
			if w.blitGC != 0 {
				_ = p.conn.FreeGC(w.blitGC)
				w.blitGC = 0
			}
		}

		// Free process-level cursor resources
		for _, cursor := range p.cursorCache {
			_ = p.conn.FreeCursor(cursor)
		}
		p.cursorCache = nil
		if p.cursorFontID != 0 {
			_ = p.conn.CloseFont(p.cursorFontID)
			p.cursorFontID = 0
		}
		if p.blankCursorID != 0 {
			_ = p.conn.FreeCursor(p.blankCursorID)
			p.blankCursorID = 0
		}

		// Destroy window and unregister
		if w != nil && w.window != 0 {
			p.windowMu.Lock()
			delete(p.windows, w.window)
			p.windowMu.Unlock()

			_ = p.conn.DestroyWindow(w.window)
			w.window = 0
		}
		_ = p.conn.Close()
		p.conn = nil
	}

	p.primary = nil
	p.atoms = nil
	p.keymap = nil
}

// setDetectableAutoRepeat calls XkbSetDetectableAutoRepeat to suppress
// spurious KeyRelease events during key repeat (ADR-033).
// Without this, X11 sends a KeyRelease immediately before each synthetic
// KeyPress during repeat, making it impossible to distinguish a real
// key release from a repeat-related one.
func (p *Platform) setDetectableAutoRepeat() {
	if p.xlib == nil || p.xlib.display == 0 {
		return
	}

	// XkbSetDetectableAutoRepeat(Display*, Bool detectable, Bool* supported_rtrn) -> Bool
	sym, err := ffi.GetSymbol(p.xlib.lib, "XkbSetDetectableAutoRepeat")
	if err != nil || sym == nil {
		logger().Debug("XkbSetDetectableAutoRepeat not available", "err", err)
		return
	}

	// Prepare CIF: (Display*, Bool, Bool*) -> Bool
	// In X11: Bool = int (4 bytes), Display* = ptr
	var cif types.CallInterface
	if err := ffi.PrepareCallInterface(&cif, types.DefaultCall,
		types.SInt32TypeDescriptor, // return Bool
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // Display*
			types.SInt32TypeDescriptor,  // Bool detectable
			types.PointerTypeDescriptor, // Bool* supported_rtrn
		}); err != nil {
		logger().Debug("XkbSetDetectableAutoRepeat: failed to prepare CIF", "err", err)
		return
	}

	detectable := int32(1) // True — enable detectable auto-repeat
	var supported int32
	var result int32
	display := p.xlib.display
	args := [3]unsafe.Pointer{
		unsafe.Pointer(&display),
		unsafe.Pointer(&detectable),
		unsafe.Pointer(&supported),
	}
	_ = ffi.CallFunction(&cif, sym, unsafe.Pointer(&result), args[:])

	if supported != 0 {
		logger().Info("XkbSetDetectableAutoRepeat enabled")
	} else {
		logger().Debug("XkbSetDetectableAutoRepeat: server does not support detectable auto-repeat")
	}
}

// dispatchPointerEvent pushes a pointer event to the event queue.
func (w *x11Window) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	var evType EventType
	switch ev.Type {
	case gpucontext.PointerDown:
		evType = EventTypePointerDown
	case gpucontext.PointerUp:
		evType = EventTypePointerUp
	case gpucontext.PointerMove:
		evType = EventTypePointerMove
	case gpucontext.PointerEnter:
		evType = EventTypePointerEnter
	case gpucontext.PointerLeave:
		evType = EventTypePointerLeave
	default:
		evType = EventTypePointerMove
	}
	w.queueEvent(PlatformEvent{Type: evType, Pointer: ev})
}

// dispatchScrollEvent pushes a scroll event to the event queue.
func (w *x11Window) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	w.queueEvent(PlatformEvent{Type: EventTypeScroll, Scroll: ev})
}

// dispatchKeyEvent pushes a keyboard event to the event queue.
func (w *x11Window) dispatchKeyEvent(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	evType := EventTypeKeyDown
	if !pressed {
		evType = EventTypeKeyUp
	}
	w.queueEvent(PlatformEvent{Type: evType, Key: key, Mods: mods})
}

// handleKeyEvent processes a key press or release event.
// X11 keycodes = evdev keycodes + 8.
//
// Text input uses xkbcommon when available (handles AltGr/Level3 correctly).
// Falls back to manual KeycodeToKeysymGroup (no AltGr support) if xkbcommon is unavailable.
func (p *Platform) handleKeyEvent(w *x11Window, keycode uint8, state uint16, pressed bool) {
	mods := extractModifiers(state)

	w.eventMu.Lock()
	w.modifiers = mods
	w.eventMu.Unlock()

	// X11 keycodes are evdev keycodes offset by 8
	key := x11KeycodeToKey(keycode)
	if key == gpucontext.KeyUnknown {
		return
	}

	w.dispatchKeyEvent(key, mods, pressed)

	// Evdev keycode for xkbcommon: X11 keycode - 8
	evdevKey := uint32(keycode) - 8

	// Text input: use xkbcommon if available (handles AltGr/Level3 correctly).
	// Fallback to manual KeycodeToKeysymGroup (no AltGr support).
	p.mu.Lock()
	xkbState := p.xkbState
	keymap := p.keymap
	p.mu.Unlock()

	if pressed {
		dispatched := false

		// Primary: xkbcommon (handles AltGr/Level3 and all layouts correctly).
		// State is synced via UpdateMask from XkbStateNotify events (winit pattern).
		// Do NOT call UpdateKey here — winit never does on X11.
		if xkbState != nil && xkbState.Ready() {
			s := xkbState.KeyGetUtf8(evdevKey)
			if s != "" {
				for _, r := range s {
					if r >= 32 {
						w.queueEvent(PlatformEvent{Type: EventTypeChar, Char: r})
						dispatched = true
					}
				}
			}
		}

		// Fallback: manual lookup with group-aware keysym resolution.
		// Covers cases where xkb_keymap_new_from_names(NULL) doesn't include
		// the user's configured layouts (e.g., Russian via desktop settings).
		if !dispatched && keymap != nil {
			p.mu.Lock()
			group := p.xkbGroup
			p.mu.Unlock()
			shift := mods&gpucontext.ModShift != 0
			capsLock := mods&gpucontext.ModCapsLock != 0
			keysym := keymap.KeycodeToKeysymGroup(keycode, shift, capsLock, group)
			if r, ok := KeysymToRune(keysym); ok && r >= 32 {
				w.queueEvent(PlatformEvent{Type: EventTypeChar, Char: r})
			}
		}
	}
}

// x11KeycodeToKey converts an X11 keycode to a gpucontext.Key.
// X11 keycodes = Linux evdev keycodes + 8.
//
//nolint:maintidx // large switch for keycode mapping is inherently complex
func x11KeycodeToKey(keycode uint8) gpucontext.Key {
	// X11 keycode offset: X11 keycode = evdev keycode + 8
	const x11Offset = 8

	if keycode < x11Offset {
		return gpucontext.KeyUnknown
	}
	evdev := keycode - x11Offset

	// Linux evdev keycodes from linux/input-event-codes.h
	switch evdev {
	// Letters (evdev 30-49, 16-25)
	case 16:
		return gpucontext.KeyQ
	case 17:
		return gpucontext.KeyW
	case 18:
		return gpucontext.KeyE
	case 19:
		return gpucontext.KeyR
	case 20:
		return gpucontext.KeyT
	case 21:
		return gpucontext.KeyY
	case 22:
		return gpucontext.KeyU
	case 23:
		return gpucontext.KeyI
	case 24:
		return gpucontext.KeyO
	case 25:
		return gpucontext.KeyP
	case 30:
		return gpucontext.KeyA
	case 31:
		return gpucontext.KeyS
	case 32:
		return gpucontext.KeyD
	case 33:
		return gpucontext.KeyF
	case 34:
		return gpucontext.KeyG
	case 35:
		return gpucontext.KeyH
	case 36:
		return gpucontext.KeyJ
	case 37:
		return gpucontext.KeyK
	case 38:
		return gpucontext.KeyL
	case 44:
		return gpucontext.KeyZ
	case 45:
		return gpucontext.KeyX
	case 46:
		return gpucontext.KeyC
	case 47:
		return gpucontext.KeyV
	case 48:
		return gpucontext.KeyB
	case 49:
		return gpucontext.KeyN
	case 50:
		return gpucontext.KeyM

	// Numbers
	case 2:
		return gpucontext.Key1
	case 3:
		return gpucontext.Key2
	case 4:
		return gpucontext.Key3
	case 5:
		return gpucontext.Key4
	case 6:
		return gpucontext.Key5
	case 7:
		return gpucontext.Key6
	case 8:
		return gpucontext.Key7
	case 9:
		return gpucontext.Key8
	case 10:
		return gpucontext.Key9
	case 11:
		return gpucontext.Key0

	// Function keys
	case 59:
		return gpucontext.KeyF1
	case 60:
		return gpucontext.KeyF2
	case 61:
		return gpucontext.KeyF3
	case 62:
		return gpucontext.KeyF4
	case 63:
		return gpucontext.KeyF5
	case 64:
		return gpucontext.KeyF6
	case 65:
		return gpucontext.KeyF7
	case 66:
		return gpucontext.KeyF8
	case 67:
		return gpucontext.KeyF9
	case 68:
		return gpucontext.KeyF10
	case 87:
		return gpucontext.KeyF11
	case 88:
		return gpucontext.KeyF12

	// Navigation
	case 1:
		return gpucontext.KeyEscape
	case 15:
		return gpucontext.KeyTab
	case 14:
		return gpucontext.KeyBackspace
	case 28:
		return gpucontext.KeyEnter
	case 57:
		return gpucontext.KeySpace
	case 110:
		return gpucontext.KeyInsert
	case 111:
		return gpucontext.KeyDelete
	case 102:
		return gpucontext.KeyHome
	case 107:
		return gpucontext.KeyEnd
	case 104:
		return gpucontext.KeyPageUp
	case 109:
		return gpucontext.KeyPageDown
	case 105:
		return gpucontext.KeyLeft
	case 106:
		return gpucontext.KeyRight
	case 103:
		return gpucontext.KeyUp
	case 108:
		return gpucontext.KeyDown

	// Modifiers
	case 42:
		return gpucontext.KeyLeftShift
	case 54:
		return gpucontext.KeyRightShift
	case 29:
		return gpucontext.KeyLeftControl
	case 97:
		return gpucontext.KeyRightControl
	case 56:
		return gpucontext.KeyLeftAlt
	case 100:
		return gpucontext.KeyRightAlt
	case 125:
		return gpucontext.KeyLeftSuper
	case 126:
		return gpucontext.KeyRightSuper

	// Punctuation
	case 12:
		return gpucontext.KeyMinus
	case 13:
		return gpucontext.KeyEqual
	case 26:
		return gpucontext.KeyLeftBracket
	case 27:
		return gpucontext.KeyRightBracket
	case 43:
		return gpucontext.KeyBackslash
	case 39:
		return gpucontext.KeySemicolon
	case 40:
		return gpucontext.KeyApostrophe
	case 41:
		return gpucontext.KeyGrave
	case 51:
		return gpucontext.KeyComma
	case 52:
		return gpucontext.KeyPeriod
	case 53:
		return gpucontext.KeySlash

	// Numpad
	case 82:
		return gpucontext.KeyNumpad0
	case 79:
		return gpucontext.KeyNumpad1
	case 80:
		return gpucontext.KeyNumpad2
	case 81:
		return gpucontext.KeyNumpad3
	case 75:
		return gpucontext.KeyNumpad4
	case 76:
		return gpucontext.KeyNumpad5
	case 77:
		return gpucontext.KeyNumpad6
	case 71:
		return gpucontext.KeyNumpad7
	case 72:
		return gpucontext.KeyNumpad8
	case 73:
		return gpucontext.KeyNumpad9
	case 83:
		return gpucontext.KeyNumpadDecimal
	case 98:
		return gpucontext.KeyNumpadDivide
	case 55:
		return gpucontext.KeyNumpadMultiply
	case 74:
		return gpucontext.KeyNumpadSubtract
	case 78:
		return gpucontext.KeyNumpadAdd
	case 96:
		return gpucontext.KeyEnter // KP Enter

	// Lock keys
	case 58:
		return gpucontext.KeyCapsLock
	case 70:
		return gpucontext.KeyScrollLock
	case 69:
		return gpucontext.KeyNumLock
	case 119:
		return gpucontext.KeyPause
	}

	return gpucontext.KeyUnknown
}

// eventTimestamp returns the event timestamp as duration since start.
func (w *x11Window) eventTimestamp() time.Duration {
	return time.Since(w.startTime)
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
func (w *x11Window) createPointerEvent(
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
		Timestamp:   w.eventTimestamp(),
	}
}

// Frameless window support

func (p *Platform) SetFrameless(frameless bool) {
	w := p.primary
	w.callbackMu.Lock()
	w.frameless = frameless
	w.callbackMu.Unlock()

	if frameless {
		_ = p.conn.SetWindowBorderless(w.window, p.atoms)
	} else {
		// Restore decorations
		hints := &MotifWMHints{
			Flags:       MotifHintsDecorations,
			Decorations: MotifDecorAll,
		}
		_ = p.conn.SetMotifWMHints(w.window, hints, p.atoms)
	}
}

func (p *Platform) IsFrameless() bool {
	w := p.primary
	w.callbackMu.RLock()
	defer w.callbackMu.RUnlock()
	return w.frameless
}

func (p *Platform) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	w := p.primary
	w.callbackMu.Lock()
	defer w.callbackMu.Unlock()
	w.hitTestCallback = fn
}

// hitTestToMoveResizeDirection maps a HitTestResult to the _NET_WM_MOVERESIZE
// direction constant used by the EWMH specification.
// Returns the direction and true if the result should initiate a WM move/resize,
// or (0, false) for client area and button regions that we handle ourselves.
func hitTestToMoveResizeDirection(result gpucontext.HitTestResult) (uint32, bool) {
	switch result {
	case gpucontext.HitTestCaption:
		return 8, true // _NET_WM_MOVERESIZE_MOVE
	case gpucontext.HitTestResizeNW:
		return 0, true // _NET_WM_MOVERESIZE_SIZE_TOPLEFT
	case gpucontext.HitTestResizeN:
		return 1, true // _NET_WM_MOVERESIZE_SIZE_TOP
	case gpucontext.HitTestResizeNE:
		return 2, true // _NET_WM_MOVERESIZE_SIZE_TOPRIGHT
	case gpucontext.HitTestResizeE:
		return 3, true // _NET_WM_MOVERESIZE_SIZE_RIGHT
	case gpucontext.HitTestResizeSE:
		return 4, true // _NET_WM_MOVERESIZE_SIZE_BOTTOMRIGHT
	case gpucontext.HitTestResizeS:
		return 5, true // _NET_WM_MOVERESIZE_SIZE_BOTTOM
	case gpucontext.HitTestResizeSW:
		return 6, true // _NET_WM_MOVERESIZE_SIZE_BOTTOMLEFT
	case gpucontext.HitTestResizeW:
		return 7, true // _NET_WM_MOVERESIZE_SIZE_LEFT
	default:
		return 0, false
	}
}

func (p *Platform) Minimize() {
	// TODO: Implement using XIconifyWindow or _NET_WM_STATE
}

func (p *Platform) Maximize() {
	// TODO: Implement using _NET_WM_STATE_MAXIMIZED_VERT/_HORZ
}

func (p *Platform) IsMaximized() bool {
	// TODO: Query _NET_WM_STATE
	return false
}

// SetFullscreen enters or exits fullscreen mode via _NET_WM_STATE_FULLSCREEN.
func (p *Platform) SetFullscreen(fullscreen bool) {
	w := p.primary
	if w == nil || p.conn == nil {
		return
	}
	_ = p.conn.SetFullscreen(w.window, fullscreen, p.atoms)
	w.fullscreen = fullscreen
}

// IsFullscreen returns true if the window is in fullscreen mode.
func (p *Platform) IsFullscreen() bool {
	w := p.primary
	if w == nil {
		return false
	}
	return w.fullscreen
}

func (p *Platform) CloseWindow() {
	w := p.primary
	w.eventMu.Lock()
	w.shouldClose = true
	w.eventMu.Unlock()
}

// SetCursor changes the mouse cursor shape using the standard X11 cursor font.
// cursorID maps to gpucontext.CursorShape values (0-11).
func (p *Platform) SetCursor(cursorID int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary
	if p.conn == nil || w == nil || w.window == 0 {
		return
	}

	// CursorNone (11): hide cursor by setting a blank 1x1 cursor
	if cursorID == 11 {
		p.setBlankCursor(w)
		return
	}

	// Map gpucontext.CursorShape to X11 cursor font glyph index
	glyphIndex := cursorShapeToGlyph(cursorID)

	// Check cursor cache (process-level)
	if p.cursorCache == nil {
		p.cursorCache = make(map[int]ResourceID)
	}
	if cached, ok := p.cursorCache[cursorID]; ok {
		_ = p.conn.ChangeWindowCursor(w.window, cached)
		return
	}

	// Ensure cursor font is opened (process-level)
	if p.cursorFontID == 0 {
		fontID, err := p.conn.OpenFont("cursor")
		if err != nil {
			return
		}
		p.cursorFontID = fontID
	}

	// Create glyph cursor: black foreground on white background
	cursor, err := p.conn.CreateGlyphCursor(
		p.cursorFontID, p.cursorFontID,
		glyphIndex, glyphIndex+1,
		0, 0, 0, // foreground: black
		0xFFFF, 0xFFFF, 0xFFFF, // background: white
	)
	if err != nil {
		return
	}

	p.cursorCache[cursorID] = cursor
	_ = p.conn.ChangeWindowCursor(w.window, cursor)
}

// setBlankCursor creates and sets a 1x1 transparent cursor to hide the pointer.
// Must be called with p.mu held.
func (p *Platform) setBlankCursor(w *x11Window) {
	if cached, ok := p.cursorCache[11]; ok {
		_ = p.conn.ChangeWindowCursor(w.window, cached)
		return
	}

	// Revert to parent cursor (0 = None in X11 means inherit from parent).
	// For true cursor hiding we would need CreateCursor with empty pixmaps,
	// which requires CreatePixmap + CreateGC. For now, revert to default.
	_ = p.conn.ChangeWindowCursor(w.window, 0)
}

// FreeCursors releases all cached cursor resources and closes the cursor font.
// Should be called during platform cleanup.
func (p *Platform) FreeCursors() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		return
	}

	for _, cursor := range p.cursorCache {
		_ = p.conn.FreeCursor(cursor)
	}
	p.cursorCache = nil

	if p.cursorFontID != 0 {
		_ = p.conn.CloseFont(p.cursorFontID)
		p.cursorFontID = 0
	}
}

// cursorShapeToGlyph maps gpucontext.CursorShape int values to X11 cursor font glyph indices.
func cursorShapeToGlyph(cursorID int) uint16 {
	switch cursorID {
	case 0: // CursorDefault
		return XCursorLeftPtr
	case 1: // CursorPointer
		return XCursorHand2
	case 2: // CursorText
		return XCursorXterm
	case 3: // CursorCrosshair
		return XCursorCrosshair
	case 4: // CursorMove
		return XCursorFleur
	case 5: // CursorResizeNS
		return XCursorSBVDoubleArrow
	case 6: // CursorResizeEW
		return XCursorSBHDoubleArrow
	case 7: // CursorResizeNWSE
		return XCursorTopLeftCorner
	case 8: // CursorResizeNESW
		return XCursorBottomLeftCorner
	case 9: // CursorNotAllowed
		return XCursorCircle
	case 10: // CursorWait
		return XCursorWatch
	default:
		return XCursorLeftPtr
	}
}

// SetCursorMode sets cursor confinement/lock mode.
// 0=normal, 1=locked (hidden, confined, relative deltas), 2=confined (visible, confined).
func (p *Platform) SetCursorMode(mode int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary
	if p.conn == nil || w == nil || w.window == 0 {
		return
	}

	if mode == w.cursorMode {
		return
	}

	oldMode := w.cursorMode
	w.cursorMode = mode

	switch mode {
	case 1: // Locked
		// Save current mouse position
		if oldMode == 0 {
			w.savedMouseX = w.mouseX
			w.savedMouseY = w.mouseY
		}

		// Compute window center
		w.cursorCenterX = int16(w.width / 2)
		w.cursorCenterY = int16(w.height / 2)

		// Create invisible cursor if not already created (process-level resource)
		if p.blankCursorID == 0 {
			p.createBlankCursor(w)
		}

		// Grab pointer with invisible cursor, confined to our window
		grabMask := uint16(EventMaskPointerMotion | EventMaskButtonPress | EventMaskButtonRelease)
		_, _ = p.conn.GrabPointer(true, w.window, grabMask,
			GrabModeAsync, GrabModeAsync, w.window, p.blankCursorID, 0)
		w.cursorGrabbed = true

		// Warp to center
		_ = p.conn.WarpPointer(0, w.window, 0, 0, 0, 0, w.cursorCenterX, w.cursorCenterY)

	case 2: // Confined
		// Grab pointer with normal cursor, confined to window
		grabMask := uint16(EventMaskPointerMotion | EventMaskButtonPress | EventMaskButtonRelease)
		_, _ = p.conn.GrabPointer(true, w.window, grabMask,
			GrabModeAsync, GrabModeAsync, w.window, 0, 0)
		w.cursorGrabbed = true

		// Restore cursor position if coming from locked mode
		if oldMode == 1 {
			_ = p.conn.WarpPointer(0, w.window, 0, 0, 0, 0,
				int16(w.savedMouseX), int16(w.savedMouseY))
		}

	default: // Normal (0)
		// Release pointer grab
		if w.cursorGrabbed {
			_ = p.conn.UngrabPointer(0)
			w.cursorGrabbed = false
		}

		// Restore cursor (revert to normal)
		_ = p.conn.ChangeWindowCursor(w.window, 0)

		// Restore mouse position if coming from locked mode
		if oldMode == 1 {
			_ = p.conn.WarpPointer(0, w.window, 0, 0, 0, 0,
				int16(w.savedMouseX), int16(w.savedMouseY))
		}
	}
}

// GetCursorMode returns the current cursor mode.
func (p *Platform) GetCursorMode() int {
	w := p.primary
	w.eventMu.Lock()
	defer w.eventMu.Unlock()
	return w.cursorMode
}

// handleFocusIn re-applies cursor grab when window regains focus.
func (p *Platform) handleFocusIn(w *x11Window) {
	w.eventMu.Lock()
	mode := w.cursorMode
	// Reset cursorMode temporarily so SetCursorMode doesn't early-return
	if mode != 0 {
		w.cursorMode = 0
	}
	w.eventMu.Unlock()

	if mode != 0 {
		p.SetCursorMode(mode)
	}
}

// handleFocusOut releases cursor grab when window loses focus.
func (p *Platform) handleFocusOut(w *x11Window) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w.cursorGrabbed && p.conn != nil {
		_ = p.conn.UngrabPointer(0)
		w.cursorGrabbed = false
	}
}

// createBlankCursor creates a 1x1 transparent cursor for hiding the pointer.
// Must be called with p.mu held. The cursor is a process-level resource.
func (p *Platform) createBlankCursor(w *x11Window) {
	if p.conn == nil || w.window == 0 {
		return
	}

	// Create 1x1 pixmap (depth 1 for cursor source/mask)
	pixmap, err := p.conn.CreatePixmap(1, w.window, 1, 1)
	if err != nil {
		return
	}

	// Create cursor from the pixmap (both source and mask = same 1x1 pixmap)
	// With a 1-bit depth source of all zeros and mask of all zeros,
	// the cursor is fully transparent.
	cursor, err := p.conn.CreateCursor(pixmap, pixmap,
		0, 0, 0, // foreground RGB
		0, 0, 0, // background RGB
		0, 0) // hotspot
	if err != nil {
		_ = p.conn.FreePixmap(pixmap)
		return
	}

	_ = p.conn.FreePixmap(pixmap)
	p.blankCursorID = cursor
}

// BlitPixels copies RGBA pixel data to the window using X11 PutImage.
// Implements software backend presentation for X11.
// Converts RGBA to BGRA (X11 ZPixmap format on little-endian) and sends
// pixel data via the pure Go wire protocol.
func (p *Platform) BlitPixels(pixels []byte, width, height int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := p.primary
	if p.conn == nil || w == nil || w.window == 0 {
		return fmt.Errorf("x11: BlitPixels: no connection or window")
	}

	screen := p.conn.DefaultScreen()
	if screen == nil {
		return fmt.Errorf("x11: BlitPixels: no default screen")
	}

	// Lazily create a GC for blitting (per-window, bound to this drawable)
	if w.blitGC == 0 {
		gc, err := p.conn.CreateGC(w.window)
		if err != nil {
			return fmt.Errorf("x11: BlitPixels: failed to create GC: %w", err)
		}
		w.blitGC = gc
	}

	// Convert RGBA to BGRA (X11 ZPixmap on little-endian expects BGRA)
	bgra := make([]byte, len(pixels))
	for i := 0; i < len(pixels)-3; i += 4 {
		bgra[i+0] = pixels[i+2] // B
		bgra[i+1] = pixels[i+1] // G
		bgra[i+2] = pixels[i+0] // R
		bgra[i+3] = pixels[i+3] // A
	}

	err := p.conn.PutImage(
		w.window, w.blitGC,
		uint16(width), uint16(height),
		0, 0, // dst x, y
		screen.RootDepth,
		ImageFormatZPixmap,
		bgra,
	)
	if err != nil {
		return fmt.Errorf("x11: BlitPixels: %w", err)
	}

	return nil
}
