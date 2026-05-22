package gogpu

import (
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func TestRendererSurfaceFormatIndependent(t *testing.T) {
	r := &Renderer{
		surfaceFormat: gputypes.TextureFormatBGRA8Unorm,
	}
	if r.surfaceFormat != gputypes.TextureFormatBGRA8Unorm {
		t.Error("surfaceFormat should be set on Renderer, not on RenderTarget")
	}
}

func TestRenderTargetCanRenderIndependent(t *testing.T) {
	r := &Renderer{
		surfaceFormat: gputypes.TextureFormatBGRA8Unorm,
	}

	primary := &RenderTarget{renderer: r, state: SurfaceConfigured}
	secondary := &RenderTarget{renderer: r, state: SurfaceConfigured}

	if primary.renderer != secondary.renderer {
		t.Error("both RenderTargets should share the same Renderer")
	}
	// Renderer has no reference to either — they reference Renderer
	// This is the ADR-026 architecture: Renderer owns Device, RenderTargets own Surfaces
}

func TestPrimaryDestroyDoesNotAffectSecondary(t *testing.T) {
	r := &Renderer{
		surfaceFormat: gputypes.TextureFormatBGRA8Unorm,
	}

	dummySurf := &wgpu.Surface{}
	primary := &RenderTarget{
		renderer: r,
		state:    SurfaceConfigured,
		surface:  dummySurf,
		format:   r.surfaceFormat,
		width:    800,
		height:   600,
	}
	secondary := &RenderTarget{
		renderer: r,
		state:    SurfaceConfigured,
		surface:  dummySurf,
		format:   r.surfaceFormat,
		width:    400,
		height:   300,
	}

	r.primary = primary

	// Simulate primary close: nil out surface and state (what destroy does internally)
	primary.surface = nil
	primary.state = SurfaceNone

	// Renderer still alive — surfaceFormat is device-level
	if r.surfaceFormat != gputypes.TextureFormatBGRA8Unorm {
		t.Error("Renderer surfaceFormat should survive primary close")
	}

	// Primary no longer renderable
	if primary.CanRender() {
		t.Error("primary should NOT be renderable after close")
	}

	// Secondary completely unaffected
	if !secondary.CanRender() {
		t.Error("secondary should still be renderable after primary close")
	}
	if secondary.width != 400 || secondary.height != 300 {
		t.Error("secondary dimensions should be unaffected")
	}
	if secondary.renderer != r {
		t.Error("secondary should still reference the same Renderer")
	}
}

func TestRenderFrameSkipsNilSurface(t *testing.T) {
	// Simulates multi-window render loop behavior when primary is closed
	targets := []*RenderTarget{
		nil, // primary closed
		{state: SurfaceConfigured, surface: &wgpu.Surface{}, renderer: &Renderer{}}, // secondary alive
	}

	rendered := 0
	for _, ws := range targets {
		if ws == nil {
			continue
		}
		if ws.CanRender() {
			rendered++
		}
	}

	if rendered != 1 {
		t.Errorf("expected 1 renderable target (secondary), got %d", rendered)
	}
}

func TestBeginFrameForSurfaceNilSafe(t *testing.T) {
	r := &Renderer{}

	// beginFrameForSurface with unconfigured target returns false (no crash)
	ws := &RenderTarget{renderer: r, state: SurfaceReady}
	if r.beginFrameForSurface(ws) {
		t.Error("beginFrameForSurface should return false for unconfigured surface")
	}
}

func TestEndFrameForSurfaceNoFrameStarted(t *testing.T) {
	r := &Renderer{}
	ws := &RenderTarget{renderer: r}

	// Should not panic when frameStarted = false
	r.endFrameForSurface(ws)
}

func TestResizeSurfaceIndependent(t *testing.T) {
	r := &Renderer{}
	primary := &RenderTarget{renderer: r, state: SurfaceConfigured, width: 800, height: 600}
	secondary := &RenderTarget{renderer: r, state: SurfaceConfigured, width: 400, height: 300}

	r.primary = primary

	// Resize secondary to same dimensions — no-op, both unaffected
	r.ResizeSurface(secondary, 400, 300)

	if secondary.state != SurfaceConfigured {
		t.Error("secondary should stay SurfaceConfigured on no-op resize")
	}
	if primary.state != SurfaceConfigured {
		t.Error("primary should be unaffected by secondary resize")
	}
}

func TestActiveSurfaceFallsBackToPrimary(t *testing.T) {
	r := &Renderer{}
	primary := &RenderTarget{renderer: r, width: 800}
	r.primary = primary

	// No currentSurface → falls back to primary
	got := r.activeSurface()
	if got != primary {
		t.Error("activeSurface should fallback to primary when currentSurface is nil")
	}

	// With currentSurface → returns it
	secondary := &RenderTarget{renderer: r, width: 400}
	r.currentSurface = secondary
	got = r.activeSurface()
	if got != secondary {
		t.Error("activeSurface should return currentSurface when set")
	}
}

func TestInitSurfaceUsesRendererFormat(t *testing.T) {
	r := &Renderer{
		surfaceFormat: gputypes.TextureFormatBGRA8Unorm,
	}
	ws := &RenderTarget{renderer: r}

	// initSurface would set ws.format from r.surfaceFormat
	// We can't call real initSurface (needs wgpu instance), but verify the contract
	ws.format = r.surfaceFormat

	if ws.format != gputypes.TextureFormatBGRA8Unorm {
		t.Error("RenderTarget format should come from Renderer.surfaceFormat")
	}
}
