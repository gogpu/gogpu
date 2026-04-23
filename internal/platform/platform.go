// Package platform provides OS-specific windowing abstraction.
package platform

import (
	"fmt"
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

// legacyPlatformAdapter wraps a PlatformManager + single PlatformWindow
// to implement the old Platform interface. Enables gradual migration.
type legacyPlatformAdapter struct {
	mgr    PlatformManager
	window PlatformWindow
}

// Verify interface compliance.
var _ Platform = (*legacyPlatformAdapter)(nil)

func (a *legacyPlatformAdapter) Init(config Config) error {
	if err := a.mgr.Init(); err != nil {
		return fmt.Errorf("platform init: %w", err)
	}
	w, err := a.mgr.CreateWindow(config)
	if err != nil {
		return fmt.Errorf("create window: %w", err)
	}
	a.window = w
	return nil
}

func (a *legacyPlatformAdapter) PollEvents() Event {
	return a.mgr.PollEvents()
}

func (a *legacyPlatformAdapter) ShouldClose() bool {
	return a.window.ShouldClose()
}

func (a *legacyPlatformAdapter) LogicalSize() (width, height int) {
	return a.window.LogicalSize()
}

func (a *legacyPlatformAdapter) PhysicalSize() (width, height int) {
	return a.window.PhysicalSize()
}

func (a *legacyPlatformAdapter) GetHandle() (instance, window uintptr) {
	return a.window.GetHandle()
}

func (a *legacyPlatformAdapter) InSizeMove() bool {
	return a.window.InSizeMove()
}

func (a *legacyPlatformAdapter) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	a.window.SetPointerCallback(fn)
}

func (a *legacyPlatformAdapter) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	a.window.SetScrollCallback(fn)
}

func (a *legacyPlatformAdapter) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	a.window.SetKeyCallback(fn)
}

func (a *legacyPlatformAdapter) SetCharCallback(fn func(char rune)) {
	a.window.SetCharCallback(fn)
}

func (a *legacyPlatformAdapter) SetModalFrameCallback(fn func()) {
	a.window.SetModalFrameCallback(fn)
}

func (a *legacyPlatformAdapter) WaitEvents() {
	a.mgr.WaitEvents()
}

func (a *legacyPlatformAdapter) WakeUp() {
	a.mgr.WakeUp()
}

func (a *legacyPlatformAdapter) Destroy() {
	if a.window != nil {
		a.window.Destroy()
	}
	a.mgr.Destroy()
}

func (a *legacyPlatformAdapter) ScaleFactor() float64 {
	return a.window.ScaleFactor()
}

func (a *legacyPlatformAdapter) PrepareFrame() PrepareFrameResult {
	return a.window.PrepareFrame()
}

func (a *legacyPlatformAdapter) ClipboardRead() (string, error) {
	return a.mgr.ClipboardRead()
}

func (a *legacyPlatformAdapter) ClipboardWrite(text string) error {
	return a.mgr.ClipboardWrite(text)
}

func (a *legacyPlatformAdapter) SetCursor(cursorID int) {
	a.window.SetCursor(cursorID)
}

func (a *legacyPlatformAdapter) SetFrameless(frameless bool) {
	a.window.SetFrameless(frameless)
}

func (a *legacyPlatformAdapter) IsFrameless() bool {
	return a.window.IsFrameless()
}

func (a *legacyPlatformAdapter) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	a.window.SetHitTestCallback(fn)
}

func (a *legacyPlatformAdapter) Minimize() {
	a.window.Minimize()
}

func (a *legacyPlatformAdapter) Maximize() {
	a.window.Maximize()
}

func (a *legacyPlatformAdapter) IsMaximized() bool {
	return a.window.IsMaximized()
}

func (a *legacyPlatformAdapter) CloseWindow() {
	a.window.Close()
}

func (a *legacyPlatformAdapter) SyncFrame() {
	a.window.SyncFrame()
}

func (a *legacyPlatformAdapter) SetCursorMode(mode int) {
	a.window.SetCursorMode(mode)
}

