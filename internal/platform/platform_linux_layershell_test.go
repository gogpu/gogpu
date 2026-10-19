//go:build linux

package platform

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/wayland"
)

// TestLayerShellOptionsMapping checks the translation from the platform-level
// config to the Wayland bindings' options, including the namespace default.
func TestLayerShellOptionsMapping(t *testing.T) {
	cfg := &LayerShellConfig{
		Namespace:             "my-shell",
		Layer:                 3,
		Anchor:                15,
		KeyboardInteractivity: 2,
		Width:                 1280,
		Height:                40,
		ExclusiveZone:         40,
	}
	opts := layerShellOptions(cfg, 42, 5)

	if opts.GlobalName != 42 || opts.GlobalVersion != 5 {
		t.Errorf("global name/version = %d/%d, want 42/5", opts.GlobalName, opts.GlobalVersion)
	}
	if opts.Namespace != "my-shell" {
		t.Errorf("namespace = %q", opts.Namespace)
	}
	if opts.Layer != wayland.LayerOverlay {
		t.Errorf("layer = %d, want overlay (3)", opts.Layer)
	}
	if opts.Anchor != wayland.AnchorTop|wayland.AnchorBottom|wayland.AnchorLeft|wayland.AnchorRight {
		t.Errorf("anchor = %d, want 15", opts.Anchor)
	}
	if opts.KeyboardInteractivity != wayland.KeyboardInteractivityOnDemand {
		t.Errorf("keyboard = %d, want on_demand (2)", opts.KeyboardInteractivity)
	}
	if opts.Width != 1280 || opts.Height != 40 || opts.ExclusiveZone != 40 {
		t.Errorf("size/zone = %dx%d/%d", opts.Width, opts.Height, opts.ExclusiveZone)
	}
}

// TestLayerShellNamespaceDefault covers the only value the mapping fills in:
// compositors show the namespace in their rules and diagnostics, so an empty
// one would make the surface hard to identify.
func TestLayerShellNamespaceDefault(t *testing.T) {
	opts := layerShellOptions(&LayerShellConfig{}, 1, 1)
	if opts.Namespace != "gogpu" {
		t.Errorf("namespace = %q, want %q", opts.Namespace, "gogpu")
	}
}

// TestLayerShellZeroValuesPassThrough documents that protocol zero values are
// not rewritten: layer 0 is the background layer, anchor 0 centers the surface,
// and keyboard interactivity 0 means the surface never takes focus.
func TestLayerShellZeroValuesPassThrough(t *testing.T) {
	opts := layerShellOptions(&LayerShellConfig{}, 1, 1)
	if opts.Layer != wayland.LayerBackground {
		t.Errorf("layer = %d, want background (0)", opts.Layer)
	}
	if opts.Anchor != 0 {
		t.Errorf("anchor = %d, want 0 (centered)", opts.Anchor)
	}
	if opts.KeyboardInteractivity != wayland.KeyboardInteractivityNone {
		t.Errorf("keyboard = %d, want none (0)", opts.KeyboardInteractivity)
	}
}
