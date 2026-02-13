//go:build rust && windows

package gogpu

import (
	"github.com/gogpu/gogpu/gpu/backend/rust"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// rustHalAvailable returns true when the Rust HAL backend can be used.
func rustHalAvailable() bool {
	return rust.IsAvailable()
}

// newRustHalBackend returns the Rust HAL backend, name, and variant.
func newRustHalBackend() (hal.Backend, string, gputypes.Backend) {
	return rust.NewHalBackend(), rust.HalBackendName(), rust.HalBackendVariant()
}
