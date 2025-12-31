//go:build darwin

package darwin

import (
	"errors"
	"sync"
)

// Errors returned by Window operations.
var (
	ErrWindowCreationFailed = errors.New("darwin: window creation failed")
	ErrViewCreationFailed   = errors.New("darwin: view creation failed")
)

// WindowConfig holds configuration for creating a window.
type WindowConfig struct {
	Title      string
	Width      int
	Height     int
	Resizable  bool
	Fullscreen bool
}

// Window represents an NSWindow with its content view.
type Window struct {
	mu          sync.Mutex
	nsWindow    ID
	contentView ID
	metalLayer  ID
	width       int
	height      int
	shouldClose bool
	visible     bool
}

// NewWindow creates a new window with the given configuration.
func NewWindow(config WindowConfig) (*Window, error) {
	initSelectors()
	initClasses()

	w := &Window{
		width:  config.Width,
		height: config.Height,
	}

	// Calculate style mask
	styleMask := NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable
	if config.Resizable {
		styleMask |= NSWindowStyleMaskResizable
	}

	// Create content rect
	rect := MakeRect(0, 0, CGFloat(config.Width), CGFloat(config.Height))

	// Allocate NSWindow
	nsWindow := classes.NSWindow.Send(selectors.alloc)
	if nsWindow.IsNil() {
		return nil, ErrWindowCreationFailed
	}

	// Initialize window with content rect
	nsWindow = nsWindow.SendRectUintUintBool(
		selectors.initWithContentRectStyleMaskBackingDefer,
		rect,
		NSUInteger(styleMask),
		NSBackingStoreBuffered,
		false,
	)
	if nsWindow.IsNil() {
		return nil, ErrWindowCreationFailed
	}

	w.nsWindow = nsWindow

	// Set window title
	if config.Title != "" {
		title := NewNSString(config.Title)
		if title != nil {
			nsWindow.SendPtr(selectors.setTitle, title.ID().Ptr())
			title.Release()
		}
	}

	// Get content view
	w.contentView = nsWindow.Send(selectors.contentView)
	if w.contentView.IsNil() {
		return nil, ErrViewCreationFailed
	}

	// Enable mouse events
	nsWindow.SendBool(selectors.setAcceptsMouseMovedEvents, true)

	// Don't release when closed (we manage lifecycle)
	nsWindow.SendBool(selectors.setReleasedWhenClosed, false)

	// Center window on screen
	nsWindow.Send(selectors.center)

	return w, nil
}

// Show makes the window visible and brings it to front.
func (w *Window) Show() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	// Make key and order front (nil sender)
	w.nsWindow.SendPtr(selectors.makeKeyAndOrderFront, 0)
	w.visible = true
}

// Hide hides the window.
func (w *Window) Hide() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	// Order out (nil sender)
	w.nsWindow.SendPtr(selectors.orderOut, 0)
	w.visible = false
}

// Close closes the window.
func (w *Window) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.Send(selectors.close)
	w.shouldClose = true
}

// SetTitle sets the window title.
func (w *Window) SetTitle(title string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	nsTitle := NewNSString(title)
	if nsTitle != nil {
		w.nsWindow.SendPtr(selectors.setTitle, nsTitle.ID().Ptr())
		nsTitle.Release()
	}
}

// Size returns the current content size of the window.
func (w *Window) Size() (width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.width, w.height
}

// SetSize sets the window content size.
func (w *Window) SetSize(width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.width = width
	w.height = height

	// Get current frame
	frame := w.nsWindow.GetRect(selectors.frame)

	// Create new frame with updated size
	newFrame := MakeRect(
		frame.Origin.X,
		frame.Origin.Y,
		CGFloat(width),
		CGFloat(height),
	)

	// Set frame with display
	w.nsWindow.SendRect(selectors.setFrame, newFrame)
}

// ShouldClose returns true if the window should close.
func (w *Window) ShouldClose() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.shouldClose
}

// SetShouldClose sets the should close flag.
func (w *Window) SetShouldClose(shouldClose bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.shouldClose = shouldClose
}

// IsVisible returns true if the window is visible.
func (w *Window) IsVisible() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.visible
}

// NSWindow returns the underlying NSWindow ID.
func (w *Window) NSWindow() ID {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.nsWindow
}

// ContentView returns the window's content view ID.
func (w *Window) ContentView() ID {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.contentView
}

// Handle returns the native window handle (NSWindow pointer) for surface creation.
func (w *Window) Handle() uintptr {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.nsWindow.Ptr()
}

// ViewHandle returns the content view handle for Metal surface creation.
func (w *Window) ViewHandle() uintptr {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.contentView.Ptr()
}

// Destroy releases window resources.
func (w *Window) Destroy() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.metalLayer != 0 {
		// Surface owns the layer; window only keeps a weak reference.
		w.metalLayer = 0
	}

	if w.nsWindow != 0 {
		w.nsWindow.Send(selectors.close)
		w.nsWindow.Send(selectors.release)
		w.nsWindow = 0
	}

	w.contentView = 0
}

// SetMetalLayer attaches a CAMetalLayer to the content view.
// This enables Metal rendering for the window.
func (w *Window) SetMetalLayer(layer ID) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.contentView.IsNil() {
		return
	}

	// Enable layer backing
	w.contentView.SendBool(selectors.setWantsLayer, true)

	// Set the layer
	w.contentView.SendPtr(selectors.setLayer, layer.Ptr())

	w.metalLayer = layer
}

// MetalLayer returns the attached CAMetalLayer, if any.
func (w *Window) MetalLayer() ID {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.metalLayer
}

// UpdateSize updates the cached size from the actual window size.
// Call this after resize events.
func (w *Window) UpdateSize() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.contentView.IsNil() {
		return
	}

	// Get bounds of content view
	bounds := w.contentView.GetRect(selectors.bounds)
	w.width = int(bounds.Size.Width)
	w.height = int(bounds.Size.Height)
}

// Frame returns the window's frame rectangle (position and size).
func (w *Window) Frame() NSRect {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return NSRect{}
	}

	return w.nsWindow.GetRect(selectors.frame)
}

// ContentRect returns the content rectangle (inside title bar and borders).
func (w *Window) ContentRect() NSRect {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.contentView.IsNil() {
		return NSRect{}
	}

	return w.contentView.GetRect(selectors.bounds)
}

// Center centers the window on the screen.
func (w *Window) Center() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.Send(selectors.center)
}

// Miniaturize minimizes the window.
func (w *Window) Miniaturize() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendPtr(selectors.miniaturize, 0)
}

// Deminiaturize restores a minimized window.
func (w *Window) Deminiaturize() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendPtr(selectors.deminiaturize, 0)
}

// Zoom toggles the window zoom state.
func (w *Window) Zoom() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendPtr(selectors.zoom, 0)
}

// IsMiniaturized returns true if the window is minimized.
func (w *Window) IsMiniaturized() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return false
	}

	result := w.nsWindow.Send(selectors.isMiniaturized)
	return result != 0
}

// IsZoomed returns true if the window is zoomed (maximized).
func (w *Window) IsZoomed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return false
	}

	result := w.nsWindow.Send(selectors.isZoomed)
	return result != 0
}

// IsKeyWindow returns true if this is the key window.
func (w *Window) IsKeyWindow() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return false
	}

	result := w.nsWindow.Send(selectors.isKeyWindow)
	return result != 0
}
