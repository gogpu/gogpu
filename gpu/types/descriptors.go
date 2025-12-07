package types

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
	AlphaMode   AlphaMode
}

// TextureDescriptor describes a texture to create.
type TextureDescriptor struct {
	Label         string
	Size          Extent3D
	MipLevelCount uint32
	SampleCount   uint32
	Dimension     TextureDimension
	Format        TextureFormat
	Usage         TextureUsage
}

// Extent3D represents 3D dimensions.
type Extent3D struct {
	Width              uint32
	Height             uint32
	DepthOrArrayLayers uint32
}

// TextureDimension specifies texture dimensionality.
type TextureDimension uint32

const (
	TextureDimension1D TextureDimension = 0x00
	TextureDimension2D TextureDimension = 0x01
	TextureDimension3D TextureDimension = 0x02
)

// TextureViewDescriptor describes how to create a texture view.
type TextureViewDescriptor struct {
	Format          TextureFormat
	Dimension       TextureViewDimension
	BaseMipLevel    uint32
	MipLevelCount   uint32
	BaseArrayLayer  uint32
	ArrayLayerCount uint32
	Aspect          TextureAspect
}

// TextureViewDimension specifies texture view dimensionality.
type TextureViewDimension uint32

const (
	TextureViewDimensionUndefined TextureViewDimension = 0x00
	TextureViewDimension1D        TextureViewDimension = 0x01
	TextureViewDimension2D        TextureViewDimension = 0x02
	TextureViewDimension2DArray   TextureViewDimension = 0x03
	TextureViewDimensionCube      TextureViewDimension = 0x04
	TextureViewDimensionCubeArray TextureViewDimension = 0x05
	TextureViewDimension3D        TextureViewDimension = 0x06
)

// TextureAspect specifies which aspect of texture to view.
type TextureAspect uint32

const (
	TextureAspectAll         TextureAspect = 0x00
	TextureAspectStencilOnly TextureAspect = 0x01
	TextureAspectDepthOnly   TextureAspect = 0x02
)

// RenderPipelineDescriptor describes a render pipeline.
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
}

// RenderPassDescriptor describes a render pass.
type RenderPassDescriptor struct {
	Label            string
	ColorAttachments []ColorAttachment
	DepthStencil     *DepthStencilAttachment
}

// ColorAttachment describes a color render target.
type ColorAttachment struct {
	View          TextureView
	ResolveTarget TextureView // For MSAA resolve, 0 if unused
	LoadOp        LoadOp
	StoreOp       StoreOp
	ClearValue    Color
}

// DepthStencilAttachment describes depth/stencil render target.
type DepthStencilAttachment struct {
	View              TextureView
	DepthLoadOp       LoadOp
	DepthStoreOp      StoreOp
	DepthClearValue   float32
	StencilLoadOp     LoadOp
	StencilStoreOp    StoreOp
	StencilClearValue uint32
}

// Color represents an RGBA color with float64 components.
// Values are typically in range [0.0, 1.0].
type Color struct {
	R, G, B, A float64
}

// BufferDescriptor describes a buffer to create.
type BufferDescriptor struct {
	Label            string
	Size             uint64
	Usage            BufferUsage
	MappedAtCreation bool
}

// BufferUsage specifies how a buffer can be used.
type BufferUsage uint32

const (
	BufferUsageMapRead      BufferUsage = 0x0001
	BufferUsageMapWrite     BufferUsage = 0x0002
	BufferUsageCopySrc      BufferUsage = 0x0004
	BufferUsageCopyDst      BufferUsage = 0x0008
	BufferUsageIndex        BufferUsage = 0x0010
	BufferUsageVertex       BufferUsage = 0x0020
	BufferUsageUniform      BufferUsage = 0x0040
	BufferUsageStorage      BufferUsage = 0x0080
	BufferUsageIndirect     BufferUsage = 0x0100
	BufferUsageQueryResolve BufferUsage = 0x0200
)

// SamplerDescriptor describes a sampler to create.
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

// AddressMode specifies texture coordinate wrapping behavior.
type AddressMode uint32

const (
	AddressModeClampToEdge AddressMode = iota
	AddressModeRepeat
	AddressModeMirrorRepeat
)

// FilterMode specifies texture sampling filter.
type FilterMode uint32

const (
	FilterModeNearest FilterMode = iota
	FilterModeLinear
)

// MipmapFilterMode specifies mipmap level selection.
type MipmapFilterMode uint32

const (
	MipmapFilterModeNearest MipmapFilterMode = iota
	MipmapFilterModeLinear
)

// CompareFunction for depth/stencil comparisons.
type CompareFunction uint32

const (
	CompareFunctionUndefined CompareFunction = iota
	CompareFunctionNever
	CompareFunctionLess
	CompareFunctionEqual
	CompareFunctionLessEqual
	CompareFunctionGreater
	CompareFunctionNotEqual
	CompareFunctionGreaterEqual
	CompareFunctionAlways
)

// BindGroupLayoutDescriptor describes a bind group layout.
type BindGroupLayoutDescriptor struct {
	Label   string
	Entries []BindGroupLayoutEntry
}

