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
| **Platforms** | Windows (Vulkan/DX12/GLES), Linux X11/Wayland (Vulkan/GLES), macOS (Metal) |
| **Rendering** | Event-driven three-state model (idle/animating/continuous), zero-copy surface rendering |
| **Graphics** | Windowing, input handling, texture loading, frameless windows |
| **Compute** | Full compute shader support |
| **Window Chrome** | Frameless windows with custom title bars, DWM shadow, hit-test regions |
| **HiDPI** | Per-monitor DPI, WM_DPICHANGED, logical/physical coordinate split |
| **Integration** | DeviceProvider, WindowProvider, PlatformProvider, WindowChrome, SurfaceView |
| **Logging** | Structured logging via `log/slog`, silent by default |
| **Build** | Zero CGO with Pure Go backend |

---

## Installation

```bash
go get github.com/gogpu/gogpu
```

**Requirements:**
- Go 1.25+
- `CGO_ENABLED=0` (Pure Go FFI requires CGO disabled)

**Zero dependencies — just works:**
```bash
CGO_ENABLED=0 go run .
```

> **Note:** On macOS and some Linux distros, CGO is enabled by default. Always set `CGO_ENABLED=0` when building GoGPU projects.

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

# Enable Rust backend (requires wgpu-native shared library)
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
| **Rust** | `-tags rust` | wgpu-native via FFI | Maximum performance (all platforms) |

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

// Software backend — no GPU required, always available
// Windows: renders to screen via GDI. Linux/macOS: headless.
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithGraphicsAPI(gogpu.GraphicsAPISoftware))
```

| Graphics API | Platforms | Constant |
|--------------|-----------|----------|
| **Auto** | All (default) | `gogpu.GraphicsAPIAuto` |
| **Vulkan** | Windows, Linux | `gogpu.GraphicsAPIVulkan` |
| **DX12** | Windows | `gogpu.GraphicsAPIDX12` |
| **Metal** | macOS | `gogpu.GraphicsAPIMetal` |
| **GLES** | Windows, Linux | `gogpu.GraphicsAPIGLES` |
| **Software** | All (no GPU needed) | `gogpu.GraphicsAPISoftware` |

---

## Resource Management

GPU resources are automatically cleaned up on shutdown when registered with `TrackResource`:

```go
canvas, _ := ggcanvas.New(provider, 800, 600)
app.TrackResource(canvas) // auto-closed on shutdown, no OnClose needed
```

Resources are closed in LIFO (reverse) order after GPU idle, before device destruction. The shutdown sequence is: `WaitIdle → tracked resources → OnClose → Renderer.Destroy()`.

**ggcanvas auto-registration:** When created via a provider that implements `ResourceTracker` (like `App`), ggcanvas auto-registers — no `TrackResource` call needed.

**GC safety net:** Textures use `runtime.AddCleanup` as a fallback — if you forget `Destroy()`, the GC will eventually clean up GPU resources. This is a safety net, not a replacement for explicit cleanup.

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

### DeviceProvider (GPU Access)

For GPU compute and custom rendering, access the wgpu device directly:

```go
provider := app.DeviceProvider()
device := provider.Device()   // *wgpu.Device — full WebGPU API
queue := device.Queue()       // *wgpu.Queue — command submission
```

Used for compute shaders, custom render pipelines, and by [gogpu/gg](https://github.com/gogpu/gg) GPU accelerator.

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
w, h := app.Size()              // logical points (DIP)
fw, fh := app.PhysicalSize()    // physical pixels (framebuffer)
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

### Event-Driven Rendering

GoGPU uses a three-state rendering model for optimal power efficiency:

| State | Condition | CPU Usage | Latency |
|-------|-----------|-----------|---------|
| **Idle** | No activity | 0% (blocks on OS events) | <1ms wakeup |
| **Animating** | Active animation tokens | VSync (~60fps) | Smooth |
| **Continuous** | `ContinuousRender=true` | 100% (game loop) | Immediate |

```go
// Event-driven mode (default for UI apps)
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithContinuousRender(false))

// Start animation — renders at VSync while token is alive
token := app.StartAnimation()
// ... animation runs at 60fps ...
token.Stop() // Loop returns to idle (0% CPU)

// Request single-frame redraw from any goroutine
app.RequestRedraw()
```

Multiple animation tokens can be active simultaneously. The loop renders continuously until all tokens are stopped.

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

Full compute shader support via wgpu public API:

```go
device := app.DeviceProvider().Device()

// WGSL compute shader
shader, _ := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
    WGSL: `
        @group(0) @binding(0) var<storage, read> input: array<f32>;
        @group(0) @binding(1) var<storage, read_write> output: array<f32>;

        @compute @workgroup_size(64)
        fn main(@builtin(global_invocation_id) id: vec3<u32>) {
            output[id.x] = input[id.x] * 2.0;
        }
    `,
})

// Create storage buffers
inputBuf, _ := device.CreateBuffer(&wgpu.BufferDescriptor{
    Size:  dataSize,
    Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
})

