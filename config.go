package gogpu

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