// BindGroupLayoutEntry describes a single binding in a layout.
type BindGroupLayoutEntry struct {
	Binding    uint32
	Visibility ShaderStage
	Buffer     *BufferBindingLayout
	Sampler    *SamplerBindingLayout
	Texture    *TextureBindingLayout
}

// ShaderStage flags indicate which shader stages can access a resource.
type ShaderStage uint32

const (
	ShaderStageNone     ShaderStage = 0
	ShaderStageVertex   ShaderStage = 0x1
	ShaderStageFragment ShaderStage = 0x2
	ShaderStageCompute  ShaderStage = 0x4
)

// BufferBindingLayout describes a buffer binding.
type BufferBindingLayout struct {
	Type             BufferBindingType
	HasDynamicOffset bool
	MinBindingSize   uint64
}

// BufferBindingType specifies buffer binding type.
type BufferBindingType uint32

const (
	BufferBindingTypeUndefined BufferBindingType = iota
	BufferBindingTypeUniform
	BufferBindingTypeStorage
	BufferBindingTypeReadOnlyStorage
)

// SamplerBindingLayout describes a sampler binding.
type SamplerBindingLayout struct {
	Type SamplerBindingType
}

// SamplerBindingType specifies sampler binding type.
type SamplerBindingType uint32

const (
	SamplerBindingTypeUndefined SamplerBindingType = iota
	SamplerBindingTypeFiltering
	SamplerBindingTypeNonFiltering
	SamplerBindingTypeComparison
)

// TextureBindingLayout describes a texture binding.
type TextureBindingLayout struct {
	SampleType    TextureSampleType
	ViewDimension TextureViewDimension
	Multisampled  bool
}

// TextureSampleType specifies texture sampling type.
type TextureSampleType uint32

const (
	TextureSampleTypeUndefined TextureSampleType = iota
	TextureSampleTypeFloat
	TextureSampleTypeUnfilterableFloat
	TextureSampleTypeDepth
	TextureSampleTypeSint
	TextureSampleTypeUint
)

// BindGroupDescriptor describes a bind group to create.
type BindGroupDescriptor struct {
	Label   string
	Layout  BindGroupLayout
	Entries []BindGroupEntry
}

// BindGroupEntry describes a single resource binding.
type BindGroupEntry struct {
	Binding     uint32
	Buffer      Buffer
	Offset      uint64
	Size        uint64
	Sampler     Sampler
	TextureView TextureView
}

// PipelineLayoutDescriptor describes a pipeline layout.
type PipelineLayoutDescriptor struct {
	Label            string
	BindGroupLayouts []BindGroupLayout
}

// ImageCopyTexture identifies a texture subresource for copy operations.
type ImageCopyTexture struct {
	Texture  Texture
	MipLevel uint32
	Origin   Origin3D
	Aspect   TextureAspect
}

// Origin3D represents a 3D origin point.
type Origin3D struct {
	X, Y, Z uint32
}

// ImageDataLayout describes layout of image data in memory.
type ImageDataLayout struct {
	Offset       uint64
	BytesPerRow  uint32
	RowsPerImage uint32
}

// VertexBufferLayout describes vertex buffer layout for a pipeline.
type VertexBufferLayout struct {
	ArrayStride uint64
	StepMode    VertexStepMode
	Attributes  []VertexAttribute
}

// VertexStepMode specifies how vertex data is stepped.
type VertexStepMode uint32

const (
	VertexStepModeVertex VertexStepMode = iota
	VertexStepModeInstance
)

// VertexAttribute describes a single vertex attribute.
type VertexAttribute struct {
	Format         VertexFormat
	Offset         uint64
	ShaderLocation uint32
}

// VertexFormat specifies vertex attribute format.
type VertexFormat uint32

// IndexFormat specifies index buffer element format.
type IndexFormat uint32

const (
	IndexFormatUint16 IndexFormat = iota
	IndexFormatUint32
)

const (
	VertexFormatUint8x2 VertexFormat = iota
	VertexFormatUint8x4
	VertexFormatSint8x2
	VertexFormatSint8x4
	VertexFormatUnorm8x2
	VertexFormatUnorm8x4
	VertexFormatSnorm8x2
	VertexFormatSnorm8x4
	VertexFormatUint16x2
	VertexFormatUint16x4
	VertexFormatSint16x2
	VertexFormatSint16x4
	VertexFormatUnorm16x2
	VertexFormatUnorm16x4
	VertexFormatSnorm16x2
	VertexFormatSnorm16x4
	VertexFormatFloat16x2
	VertexFormatFloat16x4
	VertexFormatFloat32
	VertexFormatFloat32x2
	VertexFormatFloat32x3
	VertexFormatFloat32x4
	VertexFormatUint32
	VertexFormatUint32x2
	VertexFormatUint32x3
	VertexFormatUint32x4
	VertexFormatSint32
	VertexFormatSint32x2
	VertexFormatSint32x3
	VertexFormatSint32x4
)
