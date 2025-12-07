package gogpu

import "github.com/gogpu/gogpu/gpu/types"

// Config configures the application.
type Config struct {
	// Title is the window title.
	Title string

	// Width is the initial window width in pixels.
	Width int

	// Height is the initial window height in pixels.
	Height int

	// Resizable allows the window to be resized.
	Resizable bool

	// VSync enables vertical synchronization.
	VSync bool

	// Fullscreen starts in fullscreen mode.
	Fullscreen bool

	// Backend specifies which WebGPU implementation to use.
	// BackendAuto (default) selects the best available.
	Backend types.BackendType
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Title:     "GoGPU Application",
		Width:     800,
		Height:    600,
		Resizable: true,
		VSync:     true,
	}
}

// WithTitle returns a copy with the title set.
func (c Config) WithTitle(title string) Config {
	c.Title = title
	return c
}

// WithSize returns a copy with the size set.
func (c Config) WithSize(width, height int) Config {
	c.Width = width
	c.Height = height
	return c
}

// WithBackend returns a copy with the backend set.
// Use types.BackendRust for maximum performance (requires native library).
// Use types.BackendGo for zero dependencies (pure Go, may be slower).
// Use types.BackendAuto (default) to automatically select the best available.
func (c Config) WithBackend(backend types.BackendType) Config {
	c.Backend = backend
	return c
}

// Re-export backend types for convenience.
const (
	BackendAuto = types.BackendAuto
	BackendRust = types.BackendRust
	BackendGo   = types.BackendGo
)
