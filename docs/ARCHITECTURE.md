# GoGPU Architecture

This document describes the architecture of the GoGPU ecosystem.

## Overview

GoGPU is a Pure Go GPU computing ecosystem with dual-backend WebGPU support.

```
┌────────────────────────────────────────────────────────────────────┐
│                        User Application                            │
└───────────────────────────────┬────────────────────────────────────┘
                                │
              ┌─────────────────┴────────────────┐
              │                                  │
       ┌──────▼──────┐                    ┌──────▼──────┐
       │   gogpu     │  ◄─DeviceProvider─►│     gg      │
       │  Framework  │  (device sharing)  │ 2D Graphics │
       └──────┬──────┘                    └──────┬──────┘
              │                                  │
              │ Uses wgpu.Device/Queue           │
              │ (wgpu public API)                │
              │                    ┌─────────────┼──────────────┐
              │                    │             │              │
              │             ┌──────▼────┐  ┌─────▼─────┐  ┌─────▼─────┐
              │             │gg/internal│  │gg/internal│  │  gg/gpu   │
              │             │  /raster/ │  │   /gpu/   │  │ (opt-in   │
              │             │ CPU Core  │  │ GPU Accel │  │  import)  │
              │             └───────────┘  └─────┬─────┘  └───────────┘
              │                                  │
              └──────────────────────────────────┘
                              │
                       ┌──────▼──────┐
                       │  wgpu API   │  ◄── public API (typed wrappers)
                       └──────┬──────┘
                              │
                       ┌──────▼──────┐
                       │  wgpu/core  │  ◄── validation, state machine, lifecycle
                       └──────┬──────┘
                              │
                       ┌──────▼──────┐
                       │  wgpu/hal   │  ◄── GPU API interfaces (advanced users)
                       └──────┬──────┘
                              │
           ┌──────────┬───────┼───────┬──────────┐
           │          │       │       │          │
      ┌────▼───┐ ┌────▼──┐ ┌──▼──┐ ┌──▼───┐ ┌────▼────┐
      │ Vulkan │ │ Metal │ │DX12 │ │ GLES │ │Software │
      │(Win/   │ │(macOS)│ │(Win)│ │(Win/ │ │ (CPU)   │
      │ Lin)   │ │       │ │     │ │ Lin) │ │         │
      └────────┘ └───────┘ └─────┘ └──────┘ └─────────┘
```

## Projects

