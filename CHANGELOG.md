# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0-alpha] - 2025-12-08

### Added
- **Texture Loading API** - Load textures from files, Go images, or raw RGBA data
  - `LoadTexture(path)` - Load from PNG/JPEG files
  - `LoadTextureFromReader(reader)` - Load from io.Reader
  - `NewTextureFromImage(img)` - Create from image.Image
  - `NewTextureFromRGBA(w, h, data)` - Create from raw RGBA pixels
  - `TextureOptions` - Configure filtering and address modes
- **GPU Types Extensions**
  - New handle types: `Buffer`, `Sampler`, `BindGroupLayout`, `BindGroup`, `PipelineLayout`
  - Sampler descriptors with filter modes and address modes
  - Bind group layout and bind group descriptors
  - Vertex buffer layouts and vertex formats
  - Image copy and data layout types
- **Texture Shaders**
  - `TexturedQuadShader()` - Full shader with uniforms and transforms
  - `SimpleTextureShader()` - Simple full-screen textured quad
- **Backend Interface Extensions**
  - ~20 new methods for texture, sampler, buffer, and bind group operations
  - Full implementation in Rust backend
  - Stubs in Native Go backend
- **Examples**
  - `examples/texture/` - Texture API demonstration
- **Tests**
  - 10 new tests for Texture API
  - 40+ new tests for texture-related types

## [0.2.0-alpha] - 2025-12-07

### Added
- **Dual Backend Architecture** - Choose between Rust (wgpu-native) and Pure Go backends
  - `WithBackend(gogpu.BackendRust)` - Maximum performance
  - `WithBackend(gogpu.BackendGo)` - Zero dependencies
  - `BackendAuto` - Auto-select best available (default)
- **Backend Abstraction Layer**
  - `gpu/backend.go` - Backend interface definition
  - `gpu/backend/rust/` - Rust backend wrapper (wgpu-native via FFI)
  - `gpu/backend/native/` - Native Go backend stub
- **gpu/types Package** - Standalone types following wgpu-types pattern
  - `handles.go` - Instance, Device, Texture, Queue, etc.
  - `enums.go` - TextureFormat, PresentMode, BackendType, etc.
  - `descriptors.go` - SurfaceConfig, RenderPipelineDescriptor, etc.
- **CI/CD Infrastructure**
  - GitHub Actions workflow (build, test, lint)
  - Codecov integration (98%+ core coverage)
  - golangci-lint v2 configuration with 30+ linters
- **Community Files**
  - CODE_OF_CONDUCT.md
  - SECURITY.md
  - CODEOWNERS

### Changed
- Renamed `math/` package to `gmath/` to avoid stdlib conflict
- Refactored renderer to use Backend interface

## [0.1.0-alpha] - 2025-12-06

### Added
- **First Working Rendering** - Triangle renders on screen!
- **Simple API** - ~20 lines vs 480+ lines of raw WebGPU
  ```go
  app := gogpu.NewApp(gogpu.DefaultConfig())
  app.OnDraw(func(ctx *gogpu.Context) {
      ctx.DrawTriangleColor(gmath.DarkGray)
  })
  app.Run()
  ```
- **Core Packages**
  - `app.go` - Application lifecycle management
  - `config.go` - Configuration with builder pattern
  - `context.go` - Drawing context API
  - `renderer.go` - WebGPU rendering integration
  - `shader.go` - Built-in WGSL shaders
- **Platform Abstraction**
  - `internal/platform/` - Cross-platform interface
  - Windows implementation (Win32)
  - macOS/Linux stubs
- **Math Library** (`gmath/`)
  - Vec2, Vec3, Vec4 - Vector types with operations
  - Mat4 - 4x4 matrix with transforms, projections
  - Color - RGBA colors with hex parsing, lerp
- **Window & Input**
  - `window/` - Window configuration and events
  - `input/` - Keyboard and mouse handling
- **Examples**
  - `examples/triangle/` - Simple triangle demo

### Notes
- First public announcement on Dev.to
- Windows platform only (macOS/Linux stubs)

## [0.0.1-dev] - 2025-12-05

### Added
- Initial project structure
- GitHub organization setup
- Basic type definitions
- Kanban system for task management
- Documentation structure

[Unreleased]: https://github.com/gogpu/gogpu/compare/v0.3.0-alpha...HEAD
[0.3.0-alpha]: https://github.com/gogpu/gogpu/compare/v0.2.0-alpha...v0.3.0-alpha
[0.2.0-alpha]: https://github.com/gogpu/gogpu/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/gogpu/gogpu/compare/v0.0.1-dev...v0.1.0-alpha
[0.0.1-dev]: https://github.com/gogpu/gogpu/releases/tag/v0.0.1-dev
