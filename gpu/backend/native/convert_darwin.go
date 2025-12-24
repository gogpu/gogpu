//go:build darwin

package native

import (
	gogputypes "github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/types"
)

// convertTextureFormat converts gogpu TextureFormat to wgpu types.TextureFormat.
func convertTextureFormat(format gogputypes.TextureFormat) types.TextureFormat {
	// Both use the same underlying values from WebGPU spec
	return types.TextureFormat(format)
}

// convertPresentMode converts gogpu PresentMode to wgpu hal.PresentMode.
func convertPresentMode(mode gogputypes.PresentMode) hal.PresentMode {
	switch mode {
	case gogputypes.PresentModeImmediate:
		return hal.PresentModeImmediate
	case gogputypes.PresentModeMailbox:
		return hal.PresentModeMailbox
	case gogputypes.PresentModeFifo:
		return hal.PresentModeFifo
	default:
		return hal.PresentModeFifo // Default to FIFO (VSync)
	}
}

// convertTextureUsage converts gogpu TextureUsage to wgpu types.TextureUsage.
func convertTextureUsage(usage gogputypes.TextureUsage) types.TextureUsage {
	var result types.TextureUsage

	if usage&gogputypes.TextureUsageCopySrc != 0 {
		result |= types.TextureUsageCopySrc
	}
	if usage&gogputypes.TextureUsageCopyDst != 0 {
		result |= types.TextureUsageCopyDst
	}
	if usage&gogputypes.TextureUsageTextureBinding != 0 {
		result |= types.TextureUsageTextureBinding
	}
	if usage&gogputypes.TextureUsageStorageBinding != 0 {
		result |= types.TextureUsageStorageBinding
	}
	if usage&gogputypes.TextureUsageRenderAttachment != 0 {
		result |= types.TextureUsageRenderAttachment
	}

	return result
}

// convertLoadOp converts gogpu LoadOp to wgpu types.LoadOp.
func convertLoadOp(op gogputypes.LoadOp) types.LoadOp {
	switch op {
	case gogputypes.LoadOpLoad:
		return types.LoadOpLoad
	case gogputypes.LoadOpClear:
		return types.LoadOpClear
	default:
		return types.LoadOpClear
	}
}

// convertStoreOp converts gogpu StoreOp to wgpu types.StoreOp.
func convertStoreOp(op gogputypes.StoreOp) types.StoreOp {
	switch op {
	case gogputypes.StoreOpStore:
		return types.StoreOpStore
	case gogputypes.StoreOpDiscard:
		return types.StoreOpDiscard
	default:
		return types.StoreOpStore
	}
}

// convertPrimitiveTopology converts gogpu PrimitiveTopology to wgpu types.PrimitiveTopology.
func convertPrimitiveTopology(topology gogputypes.PrimitiveTopology) types.PrimitiveTopology {
	switch topology {
	case gogputypes.PrimitiveTopologyPointList:
		return types.PrimitiveTopologyPointList
	case gogputypes.PrimitiveTopologyLineList:
		return types.PrimitiveTopologyLineList
	case gogputypes.PrimitiveTopologyLineStrip:
		return types.PrimitiveTopologyLineStrip
	case gogputypes.PrimitiveTopologyTriangleList:
		return types.PrimitiveTopologyTriangleList
	case gogputypes.PrimitiveTopologyTriangleStrip:
		return types.PrimitiveTopologyTriangleStrip
	default:
		return types.PrimitiveTopologyTriangleList
	}
}

// convertFrontFace converts gogpu FrontFace to wgpu types.FrontFace.
func convertFrontFace(face gogputypes.FrontFace) types.FrontFace {
	switch face {
	case gogputypes.FrontFaceCCW:
		return types.FrontFaceCCW
	case gogputypes.FrontFaceCW:
		return types.FrontFaceCW
	default:
		return types.FrontFaceCCW
	}
}

