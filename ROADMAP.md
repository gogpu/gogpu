# GoGPU Roadmap

> **Updated:** December 2025

---

## Vision

**GoGPU** is a Pure Go GPU Computing Ecosystem designed for:
- Professional graphics applications
- IDEs and development tools
- Game engines and simulations
- Cross-platform GUI applications

Our goal is to become the **reference graphics ecosystem** for Go — comparable to the Rust ecosystem (wgpu, naga, vello).

---

## Current State (v0.8.4)

| Component | Version | Description |
|-----------|---------|-------------|
| **gogpu/gogpu** | v0.8.4 | GPU abstraction, windowing, Metal macOS fix |
| **gogpu/wgpu** | v0.8.3 | Pure Go WebGPU (Vulkan, Metal, DX12, GLES, Software) |
| **gogpu/naga** | v0.8.0 | WGSL shader compiler (SPIR-V, MSL, GLSL, HLSL) |
| **gogpu/gg** | v0.15.1 | 2D graphics library (53K+ LOC) |

**Key Features:**
- Zero CGO — Pure Go, easy cross-compilation
- Dual backend — Rust (wgpu-native) or Pure Go
- **Cross-platform Pure Go backend** — Windows (Vulkan/DX12), Linux (Vulkan), macOS (Metal)
- **All 4 shader backends** — SPIR-V, MSL, GLSL, HLSL
- **5 HAL backends** — Vulkan, Metal, DX12, GLES, Software
- WebGPU-first API design

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

## Roadmap

### Q4 2025 (Current) ✅

**Platform Expansion:**
- ✅ Linux Wayland windowing (Pure Go, 5,700 LOC)
- ✅ macOS Cocoa windowing (Pure Go, 950 LOC)
- ✅ Metal backend for macOS (wgpu v0.7.0, ~3K LOC)
- ✅ MSL shader backend (naga v0.5.0, ~3.6K LOC)
- ✅ Linux X11 windowing (Pure Go, ~5K LOC)
- ✅ Cross-platform Pure Go backend integration (v0.7.0)
- ✅ **Metal backend fixed (v0.8.0)** — Present, WGSL→MSL, CreateRenderPipeline
- ✅ **GLSL shader backend (naga v0.6.0, ~2.8K LOC)** — OpenGL 3.3+, ES 3.0+
- ✅ **DX12 backend complete (wgpu v0.8.0, ~12K LOC)** — Pure Go COM via syscall
- ✅ **HLSL shader backend (naga v0.8.0)** — DirectX 11/12
- ✅ **Metal macOS blank window fix (gogpu v0.8.4)** — Issue #24

### Q1 2026

**Performance & Stability:**
- SIMD optimization for 2D rendering (gg)
- Parallel rendering pipeline
- Platform testing and bug fixes

### Q2 2026

**GPU Backends:**
- ✅ ~~DX12 backend for Windows~~ — **Done in v0.8.0!** (~12K LOC)
- GLES improvements for Linux
- Compute shader pipeline

**Shader Compiler:**
- ✅ ~~GLSL output support in naga~~ — **Done in v0.6.0!**
- ✅ ~~HLSL output support in naga~~ — **Done in v0.8.0!**
- Shader optimization passes (dead code elimination, constant folding)
- Source maps for debugging

### Q3 2026

**Ecosystem Maturity:**
- gg v1.0.0 — Production-ready 2D graphics
- GPU-accelerated text rendering
- Scene graph (retained mode)

### 2027+

**Future Vision:**
- gogpu/ui — GUI toolkit
- Full cross-platform support
- Production-ready ecosystem

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Application                         │
├─────────────────────────────────────────────────────────────┤
│     gogpu/gg          │     gogpu/gogpu      │   Custom     │
│   2D Graphics         │    GPU Framework     │    Apps      │
├─────────────────────────────────────────────────────────────┤
│   Rust Backend        │     Pure Go Backend                 │
│  (go-webgpu/webgpu)   │       (gogpu/wgpu)                  │
├─────────────────────────────────────────────────────────────┤
│ Vulkan ✅ │ DX12 ✅ │ Metal ✅ │ OpenGL ES │ Software │
│ Win+Lin   │ Windows │  macOS   │  Win+Lin  │ Headless │
└─────────────────────────────────────────────────────────────┘
```

---

## Contributing

We welcome contributions! Priority areas:
- Linux/macOS platform support
- GPU backend improvements
- Documentation and examples
- Performance optimization

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Links

- [GitHub Organization](https://github.com/gogpu)
- [gogpu/wgpu](https://github.com/gogpu/wgpu) — Pure Go WebGPU
- [gogpu/naga](https://github.com/gogpu/naga) — Shader Compiler
- [gogpu/gg](https://github.com/gogpu/gg) — 2D Graphics

---

*This roadmap is updated as the project evolves.*
