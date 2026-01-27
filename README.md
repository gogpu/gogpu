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
  <a href="https://github.com/gogpu/gogpu/releases"><img src="https://img.shields.io/github/v/release/gogpu/gogpu" alt="Latest Release"></a>
  <a href="https://github.com/gogpu/gogpu"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://github.com/gogpu/gogpu/stargazers"><img src="https://img.shields.io/github/stars/gogpu/gogpu?style=flat&labelColor=555&color=yellow" alt="Stars"></a>
</p>

---

## Overview

**GoGPU** is a GPU computing framework for Go that provides a high-level API for graphics and compute operations. It supports dual backends: a high-performance Rust backend (wgpu-native) and a pure Go backend for zero-dependency builds.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Backends** | Rust (wgpu-native) or Pure Go (gogpu/wgpu) |
| **Platforms** | Windows (Vulkan/DX12), Linux (Vulkan), macOS (Metal) |
| **Graphics** | Windowing, input handling, texture loading |
| **Compute** | Full compute shader support |
| **Integration** | DeviceProvider for external libraries |
| **Build** | Zero CGO with Pure Go backend |

---

## Installation

```bash
go get github.com/gogpu/gogpu
```

**Requirements:**
- Go 1.25+

**Zero dependencies — just works:**
```bash
go run .
```

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

    app.OnDraw(func(dc *gogpu.Context) {
        dc.DrawTriangleColor(gmath.DarkGray)
    })

    app.Run()
}
```

**Result:** A window with a rendered triangle in approximately 20 lines of code, compared to 480+ lines of raw WebGPU.

---

## Backend Selection

GoGPU supports two WebGPU implementations, selectable at compile time or runtime.

### Build Tags

```bash
# Pure Go backend (default, zero dependencies)
go build ./...

# Enable Rust backend (requires wgpu-native DLL, Windows only)
go build -tags rust ./...
```

### Runtime Selection

```go
// Auto-select best available (default)
app := gogpu.NewApp(gogpu.DefaultConfig())

// Explicit Rust backend
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendRust))

// Explicit Pure Go backend
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendGo))
```

| Backend | Build Tag | Library | Use Case |
|---------|-----------|---------|----------|
| **Native Go** | (default) | gogpu/wgpu | Zero dependencies, simple deployment |
| **Rust** | `-tags rust` | wgpu-native via FFI | Maximum performance (Windows only) |

> **Note:** Rust backend requires [wgpu-native](https://github.com/gfx-rs/wgpu-native/releases) DLL.

---

## Texture Loading

```go
// Load from file (PNG, JPEG)
tex, err := renderer.LoadTexture("sprite.png")
defer tex.Destroy()

// Create from Go image
img := image.NewRGBA(image.Rect(0, 0, 128, 128))
tex, err := renderer.NewTextureFromImage(img)

// With custom filtering options
opts := gogpu.TextureOptions{
    MagFilter:    types.FilterModeNearest,  // Crisp pixels
    AddressModeU: types.AddressModeRepeat,  // Tiling
}
tex, err := renderer.LoadTextureWithOptions("tile.png", opts)
```

---

## DeviceProvider Interface

GoGPU exposes GPU resources through the `DeviceProvider` interface for integration with external libraries:

```go
type DeviceProvider interface {
    Backend() gpu.Backend        // GPU backend (rust or native)
    Device() types.Device        // GPU device handle
    Queue() types.Queue          // Command queue
    SurfaceFormat() types.TextureFormat
}

// Usage
provider := app.DeviceProvider()
device := provider.Device()
queue := provider.Queue()
```

### Cross-Package Integration (gpucontext)

For integration with external libraries like [gogpu/gg](https://github.com/gogpu/gg), use the standard [gpucontext](https://github.com/gogpu/gpucontext) interfaces:

```go
import "github.com/gogpu/gpucontext"

// Get gpucontext.DeviceProvider for external libraries
provider := app.GPUContextProvider()
device := provider.Device()   // gpucontext.Device interface
queue := provider.Queue()     // gpucontext.Queue interface
format := provider.SurfaceFormat() // gpucontext.TextureFormat

// Get gpucontext.EventSource for UI frameworks
events := app.EventSource()
events.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
    // Handle keyboard input
})
events.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
    // Handle mouse click
})
```

This enables enterprise-grade dependency injection between packages without circular imports.

---

## Compute Shaders

Full compute shader support in both backends:

```go
// Create compute pipeline via backend
pipeline, _ := backend.CreateComputePipeline(device, &types.ComputePipelineDescriptor{
    Layout: pipelineLayout,
    Compute: types.ProgrammableStageDescriptor{
        Module:     shaderModule,
        EntryPoint: "main",
    },
})

// Create storage buffers
inputBuffer, _ := backend.CreateBuffer(device, &types.BufferDescriptor{
    Size:  dataSize,
    Usage: types.BufferUsageStorage | types.BufferUsageCopyDst,
})

