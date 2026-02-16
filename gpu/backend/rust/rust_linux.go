//go:build rust && linux

package rust

import (
	"fmt"
	"os"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// createPlatformSurface creates a rendering surface on Linux.
// Detects Wayland vs X11 based on WAYLAND_DISPLAY environment variable,
// matching the platform layer detection in internal/platform/platform_linux.go.
func (i *rustInstance) createPlatformSurface(displayHandle, windowHandle uintptr) (hal.Surface, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		surf, err := i.inst.CreateSurfaceFromWaylandSurface(displayHandle, windowHandle)
		if err != nil {
			return nil, fmt.Errorf("rust backend: create surface (Wayland): %w", err)
		}
		return &rustSurface{surf: surf}, nil
	}

	// X11 fallback
	surf, err := i.inst.CreateSurfaceFromXlibWindow(displayHandle, uint64(windowHandle))
	if err != nil {
		return nil, fmt.Errorf("rust backend: create surface (X11 Xlib): %w", err)
	}
	return &rustSurface{surf: surf}, nil
}

// platformVariant returns the preferred backend variant for Linux.
func platformVariant() gputypes.Backend { return gputypes.BackendVulkan }
