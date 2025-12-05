// Package platform provides OS-specific windowing abstraction.
package platform

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

	// Destroy closes the window and releases resources.
	Destroy()
}

// New creates a platform-specific implementation.
// This is implemented in platform-specific files.
func New() Platform {
	return newPlatform()
}
