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
