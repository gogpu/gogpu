package gogpu

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/wgpu"
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
	id         WindowID
	config     Config
	surface    *windowSurface
	platWindow platform.PlatformWindow // underlying platform window

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
	if w.platWindow != nil {
		w.platWindow.Close()
	}
}

// Size returns the logical window size in platform points (DIP).
func (w *Window) Size() (int, int) {
	if w.platWindow != nil {
		return w.platWindow.LogicalSize()
	}
	return w.config.Width, w.config.Height
}

// PhysicalSize returns the GPU framebuffer size in device pixels.
func (w *Window) PhysicalSize() (int, int) {
	if w.platWindow != nil {
		return w.platWindow.PhysicalSize()
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
// Used by VSync strategy: focused window = Fifo, others = Immediate (ADR-010).
// Called when platform sends focus events (EventFocus — not yet implemented).
func (wm *WindowManager) focusedWindow() *Window { //nolint:unused // VSync focus switching, wired when EventFocus is added
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if wm.focused == 0 {
		return nil
	}
	return wm.windows[wm.focused]
}

// setFocus changes the focused window and adjusts VSync strategy.
// Called when platform sends focus events (EventFocus — not yet implemented).
func (wm *WindowManager) setFocus(id WindowID) { //nolint:unused // VSync focus switching, wired when EventFocus is added
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.windows[id]; ok {
		wm.focused = id
	}
}

// NewWindow creates a new secondary window with its own rendering surface.
// The window is registered in the WindowManager and participates in the
// multi-window frame loop. Set OnDraw via Window.SetOnDraw() to render content.
//
// Must be called after Run() has started (the renderer must be initialized).
// The GPU surface is created on the render thread. Secondary windows default
// to VSync off (Immediate present mode) following ADR-010 strategy.
//
// The primary window (created by Run) is always available via PrimaryWindow().
func (a *App) NewWindow(config Config) (*Window, error) {
	if a.manager == nil || a.renderer == nil {
		return nil, fmt.Errorf("gogpu: NewWindow called before Run()")
	}

	// Create platform window via PlatformManager.
	platWindow, err := a.manager.CreateWindow(platform.Config{
		Title:      config.Title,
		Width:      config.Width,
		Height:     config.Height,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
		Frameless:  config.Frameless,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: create window: %w", err)
	}

	// Create GPU surface for the new window on the render thread.
	// GPU operations are thread-bound; surface creation must happen there.
	var surface *wgpu.Surface
	var surfaceErr error
	a.renderLoop.RunOnRenderThreadVoid(func() {
		displayHandle, windowHandle := platWindow.GetHandle()
		surface, surfaceErr = a.renderer.instance.CreateSurface(displayHandle, windowHandle)
	})
	if surfaceErr != nil {
		platWindow.Destroy()
		return nil, fmt.Errorf("gogpu: create surface: %w", surfaceErr)
	}

	// Create windowSurface for this window.
	ws := &windowSurface{
		renderer: a.renderer,
		surface:  surface,
		format:   a.renderer.primary.format,
		vsync:    false, // Secondary windows: Immediate (ADR-010 VSync strategy)
	}

	// Configure surface with initial dimensions on the render thread.
	a.renderLoop.RunOnRenderThreadVoid(func() {
		pw, ph := platWindow.PhysicalSize()
		if pw > 0 && ph > 0 {
			ws.width = uint32(pw)  //nolint:gosec // G115: validated positive above
			ws.height = uint32(ph) //nolint:gosec // G115: validated positive above
			if cfgErr := ws.configure(a.renderer.device, a.renderer.adapter); cfgErr != nil {
				slog.Warn("gogpu: failed to configure secondary surface", "err", cfgErr)
			} else {
				ws.configured = true
			}
		}
	})

	// Create Window and register in WindowManager.
	w := &Window{
		id:         platWindow.ID(),
		config:     config,
		surface:    ws,
		platWindow: platWindow,
		visible:    true,
	}
	a.windowManager.add(w)

	// Request a redraw so the new window gets its first frame.
	if a.invalidator != nil {
		a.invalidator.Invalidate()
	}

	return w, nil
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

// closeSecondaryWindow removes a secondary window and releases its GPU and
// platform resources. The GPU surface is released on the render thread.
// Does nothing if the window is the primary window (use Quit() instead).
func (a *App) closeSecondaryWindow(id WindowID) {
	if a.primaryWindow != nil && id == a.primaryWindow.id {
		return // Primary window closes via a.running = false
	}

	w := a.windowManager.get(id)
	if w == nil {
		return
	}

	a.windowManager.remove(id)

	// Release GPU surface on the render thread.
	if w.surface != nil {
		a.renderLoop.RunOnRenderThreadVoid(func() {
			w.surface.destroy()
		})
	}

	// Destroy the platform window.
	if w.platWindow != nil {
		w.platWindow.Destroy()
	}

	w.visible = false
}
