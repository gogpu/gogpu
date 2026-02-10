//go:build darwin

package native

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/metal"
)

// newHalBackend returns the Metal HAL backend for macOS.
func newHalBackend() hal.Backend { return metal.Backend{} }

// halBackendName returns the human-readable backend name.
func halBackendName() string { return "Pure Go (gogpu/wgpu/metal)" }

// halBackendVariant returns the backend variant for instance creation.
func halBackendVariant() gputypes.Backend { return gputypes.BackendMetal }

// platformPreSubmit attaches the current drawable to a command buffer.
// This is required for Metal where presentDrawable: must be called before commit.
func platformPreSubmit(cmdBuffer hal.CommandBuffer, registry *ResourceRegistry) {
	// Type-assert to Metal command buffer
	metalCmdBuffer, ok := cmdBuffer.(*metal.CommandBuffer)
	if !ok {
		return
	}

	// Find any current surface texture and get its drawable.
	// In practice, there's only one surface per frame.
	surfaceTexture := registry.GetAnySurfaceTexture()
	if surfaceTexture == nil {
		return
	}

	// Type-assert to Metal surface texture
	metalSurfaceTex, ok := surfaceTexture.(*metal.SurfaceTexture)
	if !ok {
		return
	}

	// Get drawable and attach to command buffer
	drawable := metalSurfaceTex.Drawable()
	if drawable != 0 {
		metalCmdBuffer.SetDrawable(drawable)
	}
}
