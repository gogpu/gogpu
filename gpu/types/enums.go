package types

// BackendType specifies which WebGPU implementation to use.
type BackendType uint8

const (
	// BackendAuto automatically selects the best available backend.
	// Pure Go is default, Rust is opt-in with -tags rust.
	BackendAuto BackendType = iota

	// BackendNative uses pure Go WebGPU implementation (gogpu/wgpu).
	// Zero dependencies, just `go build`. Default backend.
	BackendNative

	// BackendRust uses wgpu-gpu (Rust) via go-webgpu/webgpu.
	// Maximum performance, requires wgpu-native shared library.
	BackendRust

	// BackendGo is an alias for BackendNative.
	// Provided for user convenience ("I want the Go backend").
	BackendGo = BackendNative
)

// String returns the backend name.
func (b BackendType) String() string {
	switch b {
	case BackendRust:
		return "Rust (wgpu-gpu)"
	case BackendNative:
		return "Native (Pure Go)"
	default:
		return "Auto"
	}
}

// GraphicsAPI specifies which graphics API to use for rendering.
// This is orthogonal to BackendType (Rust/Native implementation choice).
type GraphicsAPI uint8

const (
	// GraphicsAPIAuto automatically selects the best graphics API for the platform.
	// Windows: Vulkan (default), macOS: Metal, Linux: Vulkan.
	GraphicsAPIAuto GraphicsAPI = iota

	// GraphicsAPIVulkan forces Vulkan. Available on Windows and Linux.
	GraphicsAPIVulkan

	// GraphicsAPIDX12 forces DirectX 12. Available on Windows only.
	GraphicsAPIDX12

	// GraphicsAPIMetal forces Metal. Available on macOS only.
	GraphicsAPIMetal

	// GraphicsAPIGLES forces OpenGL ES. Available on Windows and Linux.
	GraphicsAPIGLES

	// GraphicsAPISoftware forces the CPU-based software rasterizer.
	// Always available — no build tags or GPU hardware required.
	// Useful for headless rendering, CI/CD, and systems without GPU.
	GraphicsAPISoftware
)

// String returns the graphics API name.
func (g GraphicsAPI) String() string {
	switch g {
	case GraphicsAPIVulkan:
		return "Vulkan"
	case GraphicsAPIDX12:
		return "DX12"
	case GraphicsAPIMetal:
		return "Metal"
	case GraphicsAPIGLES:
		return "GLES"
	case GraphicsAPISoftware:
		return "Software"
	default:
		return "Auto"
	}
}
