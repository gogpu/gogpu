//go:build windows || linux

package native

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/vulkan"
)

// NewHalBackend returns the Vulkan HAL backend for Windows/Linux.
func NewHalBackend() hal.Backend { return vulkan.Backend{} }

// HalBackendName returns the human-readable backend name.
func HalBackendName() string { return "Pure Go (gogpu/wgpu/vulkan)" }

// HalBackendVariant returns the backend variant for instance creation.
func HalBackendVariant() gputypes.Backend { return gputypes.BackendVulkan }
