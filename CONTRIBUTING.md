# Contributing to GoGPU

Thank you for your interest in contributing to GoGPU! 🎉

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/gogpu`
3. Create a branch: `git checkout -b feat/your-feature`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit: `git commit -m "feat: add your feature"`
7. Push: `git push origin feat/your-feature`
8. Open a Pull Request

## Development Setup

```bash
# Clone the repository
git clone https://github.com/gogpu/gogpu
cd gogpu

# Install dependencies
go mod download

# Download wgpu-native (required for FFI backend)
# See: https://github.com/gfx-rs/wgpu-native/releases

# Run tests
go test ./...

# Run linter
golangci-lint run
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Use `golangci-lint` for linting
- Write tests for new functionality
- Document public APIs

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(component): add new feature
fix(component): fix bug
docs: update documentation
test: add tests
refactor: code refactoring
chore: maintenance tasks
```

Components: `gpu`, `window`, `input`, `math`, `examples`, `docs`, `ci`

## Pull Request Guidelines

- Keep PRs focused on a single change
- Update documentation if needed
- Add tests for new features
- Ensure all tests pass
- Reference related issues

## Reporting Issues

- Use GitHub Issues
- Include Go version and OS
- Provide minimal reproduction steps
- Include error messages and logs

## Questions?

Open a GitHub Discussion or reach out to maintainers.

---

Thank you for contributing! 🚀
