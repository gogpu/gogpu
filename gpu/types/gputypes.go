package types

// Re-export gputypes for backward compatibility.
// This allows existing code importing github.com/gogpu/gogpu/gpu/types
// to continue working while internally using gputypes.
//
// IMPORTANT: Types that reference handles (BindGroupDescriptor, etc.) are
// defined separately in this package because gogpu uses typed handles
// (types.Buffer, types.TextureView) while gputypes uses raw uintptr.

import "github.com/gogpu/gputypes"

// Type aliases for WebGPU value types from gputypes package.
// These provide backward compatibility for existing code.
type (
	// Texture types
	TextureFormat        = gputypes.TextureFormat
	TextureUsage         = gputypes.TextureUsage
	TextureDimension     = gputypes.TextureDimension
	TextureViewDimension = gputypes.TextureViewDimension
	TextureAspect        = gputypes.TextureAspect
	TextureSampleType    = gputypes.TextureSampleType

	// Buffer types
	BufferUsage       = gputypes.BufferUsage
	BufferBindingType = gputypes.BufferBindingType
	IndexFormat       = gputypes.IndexFormat

	// Sampler types
	AddressMode        = gputypes.AddressMode
	FilterMode         = gputypes.FilterMode
	MipmapFilterMode   = gputypes.MipmapFilterMode
	CompareFunction    = gputypes.CompareFunction
	SamplerBindingType = gputypes.SamplerBindingType

	// Render types
	LoadOp            = gputypes.LoadOp
	StoreOp           = gputypes.StoreOp
	BlendFactor       = gputypes.BlendFactor
	BlendOperation    = gputypes.BlendOperation
	BlendComponent    = gputypes.BlendComponent
	BlendState        = gputypes.BlendState
	ColorWriteMask    = gputypes.ColorWriteMask
	PrimitiveTopology = gputypes.PrimitiveTopology
	FrontFace         = gputypes.FrontFace
	CullMode          = gputypes.CullMode

	// Shader types
	ShaderStage  = gputypes.ShaderStage
	ShaderStages = gputypes.ShaderStages

	// Binding layout types (no handles)
	BufferBindingLayout  = gputypes.BufferBindingLayout
	SamplerBindingLayout = gputypes.SamplerBindingLayout
	TextureBindingLayout = gputypes.TextureBindingLayout

	// Vertex types
	VertexFormat   = gputypes.VertexFormat
	VertexStepMode = gputypes.VertexStepMode

	// Geometry types
	Extent3D = gputypes.Extent3D
	Origin3D = gputypes.Origin3D
	Color    = gputypes.Color

	// Surface types
	PresentMode        = gputypes.PresentMode
	CompositeAlphaMode = gputypes.CompositeAlphaMode

	// Adapter types
	PowerPreference = gputypes.PowerPreference
)

// Re-export texture format constants.
const (
	TextureFormatUndefined      = gputypes.TextureFormatUndefined
	TextureFormatR8Unorm        = gputypes.TextureFormatR8Unorm
	TextureFormatR8Snorm        = gputypes.TextureFormatR8Snorm
	TextureFormatR8Uint         = gputypes.TextureFormatR8Uint
	TextureFormatR8Sint         = gputypes.TextureFormatR8Sint
	TextureFormatR16Uint        = gputypes.TextureFormatR16Uint
	TextureFormatR16Sint        = gputypes.TextureFormatR16Sint
	TextureFormatR16Float       = gputypes.TextureFormatR16Float
	TextureFormatRG8Unorm       = gputypes.TextureFormatRG8Unorm
	TextureFormatRG8Snorm       = gputypes.TextureFormatRG8Snorm
	TextureFormatRG8Uint        = gputypes.TextureFormatRG8Uint
	TextureFormatRG8Sint        = gputypes.TextureFormatRG8Sint
	TextureFormatR32Uint        = gputypes.TextureFormatR32Uint
	TextureFormatR32Sint        = gputypes.TextureFormatR32Sint
	TextureFormatR32Float       = gputypes.TextureFormatR32Float
	TextureFormatRG16Uint       = gputypes.TextureFormatRG16Uint
	TextureFormatRG16Sint       = gputypes.TextureFormatRG16Sint
	TextureFormatRG16Float      = gputypes.TextureFormatRG16Float
	TextureFormatRGBA8Unorm     = gputypes.TextureFormatRGBA8Unorm
	TextureFormatRGBA8UnormSrgb = gputypes.TextureFormatRGBA8UnormSrgb
	TextureFormatRGBA8Snorm     = gputypes.TextureFormatRGBA8Snorm
	TextureFormatRGBA8Uint      = gputypes.TextureFormatRGBA8Uint
	TextureFormatRGBA8Sint      = gputypes.TextureFormatRGBA8Sint
	TextureFormatBGRA8Unorm     = gputypes.TextureFormatBGRA8Unorm
	TextureFormatBGRA8UnormSrgb = gputypes.TextureFormatBGRA8UnormSrgb
	TextureFormatDepth16Unorm   = gputypes.TextureFormatDepth16Unorm
	TextureFormatDepth24Plus    = gputypes.TextureFormatDepth24Plus
	TextureFormatDepth32Float   = gputypes.TextureFormatDepth32Float
)