| Project       | Description                          | Repository                                           |
|---------------|--------------------------------------|------------------------------------------------------|
| **gogpu**     | GPU graphics framework               | [gogpu/gogpu](https://github.com/gogpu/gogpu)        |
| **gputypes**  | Shared WebGPU types (ZERO deps)      | [gogpu/gputypes](https://github.com/gogpu/gputypes)  |
| **gpucontext**| Shared interfaces (imports gputypes) | [gogpu/gpucontext](https://github.com/gogpu/gpucontext) |
| **gg**        | 2D graphics library (Canvas API)     | [gogpu/gg](https://github.com/gogpu/gg)              |
| **wgpu**      | Pure Go WebGPU implementation        | [gogpu/wgpu](https://github.com/gogpu/wgpu)          |
| **naga**      | WGSL shader compiler                 | [gogpu/naga](https://github.com/gogpu/naga)          |

### Shared Infrastructure: gputypes + gpucontext

The ecosystem uses two shared packages to ensure type compatibility:

| Package | Role | Dependencies |
|---------|------|--------------|
| `gputypes` | All WebGPU types (TextureFormat, BufferUsage, etc.) | **ZERO** |
| `gpucontext` | Integration interfaces (DeviceProvider, Texture, etc.) | imports gputypes |

**Why two packages?**
- **gputypes** = Data definitions (stable, follows WebGPU spec)
- **gpucontext** = Behavioral contracts (evolves with API)
- Separation of concerns: types vs interfaces

**Why gpucontext imports gputypes?**
- Interfaces need types in method signatures
- Ensures type compatibility across all implementations
- No type conversion needed between projects

See the internal research document GPUCONTEXT_GPUTYPES_DECISION.md for full rationale.

## Backend System

### gogpu Backends

The renderer uses `*wgpu.Device`/`*wgpu.Queue` through the wgpu public API. All GPU operations go through the three-layer stack: wgpu API → wgpu/core → wgpu/hal.

| Backend      | Description                | Build Tag      | GPU Required |
|--------------|----------------------------|----------------|--------------|
| **Native**   | Pure Go via wgpu API → core → hal | (default)      | Yes          |
| **Rust**     | wgpu-native via FFI        | `-tags rust`   | Yes          |

### gg: CPU Core + GPU Accelerator (ARCH-008)

gg uses a fundamentally different model: **CPU is the core, GPU is an optional accelerator**.

| Component | Description | GPU Required |
|-----------|-------------|--------------|
| **internal/raster/** | CPU rasterization core (always available) | No |
| **internal/gpu/** | GPU three-tier rendering: SDF shapes (Tier 1), convex fast-path (Tier 2a), stencil-then-cover (Tier 2b) | Yes |
| **gpu/** | Public opt-in registration (`import _ "gg/gpu"`) | Yes |

GPU accelerator uses wgpu API — works with any backend (Vulkan, Metal, DX12).
When gogpu is present, gg receives the shared device via `gpucontext.DeviceProvider`.

### wgpu HAL Backends

| Backend      | Description                | Platform       | GPU Required |
|--------------|----------------------------|----------------|--------------|
| **Vulkan**   | Vulkan 1.x                 | Windows, Linux | Yes          |
| **Metal**    | Metal 2.x                  | macOS, iOS     | Yes          |
| **DX12**     | DirectX 12                 | Windows        | Yes          |
| **GLES**     | OpenGL ES 3.x              | Windows, Linux, Android | Yes |
| **Software** | CPU rasterizer             | All platforms  | No           |

### Software Rendering: Two Levels

There are **two different** software rendering options:

| Component            | Level     | Purpose                              | Always Compiled |
|----------------------|-----------|--------------------------------------|-----------------|
| `wgpu/hal/software`  | HAL       | Full WebGPU emulation on CPU         | Yes             |
| `gg/internal/raster` | Core      | CPU 2D rasterizer (always available) | Yes             |

- **wgpu/hal/software** — Full WebGPU HAL implementation on CPU. Always compiled (no build tags). Used for headless rendering, CI/CD, servers without GPU, and as last-resort fallback. On Windows, software-rendered frames are displayed via GDI `SetDIBitsToDevice` (PixelBlitter interface).
- **gg/internal/raster** — CPU rasterization core with analytic AA, always works without GPU

## Backend Selection

### gogpu

```go
// Default: Pure Go backend, auto-select graphics API
app := gogpu.NewApp(gogpu.DefaultConfig())

// Explicit backend selection
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendGo))
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendRust))

// Explicit graphics API selection (added in v0.18.0)
// Options: GraphicsAPIAuto, GraphicsAPIVulkan, GraphicsAPIDX12,
//          GraphicsAPIMetal, GraphicsAPIGLES, GraphicsAPISoftware
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithGraphicsAPI(gogpu.GraphicsAPIVulkan))

// Software backend — no GPU required, works everywhere
// On Windows: renders to screen via GDI. Linux/macOS: headless (no screen output yet).
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithGraphicsAPI(gogpu.GraphicsAPISoftware))

// Combined: specific backend + specific graphics API
app := gogpu.NewApp(gogpu.DefaultConfig().
    WithBackend(gogpu.BackendNative).
    WithGraphicsAPI(gogpu.GraphicsAPIDX12))
```

### gg

```go
import _ "github.com/gogpu/gg/gpu" // opt-in GPU acceleration

// CPU rasterization always works (no imports needed)
dc := gg.NewContext(800, 600)
dc.DrawCircle(400, 300, 100)
dc.Fill() // tries GPU first, falls back to CPU
```

### Build Tags

```bash
# Default: Native backend only
go build ./...

# With Rust backend (maximum performance)
go build -tags rust ./...
```

### Backend Priority

When multiple backends are available:

**gogpu:** Rust → Native

**gg:** GPU Accelerator (if registered) → CPU Core (always available)

## Dependency Graph

```
                         gputypes (ZERO deps)
                    All WebGPU types (100+)
                              │
                              ▼
                    gpucontext (imports gputypes)
                    Integration interfaces
                              │
         ┌────────────────────┼────────────────────┐
         │                    │                    │
         ▼                    ▼                    ▼
naga (shader)              wgpu              go-webgpu/webgpu
         │                    │                    │
         └────────►───────────┤                    │
                              │                    │
              ┌───────────────┼───────────────┐    │
              │               │               │    │
              ▼               ▼               ▼    │
           gogpu             gg           born-ml ◄┘
```

**Key relationships:**
- `gputypes` is the foundation — ZERO dependencies, all WebGPU types
- `gpucontext` imports `gputypes` — interfaces use shared types
- gogpu and gg do NOT depend on each other
- Both implement/consume gpucontext interfaces for interoperability
- gg receives GPU device from gogpu via `gpucontext.DeviceProvider`
- gg GPU accelerator uses `*wgpu.Device`/`*wgpu.Queue` for render pipeline dispatch
- All projects use compatible `gputypes.TextureFormat` etc.

## Package Structure

### gogpu

```
gogpu/
├── app.go              # Application lifecycle (three-state main loop)
├── config.go           # Configuration (builder pattern)
├── context.go          # Drawing context
├── renderer.go         # Uses *wgpu.Device/*wgpu.Queue (wgpu public API)
├── texture.go          # Texture management (*wgpu.Texture/View/Sampler)
├── fence_pool.go       # GPU fence pool (*wgpu.Fence)
├── animation.go        # AnimationController + AnimationToken
├── invalidator.go      # Goroutine-safe redraw coalescing
├── event_source.go     # gpucontext.EventSource adapter
├── resource_tracker.go  # ResourceTracker: automatic GPU resource cleanup (LIFO)
├── gpucontext_adapter.go # gpucontext.DeviceProvider + HalProvider
├── gesture.go          # GestureRecognizer (Vello-style)
├── gpu/
│   ├── types/          # Backend type enum (BackendType)
│   └── backend/
│       ├── native/     # HAL backend creation (Vulkan/Metal selection)
│       └── rust/       # Rust HAL adapter (opt-in, -tags rust)
├── gmath/              # Math (Vec2, Vec3, Mat4, Color)
├── window/             # Window config
├── input/              # Ebiten-style input state (keyboard, mouse)
└── internal/platform/  # OS windowing + input (Win32, Cocoa, X11, Wayland)
```

**Note:** The renderer uses `*wgpu.Device`/`*wgpu.Queue` from the wgpu public API.
All GPU operations go through the three-layer stack: wgpu API → wgpu/core → wgpu/hal.
WebGPU types (TextureFormat, BufferUsage, etc.) are imported from `github.com/gogpu/gputypes`.

### wgpu

```
wgpu/
├── core/               # Device, Queue, Surface
├── types/              # WebGPU type definitions
└── hal/
    ├── vulkan/         # Vulkan backend
    ├── metal/          # Metal backend
    ├── dx12/           # DirectX 12 backend
    ├── gles/           # OpenGL ES backend
    ├── software/       # CPU emulation
    └── noop/           # No-op (testing)
```

## HiDPI/Retina Support

GoGPU properly separates logical coordinates (points/DIP) from physical pixels
(framebuffer), following the industry-standard pattern used by GLFW, winit, SDL,
and every enterprise graphics library.

### Platform Interface

```go
type Platform interface {
    // LogicalSize returns window size in platform points (DIP).
    // 800x600 on Retina 2x. Use for layout and UI coordinates.
    LogicalSize() (width, height int)

    // PhysicalSize returns GPU framebuffer size in device pixels.
    // 1600x1200 on Retina 2x. Use for surface configuration.
    PhysicalSize() (width, height int)

    // ScaleFactor returns the DPI scale factor.
    // 2.0 on macOS Retina, 1.0-3.0 on Windows HiDPI, 1.0 on most Linux.
    ScaleFactor() float64
}
```

### User-Facing API

```go
app := gogpu.NewApp(gogpu.DefaultConfig().WithSize(800, 600))

// In callbacks:
w, h := app.Size()              // 800, 600 (logical points)
fw, fh := app.PhysicalSize()    // 1600, 1200 (physical pixels on Retina 2x)
scale := app.ScaleFactor()      // 2.0

// Context also exposes both:
ctx.Width()              // 800 (logical)
ctx.FramebufferWidth()   // 1600 (physical)
```

### Platform Implementation

| Platform | LogicalSize | PhysicalSize | ScaleFactor |
|----------|-------------|-------------|-------------|
| **macOS** | NSView bounds (points) | bounds × backingScaleFactor | `backingScaleFactor` |
| **Windows** | Client rect / DPI scale | Client rect (raw pixels) | `GetDpiForWindow() / 96` |
| **Linux X11** | Window geometry | Same (scale=1.0 baseline) | 1.0 |
| **Linux Wayland** | Window geometry | Same (scale=1.0 baseline) | 1.0 |

### Thread Safety

Surface reconfiguration happens exclusively on the render thread.
`PollEvents()` detects size changes and emits resize events, but does NOT
resize the GPU surface directly. The render thread handles surface
reconfiguration via `RequestResize()`.

## Multi-Thread Architecture

GoGPU uses enterprise-level multi-thread architecture (Ebiten/Gio pattern):

```
Main Thread (OS Thread 0)       Render Thread (Dedicated)
├─ runtime.LockOSThread()       ├─ runtime.LockOSThread()
├─ Win32/Cocoa/X11 Messages     ├─ GPU Initialization
├─ Window Events                ├─ ConsumePendingResize()
├─ RequestResize()              ├─ Surface.Configure()
└─ User Input                   └─ Acquire → Render → Present
```

**Benefits:**
- Window never shows "Not Responding" during heavy GPU operations
- Smooth resize without blocking on `vkDeviceWaitIdle`
- Professional responsiveness matching native applications

**Key Components:**
- `internal/thread.Thread` — OS thread abstraction with `runtime.LockOSThread()`
- `internal/thread.RenderLoop` — Deferred resize pattern
- `Platform.InSizeMove()` — Tracks modal resize loop (Windows)

## Event-Driven Rendering

The main loop uses a three-state model for optimal power efficiency:

```
┌─────────────────────────────────────────────────────────┐
│                    Main Loop States                     │
│                                                         │
│  ┌──────────┐    StartAnimation()    ┌───────────────┐  │
│  │   IDLE   │ ─────────────────────► │  ANIMATING    │  │
│  │  0% CPU  │ ◄───────────────────── │  VSync 60fps  │  │
│  │ WaitEvents│    token.Stop()       │               │  │
│  └────┬─────┘                        └───────────────┘  │
│       │                                                 │
│       │ RequestRedraw()                                 │
│       ▼                                                 │
│  ┌──────────┐    ContinuousRender=true                  │
│  │ ONE FRAME│ ──────────────────────►┌───────────────┐  │
│  │  render  │                        │  CONTINUOUS   │  │
│  │  + idle  │                        │  game loop    │  │
│  └──────────┘                        └───────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### States

| State | Trigger | Behavior | CPU |
|-------|---------|----------|-----|
| **IDLE** | No animations, no invalidation | Blocks on `platform.WaitEvents()` | 0% |
| **ANIMATING** | `StartAnimation()` token active | Renders at VSync rate | ~2-5% |
| **CONTINUOUS** | `ContinuousRender=true` | Renders every frame | ~100% |

### Key Components

- **`Invalidator`** — Goroutine-safe redraw coalescing (Gio pattern).
  Uses a buffered channel (capacity 1) as a lock-free signal.
  Multiple concurrent `Invalidate()` calls produce exactly one wakeup.

- **`AnimationController`** / **`AnimationToken`** — Token-based animation lifecycle.
  Atomic counter tracks active animations. Loop renders at VSync while count > 0.

- **Platform `WaitEvents` / `WakeUp`** — Native OS blocking:
  - Windows: `MsgWaitForMultipleObjectsEx` / `PostMessageW(WM_NULL)`
  - macOS: `[NSApp nextEventMatchingMask:]` / `[NSApp postEvent:atStart:]`
  - Linux X11: `poll()` on connection fd / `XSendEvent(ClientMessage)`

### Main Loop Pseudocode

```
for running {
    continuous := config.ContinuousRender || animations.IsAnimating()
    invalidated := invalidator.Consume()

    if !continuous && !invalidated {
        platform.WaitEvents()   // blocks until OS event arrives (0% CPU)
    }

    processEvents()
    if continuous || invalidated || hasEvents {
        renderFrame()
    }
}
```

## Event System

GoGPU provides two complementary input handling patterns:

### Callback-based (UI Frameworks)

For UI frameworks that need discrete event handling:

```
Platform Layer          EventSource              User Code
     │                       │                       │
     │──PointerEvent────────►│                       │
     │                       │──OnPointer()─────────►│
     │──ScrollEvent─────────►│                       │
     │                       │──OnScrollEvent()─────►│
     │──KeyEvent────────────►│                       │
     │                       │──OnKeyPress()────────►│
```

**Key interfaces (gpucontext):**
- `PointerEventSource` — W3C Pointer Events Level 3 (mouse/touch/pen)
- `ScrollEventSource` — Detailed scroll with delta mode
- `GestureEventSource` — Vello-style gestures (pinch, rotate, pan)
- `EventSource` — Keyboard, IME, focus events

### Polling-based (Game Loops)

For game loops that check input state each frame:

```
Platform Layer          InputState               Game Loop
     │                       │                       │
     │──PointerEvent────────►│ (update state)        │
     │──KeyEvent────────────►│ (update state)        │
     │                       │                       │
     │                       │◄──JustPressed()?──────│
     │                       │◄──Position()?─────────│
```

**Key types (input package):**
- `input.State` — Thread-safe input state container
- `input.KeyboardState` — JustPressed, Pressed, JustReleased
- `input.MouseState` — Position, Delta, Button state, Scroll

### Platform Implementation

| Platform | Pointer Events | Keyboard | Text Input | Scroll |
|----------|---------------|----------|------------|--------|
| Windows  | WM_MOUSE*     | WM_KEYDOWN/UP | WM_CHAR/SYSCHAR/UNICHAR (UTF-16 surrogates) | WM_MOUSEWHEEL |
| Linux (Wayland) | wl_pointer | wl_keyboard | TODO: xkb_state_key_get_utf8 | wl_pointer.axis |
| Linux (X11) | MotionNotify, ButtonPress | KeyPress/Release | TODO: xkb_state_key_get_utf8 | Button 4-7 |
| macOS    | NSEvent mouse | NSEvent key | NSEvent characters (UTF-8) | NSEvent scroll |

## Renderer Pipeline

```
1. newRenderer()   → Create backend based on GraphicsAPI selection [on render thread]
                     (Vulkan/DX12/Metal/GLES/Software — controlled by WithGraphicsAPI())
2. init()          → Instance → Surface → Adapter → Device (*wgpu.Device) → Queue (*wgpu.Queue)
3. BeginFrame()    → surface.AcquireTexture() → device.CreateTextureView()
4. User draws      → Via Context in OnDraw callback
5. EndFrame()      → queue.Submit() → queue.Present() → blitSoftwareFramebuffer()
                     (software backend: reads framebuffer, blits to window via PixelBlitter)
```

## Why Different GPU Models?

gogpu and gg use GPU differently by design:

| Aspect           | gogpu                         | gg                         |
|------------------|-------------------------------|----------------------------|
| **Purpose**      | GPU framework                 | 2D graphics library        |
| **GPU model**    | wgpu API (*wgpu.Device/Queue) | CPU core + GPU accelerator |
| **GPU API**      | *wgpu.Device/*wgpu.Queue      | *wgpu.Device/*wgpu.Queue   |
| **Without GPU**  | Cannot run                    | Falls back to CPU core     |
| **Integration**  | Owns device                   | Borrows via DeviceProvider |

Both use `*wgpu.Device`/`*wgpu.Queue` from the wgpu public API — three-layer stack (API → core → hal).

## Architecture Evolution

### Historical Context

GoGPU started (December 2025) with **only a Rust backend** — wrapping wgpu-native via FFI.
The `gpu.Backend` interface was designed for this C-style world with `uintptr` handles.

### Phase 1: HAL Direct (v0.18.0)

When we added the **Pure Go backend** (gogpu/wgpu), the handle pattern became redundant —
creating Go objects, converting to `uintptr`, storing in maps, looking up by handle.
This added ~2000 lines of pure indirection. The fix: use HAL interfaces directly.

### Phase 2: wgpu Public API (v0.24.0)

After building wgpu's three-layer architecture (public API → core → hal), both gogpu
and gg were migrated from HAL interfaces to the wgpu public API:

- **gogpu renderer** stores `*wgpu.Device`, `*wgpu.Queue`, `*wgpu.Surface` (concrete types)
- **gg GPU accelerator** uses `*wgpu.Device`/`*wgpu.Queue` for all GPU operations
- **HAL layer** is an implementation detail — consumers never import `wgpu/hal`
- All GPU types (descriptors, barriers, copies) use `wgpu.*` types with `toHAL()` converters

Industry pattern confirmed: **Bevy**, **Vello**, **Skia Graphite** all use typed wrappers
over their GPU backends, not raw interfaces.

## Resource Lifecycle

GPU resources (textures, buffers, pipeline states) require explicit cleanup because
the Go garbage collector cannot release GPU memory. GoGPU provides three layers of
resource lifecycle management:

### 1. TrackResource — Automatic LIFO Cleanup

```go
app := gogpu.NewApp(config)

// Resources tracked on App are destroyed automatically at shutdown
texture := app.CreateTexture(desc)
app.TrackResource(texture)   // LIFO: last tracked = first destroyed
```

`App.TrackResource(io.Closer)` registers a resource for automatic destruction.
On shutdown, all tracked resources are closed in **reverse registration order** (LIFO),
matching the dependency order of GPU resources (child resources destroyed before parents).

Key properties:
- **Thread-safe** — `Track`/`Untrack` can be called from any goroutine
- **Panic recovery** — a panicking `Close()` does not prevent remaining resources from closing
- **Idempotent** — double `CloseAll` is a no-op; tracking after shutdown closes immediately
- **Untrack** — `UntrackResource()` removes a resource (e.g., when manually destroyed early)

### 2. Deferred Destruction Queue

GPU resources must be destroyed on the render thread. `Renderer.EnqueueDeferredDestroy(fn)`
accepts a cleanup function from any goroutine; the queue is drained at the start of each
`BeginFrame()` call on the render thread.

### 3. runtime.AddCleanup Safety Net

`Texture` registers a `runtime.AddCleanup` callback (Go 1.24+) that enqueues deferred
destruction if the texture is garbage-collected without an explicit `Destroy()` call.
This is a **safety net only** — explicit `Destroy()` (or `TrackResource`) is the correct pattern.

### Shutdown Sequence

```
App.Run() exits
  → WaitForGPU()                    // fence-based GPU idle
  → DrainDeferredDestroys()         // process any pending cleanup
  → tracker.CloseAll()              // LIFO resource destruction
  → OnClose() callback              // user cleanup
  → Renderer.Destroy()              // release GPU device
```

### Auto-Registration (ggcanvas)

`ggcanvas.Canvas` auto-registers itself with the tracker via duck-typed interface detection.
When the `gpucontext.DeviceProvider` also implements `TrackResource(io.Closer)`, the canvas
registers at creation and unregisters at `Close()` — no manual `OnClose` wiring needed.

## SurfaceView (Zero-Copy Rendering)

When gg runs inside a gogpu window (via ggcanvas), the standard path involves a
GPU-to-CPU readback of the rendered image followed by a CPU-to-GPU upload to the
surface texture. The `Context.SurfaceView()` method exposes the current frame's
surface texture view, enabling gg to render directly to the gogpu surface with no
readback. This is the `RenderModeSurface` path in gg's `GPURenderSession`.

```
Standard path:    gg GPU render -> ReadBuffer (GPU->CPU) -> WriteTexture (CPU->GPU) -> Present
SurfaceView path: gg GPU render -> resolve to surface view -> Present (zero copy)
```

The accelerator implements `SurfaceTargetAware` so that ggcanvas can call
`SetAcceleratorSurfaceTarget(view, w, h)` each frame, switching the session to
surface-direct mode. When the view is nil, the session falls back to offscreen
readback for standalone usage.

## Structured Logging

All ecosystem packages use `log/slog` for structured logging. By default, gogpu
and gg produce no log output (silent nop handler). Users opt in via `SetLogger`:

```go
gogpu.SetLogger(slog.Default()) // info-level logging to stderr

// Or with full diagnostics:
gogpu.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})))
```

Log levels across the ecosystem:
- `slog.LevelDebug` -- internal diagnostics (texture creation, pipeline state, shader compilation)
- `slog.LevelInfo` -- lifecycle events (backend selected, adapter info, GPU capabilities)
- `slog.LevelWarn` -- non-fatal issues (resource cleanup errors, fallback paths)

The logger is stored atomically and is safe for concurrent use. Accelerators
inherit the logger configuration when registered.

## Platform Support

| Platform | Status       | GPU Backends                   |
|----------|--------------|--------------------------------|
| Windows  | Full support | Vulkan, DX12, GLES, Software   |
| macOS    | Full support | Metal, Software (headless)     |
| Linux    | Full support | Vulkan, GLES, Software (headless) |
| Web      | Planned      | WebGPU                         |

## See Also

- [README.md](../README.md) — Quick start guide
- [CHANGELOG.md](../CHANGELOG.md) — Version history
- [Examples](../examples/) — Code examples
