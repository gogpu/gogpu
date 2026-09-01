# Contributing to GoGPU

Thank you for your interest in contributing to GoGPU!

---

## Requirements

- **Go 1.25+** (required for iterators, generics, and modern features)
- **golangci-lint** for code quality checks

---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/gogpu/gogpu
cd gogpu

# Build
go build ./...

# Run tests
go test ./...

# Run linter
golangci-lint run --timeout=5m
```

---

## Development Workflow

### 1. Fork & Clone

```bash
git clone https://github.com/YOUR_USERNAME/gogpu
cd gogpu
git remote add upstream https://github.com/gogpu/gogpu
```

### 2. Sync with upstream

```bash
git fetch upstream main
git checkout main
git pull upstream main
git checkout -b feat/your-feature
```

### 3. Make Changes

- Follow code style guidelines below
- Add tests for new functionality
- Update documentation if needed

### 4. Validate Before Commit

```bash
# Format ALL files (including platform-specific)
gofmt -w .

# Build
go build ./...

# Run tests
go test ./...

# Lint (CI uses latest golangci-lint — keep yours updated)
golangci-lint run --timeout=5m

# Cross-platform lint (if touching platform-specific files)
GOOS=linux GOARCH=amd64 golangci-lint run --timeout=5m
GOOS=darwin GOARCH=arm64 golangci-lint run --timeout=5m
```

### 5. Create Pull Request

```bash
git add .
git commit -m "feat(component): description"
git push origin feat/your-feature
```

Then open a PR on GitHub.

---

## Pull Request Guidelines

### PR Requirements

- [ ] All tests pass (`go test ./...`)
- [ ] Linter passes (`golangci-lint run --timeout=5m`)
- [ ] Code is formatted (`gofmt -l .` returns nothing)
- [ ] Documentation updated (if applicable)
- [ ] CHANGELOG.md updated (for features/fixes)

### PR Title Format

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(platform): add Wayland fractional scaling
fix(compositor): prevent stale damage rect on resize
docs: update ROADMAP for v0.54.0
test(golden): add headless triangle snapshot
refactor(sound): extract platform sound constants
chore: update wgpu v0.34.0 → v0.34.2
```

### PR Description Template

```markdown
## Summary
Brief description of changes.

## Test plan
- [x] `go test ./...`
- [x] `golangci-lint run`
- [ ] Visual verification on [backends]

## Related Issues
Closes #123
```

### Merge Strategy

We use two merge modes depending on commit quality:

| Situation | Merge type | Command |
|-----------|-----------|---------|
| PR with iteration commits (review fixes, deps bumps, gofmt) | **Squash** | `gh pr merge N --squash --subject "type(scope): description"` |
| PR with multiple meaningful, self-contained commits | **Regular merge** | `gh pr merge N --merge` |

**Rule of thumb:** if each commit is independently valuable in `git log` → regular merge. If intermediate commits are "fix lint" / "address review" → squash into one clean commit.

Commit message **always** in conventional commits format, regardless of merge type.

---

## Code Style

### Go Conventions

- Use `gofmt` for formatting (tabs, not spaces)
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use pointer receivers for structs with mutexes
- `ID`, `URL`, `HTTP` are uppercase in names

### Naming

| Type | Convention | Example |
|------|------------|---------|
| Exported | PascalCase | `CreateSurface` |
| Unexported | camelCase | `handleEvent` |
| Acronyms | Uppercase | `GetHTTPURL`, `DeviceID` |
| Constants | PascalCase | `MaxTextureSize` |

### Error Handling

```go
// Always propagate errors with context
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Never silently discard errors
// ❌ _ = surface.Commit()
// ✅ if err := surface.Commit(); err != nil { ... }
```

### JSON Tags

```go
// Always camelCase for JSON tags
UserID string `json:"userId"`
CreatedAt time.Time `json:"createdAt"`
```

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `test` | Tests |
| `refactor` | Code refactoring |
| `perf` | Performance |
| `ci` | CI/CD changes |
| `chore` | Maintenance (deps, tooling) |

### Scopes

| Scope | Description |
|-------|-------------|
| `platform` | Platform code (Win32, Cocoa, X11, Wayland, Browser) |
| `compositor` | Surface compositor, damage tracking, blit pipeline |
| `gpu` | GPU backend, renderer |
| `gmath` | Math library |
| `input` | Input handling |
| `sound` | System sounds |
| `golden` | Golden image test harness |
| `examples` | Example code |
| `deps` | Dependencies |

---

## Project Structure

