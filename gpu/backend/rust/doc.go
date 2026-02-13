// Package rust provides the HAL backend using wgpu-native (Rust) via go-webgpu/webgpu.
//
// This backend implements the hal.Backend interface by wrapping go-webgpu/webgpu types
// in thin adapter structs. It offers maximum performance but requires:
//   - Windows OS (due to go-webgpu/goffi limitations)
//   - wgpu_native.dll (the wgpu-native shared library)
//
// # Build Tags
//
// The rust backend is opt-in. Build with -tags rust to enable:
//
//	go build -tags rust ./...
//
// Without the rust tag, only the native (Pure Go) backend is available.
//
// # Requirements
//
// Download wgpu_native.dll from:
// https://github.com/gfx-rs/wgpu-native/releases
//
// Place it in your project directory or a directory in your PATH.
//
// # HAL Integration
//
// The renderer uses this package via three entry points:
//   - NewHalBackend() — returns a hal.Backend implementation
//   - HalBackendName() — returns "Rust (wgpu-native)"
//   - HalBackendVariant() — returns gputypes.BackendVulkan
package rust
