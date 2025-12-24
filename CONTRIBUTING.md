# Contributing to GoGPU

Thank you for your interest in contributing to GoGPU!

---

## Requirements

- **Go 1.25+** (required for iterators, generics, and modern features)
- **golangci-lint** for code quality checks
- **wgpu-native** (optional, for Rust backend testing)

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

### 2. Create Feature Branch

```bash
git checkout -b feat/your-feature
# or
git checkout -b fix/issue-number-description
```

### 3. Make Changes

- Follow code style guidelines below
- Add tests for new functionality
- Update documentation if needed

### 4. Validate Before Commit

```bash
# Format code
go fmt ./...

# Run pre-release checks
bash scripts/pre-release-check.sh
```

### 5. Create Pull Request

**All contributions must go through Pull Requests:**

```bash
git add .
git commit -m "feat(component): description"
git push origin feat/your-feature
```

Then open a PR on GitHub: `https://github.com/gogpu/gogpu/compare`

---

## Pull Request Guidelines

### PR Requirements

- [ ] All tests pass (`go test ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Documentation updated (if applicable)
- [ ] CHANGELOG.md updated (for features/fixes)

### PR Title Format

```
feat(gpu): add Metal backend support
fix(platform): resolve macOS window sizing issue
docs: update ROADMAP for v0.7.0
test(backend): add smoke tests for native backend
```

### PR Description Template

```markdown
## Summary
Brief description of changes.

## Changes
- Change 1
- Change 2

## Testing
How was this tested?

## Related Issues
Closes #123
```

---

## Code Style

### Go Conventions

- Use `gofmt` for formatting (tabs, not spaces)
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use pointer receivers for structs with mutexes

### Naming

| Type | Convention | Example |
|------|------------|---------|
| Exported | PascalCase | `CreateSurface` |
| Unexported | camelCase | `handleEvent` |
| Acronyms | Uppercase | `GetHTTPURL`, `DeviceID` |
| Constants | PascalCase | `MaxTextureSize` |

### Error Handling

```go
// Always check errors
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Or explicitly ignore
_ = file.Close()
```

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description

[optional body]

[optional footer]
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
| `chore` | Maintenance |

### Scopes

| Scope | Description |
|-------|-------------|
| `gpu` | GPU backend |
| `platform` | Platform code (Win32, Cocoa, X11, Wayland) |
| `backend` | Native/Rust backend |
| `gmath` | Math library |
| `window` | Window management |
| `input` | Input handling |
| `examples` | Example code |
| `deps` | Dependencies |

---

## Project Structure

```
gogpu/
├── gpu/                    # GPU abstraction layer
│   ├── backend.go          # Backend interface
│   ├── types/              # WebGPU types
│   └── backend/
│       ├── rust/           # Rust backend (wgpu-native)
│       └── native/         # Pure Go backend (gogpu/wgpu)
├── internal/platform/      # Platform-specific code
│   ├── windows/            # Win32
│   ├── darwin/             # macOS Cocoa
│   ├── wayland/            # Linux Wayland
│   └── x11/                # Linux X11
├── gmath/                  # Math primitives
├── window/                 # Window configuration
├── input/                  # Input types
├── examples/               # Example applications
└── scripts/                # Build/release scripts
```

---

## Platform Support

| Platform | Windowing | Pure Go Backend | Status |
|----------|-----------|-----------------|--------|
| Windows | Win32 | Vulkan | Production |
| Linux X11 | X11 | Vulkan | Community Testing |
| Linux Wayland | Wayland | Vulkan | Community Testing |
| macOS | Cocoa | Metal | Community Testing |

---

## Testing

### Run All Tests

```bash
go test ./...
```

### Run Specific Package

```bash
go test -v ./gpu/backend/native/...
```

### Run with Race Detector

```bash
go test -race ./...
```

### Pre-Release Validation

```bash
bash scripts/pre-release-check.sh
```

---

## Areas Where We Need Help

- **Platform Testing** — Test on Linux X11, Wayland, macOS
- **DX12 Backend** — Windows DirectX 12 implementation
- **Documentation** — Examples, tutorials, API docs
- **Performance** — Profiling, optimization

---

## Questions?

- Open a [GitHub Issue](https://github.com/gogpu/gogpu/issues)
- Check existing [Discussions](https://github.com/gogpu/gogpu/discussions)

---

*Thank you for contributing to GoGPU!*
