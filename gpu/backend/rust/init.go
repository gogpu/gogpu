//go:build rust && windows

// Package rust provides the WebGPU backend using wgpu-native (Rust) via go-webgpu/webgpu.
// This backend offers maximum performance but requires wgpu_native.dll.
//
// Build with: go build -tags rust
//
// Currently only supported on Windows due to go-webgpu/goffi limitations.
//
// The renderer imports this package via build-tag-guarded files and calls:
//   - NewHalBackend() to get the hal.Backend
//   - HalBackendName() to get the human-readable name
//   - HalBackendVariant() to get the backend variant for instance creation
package rust
