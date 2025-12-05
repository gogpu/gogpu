package gpu

// TextureFormat represents texture pixel formats.
type TextureFormat uint32

const (
	// 8-bit formats
	TextureFormatR8Unorm TextureFormat = iota
	TextureFormatR8Snorm
	TextureFormatR8Uint
	TextureFormatR8Sint

	// 16-bit formats
	TextureFormatR16Uint
	TextureFormatR16Sint
	TextureFormatR16Float
	TextureFormatRG8Unorm
	TextureFormatRG8Snorm
	TextureFormatRG8Uint
	TextureFormatRG8Sint

	// 32-bit formats
	TextureFormatR32Uint
	TextureFormatR32Sint
	TextureFormatR32Float
	TextureFormatRG16Uint
	TextureFormatRG16Sint
	TextureFormatRG16Float
	TextureFormatRGBA8Unorm
	TextureFormatRGBA8UnormSrgb
	TextureFormatRGBA8Snorm
	TextureFormatRGBA8Uint
	TextureFormatRGBA8Sint
	TextureFormatBGRA8Unorm
	TextureFormatBGRA8UnormSrgb

	// Packed 32-bit formats
	TextureFormatRGB9E5Ufloat
	TextureFormatRGB10A2Uint
	TextureFormatRGB10A2Unorm
	TextureFormatRG11B10Ufloat

	// 64-bit formats
	TextureFormatRG32Uint
	TextureFormatRG32Sint
	TextureFormatRG32Float
	TextureFormatRGBA16Uint
	TextureFormatRGBA16Sint
	TextureFormatRGBA16Float

	// 128-bit formats
	TextureFormatRGBA32Uint
	TextureFormatRGBA32Sint
	TextureFormatRGBA32Float

	// Depth/stencil formats
	TextureFormatStencil8
	TextureFormatDepth16Unorm
	TextureFormatDepth24Plus
	TextureFormatDepth24PlusStencil8
	TextureFormatDepth32Float
	TextureFormatDepth32FloatStencil8
)

// TextureUsage describes how a texture will be used.
type TextureUsage uint32

const (
	TextureUsageCopySrc         TextureUsage = 1 << 0
	TextureUsageCopyDst         TextureUsage = 1 << 1
	TextureUsageTextureBinding  TextureUsage = 1 << 2
	TextureUsageStorageBinding  TextureUsage = 1 << 3
	TextureUsageRenderAttachment TextureUsage = 1 << 4
)

// TextureDimension represents texture dimensions.
type TextureDimension uint8

const (
	TextureDimension1D TextureDimension = iota
	TextureDimension2D
	TextureDimension3D
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

// Extent3D represents 3D dimensions.
type Extent3D struct {
	Width              uint32
	Height             uint32
	DepthOrArrayLayers uint32
}

// Texture represents a GPU texture.
type Texture struct {
	label         string
	size          Extent3D
	mipLevelCount uint32
	sampleCount   uint32
	dimension     TextureDimension
	format        TextureFormat
	usage         TextureUsage
	// Internal handle will be added when integrating with webgpu
}

// Label returns the texture label.
func (t *Texture) Label() string {
	return t.label
}

// Width returns the texture width.
func (t *Texture) Width() uint32 {
	return t.size.Width
}

// Height returns the texture height.
func (t *Texture) Height() uint32 {
	return t.size.Height
}

// DepthOrArrayLayers returns depth or array layers.
func (t *Texture) DepthOrArrayLayers() uint32 {
	return t.size.DepthOrArrayLayers
}

// Format returns the texture format.
func (t *Texture) Format() TextureFormat {
	return t.format
}
