// Package gpu provides GPU types for gogpu.
//
// # Architecture (ADR-038)
//
// Backend selection (Native Go / Rust FFI / Browser) is handled inside wgpu
// via build tags. gogpu always uses the wgpu public API — the implementation
// is transparent.
//
//   - Default: Pure Go (wgpu/core → wgpu/hal → Vulkan/Metal/DX12/GLES/Software)
//   - -tags rust: Rust FFI (go-webgpu/webgpu → wgpu-native)
//   - js,wasm: Browser (syscall/js → Browser WebGPU)
//
// # Subpackages
//
//   - gpu/types: BackendType enum and related constants
//   - gpu/backend/native: Native HAL backend registration
package gpu
