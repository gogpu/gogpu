package gogpu

import (
	"image"
	"os"
	"strings"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
)

const defaultTitle = "GoGPU Application"

// Environment variable values for GOGPU_GRAPHICS_API and GOGPU_RENDER_MODE.
const (
	envVulkan   = "vulkan"
	envDX12     = "dx12"
	envMetal    = "metal"
	envGLES     = "gles"
	envSoftware = "software"
	envCPU      = "cpu"
	envGPU      = "gpu"
)

// RenderMode controls the 2D rendering path selection (ADR-020).
// This determines whether gg uses GPU accelerator or CPU rasterizer.
type RenderMode int

const (
	// RenderModeAuto selects rendering path based on adapter type.
	// Software adapter → CPU rasterizer (fast), real GPU → GPU accelerator.
	RenderModeAuto RenderMode = iota
	// RenderModeCPU forces CPU rasterizer even with a real GPU (for benchmarking).
	RenderModeCPU
	// RenderModeGPU forces GPU path even on software adapter (for shader testing).
	RenderModeGPU
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
	// or when events occur (resize, input, etc.) — power efficient for UI apps.
	// When true, renders every frame at VSync rate — suitable for games/animations.
	// Use WithContinuousRender(true) for game loops.
	ContinuousRender bool

	// Frameless removes the OS window chrome (title bar, borders).
	// When true, the application must provide its own title bar via
	// WindowChrome.SetHitTestCallback for drag, resize, and button regions.
	Frameless bool

	// LayerShell, when non-nil, gives the window a wlr-layer-shell role
	// instead of a regular desktop window: it is placed in a layer, anchored
	// to output edges, and drawn without decorations. This is how shell
	// components (panels, overlays, wallpapers, launchers) are built.
	//
	// Linux/Wayland only, and the compositor must advertise
	// zwlr_layer_shell_v1. Ignored on other platforms.
	//
	// Note that a layer surface has no title bar, no move/resize, and no
	// server-side decorations; window-management APIs such as SetTitle,
	// SetMinSize, SetMaxSize and SetFullscreen do not apply to it.
	LayerShell *LayerShellConfig

	// Transparent enables a per-pixel-alpha window (rounded corners, tray
	// popups, overlays). When true, the swapchain uses
	// CompositeAlphaModePremultiplied when the surface supports it and the
	// platform window is created with the native transparency style
	// (DwmBlurBehind on Windows, NSWindow.isOpaque=false on macOS, ARGB
	// visual on X11, no opaque region on Wayland). The application must
	// clear with alpha=0 for transparent regions.
	Transparent bool

	// PowerPreference specifies GPU power consumption preference for adapter selection.
	// On systems with both integrated and discrete GPUs (e.g. laptops),
	// this controls which GPU is selected.
	// PowerPreferenceNone (default) lets the driver decide.
	PowerPreference gputypes.PowerPreference

	// RenderMode controls 2D rendering path selection (ADR-020).
	// RenderModeAuto (default): CPU rasterizer on software adapter, GPU on real hardware.
	// Can be overridden via GOGPU_RENDER_MODE=auto|cpu|gpu environment variable.
	RenderMode RenderMode

	// TabbingMode controls macOS system window tabbing.
	// Default: TabbingDisallowed (set by DefaultConfig — GLFW/SDL3/Qt6 enterprise pattern).
	// Set TabbingPreferred + TabbingIdentifier for terminal-style tabbing.
	// No-op on Windows/Linux.
	TabbingMode TabbingMode

	// TabbingIdentifier groups windows into the same tab bar.
	// Only effective when TabbingMode is TabbingPreferred or TabbingAutomatic.
	// Windows with the same identifier will be grouped together.
	// See: https://developer.apple.com/documentation/appkit/nswindow/1644704-tabbingidentifier
	TabbingIdentifier string

	// AppName is the application name (displayed in menus).
	AppName string

	// HeaderAlignment controls the position of the title in the OS header bar.
	// HeaderAlignCenter (default): centers the title (standard macOS behavior).
	// HeaderAlignLeft / HeaderAlignRight: on macOS, enables a full-size content
	// view with a transparent title bar so GPU-rendered content fills the header.
	// No-op on Windows and Linux where title position is not OS-configurable.
	HeaderAlignment HeaderAlignment

	// MinWidth is the minimum window width in logical pixels (0 = no constraint).
	MinWidth int

	// MinHeight is the minimum window height in logical pixels (0 = no constraint).
	MinHeight int

	// MaxWidth is the maximum window width in logical pixels (0 = no constraint).
	MaxWidth int

	// MaxHeight is the maximum window height in logical pixels (0 = no constraint).
	MaxHeight int

	// Icon is the window icon displayed in taskbars and window decorations.
	// On X11: set via _NET_WM_ICON. On Windows/macOS: no-op (icons come from app resources).
	Icon image.Image
}

