//go:build darwin

package platform

import (
	"sync"

	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// darwinPlatform implements Platform for macOS using Cocoa/AppKit.
type darwinPlatform struct {
	mu          sync.Mutex
	app         *darwin.Application
	window      *darwin.Window
	surface     *darwin.Surface
	config      Config
	shouldClose bool
	events      []Event
}

func newPlatform() Platform {
	return &darwinPlatform{}
}

func (p *darwinPlatform) Init(config Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config

	// Initialize NSApplication
	p.app = darwin.GetApplication()
	if err := p.app.Init(); err != nil {
		return err
	}

	// Create window
	windowConfig := darwin.WindowConfig{
		Title:      config.Title,
		Width:      config.Width,
		Height:     config.Height,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
	}

	window, err := darwin.NewWindow(windowConfig)
	if err != nil {
		return err
	}
	p.window = window

	// Create Metal surface for GPU rendering
	surface, err := darwin.NewSurface(window)
	if err != nil {
		// Non-fatal: window works without Metal surface
		// This allows the window to still be used with software rendering
		p.surface = nil
	} else {
		p.surface = surface
	}

	// Show window
	p.window.Show()

	return nil
}

func (p *darwinPlatform) PollEvents() Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Process OS events
	if p.app != nil {
		p.app.PollEvents()
	}

	// Check if window should close
	if p.window != nil && p.window.ShouldClose() {
		p.shouldClose = true
		return Event{Type: EventClose}
	}

	// Update window size and check for resize
	if p.window != nil {
		oldWidth, oldHeight := p.config.Width, p.config.Height
		p.window.UpdateSize()
		newWidth, newHeight := p.window.Size()

		if newWidth != oldWidth || newHeight != oldHeight {
			p.config.Width = newWidth
			p.config.Height = newHeight

			// Update surface size
			if p.surface != nil {
				p.surface.Resize(newWidth, newHeight)
			}

			return Event{
				Type:   EventResize,
				Width:  newWidth,
				Height: newHeight,
			}
		}
	}

	// Return queued event if any
	if len(p.events) > 0 {
		event := p.events[0]
		p.events = p.events[1:]
		return event
	}

	return Event{Type: EventNone}
}

func (p *darwinPlatform) ShouldClose() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.ShouldClose() || p.shouldClose
	}
	return p.shouldClose
}

func (p *darwinPlatform) GetSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.window != nil {
		return p.window.Size()
	}
	return p.config.Width, p.config.Height
}

func (p *darwinPlatform) GetHandle() (instance, window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// On macOS:
	// - instance: 0 (not used)
	// - window: CAMetalLayer pointer for surface creation
	if p.surface != nil {
		return 0, p.surface.LayerPtr()
	}

	// Fallback to content view if no surface
	if p.window != nil {
		return 0, p.window.ViewHandle()
	}

	return 0, 0
}

func (p *darwinPlatform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.surface != nil {
		p.surface.Destroy()
		p.surface = nil
	}

	if p.window != nil {
		p.window.Destroy()
		p.window = nil
	}

	if p.app != nil {
		p.app.Destroy()
		p.app = nil
	}
}

// queueEvent adds an event to the event queue.
func (p *darwinPlatform) queueEvent(event Event) {
	p.events = append(p.events, event)
}
