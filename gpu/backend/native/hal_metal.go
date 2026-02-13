//go:build darwin

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/metal"
)

// NewHalBackend returns the Metal HAL backend for macOS.
// macOS only supports Metal, so the api parameter is ignored.
func NewHalBackend(api types.GraphicsAPI) (hal.Backend, string, gputypes.Backend) {
	return metal.Backend{}, "Pure Go (gogpu/wgpu/metal)", gputypes.BackendMetal
}
