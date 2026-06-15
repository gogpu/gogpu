# GoGPU Roadmap

> **Pure Go GPU Computing Ecosystem**
>
> Designed to power professional graphics applications, game engines, and IDEs.

---

## Vision

**GoGPU** is a Pure Go GPU computing ecosystem designed for:
- Professional graphics applications
- IDEs and development tools
- Game engines and simulations
- Cross-platform GUI applications

Our goal is to become the **reference graphics ecosystem** for Go — comparable to the Rust ecosystem (wgpu, naga, vello).

### Core Principles

1. **Pure Go** — No CGO, easy cross-compilation, single binary deployment
2. **WebGPU-First** — Follow W3C WebGPU specification
3. **Triple Backend** — Pure Go (default), Rust FFI (`-tags rust`), Browser WASM (ADR-038)
4. **Enterprise-Ready** — Production-grade error handling and patterns

---

## Current State: v0.41.14

✅ **Production-ready** with full feature set:
- **CSD maximize/fullscreen geometry** (#300) — 5 bugs fixed (enterprise research: GTK4, winit/SCTK, SDL3/libdecor). Negative offset geometry model, fullscreen state parsing, decoration lifecycle.
- **Hidden-then-show window creation** — GLFW/Ebiten/SDL3/Flutter pattern: window created hidden, shown after GPU init. Eliminates black flash and WM_SETFOCUS race on all platforms.
- **Universal App Lifecycle** — RenderTarget, QuitOnLastWindowClosed, AppLifecycle enum (5 states), surface/lifecycle callbacks (ADR-026, Phases 1-3)
- **macOS system menu** — `SetMenu()`, `SetCustomMenu()`, `MenuRole`, native menu bar (ADR-022, @lkmavi)
- **Linux clipboard** — X11 ICCCM selection + Wayland wl_data_device (PLAT-009, ADR-037)
- **Wayland cursor shapes** — 12 cursor shapes via wp_cursor_shape_manager_v1, CSD resize cursors (PLAT-008)
- **Three-mode render loop** — IDLE/ANIMATING/CONTINUOUS modes with lazy swapchain acquire (ADR-023)
- **SubpixelLayout detection** — LCD/ClearType auto-detect on all platforms (ADR-024)
- **Platform system sounds** — `sound.Play(sound.Click)` on Windows/macOS/Linux, zero CGO (ADR-025)
- **Window close lifecycle** — `SetOnClose(func() bool)` rejection, ID pool, `OnAnyWindowClosed` (ADR-022, @lkmavi)
- **macOS window delegate** — `GoGPUWindowDelegate` with `windowShouldClose:`, per-window routing (ADR-022 Phase 2, @lkmavi)
- **Centralized input dispatch** — all input events through `PollEvents()` with `WindowID` (ADR-021, @lkmavi)
- **Adapter-aware render mode** — `GOGPU_RENDER_MODE=auto|cpu|gpu` (ADR-020)
- **macOS native window tabbing** — `Config.WithTabbingMode()` + `WithTabbingIdentifier()` (@lkmavi)
- **Runtime fullscreen** — `App.SetFullscreen(bool)`, `App.ToggleFullscreen()` on all platforms (ADR-018)
- **Multi-window** — `App.NewWindow()` creates additional windows with shared GPU device (ADR-010)
- **Damage-aware presentation** — `Context.SetDamageRects()` passes dirty regions to compositor (ADR-013)
- **Triple-backend WebGPU** — Pure Go / Rust FFI / Browser WASM via build tags (ADR-038)
- **Native file dialogs** — macOS NSPanel, Windows COM, Linux D-Bus + zenity/kdialog (ADR-036, @lkmavi)
- **Native menus** — macOS NSMenu, Windows HMENU, Linux D-Bus AppMenu (ADR-040, @lkmavi)
- **Wayland thread safety** — app event queue separation, CSD displayMu, configure gate, wl_shm eager init (ADR-041, #292)
- **PlatformManager / PlatformWindow** — clean process-level / per-window split (Qt6 pattern)
- Multi-thread architecture (Ebiten/Gio pattern)
- Event-driven rendering with three-state model (0% CPU when idle)
- **ScrollPhase / IsMomentum** — macOS trackpad momentum detection (ADR-032), pixel/line/page delta modes
- **Wayland key repeat** — client-side timer via timerfd, `xkb_keymap_key_repeats` (ADR-033)
- **Multi-keyboard layout (X11 + Wayland)** — XKB extension + xkbcommon, group-aware keysym lookup, Cyrillic/Ukrainian/Belarusian (ADR-027, @unxed)
- **Unified XKB text input** — shared xkbcommon for X11+Wayland, AltGr/Level3 on all international layouts, named modifier resolution, `KeyWithoutModifiers` for shortcuts (ADR-029, @unxed)
- **Unicode text input** — SetCharCallback on all platforms (Win32/macOS/X11/Wayland)
- **HiDPI logical sizing** — `WithSize` in logical DIP, per-monitor DPI, WM_DPICHANGED (ADR-030)
- **Ring buffer event queue** — lock-free EventQueue[T] on all platforms (ADR-031)
- **Automatic GPU resource lifecycle** — `TrackResource(io.Closer)` + LIFO shutdown
- DeviceProvider/EventSource/WindowProvider/PlatformProvider for UI integration
- Zero-copy surface rendering via SurfaceView
- Cross-platform: Windows (Vulkan/DX12), Linux (Vulkan/Wayland), macOS (Metal)
- **Software backend** — always available, Windows/macOS/X11 screen presentation
- Structured logging via log/slog with `GOGPU_LOG` env var
- **HiDPI/Retina** — logical/physical pixel split, per-monitor DPI, programmatic DPI awareness
- **X11 multi-touch** via XInput2 pure Go wire protocol
- **Frameless windows** — `Config.Frameless` + WindowChrome interface (JBR pattern on Win32)
- **Wayland CSD** — client-side decorations with title bar, buttons, edge resize
- **GPU compute** — compute shaders with GPU particles example
- **Deferred resource destruction** — Rust LifetimeTracker parity in wgpu
- **Mouse grab / pointer lock** — locked, confined, normal modes (SDL parity, Win32 + X11 + Wayland)
- **Adapter power preference** — `GOGPU_POWER_PREFERENCE` env var for dual-GPU laptops
- **Event-driven frame pacing** — render only on invalidation, 0% GPU when idle (winit/Flutter/Qt pattern)

### Recent Highlights

| Version | Date | Key Changes |
|---------|------|-------------|
| **v0.39.3** | 2026-05-26 | **Linux clipboard** (PLAT-009, ADR-037) — X11 ICCCM selection + Wayland data_device. ClipboardRead/Write work on all Linux platforms. deps: wgpu v0.28.8. |
| **v0.39.2** | 2026-05-25 | **Wayland cursor shapes** (wp_cursor_shape_manager_v1, 12 shapes + CSD resize cursors) + **platform fixes** — damage_buffer (#272), Activated→EventFocus (#273), DPI MouseLeave (#271), X11 HitTest (#270), dispatchFocus (BUG-FOCUS-001) |
| **v0.39.1** | 2026-05-22 | **AppLifecycle enum + callbacks** (ADR-026 Phase 3 complete) — AppLifecycle (5 states), OnSurfaceAvailable/Destroyed, OnResumed/Suspended/MemoryWarning |
| **v0.39.0** | 2026-05-22 | **ADR-026 Universal Lifecycle** — QuitOnLastWindowClosed, primary close resilience, RenderTarget (public type), initDevice/initSurface split, SurfaceState, WindowID real type. `examples/lifecycle/`. deps: wgpu v0.28.7 |
| **v0.38.0** | 2026-05-21 | **macOS menu API** (#242, @lkmavi) + **custom dynamic menus** (#264, @lkmavi) — SetMenu, SetCustomMenu, MenuRole, Submenu. **Renderer decoupling** (LIFECYCLE Phase 2) — SurfaceState, per-window platWindow. PlatformProvider delegation (ADR-024). deps: wgpu v0.28.6 (GLES hidden window) |
| **v0.37.12** | 2026-05-21 | **PlatformProvider delegation** (ADR-024) -- GPUContextAdapter implements PlatformProvider, LCD auto-detection works. deps: wgpu v0.28.6 (GLES hidden window) |
| **v0.37.11** | 2026-05-21 | **deps:** wgpu v0.28.5 (indirect validation nil guard, Metal present fixes) |
| **v0.37.10** | 2026-05-19 | **timerfd key repeat** (#240) -- goroutine→timerfd in Poll set, fixes GUI freeze + xkb data race |
| **v0.37.9** | 2026-05-17 | **deps:** wgpu v0.28.3 |
| **v0.37.8** | 2026-05-17 | **deps:** wgpu v0.28.2 (swapchain extent diagnostics) |
| **v0.37.7** | 2026-05-17 | **Windows ESC fix** (#254) -- removed hardcoded ESC=close, app decides |
| **v0.37.6** | 2026-05-17 | **Universal keysym-to-Unicode** (ADR-034) -- 828-entry table replaces 70-entry Cyrillic, all X11 scripts |
| **v0.37.5** | 2026-05-17 | **AltGr/Level3 fix** (#233) -- XkbModifierStateMask subscription, guillemets work |
| **v0.37.4** | 2026-05-17 | **X11 layout switch fix** (#233) -- lockedGroup uint8 bug + Wayland pattern for effective group |
| **v0.37.3** | 2026-05-17 | **X11 initial state sync + XWayland** (#233) -- xkbGetFullState + UpdateMask, XWayland detection, XkbNewKeyboardNotify |
| **v0.37.2** | 2026-05-17 | **Diagnostic logging** (#247) — slog.Debug in PresentTexture/drawTexturedQuad/resize for HiDPI debugging |
| **v0.37.1** | 2026-05-17 | **X11 Russian keyboard fix** (#233) — `_XKB_RULES_NAMES` root window property for multi-layout keymap (pure Go, zero deps) |
| **v0.37.0** | 2026-05-17 | **ScrollPhase/IsMomentum** (#239, ADR-032) + **Wayland key repeat** (#240, ADR-033) + **X11 detectable auto-repeat** — gpucontext v0.19.0 |
| **v0.36.2** | 2026-05-16 | **HiDPI logical sizing** (#237, ADR-030) + **Ring buffer event queue** (#238, ADR-031) — WithSize=logical, EventQueue[T] all platforms |
| **v0.36.1** | 2026-05-16 | **X11 keyboard regression fix** — XkbStateNotify 6-field sync (winit pattern), UpdateKey→UpdateMask, cascading fallback |
| **v0.36.0** | 2026-05-16 | **Unified XKB text input** (#233, ADR-029, @unxed) — AltGr/Level3 on all layouts, shared xkbcommon for X11+Wayland, 15 FFI bindings, ModsIndices, KeyWithoutModifiers |
| **v0.35.0** | 2026-05-15 | **Browser/WASM platform** + XKB constant fix (#70, #227) — `GOOS=js GOARCH=wasm`, wgpu v0.28.1, bits 13-14 group extraction |
| **v0.34.8** | 2026-05-15 | **Wayland keyboard layout** + X11 runtime switch fix (#227, @paulie-g) — xkbcommon, MappingNotify fallback, 44 tests |
| **v0.34.7** | 2026-05-14 | **Multi-keyboard layout X11** (#227, ADR-027, @unxed) — XKB group tracking, Cyrillic keysyms, 27 tests |
| **v0.34.6** | 2026-05-14 | Deferred SetHitTestCallback — frameless drag fix |
| **v0.34.5** | 2026-05-14 | deps: wgpu v0.27.5 (NULL handle guards) |
| **v0.34.4** | 2026-05-13 | macOS delegate + PUA key filter + Linux EventClose + deps wgpu v0.27.4 |
| **v0.34.3** | 2026-05-11 | deps: wgpu v0.27.3 |
| **v0.34.2** | 2026-05-11 | **Window close lifecycle** (#213, ADR-022, @lkmavi) — close rejection, ID pool, OnAnyWindowClosed |
| **v0.34.1** | 2026-05-10 | deps: wgpu v0.27.2 |
| **v0.34.0** | 2026-05-09 | **System sounds** (ADR-025) — `sound/` subpackage, winmm/NSSound/canberra, zero CGO |
| **v0.33.0** | 2026-05-09 | **SubpixelLayout detection** (ADR-024) — LCD/ClearType auto-detect, all platforms, gpucontext v0.18.0 |
| **v0.32.3** | 2026-05-08 | **Three-mode render loop** (ADR-023) — IDLE/ANIMATING/CONTINUOUS, lazy acquire, 10%→<1% GPU for UI |
| **v0.32.2** | 2026-05-07 | Fix EventSource callback loss on Run() — lazy init for pre-Run() registrations |
| **v0.32.1** | 2026-05-07 | **Centralized input dispatch** (ADR-021, #210, @lkmavi) — multi-window input fix, per-window callbacks, 54 tests |
| **v0.32.0** | 2026-05-06 | **Render mode** (ADR-020), **macOS tabbing** (@lkmavi), AdapterInfo, wgpu v0.27.0 |
| **v0.31.1** | 2026-05-05 | X11 remote display auth fix (#203, @sverrehu), lint cleanup |
| **v0.31.0** | 2026-05-01 | **Runtime fullscreen** (ADR-018) — all 4 platforms, wgpu v0.26.12, gpucontext v0.16.0 |
| **v0.30.3** | 2026-04-30 | Multi-window deadlock + lost events fix (ADR-017), scroll accumulate+snapshot, particle sim example (@snakeru), wgpu v0.26.10 (45% validation) |
| **v0.29.4** | 2026-04-26 | wgpu v0.26.6 — compute barriers (VAL-008/009/010) |
| **v0.29.2** | 2026-04-25 | **Damage-aware presentation** + Vulkan validation fixes (uniform buffer CopyDst, PRESENT_SRC_KHR), wgpu v0.26.4 |
| **v0.28.1** | 2026-04-23 | EventFocus on all platforms (Win32, X11, Wayland, macOS), WindowID on all events |
| **v0.28.0** | 2026-04-23 | **Multi-window** — App.NewWindow(), PlatformManager/PlatformWindow, shared GPU device, per-window frame loop |
| **v0.27.1** | 2026-04-21 | Wayland pointer lock, adapter power preference, X11 event loop fix, macOS blit fix |
| **v0.27.0** | 2026-04-09 | Mouse grab / pointer lock — Win32 + X11 (SDL parity) |
| **v0.26.0** | 2026-03-31 | Enterprise fence architecture, Wayland CSD, GPU particles, present mode fallback |
| **v0.25.0** | 2026-03-21 | Frameless windows (Win32/macOS/X11/Wayland), WM_DPICHANGED, VSync config |

---

## Upcoming

### v0.42.x — HiDPI Enterprise Fix (#306, ADR-044)

Windows DPI-aware window creation. Unified pre-scale + verify pattern (Qt6/SDL3/winit research).

- [x] macOS ScaleFactor three-tier resolution (PR #313, @lkmavi)
- [x] Windows pre-scale + verify + WM_GETDPISCALEDSIZE + PlatScaleProvider (PR #314, @lkmavi, ADR-044)
- [x] macOS physical-size tracking + windowDidChangeScreen delegate (PR #316, @lkmavi)
- [ ] `Config.Width`/`Config.Height` docs: "pixels" → "logical points (DIP)"
- [ ] @TimLai666 verification on AMD Radeon 890M, Windows 11, 200%

### v0.42.x — API Cleanup (#311)

Enterprise API audit: unexport internals, remove dead code, fix documentation.

- [ ] Unexport 7 internal types: SurfaceState, RenderTarget, WindowManager, PlatformWindowCloser, AnimationController, Invalidator, GestureRecognizer
- [ ] Remove dead `gogpu.DeviceProvider` interface (replaced by `gpucontext.DeviceProvider`)
- [ ] Replace type aliases `FileDialogOptions`/`FileTypeFilter` with real types
- [ ] Rewrite `doc.go` (says "macOS: Planned" — fully supported since v0.21.0)
- [ ] Deprecate `Config.Backend` + `WithBackend()` (obsolete per ADR-038 build tags)
- [ ] `App.SetTitle()` — platform layer ready, expose in public API

### v0.43.x — Wayland Cursor Fallback (ADR-043)

- [ ] libwayland-cursor.so via goffi — 4 functions for cursor image loading
- [ ] Fallback chain: wp_cursor_shape_v1 → libwayland-cursor → embedded defaults

### Ecosystem Dependencies (#312)

- [ ] Cascade wgpu v0.29.15 + naga v0.17.15 → gg → ui → g3d
- [ ] Sync x/sys v0.46.0, goffi v0.5.3 across systray, audio

### Universal App Lifecycle (ADR-026)

Surface-based lifecycle for desktop + mobile + web + headless. Replaces "primary window" concept. GPU Device decoupled from any window. Research: 15 enterprise frameworks (Flutter, SDL3, Qt6, winit, Bevy).

- [x] **Phase 2** — Renderer decoupling: RenderTarget, SurfaceState, per-window platWindow, initDevice/initSurface split (v0.38.0)
- [x] **Phase 3** — Lifecycle API: `AppLifecycle` enum (5 states), `QuitOnLastWindowClosed`, OnSurfaceAvailable/Destroyed, OnResumed/Suspended/MemoryWarning (v0.39.0-v0.39.1)
- [ ] **Phase 4** — Mobile platforms: Android ANativeWindow, iOS CAMetalLayer, Web canvas (community)

### v1.0.0 — Production Release

- [ ] API stability guarantee
- [ ] Semantic versioning commitment
- [ ] Long-term support plan
- [ ] Enterprise deployment guide
- [ ] Comprehensive documentation

---

## Future Ideas

| Theme | Description | Status |
|-------|-------------|--------|
| **Multi-Window** | Multiple windows per App (IDE/tool pattern) | ✅ Shipped (v0.28.0) |
| **WebAssembly** | WASM target for browser via WebGPU | ✅ Shipped — v0.35.0, `GOOS=js GOARCH=wasm` (#70) |
| **Android** | Android platform support | Backlog (ANDROID-001) |
| **iOS** | iOS platform support | Planned |
| **Ecosystem Logging** | Unified slog-based logging across all repos | Backlog (TASK-LOG-001) |
| **System Tray** | OS-level tray icon (Win32/macOS/Linux) | ✅ Shipped — [gogpu/systray](https://github.com/gogpu/systray) v0.1.0 |
| **Native Dialogs** | File open/save, color picker, message box | Planned |
| **Drag & Drop** | OS-level and inter-window drag and drop | Planned |
| **Clipboard** | Text clipboard on all platforms (Win32/macOS/X11/Wayland). Rich clipboard (images, HTML, custom types) planned | ✅ Text shipped (v0.39.3) |
| **Notifications** | OS-level desktop notifications | Planned |
| **Independent Render Thread** | Decouple render loop from message pump | [Research](docs/dev/research/INDEPENDENT_RENDER_THREAD.md) |
| **Ray Tracing** | RT extensions when available | Future |

---

## Architecture

```
                    User Application
                          │
          ┌───────────────┼───────────────┐
          │               │               │
      gogpu/gg        gogpu/gogpu      Custom
    2D Graphics       GPU Framework     Apps
          │               │               │
          └───────────────┼───────────────┘
                          │
             gogpu/gpucontext (shared interfaces)
                          │
          ┌───────────────┼───────────────┐
          │                               │
     Rust Backend                  Pure Go Backend
   (go-webgpu/webgpu)               (gogpu/wgpu)
          │                               │
          └───────────────┼───────────────┘
                          │
    ┌─────────┬─────────┬─────────┬─────────┬─────────┐
    │ Vulkan  │  DX12   │  Metal  │  GLES   │ Software│
    │ Win+Lin │ Windows │  macOS  │ Win+Lin │ All     │
    └─────────┴─────────┴─────────┴─────────┴─────────┘
```

---

## Ecosystem

| Component | Version | Description |
|-----------|---------|-------------|
| **gogpu/gogpu** | v0.29.2 | GPU application framework, windowing, multi-window, damage-aware present |
| **gogpu/wgpu** | v0.26.4 | Pure Go WebGPU (Vulkan, Metal, DX12, GLES, Software) |
| **gogpu/naga** | v0.17.6 | Shader compiler (WGSL → SPIR-V/MSL/GLSL/HLSL/DXIL) |
| **gogpu/gg** | v0.41.2 | 2D graphics with GPU acceleration, Vello compute, scene renderer |
| **gogpu/ui** | v0.1.13 | GUI toolkit: 22+ widgets, 4 themes, offscreen renderer |
| **gogpu/gpucontext** | v0.14.0 | Shared interfaces (DeviceProvider, TextureView, TextureRegionUpdater) |
| **gogpu/gputypes** | v0.5.0 | WebGPU type definitions (zero value = spec default) |
| **gogpu/compose** | design | Multi-process composition library |
| **gogpu/g3d** | v0.1.0 | 3D rendering (scene graph, PBR Blinn-Phong, forward renderer, 5 backends) |
| **gogpu/gg-pdf** | v0.1.0 | PDF export |
| **gogpu/gg-svg** | v0.1.0 | SVG export |

**Total: ~635K+ lines of Pure Go, zero CGO.**

---

## Released Versions

| Version | Date | Highlights |
|---------|------|------------|
| **v0.29.2** | 2026-04-25 | Damage-aware presentation, Vulkan validation fixes, wgpu v0.26.4 |
| **v0.28.1** | 2026-04-23 | EventFocus on all platforms, WindowID on all events |
| **v0.28.0** | 2026-04-23 | Multi-window (ADR-010), PlatformManager/PlatformWindow, WindowManager |
| **v0.27.0** | 2026-04-09 | Mouse grab / pointer lock (SDL parity) |
| **v0.26.1** | 2026-04-05 | CSD resize overhaul, event queue pattern, DPI awareness |
| **v0.26.0** | 2026-03-31 | Enterprise fence, Wayland CSD + single connection, GPU particles, present mode fallback |
| **v0.25.1** | 2026-03-25 | X11/Wayland DPI, macOS platform stubs, BlitPixels cross-platform |
| **v0.25.0** | 2026-03-21 | Frameless windows, WM_DPICHANGED, VSync config, WindowChrome |
| v0.24.5 | 2026-03-18 | SetLogger propagation to all subsystems (#150) |
| v0.24.4 | 2026-03-16 | GOGPU_GRAPHICS_API env var, PresentTexture, RenderTarget |
| v0.24.3 | 2026-03-16 | wgpu v0.21.2 (core validation layer) |
| v0.24.2 | 2026-03-15 | Rust backend adapter limits fix |
| v0.24.1 | 2026-03-15 | X11/Wayland Unicode text input (#138) |
| **v0.24.0** | 2026-03-15 | Renderer → wgpu public API, Unicode text input, FencePool migration |
| **v0.23.0** | 2026-03-11 | Logical/physical pixel split, macOS Retina, PhysicalSize API |
| **v0.22.0** | 2026-02-27 | X11 multi-touch (XInput2), extension query infrastructure |
| v0.21.0 | 2026-02 | Wayland Vulkan surface, server-side decorations |
| **v0.20.0** | 2026-02 | TrackResource, ResourceTracker, deferred destruction queue |
| v0.19.0 | 2026-02 | Cross-platform Rust backend |
| **v0.18.0** | 2026-02 | HAL-direct, GraphicsAPI selection, SurfaceView, slog, event-driven model |
| v0.17.0 | 2026-02 | HalProvider, compute support |
| v0.16.0 | 2026-02 | WindowProvider, PlatformProvider |
| v0.15.x | 2026-02 | Render-on-demand, Event System, modal loop rendering |
| v0.14.x | 2026-01 | gpucontext.TextureDrawer, gg/ggcanvas integration |
| v0.13.x | 2026-01 | Multi-thread architecture, gputypes integration |
| v0.12.x | 2026-01 | gpucontext integration (DeviceProvider, EventSource) |
| v0.1–0.11 | 2025-12 – 2026-01 | Core features, Wayland, X11, Cocoa, Metal, Vulkan |

> **See [CHANGELOG.md](CHANGELOG.md) for detailed release notes**

---

## Platform Support

| Platform | Windowing | GPU Backends | Status |
|----------|-----------|--------------|--------|
| **Windows** | Win32 | Vulkan, DX12, GLES, Software | Production |
| **Linux X11** | X11 | Vulkan, GLES, Software | Community Testing |
| **Linux Wayland** | Wayland (xdg-shell v6, CSD) | Vulkan, GLES, Software | Community Testing |
| **macOS** | Cocoa (AppKit) | Metal, Software | Community Testing |

All platforms use Pure Go FFI (no CGO required).

---

## Contributing

We welcome contributions! Priority areas:

1. **Platform Testing** — macOS, Linux X11/Wayland
2. **API Feedback** — Try the library and report pain points
3. **Test Cases** — Expand test coverage
4. **Examples** — Real-world usage examples
5. **Documentation** — Improve docs and guides

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Non-Goals

- **2D graphics library** — See gogpu/gg
- **Shader language design** — Follow WGSL spec

---

## License

MIT License — see [LICENSE](LICENSE) for details.
