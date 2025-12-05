package gpu

// PrimitiveTopology defines how vertices are assembled into primitives.
type PrimitiveTopology uint8

const (
	PrimitiveTopologyPointList PrimitiveTopology = iota
	PrimitiveTopologyLineList
	PrimitiveTopologyLineStrip
	PrimitiveTopologyTriangleList
	PrimitiveTopologyTriangleStrip
)

// FrontFace defines front-facing polygon orientation.
type FrontFace uint8

const (
	FrontFaceCCW FrontFace = iota // Counter-clockwise
	FrontFaceCW                   // Clockwise
)

// CullMode defines face culling mode.
type CullMode uint8

const (
	CullModeNone CullMode = iota
	CullModeFront
	CullModeBack
)

// VertexFormat defines vertex attribute formats.
type VertexFormat uint8

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

// VertexStepMode defines how vertex buffer data steps.
type VertexStepMode uint8

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

// VertexBufferLayout describes a vertex buffer layout.
type VertexBufferLayout struct {
	ArrayStride uint64
	StepMode    VertexStepMode
	Attributes  []VertexAttribute
}

// PrimitiveState describes primitive assembly.
type PrimitiveState struct {
	Topology         PrimitiveTopology
	StripIndexFormat IndexFormat
	FrontFace        FrontFace
	CullMode         CullMode
}

// IndexFormat defines index buffer element format.
type IndexFormat uint8

const (
	IndexFormatUint16 IndexFormat = iota
	IndexFormatUint32
)

// BlendFactor defines blend factors.
type BlendFactor uint8

const (
	BlendFactorZero BlendFactor = iota
	BlendFactorOne
	BlendFactorSrc
	BlendFactorOneMinusSrc
	BlendFactorSrcAlpha
	BlendFactorOneMinusSrcAlpha
	BlendFactorDst
	BlendFactorOneMinusDst
	BlendFactorDstAlpha
	BlendFactorOneMinusDstAlpha
	BlendFactorSrcAlphaSaturated
	BlendFactorConstant
	BlendFactorOneMinusConstant
)

// BlendOperation defines blend operations.
type BlendOperation uint8

const (
	BlendOperationAdd BlendOperation = iota
	BlendOperationSubtract
	BlendOperationReverseSubtract
	BlendOperationMin
	BlendOperationMax
)

// BlendComponent describes blending for a single channel.
type BlendComponent struct {
	Operation BlendOperation
	SrcFactor BlendFactor
	DstFactor BlendFactor
}

// BlendState describes color blending.
type BlendState struct {
	Color BlendComponent
	Alpha BlendComponent
}

// ColorWriteMask defines which color channels to write.
type ColorWriteMask uint8

const (
	ColorWriteMaskRed   ColorWriteMask = 1 << 0
	ColorWriteMaskGreen ColorWriteMask = 1 << 1
	ColorWriteMaskBlue  ColorWriteMask = 1 << 2
	ColorWriteMaskAlpha ColorWriteMask = 1 << 3
	ColorWriteMaskAll   ColorWriteMask = ColorWriteMaskRed | ColorWriteMaskGreen | ColorWriteMaskBlue | ColorWriteMaskAlpha
)

// ColorTargetState describes a render target.
type ColorTargetState struct {
	Format    TextureFormat
	Blend     *BlendState
	WriteMask ColorWriteMask
}

// RenderPipeline represents a render pipeline.
type RenderPipeline struct {
	label string
	// Internal handle will be added when integrating with webgpu
}

// Label returns the pipeline label.
func (p *RenderPipeline) Label() string {
	return p.label
}

// ComputePipeline represents a compute pipeline.
type ComputePipeline struct {
	label string
	// Internal handle will be added when integrating with webgpu
}

// Label returns the pipeline label.
func (p *ComputePipeline) Label() string {
	return p.label
}
