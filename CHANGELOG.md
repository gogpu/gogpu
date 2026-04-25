# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.29.2] - 2026-04-25

### Fixed

- **Vulkan validation error on uniform buffer** (BUG-GOGPU-UNIFORM-TRANSFER-DST-001) — `texQuadUniformBuffer` was created with `Uniform | MapWrite` but `Queue.WriteBuffer()` requires `CopyDst` for PendingWrites staging copy. Fixed to `Uniform | CopyDst`. Also removed dead `MapWrite` and `MappedAtCreation` flags (buffer was never re-mapped after creation).

### Changed

- **deps:** update wgpu v0.26.4 — Vulkan PRESENT_SRC_KHR layout transition fix (BUG-WGPU-VK-006), per-image layout tracking, fence-synchronized barrier pool

## [0.29.1] - 2026-04-25

### Changed

- **deps:** update wgpu v0.26.3 — Vulkan offscreen submit semaphore fix (BUG-WGPU-VK-005), automatic resource cleanup

## [0.29.0] - 2026-04-25

### Added

- **Damage-aware surface presentation** — `Context.SetDamageRects([]image.Rectangle)` passes dirty regions to the platform compositor, allowing it to skip recompositing unchanged pixels. Rects are in physical pixels (top-left origin), consumed after presentation. Works on all wgpu backends: Vulkan (`VK_KHR_incremental_present`), DX12 (`Present1` + dirty rects), GLES (`eglSwapBuffersWithDamageKHR`), Software (partial `BitBlt`/`XPutImage`). `ContextRenderTarget.SetDamageRects()` adapter for ggcanvas integration. ADR-013.
- **`Texture.TextureView()`** — returns `gpucontext.TextureView` for duck-typed access from packages that cannot import gogpu (e.g., gg/ggcanvas uses Go structural typing to call this method without import cycle).

### Changed

- **deps:** update wgpu v0.26.2 — damage-aware `PresentWithDamage` API, automatic Buffer/BindGroup cleanup via `runtime.AddCleanup`, zero-alloc WriteBuffer batching

## [0.28.1] - 2026-04-23

### Added

- **EventFocus** — window focus/blur events on all 4 platforms for multi-window VSync strategy (ADR-010). Win32: `WM_SETFOCUS`/`WM_KILLFOCUS`. X11: `FocusIn`/`FocusOut` with `NotifyPointer`/`NotifyNormal` filtering. Wayland: `keyboard_enter`/`keyboard_leave`. macOS: `IsKeyWindow()` polling. Events carry `WindowID` for multi-window routing. `WindowManager.setFocus()` wired on focus gain. (FEAT-MW-FOCUS-001)
- **WindowID on all Win32 events** — `EventClose`, `EventResize` now include `WindowID` for proper multi-window event routing

## [0.28.0] - 2026-04-23

### Added

