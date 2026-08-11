package gogpu

import (
	"github.com/gogpu/gogpu/internal/compositor"
)

// initFPSOverlayIfNeeded checks the GOGPU_DEBUG_FPS env var and
// auto-registers the FPS overlay on the given RenderTarget. Called once
// per surface during the first drawDebugOverlays call.
//
// This function is called from drawDebugOverlays in renderer.go. The overlay
// self-registers into the RenderTarget's debugOverlays list.
func initFPSOverlayIfNeeded(ws *RenderTarget) {
	mode := compositor.GetFPSDebugMode()
	if !mode.Overlay && !mode.Log {
		return
	}
	// Check if already registered.
	for _, ov := range ws.debugOverlays {
		if ov.Name() == compositor.OverlayNameFPS {
			return
		}
	}
	overlay := &compositor.FPSDebugOverlay{
		Device:        ws.renderer.device,
		SurfaceFormat: ws.renderer.surfaceFormat,
		Mode:          mode,
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}
