package gogpu

import (
	"io"
	"runtime"
	"time"

	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gogpu/internal/thread"
	"github.com/gogpu/gpucontext"
)

type appRenderLoop interface {
	RunOnRenderThreadVoid(fn func())
	Stop()
	RequestResize(w, h uint32)
	ConsumePendingResize() (uint32, uint32, bool)
}

// App is the main application type.
// It manages the window, rendering, and application lifecycle.
//
// The App uses a multi-thread architecture for maximum responsiveness:
//   - Main thread: Window events (Win32/Cocoa/X11 message pump)
//   - Render thread: All GPU operations (device, swapchain, commands)
//
// This separation ensures the window stays responsive during heavy GPU
// operations like swapchain recreation.
type App struct {
	config     Config
	manager    platform.PlatformManager // process-level (multi-window)
	platWindow platform.PlatformWindow  // primary window (per-window ops)
	renderer   *Renderer

	// Multi-thread rendering
	renderLoop appRenderLoop

	// User callbacks
	onDraw            func(*Context)
	onUpdate          func(float64) // delta time in seconds
	onResize          func(int, int)
	onClose           func() // called before renderer destruction
	onAnyWindowClosed func(WindowID)

	// State
	running   bool
	lastFrame time.Time

	// Event-driven rendering
	invalidator *Invalidator
	animations  *AnimationController

	// Event source for gpucontext integration
	eventSource *eventSourceAdapter

	// Input state for Ebiten-style polling (KeyJustPressed, etc.)
	inputState *input.State

	// Resource tracker for automatic GPU resource cleanup on shutdown.
	tracker *resourceTracker

	// Multi-window management (Phase 3 of ADR-010).
	// windowManager tracks all open windows; primaryWindow is the first window
	// created by Run(). The existing frame loop uses a.renderer (single-window
	// path); the multi-window frame loop will iterate windowManager when
	// platforms implement PlatformManager.
	windowManager *WindowManager
	primaryWindow *Window
}

// NewApp creates a new application with the given configuration.
func NewApp(config Config) *App {
	return &App{
		config: config,
	}
}

// OnDraw sets the callback for rendering each frame.
// The Context is only valid during the callback.
// If the primary window has been created (after Run starts), its draw
// callback is also updated to stay in sync.
func (a *App) OnDraw(fn func(*Context)) *App {
	a.onDraw = fn
	if a.primaryWindow != nil {
		a.primaryWindow.onDraw = fn
	}
	return a
}

// OnUpdate sets the callback for logic updates each frame.
// The parameter is delta time in seconds since the last frame.
func (a *App) OnUpdate(fn func(float64)) *App {
	a.onUpdate = fn
	return a
}

// OnResize sets the callback for window resize events.
// If the primary window has been created (after Run starts), its resize
// callback is also updated to stay in sync.
func (a *App) OnResize(fn func(width, height int)) *App {
	a.onResize = fn
	if a.primaryWindow != nil {
		a.primaryWindow.onResize = fn
	}
	return a
}

// OnClose sets the callback invoked when the application is shutting down,
// before the GPU renderer is destroyed. Use this to release GPU resources
// (e.g., ggcanvas.Canvas) that depend on the renderer being alive.
//
// The callback runs on the render thread.
func (a *App) OnClose(fn func()) *App {
	a.onClose = fn
	return a
}

// OnAnyWindowClosed registers a callback invoked after a window has been
// destroyed (secondary) or when the primary window closes.
// The callback receives the internal window ID.
// Use it for app‑level observations like updating a tab count.
func (a *App) OnAnyWindowClosed(fn func(WindowID)) *App {
	a.onAnyWindowClosed = fn
	return a
}

// TrackResource registers an io.Closer for automatic cleanup during shutdown.
// Tracked resources are closed in LIFO (reverse) order after WaitIdle and
// before the renderer is destroyed, so the GPU device is still alive.
//
// Use this instead of OnClose for automatic resource lifecycle management.
// Resources that implement io.Closer (like ggcanvas.Canvas) can be tracked.
//
// Safe to call from any goroutine. If called after shutdown, the resource
// is closed immediately.
//
// Example:
//
//	canvas, _ := ggcanvas.New(provider, 800, 600)
//	app.TrackResource(canvas)
//	// canvas.Close() will be called automatically on shutdown
func (a *App) TrackResource(c io.Closer) {
	if a.tracker == nil {
		a.tracker = &resourceTracker{}
	}
	a.tracker.Track(c, "")
}

// UntrackResource removes a resource from automatic cleanup tracking.
// Call this when you close a resource manually before shutdown to prevent
// double-close.
func (a *App) UntrackResource(c io.Closer) {
	if a.tracker == nil {
		return
	}
	a.tracker.Untrack(c)
}

// Compile-time check that App implements ResourceTracker.
var _ ResourceTracker = (*App)(nil)

