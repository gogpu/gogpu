# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.13.3] - 2026-01-29

### Changed

- **Update dependencies** for webgpu.h spec compliance
  - `github.com/gogpu/gpucontext` v0.3.0 → v0.3.1
  - `github.com/gogpu/gputypes` v0.2.0 (webgpu.h spec-compliant enum values)
  - `github.com/gogpu/wgpu` v0.11.1 → v0.11.2 (CompositeAlphaMode naming fix)

### Added

- **gg integration example** (`examples/gg_integration/`) — Demonstrates gg 2D → gogpu GPU pipeline

## [0.13.2] - 2026-01-29

### Changed

#### Clean Architecture: Remove gputypes Re-export Layer
- **BREAKING:** `gpu/types/` no longer re-exports `gputypes` types
- **Direct imports required:** Use `github.com/gogpu/gputypes` directly for WebGPU types
- `gpu/types/` now contains only gogpu-specific types: `BackendType`, handles, `SurfaceStatus`, descriptors
- Deleted `gpu/types/gputypes.go` (~20KB re-export layer)
- Created `gpu/types/descriptors.go` with gogpu-specific descriptors importing gputypes

#### Migration Guide
```go
// Before (v0.13.1)
import "github.com/gogpu/gogpu/gpu/types"
format := types.TextureFormatRGBA8Unorm

// After (v0.13.2)
import "github.com/gogpu/gputypes"
format := gputypes.TextureFormatRGBA8Unorm
```

### Fixed
- **gputypes webgpu.h compliance** — All enum values now match webgpu.h specification exactly
  - TextureFormat values corrected (BC formats 0x32-0x3F, depth/stencil 0x2C-0x31)
  - Added missing formats: R16Unorm, R16Snorm, RG16Unorm, RG16Snorm, RGBA16Unorm, RGBA16Snorm

### Dependencies
- Update `github.com/gogpu/gputypes` v0.1.0 → v0.2.0 (webgpu.h compliance)

## [0.13.1] - 2026-01-29

**Note:** v0.13.0 was cached by Go module proxy without gputypes migration. Use v0.13.1.

### Added

#### DrawTexture API
- **Context.DrawTexture()** — Draw textures directly to the screen
- **Texture.UpdateData()** — Update texture data from CPU
- **Textured quad pipeline** — GPU rendering for textures

#### Multi-Thread Architecture
- **Enterprise-level multi-thread rendering** (Ebiten/Gio pattern)
  - Main thread: Window events only (Win32/Cocoa/X11 message pump)
  - Render thread: All GPU operations (device, swapchain, commands)
  - Deferred resize: `RequestResize()` / `ConsumePendingResize()` pattern
- **internal/thread package** — Thread management for GPU operations

### Changed

#### gputypes Migration
- **Unified WebGPU types** via `github.com/gogpu/gputypes` v0.1.0
- **No more type converters** — HAL uses gputypes directly
- Delete redundant `convert.go` and `convert_darwin.go`
- `gpu/types/` now re-exports gputypes for backward compatibility

### Fixed
- **Window "Not Responding"** during resize/move on Windows
- **Resize cursor stuck** for 5-10 seconds after resize ends

### Dependencies
- Add `github.com/gogpu/gputypes` v0.1.0
- Update `github.com/gogpu/gpucontext` v0.2.0 → v0.3.0
- Update `github.com/gogpu/wgpu` v0.10.2 → v0.11.1

## [0.12.0] - 2026-01-27

### Added

#### gpucontext Integration
- **GPUContextProvider()** — Returns `gpucontext.DeviceProvider` for cross-package integration
  - `Device()` — Returns `gpucontext.Device` interface
  - `Queue()` — Returns `gpucontext.Queue` interface
  - `Adapter()` — Returns `gpucontext.Adapter` interface
  - `SurfaceFormat()` — Returns `gpucontext.TextureFormat`
