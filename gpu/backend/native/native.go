//go:build !windows && !linux && !darwin

package native

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// NewHalBackend returns nil on unsupported platforms.
// gogpu requires Windows (Vulkan), Linux (Vulkan), or macOS (Metal).
func NewHalBackend() hal.Backend { return nil }

// HalBackendName returns "unsupported" on unsupported platforms.
func HalBackendName() string { return "unsupported" }

// HalBackendVariant returns 0 on unsupported platforms.
func HalBackendVariant() gputypes.Backend { return 0 }
