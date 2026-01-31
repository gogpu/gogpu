// Package platform provides OS-specific windowing abstraction.
package platform

import (
	"github.com/gogpu/gpucontext"
)

// Config holds platform-agnostic window configuration.
type Config struct {
	Title      string
	Width      int
	Height     int
	Resizable  bool
	Fullscreen bool
}

// Event represents a platform event.
type Event struct {
	Type   EventType
	Width  int // for resize events
	Height int // for resize events
}

// EventType represents the type of platform event.
type EventType uint8

const (
	EventNone EventType = iota
	EventClose
	EventResize
)

// Platform abstracts OS-specific windowing.
type Platform interface {
	// Init creates the window.
	Init(config Config) error

	// PollEvents processes pending events.
	// Returns the next event, or EventNone if no events.
	PollEvents() Event

	// ShouldClose returns true if window close was requested.
	ShouldClose() bool

	// GetSize returns current window size in pixels.
	GetSize() (width, height int)

	// GetHandle returns platform-specific handles for surface creation.
	// On Windows: (hinstance, hwnd)
	// On macOS: (0, nsview)
	// On Linux: (display, window)
	GetHandle() (instance, window uintptr)

	// InSizeMove returns true if the window is currently being resized/moved.
	// During modal resize (Windows) or live resize (macOS), this returns true.
	// Used to defer swapchain recreation until resize ends.
	InSizeMove() bool

	// SetPointerCallback registers a callback for pointer events.
	// The callback receives W3C Pointer Events Level 3 compliant events.
	SetPointerCallback(fn func(gpucontext.PointerEvent))

	// SetScrollCallback registers a callback for scroll events.
	// The callback receives scroll events with position, delta, and modifiers.
	SetScrollCallback(fn func(gpucontext.ScrollEvent))

	// SetKeyCallback registers a callback for keyboard events.
	// The callback receives the key, modifiers, and whether the key was pressed (true) or released (false).
	SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool))

	// Destroy closes the window and releases resources.
	Destroy()
}

// New creates a platform-specific implementation.
// This is implemented in platform-specific files.
func New() Platform {
	return newPlatform()
}
