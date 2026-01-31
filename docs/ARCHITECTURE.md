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
       │   gogpu     │                    │     gg      │
       │  Framework  │                    │ 2D Graphics │
       └──────┬──────┘                    └──────┬──────┘
              │                                  │
              │                    ┌─────────────┼──────────────┐
              │                    │             │              │
              │             ┌──────▼────┐  ┌─────▼─────┐  ┌─────▼─────┐
              │             │gg/backend │  │gg/backend │  │gg/backend │
              │             │   rust    │  │  native   │  │ software  │
              │             └─────┬─────┘  └─────┬─────┘  └─────┬─────┘
              │                   │              │              │
       ┌──────┴──────┐            │              │              │
       │             │            │              │         ┌────▼────┐
┌──────▼────┐ ┌──────▼────┐       │              │         │   CPU   │
│gogpu/back-│ │gogpu/back-│       │              │         │  2D     │
│end/rust   │ │end/native │       │              │         │Rasteriz.│
└─────┬─────┘ └─────┬─────┘       │              │         └─────────┘
      │             │             │              │
      │             └─────────────┼──────────────┘
      │                           │
      │                    ┌──────▼──────┐
      │                    │    wgpu     │
      │                    │    core     │
      │                    └──────┬──────┘
      │                           │
      │              ┌────────────┼────────────┐
      │              │            │            │
      │       ┌──────▼────┐ ┌─────▼─────┐ ┌────▼─────┐
      │       │  Vulkan   │ │   Metal   │ │ Software │
      │       │ (Win/Lin) │ │  (macOS)  │ │  (CPU)   │
      │       └───────────┘ └───────────┘ └──────────┘
      │                           │
      │                       wgpu/hal
      │
┌─────▼─────────┐
│  wgpu-native  │
│  (Rust FFI)   │
└───────────────┘
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

See [GPUCONTEXT_GPUTYPES_DECISION.md](dev/research/GPUCONTEXT_GPUTYPES_DECISION.md) for full rationale.

## Backend System

### gogpu Backends

| Backend      | Description                | Build Tag      | GPU Required |
|--------------|----------------------------|----------------|--------------|
| **Native**   | Pure Go via gogpu/wgpu     | (default)      | Yes          |
| **Rust**     | wgpu-native via FFI        | `-tags rust`   | Yes          |

### gg Backends

| Backend      | Description                | Build Tag      | GPU Required |
|--------------|----------------------------|----------------|--------------|
| **Native**   | Pure Go via gogpu/wgpu     | (default)      | Yes          |
| **Rust**     | wgpu-native via FFI        | `-tags rust`   | Yes          |
| **Software** | CPU 2D rasterizer          | (fallback)     | No           |

### wgpu HAL Backends

| Backend      | Description                | Platform       |
|--------------|----------------------------|----------------|
| **Vulkan**   | Vulkan 1.x                 | Windows, Linux |
| **Metal**    | Metal 2.x                  | macOS, iOS     |
| **DX12**     | DirectX 12                 | Windows        |
| **GLES**     | OpenGL ES 3.x              | Android, Web   |
| **Software** | CPU emulation              | All platforms  |

### Software Rendering: Two Levels

There are **two different** software rendering options:

| Component            | Level     | Purpose                              |
|----------------------|-----------|--------------------------------------|
| `wgpu/hal/software`  | HAL       | Full WebGPU emulation on CPU         |
| `gg/backend/software`| Backend   | Lightweight 2D rasterizer (no wgpu)  |

- **wgpu/hal/software** — Emulates GPU operations for testing or headless environments
- **gg/backend/software** — Direct 2D rendering without WebGPU overhead

## Backend Selection

### gogpu

```go
// Default: Pure Go backend
app := gogpu.NewApp(gogpu.DefaultConfig())

// Explicit backend selection
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendNative))
app := gogpu.NewApp(gogpu.DefaultConfig().WithBackend(gogpu.BackendRust))
```

### gg

```go
import "github.com/gogpu/gg/backend"

// Auto-select best available
b := backend.Default()

// Explicit selection
b := backend.Get(backend.BackendNative)
b := backend.Get(backend.BackendRust)
b := backend.Get(backend.BackendSoftware)
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

**gg:** Rust → Native → Software

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
- gg can receive GPU device from gogpu via DeviceProvider pattern
- All projects use compatible `gputypes.TextureFormat` etc.

## Package Structure

### gogpu

```
gogpu/
├── app.go              # Application lifecycle
├── config.go           # Configuration (builder pattern)
├── context.go          # Drawing context
├── renderer.go         # WebGPU pipeline
├── texture.go          # Texture management
├── event_source.go     # gpucontext.EventSource adapter
├── gesture.go          # GestureRecognizer (Vello-style)
├── gpu/
│   ├── backend.go      # Backend interface (120+ methods)
│   ├── registry.go     # Auto-registration
│   ├── types/          # GoGPU-specific types (handles, descriptors)
│   └── backend/
│       ├── native/     # Pure Go backend
│       └── rust/       # Rust FFI backend
├── gmath/              # Math (Vec2, Vec3, Mat4, Color)
├── window/             # Window config
├── input/              # Ebiten-style input state (keyboard, mouse)
└── internal/platform/  # OS windowing + input (Win32, Cocoa, X11, Wayland)
```

**Note:** WebGPU types (TextureFormat, BufferUsage, etc.) are imported directly from `github.com/gogpu/gputypes`.

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
1. newRenderer()   → Create backend (Auto/Rust/Native) [on render thread]
2. init()          → Instance → Surface → Adapter → Device → Queue
3. BeginFrame()    → Acquire surface texture
4. User draws      → Via Context in OnDraw callback
5. EndFrame()      → Present surface
```

## Why Separate Backend Systems?

gogpu and gg have **separate backend interfaces** by design:

| Aspect               | gogpu                | gg                    |
|----------------------|----------------------|-----------------------|
| **Purpose**          | GPU framework        | 2D graphics library   |
| **Interface methods**| 120+                 | 6                     |
| **API style**        | Handle-based         | Object-oriented       |
| **Software fallback**| No                   | Yes                   |

Both share **gogpu/wgpu** as the common WebGPU implementation.

## Platform Support

| Platform | Status       | GPU Backends          |
|----------|--------------|----------------------|
| Windows  | Full support | Vulkan, DX12         |
| macOS    | Full support | Metal                |
| Linux    | Full support | Vulkan               |
| Web      | Planned      | WebGPU               |

## See Also

- [README.md](../README.md) — Quick start guide
- [CHANGELOG.md](../CHANGELOG.md) — Version history
- [Examples](../examples/) — Code examples
