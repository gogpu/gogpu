<p align="center">
  <img src="assets/logo.png" alt="GoGPU Logo" width="180" />
</p>

<h1 align="center">GoGPU</h1>

<p align="center">
  <strong>Pure Go GPU Computing Ecosystem</strong><br>
  GPU power, Go simplicity. Zero CGO.
</p>

<p align="center">
  <a href="https://github.com/gogpu/gogpu/actions/workflows/ci.yml"><img src="https://github.com/gogpu/gogpu/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/gogpu/gogpu"><img src="https://codecov.io/gh/gogpu/gogpu/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://pkg.go.dev/github.com/gogpu/gogpu"><img src="https://pkg.go.dev/badge/github.com/gogpu/gogpu.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/gogpu/gogpu"><img src="https://goreportcard.com/badge/github.com/gogpu/gogpu" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/gogpu/gogpu"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version"></a>
</p>

---

## Status: v0.2.0-alpha — Dual Backend Architecture

> **Triangle rendering works!** Now with switchable backends.
>
> **Star the repo to follow progress!**

---

## Key Feature: Choose Your Backend

GoGPU lets you choose between two WebGPU implementations:

| Backend | Library | Use Case |
|---------|---------|----------|
| **Rust** | wgpu-native via FFI | Maximum performance, production apps |
| **Native Go** | gogpu/wgpu (planned) | Zero dependencies, simple `go build` |

```go
// Auto-select best available (default)
app := gogpu.NewApp(gogpu.DefaultConfig())

// Explicit Rust backend — max performance
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendRust))

// Explicit Native Go backend — zero dependencies
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendGo))
```

**Same API, your choice of backend.**

---

## Quick Start

```go
package main

import (
    "github.com/gogpu/gogpu"
    "github.com/gogpu/gogpu/gmath"
)

func main() {
    app := gogpu.NewApp(gogpu.DefaultConfig().
        WithTitle("Hello GoGPU").
        WithSize(800, 600))

    app.OnDraw(func(ctx *gogpu.Context) {
        ctx.DrawTriangleColor(gmath.DarkGray)
    })

    app.Run()
}
```

**~20 lines vs 480+ lines of raw WebGPU code.**

---

## Why GoGPU?

This project was inspired by [a discussion on r/golang](https://www.reddit.com/r/golang/comments/1pdw9i7/go_deserves_more_support_in_gui_development/) about the state of GUI and graphics development in Go.

**GoGPU provides:**

1. **Simple API** — Hide WebGPU complexity behind intuitive Go code
2. **Dual Backend** — Choose performance (Rust) or simplicity (Pure Go)
3. **Zero CGO** — No C compiler required
4. **Cross-Platform** — Windows, Linux, macOS

| Layer | Component |
|-------|-----------|
| **Application** | Your App / GUI |
| **High-Level** | gogpu/ui (future), gogpu/gg (2D) |
| **Core** | **gogpu/gogpu** — GPU abstraction, windowing, input |
| **Backend** | gpu/backend/rust (wgpu-native) · gpu/backend/native (Pure Go) |
| **Graphics API** | Vulkan · Metal · DX12 · OpenGL |

---

## Installation

```bash
go get github.com/gogpu/gogpu
```

**Requirements:**
- Go 1.25+
- [wgpu-native](https://github.com/gfx-rs/wgpu-native/releases) DLL/dylib/so (for Rust backend)

---

## Package Structure

```
gogpu/
├── gpu/                    # Backend abstraction layer
│   ├── backend.go         # Backend interface
│   ├── types/             # Standalone types (wgpu-types pattern)
│   │   ├── handles.go     # Instance, Device, Texture, etc.
│   │   ├── enums.go       # TextureFormat, PresentMode, etc.
│   │   └── descriptors.go # SurfaceConfig, Color, etc.
│   └── backend/
│       ├── rust/          # Rust backend (wgpu-native)
│       └── native/        # Native Go backend (stub)
├── window/                # Window configuration
├── input/                 # Keyboard, mouse input
├── gmath/                 # Vec2, Vec3, Vec4, Mat4, Color
├── examples/              # Example applications
│   └── triangle/         # Simple triangle demo
└── internal/
    └── platform/          # Platform abstraction (Win32, etc.)
```

---

## Roadmap

### v0.1.0-alpha — First Rendering ✅
- [x] Window creation (Windows)
- [x] Basic rendering (triangle)
- [x] Simple API (~20 lines)

### v0.2.0-alpha — Dual Backend ✅ (Current)
- [x] Backend interface abstraction
- [x] Rust backend wrapper
- [x] Native Go backend stub
- [x] `WithBackend()` configuration

### v0.3.0-alpha — Textures & Sprites
- [ ] Texture loading
- [ ] Sprite rendering
- [ ] Basic shapes

### v0.4.0-alpha — Native Go Backend
- [ ] Pure Go WebGPU implementation
- [ ] Vulkan backend (via purego)

### v1.0.0 — Stable
- [ ] Both backends production-ready
- [ ] Full documentation
- [ ] Performance optimized

---

## Ecosystem

| Project | Description | Status |
|---------|-------------|--------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | Graphics framework (this repo) | v0.2.0-alpha |
| [gogpu/naga](https://github.com/gogpu/naga) | Pure Go shader compiler (WGSL → SPIR-V) | Active |
| [gogpu/gg](https://github.com/gogpu/gg) | Simple 2D graphics library | Planned |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation | Future |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | Zero-CGO WebGPU bindings | Stable |

---

## Announcement

Read our launch announcement on Dev.to:
**[GoGPU: A Pure Go Graphics Library for GPU Programming](https://dev.to/kolkov/gogpu-a-pure-go-graphics-library-for-gpu-programming-2j5d)**

---

## Contributing

Contributions are welcome! This is an early-stage project, so there's lots to do.

**Areas where we need help:**
- Pure Go WebGPU implementation
- Platform support (Linux, macOS)
- WGSL parser implementation
- Documentation and examples

```bash
git clone https://github.com/gogpu/gogpu
cd gogpu
go build ./...
go test ./...
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>GoGPU</strong> — Building the GPU computing ecosystem Go deserves
</p>
