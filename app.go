package gogpu

import (
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

	// State
	running   bool
	lastFrame time.Time

	// Event source for gpucontext integration
	eventSource *eventSourceAdapter

	// Input state for Ebiten-style polling (KeyJustPressed, etc.)
	inputState *input.State
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
	a.platform = platform.New()
	if err := a.platform.Init(platform.Config{
		Title:      a.config.Title,
		Width:      a.config.Width,
		Height:     a.config.Height,
		Resizable:  a.config.Resizable,
		Fullscreen: a.config.Fullscreen,
	}); err != nil {
		return err
	}
	defer a.platform.Destroy()

	// Wire platform callbacks to eventSourceAdapter for input events
	a.setupInputEvents()

	// Create render loop with dedicated render thread
	a.renderLoop = thread.NewRenderLoop()
	defer a.renderLoop.Stop()

	// Initialize renderer on render thread (all GPU operations must be on same thread)
	var initErr error
	a.renderLoop.RunOnRenderThreadVoid(func() {
		a.renderer, initErr = newRenderer(a.platform, a.config.Backend)
	})
	if initErr != nil {
		return initErr
	}
	defer func() {
		a.renderLoop.RunOnRenderThreadVoid(func() {
			a.renderer.Destroy()
		})
	}()

	// Main loop
	a.running = true
	a.lastFrame = time.Now()

	for a.running && !a.platform.ShouldClose() {
		// Process platform events (main thread)
		a.processEventsMultiThread()

		// Calculate delta time
		now := time.Now()
		deltaTime := now.Sub(a.lastFrame).Seconds()
		a.lastFrame = now

		// Update input state for next frame (Ebiten-style polling)
		// This must be called before onUpdate so JustPressed/JustReleased work correctly
		if a.inputState != nil {
			a.inputState.Update()
		}

		// Call update callback (main thread - logic updates)
		if a.onUpdate != nil {
			a.onUpdate(deltaTime)
		}

		// Render frame on render thread
		a.renderFrameMultiThread()
	}

	return nil
}

// processEventsMultiThread handles platform events with multi-thread pattern.
// Resize events are deferred to the render thread via RequestResize.
func (a *App) processEventsMultiThread() {
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
		// Queue resize for render thread
		if lastResize.Width > 0 && lastResize.Height > 0 {
			a.renderLoop.RequestResize(uint32(lastResize.Width), uint32(lastResize.Height)) //nolint:gosec // G115: validated positive
		}

		// Call user callback immediately (for UI updates)
		if a.onResize != nil {
			a.onResize(lastResize.Width, lastResize.Height)
		}
	}

	// Dispatch end-of-frame events (gestures computed from pointer events)
	if a.eventSource != nil {
		a.eventSource.dispatchEndFrame()
	}
}

// renderFrameMultiThread renders a frame using the render thread.
// All GPU operations happen on the render thread to keep main thread responsive.
func (a *App) renderFrameMultiThread() {
	// Skip rendering if window is minimized (zero dimensions)
	width, height := a.platform.GetSize()
	if width <= 0 || height <= 0 {
		return // Window minimized, skip frame
	}

	// Capture callback for render thread
	onDraw := a.onDraw

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
			ctx := newContext(a.renderer)
			onDraw(ctx)
		}

		// Present frame
		a.renderer.EndFrame()
	})
}

// Quit requests the application to quit.
// The main loop will exit after completing the current frame.
func (a *App) Quit() {
	a.running = false
}

// Size returns the current window size.
func (a *App) Size() (width, height int) {
	if a.platform != nil {
		return a.platform.GetSize()
	}
	return a.config.Width, a.config.Height
}

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
