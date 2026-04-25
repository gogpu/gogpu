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
3. **Dual Backend** — Rust (wgpu-native) or Pure Go (gogpu/wgpu)
4. **Enterprise-Ready** — Production-grade error handling and patterns

---

## Current State: v0.29.2

✅ **Production-ready** with full feature set:
- **Multi-window** — `App.NewWindow()` creates additional windows with shared GPU device (ADR-010)
- **EventFocus** — window focus/blur events on all platforms for multi-window VSync routing
- **Damage-aware presentation** — `Context.SetDamageRects()` passes dirty regions to compositor (ADR-013)
- Dual backend (Rust/Pure Go) — cross-platform (Windows, macOS, Linux)
- **PlatformManager / PlatformWindow** — clean process-level / per-window split (Qt6 pattern)
- Multi-thread architecture (Ebiten/Gio pattern)
- Event-driven rendering with three-state model (0% CPU when idle)
- **Unicode text input** — SetCharCallback on all platforms (Win32/macOS/X11/Wayland)
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

### Recent Highlights

| Version | Date | Key Changes |
|---------|------|-------------|
| **v0.29.2** | 2026-04-25 | **Damage-aware presentation** + Vulkan validation fixes (uniform buffer CopyDst, PRESENT_SRC_KHR), wgpu v0.26.4 |
| **v0.28.1** | 2026-04-23 | EventFocus on all platforms (Win32, X11, Wayland, macOS), WindowID on all events |
| **v0.28.0** | 2026-04-23 | **Multi-window** — App.NewWindow(), PlatformManager/PlatformWindow, shared GPU device, per-window frame loop |
| **v0.27.1** | 2026-04-21 | Wayland pointer lock, adapter power preference, X11 event loop fix, macOS blit fix |
| **v0.27.0** | 2026-04-09 | Mouse grab / pointer lock — Win32 + X11 (SDL parity) |
| **v0.26.0** | 2026-03-31 | Enterprise fence architecture, Wayland CSD, GPU particles, present mode fallback |
| **v0.25.0** | 2026-03-21 | Frameless windows (Win32/macOS/X11/Wayland), WM_DPICHANGED, VSync config |

---

## Upcoming

### v0.27.x — Platform Polish

- [x] Mouse grab / pointer lock — Win32 + X11 (v0.27.0)
- [x] Wayland pointer lock — `zwp_pointer_constraints_v1` + `zwp_relative_pointer_v1` (v0.27.1, #175)
- [x] Adapter power preference — `Config.PowerPreference` + `GOGPU_POWER_PREFERENCE` env var (v0.27.1, #176)
- [x] X11 event loop fix — dual-poller race with `ContinuousRender(false)` (v0.27.1, #178)
- [x] macOS software backend blit fix — `setNeedsDisplay:` after `setContents:` (v0.27.1, #172)
- [x] Software backend double-blit fix (v0.27.1)
- [ ] CSD resize cursor shapes (FEAT-CSD-CURSOR-001)
- [ ] CSD resize click jump fix (BUG-CSD-002)
- [ ] Adapter.GetInfo() API
- [ ] RenderTo method for offscreen rendering

### v0.29.0 — Damage-Aware Presentation

- [x] `Context.SetDamageRects()` — pass dirty regions to platform compositor (v0.29.0, ADR-013)
- [x] `ContextRenderTarget.SetDamageRects()` — adapter for ggcanvas integration (v0.29.0)
- [x] `Texture.TextureView()` — `gpucontext.TextureView` for duck-typed access (v0.29.0)
- [x] wgpu v0.26.2 — damage-aware `PresentWithDamage` on all backends (v0.29.0)

### v0.28.0 — Multi-Window ([RFC #167](https://github.com/orgs/gogpu/discussions/167))

- [x] Multi-window architecture (ADR-010, 7 framework studies) (v0.28.0)
- [x] PlatformManager + PlatformWindow split (v0.28.0)
- [x] Renderer split: shared GPU + per-window windowSurface (v0.28.0)
- [x] Monotonic WindowID, WindowManager with Go map registry (v0.28.0)
- [x] Per-window callbacks (onDraw, onResize, onClose) (v0.28.0)
- [x] VSync: primary window Fifo, secondary Immediate (v0.28.0)
- [x] Multi-window frame loop with activeSurface() dispatch (v0.28.0)
- [x] App.NewWindow() + real window creation + GPU surface (v0.28.0)
- [x] EventFocus on all platforms — Win32, X11, Wayland, macOS (v0.28.1)
- [x] WindowID on all events for multi-window routing (v0.28.1)
- [ ] VSync mode switching on focus change (surface reconfigure)
- [ ] Window types: Normal, Dialog, Tool, Popup with parent-child
- [ ] Close-as-request (OnClose returns bool to reject)
- [ ] Unified platform package structure (REFACTOR-PLATFORM-001)

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
| **WebAssembly** | WASM target for browser via WebGPU | Backlog (WASM-001) |
| **Android** | Android platform support | Backlog (ANDROID-001) |
| **iOS** | iOS platform support | Planned |
| **Ecosystem Logging** | Unified slog-based logging across all repos | Backlog (TASK-LOG-001) |
| **System Tray** | OS-level tray icon (Win32 Notification Area, macOS Menu Bar Extra, Linux AppIndicator/SNI) | Planned — [Research](docs/dev/research/UI_FRAMEWORK_CONCERNS.md), low retrofit cost |
| **Native Dialogs** | File open/save, color picker, message box | Planned |
| **Drag & Drop** | OS-level and inter-window drag and drop | Planned |
| **Clipboard** | Rich clipboard (images, HTML, custom types) | Planned |
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
| **gogpu/g3d** | design | 3D rendering (scene graph, PBR, GLTF) |
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
