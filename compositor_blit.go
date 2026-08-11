package gogpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"math"

	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// BlitDrawRecorder is the interface content renderers (gg, g3d) implement to
// record draw calls into a compositor-owned render pass. The compositor owns
// the render pass lifecycle (BeginRenderPass, LoadOp, scissor, End) and lets
// each renderer record its draws at the appropriate point.
//
// This decouples the compositor (gogpu) from renderer internals. Each renderer
// implements RecordBlitDraws with its own pipeline/bind-group logic.
type BlitDrawRecorder interface {
	// RecordBlitDraws records non-MSAA textured quad draws into the render pass.
	// The pass is already begun with the correct LoadOp and viewport.
	// The recorder must only call SetPipeline/SetBindGroup/SetVertexBuffer/Draw.
	RecordBlitDraws(pass *wgpu.RenderPassEncoder)
}

// CompositorBlitResources holds per-surface GPU resources for the surface
// compositor's MSAA overlay alpha-composite path. When an MSAA render pass
// produces a transparent overlay, it resolves into a single-sample texture.
// These resources blit that texture onto the existing surface content.
//
// Corresponds to gg's surfaceCompositeVertBuf/UniformBuf/BindGroup/BoundView
// that previously lived in GPURenderSession.
type CompositorBlitResources struct {
	VertBuf    *wgpu.Buffer
	UniformBuf *wgpu.Buffer
	BindGroup  *wgpu.BindGroup
	BoundView  *wgpu.TextureView
}

// ReleaseBinding drops the bind group before its sampled overlay resolve
// texture is destroyed or replaced. The vertex and uniform buffers remain
// reusable across surface resizes.
func (r *CompositorBlitResources) ReleaseBinding() {
	if r.BindGroup != nil {
		r.BindGroup.Release()
		r.BindGroup = nil
	}
	r.BoundView = nil
}

// Destroy releases all compositor blit resources.
func (r *CompositorBlitResources) Destroy() {
	r.ReleaseBinding()
	if r.UniformBuf != nil {
		r.UniformBuf.Release()
		r.UniformBuf = nil
	}
	if r.VertBuf != nil {
		r.VertBuf.Release()
		r.VertBuf = nil
	}
}

// EncodeCompositorBlitPass records a non-MSAA blit render pass to the
// swapchain surface with damage-aware scissoring. Exported for use by
// content renderers that need compositor-controlled blit passes.
func (r *Renderer) EncodeCompositorBlitPass(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	w, h uint32,
	preserveContent bool,
	damageRects []image.Rectangle,
	baseRecorder BlitDrawRecorder,
	overlayRecorders []BlitOverlayDraw,
) error {
	hasDamage := len(damageRects) > 0
	loadOp := gputypes.LoadOpClear
	if hasDamage || preserveContent {
		loadOp = gputypes.LoadOpLoad
	}

	rp, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "compositor_blit_pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     loadOp,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 1},
		}},
	})
	if err != nil {
		return fmt.Errorf("begin compositor blit pass: %w", err)
	}
	rp.SetViewport(0, 0, float32(w), float32(h), 0, 1)

	// Base layer: per-rect scissor when damage rects exist (ADR-028).
	r.recordBaseBlitDraws(rp, baseRecorder, hasDamage, damageRects, w, h)

	// Overlay draws: per-overlay scissor intersected with damage union.
	r.recordOverlayBlitDraws(rp, overlayRecorders, damageRects, w, h)

	if endErr := rp.End(); endErr != nil {
		slog.Warn("compositor blit pass End failed", "err", endErr)
	}
	return nil
}

// recordBaseBlitDraws records base layer draws with per-rect damage scissoring.
func (r *Renderer) recordBaseBlitDraws(rp *wgpu.RenderPassEncoder, base BlitDrawRecorder, hasDamage bool, rects []image.Rectangle, w, h uint32) {
	if base == nil {
		return
	}
	if hasDamage {
		for _, dr := range rects {
			dx, dy, dw, dh, valid := compositor.ComputeDamageScissor(nil, w, h, dr)
			if valid {
				rp.SetScissorRect(dx, dy, dw, dh)
				base.RecordBlitDraws(rp)
			}
		}
	} else {
		base.RecordBlitDraws(rp)
	}
}

// recordOverlayBlitDraws records overlay draws with per-overlay damage scissoring.
func (r *Renderer) recordOverlayBlitDraws(rp *wgpu.RenderPassEncoder, overlays []BlitOverlayDraw, rects []image.Rectangle, w, h uint32) {
	if len(overlays) == 0 {
		return
	}
	damageUnion := compositor.DamageRectsUnion(rects)
	for _, overlay := range overlays {
		if ApplyOverlayScissorWithDamage(rp, overlay.ScissorRect, w, h, damageUnion) {
			overlay.Recorder.RecordBlitDraws(rp)
		}
	}
}

