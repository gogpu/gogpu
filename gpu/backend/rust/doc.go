// Package rust provides the WebGPU backend using wgpu-gpu (Rust) via go-webgpu/webgpu.
//
// This backend offers maximum performance but requires:
//   - Windows OS (due to go-webgpu/goffi limitations)
//   - wgpu-gpu library (wgpu_native.dll)
//
// # Build Tags
//
// The rust backend is opt-in. Build with -tags rust to enable:
//
//	go build -tags rust ./...
//
// Without the rust tag, only the gpu (Pure Go) backend is available.
//
// # Requirements
//
// Download wgpu_native.dll from:
// https://github.com/gfx-rs/wgpu-native/releases
//
// Place it in your project directory or a directory in your PATH.
package rust
