package types

// BackendType specifies which WebGPU implementation to use.
type BackendType uint8

const (
	// BackendAuto automatically selects the best available backend.
	// Currently defaults to Rust, will prefer Pure Go when stable.
	BackendAuto BackendType = iota

	// BackendRust uses wgpu-native (Rust) via go-webgpu/webgpu.
	// Maximum performance, battle-tested, requires native library.
	BackendRust

	// BackendGo uses pure Go WebGPU implementation (gogpu/wgpu).
	// Zero dependencies, just `go build`, may be slower.
	BackendGo
)

// String returns the backend name.
func (b BackendType) String() string {
	switch b {
	case BackendRust:
		return "Rust (wgpu-native)"
	case BackendGo:
		return "Pure Go"
	default:
		return "Auto"
	}
}

// TextureFormat specifies texture pixel format.
// Values match WebGPU specification.
type TextureFormat uint32

const (
	TextureFormatRGBA8Unorm TextureFormat = 0x12
	TextureFormatBGRA8Unorm TextureFormat = 0x17
)

// TextureUsage specifies how a texture can be used.
// Values match WebGPU specification.
type TextureUsage uint32

const (
	TextureUsageCopySrc          TextureUsage = 0x01
	TextureUsageCopyDst          TextureUsage = 0x02
	TextureUsageTextureBinding   TextureUsage = 0x04
	TextureUsageStorageBinding   TextureUsage = 0x08
	TextureUsageRenderAttachment TextureUsage = 0x10
)

// PresentMode specifies surface presentation timing.
type PresentMode uint32

const (
	PresentModeFifo        PresentMode = 0x01 // VSync enabled
	PresentModeFifoRelaxed PresentMode = 0x02 // VSync with tearing allowed
	PresentModeImmediate   PresentMode = 0x03 // No VSync
	PresentModeMailbox     PresentMode = 0x04 // Triple buffering
)

// AlphaMode specifies surface alpha compositing.
type AlphaMode uint32

const (
	AlphaModeOpaque         AlphaMode = 0x01
	AlphaModePremultiplied  AlphaMode = 0x02
	AlphaModePostmultiplied AlphaMode = 0x03
)

// PowerPreference specifies GPU power profile.
type PowerPreference uint32

const (
	PowerPreferenceDefault PowerPreference = iota
	PowerPreferenceLowPower
	PowerPreferenceHighPerformance
)

// LoadOp specifies how to load render target at pass start.
type LoadOp uint32

const (
	LoadOpClear LoadOp = 0x01
	LoadOpLoad  LoadOp = 0x02
)

// StoreOp specifies how to store render target at pass end.
type StoreOp uint32

const (
	StoreOpStore   StoreOp = 0x01
	StoreOpDiscard StoreOp = 0x02
)

// PrimitiveTopology specifies how vertices are assembled.
type PrimitiveTopology uint32

const (
	PrimitiveTopologyPointList     PrimitiveTopology = 0x00
	PrimitiveTopologyLineList      PrimitiveTopology = 0x01
	PrimitiveTopologyLineStrip     PrimitiveTopology = 0x02
	PrimitiveTopologyTriangleList  PrimitiveTopology = 0x03
	PrimitiveTopologyTriangleStrip PrimitiveTopology = 0x04
)

// FrontFace specifies which triangle winding is front-facing.
type FrontFace uint32

const (
	FrontFaceCCW FrontFace = 0x00 // Counter-clockwise
	FrontFaceCW  FrontFace = 0x01 // Clockwise
)

// CullMode specifies which triangles to cull.
type CullMode uint32

const (
	CullModeNone  CullMode = 0x00
	CullModeFront CullMode = 0x01
	CullModeBack  CullMode = 0x02
)
