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
  <a href="https://opencollective.com/gogpu"><img src="https://opencollective.com/gogpu/all/badge.svg?label=financial+contributors" alt="Financial Contributors on Open Collective"></a>
</p>

---

## Overview

**gogpu** is the application framework of the [GoGPU ecosystem](https://github.com/gogpu) — windowing, input, lifecycle management, and platform abstraction for GPU-accelerated applications. Part of a 1.1M+ LOC Pure Go ecosystem across 15 repositories.

Built on [gogpu/wgpu](https://github.com/gogpu/wgpu) — the unified Go WebGPU package with three backends: Pure Go (default), Rust FFI (`-tags rust`), and Browser WASM.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Backends** | Pure Go (default), Rust FFI (`-tags rust`), Browser WASM — via [gogpu/wgpu](https://github.com/gogpu/wgpu) |
| **Graphics API** | Runtime selection: Vulkan, DX12, Metal, GLES, Software |
| **Platforms** | Windows (Vulkan/DX12/GLES), Linux X11/Wayland (Vulkan/GLES), macOS (Metal), Browser/WASM (WebGPU) |
| **Rendering** | Event-driven three-state model (idle/animating/continuous), zero-copy surface rendering, damage-aware presentation |
| **Graphics** | Windowing, input handling, multi-keyboard layout (X11 XKB + Wayland xkbcommon), AltGr/international text input (unified xkbcommon, ADR-029), key repeat on all platforms (Wayland client-side timer, ADR-033), texture loading, frameless windows, mouse grab / pointer lock (Win32 + X11 + Wayland, SDL parity), GPU adapter power preference, native macOS window tabbing, native system menus (macOS + Windows), native file dialogs (macOS + Windows + Linux D-Bus/zenity/kdialog) |
| **Scroll** | ScrollPhase + IsMomentum for macOS trackpad momentum detection (ADR-032), pixel/line/page delta modes |
| **Sound** | Platform system sounds for UI feedback (winmm, NSSound, canberra/PulseAudio) |
| **Compute** | Full compute shader support |
| **Window Chrome** | Frameless windows with custom title bars, DWM shadow, hit-test regions |
| **HiDPI** | Per-monitor DPI, WM_DPICHANGED, logical/physical coordinate split, WithSize in logical DIP (ADR-030) |
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

## Backend Selection (ADR-038)

Backend selection is handled inside [gogpu/wgpu](https://github.com/gogpu/wgpu) via build tags. gogpu uses `wgpu.CreateInstance()` — the build tag determines which implementation runs.

```bash
# Pure Go (default, zero dependencies)
go build ./...

# Rust FFI (requires wgpu-native v29 shared library)
go build -tags rust ./...

# Download wgpu-native automatically:
go run github.com/go-webgpu/webgpu/cmd/setup@latest
```

| Backend | Build Tag | Use Case |
|---------|-----------|----------|
| **Pure Go** | (default) | Zero deps, cross-compile, air-gapped deploy |
| **Rust FFI** | `-tags rust` | Battle-tested GPU drivers, max compatibility |
| **Browser** | `GOOS=js GOARCH=wasm` | Web applications |

> **Note:** Rust backend requires [wgpu-native](https://github.com/gfx-rs/wgpu-native/releases/tag/v29.0.0.0). See [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) for setup.

### Graphics API Selection

Graphics API (Vulkan/DX12/Metal/GLES) is selected independently from the backend:

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
// Renders to screen (GDI, X11, Wayland SHM, macOS) or headless (CI/testing)
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

### Environment Variables

All settings can be overridden via environment variables (no code changes needed):

| Variable | Values | Default | Purpose |
|----------|--------|---------|---------|
| `GOGPU_GRAPHICS_API` | `vulkan`, `dx12`, `metal`, `gles`, `software` | auto | GPU backend selection |
| `GOGPU_POWER_PREFERENCE` | `low`, `high` | none | GPU power/performance trade-off |
| `GOGPU_RENDER_MODE` | `auto`, `cpu`, `gpu` | auto | 2D rendering path (ADR-020) |
| `GOGPU_DEBUG_DAMAGE` | `1` | off | Show damage region overlay (ADR-021) |

```bash
# Examples:
GOGPU_GRAPHICS_API=vulkan ./myapp        # Force Vulkan
GOGPU_GRAPHICS_API=software ./myapp      # Force software renderer
GOGPU_RENDER_MODE=cpu ./myapp            # Force CPU rasterizer (benchmarking)
GOGPU_DEBUG_DAMAGE=1 ./myapp             # Show green overlay on dirty regions
```

`Config.With*()` methods in code take precedence over environment variables.

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

// Mouse grab / pointer lock (FPS games, 3D editors)
app.SetCursorMode(gpucontext.CursorModeLocked)   // hide + capture relative deltas (SDL parity)
app.SetCursorMode(gpucontext.CursorModeConfined)  // visible, confined to window
app.SetCursorMode(gpucontext.CursorModeNormal)    // release

// System preferences
if app.DarkMode() { /* switch to dark theme */ }
if app.ReduceMotion() { /* disable animations */ }
if app.HighContrast() { /* increase contrast */ }
fontMul := app.FontScale() // user's font size preference
```

### macOS System Menu

Native menu bar with role-based items (About, Preferences, Quit, etc.):

```go
app := gogpu.NewApp(gogpu.DefaultConfig().WithAppName("My App"))

app.SetMenu(gogpu.NewMenu().
    AddItem(gogpu.MenuItem{Title: "About My App", Role: gogpu.RoleAbout}).
    AddItem(gogpu.MenuItem{Separator: true}).
    AddItem(gogpu.MenuItem{Title: "Preferences…", Role: gogpu.RolePreferences}).
    AddItem(gogpu.MenuItem{Separator: true}).
    AddItem(gogpu.MenuItem{Title: "Quit", Role: gogpu.RoleQuit}),
)

if windowMenu := app.GetSystemMenu(gogpu.SystemMenuWindow); windowMenu != nil {
    windowMenu.AddItem(gogpu.MenuItem{Title: "Minimize", Role: gogpu.RoleMinimize})
    windowMenu.AddItem(gogpu.MenuItem{Title: "Close", Role: gogpu.RoleClose})
}
```

Menu can be set before or after `Run()`. On non-macOS platforms these calls are no-ops. See `examples/menu/` for a complete example.

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

### Multi-Window Input

Each window can have its own input handlers. `App.EventSource()` receives events from whichever window is focused:

```go
// Create secondary window
w2, _ := app.NewWindow(gogpu.DefaultConfig().WithTitle("Inspector"))

// Per-window input
w2.SetOnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
    // fires only when w2 is focused
})
w2.SetOnPointer(func(ev gpucontext.PointerEvent) {
    // pointer events for this window
})

// App-level: receives from ANY focused window
app.EventSource().OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
    if key == gpucontext.KeyN && mods.HasControl() {
        // Ctrl+N works regardless of which window is focused
    }
})
```

All input events flow through centralized dispatch with `WindowID` routing (winit/SDL3/Qt6 pattern).

### Window Lifecycle (ADR-026)

GPU Device is independent of any window. Closing a window destroys its surface but the device stays alive — remaining windows keep rendering. Same API on desktop, mobile, and web.

```go
// App lifecycle state: Idle → Running → Suspending → Suspended → Resuming
state := app.Lifecycle()      // AppRunning on desktop
if state.IsActive() { ... }   // true for Running/Suspending/Resuming

