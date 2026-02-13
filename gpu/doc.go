// Package gpu provides backend selection and GPU types for gogpu.
//
// # Architecture
//
// The renderer uses wgpu HAL interfaces (hal.Device, hal.Queue, etc.) directly.
// Backend selection determines which HAL implementation to use:
//
//   - Native (Pure Go): Uses gogpu/wgpu with Vulkan (Windows/Linux) or Metal (macOS)
//   - Rust: Uses wgpu-native via go-webgpu/webgpu (Windows only, build with -tags rust)
//
// # Backend Selection
//
// By default, gogpu uses the Pure Go backend. Users can choose explicitly:
//
//	config := gogpu.DefaultConfig()                              // Auto-select
//	config := gogpu.DefaultConfig().WithBackend(gogpu.BackendRust)   // Rust backend
//	config := gogpu.DefaultConfig().WithBackend(gogpu.BackendNative) // Pure Go backend
//
// # Subpackages
//
//   - gpu/types: BackendType enum and related constants
//   - gpu/backend/native: Pure Go HAL backend (Vulkan/Metal)
//   - gpu/backend/rust: Rust HAL backend (wgpu-native, Windows only)
package gpu