// Re-export texture usage constants.
const (
	TextureUsageNone             = gputypes.TextureUsageNone
	TextureUsageCopySrc          = gputypes.TextureUsageCopySrc
	TextureUsageCopyDst          = gputypes.TextureUsageCopyDst
	TextureUsageTextureBinding   = gputypes.TextureUsageTextureBinding
	TextureUsageStorageBinding   = gputypes.TextureUsageStorageBinding
	TextureUsageRenderAttachment = gputypes.TextureUsageRenderAttachment
)

// Re-export texture dimension constants.
const (
	TextureDimension1D = gputypes.TextureDimension1D
	TextureDimension2D = gputypes.TextureDimension2D
	TextureDimension3D = gputypes.TextureDimension3D
)

// Re-export texture view dimension constants.
const (
	TextureViewDimensionUndefined = gputypes.TextureViewDimensionUndefined
	TextureViewDimension1D        = gputypes.TextureViewDimension1D
	TextureViewDimension2D        = gputypes.TextureViewDimension2D
	TextureViewDimension2DArray   = gputypes.TextureViewDimension2DArray
	TextureViewDimensionCube      = gputypes.TextureViewDimensionCube
	TextureViewDimensionCubeArray = gputypes.TextureViewDimensionCubeArray
	TextureViewDimension3D        = gputypes.TextureViewDimension3D
)

// Re-export texture aspect constants.
const (
	TextureAspectAll         = gputypes.TextureAspectAll
	TextureAspectStencilOnly = gputypes.TextureAspectStencilOnly
	TextureAspectDepthOnly   = gputypes.TextureAspectDepthOnly
)

// Re-export texture sample type constants.
const (
	TextureSampleTypeFloat             = gputypes.TextureSampleTypeFloat
	TextureSampleTypeUnfilterableFloat = gputypes.TextureSampleTypeUnfilterableFloat
	TextureSampleTypeDepth             = gputypes.TextureSampleTypeDepth
	TextureSampleTypeSint              = gputypes.TextureSampleTypeSint
	TextureSampleTypeUint              = gputypes.TextureSampleTypeUint
)

// Re-export buffer usage constants.
const (
	BufferUsageNone         = gputypes.BufferUsageNone
	BufferUsageMapRead      = gputypes.BufferUsageMapRead
	BufferUsageMapWrite     = gputypes.BufferUsageMapWrite
	BufferUsageCopySrc      = gputypes.BufferUsageCopySrc
	BufferUsageCopyDst      = gputypes.BufferUsageCopyDst
	BufferUsageIndex        = gputypes.BufferUsageIndex
	BufferUsageVertex       = gputypes.BufferUsageVertex
	BufferUsageUniform      = gputypes.BufferUsageUniform
	BufferUsageStorage      = gputypes.BufferUsageStorage
	BufferUsageIndirect     = gputypes.BufferUsageIndirect
	BufferUsageQueryResolve = gputypes.BufferUsageQueryResolve
)

