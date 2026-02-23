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

## Current State: v0.20.2

✅ **Production-ready** with full feature set:
- Dual backend (Rust/Pure Go) — **Rust backend now cross-platform** (Windows, macOS, Linux)
- Multi-thread architecture (Ebiten/Gio pattern)
- Event-driven rendering with three-state model (0% CPU when idle)
- **Automatic GPU resource lifecycle** — `TrackResource(io.Closer)` + LIFO shutdown
- DeviceProvider/EventSource/WindowProvider/PlatformProvider for UI integration
- Zero-copy surface rendering via SurfaceView
- Cross-platform: Windows (Vulkan/DX12), Linux (Vulkan), macOS (Metal)
- Structured logging via log/slog
- HAL-direct architecture (no handle maps)

**New in v0.20.2:**
- Renderer: unconfigure surface on minimize (VK-VAL-001) — prevents stale swapchain (#98)
- wgpu v0.16.11 (Vulkan zero-extent swapchain fix, unconditional viewport/scissor)

**New in v0.20.1:**
- Touch/pen input for Win32, macOS, Wayland
- wgpu v0.16.10, naga v0.14.2

**New in v0.20.0:**
- `App.TrackResource(io.Closer)` — automatic GPU resource cleanup on shutdown (LIFO order)
- `ResourceTracker` interface — ggcanvas auto-registers, no manual OnClose needed
- `runtime.AddCleanup` safety net on Texture — GC catches forgotten Destroy()
- Deferred destruction queue on Renderer — thread-safe GPU cleanup from any goroutine

**New in v0.19.6:**
- Rust backend: implement CreateQuerySet, DestroyQuerySet, ResolveQuerySet, WaitIdle (fixes `-tags rust`)

**New in v0.19.5:**
- go-webgpu/webgpu v0.3.1 (Rust backend: ARM64 callback trampoline fix)

**New in v0.19.4:**
- wgpu v0.16.6 (Metal debug logging, goffi v0.3.9)

**New in v0.19.3:**
- wgpu v0.16.5 (per-encoder command pools, fixes VkCommandBuffer crash)

**New in v0.19.2:**
- 52 enterprise hot-path benchmarks (zero-allocation math confirmed)
- wgpu v0.16.4 (timeline semaphore, FencePool, −32% naga allocations)

**New in v0.19.1:**
- `WaitIdle()` call before GPU resource destruction — fixes DX12 crash on exit

### v0.18.x Features
- ✅ **Event-driven three-state model** — IDLE (0% CPU) / ANIMATING (VSync) / CONTINUOUS
- ✅ **AnimationToken** — Token-based animation lifecycle with atomic counter
- ✅ **Native WaitEvents/WakeUp** — macOS (Cocoa), Linux (X11 poll), Windows (MsgWait)
- ✅ **SurfaceView** — Zero-copy rendering for gg/ggcanvas integration
- ✅ **GraphicsAPI selection** — Runtime Vulkan/DX12/Metal/GLES/Software choice
- ✅ **HAL-direct architecture** — hal.Device/Queue interfaces, no handle maps
- ✅ **Structured logging** — log/slog, silent by default
- ✅ **DX12 deferred clear** — Fixes content loss on FLIP_DISCARD swapchains

---

## Upcoming

### v0.19.0 — API Polish
- [ ] Adapter.GetInfo() API
- [ ] RenderTo method for offscreen rendering
- [ ] macOS/Linux PlatformProvider native implementations
- [ ] Performance optimizations

### v1.0.0 — Production Release
- [ ] API stability guarantee
- [ ] Semantic versioning commitment
- [ ] Long-term support plan
- [ ] Enterprise deployment guide
- [ ] Comprehensive documentation

---

## Future Ideas

| Theme | Description | Research |
|-------|-------------|----------|
| **Independent Render Thread** | Decouple render loop from message pump via command-buffer pattern | [Research](docs/dev/research/INDEPENDENT_RENDER_THREAD.md) |
| **gogpu/ui** | GUI toolkit based on gg | — |
| **WebAssembly** | WASM target for browser | — |
| **Mobile** | Android/iOS support | — |
| **Ray Tracing** | RT extensions when available | — |

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
    │ Win+Lin │ Windows │  macOS  │ Win+Lin │ Headless│
    └─────────┴─────────┴─────────┴─────────┴─────────┘
```

---

## Ecosystem

| Component | Description |
|-----------|-------------|
| **gogpu/gogpu** | GPU abstraction, windowing, input |
| **gogpu/gpucontext** | Shared interfaces (DeviceProvider, EventSource) |
| **gogpu/gputypes** | Shared WebGPU types (TextureFormat, BufferUsage) |
| **gogpu/wgpu** | Pure Go WebGPU (Vulkan, Metal, DX12, GLES, Software) |
| **gogpu/naga** | WGSL shader compiler (SPIR-V, MSL, GLSL, HLSL) |
| **gogpu/gg** | 2D graphics library with GPU acceleration |

---

## Released Versions

| Version | Date | Highlights |
|---------|------|------------|
| **v0.19.5** | 2026-02 | webgpu v0.3.1 (Rust backend: ARM64 callback fix) |
| v0.19.4 | 2026-02 | wgpu v0.16.6 (Metal debug logging, goffi v0.3.9) |
| v0.19.3 | 2026-02 | wgpu v0.16.5 (per-encoder command pools) |
| v0.19.2 | 2026-02 | Hot-path benchmarks, wgpu v0.16.4 (timeline semaphore, FencePool) |
| v0.19.1 | 2026-02 | WaitIdle cleanup, wgpu v0.16.3 |
| v0.19.0 | 2026-02 | Cross-platform Rust backend, wgpu v0.16.2 |
| v0.18.2 | 2026-02 | Update wgpu v0.16.1 (Vulkan framebuffer cache fix) |
| v0.18.1 | 2026-02 | Event-driven three-state model, native WaitEvents, AnimationToken |
| v0.18.0 | 2026-02 | HAL-direct, GraphicsAPI selection, SurfaceView, slog |
| v0.17.0 | 2026-02 | HalProvider, compute support, unified native backend |
| v0.16.0 | 2026-02 | WindowProvider, PlatformProvider (clipboard, cursor, dark mode) |
| v0.15.7 | 2026-02 | NVIDIA crash fix, single pipeline alpha, naga v0.11.0 |
| v0.15.6 | 2026-02 | Modal loop rendering (WM_TIMER), smooth drag/resize on Windows |
| v0.15.x | 2026-02 | Render-on-demand, Event System, Fence sync, Texture.BytesPerPixel |
| v0.14.x | 2026-01 | gpucontext.TextureDrawer, gg/ggcanvas integration |
| v0.13.x | 2026-01 | Multi-thread architecture, gputypes integration |
| v0.12.x | 2026-01 | gpucontext integration (DeviceProvider, EventSource) |
| v0.11.x | 2026-01 | Pure Go default, non-blocking GPU acquire |
| v0.10.x | 2026-01 | DeviceProvider interface, compute shaders |
| v0.9.x | 2026-01 | Intel Vulkan compatibility, CI fixes |
| v0.8.x | 2025-12 | macOS ARM64 fixes, Metal backend |
| v0.7.x | 2025-12 | Cross-platform Pure Go backend |
| v0.1-6 | 2025-12 | Core features, Wayland, X11, Cocoa |

> **See [CHANGELOG.md](CHANGELOG.md) for detailed release notes**

---

## Platform Support

| Platform | Windowing | Pure Go Backend | Rust Backend | Status |
|----------|-----------|-----------------|--------------|--------|
| **Windows** | Win32 | Vulkan ✅ | Vulkan ✅ | Production |
| **Linux X11** | X11 | Vulkan ✅ | Vulkan ✅ | Community Testing |
| **Linux Wayland** | Wayland | Vulkan ✅ | Vulkan ✅ | Community Testing |
| **macOS** | Cocoa | Metal ✅ | Metal ✅ | Community Testing |

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
- **Browser embedding** — WebGPU for native only

---

## License

MIT License — see [LICENSE](LICENSE) for details.