// Run starts the application main loop with multi-thread architecture.
// This function blocks until the application quits.
//
// The main loop uses a professional multi-thread pattern (Ebiten/Gio):
//   - Main thread: Window events only (keeps window responsive)
//   - Render thread: All GPU operations (device, swapchain, commands)
//
// This ensures the window never shows "Not Responding" during heavy
// GPU operations like swapchain recreation (vkDeviceWaitIdle).
func (a *App) Run() error {
	// Lock main goroutine to OS main thread.
	// Required for Win32/Cocoa window operations.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Initialize platform manager (process-level) — must be on main thread.
	platform.SetLogger(slogger())
	a.manager = platform.NewManager()
	if err := a.manager.Init(); err != nil {
		return err
	}
	defer a.manager.Destroy()

	// Create primary platform window.
	platWindow, err := a.manager.CreateWindow(platform.Config{
		Title:      a.config.Title,
		Width:      a.config.Width,
		Height:     a.config.Height,
		Resizable:  a.config.Resizable,
		Fullscreen: a.config.Fullscreen,
		Frameless:  a.config.Frameless,
	})
	if err != nil {
		return err
	}
	defer platWindow.Destroy()

	// Store the primary platform window for per-window operations.
	a.platWindow = platWindow

	// Ensure input subsystems exist. Both EventSource() and Input() use
	// lazy init so callers can register callbacks before Run(). We must
	// NOT overwrite instances that were already created — UI frameworks
	// register callbacks on the EventSource obtained before Run().
	_ = a.Input()       // ensures a.inputState is initialized
	_ = a.EventSource() // ensures a.eventSource is initialized

	// Enable rendering during Win32 modal drag/resize loop.
	//
	// On Windows, DefWindowProc enters a modal message loop during window
	// drag/resize that blocks our main loop entirely. A WM_TIMER (~60fps)
	// fires inside the modal loop to invoke this callback, which runs the
	// same update+render cycle as the normal main loop.
	//
	// This callback runs on the main thread (same as the normal loop),
	// preserving serialization between onUpdate and onDraw — no data races.
	//
	// On macOS/Linux this is a no-op (those platforms have no modal loops).
	//
	// Future: An independent render thread running on its own schedule
	// would eliminate this callback entirely. See ROADMAP.md for details.
	a.platWindow.SetModalFrameCallback(a.modalFrameTick)

	// Create render loop with dedicated render thread
	a.renderLoop = thread.NewRenderLoop()
	defer a.renderLoop.Stop()

	// Initialize renderer on render thread (all GPU operations must be on same thread)
	var initErr error
	a.renderLoop.RunOnRenderThreadVoid(func() {
		a.renderer, initErr = newRenderer(a.platWindow, a.config.Backend, a.config.GraphicsAPI, a.config.VSync, a.config.PowerPreference)
	})
	if initErr != nil {
		return initErr
	}
	defer func() {
		// Shutdown sequence (all on render thread for GPU safety):
		// 1. WaitIdle — ensure all GPU work completes
		// 2. DrainDeferredDestroys — release GC-enqueued resources
		// 3. tracker.CloseAll() — auto-tracked resources (LIFO)
		// 4. onClose callback — manual cleanup (legacy pattern)
		// 5. Renderer.Destroy() — release GPU device
		a.renderLoop.RunOnRenderThreadVoid(func() {
			a.renderer.WaitForGPU()
			a.renderer.DrainDeferredDestroys()
			if a.tracker != nil {
				_ = a.tracker.CloseAll()
			}
			if a.onClose != nil {
				a.onClose()
			}
		})
		a.renderLoop.RunOnRenderThreadVoid(func() {
			a.renderer.Destroy()
		})
	}()

	// Register the primary window in the WindowManager.
	// This validates the multi-window architecture end-to-end with the
	// existing single-window case. The frame loop still renders via
	// a.renderer (proven path); WindowManager tracks the window for
	// future multi-window iteration.
	a.windowManager = newWindowManager()

	// Allocate internal ID from pool
	internalID := a.windowManager.allocate()
	a.primaryWindow = &Window{
		id:         internalID,
		platformID: platWindow.ID(),
		config:     a.config,
		surface:    a.renderer.primary,
		platWindow: a.platWindow,
		onDraw:     a.onDraw,
		onResize:   a.onResize,
		visible:    true,
	}
	a.windowManager.add(a.primaryWindow)

	// Main loop with three rendering modes (ADR-023):
	//   1. IDLE: No activity — block on OS events (0% CPU, <1ms response)
	//   2. ANIMATING: StartAnimation() — loop active, onUpdate every tick,
	//      OnDraw ONLY when RequestRedraw() called (demand-driven, <1% GPU)
	//   3. CONTINUOUS: ContinuousRender=true — OnDraw every VSync (game loop)
	a.running = true
	a.lastFrame = time.Now()
	a.invalidator = newInvalidator(a.manager.WakeUp)
	a.animations = &AnimationController{}
	a.invalidator.Invalidate() // Request initial frame

	for a.running && !a.platWindow.ShouldClose() {
		// Three-mode state detection (ADR-023)
		continuousRender := a.config.ContinuousRender
		animating := a.animations.IsAnimating()
		invalidated := a.invalidator.Consume()

		if !continuousRender && !animating && !invalidated {
			// IDLE: block on OS events (0% CPU, <1ms response)
			a.manager.WaitEvents()
		}

		// Process all pending platform events.
		// Events may call RequestRedraw() which sets invalidated for next check.
		a.processEventsMultiThread()

		// Check if invalidation arrived during event processing
		// (e.g., resize or UI framework responding to hover/click).
		if a.invalidator.Consume() {
			invalidated = true
		}

		// Calculate delta time
		now := time.Now()
		deltaTime := now.Sub(a.lastFrame).Seconds()
		a.lastFrame = now

		// Clamp deltaTime after long idle (WaitEvents can block for seconds/minutes).
		// Without clamping, physics and animations would jump on the first frame.
		// 66ms = ~15 FPS minimum, a safe upper bound for a single frame step.
		if deltaTime > 0.066 {
			deltaTime = 0.066
		}

		// Update input state for next frame (Ebiten-style polling)
		// This must be called before onUpdate so JustPressed/JustReleased work correctly
		if a.inputState != nil {
			a.inputState.Update()
		}

		// onUpdate: ALWAYS when loop is active (ANIMATING + CONTINUOUS).
		// UI frameworks tick animations, process signals, run layout here.
		// If something changed, they call RequestRedraw() → invalidated = true.
		if a.onUpdate != nil {
			a.onUpdate(deltaTime)
		}

		// Check if onUpdate triggered RequestRedraw (UI spinner, animation tick).
		if a.invalidator.Consume() {
			invalidated = true
		}

		// OnDraw: CONTINUOUS = every VSync (games), otherwise only when dirty.
		// ANIMATING mode: onUpdate runs every tick, OnDraw only on RequestRedraw.
		// Lazy acquire inside: if OnDraw doesn't draw, no swapchain acquire.
		if continuousRender || invalidated {
			a.renderFrameMultiThread()
		}
	}

	return nil
}

