package gpu

import "errors"

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

// Common backend errors.
var (
	ErrBackendNotAvailable = errors.New("gpu: backend not available")
	ErrNotImplemented      = errors.New("gpu: not implemented")
)

// Backend is the interface that both Rust and Pure Go implementations satisfy.
// This abstraction allows users to switch backends without changing their code.
type Backend interface {
	// Name returns the backend identifier.
	Name() string

	// Init initializes the backend.
	Init() error

	// Destroy releases all backend resources.
	Destroy()

	// Instance operations
	CreateInstance() (Instance, error)

	// Adapter operations
	RequestAdapter(instance Instance, opts *AdapterOptions) (Adapter, error)

	// Device operations
	RequestDevice(adapter Adapter, opts *DeviceOptions) (Device, error)
	GetQueue(device Device) Queue

	// Surface operations
	CreateSurface(instance Instance, handle SurfaceHandle) (Surface, error)
	ConfigureSurface(surface Surface, device Device, config *SurfaceConfig)
	GetCurrentTexture(surface Surface) (SurfaceTexture, error)
	Present(surface Surface)

	// Shader operations
	CreateShaderModuleWGSL(device Device, code string) (ShaderModule, error)

	// Pipeline operations
	CreateRenderPipeline(device Device, desc *RenderPipelineDescriptor) (RenderPipeline, error)

	// Command operations
	CreateCommandEncoder(device Device) CommandEncoder
	BeginRenderPass(encoder CommandEncoder, desc *RenderPassDescriptor) RenderPass
	EndRenderPass(pass RenderPass)
	FinishEncoder(encoder CommandEncoder) CommandBuffer
	Submit(queue Queue, commands CommandBuffer)

	// Render pass operations
	SetPipeline(pass RenderPass, pipeline RenderPipeline)
	Draw(pass RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32)

	// Resource operations
	CreateTextureView(texture Texture, desc *TextureViewDescriptor) TextureView
	ReleaseTextureView(view TextureView)
	ReleaseTexture(texture Texture)
	ReleaseCommandBuffer(buffer CommandBuffer)
	ReleaseCommandEncoder(encoder CommandEncoder)
	ReleaseRenderPass(pass RenderPass)
}

// Handle types - opaque references to backend-specific objects.
// These are type-safe wrappers around uintptr.
type (
	Instance       uintptr
	Adapter        uintptr
	Device         uintptr
	Queue          uintptr
	Surface        uintptr
	SurfaceTexture struct {
		Texture Texture
		Status  SurfaceStatus
	}
	Texture        uintptr
	TextureView    uintptr
	ShaderModule   uintptr
	RenderPipeline uintptr
	CommandEncoder uintptr
	CommandBuffer  uintptr
	RenderPass     uintptr
)

// SurfaceHandle contains platform-specific window handles.
type SurfaceHandle struct {
	// Windows: HINSTANCE and HWND
	// macOS: NSView pointer
	// Linux: Display and Window (X11)
	Instance uintptr
	Window   uintptr
}

// SurfaceStatus indicates the result of GetCurrentTexture.
type SurfaceStatus uint32

const (
	SurfaceStatusSuccess SurfaceStatus = iota
	SurfaceStatusTimeout
	SurfaceStatusOutdated
	SurfaceStatusLost
	SurfaceStatusError
)

// AdapterOptions configures adapter request.
type AdapterOptions struct {
	PowerPreference PowerPreference
}

// PowerPreference specifies GPU power profile.
type PowerPreference uint32

const (
	PowerPreferenceDefault PowerPreference = iota
	PowerPreferenceLowPower
	PowerPreferenceHighPerformance
)

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

// TextureFormat specifies texture pixel format.
type TextureFormat uint32

const (
	TextureFormatBGRA8Unorm TextureFormat = 0x17
	TextureFormatRGBA8Unorm TextureFormat = 0x12
)

// TextureUsage specifies how a texture can be used.
type TextureUsage uint32

const (
	TextureUsageRenderAttachment TextureUsage = 0x10
)

// PresentMode specifies surface presentation timing.
type PresentMode uint32

const (
	PresentModeFifo     PresentMode = 0x01 // VSync
	PresentModeImmediate PresentMode = 0x03 // No VSync
)

// AlphaMode specifies surface alpha compositing.
type AlphaMode uint32

const (
	AlphaModeOpaque AlphaMode = 0x01
)

// TextureViewDescriptor describes how to create a texture view.
type TextureViewDescriptor struct {
	Format TextureFormat
}

// RenderPipelineDescriptor describes a render pipeline.
type RenderPipelineDescriptor struct {
	VertexShader     ShaderModule
	VertexEntryPoint string
	FragmentShader   ShaderModule
	FragmentEntry    string
	TargetFormat     TextureFormat
}

// RenderPassDescriptor describes a render pass.
type RenderPassDescriptor struct {
	ColorAttachments []ColorAttachment
}

// ColorAttachment describes a color render target.
type ColorAttachment struct {
	View       TextureView
	LoadOp     LoadOp
	StoreOp    StoreOp
	ClearColor Color
}

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

// Color represents an RGBA color.
type Color struct {
	R, G, B, A float64
}

// activeBackend is the currently selected backend.
var activeBackend Backend

// SetBackend sets the active backend.
func SetBackend(b Backend) {
	activeBackend = b
}

// GetBackend returns the active backend.
func GetBackend() Backend {
	return activeBackend
}
