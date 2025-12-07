# Security Policy

## Supported Versions

GoGPU is currently in early development (v0.x.x).

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| < 0.2.0 | :x:                |

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

GoGPU uses native GPU libraries via FFI. Users should be aware of:

1. **Native Library Loading** - Rust backend loads wgpu-native DLL/so/dylib
2. **GPU Memory** - Ensure proper resource cleanup to avoid GPU memory leaks
3. **Shader Code** - WGSL shaders are compiled by wgpu-native

## Security Contact

- **GitHub Security Advisory**: https://github.com/gogpu/gogpu/security/advisories/new
- **Public Issues**: https://github.com/gogpu/gogpu/issues

---

**Thank you for helping keep GoGPU secure!**