// processEventsMultiThread handles platform events with multi-thread pattern.
// Resize events are deferred to the render thread via RequestResize.
// Events that require visual update (resize, focus) call RequestRedraw()
// explicitly — the render loop renders only when invalidated or continuous.
// This matches the winit/Flutter/Qt pattern: platform layer delivers events,
// handlers decide whether to invalidate, render loop never guesses.
func (a *App) processEventsMultiThread() {
	// Collect all events first, then process.
	// This allows us to coalesce resize events.
	var lastResize *platform.Event
	var events []platform.Event

	for {
		event := a.manager.PollEvents()
		if event.Type == platform.EventNone {
			break
		}
		events = append(events, event)
	}

	// Classify events by type and window.
	var secondaryResizes []platform.Event
	for i := range events {
		lastResize, secondaryResizes = a.classifyEvent(&events[i], lastResize, secondaryResizes)
	}

	// Queue primary window resize for render thread (deferred pattern).
	// Don't apply resize during modal resize loop (Windows).
	if lastResize != nil && !a.platWindow.InSizeMove() {
		// Queue PHYSICAL size for render thread (GPU surface reconfiguration)
		physW, physH := lastResize.PhysicalWidth, lastResize.PhysicalHeight
		if physW > 0 && physH > 0 {
			a.renderLoop.RequestResize(uint32(physW), uint32(physH)) //nolint:gosec // G115: validated positive
		}

		// Call user callback with LOGICAL size (what user expects for layout)
		if a.onResize != nil {
			a.onResize(lastResize.Width, lastResize.Height)
		}

		// Resize requires a render to reconfigure swapchain and repaint.
		a.RequestRedraw()
	}

	// Handle secondary window resize events.
	for i := range secondaryResizes {
		a.handleSecondaryResize(secondaryResizes[i])
	}

	// Dispatch end-of-frame events (gestures computed from pointer events)
	if a.eventSource != nil {
		a.eventSource.dispatchEndFrame()
	}
}

// handleSecondaryResize resizes a secondary window's surface on the render thread.
func (a *App) handleSecondaryResize(ev platform.Event) {
	w := a.windowManager.getByPlatformID(ev.WindowID)
	if w == nil || w.surface == nil {
		return
	}
	physW, physH := ev.PhysicalWidth, ev.PhysicalHeight
	if physW > 0 && physH > 0 {
		ws := w.surface
		a.renderLoop.RunOnRenderThreadVoid(func() {
			ws.resize(physW, physH, a.renderer.device, a.renderer.adapter)
		})
	}
	if w.onResize != nil {
		w.onResize(ev.Width, ev.Height)
	}
	a.RequestRedraw()
}

// windowFrame holds a snapshot of per-window state captured on the main thread
// for the render thread to use. This avoids accessing window/platform state
// from the render thread.
// classifyEvent routes a single platform event to the appropriate handler.
func (a *App) classifyEvent(event *platform.Event, lastResize *platform.Event, secondaryResizes []platform.Event) (*platform.Event, []platform.Event) {
	isPrimary := event.WindowID == 0 ||
		(a.primaryWindow != nil && event.WindowID == a.primaryWindow.platformID)

	switch event.Type {
	case platform.EventResize:
		if isPrimary {
			lastResize = event
		} else {
			secondaryResizes = append(secondaryResizes, *event)
		}
	case platform.EventClose:
		a.windowCloseEvent(event)
	case platform.EventFocus:
		if event.Focused {
			if w := a.windowManager.getByPlatformID(event.WindowID); w != nil {
				a.windowManager.setFocus(w.id)
			}
		}
		a.RequestRedraw()

	case platform.EventKeyDown:
		a.dispatchKeyEvent(event, true)
	case platform.EventKeyUp:
		a.dispatchKeyEvent(event, false)
	case platform.EventChar:
		a.dispatchCharEvent(event)
	case platform.EventPointerDown, platform.EventPointerUp, platform.EventPointerMove,
		platform.EventPointerEnter, platform.EventPointerLeave:
		a.dispatchPointerEvent(event)
	case platform.EventScroll:
		a.dispatchScrollEvent(event)
	}
	return lastResize, secondaryResizes
}

