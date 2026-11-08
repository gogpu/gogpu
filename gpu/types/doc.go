// Package types defines graphics API and power-preference configuration types
// for gogpu.
//
// BackendType is kept for code that still refers to it; gogpu itself no longer
// uses it. Backend implementation selection happens in wgpu build tags, and
// gogpu.Config.GraphicsAPI or GOGPU_GRAPHICS_API choose the graphics API at
// runtime.
//
// For WebGPU resource types (textures, buffers, and pipelines), use the
// concrete API from github.com/gogpu/wgpu or shared values from
// github.com/gogpu/gputypes.
package types
