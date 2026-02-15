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
       │   gogpu     │  ◄─HalProvider──►  │     gg      │
       │  Framework  │  (device sharing)  │ 2D Graphics │
       └──────┬──────┘                    └──────┬──────┘
              │                                  │
              │ Uses hal.Device/Queue            │
              │ directly (Go interfaces)         │
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
                       │    wgpu     │
                       │   hal.*     │
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

The renderer uses `hal.Device`/`hal.Queue` Go interfaces directly — no handle-based abstraction layer.

| Backend      | Description                | Build Tag      | GPU Required |
|--------------|----------------------------|----------------|--------------|
| **Native**   | Pure Go via gogpu/wgpu HAL | (default)      | Yes          |
| **Rust**     | wgpu-native via FFI        | `-tags rust`   | Yes          |

### gg: CPU Core + GPU Accelerator (ARCH-008)

gg uses a fundamentally different model: **CPU is the core, GPU is an optional accelerator**.

| Component | Description | GPU Required |
|-----------|-------------|--------------|
| **internal/raster/** | CPU rasterization core (always available) | No |
| **internal/gpu/** | GPU three-tier rendering: SDF shapes (Tier 1), convex fast-path (Tier 2a), stencil-then-cover (Tier 2b) | Yes |
| **gpu/** | Public opt-in registration (`import _ "gg/gpu"`) | Yes |

GPU accelerator uses `hal.Queue` interface — works with any wgpu backend (Vulkan, Metal, DX12).
When gogpu is present, gg receives the shared device via `gpucontext.HalProvider`.

### wgpu HAL Backends

| Backend      | Description                | Platform       |
|--------------|----------------------------|----------------|
| **Vulkan**   | Vulkan 1.x                 | Windows, Linux |
| **Metal**    | Metal 2.x                  | macOS, iOS     |
| **DX12**     | DirectX 12                 | Windows        |
| **GLES**     | OpenGL ES 3.x              | Windows, Linux, Android |
| **Software** | CPU emulation              | All platforms  |

### Software Rendering: Two Levels

There are **two different** software rendering options:

| Component            | Level     | Purpose                              |
|----------------------|-----------|--------------------------------------|
| `wgpu/hal/software`  | HAL       | Full WebGPU emulation on CPU         |
| `gg/internal/raster` | Core      | CPU 2D rasterizer (always available) |

- **wgpu/hal/software** — Emulates GPU operations for testing or headless environments
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
- gg receives GPU device from gogpu via `gpucontext.HalProvider` (direct HAL access)
- gg GPU accelerator uses `hal.Device`/`hal.Queue` for render pipeline dispatch
- All projects use compatible `gputypes.TextureFormat` etc.

## Package Structure

### gogpu

```
gogpu/
├── app.go              # Application lifecycle (three-state main loop)
├── config.go           # Configuration (builder pattern)
├── context.go          # Drawing context
├── renderer.go         # Uses hal.Device/Queue directly
├── texture.go          # Texture management (hal.Texture/View/Sampler)
├── fence_pool.go       # GPU fence pool (hal.Fence)
├── animation.go        # AnimationController + AnimationToken
├── invalidator.go      # Goroutine-safe redraw coalescing
├── event_source.go     # gpucontext.EventSource adapter
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

**Note:** The renderer uses `hal.Device`/`hal.Queue` Go interfaces directly from `gogpu/wgpu/hal`.
Both Native and Rust backends implement the same `hal.*` interfaces — thin wrapper structs with zero handle maps.
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

| Platform | Pointer Events | Keyboard | Scroll |
|----------|---------------|----------|--------|
| Windows  | WM_MOUSE*     | WM_KEYDOWN/UP | WM_MOUSEWHEEL |
| Linux (Wayland) | wl_pointer | wl_keyboard | wl_pointer.axis |
| Linux (X11) | MotionNotify, ButtonPress | KeyPress/Release | Button 4-7 |
| macOS    | NSEvent mouse | NSEvent key | NSEvent scroll |

## Renderer Pipeline

```
1. newRenderer()   → Create HAL backend based on GraphicsAPI selection [on render thread]
                     (Vulkan/DX12/Metal/GLES/Software — controlled by WithGraphicsAPI())
2. init()          → Instance → Surface → Adapter → Device (hal.Device) → Queue (hal.Queue)
3. BeginFrame()    → surface.AcquireTexture() → device.CreateTextureView()
4. User draws      → Via Context in OnDraw callback
5. EndFrame()      → queue.Submit() → queue.Present() (with fence-based tracking)
```

## Why Different GPU Models?

gogpu and gg use GPU differently by design:

| Aspect           | gogpu                         | gg                         |
|------------------|-------------------------------|----------------------------|
| **Purpose**      | GPU framework                 | 2D graphics library        |
| **GPU model**    | HAL direct (hal.Device/Queue) | CPU core + GPU accelerator |
| **GPU API**      | hal.Device/Queue              | hal.Device/Queue (HAL)     |
| **Without GPU**  | Cannot run                    | Falls back to CPU core     |
| **Integration**  | Owns device                   | Borrows via HalProvider    |

Both use `hal.Device`/`hal.Queue` Go interfaces from **gogpu/wgpu** — no intermediate abstractions.

## Why HAL Direct? (Architecture Decision)

### Historical Context

GoGPU started (December 2025) with **only a Rust backend** — wrapping wgpu-native via FFI.
The `gpu.Backend` interface was designed for this C-style world:

```
Go code → gpu.Backend (Go interface)
    → rust.Backend (Go struct with uintptr handles)
        → wgpu-native C API (returns opaque pointers as uintptr)
```

In this design, `uintptr` handles were **natural** — wgpu-native returns C pointers,
Go stores them as `uintptr`, and maps track the association. This is exactly how every
Go wrapper for a C library works (database/sql, OpenGL bindings, etc.).

### The Problem: Pure Go Backend (January 2026)

When we added the **Pure Go backend** (gogpu/wgpu), the handle pattern became redundant:

```
Go code → gpu.Backend → native.Backend (Go struct with uintptr handles)
    → ResourceRegistry (40+ maps: uintptr → Go interface)
        → hal.Device (already a Go interface!)
```

The Pure Go path was creating Go objects, converting them to `uintptr` handles,
storing them in maps, then looking them up by handle to call the same Go methods.
This added **~2000 lines of pure indirection** with no benefit:

1. **Error swallowing** — 10+ Backend methods returned no error, silently discarding GPU failures
2. **O(1) overhead per call** — map lookup for every GPU operation
3. **Memory pressure** — 40+ maps holding references that the GC must scan

### The Fix: HAL Direct (v0.18.0)

Industry research confirmed that **no production 2D/3D engine** adds a handle layer over WebGPU:
- **Bevy** → wgpu directly (Rust traits, not handles)
- **Vello** → wgpu directly
- **Skia Graphite** → Dawn directly (C++ objects, not handles)
- **gg** → hal.Queue directly (already working)

The refactoring eliminates the indirection entirely:
- Renderer stores `hal.Device`, `hal.Queue`, `hal.Texture` etc. as Go interface values
- All GPU errors propagate via `fmt.Errorf("context: %w", err)` chains
- ~2700 net lines removed
- Rust backend rewritten as thin HAL adapter (24 wrapper structs, zero handle maps)

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

| Platform | Status       | GPU Backends       |
|----------|--------------|--------------------|
| Windows  | Full support | Vulkan, DX12, GLES |
| macOS    | Full support | Metal              |
| Linux    | Full support | Vulkan, GLES       |
| Web      | Planned      | WebGPU             |

## See Also

- [README.md](../README.md) — Quick start guide
- [CHANGELOG.md](../CHANGELOG.md) — Version history
- [Examples](../examples/) — Code examples
