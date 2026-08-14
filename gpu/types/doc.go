// Package types defines graphics API and power-preference configuration types
// for gogpu.
//
// BackendType is retained for source compatibility with the deprecated
// gogpu.Config.Backend field. Backend implementation selection now happens in
// wgpu build tags; use gogpu.Config.GraphicsAPI or GOGPU_GRAPHICS_API to
// choose a graphics API at runtime.
//
// For WebGPU resource types (textures, buffers, and pipelines), use the
// concrete API from github.com/gogpu/wgpu or shared values from
// github.com/gogpu/gputypes.
package types
