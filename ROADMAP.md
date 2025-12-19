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

## Current State (v0.3.0)

| Component | Version | Description |
|-----------|---------|-------------|
| **gogpu/gogpu** | v0.3.0 | GPU abstraction, windowing, dual backend |
| **gogpu/wgpu** | v0.5.0 | Pure Go WebGPU (Vulkan, GLES, Software) |
| **gogpu/naga** | v0.4.0 | WGSL shader compiler |
| **gogpu/gg** | v0.9.1 | 2D graphics library |

**Key Features:**
- Zero CGO — Pure Go, easy cross-compilation
- Dual backend — Rust (wgpu-native) or Pure Go
- Windows platform fully supported
- WebGPU-first API design

---

## Platform Support

| Platform | Windowing | GPU Backend | Status |
|----------|-----------|-------------|--------|
| **Windows** | Win32 | Vulkan, GLES | Production |
| **Linux X11** | X11 | Vulkan, GLES | Planned |
| **Linux Wayland** | Wayland | Vulkan, GLES | Planned |
| **macOS** | Cocoa | Metal | Planned |

All platforms use Pure Go FFI (no CGO required).

---

## Roadmap

### Q1 2026

**Platform Expansion:**
- Linux X11 windowing support
- macOS Cocoa windowing support
- Linux Wayland windowing support

**Performance:**
- SIMD optimization for 2D rendering (gg)
- Parallel rendering pipeline

### Q2 2026

**GPU Backends:**
- Metal backend for macOS/iOS
- GLES improvements for Linux

**Shader Compiler:**
- GLSL output support in naga
- Shader optimization passes

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
│                    User Application                          │
├─────────────────────────────────────────────────────────────┤
│     gogpu/gg          │     gogpu/gogpu      │   Custom     │
│   2D Graphics         │    GPU Framework     │    Apps      │
├─────────────────────────────────────────────────────────────┤
│   Rust Backend        │     Pure Go Backend                 │
│  (go-webgpu/webgpu)   │       (gogpu/wgpu)                  │
├─────────────────────────────────────────────────────────────┤
│   Vulkan    │   OpenGL ES   │   Software   │    Metal      │
│  (Win+Lin)  │   (Win+Lin)   │  (Headless)  │   (macOS)     │
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