// DefaultConfig returns a sensible default configuration.
// By default, uses event-driven rendering (0% CPU when idle).
// For game loops, use WithContinuousRender(true).
//
// The graphics API can be overridden via the GOGPU_GRAPHICS_API environment variable:
//
//	GOGPU_GRAPHICS_API=vulkan   — force Vulkan
//	GOGPU_GRAPHICS_API=dx12     — force DirectX 12
//	GOGPU_GRAPHICS_API=metal    — force Metal
//	GOGPU_GRAPHICS_API=gles     — force OpenGL ES
//	GOGPU_GRAPHICS_API=software — force CPU software rasterizer
//
// The GPU power preference can be overridden via the GOGPU_POWER_PREFERENCE
// environment variable:
//
//	GOGPU_POWER_PREFERENCE=low  — prefer integrated GPU (power saving)
//	GOGPU_POWER_PREFERENCE=high — prefer discrete GPU (performance)
//
// The 2D render mode can be overridden via the GOGPU_RENDER_MODE environment variable:
//
//	GOGPU_RENDER_MODE=auto — CPU rasterizer on software adapter, GPU on real hardware (default)
//	GOGPU_RENDER_MODE=cpu  — force CPU rasterizer (for benchmarking)
//	GOGPU_RENDER_MODE=gpu  — force GPU path even on software (for shader testing)
//
// WithGraphicsAPI(), WithPowerPreference(), and WithRenderMode() in code take precedence
// over the environment variables.
func DefaultConfig() Config {
	return Config{
		Title:            defaultTitle,
		Width:            800,
		Height:           600,
		Resizable:        true,
		VSync:            true,
		ContinuousRender: false,
		GraphicsAPI:      graphicsAPIFromEnv(),
		PowerPreference:  powerPreferenceFromEnv(),
		RenderMode:       renderModeFromEnv(),
		TabbingMode:      TabbingDisallowed,
	}
}

// graphicsAPIFromEnv reads GOGPU_GRAPHICS_API environment variable.
func graphicsAPIFromEnv() types.GraphicsAPI {
	v := strings.ToLower(os.Getenv("GOGPU_GRAPHICS_API"))
	switch v {
	case envVulkan, "vk":
		return types.GraphicsAPIVulkan
	case envDX12, "d3d12", "directx":
		return types.GraphicsAPIDX12
	case envMetal:
		return types.GraphicsAPIMetal
	case envGLES, "gl", "opengl":
		return types.GraphicsAPIGLES
	case envSoftware, "sw", envCPU:
		return types.GraphicsAPISoftware
	default:
		return types.GraphicsAPIAuto
	}
}

// powerPreferenceFromEnv reads GOGPU_POWER_PREFERENCE environment variable.
func powerPreferenceFromEnv() gputypes.PowerPreference {
	switch strings.ToLower(os.Getenv("GOGPU_POWER_PREFERENCE")) {
	case "lowpower", "low", "integrated":
		return gputypes.PowerPreferenceLowPower
	case "highperformance", "high", "discrete":
		return gputypes.PowerPreferenceHighPerformance
	default:
		return gputypes.PowerPreferenceNone
	}
}