// convertCullMode converts gogpu CullMode to wgpu types.CullMode.
func convertCullMode(mode gogputypes.CullMode) types.CullMode {
	switch mode {
	case gogputypes.CullModeNone:
		return types.CullModeNone
	case gogputypes.CullModeFront:
		return types.CullModeFront
	case gogputypes.CullModeBack:
		return types.CullModeBack
	default:
		return types.CullModeNone
	}
}

// convertBufferUsage converts gogpu BufferUsage to wgpu types.BufferUsage.
// Used by CreateBuffer (not yet fully implemented).
func convertBufferUsage(usage gogputypes.BufferUsage) types.BufferUsage { //nolint:unused
	var result types.BufferUsage

	if usage&gogputypes.BufferUsageMapRead != 0 {
		result |= types.BufferUsageMapRead
	}
	if usage&gogputypes.BufferUsageMapWrite != 0 {
		result |= types.BufferUsageMapWrite
	}
	if usage&gogputypes.BufferUsageCopySrc != 0 {
		result |= types.BufferUsageCopySrc
	}
	if usage&gogputypes.BufferUsageCopyDst != 0 {
		result |= types.BufferUsageCopyDst
	}
	if usage&gogputypes.BufferUsageIndex != 0 {
		result |= types.BufferUsageIndex
	}
	if usage&gogputypes.BufferUsageVertex != 0 {
		result |= types.BufferUsageVertex
	}
	if usage&gogputypes.BufferUsageUniform != 0 {
		result |= types.BufferUsageUniform
	}
	if usage&gogputypes.BufferUsageStorage != 0 {
		result |= types.BufferUsageStorage
	}
	if usage&gogputypes.BufferUsageIndirect != 0 {
		result |= types.BufferUsageIndirect
	}
	if usage&gogputypes.BufferUsageQueryResolve != 0 {
		result |= types.BufferUsageQueryResolve
	}

	return result
}

// convertIndexFormat converts gogpu IndexFormat to wgpu types.IndexFormat.
// Used by SetIndexBuffer (not yet fully implemented).
func convertIndexFormat(format gogputypes.IndexFormat) types.IndexFormat { //nolint:unused
	switch format {
	case gogputypes.IndexFormatUint16:
		return types.IndexFormatUint16
	case gogputypes.IndexFormatUint32:
		return types.IndexFormatUint32
	default:
		return types.IndexFormatUint16
	}
}

// convertAddressMode converts gogpu AddressMode to wgpu types.AddressMode.
// Used by CreateSampler (not yet fully implemented).
func convertAddressMode(mode gogputypes.AddressMode) types.AddressMode { //nolint:unused
	switch mode {
	case gogputypes.AddressModeRepeat:
		return types.AddressModeRepeat
	case gogputypes.AddressModeMirrorRepeat:
		return types.AddressModeMirrorRepeat
	case gogputypes.AddressModeClampToEdge:
		return types.AddressModeClampToEdge
	default:
		return types.AddressModeClampToEdge
	}
}

// convertFilterMode converts gogpu FilterMode to wgpu types.FilterMode.
// Used by CreateSampler (not yet fully implemented).
func convertFilterMode(mode gogputypes.FilterMode) types.FilterMode { //nolint:unused
	switch mode {
	case gogputypes.FilterModeNearest:
		return types.FilterModeNearest
	case gogputypes.FilterModeLinear:
		return types.FilterModeLinear
	default:
		return types.FilterModeLinear
	}
}

// convertMipmapFilterMode converts gogpu MipmapFilterMode to wgpu types.MipmapFilterMode.
// Used by CreateSampler (not yet fully implemented).
func convertMipmapFilterMode(mode gogputypes.MipmapFilterMode) types.MipmapFilterMode { //nolint:unused
	switch mode {
	case gogputypes.MipmapFilterModeNearest:
		return types.MipmapFilterModeNearest
	case gogputypes.MipmapFilterModeLinear:
		return types.MipmapFilterModeLinear
	default:
		return types.MipmapFilterModeLinear
	}
}

