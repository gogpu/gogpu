# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[Unreleased]: https://github.com/gogpu/gogpu/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/gogpu/gogpu/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/gogpu/gogpu/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/gogpu/gogpu/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/gogpu/gogpu/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/gogpu/gogpu/releases/tag/v0.1.0
