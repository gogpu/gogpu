// Package types provides WebGPU type definitions for the gogpu ecosystem.
//
// This package is designed to be standalone with no external dependencies,
// following the pattern of wgpu-types in the Rust wgpu ecosystem.
// It can be imported by any package without causing circular dependencies.
//
// # Organization
//
// Types are organized into several categories:
//
//   - Handles: Opaque references to GPU objects (Instance, Device, Texture, etc.)
//   - Enums: Enumeration types (TextureFormat, PresentMode, LoadOp, etc.)
//   - Descriptors: Configuration structs (SurfaceConfig, RenderPipelineDescriptor, etc.)
//
// # Usage
//
// Import this package to use WebGPU types:
//
//	import "github.com/gogpu/gogpu/gpu/types"
//
//	config := &types.SurfaceConfig{
//	    Format:      types.TextureFormatBGRA8Unorm,
//	    Usage:       types.TextureUsageRenderAttachment,
//	    Width:       800,
//	    Height:      600,
//	    PresentMode: types.PresentModeFifo,
//	}
//
// # WebGPU Alignment
//
// All types and constants are aligned with the WebGPU specification
// (https://www.w3.org/TR/webgpu/). Numeric values for enums match
// the WebGPU C header (webgpu.h) for compatibility with wgpu-native.
package types
