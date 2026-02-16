//go:build rust && darwin

package rust

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// createPlatformSurface creates a rendering surface on macOS via CAMetalLayer.
// On macOS, displayHandle is unused (0) and windowHandle is a CAMetalLayer pointer.
func (i *rustInstance) createPlatformSurface(_, windowHandle uintptr) (hal.Surface, error) {
	surf, err := i.inst.CreateSurfaceFromMetalLayer(windowHandle)
	if err != nil {
		return nil, fmt.Errorf("rust backend: create surface (Metal layer): %w", err)
	}
	return &rustSurface{surf: surf}, nil
}

// platformVariant returns the preferred backend variant for macOS.
func platformVariant() gputypes.Backend { return gputypes.BackendMetal }