// Re-export buffer binding type constants.
const (
	BufferBindingTypeUndefined       = gputypes.BufferBindingTypeUndefined
	BufferBindingTypeUniform         = gputypes.BufferBindingTypeUniform
	BufferBindingTypeStorage         = gputypes.BufferBindingTypeStorage
	BufferBindingTypeReadOnlyStorage = gputypes.BufferBindingTypeReadOnlyStorage
)

// Re-export index format constants.
const (
	IndexFormatUint16 = gputypes.IndexFormatUint16
	IndexFormatUint32 = gputypes.IndexFormatUint32
)

// Re-export address mode constants.
const (
	AddressModeClampToEdge  = gputypes.AddressModeClampToEdge
	AddressModeRepeat       = gputypes.AddressModeRepeat
	AddressModeMirrorRepeat = gputypes.AddressModeMirrorRepeat
)

// Re-export filter mode constants.
const (
	FilterModeNearest = gputypes.FilterModeNearest
	FilterModeLinear  = gputypes.FilterModeLinear
)

// Re-export mipmap filter mode constants.
const (
	MipmapFilterModeNearest = gputypes.MipmapFilterModeNearest
	MipmapFilterModeLinear  = gputypes.MipmapFilterModeLinear
)

// Re-export compare function constants.
const (
	CompareFunctionUndefined    = gputypes.CompareFunctionUndefined
	CompareFunctionNever        = gputypes.CompareFunctionNever
	CompareFunctionLess         = gputypes.CompareFunctionLess
	CompareFunctionEqual        = gputypes.CompareFunctionEqual
	CompareFunctionLessEqual    = gputypes.CompareFunctionLessEqual
	CompareFunctionGreater      = gputypes.CompareFunctionGreater
	CompareFunctionNotEqual     = gputypes.CompareFunctionNotEqual
	CompareFunctionGreaterEqual = gputypes.CompareFunctionGreaterEqual
	CompareFunctionAlways       = gputypes.CompareFunctionAlways
)

// Re-export sampler binding type constants.
const (
	SamplerBindingTypeUndefined    = gputypes.SamplerBindingTypeUndefined
	SamplerBindingTypeFiltering    = gputypes.SamplerBindingTypeFiltering
	SamplerBindingTypeNonFiltering = gputypes.SamplerBindingTypeNonFiltering
	SamplerBindingTypeComparison   = gputypes.SamplerBindingTypeComparison
)

// Re-export load/store op constants.
const (
	LoadOpClear    = gputypes.LoadOpClear
	LoadOpLoad     = gputypes.LoadOpLoad
	StoreOpDiscard = gputypes.StoreOpDiscard
	StoreOpStore   = gputypes.StoreOpStore
)

// Re-export blend factor constants.
const (
	BlendFactorZero              = gputypes.BlendFactorZero
	BlendFactorOne               = gputypes.BlendFactorOne
	BlendFactorSrc               = gputypes.BlendFactorSrc
	BlendFactorOneMinusSrc       = gputypes.BlendFactorOneMinusSrc
	BlendFactorSrcAlpha          = gputypes.BlendFactorSrcAlpha
	BlendFactorOneMinusSrcAlpha  = gputypes.BlendFactorOneMinusSrcAlpha
	BlendFactorDst               = gputypes.BlendFactorDst
	BlendFactorOneMinusDst       = gputypes.BlendFactorOneMinusDst
	BlendFactorDstAlpha          = gputypes.BlendFactorDstAlpha
	BlendFactorOneMinusDstAlpha  = gputypes.BlendFactorOneMinusDstAlpha
	BlendFactorSrcAlphaSaturated = gputypes.BlendFactorSrcAlphaSaturated
	BlendFactorConstant          = gputypes.BlendFactorConstant
	BlendFactorOneMinusConstant  = gputypes.BlendFactorOneMinusConstant
)

// Re-export blend operation constants.
const (
	BlendOperationAdd             = gputypes.BlendOperationAdd
	BlendOperationSubtract        = gputypes.BlendOperationSubtract
	BlendOperationReverseSubtract = gputypes.BlendOperationReverseSubtract
	BlendOperationMin             = gputypes.BlendOperationMin
	BlendOperationMax             = gputypes.BlendOperationMax
)