// dispatchKeyEvent handles EventKeyDown and EventKeyUp from the platform.
func (a *App) dispatchKeyEvent(event *platform.Event, pressed bool) {
	w := a.windowManager.getByPlatformID(event.WindowID)
	a.dispatchKeyToWindow(w, event.Key, event.Mods, pressed)
	a.dispatchKeyToEventSource(event.Key, event.Mods, pressed)
	a.dispatchKeyToInputState(event.Key, pressed)
}

func (a *App) dispatchKeyToWindow(w *Window, key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	if w == nil {
		return
	}
	if pressed && w.onKeyPress != nil {
		w.onKeyPress(key, mods)
	}
	if !pressed && w.onKeyRelease != nil {
		w.onKeyRelease(key, mods)
	}
}

func (a *App) dispatchKeyToEventSource(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	if a.eventSource == nil {
		return
	}
	if pressed {
		a.eventSource.dispatchKeyPress(key, mods)
	} else {
		a.eventSource.dispatchKeyRelease(key, mods)
	}
}

func (a *App) dispatchKeyToInputState(key gpucontext.Key, pressed bool) {
	if a.inputState == nil {
		return
	}
	inputKey := gpucontextKeyToInputKey(key)
	if inputKey != input.KeyUnknown {
		a.inputState.Keyboard().SetKey(inputKey, pressed)
	}
}

// dispatchCharEvent handles EventChar from the platform.
func (a *App) dispatchCharEvent(event *platform.Event) {
	w := a.windowManager.getByPlatformID(event.WindowID)
	if w != nil && w.onTextInput != nil {
		w.onTextInput(string(event.Char))
	}
	if a.eventSource != nil {
		a.eventSource.dispatchTextInput(string(event.Char))
	}
}

// dispatchPointerEvent handles pointer events (down/up/move/enter/leave) from the platform.
func (a *App) dispatchPointerEvent(event *platform.Event) {
	w := a.windowManager.getByPlatformID(event.WindowID)
	if w != nil && w.onPointer != nil {
		w.onPointer(event.Pointer)
	}
	if a.eventSource != nil {
		a.eventSource.dispatchPointerEvent(event.Pointer)
	}
	a.updateMouseStateFromPointer(event.Pointer)
}

// dispatchScrollEvent handles EventScroll from the platform.
func (a *App) dispatchScrollEvent(event *platform.Event) {
	w := a.windowManager.getByPlatformID(event.WindowID)
	if w != nil && w.onScroll != nil {
		w.onScroll(event.Scroll)
	}
	if a.eventSource != nil {
		a.eventSource.dispatchScrollEventDetailed(event.Scroll)
	}
	if a.inputState != nil {
		a.inputState.Mouse().SetScroll(float32(event.Scroll.DeltaX), float32(event.Scroll.DeltaY))
	}
}

// windowCloseEvent handles CloseEvent from the platform.
func (a *App) windowCloseEvent(event *platform.Event) {
	isPrimary := event.WindowID == 0 ||
		(a.primaryWindow != nil && event.WindowID == a.primaryWindow.platformID)

	if isPrimary {
		if a.primaryWindow != nil && a.primaryWindow.onClose != nil && !a.primaryWindow.onClose() {
			return
		}
		a.running = false
		a.windowManager.release(a.primaryWindow.id)
		if a.onAnyWindowClosed != nil {
			a.onAnyWindowClosed(a.primaryWindow.id)
		}
		return
	}

	// Secondary window
	w := a.windowManager.getByPlatformID(event.WindowID)
	if w != nil && w.onClose != nil && !w.onClose() {
		return
	}
	if w != nil {
		a.closeSecondaryWindow(w.id)
	}
}

type windowFrame struct {
	window *Window
	onDraw func(*Context)
	scale  float64
	physW  int
	physH  int
}

