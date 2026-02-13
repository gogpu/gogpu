//go:build !rust || !windows

package gogpu

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// rustHalAvailable returns false when the Rust backend is not compiled in.
func rustHalAvailable() bool {
	return false
}

// newRustHalBackend returns nil when the Rust backend is not compiled in.
func newRustHalBackend() (hal.Backend, string, gputypes.Backend) {
	return nil, "", 0
}
