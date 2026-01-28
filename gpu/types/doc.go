// Package types defines GPU types and handles used throughout gogpu.
//
// # Architecture
//
// This package provides:
//   - Handle types: Opaque references to GPU resources (Instance, Device, Texture, etc.)
//   - BackendType: Enum for selecting Rust vs Pure Go backend
//   - Re-exports of gputypes: WebGPU types for backward compatibility
//
// # Type Sources
//
// Types in this package come from two sources:
//
// 1. Gogpu-specific (defined here):
//   - All Handle types (Instance, Adapter, Device, Queue, Surface, etc.)
//   - BackendType (Auto, Native, Rust)
//   - SurfaceHandle, SurfaceTexture, SurfaceStatus
//   - Gogpu-specific descriptors that use handles
//
// 2. WebGPU standard (re-exported from gputypes):
//   - TextureFormat, TextureUsage, TextureDimension, etc.
//   - BufferUsage, BufferDescriptor, etc.
//   - BlendState, BlendComponent, BlendFactor, etc.
//   - All WebGPU spec constants and types
//
// # Migration to gputypes
//
// For new code, prefer importing gputypes directly:
//
//	import "github.com/gogpu/gputypes"
//
// This package re-exports all gputypes types for backward compatibility
// with existing code that imports github.com/gogpu/gogpu/gpu/types.
//
// # Usage Example
//
//	import "github.com/gogpu/gogpu/gpu/types"
//
//	// Using handle types (gogpu-specific)
//	var device types.Device
//	var texture types.Texture
//
//	// Using WebGPU types (from gputypes)
//	format := types.TextureFormatRGBA8Unorm
//	usage := types.TextureUsageTextureBinding | types.TextureUsageCopyDst
package types