// renderFrameMultiThread renders a frame using the render thread.
// All GPU operations happen on the render thread to keep main thread responsive.
//
// When multiple windows are open, each window with an onDraw callback gets
// its own beginFrame/draw/endFrame cycle. GPU submission polling happens once
// after all windows are presented.
func (a *App) renderFrameMultiThread() {
	// Collect visible windows with their main-thread state.
	a.windowManager.mu.RLock()
	frames := make([]windowFrame, 0, len(a.windowManager.order))
	for _, id := range a.windowManager.order {
		w := a.windowManager.windows[id]
		if w == nil || !w.visible || w.onDraw == nil {
			continue
		}
		pw, ph := w.platWindow.PhysicalSize()
		if pw <= 0 || ph <= 0 {
			continue // Minimized
		}
		frames = append(frames, windowFrame{
			window: w,
			onDraw: w.onDraw,
			scale:  w.platWindow.ScaleFactor(),
			physW:  pw,
			physH:  ph,
		})
	}
	a.windowManager.mu.RUnlock()

	if len(frames) == 0 {
		return
	}

	// Execute GPU operations on render thread.
	a.renderLoop.RunOnRenderThreadVoid(func() {
		// Apply pending resize for the primary window.
		if w, h, ok := a.renderLoop.ConsumePendingResize(); ok {
			a.renderer.Resize(int(w), int(h))
		}

		// Drain deferred destroys once per frame, not per window.
		a.renderer.DrainDeferredDestroys()

		for _, frame := range frames {
			ws := frame.window.surface
			if ws == nil {
				continue
			}

			// Lazy acquire: store state for deferred beginFrame.
			// beginFrame is called on first draw call, not upfront.
			// If OnDraw produces no GPU work → no acquire, no present.
			platWin := frame.window.platWindow
			ws.prepareLazyAcquire(platWin, a.renderer.device, a.renderer.adapter)

			// Set renderer's currentSurface so draw methods target this window.
			a.renderer.currentSurface = ws

			// Call per-window draw callback.
			ctx := newContextForSurface(a.renderer, ws, frame.scale)
			frame.onDraw(ctx)

			// End frame only if beginFrame was actually called (lazy acquire fired).
			if ws.frameStarted {
				a.renderer.endFrameForSurface(ws)
			}
			ws.resetLazyState()
			a.renderer.currentSurface = nil
		}

		// Poll submissions once after all windows are presented.
		a.renderer.pollSubmissions()
	})
}

// modalFrameTick executes one update+render cycle during the Win32 modal
// drag/resize loop. Called from the WM_TIMER handler on the main thread.
//
// During modal resize, we propagate the current window size to the render
// thread so the swapchain is reconfigured to match. This prevents DWM from
// stretching the old-size frame to the new window dimensions.
//
// Note: only the swapchain is resized — the application's onResize callback
// is NOT called during modal drag. This prevents content re-centering artifacts.
// The onResize callback fires after WM_EXITSIZEMOVE via normal event processing.
func (a *App) modalFrameTick() {
	// Delta time
	now := time.Now()
	deltaTime := now.Sub(a.lastFrame).Seconds()
	a.lastFrame = now

	// Clamp deltaTime after long idle (same as main loop).
	if deltaTime > 0.066 {
		deltaTime = 0.066
	}

	// Update input state
	if a.inputState != nil {
		a.inputState.Update()
	}

	// User logic callback
	if a.onUpdate != nil {
		a.onUpdate(deltaTime)
	}

	// Propagate PHYSICAL window size to render thread for swapchain resize.
	// During modal loop, processEventsMultiThread doesn't run, so
	// RequestResize wouldn't be called otherwise.
	width, height := a.platWindow.PhysicalSize()
	if width > 0 && height > 0 {
		a.renderLoop.RequestResize(uint32(width), uint32(height)) //nolint:gosec // G115: validated positive
	}

	// Render frame on render thread (blocks until complete).
	a.renderFrameMultiThread()

	// Synchronize with compositor (DwmFlush on Windows).
	// This ensures our frame and the DWM window border update
	// appear in the same composition cycle, reducing resize lag.
	a.platWindow.SyncFrame()
}

// Quit requests the application to quit.
// The main loop will exit after completing the current frame.
func (a *App) Quit() {
	a.running = false
}

// RequestRedraw requests a frame redraw.
// In render-on-demand mode (ContinuousRender=false), this triggers a single frame render.
// In continuous mode, this has no effect as frames are rendered continuously.
// Safe to call from any goroutine.
func (a *App) RequestRedraw() {
	if a.invalidator != nil {
		a.invalidator.Invalidate()
	}
}

// StartAnimation signals that an animation is starting.
// While any animation is active, the main loop renders at VSync rate.
// Call Stop() on the returned token when the animation completes.
func (a *App) StartAnimation() *AnimationToken {
	token := a.animations.StartAnimation()
	a.RequestRedraw() // Wake up to start rendering
	return token
}

// Size returns the current window size in logical points (DIP).
// Use this for layout, UI coordinates, and user-facing dimensions.
func (a *App) Size() (width, height int) {
	if a.platWindow != nil {
		return a.platWindow.LogicalSize()
	}
	return a.config.Width, a.config.Height
}

// PhysicalSize returns the current GPU framebuffer size in device pixels.
// On Retina/HiDPI displays this is larger than Size() by ScaleFactor().
func (a *App) PhysicalSize() (width, height int) {
	if a.platWindow != nil {
		return a.platWindow.PhysicalSize()
	}
	return a.config.Width, a.config.Height
}

// ScaleFactor returns the DPI scale factor.
// 1.0 = standard (96 DPI on Windows, 72 on macOS), 2.0 = Retina/HiDPI.
// Implements gpucontext.WindowProvider.
func (a *App) ScaleFactor() float64 {
	if a.platWindow != nil {
		return a.platWindow.ScaleFactor()
	}
	return 1.0
}

// ClipboardRead reads text content from the system clipboard.
// Implements gpucontext.PlatformProvider.
func (a *App) ClipboardRead() (string, error) {
	if a.manager != nil {
		return a.manager.ClipboardRead()
	}
	return "", nil
}

// ClipboardWrite writes text content to the system clipboard.
// Implements gpucontext.PlatformProvider.
func (a *App) ClipboardWrite(text string) error {
	if a.manager != nil {
		return a.manager.ClipboardWrite(text)
	}
	return nil
}

