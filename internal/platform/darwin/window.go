//go:build darwin

package darwin

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Errors returned by Window operations.
var (
	ErrWindowCreationFailed = errors.New("darwin: window creation failed")
	ErrViewCreationFailed   = errors.New("darwin: view creation failed")
)

// WindowConfig holds configuration for creating a window.
type WindowConfig struct {
	Title             string
	Width             int
	Height            int
	Resizable         bool
	Fullscreen        bool
	Frameless         bool
	TabbingMode       int
	TabbingIdentifier string
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
	delegate    ID
	onClose     func() bool

	// screenChangedCh receives a signal when the window moves to a display
	// with a different backing scale factor (windowDidChangeScreen: delegate).
	// Capacity 1: rapid transitions are coalesced; consumer need not be realtime.
	screenChangedCh chan struct{}

	// inLiveResize is true while the user is dragging a resize handle.
	// Set/cleared by the windowWillStartLiveResize:/windowDidEndLiveResize: delegate.
	// Atomic so the render thread can safely read it without holding mu.
	inLiveResize atomic.Bool

	// Header alignment state.
	alignment      int // 0=center, 1=left, 2=right
	cachedTitle    string
	titleTextField ID // NSTextField injected into the traffic-light button container for center/right alignment; 0 when inactive

	// liveResizeHook, if non-nil, is called on every windowDidResize: notification
	// (including notifications fired during AppKit's live-resize modal loop).
	// Used by the app layer to render a frame while the event loop is blocked.
	// Read/written under mu.
	liveResizeHook func()
}

// NSID returns the underlying NSWindow object ID.
// Used by the platform layer to match NSEvents to their originating window.
func (w *Window) NSID() ID { return w.nsWindow }

// NewWindow creates a new window with the given configuration.
func NewWindow(config WindowConfig) (*Window, error) {
	initSelectors()
	initClasses()

	w := &Window{
		width:           config.Width,
		height:          config.Height,
		screenChangedCh: make(chan struct{}, 1),
	}

	// Calculate style mask
	var styleMask NSWindowStyleMask
	if config.Frameless {
		styleMask = NSWindowStyleMaskBorderless
		if config.Resizable {
			styleMask |= NSWindowStyleMaskResizable
		}
	} else {
		styleMask = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable
		if config.Resizable {
			styleMask |= NSWindowStyleMaskResizable
		}
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
		w.cachedTitle = config.Title
		title := NewNSString(config.Title)
		if title != nil {
			nsWindow.SendPtr(selectors.setTitle, title.ID().Ptr())
			title.Release()
		}
	}

	// Create custom GoGPUView (ADR-015: prevents macOS system beep on key press).
	// Every enterprise framework (Qt6, Chromium, Flutter, GLFW, SDL3) uses a custom
	// NSView subclass that overrides keyDown:/doCommandBySelector: to prevent NSBeep.
	goGPUView, viewErr := CreateGoGPUView(MakeRect(0, 0, CGFloat(config.Width), CGFloat(config.Height)))
	if viewErr != nil {
		// Fallback to stock contentView if custom class registration fails.
		w.contentView = nsWindow.Send(selectors.contentView)
		if w.contentView.IsNil() {
			return nil, ErrViewCreationFailed
		}
	} else {
		nsWindow.SendPtr(selectors.setContentView, goGPUView.Ptr())
		w.contentView = goGPUView
	}

	// Set tabbing mode (macOS 10.12+).
	// Values match NSWindowTabbingMode directly (0=Auto, 1=Preferred, 2=Disallowed).
	nsWindow.SendUint(selectors.setTabbingMode, uint64(config.TabbingMode))
	if config.TabbingIdentifier != "" {
		tabID := NewNSString(config.TabbingIdentifier)
		if tabID != nil {
			nsWindow.SendPtr(selectors.setTabbingIdentifier, tabID.ID().Ptr())
			tabID.Release()
		}
	}

	// Enable native fullscreen support (green button / toggleFullScreen:).
	// Must be set before makeKeyAndOrderFront.
	nsWindow.SendUint(
		selectors.setCollectionBehavior,
		uint64(NSWindowCollectionBehaviorFullScreenPrimary),
	)

	// Enable mouse events
	nsWindow.SendBool(selectors.setAcceptsMouseMovedEvents, true)

	// Don't release when closed (we manage lifecycle)
	nsWindow.SendBool(selectors.setReleasedWhenClosed, false)

	// Center window on screen
	nsWindow.Send(selectors.center)

	delegate, err := CreateWindowDelegate(w)
	if err == nil {
		delegate.Send(selectors.retain)
		nsWindow.SendPtr(selectors.setDelegate, delegate.Ptr())
		w.delegate = delegate
	}

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

	w.cachedTitle = title
	nsTitle := NewNSString(title)
	if nsTitle != nil {
		w.nsWindow.SendPtr(selectors.setTitle, nsTitle.ID().Ptr())
		// Keep the injected text field in sync when right-alignment is active.
		if !w.titleTextField.IsNil() {
			w.titleTextField.SendPtr(selectors.setStringValue, nsTitle.ID().Ptr())
		}
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

// Handle returns the gpu window handle (NSWindow pointer) for surface creation.
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
		w.metalLayer = 0
	}

	// Remove delegate properly
	if w.delegate != 0 {
		w.nsWindow.SendPtr(selectors.setDelegate, 0) // setDelegate:nil
		SetAssociatedObject(w.delegate, unsafe.Pointer(&delegateAssociatedKey), nil, 0)
		w.delegate.Send(selectors.release)
		w.delegate = 0
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

	// Layer-hosting mode: setLayer: FIRST, then setWantsLayer:YES.
	// Apple docs require this exact order to enter layer-hosting mode.
	// Reversing the order creates layer-backed mode where AppKit manages
	// the layer lifecycle — on macOS 15+ this causes the Metal content
	// to composite above the title bar, making it invisible.
	w.contentView.SendPtr(selectors.setLayer, layer.Ptr())
	w.contentView.SendBool(selectors.setWantsLayer, true)

	w.metalLayer = layer
}

// MetalLayer returns the attached CAMetalLayer, if any.
func (w *Window) MetalLayer() ID {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.metalLayer
}

// BackingScaleFactor returns the window's backing scale factor.
// On Retina displays this is 2.0, on standard displays 1.0.
// Returns 1.0 if the window is nil or the query fails.
func (w *Window) BackingScaleFactor() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return 1.0
	}

	scale := w.nsWindow.GetDouble(selectors.backingScaleFactor)
	if scale <= 0 {
		return 1.0
	}
	return scale
}