- **EventSource()** — Returns `gpucontext.EventSource` for UI framework integration
  - `OnKeyPress/OnKeyRelease` — Keyboard events
  - `OnMouseMove/OnMousePress/OnMouseRelease` — Mouse events
  - `OnScroll` — Scroll wheel events
  - `OnResize` — Window resize events
  - `OnFocus` — Focus change events
  - `OnIME*` — Input Method Editor events for international text input
- **Example** (`examples/gpucontext_integration/`) — Demonstrates cross-package integration

### Dependencies
- Add `github.com/gogpu/gpucontext` v0.2.0

## [0.11.2] - 2026-01-24

### Changed

- **wgpu v0.10.2** — FFI build tag fix
  - Clear error message when CGO enabled: `undefined: GOFFI_REQUIRES_CGO_ENABLED_0`
  - See [wgpu v0.10.2 release](https://github.com/gogpu/wgpu/releases/tag/v0.10.2)

### Dependencies
- Update `github.com/gogpu/wgpu` v0.10.1 → v0.10.2
- Update `github.com/go-webgpu/goffi` v0.3.7 → v0.3.8

## [0.11.1] - 2026-01-16

Window responsiveness fix for Pure Go Vulkan backend.

### Added
- **GPU Timing Example** (`examples/gpu_timing`) — Diagnostic tool for frame timing analysis
  - Measures BeginFrame and Draw phases separately
  - Shows avg/max timing per second for performance debugging

### Changed
- **Non-blocking GPU acquire** — Improved window responsiveness
  - Handle `SurfaceStatusTimeout` separately in renderer (skip frame, no reconfigure)
  - Works with wgpu v0.10.1 non-blocking swapchain acquire

### Fixed
- Window lag during resize/drag operations on Windows
- "Not responding" window state during GPU-bound rendering

### Dependencies
- Update `github.com/gogpu/wgpu` v0.10.0 → v0.10.1

## [0.11.0] - 2026-01-16

### Changed
- **BREAKING: Pure Go is now the default backend** ([#40])
  - No build tags needed for Pure Go — just `go build ./...`
  - Rust backend now opt-in with `-tags rust`
  - Unified approach across gogpu ecosystem (same as gg)

### Removed
- `-tags purego` — no longer needed, Pure Go is default
- `rust_stub.go` — no longer needed with opt-in approach

### Refactored
- `renderer.go` — uses registry pattern instead of direct rust import
- Build tags simplified: `rust && windows` for Rust backend files

## [0.10.1] - 2026-01-16

### Fixed
- **Pure Go Build Tags** — `-tags purego` now correctly excludes Rust backend ([#40])
  - `rust.go`: `windows` → `windows && !purego`
  - `rust_stub.go`: `!windows` → `!windows || purego`

### Documentation
- Added quick start tip for `-tags purego` in README
- Added troubleshooting note for `wgpu_native.dll` error

## [0.10.0] - 2026-01-15

### Added

#### DeviceProvider Interface
- **DeviceProvider Interface** — Standardized GPU resource access for external libraries
  - `Backend()` — Access to underlying gpu.Backend
  - `Device()` — GPU device handle
  - `Queue()` — Command queue for submission
  - `SurfaceFormat()` — Texture format for surface rendering
- **App.DeviceProvider()** — Access GPU resources from App instance

#### Compute Shader Support
- **gpu.Backend.CreateComputePipeline()** — Compute pipeline creation
- **gpu.Backend.CreateBindGroupLayout()** — Bind group layout for compute
- **gpu.Backend.CreateBindGroup()** — Bind group with storage buffers
- **gpu.Backend.CreateBuffer()** — Buffer creation with compute usage
- Full compute shader support in both Rust and Native backends

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.9.3 → v0.10.0
  - HAL Backend Integration layer

### Removed
- **ggrender package** — Removed to eliminate circular dependency with gg
  - gogpu/gg has its own native GPU backend (`backend/native/`) using gogpu/wgpu
  - Use gg's built-in GPU backend directly instead

## [0.9.3] - 2026-01-10

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.9.2 → v0.9.3
  - Intel Vulkan compatibility: VkRenderPass, wgpu-style swapchain sync
  - Triangle rendering works on Intel Iris Xe Graphics
- Updated dependency: `github.com/gogpu/naga` v0.8.3 → v0.8.4
  - SPIR-V instruction ordering fix for Intel Vulkan

## [0.9.2] - 2026-01-05

### Fixed

#### CI
- **Metal Tests on CI** — Skip Metal-dependent darwin tests on GitHub Actions ([#36])
  - Metal unavailable in virtualized macOS runners
  - See: https://github.com/actions/runner-images/discussions/6138

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.9.1 → v0.9.2
  - Metal NSString double-free fix on autorelease pool drain

[#36]: https://github.com/gogpu/gogpu/pull/36

## [0.9.1] - 2026-01-05

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.9.0 → v0.9.1
  - Fix vkDestroyDevice memory leak
  - Vulkan features mapping (9 features)
  - Vulkan limits mapping (25+ limits)

## [0.9.0] - 2026-01-05

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.8.8 → v0.9.0
  - Core-HAL Bridge implementation
  - Snatchable pattern for safe resource destruction
  - TrackerIndex Allocator for state tracking
  - Buffer State Tracker
  - 58 TODO comments replaced with proper documentation

## [0.8.9] - 2026-01-04

### Fixed

#### CI
- **Metal Tests on CI** — Skip Metal-dependent darwin tests on GitHub Actions
  - Metal unavailable in virtualized macOS runners
  - See: https://github.com/actions/runner-images/discussions/6138

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.8.7 → v0.8.8
  - Skip Metal tests on CI
  - MSL `[[position]]` attribute fix via naga v0.8.3
- Updated dependency: `github.com/gogpu/naga` v0.8.2 → v0.8.3
  - Fixes MSL `[[position]]` attribute placement (now on struct member, not function)

## [0.8.8] - 2026-01-04

### Fixed

#### macOS ARM64
- **ObjC Typed Arguments** — Proper type-safe wrappers for ARM64 AAPCS64 ABI compliance
- **Triangle Demo** — Fixed shader WGSL and improved error handling
- **Panic Safety** — Fixed segfault on panic with ObjC interop

### Added
- **Darwin ObjC Tests** — Comprehensive test coverage (1000+ lines in `darwin_objc_test.go`)
- **Metal Backend Tests** — Platform-specific Metal tests
- **Backend Registry Tests** — Backend selection and registration tests

### Changed
- Updated dependency: `github.com/go-webgpu/goffi` v0.3.6 → v0.3.7
- Updated dependency: `github.com/go-webgpu/webgpu` v0.1.3 → v0.1.4
- Updated dependency: `github.com/gogpu/wgpu` v0.8.6 → v0.8.7

### Contributors
- @ppoage — ARM64 ObjC fixes, tests, and triangle demo fix

## [0.8.7] - 2025-12-29

### Fixed
- **macOS ARM64 Blank Window** — Final fix for Issue [#24](https://github.com/gogpu/gogpu/issues/24)
  - `GetSize()` now returns correct dimensions on Apple Silicon (M1/M2/M3/M4)
  - Triangle example renders correctly on macOS ARM64

### Changed
- Updated dependency: `github.com/go-webgpu/webgpu` v0.1.2 → v0.1.3
  - Includes goffi v0.3.6 with ARM64 ABI fixes
- Updated dependency: `github.com/go-webgpu/goffi` v0.3.5 → v0.3.6
  - **ARM64 HFA Returns** — `NSRect` (4×float64) correctly returns on Apple Silicon
  - **Large Struct Returns** — Structs >16 bytes use X8 register properly
  - Fixes Objective-C `objc_msgSend` struct return calling convention
- Updated dependency: `github.com/gogpu/wgpu` v0.8.5 → v0.8.6
  - Metal double present fix
  - goffi v0.3.6 integration

## [0.8.6] - 2025-12-29

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.8.4 → v0.8.5
  - DX12 backend now auto-registers on Windows
  - Windows backend priority: Vulkan → DX12 → GLES → Software

## [0.8.5] - 2025-12-29

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.8.3 → v0.8.4
  - Fixes missing `clamp()` WGSL built-in function (naga v0.8.1)
- Made README version-agnostic (removed hardcoded version numbers)

## [0.8.4] - 2025-12-29

### Fixed
- **macOS Metal Blank Window** — Fixes Issue [#24](https://github.com/gogpu/gogpu/issues/24)
  - Root cause: Metal presentation timing and resource release order
  - Fix: Wire up drawable attachment to command buffer for `presentDrawable:` before `commit`
  - Fix: Reorder `EndFrame()` to present surface before releasing texture resources
  - Added `attachDrawableToCommandBuffer()` helper in native Metal backend
  - Added `GetAnySurfaceTexture()` to registry for Metal drawable access

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.8.1 → v0.8.3
  - Metal present timing: schedule `presentDrawable:` before `commit`
  - TextureView NSRange parameters fix
- Updated dependency: `github.com/go-webgpu/webgpu` v0.1.1 → v0.1.2
- Updated dependency: `github.com/go-webgpu/goffi` v0.3.3 → v0.3.5

## [0.8.3] - 2025-12-29

### Changed
- Updated dependency: `github.com/gogpu/wgpu` v0.7.2 → v0.8.1
  - DX12 backend complete
  - Intel GPU COM calling convention fix
- Updated dependency: `github.com/gogpu/naga` v0.6.0 → v0.8.0
  - HLSL backend for DirectX 11/12
  - All 4 shader backends stable

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
  app.OnDraw(func(dc *gogpu.Context) {
      dc.DrawTriangleColor(gmath.DarkGray)
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

[Unreleased]: https://github.com/gogpu/gogpu/compare/v0.13.3...HEAD
[0.13.3]: https://github.com/gogpu/gogpu/compare/v0.13.2...v0.13.3
[0.13.2]: https://github.com/gogpu/gogpu/compare/v0.13.1...v0.13.2
[0.13.1]: https://github.com/gogpu/gogpu/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/gogpu/gogpu/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/gogpu/gogpu/compare/v0.11.2...v0.12.0
[0.11.2]: https://github.com/gogpu/gogpu/compare/v0.11.1...v0.11.2
[0.11.1]: https://github.com/gogpu/gogpu/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/gogpu/gogpu/compare/v0.10.1...v0.11.0
[0.10.1]: https://github.com/gogpu/gogpu/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/gogpu/gogpu/compare/v0.9.3...v0.10.0
[0.9.3]: https://github.com/gogpu/gogpu/compare/v0.9.2...v0.9.3
[0.9.2]: https://github.com/gogpu/gogpu/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/gogpu/gogpu/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/gogpu/gogpu/compare/v0.8.9...v0.9.0
[0.8.9]: https://github.com/gogpu/gogpu/compare/v0.8.8...v0.8.9
[0.8.8]: https://github.com/gogpu/gogpu/compare/v0.8.7...v0.8.8
[0.8.7]: https://github.com/gogpu/gogpu/compare/v0.8.6...v0.8.7
[0.8.6]: https://github.com/gogpu/gogpu/compare/v0.8.5...v0.8.6
[0.8.5]: https://github.com/gogpu/gogpu/compare/v0.8.4...v0.8.5
[0.8.4]: https://github.com/gogpu/gogpu/compare/v0.8.3...v0.8.4
[0.8.3]: https://github.com/gogpu/gogpu/compare/v0.8.2...v0.8.3
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
