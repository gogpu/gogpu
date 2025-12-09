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

## Status: v0.3.0-alpha — Texture API

> **Texture loading works!** Load from files, Go images, or raw RGBA data.
>
> **Star the repo to follow progress!**

---

## Key Feature: Choose Your Backend

GoGPU lets you choose between two WebGPU implementations:

| Backend | Library | Use Case |
|---------|---------|----------|
| **Rust** | wgpu-native via FFI | Maximum performance, production apps |
| **Native Go** | gogpu/wgpu | Zero dependencies, simple `go build` |

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

## Texture Loading

```go
// Load texture from file (PNG, JPEG)
tex, err := renderer.LoadTexture("sprite.png")
defer tex.Destroy()

// Create from Go image
img := image.NewRGBA(image.Rect(0, 0, 128, 128))
tex, err := renderer.NewTextureFromImage(img)

// Create from raw RGBA data
tex, err := renderer.NewTextureFromRGBA(width, height, rgbaPixels)

// With custom options (pixel art, tiling)
opts := gogpu.TextureOptions{
    MagFilter:    types.FilterModeNearest,  // Crisp pixels
    AddressModeU: types.AddressModeRepeat,  // Tiling
}
tex, err := renderer.LoadTextureWithOptions("tile.png", opts)
```

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
├── app.go                 # Application lifecycle
├── config.go              # Configuration with builder pattern
├── context.go             # Drawing context API
├── renderer.go            # Backend-agnostic rendering
├── texture.go             # Texture loading API
├── shader.go              # Built-in WGSL shaders
├── gpu/                   # Backend abstraction layer
│   ├── backend.go         # Backend interface
│   ├── types/             # Standalone types (wgpu-types pattern)
│   │   ├── handles.go     # Instance, Device, Texture, Sampler, etc.
│   │   ├── enums.go       # TextureFormat, PresentMode, etc.
│   │   └── descriptors.go # SamplerDescriptor, BindGroup, etc.
│   └── backend/
│       ├── rust/          # Rust backend (wgpu-native)
│       └── native/        # Native Go backend (stub)
├── window/                # Window configuration
├── input/                 # Keyboard, mouse input
├── gmath/                 # Vec2, Vec3, Vec4, Mat4, Color
├── examples/              # Example applications
│   ├── triangle/         # Simple triangle demo
│   └── texture/          # Texture API demo
└── internal/
    └── platform/          # Platform abstraction (Win32, etc.)
```

---

## Roadmap

### v0.1.0-alpha — First Rendering ✅
- [x] Window creation (Windows)
- [x] Basic rendering (triangle)
- [x] Simple API (~20 lines)

### v0.2.0-alpha — Dual Backend ✅
- [x] Backend interface abstraction
- [x] Rust backend wrapper
- [x] Native Go backend stub
- [x] `WithBackend()` configuration

### v0.3.0-alpha — Textures ✅ (Current)
- [x] Texture loading from files (PNG, JPEG)
- [x] Texture from Go image.Image
- [x] Texture from raw RGBA data
- [x] Sampler configuration (filtering, addressing)
- [ ] Sprite rendering (next)
- [ ] Basic shapes (next)

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
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | Graphics framework (this repo) | v0.3.0-alpha |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation | v0.1.0-alpha |
| [gogpu/naga](https://github.com/gogpu/naga) | Pure Go shader compiler (WGSL → SPIR-V) | Active |
| [gogpu/gg](https://github.com/gogpu/gg) | Simple 2D graphics library | Planned |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | Zero-CGO WebGPU bindings | Stable |

### wgpu Backends

The Pure Go WebGPU implementation (gogpu/wgpu) now includes:

| Backend | Status | Lines | Features |
|---------|--------|-------|----------|
| OpenGL ES | ✅ Done | ~3.5K | Windows (WGL) |
| **Vulkan** | ✅ Done | ~27K | **HAL complete!** Windows, auto-gen bindings, Vulkan 1.3, memory allocator |
| Metal | Planned | - | macOS/iOS |
| DX12 | Planned | - | Windows |

**Vulkan backend now implements the complete HAL Device interface** including:
- Buffer, Texture, TextureView, Sampler
- ShaderModule, Pipeline, BindGroup
- CommandEncoder, RenderPass, ComputePass
- Fence synchronization, WriteTexture

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
