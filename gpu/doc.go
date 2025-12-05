// Package gpu provides the core GPU abstraction layer for gogpu.
//
// This package defines types and interfaces for GPU operations following
// the WebGPU specification. It provides a Go-friendly API while maintaining
// compatibility with the underlying WebGPU implementation.
//
// # Architecture
//
// The gpu package is organized into several components:
//
//   - Device: Represents a GPU device and provides methods for creating resources
//   - Buffer: GPU memory buffer for vertex, index, uniform, and storage data
//   - Texture: 2D and 3D textures with various formats
//   - Pipeline: Render and compute pipelines
//   - Shader: Shader modules compiled from WGSL
//
// # Usage
//
// First, request an adapter and create a device:
//
//	adapter, err := gpu.RequestAdapter(gpu.RequestAdapterOptions{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	device, err := adapter.RequestDevice(nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Then create resources and render:
//
//	buffer := device.CreateBuffer(&gpu.BufferDescriptor{
//	    Size:  1024,
//	    Usage: gpu.BufferUsageVertex | gpu.BufferUsageCopyDst,
//	})
//
// # WebGPU Compatibility
//
// This package follows the WebGPU specification where applicable.
// Types and methods are named to match WebGPU conventions while
// following Go naming conventions (e.g., CreateBuffer instead of createBuffer).
package gpu
