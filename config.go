package gogpu

import (
	"os"
	"strings"

	"github.com/gogpu/gogpu/gpu/types"
)

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

	// GraphicsAPI specifies which graphics API to use (Vulkan, DX12, Metal).
	// GraphicsAPIAuto (default) selects the best for the platform.
	// This is orthogonal to Backend (Rust/Native implementation choice).
	GraphicsAPI types.GraphicsAPI

	// ContinuousRender enables continuous rendering (game loop style).
	// When false (default), renders only when RequestRedraw() is called
	// or when events occur (resize, input, etc.) - more power efficient.
	// When true, renders every frame at VSync rate - suitable for games/animations.
	ContinuousRender bool

	// Frameless removes the OS window chrome (title bar, borders).
	// When true, the application must provide its own title bar via
	// WindowChrome.SetHitTestCallback for drag, resize, and button regions.
	Frameless bool
}

// DefaultConfig returns sensible default configuration.
// By default, uses continuous rendering (game loop style).
// For power-efficient UI apps, use WithContinuousRender(false).
// DefaultConfig returns a sensible default configuration.
//
// The graphics API can be overridden via the GOGPU_GRAPHICS_API environment variable:
//
//	GOGPU_GRAPHICS_API=vulkan   — force Vulkan
//	GOGPU_GRAPHICS_API=dx12     — force DirectX 12
//	GOGPU_GRAPHICS_API=metal    — force Metal
//	GOGPU_GRAPHICS_API=gles     — force OpenGL ES
//	GOGPU_GRAPHICS_API=software — force CPU software rasterizer
//
// This is useful for testing different backends without modifying code.
// WithGraphicsAPI() in code takes precedence over the environment variable.
func DefaultConfig() Config {
	return Config{
		Title:            "GoGPU Application",
		Width:            800,
		Height:           600,
		Resizable:        true,
		VSync:            true,
		ContinuousRender: true,
		GraphicsAPI:      graphicsAPIFromEnv(),
	}
}

// graphicsAPIFromEnv reads GOGPU_GRAPHICS_API environment variable.
func graphicsAPIFromEnv() types.GraphicsAPI {
	v := strings.ToLower(os.Getenv("GOGPU_GRAPHICS_API"))
	switch v {
	case "vulkan", "vk":
		return types.GraphicsAPIVulkan
	case "dx12", "d3d12", "directx":
		return types.GraphicsAPIDX12
	case "metal":
		return types.GraphicsAPIMetal
	case "gles", "gl", "opengl":
		return types.GraphicsAPIGLES
	case "software", "sw", "cpu":
		return types.GraphicsAPISoftware
	default:
		return types.GraphicsAPIAuto
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
// Use types.BackendRust for maximum performance (requires gpu library).
// Use types.BackendNative for zero dependencies (pure Go, may be slower).
// Use types.BackendAuto (default) to automatically select the best available.
func (c Config) WithBackend(backend types.BackendType) Config {
	c.Backend = backend
	return c
}

// WithGraphicsAPI returns a copy with the graphics API set.
// Use types.GraphicsAPIVulkan to force Vulkan (Windows/Linux).
// Use types.GraphicsAPIDX12 to force DirectX 12 (Windows only).
// Use types.GraphicsAPIMetal to force Metal (macOS only).
// Use types.GraphicsAPIAuto (default) to let the platform choose.
func (c Config) WithGraphicsAPI(api types.GraphicsAPI) Config {
	c.GraphicsAPI = api
	return c
}

// WithVSync enables or disables vertical synchronization.
// When true (default): presentation synchronized with display refresh rate.
// When false: frames presented immediately without waiting for vblank.
func (c Config) WithVSync(vsync bool) Config {
	c.VSync = vsync
	return c
}

// WithContinuousRender sets the rendering mode.
// When true (default): renders every frame at VSync rate - for games/animations.
// When false: renders only on RequestRedraw() or events - power efficient for UI.
func (c Config) WithContinuousRender(continuous bool) Config {
	c.ContinuousRender = continuous
	return c
}

// WithFrameless enables or disables frameless window mode.
// When true, the OS title bar and borders are removed.
// Use WindowChrome.SetHitTestCallback to define drag, resize, and button regions.
func (c Config) WithFrameless(frameless bool) Config {
	c.Frameless = frameless
	return c
}

// Re-export backend types for convenience.
const (
	BackendAuto   = types.BackendAuto
	BackendRust   = types.BackendRust
	BackendNative = types.BackendNative
	BackendGo     = types.BackendGo // Alias for BackendNative
)

// Re-export graphics API types for convenience.
const (
	GraphicsAPIAuto     = types.GraphicsAPIAuto
	GraphicsAPIVulkan   = types.GraphicsAPIVulkan
	GraphicsAPIDX12     = types.GraphicsAPIDX12
	GraphicsAPIMetal    = types.GraphicsAPIMetal
	GraphicsAPIGLES     = types.GraphicsAPIGLES
	GraphicsAPISoftware = types.GraphicsAPISoftware
)