- **Multi-window support** — `App.NewWindow(config)` creates additional windows, each with its own GPU surface, swapchain, and per-window callbacks (`SetOnDraw`, `SetOnResize`, `SetOnClose`). Shared GPU device across all windows (one Instance/Adapter/Device, N Surfaces). Multi-window frame loop renders all visible windows per tick. Secondary windows use VSync=false to prevent N × VBlank delay (ADR-010). Verified with two-window demo on Win32 Vulkan.
- **PlatformManager / PlatformWindow interfaces** — process-level operations (`Init`, `CreateWindow`, `PollEvents`, `WaitEvents`, clipboard, system preferences) separated from per-window operations (`GetHandle`, size/scale queries, cursor, input callbacks, lifecycle). Win32 has native implementation with Init()/CreateWindow() split. All platforms implement PlatformManager directly.
- **WindowID** — monotonic `uint32` identifier for each window (SDL3 pattern). Zero is invalid. Assigned by `PlatformManager.CreateWindow()`.
- **WindowManager** — tracks all open windows with `map[WindowID]*Window` + insertion order slice for deterministic render iteration. `App.PrimaryWindow()`, `App.WindowCount()`.
- **Renderer split** — shared GPU state (instance, adapter, device, pipelines, caches) separated from per-window state (`windowSurface`: surface, format, dimensions, frame/clear state). `activeSurface()` pattern for multi-window draw dispatch.
- **examples/multiwindow/** — two-window demo (primary blue, secondary red)

### Removed

- **`Platform` interface** — replaced by `PlatformManager` + `PlatformWindow`. No backward compatibility layers.
- **`platform.New()`** — replaced by `platform.NewManager()`.
- **`window/` package** — dead code (zero imports, all TODO stubs). Removed entirely.
- **Adapter layers** — `legacyPlatformAdapter`, `platformManagerAdapter`, `platformWindowAdapter`, `WrapAsLegacy()` all deleted.

### Changed

- **Dependencies:** wgpu v0.25.3 → v0.25.4, naga v0.17.4 → v0.17.5

## [0.27.3] - 2026-04-23

### Changed

- **Dependencies:** wgpu v0.25.2 → v0.25.3

## [0.27.2] - 2026-04-23

### Changed

- **Dependencies:** wgpu v0.25.1 → v0.25.2, gpucontext v0.12.0 → v0.14.0 (TextureRegionUpdater + TextureView type token), gputypes v0.4.0 → v0.5.0 (BREAKING: PrimitiveState zero value = WebGPU spec default, `*Undefined` constants removed)

## [0.27.1] - 2026-04-21

### Added

- **Adapter power preference** — `Config.PowerPreference` controls GPU selection on dual-GPU systems (laptops with integrated + discrete). `WithPowerPreference()` builder + `GOGPU_POWER_PREFERENCE` env var (`low`/`high`). Default `None` preserves existing behavior. (#176)
- **Wayland pointer lock** — `SetCursorMode()` now works on Wayland via `zwp_pointer_constraints_v1` + `zwp_relative_pointer_v1` protocols. Locked mode hides cursor and delivers relative deltas (`PointerEvent.DeltaX/DeltaY`). Confined mode restricts cursor to surface. Persistent lifetime (auto-relock on focus regain). Graceful degradation if compositor doesn't support the protocols. Completes v0.27.0 platform parity for pointer lock (Win32 + X11 + Wayland). (#175)

### Fixed

- **X11 event loop:** fixed dual-poller race that blocked resize/close events with `ContinuousRender(false)`. Root cause: `unix.Poll()` on dup'd fd competed with Go runtime netpoller on original `net.Conn` fd. Replaced with channel-based wait using `PollEventTimeout` through Go's single runtime poller. (#178)
- **macOS software backend:** added `setNeedsDisplay:` calls after `setContents:` in `BlitPixels` — per Apple docs, Core Animation requires explicit display trigger after setting layer contents. Without this, the window stayed blank until an external recomposite event. (#172)
- **Software backend:** removed redundant `blitSoftwareFramebuffer()` call in `EndFrame()` — was causing flicker and ~12% CPU overhead at fullscreen. `surface.Present()` already handles GDI blit via SurfaceTexture buffer.

### Changed

- **Dependencies:** wgpu v0.24.4 → v0.25.1, naga v0.17.4 (indirect), gpucontext v0.12.0 (CursorMode), golang.org/x/sys v0.43.0
- **Cleanup:** removed dead test files in `tmp/` that caused `go build ./...` failure (multiple `main` declarations) and incorrectly pulled `naga` as a direct dependency

## [0.27.0] - 2026-04-09

### Added

- **Mouse grab / pointer lock** — `App.SetCursorMode()` with three modes matching SDL:
  - `CursorModeLocked` — hidden cursor, confined to window, relative deltas in PointerEvent.DeltaX/DeltaY
    (SDL `SDL_SetRelativeMouseMode` parity). Uses warp-to-center pattern on Win32 and X11.
  - `CursorModeConfined` — visible cursor, confined to window bounds
    (SDL `SDL_SetWindowMouseGrab` parity). Win32 `ClipCursor`, X11 `XGrabPointer` confine_to.
  - `CursorModeNormal` — default behavior
  - Focus loss auto-releases grab, focus gain re-applies
  - Window resize updates clip rect
  - macOS and Wayland: stubs (implementation after deep research)
  (#173, requested by @darkliquid for ironwail-go Quake engine)

## [0.26.4] - 2026-04-08

### Changed

- **Particles example:** orbital dynamics (stable orbits, 1/r³ gravity), periodic FPS counter,
  soft boundary reflection. Tested 43800+ frames at 60 FPS.
- **Dependencies:** wgpu v0.24.2 → v0.24.4 (software backend enterprise Present via GDI,
  core routing for software surface, GLES X11 display use-after-close fix, adapter logging)

## [0.26.3] - 2026-04-07

### Changed

- **Dependencies:** wgpu v0.24.1 → v0.24.2 (Metal SetBindGroup cross-group slot fix — SDF shapes now render on Metal)

## [0.26.2] - 2026-04-07

### Changed

- **Dependencies:** wgpu v0.23.9 → v0.24.1 (Metal texture flicker fix, DX12 encoder pool,
  HEAP_TYPE_CUSTOM, unified encoder lifecycle), naga v0.16.6 → v0.17.0 (DXIL backend)

## [0.26.1] - 2026-04-05

### Fixed

- **Wayland CSD resize** — Complete CSD resize overhaul:
  - Interactive edge resize works correctly (no jump on first click)
  - Maximize: title bar at (0,0) inside window, borders destroyed to clear
    WSLg ghost pixels. Content fills screen below title bar.
  - Restore: borders recreated, title bar returns to normal position.
  - `set_window_geometry` = content area (0,0), no negative origin (WSLg compat).
  - (BUG-CSD-001, BUG-WAYLAND-001)
- **Wayland PollEvents queue pattern** — Replaced `closeEmitted`/`hasResize` flags
  with event queue (same architecture as X11 and Windows platforms).
  Events from Wayland callbacks are queued, PollEvents dequeues one at a time.

### Added

- **Programmatic DPI awareness** — Windows apps are now automatically DPI-aware
  without requiring a manifest file. Uses `SetProcessDpiAwarenessContext`
  (PerMonitorV2, Win10 1703+) with fallback to `SetProcessDPIAware` (Vista+).
  Fixes blurry text on high-DPI displays (200%+). Mouse coordinates converted
  from physical to logical pixels for correct hit-testing on high-DPI.
  (TASK-GOGPU-DPI-001, BUG-GOGPU-DPI-001)

### Changed (Dependencies)

- **wgpu** v0.23.0 → **v0.23.9** (GLES ADJUST_COORDINATE_SPACE, DX12 deferred resource destruction, DRED diagnostics, shader cache, Vulkan validation fixes)
- **naga** v0.15.0 → **v0.16.6** (HLSL 72/72, SPIR-V 164/164, ForceLoopBounding, zero-init loop 330× faster FXC, GLSL zero-init loop)
- **gputypes** v0.3.0 → **v0.4.0** (new vertex format constants, blend constant type)

## [0.26.0] - 2026-03-31

### Added

- **GPU particles example** — Compute + render in one window. 4096 particles
  simulated on GPU, rendered directly from compute buffer (zero CPU readback).
- **GOGPU_LOG env var** — Auto-initialize slog logger from environment variable.
  `GOGPU_LOG=debug|info|warn|error` — no code changes needed.
- **Wayland CSD (client-side decorations)** — Title bar with bitmap font title text,
  close/minimize/maximize/restore buttons with hover highlights. Activates
  automatically when `zxdg_decoration_manager_v1` unavailable. SHM subsurfaces
  via libwayland-client, resize on `xdg_toplevel.configure`. (FEAT-GOGPU-001)
- **Wayland single connection architecture** — Eliminated dual connection
  (Pure Go wire + C libwayland). All Wayland operations through single
  libwayland-client connection via goffi. Input (pointer, keyboard, touch)
  via goffi callbacks. Fixes zero-input on ALL Wayland compositors.
  (BUG-GOGPU-002, fixes gogpu#157, ui#64)
- **TextureCreator on ContextRenderTarget** — Enables universal rendering path
  (CPU pixmap → GPU texture → present) for CPU-only adapters.
- **`Config.WithVSync()`** — Builder method for VSync configuration.
- **Present mode fallback chain** — Capability-based selection:
  VSync on → FifoRelaxed → Fifo, VSync off → Immediate → Mailbox → Fifo.
  Queries driver via `Adapter.GetSurfaceCapabilities()`.
  Follows Rust wgpu `AutoVsync`/`AutoNoVsync` pattern.
- **BlitPixels on macOS and Linux X11** — Software backend pixel blitting.
- **Platform stubs** — macOS + Linux: ScaleFactor, PrepareFrame, PhysicalSize.
- **X11 DPI + Wayland scale factor** — Xft.dpi, wl_output.scale, env fallback.

### Fixed

- **PresentTexture error handling** — Returns error on nil/wrong type instead of
  silent nil. Fixes black screen on CPU adapters (llvmpipe, SwiftShader).
- **X11/macOS window close** — Rewritten PollEvents to queue pattern, fixes
  infinite EventClose loop (#129).
- **X11 poll timing** — Increased deadline from 1μs to 1ms to prevent missed events.
- **Wayland close** — PollEvents no longer returns EventClose infinitely
  (closeEmitted flag). Flush + roundtrip on disconnect for clean compositor cleanup.

### Changed

- **Enterprise fence architecture** — Replaced `FencePool` (per-submission VkFence)
  with `SubmissionTracker` (tracks by submission index from wgpu HAL). No more
  application-level fence management — HAL owns all fences internally. Eliminates
  double `vkQueueSubmit` on binary fence path. (BUG-GOGPU-004, fixes ui#66)
- **deps: wgpu v0.22.2 → v0.23.0** — Enterprise fence architecture, naga v0.15.0,
  GetSurfaceCapabilities
- **deps: naga v0.14.8 → v0.15.0** — Full Rust parity: IR 144/144, SPIR-V 87/87,
  MSL 91/91, HLSL 58/58, GLSL 68/68
- **deps: goffi v0.5.0** — Windows ARM64 / Snapdragon X support
- **deps: webgpu v0.4.3** — Rust FFI backend update
- **deps: gpucontext v0.11.0** — WindowChrome interface

### Known Issues

- Wayland CSD: border subsurface artifacts visible after maximize on WSLg
  (compositor doesn't remove old pixels on subsurface destroy)

## [0.25.1] - 2026-03-25

### Added

- **X11 DPI reading** — `ScaleFactor()` reads `Xft.dpi` from RESOURCE_MANAGER,
  fallback to screen physical dimensions. No longer hardcoded 1.0.
- **Wayland scale factor** — binds `wl_output`, handles scale event, tracks
  per-output scale via surface enter/leave. Env var fallback (GDK_SCALE, QT_SCALE_FACTOR).
- **macOS platform stubs implemented** — ClipboardRead/Write (NSPasteboard),
  SetCursor (12 shapes via NSCursor), DarkMode (NSAppearance),
  ReduceMotion/HighContrast (NSWorkspace accessibility).
- **Linux platform stubs implemented** — DarkMode/HighContrast (GTK_THEME),
  FontScale (GDK_DPI_SCALE), ReduceMotion (GTK_ENABLE_ANIMATIONS),
  X11 SetCursor (CreateGlyphCursor with cursor caching).
- **BlitPixels on macOS** — CoreGraphics CGBitmapContext → CALayer.setContents.
- **BlitPixels on Linux X11** — PutImage with RGBA→BGRA conversion and
  automatic row-strip chunking for >64KB images.

## [0.25.0] - 2026-03-21

### Added

- **Frameless window support** — `Config.Frameless` + `WithFrameless()` builder.
  `App` implements `gpucontext.WindowChrome` (SetFrameless, SetHitTestCallback,
  Minimize, Maximize, IsMaximized, Close). Platform support:
  - **Windows**: JBR approach — WS_OVERLAPPEDWINDOW + WM_NCCALCSIZE (top only) +
    WM_NCACTIVATE + DwmExtendFrameIntoClientArea + DwmFlush sync + BLACK_BRUSH
  - **macOS**: NSWindowStyleMaskBorderless + SetStyleMask
  - **X11**: _MOTIF_WM_HINTS borderless
  - **Wayland**: zxdg_decoration_manager client-side mode

- **`SyncFrame()` platform method** — DwmFlush during modal resize for smoother content

- **`WM_DPICHANGED` handler** — Multi-monitor DPI transitions with SetWindowPos
  to suggested rect

- **`Config.VSync` honored** — PresentModeFifo (VSync on) or PresentModeImmediate
  (VSync off). Previously hardcoded to Fifo.

### Fixed

- **WM_MOUSEWHEEL screen→client coords** — ScreenToClient conversion for scroll events
- **Mouse capture for drag tracking** — SetCapture/ReleaseCapture on button press/release
- **System caption flash on focus loss** — WM_NCACTIVATE + InvalidateRect
- **Delta time clamped after idle** — Max 66ms to prevent animation jumps

### Testing

- **config.go**: 19 subtests (DefaultConfig, builders, env vars, immutability)
- **fence_pool.go**: 22 subtests (acquire/release/reuse, error propagation, concurrency)
- **context.go**: 14 new tests (DPI scales, aspect ratio, surface size, typed-nil safety)
- **window_chrome**: 37 tests (interface, nil-safety, delegation, hit test regions)

## [0.24.5] - 2026-03-18

### Fixed

- **`SetLogger` now propagates to all subsystems** — a single `gogpu.SetLogger()` call
  enables logging across the entire stack (platform + wgpu). Previously each subsystem
  had an isolated logger that was only set during `NewApp()`, so calling `SetLogger`
  before or after `NewApp` had no effect on platform/GPU logging. (#149)

### Testing

- **Test coverage for awesome-go submission** — enterprise-grade tests for app lifecycle,
  texture management, event dispatch, context adapters, config builder, shader helpers.
  Codecov ignore updated to exclude untestable GPU/platform code (renderer.go,
  gpu/backend/native, internal/thread). Effective coverage 80%+ on testable code.

## [0.24.4] - 2026-03-16

### Added

- **`GOGPU_GRAPHICS_API` environment variable** — Select backend without code changes:
  `vulkan`, `dx12`, `metal`, `gles`, `software` (+ short aliases `vk`, `d3d12`, `gl`, `sw`, `cpu`).
  `WithGraphicsAPI()` in code takes precedence.

- **`Context.PresentTexture(tex)`** — Draws a texture filling the entire surface.
  Universal path for presenting pre-rendered content on any backend including software.

- **`Context.RenderTarget()`** — Adapter satisfying `ggcanvas.RenderTarget` interface
  for universal backend-agnostic canvas rendering.

- **Renderer passes `CompatibleSurface` in `RequestAdapter`** — GLES backends
  enumerate adapters using the surface's GL context (WebGPU spec pattern).

### Dependencies

- wgpu v0.21.2 → v0.21.3 (GLES/DX12/software fixes, naga v0.14.8)

## [0.24.3] - 2026-03-16

### Dependencies

- wgpu v0.21.1 → v0.21.2 (core validation layer: Binder, SetBindGroup bounds, draw-time
  compatibility, dynamic offsets, vertex/index buffer checks — fixes AMD/NVIDIA crash)

## [0.24.2] - 2026-03-15

### Fixed

- **Rust backend: adapter limits propagation** — `EnumerateAdapters` now fills
  `Capabilities.Limits` from `adapter.GetLimits()`. Previously zero limits caused
  core validation to reject all textures.

### Dependencies

- wgpu v0.21.0 → v0.21.1 (per-stage resource limit validation)

## [0.24.1] - 2026-03-15

### Fixed

- **X11/Wayland: character dispatch for Unicode text input** — Completes #138
  across all platforms. Previously only Windows and macOS dispatched characters.
  - **X11:** Character dispatch via `KeysymToString` (respects server keyboard mapping)
  - **Wayland:** `evdevKeycodeToRune` US QWERTY fallback table
  - Both skip char dispatch when Ctrl/Alt/Super held (shortcuts, not text input)

## [0.24.0] - 2026-03-15

### Added

- **Platform: SetCharCallback for Unicode text input** — New `Platform.SetCharCallback(func(rune))`
  interface method, wired to `OnTextInput()` callback in app.go. Enables non-Latin text input
  (Cyrillic, CJK, Arabic, etc.) in UI widgets.
  - **Windows:** Full implementation with WM_CHAR + WM_SYSCHAR (AltGr) + WM_UNICHAR (IME),
    UTF-16 surrogate pair decoding for emoji/supplementary characters (GLFW/Ebiten pattern)
  - **macOS:** Basic implementation via `[NSEvent characters]` UTF-8 extraction
  - Fixes [#138](https://github.com/gogpu/gogpu/issues/138)

### Changed

- **Renderer migrated from HAL direct to wgpu public API** — The renderer now uses
  `*wgpu.Device`, `*wgpu.Surface`, `*wgpu.Queue` instead of `hal.Device`, `hal.Surface`,
  `hal.Queue` directly. All GPU operations go through the wgpu public API → wgpu/core →
  wgpu/hal chain. This enables proper surface lifecycle management, PrepareFrame hooks
  for HiDPI/DPI handling, and validation at the core layer. Both native (Pure Go) and
  Rust (FFI) backends continue to work through the unified `gpu.Backend` interface.

- **FencePool migrated to wgpu types** — Uses `*wgpu.Device`, `*wgpu.Fence`,
  `*wgpu.CommandBuffer` instead of hal types. Non-blocking async submission through
  `Queue.SubmitWithFence()`.

- **Texture migrated to wgpu types** — Uses `*wgpu.Texture`, `*wgpu.TextureView`,
  `*wgpu.Sampler`. Resource cleanup through `Release()` pattern instead of
  `device.Destroy*()`.

- **Native backend creates wgpu objects** — `gpu/backend/native/` now creates
  `*wgpu.Instance` → `*wgpu.Adapter` → `*wgpu.Device` instead of hal objects directly.

- **Context.SurfaceView() returns `*wgpu.TextureView`** — Was `any`, now typed.

### Fixed

- **Rust backend: WriteBuffer/WriteTexture return type** — go-webgpu changed
  these methods to void return. Adapted wrapper to match.

### Dependencies

- wgpu v0.20.2 → v0.21.0 (three-layer public API, proper type definitions)
- gpucontext v0.9.0 → v0.10.0 (typed interfaces, HalProvider removed)
- naga v0.14.6 → v0.14.7 (MSL binding index fix)

## [0.23.3] - 2026-03-12

### Fixed

- **Wayland/Sway: SIGSEGV during Vulkan surface initialization** — the xdg_toplevel
  C interface descriptor declared `Version=6` but `EventCount=2`, missing the
  `configure_bounds` (v4) and `wm_capabilities` (v5) events. When Sway sent
  `wm_capabilities` (event opcode 3), libwayland-client could not find the event
  signature in the descriptor array, failed to deserialize the wire message, and
  broke the roundtrip. This left the `wl_surface` in an unconfigured state, causing
  `vkGetPhysicalDeviceSurfaceCapabilitiesKHR` to crash with SIGSEGV (addr=0x18).
  ([ui#45](https://github.com/gogpu/ui/issues/45))

  Fix (following GLFW v3.3 pattern — declare all v6 events, handle selectively):
  - Expanded `toplevelEvents` from 2 to 4 entries with correct wire signatures
  - Added `configure_bounds` and `wm_capabilities` event handling in Pure Go dispatch
  - Added `XdgToplevelBounds`, `XdgToplevelWmCapability*` types and public API
  - Added `HasWmCapability()` for compositor feature detection

### Added

- **xdg-shell v4/v5 event support** — `configure_bounds` (recommended window bounds
  from compositor) and `wm_capabilities` (compositor feature advertisement) events
  are now properly declared in the C interface descriptor and handled in Pure Go
  dispatch. Handlers: `SetConfigureBoundsHandler()`, `SetWmCapabilitiesHandler()`.
  Accessors: `Bounds()`, `WmCapabilities()`, `HasWmCapability()`.

### Changed

- Updated `github.com/gogpu/wgpu` v0.20.0 → v0.20.2 (WSI validation)
- Updated `golang.org/x/sys` v0.41.0 → v0.42.0

## [0.23.2] - 2026-03-11

### Fixed

- **macOS Retina: CAMetalLayer contentsScale not maintained across frames** —
  in layer-hosting mode (`setWantsLayer` + `setLayer`), macOS does not manage
  the layer and may reset `contentsScale` to 1.0 during layout passes. The
  drawable-to-screen mapping then drifts, causing progressive stretching and
  offset of rendered content on Retina displays. Now `contentsScale` is re-set
  from `BackingScaleFactor()` before every `nextDrawable` call, matching Gio's
  `displayLayer:` pattern and Skia's `resize()` approach.
  ([gg#171](https://github.com/gogpu/gg/issues/171))

## [0.23.1] - 2026-03-11

### Fixed

- **macOS Retina: CAMetalLayer frame/bounds not set** — the Metal layer was
  created with zero-sized bounds (`CGRectZero`) and never given spatial dimensions.
  In layer-hosting mode (`setWantsLayer` + `setLayer`), macOS does not automatically
  manage the layer's geometry, so the drawable-to-screen mapping was undefined.
  This caused circles to render as horizontally-stretched ellipses and content
  to appear offset on Retina displays.
  ([gg#171](https://github.com/gogpu/gg/issues/171))

  Fix (following Skia and Gio patterns):
  - Set `autoresizingMask = kCALayerWidthSizable | kCALayerHeightSizable`
    so the layer auto-resizes with the view
  - Set `contentsGravity = kCAGravityTopLeft` to prevent non-uniform stretching
  - Explicitly set layer `frame` to match content view bounds before attaching
  - Update layer frame in `Resize()` and `UpdateSize()` on window resize

## [0.23.0] - 2026-03-11

### Added

- **`App.PhysicalSize()`** — returns GPU framebuffer size in device pixels
- **`Context.FramebufferWidth()`/`FramebufferHeight()`/`FramebufferSize()`** — physical pixel dimensions for GPU rendering
- **`gpuContextAdapter` implements `gpucontext.WindowProvider`** — enables ggcanvas
  and other libraries to auto-detect HiDPI scale via standard ecosystem interface

### Changed

- **Platform API: logical/physical pixel split** — redesigned Platform interface
  to properly distinguish logical points (DIP) from physical pixels (framebuffer).
  New internal methods: `LogicalSize()`, `PhysicalSize()`, `ScaleFactor()`.
  `App.Size()` / `Context.Width()`/`Height()` now return logical dimensions.
  ([gg#171](https://github.com/gogpu/gg/issues/171),
  [gg#175](https://github.com/gogpu/gg/issues/175))
  - macOS: `Window.UpdateSize()` stores logical points (not physical pixels)
  - macOS: new `Window.FramebufferSize()` for physical pixel dimensions
  - Windows: real DPI awareness via `GetDpiForWindow()`
  - Event coordinates are now consistently in logical points on all platforms

### Fixed

- **macOS Retina race condition** — removed `surface.Resize()` from `PollEvents()`
  which ran on the main thread while the render thread operated on the wgpu surface.
  Surface reconfiguration now happens exclusively on the render thread via
  `RequestResize()`. Fixes content disappearing periodically on macOS.
  ([gg#171](https://github.com/gogpu/gg/issues/171))

## [0.22.11] - 2026-03-10

### Changed

- **Update wgpu v0.19.8 → v0.20.0** — enterprise-grade validation layer:
  core validation (30+ WebGPU spec rules), 7 typed error types with `errors.As()`,
  WebGPU deferred error pattern, HAL defense-in-depth.
- **Update gputypes v0.2.0 → v0.3.0** — `TextureUsage.ContainsUnknownBits()`.

## [0.22.10] - 2026-03-10

### Fixed

- **macOS Retina scaling** — window size, pointer coordinates, scroll deltas,
  and Metal surface drawable dimensions now correctly account for
  `backingScaleFactor` on Retina displays (2x). Previously all coordinates
  and sizes used logical points, causing half-resolution rendering and
  misaligned input on HiDPI screens.
  ([gg#171](https://github.com/gogpu/gg/issues/171))
  - `GetSize()` returns physical pixels (bounds × scaleFactor)
  - `CAMetalLayer.contentsScale` set to match backing scale factor
  - Pointer events scaled from logical points to physical pixels
  - Y-coordinate flip computed in points before scaling (correct order)
  - In-window bounds check uses physical pixel dimensions
  - Trackpad scroll deltas scaled for pixel-mode precision

### Changed

- **Update wgpu v0.19.7 → v0.19.8** — Metal buffer storage mode fix for CopyDst
  buffers on Apple Silicon + staging buffer fallback
  ([wgpu#99](https://github.com/gogpu/wgpu/pull/99),
  [gg#170](https://github.com/gogpu/gg/issues/170))

## [0.22.9] - 2026-03-07

### Changed

- **Update wgpu v0.19.6 → v0.19.7** — Queue.WriteTexture public API
  ([wgpu#95](https://github.com/gogpu/wgpu/pull/95) by [@Carmen-Shannon](https://github.com/Carmen-Shannon)),
  naga v0.14.6 MSL pass-through globals fix
  ([naga#40](https://github.com/gogpu/naga/pull/40))

## [0.22.8] - 2026-03-06

### Fixed

- **X11 input handling** — keyboard and mouse events now work under Linux X11
  ([#129](https://github.com/gogpu/gogpu/issues/129))
  - Implement non-blocking `PollEvent()` via `SetReadDeadline` (was a stub returning nil)
  - Add keyboard event handling (`KeyPress`/`KeyRelease`) with X11 keycode → evdev → `gpucontext.Key` mapping
  - Wire `SetKeyCallback` through x11Platform to x11.Platform

## [0.22.7] - 2026-03-05

### Changed

- **Update wgpu v0.19.5 → v0.19.6** — Metal MSAA resolve store action fix
  ([wgpu#94](https://github.com/gogpu/wgpu/pull/94),
  [ui#23](https://github.com/gogpu/ui/issues/23))

## [0.22.6] - 2026-03-05

### Changed

- **Update wgpu v0.19.4 → v0.19.5** — Metal vertex descriptor fix: add
  `MTLVertexDescriptor` to render pipeline creation, complete vertex format
  mapping ([wgpu#93](https://github.com/gogpu/wgpu/pull/93),
  [ui#23](https://github.com/gogpu/ui/issues/23))
- **Update naga v0.14.4 → v0.14.5**

## [0.22.5] - 2026-03-04

### Fixed

- **x86_64 macOS: SIGSEGV in GetRect** — use `objc_msgSend_stret` for NSRect (32-byte)
  struct returns on Intel Macs. The x86_64 SysV ABI requires `_stret` for struct returns
  exceeding 16 bytes; ARM64 is unaffected ([#125](https://github.com/gogpu/gogpu/issues/125))

### Changed

- **Update webgpu v0.4.1 → v0.4.2** — goffi purego compatibility fix (nofakecgo build tag),
  x/sys v0.41.0
- **Update goffi v0.4.1 → v0.4.2**

## [0.22.4] - 2026-03-02

### Changed

- **Update wgpu v0.19.3 → v0.19.4** — fix SIGSEGV on Linux/macOS for Vulkan
  functions with >6 arguments ([goffi#19](https://github.com/go-webgpu/goffi/issues/19),
  [gogpu#119](https://github.com/gogpu/gogpu/issues/119))
- **Update goffi v0.4.0 → v0.4.1** — Unix amd64 stack spill for args 7+
- **Update webgpu v0.4.0 → v0.4.1**

## [0.22.3] - 2026-03-01

### Changed

- **Update wgpu v0.19.0 → v0.19.3** — includes MSL backend fixes for Apple Silicon:
  vertex `[[stage_in]]` for struct-typed arguments, `metal::discard_fragment()` namespace
  ([naga#38](https://github.com/gogpu/naga/pull/38),
  [ui#23](https://github.com/gogpu/ui/issues/23))

### Tests

- Add coverage tests for config, context, fence pool (coverage 25% → 38%+)

## [0.22.2] - 2026-03-01

### Fixed

- **Error propagation for `WriteTexture`** — all 3 call sites in `texture.go` now check
  errors and destroy the texture on upload failure instead of silently continuing with
  uninitialized GPU textures.
- **Error propagation for `WriteBuffer`** — `renderer.go` uniform buffer upload now propagates
  errors instead of silently swallowing them.
- **Rust backend error propagation** — `rustQueue.WriteBuffer` and `rustQueue.WriteTexture`
  now return errors from the underlying wgpu calls instead of discarding them.

### Changed

- Update wgpu v0.18.1 → v0.19.0 — `WriteBuffer` and `WriteTexture` breaking interface changes

## [0.22.1] - 2026-02-27

### Fixed

- **Vulkan: rounded rectangle pixel corruption** — update wgpu v0.18.0 → v0.18.1 which fixes
  buffer-to-image copy row stride corruption on non-power-of-2 width textures. Previously,
  `BytesPerRow / Width` integer division inferred wrong bytes-per-texel when BytesPerRow was
  padded to 256-byte alignment. Affected 126 out of 204 possible widths for RGBA8 textures.
  ([#96](https://github.com/gogpu/gogpu/discussions/96))

## [0.22.0] - 2026-02-27

### Added

- **X11 multi-touch input via XInput2** — pure Go wire protocol implementation of XInput2
  extension for multi-touch support on X11/Linux. Includes QueryExtension infrastructure
  (reusable for any X11 extension), GenericEvent (type 35) variable-length event parsing,
  XIQueryVersion 2.2 negotiation, XISelectEvents for touch subscription, and XIDeviceEvent
  decoding with FP16.16/FP32.32 sub-pixel coordinates. Touch events map to
  `gpucontext.PointerEvent` with `PointerTypeTouch` and multi-touch tracking (primary touch,
  per-touch IDs). Graceful fallback when XInput2 is unavailable.
- **X11 extension query infrastructure** — `Connection.QueryExtension(name)` with result caching,
  enabling future extensions (XRandR, DPMS, etc.) without additional work.
- **GenericEvent support** — fixed `readResponse()` and `WaitForEvent()` to handle variable-length
  X11 GenericEvents (type 35), preventing wire protocol desync when extensions send events.

### Changed

- Update wgpu v0.17.0 → v0.18.0 (public API root package)

## [0.21.0] - 2026-02-27

### Added

- **Wayland Vulkan surface via libwayland-client** — loads `libwayland-client.so.0` dynamically
  via goffi to create real C pointers (`wl_display*`, `wl_surface*`) required by
  `VK_KHR_wayland_surface`. Sets up xdg-shell role (`xdg_surface` + `xdg_toplevel`) with goffi
  callbacks for configure/ping events. Without a role, the compositor won't composite the surface
  and `vkQueuePresentKHR` blocks forever. Falls back to software backend if libwayland-client
  is unavailable.
- **Wayland server-side decorations** — requests window decorations (title bar, close/minimize/maximize
  buttons) from the compositor via `zxdg_decoration_manager_v1` protocol on both pure Go and
  libwayland-client connections. Sets window title and app_id on the C `xdg_toplevel` for
  decoration bar display. Falls back gracefully if the compositor does not support this extension.

### Fixed

- **Wayland non-blocking socket** — fixed fd propagation for multi-message batches. The Wayland
  socket now uses non-blocking I/O to prevent blocking when the compositor sends multiple events
  in a single batch.

### Changed

- **Pure Go Wayland protocol refactored** — object dispatch architecture replaces monolithic
  message handling. Each Wayland object type (compositor, surface, seat, keyboard, pointer, touch,
  xdg_wm_base, xdg_surface, xdg_toplevel) has its own dispatch method.

### Dependencies

- wgpu v0.16.17 → v0.17.0 (Wayland Vulkan surface creation)
- goffi v0.3.9 → v0.4.0 (crosscall2 callback support from any thread)
- webgpu v0.3.2 → v0.4.0 (FFI null handle guards, go vet cleanup, WGPU_NATIVE_PATH)

## [0.20.9] - 2026-02-26

### Dependencies

- wgpu v0.16.16 → v0.16.17 (load platform Vulkan surface creation functions — the real fix for #106)

## [0.20.8] - 2026-02-25

### Fixed

- **X11 Vulkan surface creation (root cause)** — the actual bug was in wgpu's `CreateSurface`:
  `unsafe.Pointer(&display)` passed the Go stack address of the local variable instead of the
  `Display*` value. Vulkan received a Go stack pointer instead of the real Xlib Display, causing
  null surfaces (native) and SIGSEGV (Rust). The v0.20.7 `GetHandle()` fix was necessary but
  insufficient — the Display* value never reached Vulkan due to this pointer indirection bug.
  ([#106](https://github.com/gogpu/gogpu/issues/106))

### Added

- **Platform diagnostic logging** — X11 platform now logs initialization details (platform
  selection, Display* pointer, window ID, GetHandle values) via slog. Silent by default;
  enable with `gogpu.SetLogger(slog.Default())`.

### Dependencies

- wgpu v0.16.15 → v0.16.16 (Vulkan X11/macOS surface pointer fix)

## [0.20.7] - 2026-02-25

### Fixed

- **X11 Vulkan surface creation** — `GetHandle()` now returns a real Xlib `Display*` pointer instead
  of a raw socket file descriptor. `VK_KHR_xlib_surface` requires `Display*` from `XOpenDisplay()`,
  but our pure Go X11 implementation was passing the socket FD, causing null surfaces on the native
  backend and SIGSEGV on the Rust backend. libX11 is loaded dynamically via goffi (no CGO).
  ([#106](https://github.com/gogpu/gogpu/issues/106))

## [0.20.6] - 2026-02-25

### Fixed

- **Software backend selection** — `WithGraphicsAPI(GraphicsAPISoftware)` now correctly selects the
  software backend on all platforms (Windows, Linux, macOS). Previously silently fell back to Vulkan
  due to missing switch case. ([#106](https://github.com/gogpu/gogpu/issues/106))

- **Software backend screen presentation (Windows)** — software-rendered pixels are now displayed
  on screen via GDI `SetDIBitsToDevice`. The renderer detects software surfaces and blits the
  framebuffer to the window after each `Present()`. RGBA→BGRA conversion handled automatically.

### Added

- **PixelBlitter interface** — optional platform interface for direct pixel blitting to a window.
  Implemented on Windows; Linux and macOS platforms gracefully skip blitting (headless mode).

### Dependencies

- wgpu v0.16.14 → v0.16.15 (software backend always compiled, no build tags)

## [0.20.5] - 2026-02-25

### Dependencies

- wgpu v0.16.13 → v0.16.14 (Vulkan null surface handle guard, naga v0.14.3)

## [0.20.4] - 2026-02-24

### Dependencies

- wgpu v0.16.12 → v0.16.13 (fix: load VK_EXT_debug_utils via GetInstanceProcAddr)

## [0.20.3] - 2026-02-23

### Dependencies

- wgpu v0.16.11 → v0.16.12 (Vulkan debug object naming, eliminates false-positive validation errors)

## [0.20.2] - 2026-02-23

### Fixed

- **Renderer: unconfigure surface on window minimize** (VK-VAL-001) — `Resize(0, 0)` previously
  returned early without cleaning up the surface, leaving a stale swapchain that could trigger
  validation errors on restore. Now calls `surface.Unconfigure()` and sets `surfaceConfigured = false`,
  ensuring no rendering is attempted while the window is minimized. `BeginFrame()` already checks
  `surfaceConfigured` and returns false.
  ([#98](https://github.com/gogpu/gogpu/issues/98))

### Dependencies

- wgpu v0.16.10 → v0.16.11 (Vulkan zero-extent swapchain fix, unconditional viewport/scissor)

## [0.20.1] - 2026-02-22

### Added

- **Win32 touch/pen input via WM_POINTER*** — touch events with `PointerTypeTouch`, pen events
  with pressure (0-1024 normalized to 0.0-1.0), tiltX/Y via `GetPointerPenInfo`. Existing
  WM_MOUSE* handlers preserved for mouse input.
- **macOS pen/tablet detection** — detects pen input via NSEvent `subtype == NSEventSubtypeTabletPoint`
  on mouse events. Reads pressure (0.0-1.0), tilt [-1,1] converted to degrees [-90,90],
  rotation as twist, pointing device type (pen/eraser/cursor).
- **Wayland wl_touch support** — full `WlTouch` implementation with down/up/motion/frame/cancel
  handlers. Touch IDs offset +2 from mouse, first touch marked primary. Integrated with
  `WlSeat.GetTouch()`.

### Dependencies

- wgpu v0.16.9 → v0.16.10
- naga v0.14.1 → v0.14.2

## [0.20.0] - 2026-02-20

### Added

- **Automatic GPU resource lifecycle management** — `App.TrackResource(io.Closer)` registers
  resources for automatic cleanup during shutdown. Resources are closed in LIFO (reverse)
  order after GPU idle, before renderer destruction. Eliminates manual `OnClose` callbacks.
- **ResourceTracker interface** — optional interface for auto-registration. ggcanvas.Canvas
  auto-registers when created via a provider that implements ResourceTracker.
- **runtime.AddCleanup safety net on Texture** — GC-triggered cleanup enqueues deferred
  GPU resource destruction, drained at frame boundaries. Catches forgotten `Destroy()` calls.
- **Deferred destruction queue on Renderer** — `EnqueueDeferredDestroy()`/`DrainDeferredDestroys()`
  for thread-safe GPU resource cleanup from arbitrary goroutines.

### Fixed

- **Wayland: missing globals on SOCK_STREAM sockets** ([#74](https://github.com/gogpu/gogpu/issues/74)) —
  `Display.RecvMessage()` only decoded the first message from each `recvmsg()` call. Wayland uses
  `SOCK_STREAM` sockets which don't preserve message boundaries — a single read can contain
  multiple protocol messages. Now decodes all messages and queues extras, preventing loss of
  critical globals like `xdg_wm_base`.

### Dependencies

- wgpu v0.16.6 → v0.16.9 (Metal presentDrawable fix [#89](https://github.com/gogpu/gogpu/issues/89), naga v0.14.1)
- naga v0.13.1 → v0.14.1 (Essential 15/15 reference shaders, HLSL row_major matrices, GLSL namedExpressions fix)

## [0.19.6] - 2026-02-20

### Fixed

- **Rust backend HAL compliance** — implement `CreateQuerySet`, `DestroyQuerySet`,
  `ResolveQuerySet`, and `WaitIdle` in the Rust backend. The HAL interface was extended
  but the Rust backend wasn't updated, causing `-tags rust` compilation failure.
  Reported by @amortaza in [Discussion #47](https://github.com/gogpu/gogpu/discussions/47).

## [0.19.5] - 2026-02-18

### Dependencies

- go-webgpu/webgpu v0.3.0 → v0.3.1 (goffi v0.3.9 — ARM64 callback trampoline fix)

## [0.19.4] - 2026-02-18

### Dependencies

- wgpu v0.16.5 → v0.16.6 (Metal debug logging, goffi v0.3.9)

## [0.19.3] - 2026-02-18

### Dependencies

- wgpu v0.16.4 → v0.16.5 (per-encoder command pools, fixes VkCommandBuffer crash)

## [0.19.2] - 2026-02-18

### Added

- **Enterprise hot-path benchmarks** — 52 benchmarks with `ReportAllocs()` across gmath (31 — Vec2/3/4,
  Mat4, Color operations, batch transforms), input (17 — keyboard/mouse polling, frame update),
  gpu/types (4 — backend enum operations), window (6 — config, events), root (11 — Config, Texture,
  AnimationController). All math operations confirmed **zero-allocation**. Mat4×Vec4 vertex
  transform: 5ns/op, 0 allocs.

### Dependencies

- wgpu v0.16.3 → v0.16.4 (timeline semaphore, FencePool, hot-path allocation optimization)
- naga v0.13.0 → v0.13.1 (OpArrayLength fix, −32% compiler allocations)

## [0.19.1] - 2026-02-16

### Fixed

- **GPU resource cleanup on exit** — `Renderer.Destroy()` now calls `device.WaitIdle()` before
  releasing pipelines, textures, and other GPU resources. Ensures the last frame completes on
  the GPU before destruction. Fixes DX12 crash (Exception 0x87d in `ID3D12PipelineState.Release`)
  on window close when using per-frame fence tracking.

### Dependencies

- wgpu v0.16.2 → v0.16.3 (per-frame fence tracking, GLES VSync fix)

## [0.19.0] - 2026-02-16

### Added
- **Cross-platform Rust backend** — Rust (wgpu-native) backend now supports macOS (Metal)
  and Linux (Vulkan, X11/Wayland) in addition to Windows. Build with `-tags rust`
  on any platform. Platform surface creation delegated to `rust_{windows,darwin,linux}.go`.
  Linux auto-detects Wayland vs X11 via `WAYLAND_DISPLAY` environment variable.

### Dependencies
- wgpu v0.16.1 → v0.16.2 (Metal autorelease pool LIFO fix for macOS Tahoe)

## [0.18.2] - 2026-02-15

### Dependencies
- wgpu v0.16.0 → v0.16.1 (Vulkan framebuffer cache invalidation fix)

## [0.18.1] - 2026-02-15

### Added

- **Event-driven rendering with three-state model** — Main loop now operates in three states:
  - **IDLE**: No activity — blocks on OS events via `WaitEvents` (0% CPU, <1ms response)
  - **ANIMATING**: Active animations — renders at VSync (smooth 60fps)
  - **CONTINUOUS**: `ContinuousRender=true` — renders every frame (game loop)
  - Previous behavior was a 100ms `time.Sleep` poll loop when idle

- **`App.StartAnimation()` / `AnimationToken`** — Token-based animation lifecycle.
  Call `StartAnimation()` to begin VSync rendering, `token.Stop()` when done.
  Thread-safe via `atomic.Int32`. Multiple concurrent animations supported.

- **`Invalidator`** — Goroutine-safe redraw request coalescing (Gio pattern).
  `App.RequestRedraw()` now uses lock-free buffered channel with platform wakeup.
  Multiple concurrent invalidations coalesce into a single redraw.

- **Native `WaitEvents` / `WakeUp`** for all platforms:
  - **Windows**: `MsgWaitForMultipleObjectsEx` + `PostMessageW(WM_NULL)` (already existed)
  - **macOS**: `[NSApp nextEventMatchingMask:]` blocking + `[NSApp postEvent:atStart:]`
  - **Linux X11**: `poll()` on X11 connection fd + `XSendEvent` (ClientMessage)

## [0.18.0] - 2026-02-15

### Added

- **GraphicsAPI selection** — Runtime selection of graphics API, orthogonal to backend choice.
  `Config.WithGraphicsAPI(api)` accepts `GraphicsAPIVulkan`, `GraphicsAPIDX12`, `GraphicsAPIMetal`,
  `GraphicsAPIGLES`, `GraphicsAPISoftware`, or `GraphicsAPIAuto` (default).
  Windows supports Vulkan/DX12/GLES, Linux supports Vulkan/GLES, macOS uses Metal.
  - Re-exported constants: `gogpu.GraphicsAPIVulkan`, `gogpu.GraphicsAPIDX12`, etc.
  - `types.GraphicsAPI` enum type with `String()` method

- **SurfaceView for zero-copy rendering** — `Context.SurfaceView()` exposes the current frame's
  surface texture view for direct GPU rendering. Enables zero-copy integration with gg/ggcanvas
  `RenderDirect`, bypassing the GPU→CPU→GPU readback path.

- **DX12 device health diagnostics** — `Context.CheckDeviceHealth()` returns detailed error
  information when the DX12 device is removed. Uses `DXGI_ERROR_DEVICE_REMOVED` reason codes
  for debugging GPU crashes.

- **Structured logging via log/slog** — `SetLogger(*slog.Logger)` and `Logger()` for
  configurable structured logging. Silent by default (nop handler). Thread-safe via
  `atomic.Pointer`. Log levels: Debug (diagnostics), Info (lifecycle), Warn (non-fatal issues).

- **`App.OnClose()` callback** — registers a cleanup function that runs on the render thread
  before `Renderer.Destroy()`. Ensures GPU resources (textures, bind groups, pipelines) are
  released while the device is still alive, preventing Vulkan validation errors on exit.

- **GLES triangle rendering test example** — `examples/gles_test/` demonstrates GLES backend
  selection via `WithGraphicsAPI(gogpu.GraphicsAPIGLES)`.

### Fixed

- **Rust backend: StencilOperation off-by-one** — HAL `StencilOperation` uses iota (Keep=0),
  gputypes uses WebGPU spec values (Keep=1). Direct cast was off by one, causing incorrect
  stencil operations in the stencil-then-cover pipeline (visible as star rendering artifact).
- **Rust backend: MipLevelCount panic** — HAL uses 0 for "all remaining mip levels",
  wgpu-native expects `math.MaxUint32` (WGPU_MIP_LEVEL_COUNT_UNDEFINED). Was crashing
  on `CreateTextureView`.
- **Rust backend: SetVertexBuffer/SetIndexBuffer panic** — HAL uses size 0 for "whole buffer",
  wgpu-native expects `math.MaxUint64` (WGPU_WHOLE_SIZE). Was crashing during render pass.

- **DX12 deferred clear** — `ClearColor` + `DrawTexture` merged into a single render
  pass via deferred clear pattern. Eliminates the intermediate RT→PRESENT→RT state
  transition that caused content loss on DX12 FLIP_DISCARD swapchains during resize.

### Refactored

- **Complete HAL migration** — Renderer now uses `hal.Device`/`hal.Queue` directly instead
  of going through `gpu.Backend` + `ResourceRegistry` handle maps. This removes ~2700 net
  lines of indirection code and enables proper error propagation.
  - `Renderer` fields changed from `types.*` (uintptr handles) to `hal.*` (Go interfaces)
  - `Texture` uses `hal.Texture`/`hal.TextureView`/`hal.Sampler` directly
  - `FencePool` uses `hal.Device`/`hal.Fence` directly
  - `DeviceProvider` returns `hal.Device`/`hal.Queue` directly
  - All GPU errors propagated via `fmt.Errorf("context: %w", err)` chains
  - Resolves [#84](https://github.com/gogpu/gogpu/issues/84)

- **Rust backend as thin HAL adapter** — Rewritten `gpu/backend/rust/rust.go` from handle-based
  `gpu.Backend` (17 handle maps, 1136 LOC) to thin wrapper structs implementing `hal.*`
  interfaces (24 wrappers, 1580 LOC, zero handle maps). Each `rust*` struct holds a
  `*wgpu.*` pointer and delegates directly — no map lookups, no uintptr handles.
  - `rustDevice` implements `hal.Device` (30+ methods)
  - `rustQueue` implements `hal.Queue` (Submit, WriteBuffer, ReadBuffer, Present)
  - `rustCommandEncoder` implements `hal.CommandEncoder` (barriers are no-ops)
  - `rustRenderPass`/`rustComputePass` implement render/compute pass encoders
  - Fences: stub implementation (wgpu-native uses `device.Poll()`)
  - Backend selection in `renderer.init()`: Auto/Native/Rust via build-tagged files

- **Removed diagnostic logging from renderer** — Replaced ad-hoc `fmt.Printf`/`log.Printf`
  calls with structured `slog` logger. All diagnostic output now goes through the
  configurable logging system (silent by default).

### Dependencies
- **wgpu v0.15.0 → v0.16.0** — GLES pipeline, Metal/DX12/Vulkan fixes, slog, lint cleanup
- **naga v0.12.0 → v0.13.0** — GLSL backend, HLSL/SPIR-V fixes

### Removed

- **`gpu.Backend` interface** — Legacy 40-method interface with uintptr handles, replaced by
  `hal.*` Go interfaces. Deleted `gpu/backend.go` (158 LOC).
- **`gpu/registry.go`** — Legacy backend registration system (RegisterBackend, SelectBestBackend,
  etc.). No longer needed — backends are selected directly in renderer. Deleted 122 LOC + 271 LOC tests.
- **`gpu/types/handles.go`** — Unused uintptr handle type aliases (Instance, Adapter, Device, etc.).
  All code now uses `hal.*` interface types. Deleted 122 LOC.
- **`gpu/types/descriptors.go`** — Unused descriptor types that referenced uintptr handles.
  All code now uses `hal.*` descriptor types. Deleted 175 LOC.
- **`gpu/backend_darwin_test.go`** — Metal integration test using legacy `gpu.Backend` API.
  Deleted 233 LOC.
- **`gpu/sdf` package** — GPU SDF accelerator moved to gg repository where it belongs.
- **Total: -1623 lines** of legacy indirection code removed.

## [0.17.0] - 2026-02-10

### Added

- **HalProvider support** — `GPUContextProvider()` now implements `gpucontext.HalProvider`,
  exposing low-level HAL device and queue for GPU accelerators (e.g. gg SDF compute shaders)
  - `HalDevice() any` — returns `hal.Device` for direct GPU operations
  - `HalQueue() any` — returns `hal.Queue` for command submission
- **HalResourceProvider** — `GetHalDevice()` / `GetHalQueue()` resolve handle-based
  gogpu types to underlying wgpu HAL objects (both Vulkan and Metal backends)
- **Full compute pipeline support in native backend** — compute pipelines, bind groups,
  compute passes, buffer creation with readback — works on both Vulkan and Metal via HAL
- **`MapBufferRead` / `UnmapBuffer`** — GPU→CPU buffer readback via `hal.Queue.ReadBuffer`
  in native backend
- **`CopyBufferToBuffer`** — new Backend interface method for GPU-side buffer copies
- **Full compute support in Rust backend** — CreateComputePipeline, BeginComputePass,
  SetComputePipeline, SetComputeBindGroup, DispatchWorkgroups, EndComputePass,
  MapBufferRead, CreateShaderModuleSPIRV — all implemented via go-webgpu/webgpu v0.3.0

### Refactored

- **Unified native backend** — eliminated ~950 lines of code duplication between
  Vulkan and Metal backends. Single `backend.go` implementation via `hal.Device`/`hal.Queue`
  interfaces, with thin platform files (`hal_vulkan.go`, `hal_metal.go`) for backend selection.
  Metal now gets all compute/buffer/fence operations for free through HAL abstraction.

### Changed

- **gpucontext** dependency updated v0.8.0 → v0.9.0
- **wgpu** dependency updated v0.14.0 → v0.15.0 (ReadBuffer, compute support)
- **go-webgpu/webgpu** dependency updated v0.2.1 → v0.3.0
- **naga** dependency updated v0.11.1 → v0.12.0 (indirect, function calls, SPIR-V fixes)
- **golang.org/x/sys** updated v0.40.0 → v0.41.0

## [0.16.0] - 2026-02-07

### Added

- **WindowProvider interface** — `App` implements `gpucontext.WindowProvider`
  - `ScaleFactor() float64` — DPI scale factor (Windows: GetDpiForWindow, macOS/Linux: stubs)
  - `Size()` and `RequestRedraw()` already existed

- **PlatformProvider interface** — `App` implements `gpucontext.PlatformProvider`
  - `ClipboardRead() / ClipboardWrite()` — system clipboard (Windows: full, macOS/Linux: stubs)
  - `SetCursor(CursorShape)` — 12 standard cursor shapes (Windows: full, macOS/Linux: stubs)
  - `DarkMode()` — system dark mode detection (Windows: registry query)
  - `ReduceMotion()` — accessibility preference (Windows: SystemParametersInfo)
  - `HighContrast()` — high contrast mode (Windows: SystemParametersInfo)
  - `FontScale()` — font size multiplier (Windows: from DPI)

### Changed

- **gpucontext** dependency updated v0.7.0 → v0.8.0

## [0.15.7] - 2026-02-07

### Fixed

- **Vulkan crash on NVIDIA when creating premultiplied alpha pipeline** — Eliminated the
  second GPU render pipeline entirely. Both premultiplied and straight alpha textures now
  use a single pipeline with a uniform-based shader switch (`uniforms.premultiplied`).
  The shader premultiplies straight alpha data before output, so the blend state is always
  `One / OneMinusSrcAlpha`. Fixes `Exception 0xc0000005` crash on NVIDIA RTX 2080
  (Studio Driver 591.74) in `vkCreateGraphicsPipelines`.
  - Removed: `initTexQuadPremulPipeline()`, duplicate shader module, duplicate pipeline layout
  - `Texture.SetPremultiplied()` / `Texture.Premultiplied()` API unchanged
  - Reported by @amortaza in Discussion #47

### Changed

- **naga** dependency updated v0.10.0 → v0.11.0 — fixes SPIR-V `if/else` GPU hang, adds 55 new WGSL built-in functions
- **wgpu** dependency updated v0.13.1 → v0.13.2

## [0.15.6] - 2026-02-06

### Fixed

- **Animation freeze during window drag/resize on Windows** — Rendering now continues
  smoothly during Win32 modal resize/move loop via WM_TIMER callback at ~60fps
  - Added `SetModalFrameCallback` to Platform interface (internal)
  - SetTimer/KillTimer on WM_ENTERSIZEMOVE/WM_EXITSIZEMOVE
  - Full update+render cycle on each timer tick (onUpdate, onDraw, resize propagation)
  - macOS/Linux unaffected (no modal loops on those platforms)
  - Industry-standard approach used by GLFW, SDL, winit

## [0.15.5] - 2026-02-05

### Fixed

- **Dark halos around anti-aliased shapes** — Premultiplied alpha pipeline for correct compositing
  - `Texture.Premultiplied() bool` — Check if texture uses premultiplied alpha
  - `Texture.SetPremultiplied(bool)` — Mark texture as premultiplied
  - `TextureOptions.Premultiplied` — Set during texture creation
  - Auto-set for textures created from Go `image.Image` (always premultiplied)
  - New WGSL fragment shader: `return texColor * uniforms.alpha` (premultiplied variant)
  - Dual render pipeline: `BlendFactorSrcAlpha` (straight) / `BlendFactorOne` (premultiplied)
  - Pipeline selected automatically at draw time based on `texture.premultiplied` flag
  - Fixes dark halos around anti-aliased shapes when compositing from gg/ggcanvas

## [0.15.4] - 2026-02-05

### Added

- **Compile-time check** for `gpucontext.TextureUpdater` on `Texture` type
  - Ensures `Texture.UpdateData([]byte) error` satisfies the shared interface

### Changed

- **Moved `gg_integration` example to gg repo** — gogpu no longer depends on gg
  - Example now lives at [`github.com/gogpu/gg/examples/gogpu_integration`](https://github.com/gogpu/gg/tree/main/examples/gogpu_integration)
  - Fixes inverted dependency: low-level framework should not depend on high-level library
  - Removed `github.com/gogpu/gg` from `go.mod`

## [0.15.3] - 2026-02-03

### Fixed

- **Windows Modifier Keys** — Ctrl, Shift, Alt now work correctly in `Pressed()` and `Modifier()`
  - Implemented GLFW/Ebiten scancode-based pattern for accurate Left/Right detection
  - Windows sends generic VK codes (0x10-0x12), not specific L/R codes — now handled correctly
  - Added AltGr detection for European keyboard layouts (Ctrl+Alt sequence)
  - Thanks to @qq1792569310 for testing and reporting ([#71](https://github.com/gogpu/gogpu/issues/71))

## [0.15.2] - 2026-02-03

### Fixed

- **Input State Initialization** — `app.Input().Keyboard().Pressed()` now works correctly in `OnUpdate`
  - Input state is now initialized before event callbacks are registered
  - Fixes race condition where key events were missed on first frame
  - Follows Ebitengine/GLFW/SDL pattern for eager initialization
  - Thanks to @qq1792569310 for reporting ([#71](https://github.com/gogpu/gogpu/issues/71))

## [0.15.1] - 2026-02-02

### Fixed

- **Windows Alt Key Events** — Alt key now works correctly on Windows
  - Added `WM_SYSKEYDOWN`/`WM_SYSKEYUP` message handlers
  - Windows sends Alt through system key messages, not regular key messages
  - Alt+F4 preserved, menu activation suppressed
  - Thanks to @qq1792569310 for reporting ([#67](https://github.com/gogpu/gogpu/pull/67))

## [0.15.0] - 2026-02-01

### Added

- **Render-on-Demand Mode** — Power-efficient UI rendering
  - `Config.WithContinuousRender(false)` — Only render on events
  - `App.RequestRedraw()` — Explicitly request frame redraw
  - Reduces GPU usage from ~100% to ~8% for static UI

- **Texture.UpdateData Improvements** (INT-003)
  - `Texture.BytesPerPixel()` — Format-aware size calculation
  - Support for 20+ texture formats (1/2/4/8/16 bytes per pixel)
  - Dedicated error types: `ErrTextureUpdateDestroyed`, `ErrInvalidDataSize`, `ErrRegionOutOfBounds`, `ErrInvalidRegion`

- **Fence-based GPU Synchronization** (EVENT-002)
  - `Fence` and `SubmissionIndex` types in `gpu/types`
  - Backend interface extended with fence operations:
    - `CreateFence`, `WaitFence`, `ResetFence`, `DestroyFence`
    - `GetFenceValue` for non-blocking completion check
  - `SubmissionTracker` following wgpu-rs LifetimeTracker pattern
  - Non-blocking `EndFrame` with submission-indexed fence signaling

- **Renderer Memory Optimizations** (EVENT-002)
  - Pre-allocated uniform buffer for texture rendering (eliminates 32 bytes/frame GC)
  - Bind group caching per texture (eliminates per-draw GPU allocations)

- **Unified Event System** — Complete input handling overhaul
  - **W3C Pointer Events Level 3** — Unified mouse/touch/pen input
  - **Gesture Recognition** — Vello-style pinch, rotate, pan detection
  - **Ebiten-style Input Polling** — `app.Input().Keyboard().JustPressed(key)`
  - **Thread-safe InputState** — Safe for game loop polling

- **Platform Keyboard Events** — All platforms
  - Windows: WM_KEYDOWN/WM_KEYUP with full key mapping
  - Linux (Wayland): wl_keyboard events with evdev keycodes
  - macOS: NSEvent keyDown/keyUp with virtual keycodes

- **Platform Pointer Events** — All platforms
  - Windows: WM_MOUSE* events with button/modifier tracking
  - Linux (Wayland): wl_pointer with scroll and button events
  - Linux (X11): MotionNotify, ButtonPress with scroll buttons 4-7
  - macOS: NSEvent mouse events with trackpad detection

### Changed

- **Update gpucontext v0.4.0 → v0.6.0** — Pointer, Scroll, Gesture Events
- **Update naga v0.9.0 → v0.10.0** — Storage textures, switch statements
- **Update wgpu v0.12.0 → v0.13.0** — Format capabilities, array textures, render bundles

## [0.14.0] - 2026-01-30

### Added

- **gpucontext.TextureDrawer implementation** — Cross-package texture rendering
  - `Context.AsTextureDrawer()` — Returns adapter for gpucontext.TextureDrawer interface
  - `TextureCreator.NewTextureFromRGBA()` — Create textures from RGBA pixel data
  - Enables gg/ggcanvas integration without direct gogpu imports

### Changed

- **Update gpucontext v0.3.1 → v0.4.0** — Texture, Touch interfaces
- **Update wgpu v0.11.2 → v0.12.0** — BufferRowLength fix (aspect ratio)
- **Update naga v0.8.4 → v0.9.0** — Shader compiler improvements

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

[Unreleased]: https://github.com/gogpu/gogpu/compare/v0.20.5...HEAD
[0.20.5]: https://github.com/gogpu/gogpu/compare/v0.20.4...v0.20.5
[0.20.4]: https://github.com/gogpu/gogpu/compare/v0.20.3...v0.20.4
[0.20.3]: https://github.com/gogpu/gogpu/compare/v0.20.2...v0.20.3
[0.20.2]: https://github.com/gogpu/gogpu/compare/v0.20.1...v0.20.2
[0.20.1]: https://github.com/gogpu/gogpu/compare/v0.20.0...v0.20.1
[0.20.0]: https://github.com/gogpu/gogpu/compare/v0.19.2...v0.20.0
[0.19.2]: https://github.com/gogpu/gogpu/compare/v0.19.1...v0.19.2
[0.19.1]: https://github.com/gogpu/gogpu/compare/v0.19.0...v0.19.1
[0.19.0]: https://github.com/gogpu/gogpu/compare/v0.18.2...v0.19.0
[0.18.2]: https://github.com/gogpu/gogpu/compare/v0.18.1...v0.18.2
[0.18.1]: https://github.com/gogpu/gogpu/compare/v0.18.0...v0.18.1
[0.18.0]: https://github.com/gogpu/gogpu/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/gogpu/gogpu/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/gogpu/gogpu/compare/v0.15.7...v0.16.0
[0.15.7]: https://github.com/gogpu/gogpu/compare/v0.15.6...v0.15.7
[0.15.6]: https://github.com/gogpu/gogpu/compare/v0.15.5...v0.15.6
[0.15.5]: https://github.com/gogpu/gogpu/compare/v0.15.4...v0.15.5
[0.15.4]: https://github.com/gogpu/gogpu/compare/v0.15.3...v0.15.4
[0.15.3]: https://github.com/gogpu/gogpu/compare/v0.15.2...v0.15.3
[0.15.2]: https://github.com/gogpu/gogpu/compare/v0.15.1...v0.15.2
[0.15.1]: https://github.com/gogpu/gogpu/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/gogpu/gogpu/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/gogpu/gogpu/compare/v0.13.3...v0.14.0
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
