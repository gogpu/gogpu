package types

import "github.com/gogpu/gputypes"

// AdapterOptions configures adapter request.
type AdapterOptions struct {
	PowerPreference gputypes.PowerPreference
}

// DeviceOptions configures device request.
type DeviceOptions struct {
	Label string
}

// SurfaceConfig configures surface presentation.
type SurfaceConfig struct {
	Format      gputypes.TextureFormat
	Usage       gputypes.TextureUsage
	Width       uint32
	Height      uint32
	PresentMode gputypes.PresentMode
	AlphaMode   gputypes.CompositeAlphaMode
}

// TextureDescriptor describes how to create a texture.
type TextureDescriptor struct {
	Label         string
	Size          gputypes.Extent3D
	MipLevelCount uint32
	SampleCount   uint32
	Dimension     gputypes.TextureDimension
	Format        gputypes.TextureFormat
	Usage         gputypes.TextureUsage
}

// TextureViewDescriptor describes how to create a texture view.
type TextureViewDescriptor struct {
	Label           string
	Format          gputypes.TextureFormat
	Dimension       gputypes.TextureViewDimension
	Aspect          gputypes.TextureAspect
	BaseMipLevel    uint32
	MipLevelCount   uint32
	BaseArrayLayer  uint32
	ArrayLayerCount uint32
}

// BufferDescriptor describes how to create a buffer.
type BufferDescriptor struct {
	Label            string
	Size             uint64
	Usage            gputypes.BufferUsage
	MappedAtCreation bool
}

// SamplerDescriptor describes how to create a sampler.
type SamplerDescriptor struct {
	Label         string
	AddressModeU  gputypes.AddressMode
	AddressModeV  gputypes.AddressMode
	AddressModeW  gputypes.AddressMode
	MagFilter     gputypes.FilterMode
	MinFilter     gputypes.FilterMode
	MipmapFilter  gputypes.MipmapFilterMode
	LodMinClamp   float32
	LodMaxClamp   float32
	Compare       gputypes.CompareFunction
	MaxAnisotropy uint16
}

// RenderPipelineDescriptor describes how to create a render pipeline.
type RenderPipelineDescriptor struct {
	Label            string
	VertexShader     ShaderModule
	VertexEntryPoint string
	FragmentShader   ShaderModule
	FragmentEntry    string
	TargetFormat     gputypes.TextureFormat
	Topology         gputypes.PrimitiveTopology
	FrontFace        gputypes.FrontFace
	CullMode         gputypes.CullMode
	Layout           PipelineLayout
	Blend            *gputypes.BlendState
}

// RenderPassDescriptor describes how to begin a render pass.
type RenderPassDescriptor struct {
	Label            string
	ColorAttachments []ColorAttachment
	DepthStencil     *DepthStencilAttachment
}

// ColorAttachment describes a color attachment for a render pass.
type ColorAttachment struct {
	View          TextureView
	ResolveTarget TextureView
	LoadOp        gputypes.LoadOp
	StoreOp       gputypes.StoreOp
	ClearValue    gputypes.Color
}

// DepthStencilAttachment describes a depth/stencil attachment for a render pass.
type DepthStencilAttachment struct {
	View              TextureView
	DepthLoadOp       gputypes.LoadOp
	DepthStoreOp      gputypes.StoreOp
	DepthClearValue   float32
	StencilLoadOp     gputypes.LoadOp
	StencilStoreOp    gputypes.StoreOp
	StencilClearValue uint32
	DepthReadOnly     bool
	StencilReadOnly   bool
}

// ComputePipelineDescriptor describes how to create a compute pipeline.
type ComputePipelineDescriptor struct {
	Label      string
	Layout     PipelineLayout
	Module     ShaderModule
	EntryPoint string
}

// BindGroupLayoutDescriptor describes how to create a bind group layout.
type BindGroupLayoutDescriptor struct {
	Label   string
	Entries []BindGroupLayoutEntry
}

// BindGroupLayoutEntry describes a single binding in a bind group layout.
type BindGroupLayoutEntry struct {
	Binding    uint32
	Visibility gputypes.ShaderStages
	Buffer     *gputypes.BufferBindingLayout
	Sampler    *gputypes.SamplerBindingLayout
	Texture    *gputypes.TextureBindingLayout
}

// BindGroupDescriptor describes how to create a bind group.
type BindGroupDescriptor struct {
	Label   string
	Layout  BindGroupLayout
	Entries []BindGroupEntry
}

// BindGroupEntry describes a single resource binding in a bind group.
type BindGroupEntry struct {
	Binding     uint32
	Buffer      Buffer
	Offset      uint64
	Size        uint64
	Sampler     Sampler
	TextureView TextureView
}

// PipelineLayoutDescriptor describes how to create a pipeline layout.
type PipelineLayoutDescriptor struct {
	Label            string
	BindGroupLayouts []BindGroupLayout
}

// ImageCopyTexture describes a texture location for copy operations.
type ImageCopyTexture struct {
	Texture  Texture
	MipLevel uint32
	Origin   gputypes.Origin3D
	Aspect   gputypes.TextureAspect
}

// ImageDataLayout describes the layout of image data in memory.
type ImageDataLayout struct {
	Offset       uint64
	BytesPerRow  uint32
	RowsPerImage uint32
}