// Re-export color write mask constants.
const (
	ColorWriteMaskNone  = gputypes.ColorWriteMaskNone
	ColorWriteMaskRed   = gputypes.ColorWriteMaskRed
	ColorWriteMaskGreen = gputypes.ColorWriteMaskGreen
	ColorWriteMaskBlue  = gputypes.ColorWriteMaskBlue
	ColorWriteMaskAlpha = gputypes.ColorWriteMaskAlpha
	ColorWriteMaskAll   = gputypes.ColorWriteMaskAll
)

// Re-export primitive topology constants.
const (
	PrimitiveTopologyPointList     = gputypes.PrimitiveTopologyPointList
	PrimitiveTopologyLineList      = gputypes.PrimitiveTopologyLineList
	PrimitiveTopologyLineStrip     = gputypes.PrimitiveTopologyLineStrip
	PrimitiveTopologyTriangleList  = gputypes.PrimitiveTopologyTriangleList
	PrimitiveTopologyTriangleStrip = gputypes.PrimitiveTopologyTriangleStrip
)

// Re-export front face constants.
const (
	FrontFaceCCW = gputypes.FrontFaceCCW
	FrontFaceCW  = gputypes.FrontFaceCW
)

// Re-export cull mode constants.
const (
	CullModeNone  = gputypes.CullModeNone
	CullModeFront = gputypes.CullModeFront
	CullModeBack  = gputypes.CullModeBack
)

// Re-export shader stage constants.
const (
	ShaderStageNone     = gputypes.ShaderStageNone
	ShaderStageVertex   = gputypes.ShaderStageVertex
	ShaderStageFragment = gputypes.ShaderStageFragment
	ShaderStageCompute  = gputypes.ShaderStageCompute
)

// Re-export vertex format constants.
const (
	VertexFormatUint8x2   = gputypes.VertexFormatUint8x2
	VertexFormatUint8x4   = gputypes.VertexFormatUint8x4
	VertexFormatSint8x2   = gputypes.VertexFormatSint8x2
	VertexFormatSint8x4   = gputypes.VertexFormatSint8x4
	VertexFormatUnorm8x2  = gputypes.VertexFormatUnorm8x2
	VertexFormatUnorm8x4  = gputypes.VertexFormatUnorm8x4
	VertexFormatSnorm8x2  = gputypes.VertexFormatSnorm8x2
	VertexFormatSnorm8x4  = gputypes.VertexFormatSnorm8x4
	VertexFormatUint16x2  = gputypes.VertexFormatUint16x2
	VertexFormatUint16x4  = gputypes.VertexFormatUint16x4
	VertexFormatSint16x2  = gputypes.VertexFormatSint16x2
	VertexFormatSint16x4  = gputypes.VertexFormatSint16x4
	VertexFormatUnorm16x2 = gputypes.VertexFormatUnorm16x2
	VertexFormatUnorm16x4 = gputypes.VertexFormatUnorm16x4
	VertexFormatSnorm16x2 = gputypes.VertexFormatSnorm16x2
	VertexFormatSnorm16x4 = gputypes.VertexFormatSnorm16x4
	VertexFormatFloat16x2 = gputypes.VertexFormatFloat16x2
	VertexFormatFloat16x4 = gputypes.VertexFormatFloat16x4
	VertexFormatFloat32   = gputypes.VertexFormatFloat32
	VertexFormatFloat32x2 = gputypes.VertexFormatFloat32x2
	VertexFormatFloat32x3 = gputypes.VertexFormatFloat32x3
	VertexFormatFloat32x4 = gputypes.VertexFormatFloat32x4
	VertexFormatUint32    = gputypes.VertexFormatUint32
	VertexFormatUint32x2  = gputypes.VertexFormatUint32x2
	VertexFormatUint32x3  = gputypes.VertexFormatUint32x3
	VertexFormatUint32x4  = gputypes.VertexFormatUint32x4
	VertexFormatSint32    = gputypes.VertexFormatSint32
	VertexFormatSint32x2  = gputypes.VertexFormatSint32x2
	VertexFormatSint32x3  = gputypes.VertexFormatSint32x3
	VertexFormatSint32x4  = gputypes.VertexFormatSint32x4
)

