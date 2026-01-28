package gogpu

import (
	"runtime"
	"time"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gogpu/internal/thread"
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
