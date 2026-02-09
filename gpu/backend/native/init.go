// Package gpu provides the WebGPU backend using pure Go (gogpu/wgpu).
// This is the default backend, always available without external dependencies.
//
// Supports: Windows (Vulkan), Linux (Vulkan), macOS (Metal)
package native

import (
	"github.com/gogpu/gogpu/gpu"
)

func init() {
	gpu.RegisterBackend("gpu", func() gpu.Backend {
		return New()
	})
}
