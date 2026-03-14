//go:build linux

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"

	// Importing HAL backends triggers their init() registration with hal.RegisterBackend().
	_ "github.com/gogpu/wgpu/hal/gles"
	_ "github.com/gogpu/wgpu/hal/software"
	_ "github.com/gogpu/wgpu/hal/vulkan"
)

// BackendInfo returns the backend display name and variant for the given graphics API.
// The actual HAL backends are registered via init() imports above.
func BackendInfo(api types.GraphicsAPI) (name string, variant gputypes.Backend) {
	switch api {
	case types.GraphicsAPIGLES:
		return "Pure Go (gogpu/wgpu/gles)", gputypes.BackendGL
	case types.GraphicsAPIVulkan:
		return "Pure Go (gogpu/wgpu/vulkan)", gputypes.BackendVulkan
	case types.GraphicsAPISoftware:
		return "Pure Go (gogpu/wgpu/software)", gputypes.BackendEmpty
	default: // Auto — prefer Vulkan on Linux
		return "Pure Go (gogpu/wgpu/vulkan)", gputypes.BackendVulkan
	}
}
