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
| **Graphics API** | Runtime selection: Vulkan, DX12, Metal, GLES, Software |
| **Platforms** | Windows (Vulkan/DX12/GLES), Linux (Vulkan/GLES), macOS (Metal) |
| **Graphics** | Windowing, input handling, texture loading, zero-copy surface rendering |
| **Compute** | Full compute shader support |
| **Integration** | DeviceProvider, HalProvider, WindowProvider, PlatformProvider, SurfaceView |
| **Logging** | Structured logging via `log/slog`, silent by default |
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

# Enable Rust backend (requires wgpu-gpu DLL, Windows only)
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

### Graphics API Selection

Backend (Rust/Native) and Graphics API (Vulkan/DX12/Metal/GLES) are independent choices:

```go
// Force Vulkan on Windows (instead of auto-detected default)
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithGraphicsAPI(gogpu.GraphicsAPIVulkan))

// Force DirectX 12 on Windows
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithGraphicsAPI(gogpu.GraphicsAPIDX12))

// Force GLES (useful for testing or compatibility)
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithGraphicsAPI(gogpu.GraphicsAPIGLES))
```

| Graphics API | Platforms | Constant |
|--------------|-----------|----------|
| **Auto** | All (default) | `gogpu.GraphicsAPIAuto` |
| **Vulkan** | Windows, Linux | `gogpu.GraphicsAPIVulkan` |
| **DX12** | Windows | `gogpu.GraphicsAPIDX12` |
| **Metal** | macOS | `gogpu.GraphicsAPIMetal` |
| **GLES** | Windows, Linux | `gogpu.GraphicsAPIGLES` |
| **Software** | All | `gogpu.GraphicsAPISoftware` |

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
    MagFilter:    gputypes.FilterModeNearest,  // Crisp pixels
    AddressModeU: gputypes.AddressModeRepeat,  // Tiling
}
tex, err := renderer.LoadTextureWithOptions("tile.png", opts)
```

---

## DeviceProvider Interface

GoGPU exposes GPU resources through the `DeviceProvider` interface for integration with external libraries:

```go
type DeviceProvider interface {
    Device() hal.Device              // HAL GPU device (type-safe Go interface)
    Queue() hal.Queue                // HAL command queue
    SurfaceFormat() gputypes.TextureFormat
}

// Usage
provider := app.DeviceProvider()
device := provider.Device()   // hal.Device — 30+ methods with error returns
queue := provider.Queue()     // hal.Queue — Submit, WriteBuffer, ReadBuffer
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

### HalProvider (Direct GPU Access)

For GPU accelerators that need low-level HAL access (compute shaders, buffer readback):

```go
import "github.com/gogpu/gpucontext"

provider := app.GPUContextProvider()

// Type-assert to HalProvider for direct HAL access
if hp, ok := provider.(gpucontext.HalProvider); ok {
    halDevice := hp.HalDevice() // hal.Device for compute pipelines
    halQueue := hp.HalQueue()   // hal.Queue for command submission
}
```

