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
	displayHandle, windowHandle := r.platWindow.GetHandle()

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

	// Wrap HAL surface into wgpu.Surface — stored on primary windowSurface.
	r.primary.surface = wgpu.NewSurfaceFromHAL(halSurface, "Rust Surface")

	// Note: We don't wrap halInstance and exposed.Adapter into wgpu types
	// because the renderer only needs them for cleanup. We store nil for
	// instance/adapter -- the halInstance will be cleaned up when the
	// wgpu device/surface are released (they hold the HAL references).
	// TODO: Proper lifecycle management for Rust HAL instance/adapter.

	return r.initCommon()
}
