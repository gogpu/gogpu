// Package gpu provides core GPU abstraction for gogpu.
//
// This package wraps WebGPU functionality and provides a simplified API
// for GPU operations including device management, buffer creation,
// and render pipeline setup.
package gpu

import (
	"errors"
)

// Common errors
var (
	ErrNoAdapter       = errors.New("gpu: no suitable adapter found")
	ErrDeviceCreation  = errors.New("gpu: failed to create device")
	ErrSurfaceCreation = errors.New("gpu: failed to create surface")
)

// Backend represents the underlying graphics API.
type Backend uint8

const (
	BackendAuto Backend = iota
	BackendVulkan
	BackendMetal
	BackendDX12
	BackendOpenGL
)

// String returns the backend name.
func (b Backend) String() string {
	switch b {
	case BackendVulkan:
		return "Vulkan"
	case BackendMetal:
		return "Metal"
	case BackendDX12:
		return "DX12"
	case BackendOpenGL:
		return "OpenGL"
	default:
		return "Auto"
	}
}

// Features represents optional GPU features.
type Features struct {
	// Core features
	DepthClipControl        bool
	Depth32FloatStencil8    bool
	TextureCompressionBC    bool
	TextureCompressionETC2  bool
	TextureCompressionASTC  bool
	IndirectFirstInstance   bool
	ShaderFloat16           bool
	RG11B10UfloatRenderable bool
	BGRA8UnormStorage       bool
	Float32Filterable       bool
}

// Limits represents GPU resource limits.
type Limits struct {
	MaxTextureDimension1D                     uint32
	MaxTextureDimension2D                     uint32
	MaxTextureDimension3D                     uint32
	MaxTextureArrayLayers                     uint32
	MaxBindGroups                             uint32
	MaxBindGroupsPlusVertexBuffers            uint32
	MaxBindingsPerBindGroup                   uint32
	MaxDynamicUniformBuffersPerPipelineLayout uint32
	MaxDynamicStorageBuffersPerPipelineLayout uint32
	MaxSampledTexturesPerShaderStage          uint32
	MaxSamplersPerShaderStage                 uint32
	MaxStorageBuffersPerShaderStage           uint32
	MaxStorageTexturesPerShaderStage          uint32
	MaxUniformBuffersPerShaderStage           uint32
	MaxUniformBufferBindingSize               uint64
	MaxStorageBufferBindingSize               uint64
	MinUniformBufferOffsetAlignment           uint32
	MinStorageBufferOffsetAlignment           uint32
	MaxVertexBuffers                          uint32
	MaxBufferSize                             uint64
	MaxVertexAttributes                       uint32
	MaxVertexBufferArrayStride                uint32
	MaxInterStageShaderComponents             uint32
	MaxInterStageShaderVariables              uint32
	MaxColorAttachments                       uint32
	MaxColorAttachmentBytesPerSample          uint32
	MaxComputeWorkgroupStorageSize            uint32
	MaxComputeInvocationsPerWorkgroup         uint32
	MaxComputeWorkgroupSizeX                  uint32
	MaxComputeWorkgroupSizeY                  uint32
	MaxComputeWorkgroupSizeZ                  uint32
	MaxComputeWorkgroupsPerDimension          uint32
}

// DefaultLimits returns WebGPU default limits.
func DefaultLimits() Limits {
	return Limits{
		MaxTextureDimension1D:                     8192,
		MaxTextureDimension2D:                     8192,
		MaxTextureDimension3D:                     2048,
		MaxTextureArrayLayers:                     256,
		MaxBindGroups:                             4,
		MaxBindGroupsPlusVertexBuffers:            24,
		MaxBindingsPerBindGroup:                   1000,
		MaxDynamicUniformBuffersPerPipelineLayout: 8,
		MaxDynamicStorageBuffersPerPipelineLayout: 4,
		MaxSampledTexturesPerShaderStage:          16,
		MaxSamplersPerShaderStage:                 16,
		MaxStorageBuffersPerShaderStage:           8,
		MaxStorageTexturesPerShaderStage:          4,
		MaxUniformBuffersPerShaderStage:           12,
		MaxUniformBufferBindingSize:               65536,
		MaxStorageBufferBindingSize:               134217728,
		MinUniformBufferOffsetAlignment:           256,
		MinStorageBufferOffsetAlignment:           256,
		MaxVertexBuffers:                          8,
		MaxBufferSize:                             268435456,
		MaxVertexAttributes:                       16,
		MaxVertexBufferArrayStride:                2048,
		MaxInterStageShaderComponents:             60,
		MaxInterStageShaderVariables:              16,
		MaxColorAttachments:                       8,
		MaxColorAttachmentBytesPerSample:          32,
		MaxComputeWorkgroupStorageSize:            16384,
		MaxComputeInvocationsPerWorkgroup:         256,
		MaxComputeWorkgroupSizeX:                  256,
		MaxComputeWorkgroupSizeY:                  256,
		MaxComputeWorkgroupSizeZ:                  64,
		MaxComputeWorkgroupsPerDimension:          65535,
	}
}
