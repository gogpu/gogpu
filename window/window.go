// Package window provides cross-platform windowing support for gogpu.
package window

import (
	"errors"
)

// Common errors
var (
	ErrWindowCreation = errors.New("window: failed to create window")
	ErrNotInitialized = errors.New("window: windowing system not initialized")
)

// Config describes window configuration.
type Config struct {
	Title       string
	Width       int
	Height      int
	Resizable   bool
	Fullscreen  bool
	VSync       bool
	Transparent bool
	Decorated   bool
	Visible     bool
}

// DefaultConfig returns sensible default window configuration.
func DefaultConfig() Config {
	return Config{
		Title:       "GoGPU Window",
		Width:       800,
		Height:      600,
		Resizable:   true,
		Fullscreen:  false,
		VSync:       true,
		Transparent: false,
		Decorated:   true,
		Visible:     true,
	}
}

// Window represents a platform window with GPU surface.
type Window struct {
	config Config
	// Platform-specific handle will be added
}

// New creates a new window with the given configuration.
func New(config Config) (*Window, error) {
	// TODO: Implement platform-specific window creation
	return &Window{config: config}, nil
}

// Title returns the window title.
func (w *Window) Title() string {
	return w.config.Title
}

// SetTitle sets the window title.
func (w *Window) SetTitle(title string) {
	w.config.Title = title
	// TODO: Update platform window title
}

// Size returns the window size in pixels.
func (w *Window) Size() (width, height int) {
	return w.config.Width, w.config.Height
}

// SetSize sets the window size.
func (w *Window) SetSize(width, height int) {
	w.config.Width = width
	w.config.Height = height
	// TODO: Update platform window size
}

// Position returns the window position.
func (w *Window) Position() (x, y int) {
	// TODO: Get from platform
	return 0, 0
}

// SetPosition sets the window position.
func (w *Window) SetPosition(x, y int) {
	// TODO: Update platform window position
}

// Fullscreen returns true if window is fullscreen.
func (w *Window) Fullscreen() bool {
	return w.config.Fullscreen
}

// SetFullscreen sets fullscreen mode.
func (w *Window) SetFullscreen(fullscreen bool) {
	w.config.Fullscreen = fullscreen
	// TODO: Update platform window
}

// ShouldClose returns true if the window should close.
func (w *Window) ShouldClose() bool {
	// TODO: Check platform close flag
	return false
}

// Close closes the window.
func (w *Window) Close() {
	// TODO: Platform cleanup
}