// UpdateSize updates the cached size from the actual window size.
// Stores LOGICAL size in platform points (not physical pixels).
// Use FramebufferSize() for physical pixel dimensions.
func (w *Window) UpdateSize() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.contentView.IsNil() {
		return
	}

	// Get bounds of content view in logical points (macOS Cocoa points).
	// On Retina displays, 800x600 points maps to 1600x1200 physical pixels,
	// but we store the logical size here. Physical size is derived on demand.
	bounds := w.contentView.GetRect(selectors.bounds)
	w.width = int(bounds.Size.Width)
	w.height = int(bounds.Size.Height)
}

// FramebufferSize returns the GPU framebuffer size in physical device pixels.
// On Retina displays this is LogicalSize * BackingScaleFactor.
func (w *Window) FramebufferSize() (width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.contentView.IsNil() {
		return w.width, w.height
	}

	bounds := w.contentView.GetRect(selectors.bounds)
	scale := 1.0
	if !w.nsWindow.IsNil() {
		s := w.nsWindow.GetDouble(selectors.backingScaleFactor)
		if s > 0 {
			scale = s
		}
	}
	return int(float64(bounds.Size.Width) * scale), int(float64(bounds.Size.Height) * scale)
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

// InLiveResize returns true while the user is dragging a resize handle.
// Thread-safe; reads an atomic flag set by the NSWindowDelegate callbacks.
func (w *Window) InLiveResize() bool {
	return w.inLiveResize.Load()
}

// StartLiveResize marks the beginning of a live resize operation.
// Called by the windowWillStartLiveResize: NSWindowDelegate method.
func (w *Window) StartLiveResize() {
	w.inLiveResize.Store(true)
}

// EndLiveResize marks the end of a live resize operation and wakes the event
// loop so a final EventResize is emitted with the settled dimensions.
// Called by the windowDidEndLiveResize: NSWindowDelegate method.
func (w *Window) EndLiveResize() {
	w.inLiveResize.Store(false)
	WakeEventLoop()
}

// SetLiveResizeHook installs a callback that fires on every windowDidResize:
// notification, including those generated inside AppKit's live-resize modal loop.
// The hook should trigger a render frame so the GPU surface stays in sync during
// resize and macOS does not stretch the old frame to fill the new window size.
func (w *Window) SetLiveResizeHook(fn func()) {
	w.mu.Lock()
	w.liveResizeHook = fn
	w.mu.Unlock()
}

// liveResizeHookValue returns the current hook under mu.
func (w *Window) liveResizeHookValue() func() {
	w.mu.Lock()
	fn := w.liveResizeHook
	w.mu.Unlock()
	return fn
}

// NSTextAlignment values used when injecting a custom title NSTextField.
const (
	nsTextAlignmentLeft   = 0
	nsTextAlignmentCenter = 1
	nsTextAlignmentRight  = 2
)

// SetHeaderAlignment adjusts the native title bar to reflect the requested
// alignment. alignment values: 0 = center (default), 1 = left, 2 = right.
//
// Left:   FullSizeContentView + transparent title bar; native title stays visible
// and macOS positions it after the traffic-light buttons (left side).
// Right:  FullSizeContentView + transparent title bar; native title is hidden and
// an NSTextField is injected into the title bar view, right-aligned.
// Center: standard opaque title bar; native title hidden; a centered NSTextField
// is injected to guarantee true horizontal centering on all macOS versions.
func (w *Window) SetHeaderAlignment(alignment int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	// Remove any previously injected custom title text field.
	if !w.titleTextField.IsNil() {
		w.titleTextField.Send(selectors.removeFromSuperview)
		w.titleTextField.Send(selectors.release)
		w.titleTextField = 0
	}

	current := NSWindowStyleMask(w.nsWindow.GetUint64(selectors.styleMask))
	hasFullSize := current&NSWindowStyleMaskFullSizeContentView != 0
	switch alignment {
	case 1: // HeaderAlignLeft
		if !hasFullSize {
			w.nsWindow.SendUint(selectors.setStyleMask, uint64(current|NSWindowStyleMaskFullSizeContentView))
		}
		w.nsWindow.SendBool(selectors.setTitlebarAppearsTransparent, true)
		w.nsWindow.SendUint(selectors.setTitleVisibility, uint64(NSWindowTitleVisible))
	case 2: // HeaderAlignRight
		if !hasFullSize {
			w.nsWindow.SendUint(selectors.setStyleMask, uint64(current|NSWindowStyleMaskFullSizeContentView))
		}
		w.nsWindow.SendBool(selectors.setTitlebarAppearsTransparent, true)
		w.nsWindow.SendUint(selectors.setTitleVisibility, uint64(NSWindowTitleHidden))
		w.injectTitleTextField(nsTextAlignmentRight)
	default: // HeaderAlignCenter (0)
		// Only touch the style mask when FullSizeContentView is actually present;
		// calling setStyleMask: with an unchanged value triggers a title re-layout
		// that macOS renders left-aligned instead of centered.
		if hasFullSize {
			w.nsWindow.SendUint(selectors.setStyleMask, uint64(current&^NSWindowStyleMaskFullSizeContentView))
		}
		// Inject a centered NSTextField rather than relying on macOS default title
		// positioning, which varies across OS versions and window configurations.
		w.nsWindow.SendBool(selectors.setTitlebarAppearsTransparent, false)
		w.nsWindow.SendUint(selectors.setTitleVisibility, uint64(NSWindowTitleHidden))
		w.injectTitleTextField(nsTextAlignmentCenter)
	}
	w.alignment = alignment
}

// injectTitleTextField adds an NSTextField to the title bar's button container
// (the view that holds the traffic-light buttons) with the given NSTextAlignment.
// textAlignment: 0 = left, 1 = center, 2 = right. Called with w.mu held.
func (w *Window) injectTitleTextField(textAlignment uint64) {
	// Navigate to the view that owns the traffic-light buttons: get the close
	// button (NSWindowCloseButton = 0) then its superview. This view has a
	// transparent background so adding our label behind the buttons (with
	// NSWindowBelow+nil) keeps text visible while clicks reach the buttons.
	closeBtn := w.nsWindow.SendUint(selectors.standardWindowButton, 0)
	if closeBtn.IsNil() {
		return
	}
	tbView := closeBtn.Send(selectors.superview)
	if tbView.IsNil() {
		return
	}

	// Compute inset past the traffic-light buttons. NSWindowZoomButton (2) is
	// the rightmost button; its right edge + a gap gives the safe left margin.
	tbBounds := tbView.GetRect(selectors.bounds)
	leftInset := CGFloat(0)
	zoomBtn := w.nsWindow.SendUint(selectors.standardWindowButton, 2)
	if !zoomBtn.IsNil() {
		zf := zoomBtn.GetRect(selectors.frame)
		leftInset = zf.Origin.X + zf.Size.Width + 8
	}

	// Anchor the text field's vertical position to the close button's frame.
	// NSTitlebarView uses flipped coordinates (Y=0 at top), so using Y=0 with
	// full title-bar height places NSTextField's text at the very top edge.
	// The close button's Y is already correctly centered by AppKit; mirroring
	// it keeps our label aligned with the traffic-light buttons.
	cf := closeBtn.GetRect(selectors.frame)
	textH := cf.Size.Height + 4 // 4pt taller than button keeps text un-clipped
	textY := cf.Origin.Y + (cf.Size.Height-textH)/2

	// For center alignment use a symmetric frame so text centers at exactly W/2.
	// For right alignment the frame extends to the right edge (left-inset only).
	var frame NSRect
	if textAlignment == nsTextAlignmentCenter {
		frame = MakeRect(leftInset, textY, tbBounds.Size.Width-2*leftInset, textH)
	} else {
		frame = MakeRect(leftInset, textY, tbBounds.Size.Width-leftInset, textH)
	}

	tf := classes.NSTextField.Send(selectors.alloc)
	tf = tf.SendRect(selectors.initWithFrame, frame)
	if tf.IsNil() {
		return
	}

	// Configure as a non-editable, transparent label.
	tf.SendBool(selectors.setEditable, false)
	tf.SendBool(selectors.setBezeled, false)
	tf.SendBool(selectors.setDrawsBackground, false)
	tf.SendUint(selectors.setAlignment, textAlignment)

	// Apply the standard title bar font (matches system window title).
	titleFont := ID(classes.NSFont).SendDouble(selectors.titleBarFontOfSize, 13.0)
	if !titleFont.IsNil() {
		tf.SendPtr(selectors.setFont, titleFont.Ptr())
	}

	// Use the system label color so the title is readable in both light and dark mode.
	labelColor := classes.NSColor.Send(selectors.labelColor)
	if !labelColor.IsNil() {
		tf.SendPtr(selectors.setTextColor, labelColor.Ptr())
	}

	// Set the current title.
	if w.cachedTitle != "" {
		nsTitle := NewNSString(w.cachedTitle)
		if nsTitle != nil {
			tf.SendPtr(selectors.setStringValue, nsTitle.ID().Ptr())
			nsTitle.Release()
		}
	}

	// NSViewWidthSizable (2): width tracks parent width as the window resizes.
	tf.SendUint(selectors.setAutoresizingMask, 2)

	// Insert behind all existing subviews (traffic-light buttons) so mouse events
	// reach the buttons. The button container has a transparent background, so the
	// label remains visible. NSWindowBelow = -1 as uintptr = ^uintptr(0).
	tbView.Send5Ptr(selectors.addSubviewPositionedRelativeTo, tf.Ptr(), ^uintptr(0), 0)
	w.titleTextField = tf
}

// SetStyleMask sets the window's style mask.
func (w *Window) SetStyleMask(mask NSWindowStyleMask) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendUint(selectors.setStyleMask, uint64(mask))
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

// SetCollectionBehavior sets the window's collection behavior flags.
// Used to enable fullscreen support via NSWindowCollectionBehaviorFullScreenPrimary.
func (w *Window) SetCollectionBehavior(behavior NSWindowCollectionBehavior) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendUint(selectors.setCollectionBehavior, uint64(behavior))
}

