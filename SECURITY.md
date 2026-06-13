# Security Policy

## Supported Versions

GoGPU follows semantic versioning. Security fixes are applied to the latest release.

| Version | Supported          |
| ------- | ------------------ |
| 0.41.x  | :white_check_mark: |
| < 0.41  | :x:                |

## Reporting a Vulnerability

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues via:

1. **Private Security Advisory** (preferred):
   https://github.com/gogpu/gogpu/security/advisories/new

2. **GitHub Discussions** (for less critical issues):
   https://github.com/gogpu/gogpu/discussions

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

### Response Timeline

- **Initial Response**: Within 72 hours
- **Fix & Disclosure**: Coordinated with reporter

## Security Considerations

GoGPU uses platform libraries via FFI (goffi). Users should be aware of:

1. **Native Library Loading** — Pure Go backend loads platform libraries at runtime via `dlopen`: `libwayland-client.so` (Linux), `libvulkan.so`, `libEGL.so`, `user32.dll` (Windows), Cocoa frameworks (macOS). Rust backend (optional, `-tags rust`) loads `wgpu-native` shared library
2. **GPU Memory** — Ensure proper resource cleanup (`Destroy()` or `TrackResource()`) to avoid GPU memory leaks
3. **Shader Code** — WGSL shaders are compiled by naga (Pure Go) or wgpu-native (Rust backend)
4. **Clipboard** — `ClipboardRead`/`ClipboardWrite` access system clipboard (X11 selection, Win32, Wayland `wl_data_device`)

## Security Contact

- **GitHub Security Advisory**: https://github.com/gogpu/gogpu/security/advisories/new
- **Public Issues**: https://github.com/gogpu/gogpu/issues

---

**Thank you for helping keep GoGPU secure!**
