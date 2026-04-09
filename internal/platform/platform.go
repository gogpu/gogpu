// Package platform provides OS-specific windowing abstraction.
package platform

import (
	"github.com/gogpu/gpucontext"
)

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
	Type           EventType
	Width          int // for resize events: logical size (platform points/DIP)
	Height         int // for resize events: logical size (platform points/DIP)
	PhysicalWidth  int // for resize events: physical pixels (GPU framebuffer)
	PhysicalHeight int // for resize events: physical pixels (GPU framebuffer)
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventNone EventType = iota
	EventClose
	EventResize
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

// Platform abstracts OS-specific windowing.
type Platform interface {
	// Init creates the window.
	Init(config Config) error

	// PollEvents processes pending events.
	// Returns the next event, or EventNone if no events.
	PollEvents() Event

	// ShouldClose returns true if window close was requested.
	ShouldClose() bool

	// LogicalSize returns the window size in platform points (DIP).
	// On macOS these are Cocoa points, on Windows they are DIP (96 DPI base).
	// Use this for layout, UI coordinates, and user-facing dimensions.
	LogicalSize() (width, height int)

	// PhysicalSize returns the GPU framebuffer size in device pixels.
	// On Retina/HiDPI displays this is larger than LogicalSize by ScaleFactor.
	// Use this for surface configuration, texture allocation, and GPU operations.
	PhysicalSize() (width, height int)

	// GetHandle returns platform-specific handles for surface creation.
	// On Windows: (hinstance, hwnd)
	// On macOS: (0, nsview)
	// On Linux: (display, window)
	GetHandle() (instance, window uintptr)

	// InSizeMove returns true if the window is currently being resized/moved.
	// During modal resize (Windows) or live resize (macOS), this returns true.
	// Used to defer swapchain recreation until resize ends.
	InSizeMove() bool

	// SetPointerCallback registers a callback for pointer events.
	// The callback receives W3C Pointer Events Level 3 compliant events.
	SetPointerCallback(fn func(gpucontext.PointerEvent))

	// SetScrollCallback registers a callback for scroll events.
	// The callback receives scroll events with position, delta, and modifiers.
	SetScrollCallback(fn func(gpucontext.ScrollEvent))

	// SetKeyCallback registers a callback for keyboard events.
	// The callback receives the key, modifiers, and whether the key was pressed (true) or released (false).
	SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool))

	// SetCharCallback registers a callback for Unicode character input.
	// Called when the OS translates a key press into a Unicode character,
	// supporting IME, compose sequences, and all keyboard layouts.
	SetCharCallback(fn func(char rune))

	// SetModalFrameCallback registers a callback invoked during platform modal
	// operations (e.g., Win32 drag/resize loop) to keep rendering alive.
	//
	// On Windows, DefWindowProc enters a modal message loop during window
	// drag/resize that blocks the application's main loop. A WM_TIMER fires
	// at ~60fps to invoke this callback, maintaining smooth rendering.
	//
	// On macOS and Linux this is a no-op — those platforms don't have modal
	// resize loops.
	//
	// Future: An independent render thread running on its own schedule would
	// eliminate the need for this callback entirely. See ROADMAP.md.
	SetModalFrameCallback(fn func())

	// WaitEvents blocks until at least one OS event is available.
	// Uses OS-level blocking (MsgWaitForMultipleObjectsEx on Windows).
	// Returns when an OS event arrives or WakeUp() is called.
	// Does NOT remove messages from the queue; PollEvents handles that.
	WaitEvents()

	// WakeUp unblocks WaitEvents from any goroutine.
	// Thread-safe. Uses PostMessage on Windows, pipe fd on Linux.
	WakeUp()

	// Destroy closes the window and releases resources.
	Destroy()

	// ScaleFactor returns the DPI scale factor.
	// 1.0 = standard (96 DPI on Windows), 2.0 = HiDPI.
	ScaleFactor() float64

	// PrepareFrame updates platform-specific surface state before frame acquisition.
	// Called by the renderer before each Surface.AcquireTexture().
	//
	// On macOS: refreshes CAMetalLayer.contentsScale from BackingScaleFactor.
	// On Windows: returns current DPI state (future: apply pending WM_DPICHANGED).
	// On Wayland: returns current scale (future: apply pending wl_output.scale).
	// On X11: returns static DPI state (no dynamic scaling).
	PrepareFrame() PrepareFrameResult

	// ClipboardRead reads text from system clipboard.
	ClipboardRead() (string, error)

	// ClipboardWrite writes text to system clipboard.
	ClipboardWrite(text string) error

	// SetCursor changes the mouse cursor shape.
	// cursorID maps to gpucontext.CursorShape values (0-11).
	SetCursor(cursorID int)

	// SetFrameless enables or disables frameless window mode at runtime.
	SetFrameless(frameless bool)

	// IsFrameless returns true if the window has no OS chrome.
	IsFrameless() bool

	// SetHitTestCallback sets the callback for custom hit testing in frameless mode.
	// The callback receives cursor position in logical points (DIP) and returns
	// a gpucontext.HitTestResult indicating what region the cursor is over.
	SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult)

	// Minimize minimizes the window.
	Minimize()

	// Maximize toggles between maximized and restored window state.
	Maximize()

	// IsMaximized returns true if the window is maximized.
	IsMaximized() bool

	// CloseWindow requests the window to close.
	CloseWindow()

	// SyncFrame synchronizes the rendered frame with the compositor.
	// On Windows, calls DwmFlush() during resize to sync with DWM composition.
	// On other platforms, this is a no-op.
	SyncFrame()

	// SetCursorMode sets the cursor confinement/lock mode.
	// mode: 0=normal (free movement), 1=locked (hidden, confined, relative deltas),
	// 2=confined (visible, confined to window).
	SetCursorMode(mode int)

	// CursorMode returns the current cursor mode.
	// 0=normal, 1=locked, 2=confined.
	CursorMode() int

	// DarkMode returns true if system dark mode is active.
	DarkMode() bool

	// ReduceMotion returns true if user prefers reduced animation.
	ReduceMotion() bool

	// HighContrast returns true if high contrast mode is active.
	HighContrast() bool

	// FontScale returns font size preference multiplier.
	FontScale() float32
}

// PixelBlitter is an optional interface for platforms that support
// direct pixel blitting to the window (software backend presentation).
// Platforms that do not implement this interface will not display
// software-rendered frames (headless mode still works).
type PixelBlitter interface {
	BlitPixels(pixels []byte, width, height int) error
}

// New creates a platform-specific implementation.
// This is implemented in platform-specific files.
func New() Platform {
	return newPlatform()
}
