// Package native provides the HAL backend using pure Go (gogpu/wgpu).
// This is the default backend, always available without external dependencies.
//
// Supports: Windows (Vulkan), Linux (Vulkan), macOS (Metal)
//
// The renderer imports this package directly and calls:
//   - NewHalBackend() to get the hal.Backend
//   - HalBackendName() to get the human-readable name
//   - HalBackendVariant() to get the backend variant for instance creation
package native
