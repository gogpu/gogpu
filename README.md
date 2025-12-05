# gogpu

[![Go Reference](https://pkg.go.dev/badge/github.com/gogpu/gogpu.svg)](https://pkg.go.dev/github.com/gogpu/gogpu)
[![Go Report Card](https://goreportcard.com/badge/github.com/gogpu/gogpu)](https://goreportcard.com/report/github.com/gogpu/gogpu)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Pure Go Graphics Framework** — Build GPU-accelerated applications with zero CGO.

> 🚧 **Work in Progress** — API is evolving. Star the repo to follow development!

---

## ✨ Features

- **Zero CGO** — No C compiler required
- **WebGPU Backend** — Modern, cross-platform GPU API
- **Simple API** — Inspired by raylib, Processing, and Ebitengine
- **Cross-Platform** — Windows, Linux, macOS
- **Pure Go Goal** — Gradually replacing FFI with native implementation

## 📦 Installation

```bash
go get github.com/gogpu/gogpu
```

**Requirements:**
- Go 1.25+
- [wgpu-native](https://github.com/gfx-rs/wgpu-native/releases) library

## 🚀 Quick Start

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

## 📚 Examples

| Example | Description |
|---------|-------------|
| [triangle](examples/triangle) | Basic triangle rendering |
| [texture](examples/texture) | Texture loading and rendering |
| [3d-cube](examples/cube) | 3D cube with depth buffer |
| [sprites](examples/sprites) | 2D sprite batching |

```bash
# Run an example
go run ./examples/triangle
```

## 🏗️ Architecture

```
gogpu/
├── gpu/           # Core GPU abstraction
├── window/        # Window management
├── input/         # Keyboard, mouse, gamepad
├── math/          # Vec2, Vec3, Mat4, etc.
├── examples/      # Example applications
└── internal/      # Private implementation
```

## 🗺️ Roadmap

- [x] Project structure
- [ ] Window creation
- [ ] Basic rendering (triangle, quad)
- [ ] Texture support
- [ ] 2D sprite batching
- [ ] Text rendering
- [ ] 3D rendering
- [ ] Audio (future)

## 🔗 Related Projects

| Project | Description |
|---------|-------------|
| [gogpu/naga](https://github.com/gogpu/naga) | Pure Go shader compiler |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics library |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | WebGPU FFI bindings |
| [born-ml/born](https://github.com/born-ml/born) | ML framework using gogpu |

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

```bash
# Clone the repository
git clone https://github.com/gogpu/gogpu
cd gogpu

# Run tests
go test ./...

# Run linter
golangci-lint run
```

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>GoGPU</b> — GPU power, Go simplicity
</p>
