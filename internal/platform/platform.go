// Package platform provides OS-specific windowing abstraction.
package platform

import (
	"sync/atomic"

	"github.com/gogpu/gpucontext"
)

// WindowID uniquely identifies a window. Zero is invalid.
type WindowID uint32

var nextWindowID atomic.Uint32

// NewWindowID allocates a new unique window ID.
func NewWindowID() WindowID {
	return WindowID(nextWindowID.Add(1))
}

// Config holds platform-agnostic window configuration.
type Config struct {
	Title             string
	Width             int
	Height            int
	Resizable         bool
	Fullscreen        bool
	Frameless         bool
	TabbingMode       int
	TabbingIdentifier string
	MinWidth          int // 0 = no minimum constraint
	MinHeight         int // 0 = no minimum constraint
	MaxWidth          int // 0 = no maximum constraint
	MaxHeight         int // 0 = no maximum constraint
}

// Event represents a platform event.
type Event struct {
	WindowID       WindowID
	Type           EventType
	Width          int  // for resize events: logical size (platform points/DIP)
	Height         int  // for resize events: logical size (platform points/DIP)
	PhysicalWidth  int  // for resize events: physical pixels (GPU framebuffer)
	PhysicalHeight int  // for resize events: physical pixels (GPU framebuffer)
	Focused        bool // for focus events: true = gained focus, false = lost focus

	// Keyboard (EventKeyDown, EventKeyUp)
	Key  gpucontext.Key
	Mods gpucontext.Modifiers

	// Character input (EventChar)
	Char rune

	// Pointer (EventPointer*)
	Pointer gpucontext.PointerEvent

	// Scroll (EventScroll)
	Scroll gpucontext.ScrollEvent
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventNone EventType = iota
	EventClose
	EventResize
	EventFocus
	EventKeyDown
	EventKeyUp
	EventChar
	EventPointerDown
	EventPointerUp
	EventPointerMove
	EventPointerEnter
	EventPointerLeave
	EventScroll
	EventExpose
)

// PrepareFrameResult contains per-frame surface state from the platform layer.
// Returned by PrepareFrame to inform the renderer about scale/size changes.
type PrepareFrameResult struct {
	// ScaleChanged indicates the DPI scale factor changed since last frame.
	// When true, the renderer should reconfigure the surface with new physical dimensions.
	ScaleChanged bool

	// ScaleFactor is the current DPI scale factor (1.0 = standard, 2.0 = Retina/HiDPI).
	ScaleFactor float64

	// PhysicalWidth is the current surface width in physical device pixels.
	PhysicalWidth uint32

	// PhysicalHeight is the current surface height in physical device pixels.
	PhysicalHeight uint32
}

// PixelBlitter is an optional interface for platforms that support
// direct pixel blitting to the window (software backend presentation).
// Platforms that do not implement this interface will not display
// software-rendered frames (headless mode still works).
type PixelBlitter interface {
	BlitPixels(pixels []byte, width, height int) error
}

// FileDialogOptions configures a native file open or save dialog.
type FileDialogOptions struct {
	Title            string
	Filters          []FileTypeFilter
	Directory        bool   // pick a directory instead of a file
	Multiple         bool   // allow multi-selection (open only)
	InitialDirectory string // starting directory (optional)
	DefaultFilename  string // suggested filename for save dialog (optional)
}

// FileTypeFilter restricts visible files in the dialog.
type FileTypeFilter struct {
	Name       string   // e.g. "Images"
	Extensions []string // e.g. ["*.png", "*.jpg"] or ["png", "jpg"]
}

// PlatformManager handles process-level platform operations.
// One per application. Manages window lifecycle and event loop.
type PlatformManager interface {
	// Init initializes the platform subsystem.
	Init() error

	// CreateWindow creates a new platform window and returns it.
	CreateWindow(config Config) (PlatformWindow, error)

	// PollEvents returns the next pending event across ALL windows.
	// Returns Event with Type=EventNone if no events are pending.
	PollEvents() Event

	// WaitEvents blocks until at least one OS event is available.
	WaitEvents()

	// WakeUp unblocks WaitEvents from any goroutine. Thread-safe.
	WakeUp()

	// ClipboardRead reads text from system clipboard.
	ClipboardRead() (string, error)

	// ClipboardWrite writes text to system clipboard.
	ClipboardWrite(text string) error

	// DarkMode returns true if system dark mode is active.
	DarkMode() bool

	// ReduceMotion returns true if user prefers reduced animation.
	ReduceMotion() bool

	// HighContrast returns true if high contrast mode is active.
	HighContrast() bool

	// FontScale returns font size preference multiplier.
	FontScale() float32

	// SubpixelLayout returns the display's subpixel arrangement for LCD text.
	SubpixelLayout() gpucontext.SubpixelLayout

	// SetAppName sets the application name (displayed in menus).
	SetAppName(name string)

	// ShowOpenFileDialog opens a native file picker dialog.
	// Returns nil, nil if the user cancels.
	ShowOpenFileDialog(opts FileDialogOptions) ([]string, error)

	// ShowSaveFileDialog opens a native file save dialog.
	// Returns "", nil if the user cancels.
	ShowSaveFileDialog(opts FileDialogOptions) (string, error)

	// Destroy releases all platform resources.
	Destroy()
}

