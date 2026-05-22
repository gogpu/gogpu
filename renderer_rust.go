//go:build rust

package gogpu

import (
	"fmt"

	"github.com/gogpu/gogpu/gpu/backend/rust"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
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

// initRust initializes the renderer using the Rust (wgpu-native) backend.
// The Rust backend creates HAL objects which are wrapped into wgpu types
// via NewDeviceFromHAL and NewSurfaceFromHAL for a unified API.
func (r *Renderer) initRust() error {
	halBackend, backendName, backendVariant := newRustHalBackend()
	r.backendName = backendName

	// Create HAL instance via Rust backend
	halInstance, err := halBackend.CreateInstance(&hal.InstanceDescriptor{
		Backends: gputypes.Backends(backendVariant),
		Flags:    gputypes.InstanceFlagsDebug | gputypes.InstanceFlagsValidation,
	})
	if err != nil {
		return fmt.Errorf("gogpu: failed to create rust instance: %w", err)
	}

	// Get platform handles for surface creation
	displayHandle, windowHandle := r.primary.platWindow.GetHandle()

	// Create HAL surface
	halSurface, err := halInstance.CreateSurface(displayHandle, windowHandle)
	if err != nil {
		halInstance.Destroy()
		return fmt.Errorf("gogpu: failed to create surface: %w", err)
	}

	// Enumerate adapters and pick the first compatible one
	adapters := halInstance.EnumerateAdapters(halSurface)
	if len(adapters) == 0 {
		halSurface.Destroy()
		halInstance.Destroy()
		return fmt.Errorf("gogpu: no compatible GPU adapters found")
	}
	exposed := adapters[0]

	// Open device with default features and limits
	openDevice, err := exposed.Adapter.Open(0, gputypes.DefaultLimits())
	if err != nil {
		halSurface.Destroy()
		halInstance.Destroy()
		return fmt.Errorf("gogpu: failed to open device: %w", err)
	}

	// Wrap HAL objects into wgpu types for the unified API.
	// NewDeviceFromHAL creates a core.Device internally and wraps it.
	r.device, err = wgpu.NewDeviceFromHAL(
		openDevice.Device,
		openDevice.Queue,
		exposed.Features,
		exposed.Capabilities.Limits,
		"Rust Device",
	)
	if err != nil {
		halSurface.Destroy()
		halInstance.Destroy()
		return fmt.Errorf("gogpu: failed to wrap rust device: %w", err)
	}

	// Wrap HAL surface into wgpu.Surface — stored on primary RenderTarget.
	r.primary.surface = wgpu.NewSurfaceFromHAL(halSurface, "Rust Surface")
	r.primary.state = SurfaceReady
	r.primary.format = gputypes.TextureFormatBGRA8Unorm

	// Configure surface with initial dimensions.
	width, height := r.primary.platWindow.PhysicalSize()
	if width > 0 && height > 0 {
		r.primary.width = uint32(width)   //nolint:gosec // G115: validated positive above
		r.primary.height = uint32(height) //nolint:gosec // G115: validated positive above
		if err := r.primary.configure(r.device, r.adapter); err != nil {
			return fmt.Errorf("gogpu: failed to configure surface: %w", err)
		}
		r.primary.state = SurfaceConfigured
	}

	return nil
}
