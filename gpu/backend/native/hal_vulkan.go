//go:build windows || linux

package native

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/vulkan"
)

// newHalBackend returns the Vulkan HAL backend for Windows/Linux.
func newHalBackend() hal.Backend { return vulkan.Backend{} }

// halBackendName returns the human-readable backend name.
func halBackendName() string { return "Pure Go (gogpu/wgpu/vulkan)" }

// halBackendVariant returns the backend variant for instance creation.
func halBackendVariant() gputypes.Backend { return gputypes.BackendVulkan }

// platformPreSubmit is a no-op on Vulkan platforms.
// Vulkan does not require drawable attachment before submit.
func platformPreSubmit(_ hal.CommandBuffer, _ *ResourceRegistry) {}
