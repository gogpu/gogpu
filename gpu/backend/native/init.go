// Package native provides HAL backend registration for the pure Go (gogpu/wgpu) path.
// This is the default backend, always available without external dependencies.
//
// Supports: Windows (Vulkan, DX12), Linux (Vulkan), macOS (Metal)
//
// Importing this package triggers HAL backend registration via init() side effects.
// The renderer calls BackendInfo(api) to get the display name and variant mask,
// then uses wgpu.CreateInstance() which discovers the registered backends.
package native
