//go:build purego || !rust

// Package native provides the WebGPU backend using pure Go (gogpu/wgpu).
//
// Build tags:
//   - Default (no tags): included on all platforms
//   - -tags purego: force include (use native backend only)
//   - -tags rust: exclude on non-Windows (rust backend takes priority)
package native

import (
	"github.com/gogpu/gogpu/gpu"
)

func init() {
	gpu.RegisterBackend("native", func() gpu.Backend {
		return New()
	})
}