func (a *legacyPlatformAdapter) CursorMode() int {
	return a.window.CursorMode()
}

func (a *legacyPlatformAdapter) DarkMode() bool {
	return a.mgr.DarkMode()
}

func (a *legacyPlatformAdapter) ReduceMotion() bool {
	return a.mgr.ReduceMotion()
}

func (a *legacyPlatformAdapter) HighContrast() bool {
	return a.mgr.HighContrast()
}

func (a *legacyPlatformAdapter) FontScale() float32 {
	return a.mgr.FontScale()
}

// BlitPixels delegates to the window if it implements PixelBlitter.
func (a *legacyPlatformAdapter) BlitPixels(pixels []byte, width, height int) error {
	if blitter, ok := a.window.(PixelBlitter); ok {
		return blitter.BlitPixels(pixels, width, height)
	}
	return fmt.Errorf("window does not support pixel blitting")
}

// NewManager creates a platform-specific PlatformManager.
// Each platform file provides newPlatformManager().
func NewManager() PlatformManager {
	return newPlatformManager()
}

// WrapAsLegacy wraps a PlatformManager and PlatformWindow into a legacy Platform.
// Used by App.Run() to bridge the new multi-window API with the existing
// renderer that still expects platform.Platform.
func WrapAsLegacy(mgr PlatformManager, win PlatformWindow) Platform {
	return &legacyPlatformAdapter{mgr: mgr, window: win}
}

// New creates a platform-specific implementation via the legacy adapter.
//
// Deprecated: Use NewManager() for multi-window support.
func New() Platform {
	mgr := NewManager()
	return &legacyPlatformAdapter{mgr: mgr}
}

// platformManagerAdapter wraps a legacy Platform factory to implement PlatformManager.
// Used for platforms that haven't been migrated to native PlatformManager yet
// (Linux/X11, Linux/Wayland, macOS). The adapter defers Platform creation until
// CreateWindow, which calls the old Platform.Init(config) under the hood.
type platformManagerAdapter struct {
	factory func() Platform
	plat    Platform
}

// Verify interface compliance.
var _ PlatformManager = (*platformManagerAdapter)(nil)

func (a *platformManagerAdapter) Init() error {
	// Platform struct is created eagerly so process-level state (e.g. DPI
	// awareness) can be initialized, but the old Init(config) that creates
	// the window is deferred to CreateWindow.
	a.plat = a.factory()
	return nil
}

func (a *platformManagerAdapter) CreateWindow(config Config) (PlatformWindow, error) {
	if a.plat == nil {
		return nil, fmt.Errorf("platform not initialized: call Init() first")
	}
	if err := a.plat.Init(config); err != nil {
		return nil, err
	}
	return &platformWindowAdapter{
		id:   NewWindowID(),
		plat: a.plat,
	}, nil
}

func (a *platformManagerAdapter) PollEvents() Event {
	if a.plat == nil {
		return Event{Type: EventNone}
	}
	return a.plat.PollEvents()
}

func (a *platformManagerAdapter) WaitEvents() {
	if a.plat != nil {
		a.plat.WaitEvents()
	}
}

func (a *platformManagerAdapter) WakeUp() {
	if a.plat != nil {
		a.plat.WakeUp()
	}
}

func (a *platformManagerAdapter) ClipboardRead() (string, error) {
	if a.plat != nil {
		return a.plat.ClipboardRead()
	}
	return "", nil
}

func (a *platformManagerAdapter) ClipboardWrite(text string) error {
	if a.plat != nil {
		return a.plat.ClipboardWrite(text)
	}
	return nil
}

func (a *platformManagerAdapter) DarkMode() bool {
	if a.plat != nil {
		return a.plat.DarkMode()
	}
	return false
}

func (a *platformManagerAdapter) ReduceMotion() bool {
	if a.plat != nil {
		return a.plat.ReduceMotion()
	}
	return false
}

