//go:build linux

package x11

import (
	"fmt"
	"sync"
)

// Config holds configuration for creating a platform window.
// This mirrors platform.Config to avoid import cycles.
type Config struct {
	Title      string
	Width      int
	Height     int
	Resizable  bool
	Fullscreen bool
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventTypeNone EventType = iota
	EventTypeClose
	EventTypeResize
)

// PlatformEvent represents a platform event.
// This mirrors platform.Event to avoid import cycles.
type PlatformEvent struct {
	Type   EventType
	Width  int
	Height int
}

// Platform implements X11 windowing support.
type Platform struct {
	mu sync.Mutex

	// X11 connection
	conn *Connection

	// Standard atoms
	atoms *StandardAtoms

	// Window
	window ResourceID

	// Keyboard mapping
	keymap *KeyboardMapping

	// Window state
	width       int
	height      int
	shouldClose bool
	configured  bool

	// Pending resize
	pendingWidth  int
	pendingHeight int
	hasResize     bool
}

// NewPlatform creates a new X11 platform instance.
func NewPlatform() *Platform {
	return &Platform{}
}

// Init creates the X11 window.
func (p *Platform) Init(config Config) error {
	// Connect to X server
	conn, err := Connect()
	if err != nil {
		return fmt.Errorf("x11: failed to connect: %w", err)
	}
	p.conn = conn

	// Intern standard atoms
	atoms, err := conn.InternStandardAtoms()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to intern atoms: %w", err)
	}
	p.atoms = atoms

	// Create window
	windowConfig := WindowConfig{
		Title:      config.Title,
		Width:      uint16(config.Width),
		Height:     uint16(config.Height),
		X:          0,
		Y:          0,
		Resizable:  config.Resizable,
		Fullscreen: config.Fullscreen,
	}

	window, err := conn.CreateWindow(windowConfig)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to create window: %w", err)
	}
	p.window = window

	// Set window properties
	if err := conn.SetWindowTitle(window, config.Title, atoms); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to set title: %w", err)
	}

	// Set WM protocols (for close button)
	if err := conn.SetWMProtocols(window, atoms); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to set WM protocols: %w", err)
	}

	// Set WM class
	if err := conn.SetWMClass(window, "gogpu", "GoGPU"); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to set WM class: %w", err)
	}

	// Set PID (non-fatal, some WMs don't support this)
	_ = conn.SetWMPID(window, atoms)

	// Set window type (non-fatal, some WMs don't support this)
	_ = conn.SetNetWMWindowType(window, atoms.NetWMWindowTypeNormal, atoms)

	// Handle non-resizable windows via Motif hints
	if !config.Resizable {
		hints := &MotifWMHints{
			Flags:       MotifHintsDecorations | MotifHintsFunctions,
			Decorations: MotifDecorBorder | MotifDecorTitle | MotifDecorMenu | MotifDecorMinimize,
			Functions:   1 | 2 | 8, // Move | Minimize | Close (no Resize or Maximize)
		}
		// Non-fatal, some WMs don't support Motif hints
		_ = conn.SetMotifWMHints(window, hints, atoms)
	}

	// Map (show) the window
	if err := conn.MapWindow(window); err != nil {
		_ = conn.Close()
		return fmt.Errorf("x11: failed to map window: %w", err)
	}

	// Get keyboard mapping (non-fatal - keyboard input may not work correctly without it)
	keymap, _ := conn.GetKeyboardMapping()
	p.keymap = keymap

	// Set fullscreen if requested (non-fatal, will fail if WM doesn't support EWMH)
	if config.Fullscreen {
		_ = conn.SetFullscreen(window, true, atoms)
	}

	// Store initial size
	p.width = config.Width
	p.height = config.Height
	p.configured = true

	// Flush to ensure all requests are sent
	_ = conn.Flush()

	// Sync to ensure window is created
	_ = conn.Sync()

	return nil
}

// PollEvents processes pending X11 events.
func (p *Platform) PollEvents() PlatformEvent {
	p.mu.Lock()

	// Check for pending resize
	if p.hasResize {
		p.width = p.pendingWidth
		p.height = p.pendingHeight
		p.hasResize = false
		p.mu.Unlock()

		return PlatformEvent{
			Type:   EventTypeResize,
			Width:  p.pendingWidth,
			Height: p.pendingHeight,
		}
	}

	// Check for close
	if p.shouldClose {
		p.mu.Unlock()
		return PlatformEvent{Type: EventTypeClose}
	}

	p.mu.Unlock()

	// Process pending events
	for {
		event, err := p.conn.PollEvent()
		if err != nil {
			p.mu.Lock()
			p.shouldClose = true
			p.mu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

		if event == nil {
			break
		}

		if platformEvent := p.handleEvent(event); platformEvent.Type != EventTypeNone {
			return platformEvent
		}
	}

	// Check again after processing
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.hasResize {
		p.width = p.pendingWidth
		p.height = p.pendingHeight
		p.hasResize = false
		return PlatformEvent{
			Type:   EventTypeResize,
			Width:  p.pendingWidth,
			Height: p.pendingHeight,
		}
	}

	if p.shouldClose {
		return PlatformEvent{Type: EventTypeClose}
	}

	return PlatformEvent{Type: EventTypeNone}
}

// handleEvent processes a single X11 event.
func (p *Platform) handleEvent(event Event) PlatformEvent {
	switch e := event.(type) {
	case *ConfigureNotifyEvent:
		if e.Window == p.window {
			p.mu.Lock()
			newWidth := int(e.Width)
			newHeight := int(e.Height)
			if newWidth != p.width || newHeight != p.height {
				p.pendingWidth = newWidth
				p.pendingHeight = newHeight
				p.hasResize = true
			}
			p.mu.Unlock()

			if p.hasResize {
				return PlatformEvent{
					Type:   EventTypeResize,
					Width:  newWidth,
					Height: newHeight,
				}
			}
		}

	case *ClientMessageEvent:
		if e.IsDeleteWindow(p.atoms) {
			p.mu.Lock()
			p.shouldClose = true
			p.mu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

	case *DestroyNotifyEvent:
		if e.Window == p.window {
			p.mu.Lock()
			p.shouldClose = true
			p.mu.Unlock()
			return PlatformEvent{Type: EventTypeClose}
		}

	case *ExposeEvent:
		// Could trigger redraw, but for now we just ignore
		// The main render loop should handle this

	case *MapNotifyEvent:
		p.mu.Lock()
		p.configured = true
		p.mu.Unlock()
	}

	return PlatformEvent{Type: EventTypeNone}
}

// ShouldClose returns true if window close was requested.
func (p *Platform) ShouldClose() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shouldClose
}

// GetSize returns current window size in pixels.
func (p *Platform) GetSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width, p.height
}

// GetHandle returns platform-specific handles for Vulkan surface creation.
// Returns (display_fd, window_id).
func (p *Platform) GetHandle() (instance, window uintptr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		return 0, 0
	}

	return uintptr(p.conn.Fd()), uintptr(p.window)
}

// Destroy closes the window and releases resources.
func (p *Platform) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		if p.window != 0 {
			_ = p.conn.DestroyWindow(p.window)
			p.window = 0
		}
		_ = p.conn.Close()
		p.conn = nil
	}

	p.atoms = nil
	p.keymap = nil
}
