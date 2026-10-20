package gogpu

import "github.com/gogpu/gogpu/internal/platform"

// layerShellFromConfig converts the public layer-shell configuration into the
// platform-level one, or nil when the window is an ordinary desktop window.
func layerShellFromConfig(cfg *LayerShellConfig) *platform.LayerShellConfig {
	if cfg == nil {
		return nil
	}
	return &platform.LayerShellConfig{
		Namespace:             cfg.Namespace,
		Layer:                 uint32(cfg.Layer),
		Anchor:                uint32(cfg.Anchor),
		KeyboardInteractivity: uint32(cfg.Keyboard),
		Width:                 cfg.Width,
		Height:                cfg.Height,
		ExclusiveZone:         cfg.ExclusiveZone,
	}
}