// renderModeFromEnv reads GOGPU_RENDER_MODE environment variable.
func renderModeFromEnv() RenderMode {
	switch strings.ToLower(os.Getenv("GOGPU_RENDER_MODE")) {
	case envCPU:
		return RenderModeCPU
	case envGPU:
		return RenderModeGPU
	default:
		return RenderModeAuto
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
// When true: renders every frame at VSync rate — for games/animations.
// When false (default): renders only on RequestRedraw() or events — power efficient.
func (c Config) WithContinuousRender(continuous bool) Config {
	c.ContinuousRender = continuous
	return c
}

// WithFullscreen starts the window in fullscreen mode.
// On Windows: borderless fullscreen (Chromium/GLFW pattern).
// On macOS: native fullscreen with animation.
// On X11: _NET_WM_STATE_FULLSCREEN via EWMH.
// On Wayland: xdg_toplevel.set_fullscreen.
// Use App.SetFullscreen(false) or App.ToggleFullscreen() to exit at runtime.
func (c Config) WithFullscreen() Config {
	c.Fullscreen = true
	return c
}

// WithFrameless enables or disables frameless window mode.
// When true, the OS title bar and borders are removed.
// Use WindowChrome.SetHitTestCallback to define drag, resize, and button regions.
func (c Config) WithFrameless(frameless bool) Config {
	c.Frameless = frameless
	return c
}

// WithLayerShell makes the window a wlr-layer-shell surface (Linux/Wayland
// only) instead of a regular desktop window.
//
// The zero LayerShellConfig is not useful on its own: a surface with neither a
// size nor anchors is a protocol error, so set Width/Height, or leave them zero
// and anchor opposite edges to fill the output.
func (c Config) WithLayerShell(layer LayerShellConfig) Config {
	c.LayerShell = &layer
	return c
}

// LayerShellConfig describes a wlr-layer-shell surface.
//
// It mirrors the layer-shell protocol: Layer selects the shell layer, Anchor is
// a bitmask of edges, and Keyboard selects whether the surface can take
// keyboard focus. The zero value means "background layer, centered, no keyboard
// input", which is valid but rarely what a panel or overlay wants.
type LayerShellConfig struct {
	// Namespace is the surface namespace compositors use in their rules and in
	// diagnostics (niri's `niri msg layers`, for example). Defaults to "gogpu".
	Namespace string

	// Layer is the shell layer: background, bottom, top or overlay.
	Layer LayerShellLayer

	// Anchor is the set of edges the surface is anchored to. Anchoring
	// opposite edges makes the surface stretch along that axis, which is how
	// an overlay fills the output.
	Anchor LayerShellAnchor

	// Keyboard selects whether the surface receives keyboard input. OnDemand
	// lets the user focus and unfocus it like a normal window (protocol v4+).
	Keyboard LayerShellKeyboard

	// Width and Height are surface-local pixels. Zero asks the compositor to
	// decide, which requires anchoring opposite edges on that axis.
	Width  int32
	Height int32

	// ExclusiveZone is the reserved strut in surface-local pixels. Zero moves
	// the surface to avoid other exclusive surfaces, -1 ignores them entirely
	// (and extends the surface to the anchored edges).
	ExclusiveZone int32
}

// LayerShellLayer is the shell layer a layer surface is drawn in, ordered by
// increasing z-depth.
type LayerShellLayer uint32

// Layer shell layers.
const (
	LayerShellBackground LayerShellLayer = 0
	LayerShellBottom     LayerShellLayer = 1
	LayerShellTop        LayerShellLayer = 2
	LayerShellOverlay    LayerShellLayer = 3
)

// LayerShellAnchor is a bitmask of output edges a layer surface is anchored to.
type LayerShellAnchor uint32

// Layer shell anchor edges. AnchorTop|AnchorLeft is the top-left corner; two
// opposite edges stretch the surface along that axis.
const (
	LayerShellAnchorTop    LayerShellAnchor = 1 << 0
	LayerShellAnchorBottom LayerShellAnchor = 1 << 1
	LayerShellAnchorLeft   LayerShellAnchor = 1 << 2
	LayerShellAnchorRight  LayerShellAnchor = 1 << 3
)

// LayerShellKeyboard describes how keyboard focus is granted to a layer
// surface.
type LayerShellKeyboard uint32

// Layer shell keyboard interactivity modes.
const (
	// LayerShellKeyboardNone never grants keyboard focus (the protocol default).
	LayerShellKeyboardNone LayerShellKeyboard = 0
	// LayerShellKeyboardExclusive asks for exclusive focus, for lock screens and
	// prompts.
	LayerShellKeyboardExclusive LayerShellKeyboard = 1
	// LayerShellKeyboardOnDemand lets the user focus and unfocus the surface the
	// usual way. Requires layer-shell protocol version 4.
	LayerShellKeyboardOnDemand LayerShellKeyboard = 2
)

// WithTransparent enables or disables per-pixel-alpha window transparency.
// When true, the surface selects CompositeAlphaModePremultiplied when
// supported and the platform window is created with its native transparency
// style. Falls back to an opaque surface when the backend does not support
// premultiplied alpha.
//
// Windows backend matrix (see docs/windows-transparency.md):
//   - DX12 (explicit): DirectComposition + WS_EX_NOREDIRECTIONBITMAP, full
//     per-pixel alpha against the desktop.
//   - Vulkan/GLES: DwmEnableBlurBehindWindow legacy path; per-pixel alpha
//     works through the DWM redirection surface.
//   - Software: not currently supported — GDI BitBlt/StretchDIBits does not
//     preserve alpha. Use a GPU backend or UpdateLayeredWindow.
//   - Auto: uses the legacy blur-behind path so all backends stay visible;
//     if wgpu selects DX12, per-pixel alpha may not composite (see docs).
func (c Config) WithTransparent(transparent bool) Config {
	c.Transparent = transparent
	return c
}

// WithPowerPreference returns a copy with the GPU power preference set.
// Use gputypes.PowerPreferenceLowPower to prefer integrated GPU (battery saving).
// Use gputypes.PowerPreferenceHighPerformance to prefer discrete GPU (performance).
// Use gputypes.PowerPreferenceNone (default) to let the driver decide.
func (c Config) WithPowerPreference(pref gputypes.PowerPreference) Config {
	c.PowerPreference = pref
	return c
}

// WithRenderMode sets the 2D rendering path (ADR-020).
// RenderModeAuto: CPU on software adapter, GPU on real hardware.
// RenderModeCPU: force CPU rasterizer. RenderModeGPU: force GPU path.
func (c Config) WithRenderMode(mode RenderMode) Config {
	c.RenderMode = mode
	return c
}

// WithTabbingMode sets the macOS window tabbing mode.
//
// Usage for terminal-style tabbing:
//
//	cfg := gogpu.DefaultConfig().
//	    WithTabbingMode(gogpu.TabbingPreferred).
//	    WithTabbingIdentifier("com.myapp.tabs")
//	app := gogpu.NewApp(cfg)
func (c Config) WithTabbingMode(mode TabbingMode) Config {
	c.TabbingMode = mode
	return c
}

// WithTabbingIdentifier sets the tabbing identifier.
func (c Config) WithTabbingIdentifier(id string) Config {
	c.TabbingIdentifier = id
	return c
}

// WithAppName sets the application name.
func (c Config) WithAppName(name string) Config {
	c.AppName = name
	return c
}

// WithMinSize returns a copy with the minimum window size set in logical pixels.
// Use 0 for both dimensions to clear any minimum constraint.
func (c Config) WithMinSize(width, height int) Config {
	c.MinWidth = width
	c.MinHeight = height
	return c
}

// WithMaxSize returns a copy with the maximum window size set in logical pixels.
// Use 0 for both dimensions to clear any maximum constraint.
func (c Config) WithMaxSize(width, height int) Config {
	c.MaxWidth = width
	c.MaxHeight = height
	return c
}

// WithHeaderAlignment sets the title alignment in the window header bar.
// See Config.HeaderAlignment for platform-specific behavior.
func (c Config) WithHeaderAlignment(alignment HeaderAlignment) Config {
	c.HeaderAlignment = alignment
	return c
}

// WithResizable enables or disables window resizing.
// When true (default): user can resize the window by dragging its edges.
// When false: window size is fixed at the values set by WithSize.
func (c Config) WithResizable(resizable bool) Config {
	c.Resizable = resizable
	return c
}

// WithIcon sets the window icon shown in taskbars and decorations.
// On X11: set via _NET_WM_ICON (EWMH). No-op on Windows/macOS.
func (c Config) WithIcon(img image.Image) Config {
	c.Icon = img
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

// Re-export power preference types for convenience.
const (
	PowerPreferenceNone            = gputypes.PowerPreferenceNone
	PowerPreferenceLowPower        = gputypes.PowerPreferenceLowPower
	PowerPreferenceHighPerformance = gputypes.PowerPreferenceHighPerformance
)

// TabbingMode controls macOS system window tabbing behavior.
// No-op on non-macOS platforms.
type TabbingMode int

const (
	// TabbingAutomatic lets the system decide whether to tab windows.
	TabbingAutomatic TabbingMode = 0
	// TabbingPreferred indicates the window prefers to open as a tab when possible.
	TabbingPreferred TabbingMode = 1
	// TabbingDisallowed explicitly prevents the window from being grouped into tabs.
	TabbingDisallowed TabbingMode = 2
)
