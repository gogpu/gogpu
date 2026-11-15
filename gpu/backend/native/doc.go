// Package native links the wgpu backends of the pure Go implementation and maps
// gogpu's GraphicsAPI selector to the backend mask that wgpu.CreateInstance
// needs. gogpu has no hardware abstraction of its own: the HAL lives in
// wgpu/hal, and the per-platform files here only import its backends so that
// their init() calls hal.RegisterBackend and wgpu can discover the linked
// implementations. The linked set is chosen by build tags (ADR-038).
//
//   - Windows: DX12, Vulkan, GLES, software
//   - Linux: Vulkan, GLES, software
//   - macOS: Metal, software
//   - js/wasm: none, the browser provides navigator.gpu directly
//
// The renderer calls BackendInfo to get the display name and mask for the
// selected API, then creates the wgpu instance.
package native
