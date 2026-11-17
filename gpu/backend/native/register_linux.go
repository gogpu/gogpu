//go:build linux

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
)

// BackendInfo returns the backend display name and mask for the given graphics API.
func BackendInfo(api types.GraphicsAPI) (name string, mask gputypes.Backends) {
	switch api {
	case types.GraphicsAPIGLES:
		return "Pure Go (GLES)", gputypes.BackendsGL
	case types.GraphicsAPIVulkan:
		return "Pure Go (Vulkan)", gputypes.BackendsVulkan
	case types.GraphicsAPISoftware:
		return "Pure Go (Software)", gputypes.BackendsSoftware
	default: // Auto — enumerate Vulkan, GLES; best GPU adapter wins
		return "Pure Go (Auto)", gputypes.BackendsVulkan | gputypes.BackendsGL
	}
}