// SetCursor changes the mouse cursor shape.
// Implements gpucontext.PlatformProvider.
func (a *App) SetCursor(cursor gpucontext.CursorShape) {
	if a.platWindow != nil {
		a.platWindow.SetCursor(int(cursor))
	}
}

// SetCursorMode sets the cursor confinement and visibility mode.
//
// Three modes are available:
//   - CursorModeNormal (0): Default — cursor is visible and moves freely.
//   - CursorModeLocked (1): Cursor is hidden and confined to the window.
//     Mouse movement is reported as relative deltas (DeltaX/DeltaY on PointerEvent).
//     Equivalent to SDL_SetRelativeMouseMode(SDL_TRUE).
//   - CursorModeConfined (2): Cursor is visible but confined to the window bounds.
//     Equivalent to SDL_SetWindowMouseGrab(SDL_TRUE).
//
// On focus loss, the cursor grab is temporarily released and re-applied on focus gain.
// On window resize while locked/confined, the clip rect is updated automatically.
//
// Platform support:
//   - Windows: Full support (ClipCursor + ShowCursor + SetCursorPos).
//   - Linux/X11: Full support (XGrabPointer + XWarpPointer + invisible cursor).
//   - Linux/Wayland: Stub (not yet implemented, requires pointer constraints protocol).
//   - macOS: Stub (not yet implemented, requires CGAssociateMouseAndMouseCursorPosition).
func (a *App) SetCursorMode(mode gpucontext.CursorMode) {
	if a.platWindow != nil {
		a.platWindow.SetCursorMode(int(mode))
	}
}

// CursorMode returns the current cursor confinement mode.
func (a *App) CursorMode() gpucontext.CursorMode {
	if a.platWindow != nil {
		return gpucontext.CursorMode(a.platWindow.CursorMode())
	}
	return gpucontext.CursorModeNormal
}

// DarkMode returns true if the system dark mode is active.
// Implements gpucontext.PlatformProvider.
func (a *App) DarkMode() bool {
	if a.manager != nil {
		return a.manager.DarkMode()
	}
	return false
}

// ReduceMotion returns true if the user prefers reduced animation.
// Implements gpucontext.PlatformProvider.
func (a *App) ReduceMotion() bool {
	if a.manager != nil {
		return a.manager.ReduceMotion()
	}
	return false
}

// HighContrast returns true if high contrast mode is active.
// Implements gpucontext.PlatformProvider.
func (a *App) HighContrast() bool {
	if a.manager != nil {
		return a.manager.HighContrast()
	}
	return false
}

// FontScale returns the user's font size preference multiplier.
// Implements gpucontext.PlatformProvider.
func (a *App) FontScale() float32 {
	if a.manager != nil {
		return a.manager.FontScale()
	}
	return 1.0
}

// SubpixelLayout returns the display's subpixel arrangement for LCD text rendering.
// Returns SubpixelNone when the manager is not initialized or on HiDPI displays.
// Implements gpucontext.PlatformProvider.
func (a *App) SubpixelLayout() gpucontext.SubpixelLayout {
	if a.manager != nil {
		return a.manager.SubpixelLayout()
	}
	return gpucontext.SubpixelNone
}

// SetFrameless enables or disables frameless window mode.
// Implements gpucontext.WindowChrome.
func (a *App) SetFrameless(frameless bool) {
	if a.platWindow != nil {
		a.platWindow.SetFrameless(frameless)
	}
}

// IsFrameless returns true if the window is in frameless mode.
// Implements gpucontext.WindowChrome.
func (a *App) IsFrameless() bool {
	if a.platWindow != nil {
		return a.platWindow.IsFrameless()
	}
	return false
}

// SetHitTestCallback sets the callback for custom hit testing in frameless mode.
// Implements gpucontext.WindowChrome.
func (a *App) SetHitTestCallback(callback gpucontext.HitTestCallback) {
	if a.platWindow != nil {
		a.platWindow.SetHitTestCallback(func(x, y float64) gpucontext.HitTestResult {
			if callback != nil {
				return callback(x, y)
			}
			return gpucontext.HitTestClient
		})
	}
}

// Minimize minimizes the window.
// Implements gpucontext.WindowChrome.
func (a *App) Minimize() {
	if a.platWindow != nil {
		a.platWindow.Minimize()
	}
}

// Maximize toggles between maximized and restored window state.
// Implements gpucontext.WindowChrome.
func (a *App) Maximize() {
	if a.platWindow != nil {
		a.platWindow.Maximize()
	}
}

// IsMaximized returns true if the window is maximized.
// Implements gpucontext.WindowChrome.
func (a *App) IsMaximized() bool {
	if a.platWindow != nil {
		return a.platWindow.IsMaximized()
	}
	return false
}

// SetFullscreen enters or exits fullscreen mode.
// On Windows: borderless fullscreen (Chromium/GLFW pattern).
// On macOS: native toggleFullScreen with animation.
// On X11: _NET_WM_STATE_FULLSCREEN via EWMH.
// On Wayland: xdg_toplevel.set_fullscreen / unset_fullscreen.
// Implements gpucontext.WindowChrome.
func (a *App) SetFullscreen(fullscreen bool) {
	if a.platWindow != nil {
		a.platWindow.SetFullscreen(fullscreen)
	}
}

