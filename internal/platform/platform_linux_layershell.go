//go:build linux

package platform

import "github.com/gogpu/gogpu/internal/platform/wayland"

// layerShellOptions converts the platform-level layer-shell config into the
// Wayland bindings' options, filling in the registry global name/version.
//
// Values are passed through unchanged: the layer-shell protocol's own zero
// values are meaningful (layer 0 = background, anchor 0 = centered, keyboard
// interactivity 0 = none), so nothing is defaulted here except the namespace.
func layerShellOptions(cfg *LayerShellConfig, globalName, globalVersion uint32) *wayland.LayerShellOptions {
	opts := &wayland.LayerShellOptions{
		GlobalName:            globalName,
		GlobalVersion:         globalVersion,
		Namespace:             cfg.Namespace,
		Layer:                 cfg.Layer,
		Anchor:                cfg.Anchor,
		KeyboardInteractivity: cfg.KeyboardInteractivity,
		Width:                 cfg.Width,
		Height:                cfg.Height,
		ExclusiveZone:         cfg.ExclusiveZone,
	}
	if opts.Namespace == "" {
		opts.Namespace = "gogpu"
	}
	return opts
}
