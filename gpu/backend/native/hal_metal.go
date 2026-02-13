//go:build darwin

package native

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/metal"
)

// NewHalBackend returns the Metal HAL backend for macOS.
func NewHalBackend() hal.Backend { return metal.Backend{} }

// HalBackendName returns the human-readable backend name.
func HalBackendName() string { return "Pure Go (gogpu/wgpu/metal)" }

// HalBackendVariant returns the backend variant for instance creation.
func HalBackendVariant() gputypes.Backend { return gputypes.BackendMetal }