// IsFullscreen returns true if the window is currently in fullscreen mode.
// Implements gpucontext.WindowChrome.
func (a *App) IsFullscreen() bool {
	if a.platWindow != nil {
		return a.platWindow.IsFullscreen()
	}
	return false
}

// ToggleFullscreen toggles between fullscreen and windowed mode.
// Convenience method equivalent to SetFullscreen(!IsFullscreen()).
func (a *App) ToggleFullscreen() {
	a.SetFullscreen(!a.IsFullscreen())
}

// Close requests the window to close.
// Implements gpucontext.WindowChrome.
func (a *App) Close() {
	if a.platWindow != nil {
		a.platWindow.Close()
	}
}

// Compile-time interface checks.
var _ gpucontext.WindowProvider = (*App)(nil)
var _ gpucontext.PlatformProvider = (*App)(nil)
var _ gpucontext.WindowChrome = (*App)(nil)

// Config returns the application configuration.
func (a *App) Config() Config {
	return a.config
}

// DeviceProvider returns a provider for GPU resources.
// This enables dependency injection of GPU capabilities into external
// libraries without circular dependencies.
//
// Example:
//
//	app := gogpu.NewApp(gogpu.Config{Title: "My App"})
//	provider := app.DeviceProvider()
//
//	// Access GPU resources
//	device := provider.Device()
//	queue := provider.Queue()
//
// Note: DeviceProvider is only valid after Run() has initialized
// the renderer. Calling before Run() returns nil.
func (a *App) DeviceProvider() DeviceProvider {
	if a.renderer == nil {
		return nil
	}
	return &rendererDeviceProvider{renderer: a.renderer}
}

// updateMouseStateFromPointer updates input.MouseState from a pointer event.
// This enables Ebiten-style polling for mouse state.
func (a *App) updateMouseStateFromPointer(ev gpucontext.PointerEvent) {
	if a.inputState == nil {
		return
	}

	// Update mouse position on any pointer event
	a.inputState.Mouse().SetPosition(float32(ev.X), float32(ev.Y))

	// Update button state on press/release (mouse only)
	if ev.PointerType != gpucontext.PointerTypeMouse {
		return
	}

	button := gpucontextButtonToInputButton(ev.Button)
	if button >= input.MouseButtonCount {
		return
	}

	switch ev.Type {
	case gpucontext.PointerDown:
		a.inputState.Mouse().SetButton(button, true)
	case gpucontext.PointerUp:
		a.inputState.Mouse().SetButton(button, false)
	}
}