Used by [gogpu/gg](https://github.com/gogpu/gg) GPU SDF accelerator for compute shader dispatch on shared device.

### SurfaceView (Zero-Copy Rendering)

For direct GPU rendering without CPU readback:

```go
app.OnDraw(func(dc *gogpu.Context) {
    view := dc.SurfaceView() // Current frame's GPU texture view
    // Pass to ggcanvas.RenderDirect() for zero-copy compositing
})
```

This eliminates the GPU→CPU→GPU round-trip when integrating with gg/ggcanvas.

### Window & Platform Integration

`App` implements `gpucontext.WindowProvider` and `gpucontext.PlatformProvider` for UI frameworks:

```go
// Window geometry and DPI
w, h := app.Size()              // physical pixels
scale := app.ScaleFactor()      // 1.0 = standard, 2.0 = Retina/HiDPI

// Clipboard
text, _ := app.ClipboardRead()
app.ClipboardWrite("copied text")

// Cursor management
app.SetCursor(gpucontext.CursorPointer)  // hand cursor
app.SetCursor(gpucontext.CursorText)     // I-beam for text input

// System preferences
if app.DarkMode() { /* switch to dark theme */ }
if app.ReduceMotion() { /* disable animations */ }
if app.HighContrast() { /* increase contrast */ }
fontMul := app.FontScale() // user's font size preference
```

### Ebiten-Style Input Polling

For game loops, use the polling-based Input API:

```go
import "github.com/gogpu/gogpu/input"

app.OnUpdate(func(dt float64) {
    inp := app.Input()

    // Keyboard
    if inp.Keyboard().JustPressed(input.KeySpace) {
        player.Jump()
    }
    if inp.Keyboard().Pressed(input.KeyLeft) {
        player.MoveLeft(dt)
    }

    // Mouse
    x, y := inp.Mouse().Position()
    if inp.Mouse().JustPressed(input.MouseButtonLeft) {
        player.Shoot(x, y)
    }
})
```

All input methods are thread-safe and work with the frame-based update loop.

### Resource Cleanup

Use `OnClose` to release GPU resources before the renderer is destroyed:

```go
app.OnClose(func() {
    if canvas != nil {
        _ = canvas.Close()
        canvas = nil
    }
})

if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

`OnClose` runs on the render thread before `Renderer.Destroy()`, ensuring textures, bind groups, and pipelines are released while the device is still alive.

---

## Compute Shaders

Full compute shader support via HAL interfaces:

```go
// Create compute pipeline via HAL device
pipeline, _ := device.CreateComputePipeline(&hal.ComputePipelineDescriptor{
    Layout:     pipelineLayout,
    Module:     shaderModule,
    EntryPoint: "main",
})

// Create storage buffers
inputBuffer, _ := device.CreateBuffer(&hal.BufferDescriptor{
    Size:  dataSize,
    Usage: gputypes.BufferUsageStorage | gputypes.BufferUsageCopyDst,
})

// Dispatch compute work via command encoder
encoder, _ := device.CreateCommandEncoder()
encoder.BeginEncoding("compute")
pass := encoder.BeginComputePass(&hal.ComputePassDescriptor{})
pass.SetPipeline(pipeline)
pass.SetBindGroup(0, bindGroup, nil)
pass.Dispatch(workgroupsX, 1, 1)
pass.End()
cmdBuf := encoder.EndEncoding()
queue.Submit([]hal.CommandBuffer{cmdBuf}, nil, 0)
```

---

## Logging

GoGPU uses `log/slog` for structured logging, silent by default:

```go
import "log/slog"

// Enable info-level logging
gogpu.SetLogger(slog.Default())

// Enable debug-level logging for full diagnostics
gogpu.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})))

// Get current logger
logger := gogpu.Logger()
```

Log levels: `Debug` (texture creation, pipeline state), `Info` (backend selected, adapter info), `Warn` (resource cleanup errors).

---

## Architecture

GoGPU uses **multi-thread architecture** (Ebiten/Gio pattern) for professional responsiveness:
- **Main thread:** Window events only (Win32/Cocoa/X11 message pump)
- **Render thread:** All GPU operations (device, swapchain, commands)

This ensures windows never show "Not Responding" during heavy GPU operations.

```
User Application
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│                      gogpu.App                          │
│    Multi-Thread: Events (main) + Render (dedicated)     │
└─────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│                    gogpu.Renderer                       │
│  Uses hal.Device / hal.Queue directly (Go interfaces)   │
└─────────────────────────────────────────────────────────┘
       │
       ├─────────────────┐
       ▼                 ▼
┌─────────────┐  ┌─────────────┐
│  gogpu/wgpu │  │  Platform   │
│ (Pure Go    │  │  Windowing  │
│  WebGPU)    │  │ Win32/Cocoa │
└──────┬──────┘  └─────────────┘
       │
 ┌─────┴─────┬─────┬─────┬─────────┐
 ▼           ▼     ▼     ▼         ▼
Vulkan     DX12  Metal  GLES   Software
```

### Package Structure

| Package | Purpose |
|---------|---------|
| `gogpu` (root) | App, Config, Context, Renderer, Texture |
| `gpu/` | Backend selection (HAL-based) |
| `gpu/types/` | BackendType, GraphicsAPI enums |
| `gpu/backend/rust/` | Rust backend via wgpu-native FFI (opt-in, `-tags rust`) |
| `gpu/backend/native/` | HAL backend creation (Vulkan/Metal selection) |
| `gmath/` | Vec2, Vec3, Vec4, Mat4, Color |
| `window/` | Window configuration |
| `input/` | Keyboard and mouse input |
| `internal/platform/` | Platform-specific windowing |
| `internal/thread/` | Multi-thread rendering (RenderLoop) |

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
| [gogpu/gpucontext](https://github.com/gogpu/gpucontext) | Shared interfaces (DeviceProvider, WindowProvider, PlatformProvider, EventSource) |
| [gogpu/gputypes](https://github.com/gogpu/gputypes) | Shared WebGPU types (TextureFormat, BufferUsage, Limits) |
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
