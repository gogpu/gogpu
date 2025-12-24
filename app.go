package gogpu

import (
	"time"

	"github.com/gogpu/gogpu/internal/platform"
)

// App is the main application type.
// It manages the window, rendering, and application lifecycle.
type App struct {
	config   Config
	platform platform.Platform
	renderer *Renderer

	// User callbacks
	onDraw   func(*Context)
	onUpdate func(float64) // delta time in seconds
	onResize func(int, int)

	// State
	running   bool
	lastFrame time.Time
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

// Run starts the application main loop.
// This function blocks until the application quits.
func (a *App) Run() error {
	// Initialize platform (window)
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

	// Initialize renderer with selected backend
	var err error
	a.renderer, err = newRenderer(a.platform, a.config.Backend)
	if err != nil {
		return err
	}
	defer a.renderer.Destroy()

	// Main loop
	a.running = true
	a.lastFrame = time.Now()

	for a.running && !a.platform.ShouldClose() {
		// Process platform events
		a.processEvents()

		// Calculate delta time
		now := time.Now()
		deltaTime := now.Sub(a.lastFrame).Seconds()
		a.lastFrame = now

		// Call update callback
		if a.onUpdate != nil {
			a.onUpdate(deltaTime)
		}

		// Render frame
		a.renderFrame()
	}

	return nil
}

// processEvents handles platform events.
func (a *App) processEvents() {
	for {
		event := a.platform.PollEvents()
		if event.Type == platform.EventNone {
			break
		}

		switch event.Type {
		case platform.EventResize:
			a.renderer.Resize(event.Width, event.Height)
			if a.onResize != nil {
				a.onResize(event.Width, event.Height)
			}
		case platform.EventClose:
			a.running = false
		}
	}
}

// renderFrame renders a single frame.
func (a *App) renderFrame() {
	// Skip rendering if window is minimized (zero dimensions)
	width, height := a.platform.GetSize()
	if width <= 0 || height <= 0 {
		return // Window minimized, skip frame
	}

	// Acquire frame
	if !a.renderer.BeginFrame() {
		return // Frame not available
	}

	// Create context and call draw callback
	if a.onDraw != nil {
		ctx := newContext(a.renderer)
		a.onDraw(ctx)
	}

	// Present frame
	a.renderer.EndFrame()
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
