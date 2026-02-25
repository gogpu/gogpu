//go:build linux

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/gles"
	"github.com/gogpu/wgpu/hal/software"
	"github.com/gogpu/wgpu/hal/vulkan"
)

// NewHalBackend returns the HAL backend for Linux.
// Supports runtime selection between Vulkan (default) and GLES.
func NewHalBackend(api types.GraphicsAPI) (hal.Backend, string, gputypes.Backend) {
	switch api {
	case types.GraphicsAPIGLES:
		return gles.Backend{}, "Pure Go (gogpu/wgpu/gles)", gputypes.BackendGL
	case types.GraphicsAPIVulkan:
		return vulkan.Backend{}, "Pure Go (gogpu/wgpu/vulkan)", gputypes.BackendVulkan
	case types.GraphicsAPISoftware:
		return software.API{}, "Pure Go (gogpu/wgpu/software)", gputypes.BackendEmpty
	default: // Auto — prefer Vulkan on Linux
		return vulkan.Backend{}, "Pure Go (gogpu/wgpu/vulkan)", gputypes.BackendVulkan
	}
}