// PlatformWindow represents a single OS window.
// Multiple PlatformWindows can exist per PlatformManager.
type PlatformWindow interface {
	// ID returns the unique window identifier.
	ID() WindowID

	// GetHandle returns platform-specific handles for GPU surface creation.
	GetHandle() (instance, window uintptr)

	// LogicalSize returns window size in platform points (DIP).
	LogicalSize() (width, height int)

	// PhysicalSize returns GPU framebuffer size in device pixels.
	PhysicalSize() (width, height int)

	// ScaleFactor returns the DPI scale factor.
	ScaleFactor() float64

	// PrepareFrame updates platform-specific surface state before frame acquisition.
	PrepareFrame() PrepareFrameResult

	// InSizeMove returns true during modal resize/move operations.
	InSizeMove() bool

	// ShouldClose returns true if window close was requested.
	ShouldClose() bool

	// SetTitle changes the window title.
	SetTitle(title string)

	// SetMinSize sets the minimum window size in logical pixels.
	// Use 0 for a dimension to remove that minimum constraint.
	SetMinSize(width, height int)

	// SetMaxSize sets the maximum window size in logical pixels.
	// Use 0 for a dimension to remove that maximum constraint.
	SetMaxSize(width, height int)

	// SetCursor changes the mouse cursor shape.
	SetCursor(cursorID int)

	// SetFrameless enables or disables frameless window mode.
	SetFrameless(frameless bool)

	// IsFrameless returns true if the window has no OS chrome.
	IsFrameless() bool

	// SetFullscreen enters or exits fullscreen mode.
	// On Windows: borderless fullscreen (Chromium/GLFW pattern).
	// On macOS: native toggleFullScreen with animation.
	// On X11: _NET_WM_STATE_FULLSCREEN via EWMH.
	// On Wayland: xdg_toplevel.set_fullscreen / unset_fullscreen.
	SetFullscreen(fullscreen bool)

	// IsFullscreen returns true if the window is currently in fullscreen mode.
	IsFullscreen() bool

	// SetHitTestCallback sets the callback for custom hit testing in frameless mode.
	SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult)

	// Minimize minimizes the window.
	Minimize()

	// Maximize toggles between maximized and restored window state.
	Maximize()

	// IsMaximized returns true if the window is maximized.
	IsMaximized() bool

	// Close requests the window to close.
	Close()

	// Show makes the window visible and gives it input focus.
	// Called by App after GPU initialization to avoid black-screen flash.
	// On Wayland and Browser this is a no-op (compositor controls visibility).
	Show()

	// SyncFrame synchronizes the rendered frame with the compositor.
	SyncFrame()

	// SetCursorMode sets the cursor confinement/lock mode.
	SetCursorMode(mode int)

	// CursorMode returns the current cursor mode.
	CursorMode() int

	// SetModalFrameCallback registers a callback for platform modal operations.
	SetModalFrameCallback(fn func())

	// Destroy releases native window resources.
	Destroy()
}

// DisplayLocker is an optional interface for platforms where the display
// connection is shared between threads and requires explicit synchronization.
// On Wayland, wl_display is NOT thread-safe — the main thread's event dispatch
// and the render thread's Vulkan WSI calls (present, acquire) can corrupt
// internal state if they run concurrently. Platforms that implement this
// interface provide Lock/Unlock around critical wl_display operations.
// Platforms that don't need display-level locking (X11, Win32, macOS, browser)
// simply don't implement this interface — callers use type assertion.
//
// ADR-041 Phase 2: Wayland wl_display thread safety.
type DisplayLocker interface {
	// DisplayLock acquires the display mutex. Must be called by the render
	// thread before Vulkan WSI operations (surface acquire, present).
	DisplayLock()

	// DisplayUnlock releases the display mutex.
	DisplayUnlock()
}

type MenuRole int

const (
	MenuRoleNone MenuRole = iota
	MenuRoleAbout
	MenuRolePreferences
	MenuRoleServices
	MenuRoleHide
	MenuRoleHideOthers
	MenuRoleShowAll
	MenuRoleQuit
	MenuRoleClose
	MenuRoleMinimize
	MenuRoleZoom
	MenuRoleFullScreen
	MenuRoleBringAllToFront
)

// PlatScaleProvider is an optional interface for platforms that can report the
// primary display DPI scale factor before a window is created. Callers use a
// type assertion: if sp, ok := manager.(PlatScaleProvider); ok { ... }
//
// On macOS this is implemented via [NSScreen mainScreen].backingScaleFactor,
// which is available before NSApplication initialization (Flutter/GLFW pattern).
type PlatScaleProvider interface {
	ScaleFactor() float64
}

// PlatMenuManager is an optional interface for platforms that support
// native application menus (macOS). Platforms that don't support menus
// simply don't implement this interface.
type PlatMenuManager interface {
	// SetApplicationMenu replaces the native application menu with the given items.
	SetApplicationMenu(items []MenuItem)
	// AddToSystemMenu adds items to a standard system menu (Application, Window, etc.).
	// If the platform doesn't support the requested menu, it returns false.
	AddToSystemMenu(menu SystemMenu, items []MenuItem) bool
}

// SystemMenu identifies a standard menu that can be extended.
type SystemMenu int

const (
	SystemMenuApplication SystemMenu = iota
	SystemMenuWindow
)

// MenuItem is a platform-agnostic description of a menu item.
// Mirrors gogpu.MenuItem without importing gogpu.
type MenuItem struct {
	Title     string
	Action    func()
	Role      MenuRole
	Disabled  bool
	Separator bool
	Submenu   []MenuItem
}

// NewManager creates a platform-specific PlatformManager.
// Each platform file provides newPlatformManager().
func NewManager() PlatformManager {
	return newPlatformManager()
}
