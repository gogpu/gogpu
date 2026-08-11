package gogpu

import (
	"fmt"
	"image"

	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/wgpu"
)

// EncodeCompositorBlitPass records a non-MSAA blit render pass to the
// swapchain surface with damage-aware scissoring. Exported for use by
// content renderers that need compositor-controlled blit passes.
func (r *Renderer) EncodeCompositorBlitPass(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	w, h uint32,
	preserveContent bool,
	damageRects []image.Rectangle,
	baseRecorder compositor.BlitDrawRecorder,
	overlayRecorders []compositor.BlitOverlayDraw,
) error {
	return compositor.EncodeBlitPass(encoder, view, w, h, preserveContent, damageRects, baseRecorder, overlayRecorders)
}

// encodeSurfaceCompositePass alpha-blends a transparent overlay resolve texture
// onto the existing single-sample surface using the blit pipeline's alpha
// blending mode. Used when MSAA overlay rendering resolves into an intermediate
// texture that must be composited on top of previous surface content.
func (r *Renderer) encodeSurfaceCompositePass(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	compositeView *wgpu.TextureView,
	w, h uint32,
) error {
	if !r.blitPipeline.Inited {
		if err := r.initBlitPipeline(); err != nil {
			return fmt.Errorf("gogpu: blit pipeline init: %w", err)
		}
	}

	ws := r.currentSurface
	if ws == nil {
		return fmt.Errorf("gogpu: no current surface for composite")
	}
	return r.blitPipeline.EncodeSurfaceCompositePass(encoder, view, compositeView, w, h, &ws.compositeState)
}

// CompositorShouldPreserve reports whether the compositor should use
// LoadOpLoad for a surface render pass, preserving existing content.
// True when the surface already has content from this frame (frameCleared)
// or when an external renderer has drawn to it.
func (ws *RenderTarget) CompositorShouldPreserve() bool {
	return ws.frameCleared
}

// --- gpucontext.SurfaceCompositor implementation on RenderTarget ---

// ShouldPreserveContent reports whether the current render pass should use
// LoadOpLoad to preserve existing surface content.
func (ws *RenderTarget) ShouldPreserveContent() bool {
	return ws.externalContent
}

// DamageRects returns the damage rectangles for the current frame by
// unioning all registered damage sources.
func (ws *RenderTarget) DamageRects() []image.Rectangle {
	union := compositor.UnionAllSources(ws.damageSources)
	return union
}

// MarkContentRendered signals that content has been drawn to this surface.
func (ws *RenderTarget) MarkContentRendered() {
	ws.frameCleared = true
}

// CompositeMSAAOverlay alpha-blends the resolved MSAA overlay onto the
// swapchain surface using gogpu's dedicated blit pipeline.
func (ws *RenderTarget) CompositeMSAAOverlay(encoder gpucontext.CommandEncoder, view gpucontext.TextureView, compositeView gpucontext.TextureView, w, h uint32) error {
	if ws.renderer == nil {
		return fmt.Errorf("gogpu: no renderer for surface composite")
	}
	if encoder.IsNil() || view.IsNil() || compositeView.IsNil() {
		return fmt.Errorf("gogpu: nil handle in CompositeMSAAOverlay")
	}
	enc := (*wgpu.CommandEncoder)(encoder.Pointer())
	swapView := (*wgpu.TextureView)(view.Pointer())
	compView := (*wgpu.TextureView)(compositeView.Pointer())
	return ws.renderer.encodeSurfaceCompositePass(enc, swapView, compView, w, h)
}
