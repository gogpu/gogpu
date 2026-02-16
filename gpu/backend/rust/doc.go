// Package rust provides the HAL backend using wgpu-native (Rust) via go-webgpu/webgpu.
//
// This backend implements the hal.Backend interface by wrapping go-webgpu/webgpu types
// in thin adapter structs. It offers maximum performance but requires the wgpu-native
// shared library:
//   - Windows: wgpu_native.dll
//   - macOS: libwgpu_native.dylib
//   - Linux: libwgpu_native.so
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
// Download the wgpu-native shared library for your platform from:
// https://github.com/gfx-rs/wgpu-native/releases
//
// Place it in your project directory or a directory in your PATH/DYLD_LIBRARY_PATH/LD_LIBRARY_PATH.
//
// # HAL Integration
//
// The renderer uses this package via three entry points:
//   - NewHalBackend() — returns a hal.Backend implementation
//   - HalBackendName() — returns "Rust (wgpu-native)"
//   - HalBackendVariant() — returns platform-appropriate backend (Vulkan on Windows/Linux, Metal on macOS)
package rust
