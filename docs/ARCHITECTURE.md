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
| **gpucontext**| Shared interfaces (DeviceProvider)   | [gogpu/gpucontext](https://github.com/gogpu/gpucontext) |
| **gputypes**  | Shared WebGPU types *(planned)*      | [gogpu/gputypes](https://github.com/gogpu/gputypes)  |
| **gg**        | 2D graphics library (Canvas API)     | [gogpu/gg](https://github.com/gogpu/gg)              |
| **wgpu**      | Pure Go WebGPU implementation        | [gogpu/wgpu](https://github.com/gogpu/wgpu)          |
| **naga**      | WGSL shader compiler                 | [gogpu/naga](https://github.com/gogpu/naga)          |

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
                    gpucontext (shared interfaces)
                    gputypes (shared types) [planned]
                           │
naga (shader compiler)     │
  │                        │
  └──► wgpu ◄──────────────┤
         │                 │
         ├──► gogpu ───────┤ (implements DeviceProvider)
         │                 │
         └──► gg ──────────┘ (consumes DeviceProvider)
```

**Key relationships:**
- gogpu and gg do NOT depend on each other
- Both implement/consume gpucontext interfaces for interoperability
- gg can receive GPU device from gogpu via DeviceProvider pattern

## Package Structure

### gogpu

```
gogpu/
├── app.go              # Application lifecycle
├── config.go           # Configuration (builder pattern)
├── context.go          # Drawing context
├── renderer.go         # WebGPU pipeline
├── texture.go          # Texture management
├── gpu/
│   ├── backend.go      # Backend interface (120+ methods)
│   ├── registry.go     # Auto-registration
│   ├── types/          # WebGPU types
│   └── backend/
│       ├── native/     # Pure Go backend
│       └── rust/       # Rust FFI backend
├── gmath/              # Math (Vec2, Vec3, Mat4, Color)
├── window/             # Window config
├── input/              # Input types
└── internal/platform/  # OS windowing (Win32, Cocoa, X11)
```

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

## Renderer Pipeline

```
1. newRenderer()   → Create backend (Auto/Rust/Native)
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
