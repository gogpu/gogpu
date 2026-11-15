//go:build windows

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"

	// Importing HAL backends triggers their init() registration with hal.RegisterBackend().
	// This is required for wgpu.CreateInstance() to discover available backends.
	_ "github.com/gogpu/wgpu/hal/dx12"
	_ "github.com/gogpu/wgpu/hal/gles"
	_ "github.com/gogpu/wgpu/hal/software"
	_ "github.com/gogpu/wgpu/hal/vulkan"
)

// BackendInfo returns the backend display name and mask for the given graphics API.
// For Auto mode, returns a multi-backend mask so wgpu can enumerate all available
// backends and pick the best adapter (Rust wgpu pattern). For explicit API selection,
// returns a single-backend mask.
func BackendInfo(api types.GraphicsAPI) (name string, mask gputypes.Backends) {
	switch api {
	case types.GraphicsAPIDX12:
		return "Pure Go (DX12)", gputypes.BackendsDX12
	case types.GraphicsAPIGLES:
		return "Pure Go (GLES)", gputypes.BackendsGL
	case types.GraphicsAPIVulkan:
		return "Pure Go (Vulkan)", gputypes.BackendsVulkan
	case types.GraphicsAPISoftware:
		return "Pure Go (Software)", 0 // software backend passes through mask filter
	default: // Auto — enumerate DX12, Vulkan, GLES; best GPU adapter wins
		return "Pure Go (Auto)", gputypes.BackendsDX12 | gputypes.BackendsVulkan | gputypes.BackendsGL
	}
}
