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

## Current State: v0.13.3

✅ **Production-ready** with full feature set:
- Dual backend (Rust/Pure Go)
- Multi-thread architecture (Ebiten/Gio pattern)
- DeviceProvider/EventSource for UI integration
- Cross-platform: Windows (Vulkan/DX12), Linux (Vulkan), macOS (Metal)
- Clean architecture with shared gputypes
- webgpu.h spec-compliant enum values

---

## Upcoming

### v0.14.0 — Integration & Polish
- [ ] GGCanvas integration type
- [ ] RenderTo method for offscreen rendering
- [ ] Adapter.GetInfo() API

### v1.0.0 — Production Release
- [ ] API stability guarantee
- [ ] Semantic versioning commitment
- [ ] Long-term support plan
- [ ] Enterprise deployment guide
- [ ] Comprehensive documentation

---

## Future Ideas

| Theme | Description |
|-------|-------------|
| **gogpu/ui** | GUI toolkit based on gg |
| **WebAssembly** | WASM target for browser |
| **Mobile** | Android/iOS support |
| **Ray Tracing** | RT extensions when available |

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
| **v0.13.x** | 2026-01 | Multi-thread architecture, gputypes integration |
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
