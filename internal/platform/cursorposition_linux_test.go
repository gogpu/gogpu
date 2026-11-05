//go:build linux

package platform

import (
	"errors"
	"testing"

	"github.com/gogpu/gogpu/internal/platform/wayland"
)

func TestWaylandSetCursorPositionUnlockedIsUnsupported(t *testing.T) {
	for _, mode := range []int{0, 2} {
		platform := &waylandPlatform{
			primary: &waylandWindow{cursorMode: mode},
			libwl:   &wayland.LibwaylandHandle{},
		}
		window := &waylandPlatformWindow{platform: platform, id: 1}

		if err := window.SetCursorPosition(10, 20); !errors.Is(err, ErrCursorPositionUnsupported) {
			t.Errorf("mode %d: SetCursorPosition = %v, want ErrCursorPositionUnsupported", mode, err)
		}
	}
}

func TestWaylandSetCursorPositionLockFailedIsUnsupported(t *testing.T) {
	// Locked was asked for, but the compositor gave no locked pointer.
	platform := &waylandPlatform{
		primary: &waylandWindow{cursorMode: 1},
		libwl:   &wayland.LibwaylandHandle{},
	}
	window := &waylandPlatformWindow{platform: platform, id: 1}

	if err := window.SetCursorPosition(10, 20); !errors.Is(err, ErrCursorPositionUnsupported) {
		t.Errorf("SetCursorPosition = %v, want ErrCursorPositionUnsupported", err)
	}
}