// ToggleFullScreen toggles native macOS fullscreen mode.
// Sends the toggleFullScreen: selector to the NSWindow.
func (w *Window) ToggleFullScreen() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendPtr(selectors.toggleFullScreen, 0)
}

// IsFullScreen returns true if the window is in native macOS fullscreen mode.
// Checks whether NSWindowStyleMaskFullScreen is set in the window's style mask.
func (w *Window) IsFullScreen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return false
	}

	mask := uintptr(w.nsWindow.Send(selectors.styleMask))
	return mask&uintptr(NSWindowStyleMaskFullScreen) != 0
}

// SetMinSize sets the minimum window frame size in logical points.
// Use 0 for both to remove the minimum constraint (system default).
func (w *Window) SetMinSize(width, height float64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	w.nsWindow.SendSize(selectors.setMinSize, MakeSize(CGFloat(width), CGFloat(height)))
}

// SetMaxSize sets the maximum window frame size in logical points.
// Use 0 for both to remove the maximum constraint (use a very large value).
func (w *Window) SetMaxSize(width, height float64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.nsWindow.IsNil() {
		return
	}

	var size NSSize
	if width == 0 && height == 0 {
		// CGFloat max ≈ 3.4e38 — effectively unconstrained
		const cgFloatMax = CGFloat(3.40282346638528859811704183484516925440e+38)
		size = MakeSize(cgFloatMax, cgFloatMax)
	} else {
		size = MakeSize(CGFloat(width), CGFloat(height))
	}
	w.nsWindow.SendSize(selectors.setMaxSize, size)
}

func (w *Window) SetOnClose(fn func() bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onClose = fn
}

// ScreenChangedCh returns a receive-only channel that is signaled when the
// window moves to a display with a different backing scale factor.
// The channel has capacity 1; multiple rapid transitions are coalesced.
func (w *Window) ScreenChangedCh() <-chan struct{} {
	return w.screenChangedCh
}
