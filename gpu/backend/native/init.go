// Package native provides the HAL backend using pure Go (gogpu/wgpu).
// This is the default backend, always available without external dependencies.
//
// Supports: Windows (Vulkan, DX12), Linux (Vulkan), macOS (Metal)
//
// The renderer imports this package directly and calls:
//   - NewHalBackend(api) to get the hal.Backend, name, and variant
//
// GraphicsAPI selection is supported on Windows (Vulkan or DX12).
// Other platforms have a single API and ignore the parameter.
package native
