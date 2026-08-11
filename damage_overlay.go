package gogpu

import (
	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gpucontext"
)

// initDamageOverlayIfNeeded checks the GOGPU_DEBUG_DAMAGE env var and
// auto-registers the damage overlay on the given RenderTarget. Called once
// per surface during the first frame that has damage sources.
//
// This function is called from drawDebugOverlays in renderer.go. The overlay
// self-registers into the RenderTarget's debugOverlays list.
func initDamageOverlayIfNeeded(ws *RenderTarget) {
	mode := compositor.GetDamageDebugMode()
	if !mode.Overlay && !mode.Log {
		return
	}
	// Check if already registered.
	for _, ov := range ws.debugOverlays {
		if ov.Name() == compositor.OverlayNameDamage {
			return
		}
	}
	scale := 1.0
	if ws.platWindow != nil {
		if s := ws.platWindow.ScaleFactor(); s > 0 {
			scale = s
		}
	}
	overlay := &compositor.DamageDebugOverlay{
		DamageSources: &ws.damageSources,
		HasGPUWork:    &ws.hasGPUWork,
		ScaleFactor:   scale,
		Device:        ws.renderer.device,
		SurfaceFormat: ws.renderer.surfaceFormat,
		Mode:          mode,
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}

// setCustomDamageRenderer sets or replaces the custom damage overlay renderer.
// When set, the overlay delegates rendering to this renderer instead of using
// the built-in flat-color pipeline. If the overlay is not yet registered, this
// is stored for later use.
func (ws *RenderTarget) setCustomDamageRenderer(renderer gpucontext.DamageOverlayRenderer) {
	for _, ov := range ws.debugOverlays {
		if dov, ok := ov.(*compositor.DamageDebugOverlay); ok {
			dov.CustomRenderer = renderer
			return
		}
	}
	// Overlay not registered yet. Force-register it with overlay mode so
	// the custom renderer can take effect when drawing starts.
	mode := compositor.GetDamageDebugMode()
	mode.Overlay = true
	scale := 1.0
	if ws.platWindow != nil {
		if s := ws.platWindow.ScaleFactor(); s > 0 {
			scale = s
		}
	}
	overlay := &compositor.DamageDebugOverlay{
		DamageSources:  &ws.damageSources,
		HasGPUWork:     &ws.hasGPUWork,
		ScaleFactor:    scale,
		Device:         ws.renderer.device,
		SurfaceFormat:  ws.renderer.surfaceFormat,
		Mode:           mode,
		CustomRenderer: renderer,
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}