// Exit policy
app.SetQuitOnLastWindowClosed(false) // tray/headless: stay alive with zero windows

// Surface lifecycle (desktop: once at init; mobile: per-ANativeWindow)
app.OnSurfaceAvailable(func() { /* create GPU resources */ })
app.OnSurfaceDestroyed(func() { /* drop GPU surfaces NOW */ })

// App lifecycle (desktop: no-ops; mobile: Activity/UIApplication events)
app.OnResumed(func() { /* app visible */ })
app.OnSuspended(func() { /* app backgrounded, stop rendering */ })
app.OnMemoryWarning(func() { /* free caches */ })
```

See `examples/lifecycle/` for a visual demo: close the primary window, the secondary keeps rendering.

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
- **Main thread:** Window events only (Win32/Cocoa/X11 message pump), centralized input dispatch
- **Render thread:** All GPU operations (device, swapchain, commands)

All input events (keyboard, pointer, scroll, text) flow through a single centralized event queue with `WindowID` tagging — the same pattern used by winit, SDL3, and Qt6.

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
┌─────────────┐  ┌──────────────────┐
│  gogpu/wgpu │  │     Platform     │
│ (Pure Go    │  │    Windowing     │
│  WebGPU)    │  │ Win32/Cocoa/X11  │
└──────┬──────┘  │ Wayland/Browser  │
       │         └──────────────────┘
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
| `gpu/backend/native/` | HAL backend creation (Vulkan/Metal/DX12/GLES/Software selection) |
| `gmath/` | Vec2, Vec3, Vec4, Mat4, Color |
| `window/` | Window configuration |
| `input/` | Keyboard and mouse input |
| `sound/` | Platform system sounds (Click, Alert, Error, Warning, Success) |
| `internal/platform/` | Platform-specific windowing (Win32, Cocoa, X11, Wayland, Browser) |
| `internal/thread/` | Multi-thread rendering (RenderLoop) |

> **Rust backend:** Rust/wgpu-native selection lives inside [gogpu/wgpu](https://github.com/gogpu/wgpu) via build tags (ADR-038). Build with `-tags rust` and install wgpu-native: `go run github.com/go-webgpu/webgpu/cmd/setup@latest`

---

## Platform Support

### Windows

Native Win32 windowing with Vulkan, DirectX 12, GLES, and Software backends.

### Linux

X11 and Wayland support with Vulkan, GLES, and Software (headless) backends.

- **X11** — pure Go X11 protocol with libX11 loaded via goffi for Vulkan surface creation. Multi-touch input via XInput2 wire protocol.
- **Wayland** — single libwayland-client connection via goffi for all Wayland operations (surface, input, xdg-shell, CSD, pointer constraints). Server-side decorations via `zxdg_decoration_manager_v1`, client-side decorations (CSD) with subsurface title bar when SSD unavailable. Pointer lock via `zwp_pointer_constraints_v1` + `zwp_relative_pointer_v1`. Tested on WSLg, GNOME, KDE, sway, niri, COSMIC.

### macOS

Pure Go Cocoa implementation via goffi Objective-C runtime, with Metal and Software (headless) backends:

```
internal/platform/darwin/
├── application.go   # NSApplication lifecycle
├── window.go        # NSWindow, NSView management
├── surface.go       # CAMetalLayer integration
└── objc.go          # Objective-C runtime via goffi
```

Native system window tabbing supported via `Config.WithTabbingMode(gogpu.TabbingPreferred)` — windows with the same `TabbingIdentifier` group into macOS system tabs automatically.

**Note:** macOS Cocoa requires UI operations on the main thread. GoGPU handles this automatically.

---

## Ecosystem

| Project | Description |
|---------|-------------|
| **gogpu/gogpu** | **Application framework — windowing, input, lifecycle (this repo)** |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation (Vulkan, Metal, DX12, GLES, Software) |
| [gogpu/naga](https://github.com/gogpu/naga) | Shader compiler (WGSL → SPIR-V, MSL, GLSL, HLSL, DXIL) |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics — Skia-inspired rasterizer, GPU SDF, LCD ClearType |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit — 22+ widgets, 4 themes ([awesome-go](https://github.com/avelino/awesome-go)) |
| [gogpu/g3d](https://github.com/gogpu/g3d) | 3D rendering engine — PBR, scene graph, GLTF |
| [gogpu/compose](https://github.com/gogpu/compose) | Multi-process composition (Unix socket, LZ4, hot-plug) |
| [gogpu/audio](https://github.com/gogpu/audio) | Pure Go audio engine — WASAPI, WAV, Mixer |
| [gogpu/systray](https://github.com/gogpu/systray) | System tray — Windows, macOS, Linux, dark mode |
| [gogpu/gpucontext](https://github.com/gogpu/gpucontext) | Shared interfaces (DeviceProvider, WindowProvider, EventSource) |
| [gogpu/gputypes](https://github.com/gogpu/gputypes) | Shared WebGPU types (TextureFormat, BufferUsage, Limits) |
| [go-webgpu/goffi](https://github.com/go-webgpu/goffi) | Pure Go FFI library (no CGO) |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | wgpu-native FFI bindings (Rust backend) |

### Used By

| Project | Description |
|---------|-------------|
| [born-ml/born](https://github.com/born-ml/born) | Pure Go ML framework — GPU-accelerated training and inference |
| [ironwail-go](https://github.com/darkliquid/ironwail-go) | Quake 1 engine running on GoGPU Vulkan |

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
| [@lkmavi](https://github.com/lkmavi) | Star contributor — GLES Linux FFI fix (30+ GL calls), 4-phase renderer init, KDE Plasma AppMenu protocol, CSD cursor/hit-test, multiwindow Wayland fix, macOS native tabbing, native file dialogs + menus (all platforms). 10+ PRs across wgpu + gogpu |
| [@sverrehu](https://github.com/sverrehu) | X11 remote display auth fix — found `.Xauthority` binary address mismatch ([#203](https://github.com/gogpu/gogpu/issues/203)), confirmed fix. macOS key event fix ([#194](https://github.com/gogpu/gogpu/issues/194)) |
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
| [@z46-dev](https://github.com/z46-dev) · Evan Parker | Wayland champion — AMD Radeon RADV tester, filed [#292](https://github.com/gogpu/gogpu/issues/292) (Wayland SIGSEGV) and [#300](https://github.com/gogpu/gogpu/issues/300) (CSD geometry). Confirmed Software + Vulkan + GLES on native Linux |
| [@omer316](https://github.com/omer316) | Pop!_OS tester — detailed Wayland traces for [#292](https://github.com/gogpu/gogpu/issues/292), confirmed fixes across all backends. First to ask about [sponsorship](https://opencollective.com/gogpu) |

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

## Sponsors

GoGPU is an independent open-source project. Development is sustained by contributors and sponsors.

<a href="https://opencollective.com/gogpu"><img src="https://opencollective.com/gogpu/sponsors.svg?width=890&button=false" alt="Sponsors"></a>

### Backers

<a href="https://opencollective.com/gogpu"><img src="https://opencollective.com/gogpu/backers.svg?width=890&button=false" alt="Backers"></a>

<a href="https://opencollective.com/gogpu"><img src="https://img.shields.io/badge/Sponsor-Open%20Collective-7FADF2?logo=opencollective" alt="Sponsor on Open Collective"></a>

---

## Sponsors

GoGPU is free and open source. Development is supported by our sponsors.

🥇 **First Sponsor:** [Inkflow](https://opencollective.com/inkflow) — thank you for believing in the project from the start!

<a href="https://opencollective.com/gogpu#backer"><img src="https://opencollective.com/gogpu/backers.svg?avatarHeight=36&width=600" /></a>

<a href="https://opencollective.com/gogpu/donate"><img src="https://opencollective.com/gogpu/donate/button@2x.png?color=blue" width="200" /></a>

[See all sponsors](SPONSORS.md) | [Become a sponsor](https://opencollective.com/gogpu)

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Star History

<a href="https://star-history.com/#gogpu/gogpu&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=gogpu/gogpu&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=gogpu/gogpu&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=gogpu/gogpu&type=Date" />
 </picture>
</a>

---

<p align="center">
  <strong>GoGPU</strong> — Building the GPU computing ecosystem Go deserves
</p>
