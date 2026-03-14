//go:build darwin

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"

	// Importing HAL backends triggers their init() registration with hal.RegisterBackend().
	_ "github.com/gogpu/wgpu/hal/metal"
	_ "github.com/gogpu/wgpu/hal/software"
)

// BackendInfo returns the backend display name and variant for the given graphics API.
// The actual HAL backends are registered via init() imports above.
func BackendInfo(api types.GraphicsAPI) (name string, variant gputypes.Backend) {
	switch api {
	case types.GraphicsAPISoftware:
		return "Pure Go (gogpu/wgpu/software)", gputypes.BackendEmpty
	default: // Metal (default on macOS)
		return "Pure Go (gogpu/wgpu/metal)", gputypes.BackendMetal
	}
}
