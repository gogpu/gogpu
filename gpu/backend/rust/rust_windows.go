//go:build rust && windows

package rust

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// createPlatformSurface creates a rendering surface on Windows via HWND.
func (i *rustInstance) createPlatformSurface(displayHandle, windowHandle uintptr) (hal.Surface, error) {
	surf, err := i.inst.CreateSurfaceFromWindowsHWND(displayHandle, windowHandle)
	if err != nil {
		return nil, fmt.Errorf("rust backend: create surface (Windows HWND): %w", err)
	}
	return &rustSurface{surf: surf}, nil
}

// platformVariant returns the preferred backend variant for Windows.
func platformVariant() gputypes.Backend { return gputypes.BackendVulkan }