// convertShaderStage converts gogpu ShaderStage to wgpu types.ShaderStage.
// Used by CreateBindGroupLayout (not yet fully implemented).
func convertShaderStage(stage gogputypes.ShaderStage) types.ShaderStage { //nolint:unused
	var result types.ShaderStage

	if stage&gogputypes.ShaderStageVertex != 0 {
		result |= types.ShaderStageVertex
	}
	if stage&gogputypes.ShaderStageFragment != 0 {
		result |= types.ShaderStageFragment
	}
	if stage&gogputypes.ShaderStageCompute != 0 {
		result |= types.ShaderStageCompute
	}

	return result
}

// convertTextureDimension converts gogpu TextureDimension to wgpu types.TextureDimension.
// Used by CreateTexture (not yet fully implemented).
func convertTextureDimension(dim gogputypes.TextureDimension) types.TextureDimension { //nolint:unused
	switch dim {
	case gogputypes.TextureDimension1D:
		return types.TextureDimension1D
	case gogputypes.TextureDimension2D:
		return types.TextureDimension2D
	case gogputypes.TextureDimension3D:
		return types.TextureDimension3D
	default:
		return types.TextureDimension2D
	}
}

// convertTextureViewDimension converts gogpu TextureViewDimension to wgpu types.TextureViewDimension.
func convertTextureViewDimension(dim gogputypes.TextureViewDimension) types.TextureViewDimension {
	switch dim {
	case gogputypes.TextureViewDimensionUndefined:
		return types.TextureViewDimensionUndefined
	case gogputypes.TextureViewDimension1D:
		return types.TextureViewDimension1D
	case gogputypes.TextureViewDimension2D:
		return types.TextureViewDimension2D
	case gogputypes.TextureViewDimension2DArray:
		return types.TextureViewDimension2DArray
	case gogputypes.TextureViewDimensionCube:
		return types.TextureViewDimensionCube
	case gogputypes.TextureViewDimensionCubeArray:
		return types.TextureViewDimensionCubeArray
	case gogputypes.TextureViewDimension3D:
		return types.TextureViewDimension3D
	default:
		return types.TextureViewDimensionUndefined
	}
}

// convertTextureAspect converts gogpu TextureAspect to wgpu types.TextureAspect.
func convertTextureAspect(aspect gogputypes.TextureAspect) types.TextureAspect {
	switch aspect {
	case gogputypes.TextureAspectAll:
		return types.TextureAspectAll
	case gogputypes.TextureAspectStencilOnly:
		return types.TextureAspectStencilOnly
	case gogputypes.TextureAspectDepthOnly:
		return types.TextureAspectDepthOnly
	default:
		return types.TextureAspectAll
	}
}

// convertExtent3D converts gogpu Extent3D to hal.Extent3D.
// Used by WriteTexture (not yet fully implemented).
func convertExtent3D(extent gogputypes.Extent3D) *hal.Extent3D { //nolint:unused
	return &hal.Extent3D{
		Width:              extent.Width,
		Height:             extent.Height,
		DepthOrArrayLayers: extent.DepthOrArrayLayers,
	}
}

// convertOrigin3D converts gogpu Origin3D to hal.Origin3D.
// Used by WriteTexture (not yet fully implemented).
func convertOrigin3D(origin gogputypes.Origin3D) *hal.Origin3D { //nolint:unused
	return &hal.Origin3D{
		X: origin.X,
		Y: origin.Y,
		Z: origin.Z,
	}
}

// convertImageDataLayout converts gogpu ImageDataLayout to hal.ImageDataLayout.
// Used by WriteTexture (not yet fully implemented).
func convertImageDataLayout(layout gogputypes.ImageDataLayout) *hal.ImageDataLayout { //nolint:unused
	return &hal.ImageDataLayout{
		Offset:       layout.Offset,
		BytesPerRow:  layout.BytesPerRow,
		RowsPerImage: layout.RowsPerImage,
	}
}
