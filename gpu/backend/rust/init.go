//go:build rust

// Package rust provides the WebGPU backend using wgpu-native (Rust) via go-webgpu/webgpu.
// This backend offers maximum performance but requires the wgpu-native shared library
// (wgpu_native.dll on Windows, libwgpu_native.dylib on macOS, libwgpu_native.so on Linux).
//
// Build with: go build -tags rust
//
// The renderer imports this package via build-tag-guarded files and calls:
//   - NewHalBackend() to get the hal.Backend
//   - HalBackendName() to get the human-readable name
//   - HalBackendVariant() to get the backend variant for instance creation
package rust