// gpucontextKeyToInputKey converts gpucontext.Key to input.Key.
// Returns input.KeyUnknown if no mapping exists.
//
//nolint:cyclop,gocyclo,funlen,maintidx // key mapping tables are inherently large
func gpucontextKeyToInputKey(key gpucontext.Key) input.Key {
	switch key {
	// Letters
	case gpucontext.KeyA:
		return input.KeyA
	case gpucontext.KeyB:
		return input.KeyB
	case gpucontext.KeyC:
		return input.KeyC
	case gpucontext.KeyD:
		return input.KeyD
	case gpucontext.KeyE:
		return input.KeyE
	case gpucontext.KeyF:
		return input.KeyF
	case gpucontext.KeyG:
		return input.KeyG
	case gpucontext.KeyH:
		return input.KeyH
	case gpucontext.KeyI:
		return input.KeyI
	case gpucontext.KeyJ:
		return input.KeyJ
	case gpucontext.KeyK:
		return input.KeyK
	case gpucontext.KeyL:
		return input.KeyL
	case gpucontext.KeyM:
		return input.KeyM
	case gpucontext.KeyN:
		return input.KeyN
	case gpucontext.KeyO:
		return input.KeyO
	case gpucontext.KeyP:
		return input.KeyP
	case gpucontext.KeyQ:
		return input.KeyQ
	case gpucontext.KeyR:
		return input.KeyR
	case gpucontext.KeyS:
		return input.KeyS
	case gpucontext.KeyT:
		return input.KeyT
	case gpucontext.KeyU:
		return input.KeyU
	case gpucontext.KeyV:
		return input.KeyV
	case gpucontext.KeyW:
		return input.KeyW
	case gpucontext.KeyX:
		return input.KeyX
	case gpucontext.KeyY:
		return input.KeyY
	case gpucontext.KeyZ:
		return input.KeyZ

	// Numbers
	case gpucontext.Key0:
		return input.Key0
	case gpucontext.Key1:
		return input.Key1
	case gpucontext.Key2:
		return input.Key2
	case gpucontext.Key3:
		return input.Key3
	case gpucontext.Key4:
		return input.Key4
	case gpucontext.Key5:
		return input.Key5
	case gpucontext.Key6:
		return input.Key6
	case gpucontext.Key7:
		return input.Key7
	case gpucontext.Key8:
		return input.Key8
	case gpucontext.Key9:
		return input.Key9

	// Function keys
	case gpucontext.KeyF1:
		return input.KeyF1
	case gpucontext.KeyF2:
		return input.KeyF2
	case gpucontext.KeyF3:
		return input.KeyF3
	case gpucontext.KeyF4:
		return input.KeyF4
	case gpucontext.KeyF5:
		return input.KeyF5
	case gpucontext.KeyF6:
		return input.KeyF6
	case gpucontext.KeyF7:
		return input.KeyF7
	case gpucontext.KeyF8:
		return input.KeyF8
	case gpucontext.KeyF9:
		return input.KeyF9
	case gpucontext.KeyF10:
		return input.KeyF10
	case gpucontext.KeyF11:
		return input.KeyF11
	case gpucontext.KeyF12:
		return input.KeyF12

	// Navigation
	case gpucontext.KeyEscape:
		return input.KeyEscape
	case gpucontext.KeyTab:
		return input.KeyTab
	case gpucontext.KeyBackspace:
		return input.KeyBackspace
	case gpucontext.KeyEnter:
		return input.KeyEnter
	case gpucontext.KeySpace:
		return input.KeySpace
	case gpucontext.KeyInsert:
		return input.KeyInsert
	case gpucontext.KeyDelete:
		return input.KeyDelete
	case gpucontext.KeyHome:
		return input.KeyHome
	case gpucontext.KeyEnd:
		return input.KeyEnd
	case gpucontext.KeyPageUp:
		return input.KeyPageUp
	case gpucontext.KeyPageDown:
		return input.KeyPageDown
	case gpucontext.KeyLeft:
		return input.KeyLeft
	case gpucontext.KeyRight:
		return input.KeyRight
	case gpucontext.KeyUp:
		return input.KeyUp
	case gpucontext.KeyDown:
		return input.KeyDown

	// Modifiers
	case gpucontext.KeyLeftShift:
		return input.KeyShiftLeft
	case gpucontext.KeyRightShift:
		return input.KeyShiftRight
	case gpucontext.KeyLeftControl:
		return input.KeyControlLeft
	case gpucontext.KeyRightControl:
		return input.KeyControlRight
	case gpucontext.KeyLeftAlt:
		return input.KeyAltLeft
	case gpucontext.KeyRightAlt:
		return input.KeyAltRight
	case gpucontext.KeyLeftSuper:
		return input.KeySuperLeft
	case gpucontext.KeyRightSuper:
		return input.KeySuperRight

	// Punctuation
	case gpucontext.KeyMinus:
		return input.KeyMinus
	case gpucontext.KeyEqual:
		return input.KeyEqual
	case gpucontext.KeyLeftBracket:
		return input.KeyLeftBracket
	case gpucontext.KeyRightBracket:
		return input.KeyRightBracket
	case gpucontext.KeyBackslash:
		return input.KeyBackslash
	case gpucontext.KeySemicolon:
		return input.KeySemicolon
	case gpucontext.KeyApostrophe:
		return input.KeyApostrophe
	case gpucontext.KeyGrave:
		return input.KeyGrave
	case gpucontext.KeyComma:
		return input.KeyComma
	case gpucontext.KeyPeriod:
		return input.KeyPeriod
	case gpucontext.KeySlash:
		return input.KeySlash

	// Numpad
	case gpucontext.KeyNumpad0:
		return input.KeyNumpad0
	case gpucontext.KeyNumpad1:
		return input.KeyNumpad1
	case gpucontext.KeyNumpad2:
		return input.KeyNumpad2
	case gpucontext.KeyNumpad3:
		return input.KeyNumpad3
	case gpucontext.KeyNumpad4:
		return input.KeyNumpad4
	case gpucontext.KeyNumpad5:
		return input.KeyNumpad5
	case gpucontext.KeyNumpad6:
		return input.KeyNumpad6
	case gpucontext.KeyNumpad7:
		return input.KeyNumpad7
	case gpucontext.KeyNumpad8:
		return input.KeyNumpad8
	case gpucontext.KeyNumpad9:
		return input.KeyNumpad9
	case gpucontext.KeyNumpadDecimal:
		return input.KeyNumpadDecimal
	case gpucontext.KeyNumpadDivide:
		return input.KeyNumpadDivide
	case gpucontext.KeyNumpadMultiply:
		return input.KeyNumpadMultiply
	case gpucontext.KeyNumpadSubtract:
		return input.KeyNumpadSubtract
	case gpucontext.KeyNumpadAdd:
		return input.KeyNumpadAdd
	case gpucontext.KeyNumpadEnter:
		return input.KeyNumpadEnter

	// Lock keys
	case gpucontext.KeyCapsLock:
		return input.KeyCapsLock
	case gpucontext.KeyScrollLock:
		return input.KeyScrollLock
	case gpucontext.KeyNumLock:
		return input.KeyNumLock
	case gpucontext.KeyPause:
		return input.KeyPause

	default:
		return input.KeyUnknown
	}
}

// gpucontextButtonToInputButton converts gpucontext.Button to input.MouseButton.
func gpucontextButtonToInputButton(button gpucontext.Button) input.MouseButton {
	switch button {
	case gpucontext.ButtonLeft:
		return input.MouseButtonLeft
	case gpucontext.ButtonRight:
		return input.MouseButtonRight
	case gpucontext.ButtonMiddle:
		return input.MouseButtonMiddle
	case gpucontext.ButtonX1:
		return input.MouseButton4
	case gpucontext.ButtonX2:
		return input.MouseButton5
	default:
		return input.MouseButtonLeft
	}
}
