package gogpu

import (
	"fmt"
	"sync"

	"github.com/gogpu/gogpu/internal/platform"
)

// WindowID uniquely identifies a window. Zero is invalid.
type WindowID = platform.WindowID

// Window represents an application window with its own rendering surface.
// Each Window tracks per-window callbacks and maintains a reference to the
// underlying platform window and GPU surface state.
//
// In the current implementation, only the primary window (created by Run)
// is supported. Multi-window rendering will be enabled when platforms
// implement PlatformManager.
type Window struct {
	id       WindowID
	config   Config
	surface  *windowSurface
	platform platform.Platform // underlying platform window (legacy adapter)

	// Per-window callbacks
	onDraw   func(*Context)
	onResize func(int, int)
	onClose  func() bool // return false to reject close
	visible  bool
}

// ID returns the unique identifier for this window.
func (w *Window) ID() WindowID {
	return w.id
}

// SetOnDraw sets the per-window draw callback.
// For the primary window, this also updates the App-level onDraw callback
// to maintain backward compatibility.
func (w *Window) SetOnDraw(fn func(*Context)) {
	w.onDraw = fn
}

// SetOnResize sets the per-window resize callback.
func (w *Window) SetOnResize(fn func(int, int)) {
	w.onResize = fn
}

// SetOnClose sets the close request callback.
// Return false from the callback to reject the close request.
func (w *Window) SetOnClose(fn func() bool) {
	w.onClose = fn
}

// Close requests this window to close.
func (w *Window) Close() {
	if w.platform != nil {
		w.platform.CloseWindow()
	}
}

// Size returns the logical window size in platform points (DIP).
func (w *Window) Size() (int, int) {
	if w.platform != nil {
		return w.platform.LogicalSize()
	}
	return w.config.Width, w.config.Height
}

// PhysicalSize returns the GPU framebuffer size in device pixels.
func (w *Window) PhysicalSize() (int, int) {
	if w.platform != nil {
		return w.platform.PhysicalSize()
	}
	return w.config.Width, w.config.Height
}

// Visible returns whether the window is currently visible and rendering.
func (w *Window) Visible() bool {
	return w.visible
}

// WindowManager tracks all open windows in the application.
// Thread-safe: all methods are protected by a read-write mutex.
type WindowManager struct {
	mu      sync.RWMutex
	windows map[WindowID]*Window
	order   []WindowID // insertion order for deterministic render iteration
	focused WindowID   // currently focused window, zero if none
}

// newWindowManager creates a new empty WindowManager.
func newWindowManager() *WindowManager {
	return &WindowManager{
		windows: make(map[WindowID]*Window, 8),
	}
}

// add registers a window in the manager.
// If no window has focus yet, the new window receives focus.
func (wm *WindowManager) add(w *Window) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.windows[w.id] = w
	wm.order = append(wm.order, w.id)
	if wm.focused == 0 {
		wm.focused = w.id
	}
}

// remove unregisters a window from the manager.
// If the removed window had focus, focus moves to the first remaining window.
func (wm *WindowManager) remove(id WindowID) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	delete(wm.windows, id)
	for i, wid := range wm.order {
		if wid == id {
			wm.order = append(wm.order[:i], wm.order[i+1:]...)
			break
		}
	}
	// If focused window was removed, refocus to the first remaining window.
	if wm.focused == id {
		wm.focused = 0
		if len(wm.order) > 0 {
			wm.focused = wm.order[0]
		}
	}
}

// get returns the window with the given ID, or nil if not found.
func (wm *WindowManager) get(id WindowID) *Window {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.windows[id]
}

// count returns the number of tracked windows.
func (wm *WindowManager) count() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return len(wm.windows)
}

// focusedWindow returns the currently focused window, or nil if none.
func (wm *WindowManager) focusedWindow() *Window {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if wm.focused == 0 {
		return nil
	}
	return wm.windows[wm.focused]
}

// setFocus changes the focused window.
func (wm *WindowManager) setFocus(id WindowID) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.windows[id]; ok {
		wm.focused = id
	}
}

// NewWindow creates a new window. Returns an error because multi-window
// rendering is not yet supported -- platforms must implement PlatformManager
// before additional windows can be created.
//
// The primary window (created by Run) is always available via PrimaryWindow().
func (a *App) NewWindow(_ Config) (*Window, error) {
	return nil, fmt.Errorf("gogpu: multi-window not yet implemented; use Run() for single-window apps")
}

// PrimaryWindow returns the primary application window.
// This is the window created by Run() and is always available after
// the renderer has been initialized.
//
// Returns nil if called before Run().
func (a *App) PrimaryWindow() *Window {
	return a.primaryWindow
}

// WindowCount returns the number of open windows.
// Returns 0 if called before Run().
func (a *App) WindowCount() int {
	if a.windowManager == nil {
		return 0
	}
	return a.windowManager.count()
}