```
gogpu/
├── gpu/                       # GPU abstraction layer
│   ├── types/                 # BackendType, GraphicsAPI enums
│   └── backend/native/        # HAL backend creation
├── internal/
│   ├── platform/              # Platform-specific windowing
│   │   ├── platform_windows.go
│   │   ├── platform_darwin.go
│   │   ├── platform_linux.go
│   │   ├── platform_browser.go
│   │   ├── darwin/            # Objective-C runtime via goffi
│   │   ├── wayland/           # libwayland FFI, CSD, xdg-shell
│   │   ├── x11/              # Pure Go X11 wire protocol
│   │   └── eventqueue/       # Thread-safe event queue
│   ├── compositor/            # Surface compositor, blit pipeline, damage overlay
│   └── thread/                # Cross-goroutine panic propagation
├── gmath/                     # Vec2, Vec3, Vec4, Mat4, Color
├── golden/                    # Deterministic headless golden image harness
├── input/                     # Keyboard and mouse input
├── sound/                     # Platform system sounds (Win32, macOS, Linux, Browser)
├── window/                    # Window configuration
├── examples/                  # Example applications
├── docs/                      # Public documentation
│   └── ARCHITECTURE.md        # Architecture overview
├── CHANGELOG.md
├── ROADMAP.md
└── CONTRIBUTING.md
```

---

## Ecosystem

GoGPU is a multi-repo ecosystem. Changes may span repositories:

| Repository | Purpose | Version |
|------------|---------|---------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | App framework, windowing | v0.54.0 |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU | v0.34.2 |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics | v0.52.5 |
| [gogpu/naga](https://github.com/gogpu/naga) | Shader compiler | v0.19.0 |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit | v0.1.54 |
| [gogpu/gpucontext](https://github.com/gogpu/gpucontext) | Shared interfaces | v0.31.3 |
| [gogpu/gputypes](https://github.com/gogpu/gputypes) | WebGPU type definitions | v0.8.0 |

For cross-repo changes: start with the lowest dependency (gputypes → gpucontext → wgpu → gogpu/gg → ui).

---

## Platform Support

| Platform | Windowing | GPU Backends | Status |
|----------|-----------|-------------|--------|
| Windows | Win32 | Vulkan, DX12, GLES, Software | Production |
| Linux X11 | Pure Go X11 wire protocol | Vulkan, GLES, Software | Production |
| Linux Wayland | libwayland FFI, xdg-shell v6 | Vulkan, GLES, Software | Production |
| macOS | Cocoa (goffi ObjC runtime) | Metal, Software | Production |
| Browser | WASM (syscall/js, requestAnimationFrame) | WebGPU | Production |

---

## Testing

### Run All Tests

```bash
go test ./...
```

### Run Specific Package

```bash
go test -v ./internal/platform/...
```

### Run with Race Detector

```bash
go test -race ./...
```

### Golden Image Tests

```bash
go test -v ./golden/...
```

### Backend Smoke Test (visual)

```bash
GOGPU_GRAPHICS_API=vulkan   go run examples/triangle/main.go
GOGPU_GRAPHICS_API=dx12     go run examples/triangle/main.go
GOGPU_GRAPHICS_API=gles     go run examples/triangle/main.go
GOGPU_GRAPHICS_API=software go run examples/triangle/main.go
```

---

## AI-Assisted Contributions (Smart Coding)

We welcome AI-assisted contributions. GoGPU itself is built with AI assistance (Claude Code + multi-agent architecture). However, all code — whether human-written or AI-assisted — must meet the same enterprise quality standard:

- **Understand what the code does.** Don't submit code you can't explain. AI-generated code must be reviewed and understood by the submitter.
- **Validate against enterprise references.** Architectural decisions should reference how Skia, Qt6, SDL3, Rust wgpu, or other enterprise libraries solve the same problem.
- **No stubs presented as complete.** Every feature claimed in CHANGELOG must have working implementation, not just types/interfaces.
- **Research before implementation.** For GPU/HAL/sync changes: study the reference implementation first, then implement.

Smart Coding = AI accelerates the work, human ensures the quality. Vibe coding (ship without understanding) is not accepted.

---

## Areas Where We Need Help

- **Platform Testing** — Linux Wayland (GNOME, KDE, sway, Hyprland), macOS (Intel + Apple Silicon), Windows DX12/GLES
- **Android** — arm64 Vulkan WSI (see wgpu#268)
- **Browser/WASM** — WebGPU testing across browsers
- **Documentation** — Examples, tutorials, API docs
- **Performance** — Profiling, benchmarks, optimization

---

## Questions?

- Open a [GitHub Issue](https://github.com/gogpu/gogpu/issues)
- Check existing [Discussions](https://github.com/gogpu/gogpu/discussions)

---

*Thank you for contributing to GoGPU!*
