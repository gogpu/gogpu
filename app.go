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
	config   Config
	platform platform.Platform
	renderer *Renderer

	// Multi-thread rendering
	renderLoop *thread.RenderLoop

	// User callbacks
	onDraw   func(*Context)
	onUpdate func(float64) // delta time in seconds
	onResize func(int, int)
	onClose  func() // called before renderer destruction

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
}

// NewApp creates a new application with the given configuration.
func NewApp(config Config) *App {
	return &App{
		config: config,
	}
}

// OnDraw sets the callback for rendering each frame.
// The Context is only valid during the callback.
func (a *App) OnDraw(fn func(*Context)) *App {
	a.onDraw = fn
	return a
}

// OnUpdate sets the callback for logic updates each frame.
// The parameter is delta time in seconds since the last frame.
func (a *App) OnUpdate(fn func(float64)) *App {
	a.onUpdate = fn
	return a
}

// OnResize sets the callback for window resize events.
func (a *App) OnResize(fn func(width, height int)) *App {
	a.onResize = fn
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

	// Initialize platform (window) - must be on main thread
	platform.SetLogger(slogger())
	a.platform = platform.New()
	if err := a.platform.Init(platform.Config{
		Title:      a.config.Title,
		Width:      a.config.Width,
		Height:     a.config.Height,
		Resizable:  a.config.Resizable,
		Fullscreen: a.config.Fullscreen,
		Frameless:  a.config.Frameless,
	}); err != nil {
		return err
	}
	defer a.platform.Destroy()

	// Initialize input state BEFORE setting up event callbacks.
	// This ensures keyboard/mouse state is captured from the first event.
	// (Follows Ebitengine/GLFW/SDL pattern - state must exist before callbacks)
	a.inputState = input.New()

	// Wire platform callbacks to eventSourceAdapter for input events
	a.setupInputEvents()

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
	a.platform.SetModalFrameCallback(a.modalFrameTick)

	// Create render loop with dedicated render thread
	a.renderLoop = thread.NewRenderLoop()
	defer a.renderLoop.Stop()

	// Initialize renderer on render thread (all GPU operations must be on same thread)
	var initErr error
	a.renderLoop.RunOnRenderThreadVoid(func() {
		a.renderer, initErr = newRenderer(a.platform, a.config.Backend, a.config.GraphicsAPI, a.config.VSync, a.config.PowerPreference)
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

	// Main loop with three-state event-driven model:
	//   1. IDLE: No activity — block on OS events (0% CPU, <1ms response)
	//   2. ANIMATING: Active animations — render at VSync (smooth 60fps)
	//   3. CONTINUOUS: ContinuousRender=true — always render (game loop)
	a.running = true
	a.lastFrame = time.Now()
	a.invalidator = newInvalidator(a.platform.WakeUp)
	a.animations = &AnimationController{}
	a.invalidator.Invalidate() // Request initial frame

	for a.running && !a.platform.ShouldClose() {
		// Determine rendering state
		continuous := a.config.ContinuousRender || a.animations.IsAnimating()
		invalidated := a.invalidator.Consume()

		if !continuous && !invalidated {
			// IDLE STATE: Block on OS events (0% CPU, <1ms response)
			a.platform.WaitEvents()
		}

		// Process all pending platform events
		hasEvents := a.processEventsMultiThread()

		// Check if invalidation arrived during event processing
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

		// Call update callback (main thread - logic updates)
		if a.onUpdate != nil {
			a.onUpdate(deltaTime)
		}

		// Render frame if needed
		if continuous || invalidated || hasEvents {
			a.renderFrameMultiThread()
		}
	}

	return nil
}

// processEventsMultiThread handles platform events with multi-thread pattern.
// Resize events are deferred to the render thread via RequestResize.
// Returns true if any events were processed (used for event-driven rendering).
func (a *App) processEventsMultiThread() bool {
	// Collect all events first, then process.
	// This allows us to coalesce resize events.
	var lastResize *platform.Event
	var events []platform.Event

	for {
		event := a.platform.PollEvents()
		if event.Type == platform.EventNone {
			break
		}
		events = append(events, event)
	}

	// Process all events, but track only the last resize
	for i := range events {
		event := &events[i]
		switch event.Type {
		case platform.EventResize:
			lastResize = event
		case platform.EventClose:
			a.running = false
		}
	}

	// Queue resize for render thread (deferred pattern)
	// Don't apply resize during modal resize loop (Windows)
	if lastResize != nil && !a.platform.InSizeMove() {
		// Queue PHYSICAL size for render thread (GPU surface reconfiguration)
		physW, physH := lastResize.PhysicalWidth, lastResize.PhysicalHeight
		if physW > 0 && physH > 0 {
			a.renderLoop.RequestResize(uint32(physW), uint32(physH)) //nolint:gosec // G115: validated positive
		}

		// Call user callback with LOGICAL size (what user expects for layout)
		if a.onResize != nil {
			a.onResize(lastResize.Width, lastResize.Height)
		}
	}

	// Dispatch end-of-frame events (gestures computed from pointer events)
	if a.eventSource != nil {
		a.eventSource.dispatchEndFrame()
	}

	return len(events) > 0
}

// renderFrameMultiThread renders a frame using the render thread.
// All GPU operations happen on the render thread to keep main thread responsive.
func (a *App) renderFrameMultiThread() {
	// Skip rendering if window is minimized (zero physical dimensions)
	width, height := a.platform.PhysicalSize()
	if width <= 0 || height <= 0 {
		return // Window minimized, skip frame
	}

	// Capture callback and scale factor for render thread
	onDraw := a.onDraw
	scale := a.platform.ScaleFactor()

	// Execute GPU operations on render thread
	a.renderLoop.RunOnRenderThreadVoid(func() {
		// Apply pending resize (deferred from main thread)
		if w, h, ok := a.renderLoop.ConsumePendingResize(); ok {
			a.renderer.Resize(int(w), int(h))
		}

		// Acquire frame
		if !a.renderer.BeginFrame() {
			return // Frame not available
		}

		// Create context and call draw callback
		if onDraw != nil {
			ctx := newContext(a.renderer, scale)
			onDraw(ctx)
		}

		// Present frame
		a.renderer.EndFrame()
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
	width, height := a.platform.PhysicalSize()
	if width > 0 && height > 0 {
		a.renderLoop.RequestResize(uint32(width), uint32(height)) //nolint:gosec // G115: validated positive
	}

	// Render frame on render thread (blocks until complete).
	a.renderFrameMultiThread()

	// Synchronize with compositor (DwmFlush on Windows).
	// This ensures our frame and the DWM window border update
	// appear in the same composition cycle, reducing resize lag.
	a.platform.SyncFrame()
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
	if a.platform != nil {
		return a.platform.LogicalSize()
	}
	return a.config.Width, a.config.Height
}

// PhysicalSize returns the current GPU framebuffer size in device pixels.
// On Retina/HiDPI displays this is larger than Size() by ScaleFactor().
func (a *App) PhysicalSize() (width, height int) {
	if a.platform != nil {
		return a.platform.PhysicalSize()
	}
	return a.config.Width, a.config.Height
}

// ScaleFactor returns the DPI scale factor.
// 1.0 = standard (96 DPI on Windows, 72 on macOS), 2.0 = Retina/HiDPI.
// Implements gpucontext.WindowProvider.
func (a *App) ScaleFactor() float64 {
	if a.platform != nil {
		return a.platform.ScaleFactor()
	}
	return 1.0
}

// ClipboardRead reads text content from the system clipboard.
// Implements gpucontext.PlatformProvider.
func (a *App) ClipboardRead() (string, error) {
	if a.platform != nil {
		return a.platform.ClipboardRead()
	}
	return "", nil
}

// ClipboardWrite writes text content to the system clipboard.
// Implements gpucontext.PlatformProvider.
func (a *App) ClipboardWrite(text string) error {
	if a.platform != nil {
		return a.platform.ClipboardWrite(text)
	}
	return nil
}

// SetCursor changes the mouse cursor shape.
// Implements gpucontext.PlatformProvider.
func (a *App) SetCursor(cursor gpucontext.CursorShape) {
	if a.platform != nil {
		a.platform.SetCursor(int(cursor))
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
	if a.platform != nil {
		a.platform.SetCursorMode(int(mode))
	}
}

// CursorMode returns the current cursor confinement mode.
func (a *App) CursorMode() gpucontext.CursorMode {
	if a.platform != nil {
		return gpucontext.CursorMode(a.platform.CursorMode())
	}
	return gpucontext.CursorModeNormal
}

// DarkMode returns true if the system dark mode is active.
// Implements gpucontext.PlatformProvider.
func (a *App) DarkMode() bool {
	if a.platform != nil {
		return a.platform.DarkMode()
	}
	return false
}

// ReduceMotion returns true if the user prefers reduced animation.
// Implements gpucontext.PlatformProvider.
func (a *App) ReduceMotion() bool {
	if a.platform != nil {
		return a.platform.ReduceMotion()
	}
	return false
}

// HighContrast returns true if high contrast mode is active.
// Implements gpucontext.PlatformProvider.
func (a *App) HighContrast() bool {
	if a.platform != nil {
		return a.platform.HighContrast()
	}
	return false
}

// FontScale returns the user's font size preference multiplier.
// Implements gpucontext.PlatformProvider.
func (a *App) FontScale() float32 {
	if a.platform != nil {
		return a.platform.FontScale()
	}
	return 1.0
}

// SetFrameless enables or disables frameless window mode.
// Implements gpucontext.WindowChrome.
func (a *App) SetFrameless(frameless bool) {
	if a.platform != nil {
		a.platform.SetFrameless(frameless)
	}
}

// IsFrameless returns true if the window is in frameless mode.
// Implements gpucontext.WindowChrome.
func (a *App) IsFrameless() bool {
	if a.platform != nil {
		return a.platform.IsFrameless()
	}
	return false
}

// SetHitTestCallback sets the callback for custom hit testing in frameless mode.
// Implements gpucontext.WindowChrome.
func (a *App) SetHitTestCallback(callback gpucontext.HitTestCallback) {
	if a.platform != nil {
		a.platform.SetHitTestCallback(func(x, y float64) gpucontext.HitTestResult {
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
	if a.platform != nil {
		a.platform.Minimize()
	}
}

// Maximize toggles between maximized and restored window state.
// Implements gpucontext.WindowChrome.
func (a *App) Maximize() {
	if a.platform != nil {
		a.platform.Maximize()
	}
}

// IsMaximized returns true if the window is maximized.
// Implements gpucontext.WindowChrome.
func (a *App) IsMaximized() bool {
	if a.platform != nil {
		return a.platform.IsMaximized()
	}
	return false
}

// Close requests the window to close.
// Implements gpucontext.WindowChrome.
func (a *App) Close() {
	if a.platform != nil {
		a.platform.CloseWindow()
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

// setupInputEvents wires platform callbacks to eventSourceAdapter.
// This enables W3C Pointer Events, detailed scroll events, and keyboard events
// to flow from the platform layer to the gpucontext event system.
func (a *App) setupInputEvents() {
	// Ensure eventSource exists
	if a.eventSource == nil {
		a.eventSource = &eventSourceAdapter{app: a}
	}

	// Wire pointer events from platform to eventSource
	a.platform.SetPointerCallback(func(ev gpucontext.PointerEvent) {
		a.eventSource.dispatchPointerEvent(ev)
		a.updateMouseStateFromPointer(ev)
	})

	// Wire scroll events from platform to eventSource
	a.platform.SetScrollCallback(func(ev gpucontext.ScrollEvent) {
		a.eventSource.dispatchScrollEventDetailed(ev)

		// Update input state for Ebiten-style polling
		if a.inputState != nil {
			a.inputState.Mouse().SetScroll(float32(ev.DeltaX), float32(ev.DeltaY))
		}
	})

	// Wire keyboard events from platform to eventSource
	a.platform.SetKeyCallback(func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
		// Dispatch to callbacks (gpucontext.EventSource interface)
		if pressed {
			a.eventSource.dispatchKeyPress(key, mods)
		} else {
			a.eventSource.dispatchKeyRelease(key, mods)
		}

		// Update input state for Ebiten-style polling
		if a.inputState != nil {
			inputKey := gpucontextKeyToInputKey(key)
			if inputKey != input.KeyUnknown {
				a.inputState.Keyboard().SetKey(inputKey, pressed)
			}
		}
	})

	// Wire character input from platform to eventSource
	a.platform.SetCharCallback(func(char rune) {
		a.eventSource.dispatchTextInput(string(char))
	})
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
