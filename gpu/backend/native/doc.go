// Package native maps gogpu's GraphicsAPI selector to the backend mask that
// wgpu.CreateInstance needs. gogpu has no hardware abstraction of its own: the
// backends live inside wgpu and are registered by default (ADR-070), so no
// blank imports are needed. The linked set is chosen by build tags (ADR-038).
//
//   - Windows: DX12, Vulkan, GLES, software
//   - Linux: Vulkan, GLES, software
//   - macOS: Metal, software
//   - js/wasm: none, the browser provides navigator.gpu directly
//
// The renderer calls BackendInfo to get the display name and mask for the
// selected API, then creates the wgpu instance.
package native
