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
  <a href="https://github.com/gogpu/gogpu/stargazers"><img src="https://img.shields.io/github/stars/gogpu/gogpu?style=flat&labelColor=555&color=yellow" alt="Stars"></a>
  <a href="https://github.com/gogpu/gogpu/discussions"><img src="https://img.shields.io/github/discussions/gogpu/gogpu?style=flat&labelColor=555&color=blue" alt="Discussions"></a>
</p>

---

## Status: v0.8.1 — Metal Backend Complete!

> **Pure Go backend works on ALL platforms!** Windows (Vulkan), Linux (Vulkan), macOS (Metal).
>
> **v0.8.0** — Metal backend fully implemented: Present(), WGSL→MSL compilation, CreateRenderPipeline
>
> **v0.8.1** — Hotfix for macOS zero-dimension window crash (Issue #20)
>
> **🧪 Community Testing Requested** — Help us test on macOS (M1/M2/M3/M4)!
>
> **Star the repo to follow progress!**

---

## Key Feature: Choose Your Backend

GoGPU lets you choose between two WebGPU implementations at **compile time** or **runtime**:

| Backend | Library | Use Case |
|---------|---------|----------|
| **Rust** | wgpu-native via FFI | Maximum performance, production apps |
| **Native Go** | gogpu/wgpu | Zero dependencies, simple `go build` |

### Build Tags (Compile Time)

```bash
# Include both backends (default)
go build ./...

# Only Rust backend (production)
go build -tags rust ./...

# Only Pure Go backend (zero dependencies)
go build -tags purego ./...
```

### Runtime Selection

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

## macOS Platform (v0.5.0+)

**Pure Go Cocoa implementation** — via goffi Objective-C runtime!

```
internal/platform/darwin/
├── init.go          # runtime.LockOSThread() for main thread
├── types.go         # CGFloat, CGPoint, CGRect, NSWindowStyleMask
├── objc.go          # Objective-C runtime via goffi
├── selectors.go     # Cached ObjC selectors
├── application.go   # NSApplication lifecycle
├── window.go        # NSWindow, NSView management
├── surface.go       # CAMetalLayer integration
└── ...              # ~1,000 lines total
```

**Main Thread Requirement:** macOS Cocoa requires all UI operations on the main thread.
GoGPU automatically handles this with `runtime.LockOSThread()` — no action needed from users.

**🧪 Community Testing Requested:**
- macOS 12+ (Monterey and later) on Apple Silicon (M1/M2/M3/M4)
- Run `CGO_ENABLED=0 go build -tags purego ./examples/triangle/` on macOS
- Report issues at [github.com/gogpu/gogpu/issues](https://github.com/gogpu/gogpu/issues)

---

## Why GoGPU?

This project was inspired by [a discussion on r/golang](https://www.reddit.com/r/golang/comments/1pdw9i7/go_deserves_more_support_in_gui_development/) about the state of GUI and graphics development in Go.

**GoGPU provides:**

1. **Simple API** — Hide WebGPU complexity behind intuitive Go code
2. **Dual Backend** — Choose performance (Rust) or simplicity (Pure Go)
3. **Zero CGO** — No C compiler required
4. **Cross-Platform** — Windows, Linux (Wayland), macOS (Cocoa)

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
│   ├── registry.go        # Backend registration (auto-discovery)
│   ├── types/             # Standalone types (wgpu-types pattern)
│   │   ├── handles.go     # Instance, Device, Texture, Sampler, etc.
│   │   ├── enums.go       # TextureFormat, PresentMode, etc.
│   │   └── descriptors.go # SamplerDescriptor, BindGroup, etc.
│   └── backend/
│       ├── rust/          # Rust backend (wgpu-native)
│       │   └── init.go    # Auto-registration (build tags)
│       └── native/        # Native Go backend (Vulkan via gogpu/wgpu)
│           └── init.go    # Auto-registration (build tags)
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

See **[ROADMAP.md](ROADMAP.md)** for the full roadmap.

**Current:** v0.7.2 — macOS ARM64 Fix

**Recent:**
- ✅ **macOS ARM64 main thread fix** (v0.7.2)
- ✅ **Pure Go backend for ALL platforms** (Windows/Linux Vulkan, macOS Metal)
- ✅ Linux X11 windowing (Pure Go, ~5K LOC)
- ✅ macOS Cocoa windowing (Pure Go, ~1K LOC)
- ✅ Linux Wayland windowing (Pure Go, 5,700 LOC)
- ✅ Metal backend for macOS (wgpu v0.6.0)

**Next:**
- DX12 backend for Windows
- GUI toolkit (gogpu/ui)

---

## Ecosystem

| Project | Description | Purpose |
|---------|-------------|---------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | Graphics framework (this repo) | GPU abstraction, windowing, input |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU | Vulkan, Metal, GLES, Software backends |
| [gogpu/naga](https://github.com/gogpu/naga) | Shader compiler | WGSL → SPIR-V, MSL, GLSL |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics | Canvas API, scene graph, GPU text |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit | Widgets, layouts, themes (planned) |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | FFI bindings | wgpu-native integration |

> **Note:** Always use the latest versions. Check each repository for current releases.

### wgpu Backends

The Pure Go WebGPU implementation (gogpu/wgpu) now includes:

| Backend | Status | Lines | Features |
|---------|--------|-------|----------|
| **Software** | ✅ Done | ~10K | **Full rasterizer!** Triangle rendering, depth/stencil, blending, clipping, parallel |
| OpenGL ES | ✅ Done | ~7.5K | Windows (WGL) + Linux (EGL) |
| **Vulkan** | ✅ Done | ~27K | **Cross-platform!** Windows/Linux/macOS, goffi FFI, Vulkan 1.3, memory allocator |
| **Metal** | ✅ Done | ~3K | **macOS/iOS!** Pure Go via goffi Objective-C bridge |
| DX12 | Planned | - | Windows |

**Software backend** enables headless rendering:
```bash
go build -tags software ./...  # CPU-only, no GPU needed
```

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

**Join the discussion:** [GitHub Discussions](https://github.com/gogpu/gogpu/discussions) — Share ideas, ask questions, help shape the **gogpu/ui** GUI toolkit!

**Areas where we need help:**
- 🧪 **macOS testing** — Test on real macOS systems (Monterey+)
- 🧪 **Linux X11 testing** — Test on X11 systems (Ubuntu, Fedora, etc.)
- 🧪 **Linux Wayland testing** — Test on Wayland compositors
- DX12 backend for Windows
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
