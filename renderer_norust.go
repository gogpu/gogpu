//go:build !rust && !(js && wasm)

package gogpu

import (
	"fmt"

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

// initRust returns an error because the Rust backend is not compiled in.
func (r *Renderer) initRust() error {
	return fmt.Errorf("gogpu: rust backend not available (build with -tags rust)")
}