func (a *platformManagerAdapter) HighContrast() bool {
	if a.plat != nil {
		return a.plat.HighContrast()
	}
	return false
}

func (a *platformManagerAdapter) FontScale() float32 {
	if a.plat != nil {
		return a.plat.FontScale()
	}
	return 1.0
}

func (a *platformManagerAdapter) Destroy() {
	if a.plat != nil {
		a.plat.Destroy()
		a.plat = nil
	}
}

// platformWindowAdapter wraps a legacy Platform to provide PlatformWindow.
// Used by platformManagerAdapter for platforms that haven't been migrated yet.
type platformWindowAdapter struct {
	id   WindowID
	plat Platform
}

// Verify interface compliance.
var _ PlatformWindow = (*platformWindowAdapter)(nil)

func (w *platformWindowAdapter) ID() WindowID                     { return w.id }
func (w *platformWindowAdapter) GetHandle() (uintptr, uintptr)    { return w.plat.GetHandle() }
func (w *platformWindowAdapter) LogicalSize() (int, int)          { return w.plat.LogicalSize() }
func (w *platformWindowAdapter) PhysicalSize() (int, int)         { return w.plat.PhysicalSize() }
func (w *platformWindowAdapter) ScaleFactor() float64             { return w.plat.ScaleFactor() }
func (w *platformWindowAdapter) PrepareFrame() PrepareFrameResult { return w.plat.PrepareFrame() }
func (w *platformWindowAdapter) InSizeMove() bool                 { return w.plat.InSizeMove() }
func (w *platformWindowAdapter) ShouldClose() bool                { return w.plat.ShouldClose() }
func (w *platformWindowAdapter) SetTitle(_ string)                {} // legacy Platform has no SetTitle
func (w *platformWindowAdapter) SetCursor(cursorID int)           { w.plat.SetCursor(cursorID) }
func (w *platformWindowAdapter) SetFrameless(frameless bool)      { w.plat.SetFrameless(frameless) }
func (w *platformWindowAdapter) IsFrameless() bool                { return w.plat.IsFrameless() }

func (w *platformWindowAdapter) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	w.plat.SetHitTestCallback(fn)
}

func (w *platformWindowAdapter) Minimize()              { w.plat.Minimize() }
func (w *platformWindowAdapter) Maximize()              { w.plat.Maximize() }
func (w *platformWindowAdapter) IsMaximized() bool      { return w.plat.IsMaximized() }
func (w *platformWindowAdapter) Close()                 { w.plat.CloseWindow() }
func (w *platformWindowAdapter) SyncFrame()             { w.plat.SyncFrame() }
func (w *platformWindowAdapter) SetCursorMode(mode int) { w.plat.SetCursorMode(mode) }
func (w *platformWindowAdapter) CursorMode() int        { return w.plat.CursorMode() }

func (w *platformWindowAdapter) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	w.plat.SetPointerCallback(fn)
}

func (w *platformWindowAdapter) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	w.plat.SetScrollCallback(fn)
}

func (w *platformWindowAdapter) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	w.plat.SetKeyCallback(fn)
}

func (w *platformWindowAdapter) SetCharCallback(fn func(char rune)) {
	w.plat.SetCharCallback(fn)
}

func (w *platformWindowAdapter) SetModalFrameCallback(fn func()) {
	w.plat.SetModalFrameCallback(fn)
}

// BlitPixels delegates to the underlying platform if it supports pixel blitting.
// This enables the software backend to work through the adapter on all platforms.
func (w *platformWindowAdapter) BlitPixels(pixels []byte, width, height int) error {
	if blitter, ok := w.plat.(PixelBlitter); ok {
		return blitter.BlitPixels(pixels, width, height)
	}
	return fmt.Errorf("platform does not support pixel blitting")
}

func (w *platformWindowAdapter) Destroy() {
	// Destruction is handled by platformManagerAdapter.Destroy() which
	// calls plat.Destroy(). Individual window adapter destroy is a no-op
	// to avoid double-free for single-window legacy platforms.
}
