//go:build linux

package platform

import (
	"errors"
	"testing"

	"github.com/gogpu/gogpu/internal/platform/wayland"
)

// TestWaylandASecondWindowFailsToOpen checks that a window asked for while
// the platform's connection is up answers an error and opens nothing.
func TestWaylandASecondWindowFailsToOpen(t *testing.T) {
	primary := &waylandWindow{}
	p := &waylandPlatform{libwl: &wayland.LibwaylandHandle{}, primary: primary, primaryWindowID: 1}

	window, err := p.CreateWindow(Config{Title: "second", Width: 320, Height: 240})

	if !errors.Is(err, errWaylandSecondWindow) {
		t.Fatalf("CreateWindow error = %v, want errWaylandSecondWindow", err)
	}
	if window != nil {
		t.Fatalf("CreateWindow window = %v, want nil", window)
	}
	if p.primaryWindowID != 1 || primary.winID != 0 {
		t.Fatalf("CreateWindow changed the open window: primaryWindowID %d, winID %d", p.primaryWindowID, primary.winID)
	}
}