outputBuffer, _ := backend.CreateBuffer(device, &types.BufferDescriptor{
    Size:  dataSize,
    Usage: types.BufferUsageStorage | types.BufferUsageCopySrc,
})

// Dispatch compute work
computePass.SetPipeline(pipeline)
computePass.SetBindGroup(0, bindGroup, nil)
computePass.Dispatch(workgroupsX, 1, 1)
```

---

## Architecture

```
User Application
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│                      gogpu.App                          │
│    Config, Lifecycle, Event Loop                        │
└─────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│                    gogpu.Renderer                       │
│    Surface, Device, Queue, Frame Management             │
└─────────────────────────────────────────────────────────┘
       │
       ├─────────────────┬─────────────────┐
       ▼                 ▼                 ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Rust      │  │  Native Go  │  │  Platform   │
│   Backend   │  │   Backend   │  │  Windowing  │
│ (wgpu-native)│ │ (gogpu/wgpu)│  │ Win32/Cocoa │
└─────────────┘  └─────────────┘  └─────────────┘
```

### Package Structure

| Package | Purpose |
|---------|---------|
| `gogpu` (root) | App, Config, Context, Renderer, Texture |
| `gpu/` | Backend interface, registry, auto-selection |
| `gpu/types/` | WebGPU type definitions |
| `gpu/backend/rust/` | Rust backend via wgpu-native FFI |
| `gpu/backend/native/` | Pure Go backend via gogpu/wgpu |
| `gmath/` | Vec2, Vec3, Vec4, Mat4, Color |
| `window/` | Window configuration |
| `input/` | Keyboard and mouse input |
| `internal/platform/` | Platform-specific windowing |

---

## Platform Support

### Windows

Native Win32 windowing with Vulkan and DirectX 12 backends.

### Linux

X11 and Wayland support with Vulkan backend.

### macOS

Pure Go Cocoa implementation via goffi Objective-C runtime:

```
internal/platform/darwin/
├── application.go   # NSApplication lifecycle
├── window.go        # NSWindow, NSView management
├── surface.go       # CAMetalLayer integration
└── objc.go          # Objective-C runtime via goffi
```

**Note:** macOS Cocoa requires UI operations on the main thread. GoGPU handles this automatically.

---

## Ecosystem

| Project | Description |
|---------|-------------|
| **gogpu/gogpu** | **GPU framework (this repo)** |
| [gogpu/gpucontext](https://github.com/gogpu/gpucontext) | Shared interfaces (DeviceProvider, EventSource) |
| [gogpu/gputypes](https://github.com/gogpu/gputypes) | Shared WebGPU types (TextureFormat, BufferUsage, Limits) — *planned* |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation |
| [gogpu/naga](https://github.com/gogpu/naga) | Shader compiler (WGSL to SPIR-V, MSL, GLSL) |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics library |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit (planned) |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | wgpu-native FFI bindings |
| [go-webgpu/goffi](https://github.com/go-webgpu/goffi) | Pure Go FFI library |

---

## Documentation

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** — System architecture
- **[ROADMAP.md](ROADMAP.md)** — Development milestones
- **[CHANGELOG.md](CHANGELOG.md)** — Release notes
- **[pkg.go.dev](https://pkg.go.dev/github.com/gogpu/gogpu)** — API reference

### Articles

- [GoGPU: From Idea to 100K Lines in Two Weeks](https://dev.to/kolkov/gogpu-from-idea-to-100k-lines-in-two-weeks-building-gos-gpu-ecosystem-3b2)
- [GoGPU Announcement](https://dev.to/kolkov/gogpu-a-pure-go-graphics-library-for-gpu-programming-2j5d)

---

## Contributing

Contributions welcome! See [GitHub Discussions](https://github.com/gogpu/gogpu/discussions) to share ideas and ask questions.

**Priority areas:**
- Platform testing (macOS, Linux X11/Wayland, Windows DX12)
- Documentation and examples
- Performance benchmarks
- Bug reports

```bash
git clone https://github.com/gogpu/gogpu
cd gogpu
go build ./...
go test ./...
```

---

## Acknowledgments

**Professor Ancha Baranova** — This project would not have been possible without her invaluable help and support.

### Inspiration

- [u/m-unknown-2025](https://www.reddit.com/user/m-unknown-2025/) — The [Reddit post](https://www.reddit.com/r/golang/comments/1pdw9i7/go_deserves_more_support_in_gui_development/) that started it all
- [born-ml/born](https://github.com/born-ml/born) — ML framework where go-webgpu bindings originated

### Contributors

- [@ppoage](https://github.com/ppoage) — macOS ARM64 testing (M1/M4)
- [@Nickrocky](https://github.com/Nickrocky) — macOS testing and feedback

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>GoGPU</strong> — Building the GPU computing ecosystem Go deserves
</p>