// Re-export vertex step mode constants.
const (
	VertexStepModeVertex   = gputypes.VertexStepModeVertex
	VertexStepModeInstance = gputypes.VertexStepModeInstance
)

// Re-export present mode constants.
const (
	PresentModeAutoVsync   = gputypes.PresentModeAutoVsync
	PresentModeAutoNoVsync = gputypes.PresentModeAutoNoVsync
	PresentModeFifo        = gputypes.PresentModeFifo
	PresentModeFifoRelaxed = gputypes.PresentModeFifoRelaxed
	PresentModeImmediate   = gputypes.PresentModeImmediate
	PresentModeMailbox     = gputypes.PresentModeMailbox
)

// Re-export composite alpha mode constants.
const (
	CompositeAlphaModeAuto           = gputypes.CompositeAlphaModeAuto
	CompositeAlphaModeOpaque         = gputypes.CompositeAlphaModeOpaque
	CompositeAlphaModePreMultiplied  = gputypes.CompositeAlphaModePreMultiplied
	CompositeAlphaModePostMultiplied = gputypes.CompositeAlphaModePostMultiplied
	CompositeAlphaModeInherit        = gputypes.CompositeAlphaModeInherit
)

// Re-export power preference constants.
const (
	PowerPreferenceNone            = gputypes.PowerPreferenceNone
	PowerPreferenceLowPower        = gputypes.PowerPreferenceLowPower
	PowerPreferenceHighPerformance = gputypes.PowerPreferenceHighPerformance
)

// NewExtent2D creates an Extent3D for a 2D texture with 1 layer.
var NewExtent2D = gputypes.NewExtent2D

// NewExtent3D creates an Extent3D for a 3D texture.
var NewExtent3D = gputypes.NewExtent3D

// NewColor creates a new Color with the given RGBA values.
var NewColor = gputypes.NewColor

// NewColorRGB creates a new opaque Color with the given RGB values and alpha=1.0.
var NewColorRGB = gputypes.NewColorRGB

// Predefined colors.
var (
	ColorTransparent = gputypes.ColorTransparent
	ColorBlack       = gputypes.ColorBlack
	ColorWhite       = gputypes.ColorWhite
	ColorRed         = gputypes.ColorRed
	ColorGreen       = gputypes.ColorGreen
	ColorBlue        = gputypes.ColorBlue
)

// ============================================================================
// Gogpu-specific types that use typed handles
// These cannot be aliased from gputypes because they use different handle types.
// ============================================================================

// AdapterOptions configures adapter request.
type AdapterOptions struct {
	PowerPreference PowerPreference
}

// DeviceOptions configures device request.
type DeviceOptions struct {
	Label string
}

// SurfaceConfig configures surface presentation.
type SurfaceConfig struct {
	Format      TextureFormat
	Usage       TextureUsage
	Width       uint32
	Height      uint32
	PresentMode PresentMode
	AlphaMode   CompositeAlphaMode
}

// AlphaMode is an alias for CompositeAlphaMode for backward compatibility.
type AlphaMode = CompositeAlphaMode

// AlphaMode constants for backward compatibility.
const (
	AlphaModeOpaque         = CompositeAlphaModeOpaque
	AlphaModePremultiplied  = CompositeAlphaModePreMultiplied
	AlphaModePostmultiplied = CompositeAlphaModePostMultiplied
)

// TextureDescriptor describes how to create a texture.
type TextureDescriptor struct {
	Label         string
	Size          Extent3D
	MipLevelCount uint32
	SampleCount   uint32
	Dimension     TextureDimension
	Format        TextureFormat
	Usage         TextureUsage
}

// TextureViewDescriptor describes how to create a texture view.
type TextureViewDescriptor struct {
	Label           string
	Format          TextureFormat
	Dimension       TextureViewDimension
	Aspect          TextureAspect
	BaseMipLevel    uint32
	MipLevelCount   uint32
	BaseArrayLayer  uint32
	ArrayLayerCount uint32
}

