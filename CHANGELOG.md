# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.2] - 2025-12-26

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.7.1 → v0.7.2
  - Fixes Metal CommandEncoder state bug (wgpu Issue #24)
  - Metal backend now properly tracks recording state via `cmdBuffer != 0`
- Updated dependency: `github.com/gogpu/naga` v0.5.0 → v0.6.0
  - Latest shader compiler with GLSL backend support

### Notes
- This is a maintenance release to pick up critical Metal backend fix
- No API changes, drop-in replacement for v0.8.1

## [0.8.1] - 2025-12-26

### Fixed
- **macOS Zero Dimension Crash** — Fixes Issue [#20](https://github.com/gogpu/gogpu/issues/20)
  - Added `surfaceConfigured` flag to track surface state
  - Deferred surface configuration when window has zero dimensions
  - `BeginFrame()` returns false if surface is not configured
  - `Resize()` properly configures surface when valid dimensions arrive
  - Follows wgpu-core pattern for handling minimized/invisible windows

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.7.0 → v0.7.1
  - Uses new `ErrZeroArea` sentinel error from HAL

### Notes
- macOS window visibility is async — initial GetSize() may return 0,0
- Triangle example now properly waits for valid window dimensions

## [0.8.0] - 2025-12-24

### Fixed
- **Metal Backend Blank Window** — Present() was a NO-OP and didn't call HAL's Queue.Present() method
  - Properly wires gogpu's Present() to HAL Queue.Present()
  - Added Surface→Device tracking via registry mappings for correct queue lookup
  - Added zero-dimension guard to skip rendering when window is minimized

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.6.1 → v0.7.0
  - WGSL→MSL shader compilation via naga
  - CreateRenderPipeline implementation for Metal

## [0.7.2] - 2025-12-25

### Fixed
- **macOS ARM64 Main Thread Crash** — Fixes `nextEventMatchingMask should only be called from the Main Thread`
  - Added `runtime.LockOSThread()` in darwin platform init to pin main goroutine to main OS thread
  - macOS Cocoa/AppKit requires ALL UI operations on the main thread (thread 0)
  - This is the standard approach used by Gio, Ebitengine, Fyne, and go-gl/glfw
- **CAMetalLayer Initialization Order** — Fixes `CAMetalLayer ignoring invalid setDrawableSize width=0 height=0`
  - Layer is now attached to view before setting drawable size
  - Drawable size is set after window becomes visible
  - Added validation to skip SetDrawableSize if dimensions are 0

### Changed
- Renamed internal `runtime` variable to `objcRT` to avoid conflict with standard library `runtime` package
- Updated darwin package documentation with main thread requirements

### Notes
- Fixes [#10](https://github.com/gogpu/gogpu/issues/10) (macOS ARM64 crash)
- **Community Testing Requested**: Pure Go backend on macOS ARM64 (M1/M2/M3/M4)

## [0.7.0] - 2025-12-24

### Added
- **Cross-Platform Pure Go Backend** — All major platforms now supported!
  - **macOS Metal backend** (`gpu/backend/native/metal.go`) — Pure Go via goffi
  - **Linux Vulkan backend** — Extended from Windows-only
  - Shared `ResourceRegistry` across all platforms
- Platform support matrix (Pure Go backend):
  | Platform | Backend | Status |
  |----------|---------|--------|
  | Windows | Vulkan | ✅ Working |
  | Linux | Vulkan | ✅ Working |
  | macOS | Metal | ✅ Working |

### Changed
- Build tags restructured for cross-platform support:
  - `vulkan.go`: `windows || linux`
  - `metal.go`: `darwin`
  - `native.go`: `!windows && !linux && !darwin` (stub for unsupported)

### Notes
- **Community Testing Requested**: Pure Go backend on macOS and Linux
- Closes [#10](https://github.com/gogpu/gogpu/issues/10)

## [0.6.2] - 2025-12-24

### Changed
- Updated dependency: go-webgpu/webgpu v0.1.0 → v0.1.1
- Updated dependency: go-webgpu/goffi v0.3.2 → v0.3.3
  - Fixes PointerType for ARM64 macOS in Pure Go backends

## [0.6.1] - 2025-12-23

### Fixed
- **macOS Apple Silicon (ARM64) support** — Updated goffi to v0.3.2
  - Fixes runtime failure on M1/M2/M3/M4 Macs
  - HFA structs (NSRect, NSPoint, NSSize) now correctly passed via float registers
  - Resolves: `darwin: failed to create NSAutoreleasePool`

### Changed
- Updated dependency: go-webgpu/goffi v0.3.1 → v0.3.2

## [0.6.0] - 2025-12-23

### Added
- **Linux X11 Platform** (Pure Go, ~5,000 LOC)
  - Full X11 wire protocol implementation (no libX11/libxcb dependency)
  - Connection management with MIT-MAGIC-COOKIE-1 authentication
  - Window creation and management (CreateWindow, MapWindow, DestroyWindow)
  - Event handling: KeyPress, KeyRelease, ButtonPress, ButtonRelease, MotionNotify, Expose, ConfigureNotify, ClientMessage
  - Atom interning with caching for performance
  - Keyboard mapping (keycodes to keysyms)
  - ICCCM/EWMH compliance (WM_DELETE_WINDOW, _NET_WM_NAME)
  - Cross-compilable from Windows/macOS to Linux
- Platform auto-selection: Wayland preferred if `WAYLAND_DISPLAY` set, X11 fallback if `DISPLAY` set

### Changed
- Updated dependency: gogpu/wgpu v0.5.0 → v0.6.0

### Notes
- **Community Testing Requested**: X11 implementation needs testing on real Linux X11 systems (Ubuntu, Fedora, Arch, etc.)

## [0.5.0] - 2025-12-23

### Added
- **macOS Cocoa Platform** (Pure Go, ~950 LOC)
  - Objective-C runtime via goffi (go-webgpu/goffi)
  - NSApplication lifecycle management
  - NSWindow and NSView creation
  - CAMetalLayer integration for GPU rendering
  - Cached selector system for performance
  - Cross-compilable from Windows/Linux to macOS
- **Platform types for macOS**
  - CGFloat, CGPoint, CGSize, CGRect
  - NSWindowStyleMask constants
  - NSBackingStoreType constants

### Changed
- Updated ecosystem: wgpu v0.6.0 (Metal backend), naga v0.5.0 (MSL backend)
- Pre-release check script now uses kolkov/racedetector (Pure Go, no CGO)

### Notes
- **Community Testing Requested**: macOS Cocoa implementation needs testing on real macOS systems (12+ Monterey)
- Metal backend available in wgpu v0.6.0
- MSL shader compilation available in naga v0.5.0

## [0.4.0] - 2025-12-21

### Added
- **Linux Wayland Platform** (Pure Go, ~5,700 LOC)
  - Full Wayland wire protocol implementation (no libwayland-client dependency)
  - Core interfaces: wl_display, wl_registry, wl_compositor, wl_surface
  - XDG Shell: xdg_wm_base, xdg_surface, xdg_toplevel for window management
  - Input handling: wl_seat, wl_keyboard, wl_pointer
  - Frame synchronization via wl_callback
  - Cross-compilable from Windows/macOS to Linux
- **Wayland Wire Protocol**
  - Message encoding/decoding with 24.8 fixed-point support
  - File descriptor passing via Unix sockets (SCM_RIGHTS)
  - Object ID allocation and management
- **Unit Tests** for Wayland package
  - Wire protocol tests
  - Compositor, XDG Shell, Input tests
  - 312 test cases

### Changed
- `platform_linux.go` now implements full Wayland windowing (was stub)
- Updated ecosystem: wgpu v0.5.0, gg v0.9.2

### Notes
- **Community Testing Requested**: Wayland implementation needs testing on real Linux systems with Wayland compositors (GNOME 45+, KDE Plasma 6, Sway, etc.)
- X11 support planned for next release

## [0.3.0] - 2025-12-10

### Added
- **Build Tags for Backend Selection**
  - `-tags rust` — Only Rust backend (production)
  - `-tags purego` — Only Pure Go backend (zero dependencies)
  - Default: both backends compiled, runtime selection
- **Backend Registry System**
  - `gpu/registry.go` — Centralized backend registration
  - Auto-discovery via `init()` functions
  - `RegisterBackend()`, `SelectBestBackend()`, `AvailableBackends()`
- **Native Go Backend Integration**
  - Vulkan backend via gogpu/wgpu
  - Cross-platform support (Windows/Linux/macOS)

### Changed
- Updated ecosystem documentation with wgpu v0.3.0 (software backend)

## [0.2.0] - 2025-12-07

### Added
- **Texture Loading API**
  - `LoadTexture(path)` — Load from PNG/JPEG files
  - `NewTextureFromImage(img)` — Create from image.Image
  - `NewTextureFromRGBA(w, h, data)` — Create from raw RGBA pixels
  - `TextureOptions` — Configure filtering and address modes
- **Dual Backend Architecture** — Choose between Rust and Pure Go
  - `WithBackend(gogpu.BackendRust)` — Maximum performance
  - `WithBackend(gogpu.BackendGo)` — Zero dependencies
- **Backend Abstraction Layer**
  - `gpu/backend.go` — Backend interface definition
  - `gpu/backend/rust/` — Rust backend wrapper (wgpu-native)
  - `gpu/backend/native/` — Native Go backend
- **gpu/types Package** — Standalone types
- **CI/CD Infrastructure**
  - GitHub Actions workflow
  - Codecov integration
  - golangci-lint configuration

### Changed
- Renamed `math/` package to `gmath/` to avoid stdlib conflict

## [0.1.0] - 2025-12-05

### Added
- **First Working Rendering** — Triangle renders on screen!
- **Simple API** — ~20 lines vs 480+ lines of raw WebGPU
  ```go
  app := gogpu.NewApp(gogpu.DefaultConfig())
  app.OnDraw(func(ctx *gogpu.Context) {
      ctx.DrawTriangleColor(gmath.DarkGray)
  })
  app.Run()
  ```
- **Core Packages**
  - `app.go` — Application lifecycle
  - `config.go` — Configuration with builder pattern
  - `context.go` — Drawing context API
  - `renderer.go` — WebGPU rendering
  - `shader.go` — Built-in WGSL shaders
- **Platform Abstraction**
  - Windows implementation (Win32)
  - macOS/Linux stubs
- **Math Library** (`gmath/`)
  - Vec2, Vec3, Vec4, Mat4, Color
- **Examples**
  - `examples/triangle/` — Simple triangle demo

[Unreleased]: https://github.com/gogpu/gogpu/compare/v0.8.2...HEAD
[0.8.2]: https://github.com/gogpu/gogpu/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/gogpu/gogpu/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/gogpu/gogpu/compare/v0.7.2...v0.8.0
[0.7.2]: https://github.com/gogpu/gogpu/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/gogpu/gogpu/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/gogpu/gogpu/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/gogpu/gogpu/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/gogpu/gogpu/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/gogpu/gogpu/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/gogpu/gogpu/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/gogpu/gogpu/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/gogpu/gogpu/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/gogpu/gogpu/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/gogpu/gogpu/releases/tag/v0.1.0
