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
	Title      string
	Width      int
	Height     int
	Resizable  bool
	Fullscreen bool
	Frameless  bool
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
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventNone EventType = iota
	EventClose
	EventResize
	EventFocus
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

	// SetCursor changes the mouse cursor shape.
	SetCursor(cursorID int)

	// SetFrameless enables or disables frameless window mode.
	SetFrameless(frameless bool)

	// IsFrameless returns true if the window has no OS chrome.
	IsFrameless() bool

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

	// SyncFrame synchronizes the rendered frame with the compositor.
	SyncFrame()

	// SetCursorMode sets the cursor confinement/lock mode.
	SetCursorMode(mode int)

	// CursorMode returns the current cursor mode.
	CursorMode() int

	// SetPointerCallback registers a callback for pointer events.
	SetPointerCallback(fn func(gpucontext.PointerEvent))

	// SetScrollCallback registers a callback for scroll events.
	SetScrollCallback(fn func(gpucontext.ScrollEvent))

	// SetKeyCallback registers a callback for keyboard events.
	SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool))

	// SetCharCallback registers a callback for Unicode character input.
	SetCharCallback(fn func(char rune))

	// SetModalFrameCallback registers a callback for platform modal operations.
	SetModalFrameCallback(fn func())

	// Destroy releases native window resources.
	Destroy()
}

// NewManager creates a platform-specific PlatformManager.
// Each platform file provides newPlatformManager().
func NewManager() PlatformManager {
	return newPlatformManager()
}
