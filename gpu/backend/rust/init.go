//go:build rust && windows

// Package rust provides the WebGPU backend using wgpu-gpu (Rust) via go-webgpu/webgpu.
// This backend offers maximum performance but requires wgpu-gpu library.
//
// Build with: go build -tags rust
//
// Currently only supported on Windows due to go-webgpu/goffi limitations.
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
