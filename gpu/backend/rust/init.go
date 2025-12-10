//go:build (windows && !purego) || rust

// Package rust provides the WebGPU backend using wgpu-native (Rust) via go-webgpu/webgpu.
//
// Build tags:
//   - Default (no tags): included on Windows
//   - -tags rust: force include on any platform
//   - -tags purego: exclude (use native backend only)
package rust

import (
	"github.com/gogpu/gogpu/gpu"
)

func init() {
	if IsAvailable() {
		gpu.RegisterBackend("rust", func() gpu.Backend {
			return New()
		})
	}
}