// BufferDescriptor describes how to create a buffer.
type BufferDescriptor struct {
	Label            string
	Size             uint64
	Usage            BufferUsage
	MappedAtCreation bool
}

// SamplerDescriptor describes how to create a sampler.
type SamplerDescriptor struct {
	Label         string
	AddressModeU  AddressMode
	AddressModeV  AddressMode
	AddressModeW  AddressMode
	MagFilter     FilterMode
	MinFilter     FilterMode
	MipmapFilter  MipmapFilterMode
	LodMinClamp   float32
	LodMaxClamp   float32
	Compare       CompareFunction
	MaxAnisotropy uint16
}

// RenderPipelineDescriptor describes a render pipeline.
// Uses handles for shader modules and pipeline layout.
type RenderPipelineDescriptor struct {
	Label            string
	VertexShader     ShaderModule
	VertexEntryPoint string
	FragmentShader   ShaderModule
	FragmentEntry    string
	TargetFormat     TextureFormat
	Topology         PrimitiveTopology
	FrontFace        FrontFace
	CullMode         CullMode
	Layout           PipelineLayout
	Blend            *BlendState
}

// RenderPassDescriptor describes a render pass.
type RenderPassDescriptor struct {
	Label            string
	ColorAttachments []ColorAttachment
	DepthStencil     *DepthStencilAttachment
}

// ColorAttachment describes a color render target.
// Uses TextureView handle.
type ColorAttachment struct {
	View          TextureView
	ResolveTarget TextureView
	LoadOp        LoadOp
	StoreOp       StoreOp
	ClearValue    Color
}

// DepthStencilAttachment describes depth/stencil render target.
// Uses TextureView handle.
type DepthStencilAttachment struct {
	View              TextureView
	DepthLoadOp       LoadOp
	DepthStoreOp      StoreOp
	DepthClearValue   float32
	StencilLoadOp     LoadOp
	StencilStoreOp    StoreOp
	StencilClearValue uint32
}

// ComputePipelineDescriptor describes a compute pipeline.
// Uses handles for layout and shader module.
type ComputePipelineDescriptor struct {
	Label      string
	Layout     PipelineLayout
	Module     ShaderModule
	EntryPoint string
}

// BindGroupLayoutDescriptor describes a bind group layout.
type BindGroupLayoutDescriptor struct {
	Label   string
	Entries []BindGroupLayoutEntry
}

// BindGroupLayoutEntry describes a single binding in a bind group layout.
type BindGroupLayoutEntry struct {
	Binding    uint32
	Visibility ShaderStages
	Buffer     *BufferBindingLayout
	Sampler    *SamplerBindingLayout
	Texture    *TextureBindingLayout
}

// BindGroupDescriptor describes a bind group.
// Uses BindGroupLayout handle.
type BindGroupDescriptor struct {
	Label   string
	Layout  BindGroupLayout
	Entries []BindGroupEntry
}

// BindGroupEntry describes a single binding in a bind group.
// Uses typed handles for Buffer, Sampler, and TextureView.
type BindGroupEntry struct {
	Binding     uint32
	Buffer      Buffer
	Offset      uint64
	Size        uint64
	Sampler     Sampler
	TextureView TextureView
}

// PipelineLayoutDescriptor describes a pipeline layout.
// Uses BindGroupLayout handles.
type PipelineLayoutDescriptor struct {
	Label            string
	BindGroupLayouts []BindGroupLayout
}

// ImageCopyTexture identifies a texture subresource for copy operations.
// Uses Texture handle.
type ImageCopyTexture struct {
	Texture  Texture
	MipLevel uint32
	Origin   Origin3D
	Aspect   TextureAspect
}

// ImageDataLayout describes the memory layout of texture data.
type ImageDataLayout struct {
	Offset       uint64
	BytesPerRow  uint32
	RowsPerImage uint32
}

// VertexAttribute describes a vertex attribute.
type VertexAttribute struct {
	Format         VertexFormat
	Offset         uint64
	ShaderLocation uint32
}

// VertexBufferLayout describes a vertex buffer layout.
type VertexBufferLayout struct {
	ArrayStride uint64
	StepMode    VertexStepMode
	Attributes  []VertexAttribute
}
