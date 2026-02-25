//go:build darwin

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/metal"
	"github.com/gogpu/wgpu/hal/software"
)

// NewHalBackend returns the HAL backend for macOS.
// Defaults to Metal, supports software fallback.
func NewHalBackend(api types.GraphicsAPI) (hal.Backend, string, gputypes.Backend) {
	switch api {
	case types.GraphicsAPISoftware:
		return software.API{}, "Pure Go (gogpu/wgpu/software)", gputypes.BackendEmpty
	default: // Metal (default on macOS)
		return metal.Backend{}, "Pure Go (gogpu/wgpu/metal)", gputypes.BackendMetal
	}
}
