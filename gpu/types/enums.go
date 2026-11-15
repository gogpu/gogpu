package types

// GraphicsAPI specifies which graphics API to use for rendering.
// The backend implementation itself is chosen by wgpu build tags (ADR-038).
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

// graphicsAPINameAuto is the display name of GraphicsAPIAuto, and the default
// for unknown values.
const graphicsAPINameAuto = "Auto"

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
		return graphicsAPINameAuto
	}
}
