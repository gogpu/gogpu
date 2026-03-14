//go:build !windows && !linux && !darwin

package native

import (
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
)

// BackendInfo returns metadata for unsupported platforms.
// gogpu requires Windows (Vulkan/DX12), Linux (Vulkan), or macOS (Metal).
func BackendInfo(_ types.GraphicsAPI) (name string, variant gputypes.Backend) {
	return "unsupported", 0
}