// BlitOverlayDraw pairs a scissor rect with a draw recorder for overlay
// rendering. Each overlay can have its own scissor state.
type BlitOverlayDraw struct {
	// ScissorRect is the scissor rect in device pixels. nil means full surface.
	ScissorRect *[4]uint32
	// Recorder records the overlay's draw calls.
	Recorder BlitDrawRecorder
}

// applyOverlayScissorWithDamage applies the scissor rect for an overlay,
// intersecting with the damage union rect. Returns false if the intersection
// is empty (overlay entirely outside damage).
func ApplyOverlayScissorWithDamage(rp *wgpu.RenderPassEncoder, rect *[4]uint32, w, h uint32, damage image.Rectangle) bool {
	if damage.Empty() {
		// No damage constraint — use group scissor or full surface.
		if rect != nil {
			rp.SetScissorRect(rect[0], rect[1], rect[2], rect[3])
		} else {
			rp.SetScissorRect(0, 0, w, h)
		}
		return true
	}
	dx, dy, dw, dh, valid := compositor.ComputeDamageScissor(rect, w, h, damage)
	if !valid {
		return false
	}
	rp.SetScissorRect(dx, dy, dw, dh)
	return true
}

// encodeSurfaceCompositePass alpha-blends a transparent overlay resolve texture
// onto the existing single-sample surface using the blit pipeline's alpha
// blending mode. Used when MSAA overlay rendering resolves into an intermediate
// texture that must be composited on top of previous surface content.
//
// This replaces gg's encodeSurfaceCompositePass. The pipeline is gogpu's
// dedicated blit pipeline (blitPipeline) with its own resources, independent
// of any content renderer's pipelines.
func (r *Renderer) encodeSurfaceCompositePass(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	compositeView *wgpu.TextureView,
	w, h uint32,
) error {
	if compositeView == nil || view == nil {
		return fmt.Errorf("gogpu: nil view in surface composite pass")
	}
	if !r.blitPipelineInited {
		if err := r.initBlitPipeline(); err != nil {
			return fmt.Errorf("gogpu: blit pipeline init: %w", err)
		}
	}

	// Persistent bind group for the composite texture view — recreated only
	// when compositeView changes (same pattern as old gg surfaceCompositeBindGroup).
	// MUST NOT defer Release — shared encoder is submitted AFTER this function returns.
	ws := r.currentSurface
	if ws == nil {
		return fmt.Errorf("gogpu: no current surface for composite")
	}
	if ws.compositeBindGroup == nil || ws.compositeBoundView != compositeView {
		if ws.compositeBindGroup != nil {
			ws.pendingCompositeRelease = ws.compositeBindGroup
		}
		bg, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Surface Composite Texture Bind Group",
			Layout: r.blitTextureLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Sampler: r.blitSampler},
				{Binding: 1, TextureView: compositeView},
			},
		})
		if err != nil {
			return fmt.Errorf("gogpu: create surface composite bind group: %w", err)
		}
		ws.compositeBindGroup = bg
		ws.compositeBoundView = compositeView
	}
	texBindGrp := ws.compositeBindGroup

	// Upload full-surface quad uniforms.
	binary.LittleEndian.PutUint32(r.blitUniformData[0:4], math.Float32bits(0))            // x
	binary.LittleEndian.PutUint32(r.blitUniformData[4:8], math.Float32bits(0))            // y
	binary.LittleEndian.PutUint32(r.blitUniformData[8:12], math.Float32bits(float32(w)))  // width
	binary.LittleEndian.PutUint32(r.blitUniformData[12:16], math.Float32bits(float32(h))) // height
	binary.LittleEndian.PutUint32(r.blitUniformData[16:20], math.Float32bits(float32(w))) // screenWidth
	binary.LittleEndian.PutUint32(r.blitUniformData[20:24], math.Float32bits(float32(h))) // screenHeight
	binary.LittleEndian.PutUint32(r.blitUniformData[24:28], math.Float32bits(1.0))        // alpha
	binary.LittleEndian.PutUint32(r.blitUniformData[28:32], math.Float32bits(1.0))        // premultiplied
	if err := r.device.Queue().WriteBuffer(r.blitUniformBuf, 0, r.blitUniformData); err != nil {
		return fmt.Errorf("gogpu: surface composite WriteBuffer: %w", err)
	}

	rp, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "surface_composite_pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     gputypes.LoadOpLoad,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return fmt.Errorf("gogpu: begin surface composite pass: %w", err)
	}
	rp.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	rp.SetScissorRect(0, 0, w, h)

	rp.SetPipeline(r.compositePipeline)
	rp.SetBindGroup(0, r.blitUniformBindGrp, nil)
	rp.SetBindGroup(1, texBindGrp, nil)
	rp.Draw(6, 1, 0, 0) // 6 vertices (2 triangles) for full-screen quad

	if endErr := rp.End(); endErr != nil {
		return fmt.Errorf("gogpu: end surface composite pass: %w", endErr)
	}
	return nil
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
