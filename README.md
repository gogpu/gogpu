<p align="center">
  <img src="assets/logo.png" alt="GoGPU Logo" width="180" />
</p>

<h1 align="center">GoGPU</h1>

<p align="center">
  <strong>Pure Go GPU Computing Ecosystem</strong><br>
  GPU power, Go simplicity. Zero CGO.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/gogpu/gogpu"><img src="https://pkg.go.dev/badge/github.com/gogpu/gogpu.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/gogpu/gogpu"><img src="https://goreportcard.com/badge/github.com/gogpu/gogpu" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/gogpu/gogpu"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version"></a>
</p>

---

## Status: Early Development (v0.0.x-dev)

> **This project is in active development.** The API is unstable and will change without notice. Not recommended for production use yet.
>
> **Star the repo to follow progress!**

---

## Why GoGPU?

This project was inspired by [a discussion on r/golang](https://www.reddit.com/r/golang/comments/1pdw9i7/go_deserves_more_support_in_gui_development/) about the state of GUI and graphics development in Go. The Go ecosystem lacks a cohesive, modern GPU computing stack that follows Go's philosophy of simplicity and zero-friction tooling.

**GoGPU aims to fill this gap** by providing:

1. **Graphics Foundation** — GPU abstraction layer (this repo)
2. **Shader Tooling** — Pure Go shader compiler ([gogpu/naga](https://github.com/gogpu/naga))
3. **2D Graphics** — Simple drawing API ([gogpu/gg](https://github.com/gogpu/gg)) — planned
4. **GUI Framework** — Widget toolkit (future) — planned

```
┌─────────────────────────────────────────────────────────┐
│              Your Application / GUI                      │
├─────────────────────────────────────────────────────────┤
│    gogpu/ui (future)    │    gogpu/gg (2D graphics)     │
├─────────────────────────────────────────────────────────┤
│              gogpu/gogpu (this repo)                     │
│         GPU abstraction, windowing, input                │
├─────────────────────────────────────────────────────────┤
│    go-webgpu/webgpu (FFI)  →  gogpu/wgpu (Pure Go)      │
├─────────────────────────────────────────────────────────┤
│           Vulkan  │  Metal  │  DX12  │  OpenGL          │
└─────────────────────────────────────────────────────────┘
```

---

## Features

| Feature | Description |
|---------|-------------|
| **Zero CGO** | No C compiler required, simple `go build` |
| **WebGPU API** | Modern, portable GPU abstraction |
| **Cross-Platform** | Windows, Linux, macOS |
| **Pure Go Goal** | Gradually replacing FFI with native implementation |
| **Simple API** | Inspired by raylib, Ebitengine, Processing |

---

## Installation

```bash
go get github.com/gogpu/gogpu
```

**Requirements:**
- Go 1.25+
- [wgpu-native](https://github.com/gfx-rs/wgpu-native/releases) library (auto-downloaded in future)

---

## Quick Start

```go
package main

import "github.com/gogpu/gogpu"

func main() {
    app := gogpu.NewApp(gogpu.Config{
        Title:  "Hello GoGPU",
        Width:  800,
        Height: 600,
    })

    app.OnDraw(func(ctx *gogpu.Context) {
        ctx.Clear(gogpu.Color{0.1, 0.1, 0.1, 1.0})
        ctx.DrawTriangle(
            gogpu.Vec2{400, 100},
            gogpu.Vec2{200, 500},
            gogpu.Vec2{600, 500},
            gogpu.Red,
        )
    })

    app.Run()
}
```

> **Note:** This API is not yet implemented. It represents the target design.

---

## Package Structure

```
gogpu/
├── gpu/           # Core GPU abstraction (Device, Buffer, Texture, Pipeline)
├── window/        # Cross-platform windowing
├── input/         # Keyboard, mouse, gamepad input
├── math/          # Vec2, Vec3, Vec4, Mat4, Color
├── examples/      # Example applications
└── internal/      # Private implementation details
```

---

## Roadmap

### v0.1.0-alpha — First Rendering
- [ ] Window creation
- [ ] Basic rendering (triangle)
- [ ] WGSL shader support

### v0.2.0-alpha — 2D Graphics
- [ ] Texture loading
- [ ] Sprite batching
- [ ] Basic shapes

### v0.5.0-beta — Usable
- [ ] Text rendering
- [ ] Input handling
- [ ] Audio (basic)

### v1.0.0 — Stable
- [ ] Stable API
- [ ] Full documentation
- [ ] Performance optimized

---

## Ecosystem

| Project | Description | Status |
|---------|-------------|--------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | Graphics framework (this repo) | Active |
| [gogpu/naga](https://github.com/gogpu/naga) | Pure Go shader compiler (WGSL → SPIR-V) | Active |
| [gogpu/gg](https://github.com/gogpu/gg) | Simple 2D graphics library | Planned |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation | Future |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | Zero-CGO WebGPU bindings | Stable |
| [go-webgpu/goffi](https://github.com/go-webgpu/goffi) | Pure Go FFI library | Stable |

---

## Contributing

Contributions are welcome! This is an early-stage project, so there's lots to do.

**Areas where we need help:**
- WGSL parser implementation
- WebGPU examples
- Platform testing (especially Linux/macOS)
- Documentation

```bash
# Clone
git clone https://github.com/gogpu/gogpu
cd gogpu

# Test
go test ./...

# Lint
golangci-lint run
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>GoGPU</strong> — Building the GPU computing ecosystem Go deserves
</p>