// Create pipeline and dispatch
pipeline, _ := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
    Layout: pipelineLayout, Module: shader, EntryPoint: "main",
})

encoder, _ := device.CreateCommandEncoder(nil)
pass, _ := encoder.BeginComputePass(nil)
pass.SetPipeline(pipeline)
pass.SetBindGroup(0, bindGroup, nil)
pass.Dispatch(workgroups, 1, 1)
pass.End()
cmds, _ := encoder.Finish()
_, _ = device.Queue().Submit(cmds)
device.WaitIdle()  // wait for GPU before readback

// Read results back to CPU
result := make([]byte, dataSize)
device.Queue().ReadBuffer(stagingBuf, 0, result)
```

See [`examples/particles`](examples/particles) for a full GPU particle simulation
(compute + render in one window) and
[`wgpu/examples/compute-particles`](https://github.com/gogpu/wgpu/tree/main/examples/compute-particles)
for a headless compute example.

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

Native Win32 windowing with Vulkan, DirectX 12, GLES, and Software backends.

### Linux

X11 and Wayland support with Vulkan, GLES, and Software (headless) backends.

- **X11** — pure Go X11 protocol with libX11 loaded via goffi for Vulkan surface creation. Multi-touch input via XInput2 wire protocol.
- **Wayland** — single libwayland-client connection via goffi for all Wayland operations (surface, input, xdg-shell, CSD). Server-side decorations via `zxdg_decoration_manager_v1`, client-side decorations (CSD) with subsurface title bar when SSD unavailable. Tested on WSLg, GNOME, KDE, sway, niri, COSMIC.

### macOS

Pure Go Cocoa implementation via goffi Objective-C runtime, with Metal and Software (headless) backends:

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

| Contributor | Contributions |
|-------------|---------------|
| [@ppoage](https://github.com/ppoage) | macOS ARM64 (Apple Silicon) support — 3 merged PRs across gogpu, wgpu, and naga with ~3,500 lines of code. Made Metal backend work on M1/M4 |
| [@JanGordon](https://github.com/JanGordon) | Documentation fix (wgpu) |

### Community Champions

| Champion | Contributions |
|----------|---------------|
| [@darkliquid](https://github.com/darkliquid) · Andrew Montgomery | Linux platform hero — 3 bug reports, 13+ comments with detailed stack traces and diagnostics. His persistence uncovered the critical [goffi stack spill bug](https://github.com/go-webgpu/goffi/issues/19) affecting all Linux/macOS users |
| [@i2534](https://github.com/i2534) | Most prolific gg tester — 7 bug reports covering alpha blending, patterns, transforms, and line joins. Shaped the quality of the 2D renderer |
| [@qq1792569310](https://github.com/qq1792569310) · luomo | Early stress-tester — 3 issues and 9 comments. Found memory leak and event system bugs that improved framework stability |
| [@rcarlier](https://github.com/rcarlier) · Richard Carlier | Cross-platform tester — 4 issues across gg and ui. Active tester of text rendering, image handling, and UI on macOS Apple Silicon (M3) |
| [@amortaza](https://github.com/amortaza) · Afshin Mortazavi-Nia | Architecture contributor — deep multi-week engagement in gg+gogpu integration discussions. Author of [go-bellina](https://github.com/amortaza/go-bellina) UI library |
| [@cyberbeast](https://github.com/cyberbeast) · Sandesh Gade | macOS Tahoe debugger — thorough Metal backend debugging on Apple M2 Max with detailed diagnostics |
| [@crsolver](https://github.com/crsolver) | UI architecture advisor — significant input on the UI toolkit RFC with 8+ discussion comments |
| [@neurlang](https://github.com/neurlang) | Wayland expert — author of [neurlang/wayland](https://github.com/neurlang/wayland), provided expert consultation on Wayland protocol issues |

### Early Adopters

These developers tested GoGPU on Day 1 — when nothing worked and every platform was broken. Their bug reports shaped the project:

- [@Nickrocky](https://github.com/Nickrocky) — First macOS tester (Dec 25, 2025). The very first external user to try GoGPU
- [@facemcgee](https://github.com/facemcgee) — Early Linux tester (Dec 29, 2025)
- [@soypat](https://github.com/soypat) — Early naga interest, [gsdf](https://github.com/soypat/gsdf) integration exploration
- [@jan53n](https://github.com/jan53n) — Linux X11 testing
- [@davidmichaelkarr](https://github.com/davidmichaelkarr) — Windows 11 testing
- [@martinarisk](https://github.com/martinarisk) — Wayland testing, report that led to major protocol fixes
- [@adamsanclemente](https://github.com/adamsanclemente) — Found transform rendering bug in gg
- [@beikege](https://github.com/beikege) — Touch input advocacy, UI toolkit feedback
- [@joeblew999](https://github.com/joeblew999) — WASM/browser platform interest

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>GoGPU</strong> — Building the GPU computing ecosystem Go deserves
</p>
