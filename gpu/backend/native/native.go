//go:build !windows && !linux && !darwin

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// NewHalBackend returns nil on unsupported platforms.
// gogpu requires Windows (Vulkan/DX12), Linux (Vulkan), or macOS (Metal).
func NewHalBackend(api types.GraphicsAPI) (hal.Backend, string, gputypes.Backend) {
	return nil, "unsupported", 0
}
