package compositor

import (
	"fmt"
	"image"
	"log/slog"

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

// BlitOverlayDraw pairs a scissor rect with a draw recorder for overlay
// rendering. Each overlay can have its own scissor state.
type BlitOverlayDraw struct {
	// ScissorRect is the scissor rect in device pixels. nil means full surface.
	ScissorRect *[4]uint32
	// Recorder records the overlay's draw calls.
	Recorder BlitDrawRecorder
}

// BlitResources holds per-surface GPU resources for the surface
// compositor's MSAA overlay alpha-composite path. When an MSAA render pass
// produces a transparent overlay, it resolves into a single-sample texture.
// These resources blit that texture onto the existing surface content.
//
// Corresponds to gg's surfaceCompositeVertBuf/UniformBuf/BindGroup/BoundView
// that previously lived in GPURenderSession.
type BlitResources struct {
	VertBuf    *wgpu.Buffer
	UniformBuf *wgpu.Buffer
	BindGroup  *wgpu.BindGroup
	BoundView  *wgpu.TextureView
}

// ReleaseBinding drops the bind group before its sampled overlay resolve
// texture is destroyed or replaced. The vertex and uniform buffers remain
// reusable across surface resizes.
func (r *BlitResources) ReleaseBinding() {
	if r.BindGroup != nil {
		r.BindGroup.Release()
		r.BindGroup = nil
	}
	r.BoundView = nil
}

// Destroy releases all compositor blit resources.
func (r *BlitResources) Destroy() {
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

// ApplyOverlayScissorWithDamage applies the scissor rect for an overlay,
// intersecting with the damage union rect. Returns false if the intersection
// is empty (overlay entirely outside damage).
func ApplyOverlayScissorWithDamage(rp *wgpu.RenderPassEncoder, rect *[4]uint32, w, h uint32, damage image.Rectangle) bool {
	if damage.Empty() {
		// No damage constraint — use group scissor or full surface.
		if rect != nil {
			rp.SetScissorRect(gputypes.ScissorRect{X: rect[0], Y: rect[1], Width: rect[2], Height: rect[3]})
		} else {
			rp.SetScissorRect(gputypes.ScissorRect{X: 0, Y: 0, Width: w, Height: h})
		}
		return true
	}
	dx, dy, dw, dh, valid := ComputeDamageScissor(rect, w, h, damage)
	if !valid {
		return false
	}
	rp.SetScissorRect(gputypes.ScissorRect{X: dx, Y: dy, Width: dw, Height: dh})
	return true
}

// RecordBaseBlitDraws records base layer draws with per-rect damage scissoring.
func RecordBaseBlitDraws(rp *wgpu.RenderPassEncoder, base BlitDrawRecorder, hasDamage bool, rects []image.Rectangle, w, h uint32) {
	if base == nil {
		return
	}
	if hasDamage {
		for _, dr := range rects {
			dx, dy, dw, dh, valid := ComputeDamageScissor(nil, w, h, dr)
			if valid {
				rp.SetScissorRect(gputypes.ScissorRect{X: dx, Y: dy, Width: dw, Height: dh})
				base.RecordBlitDraws(rp)
			}
		}
	} else {
		base.RecordBlitDraws(rp)
	}
}

// RecordOverlayBlitDraws records overlay draws with per-overlay damage scissoring.
func RecordOverlayBlitDraws(rp *wgpu.RenderPassEncoder, overlays []BlitOverlayDraw, rects []image.Rectangle, w, h uint32) {
	if len(overlays) == 0 {
		return
	}
	damageUnion := DamageRectsUnion(rects)
	for _, overlay := range overlays {
		if ApplyOverlayScissorWithDamage(rp, overlay.ScissorRect, w, h, damageUnion) {
			overlay.Recorder.RecordBlitDraws(rp)
		}
	}
}

// EncodeBlitPass records a non-MSAA blit render pass to the swapchain surface
// with damage-aware scissoring. Exported for use by content renderers that
// need compositor-controlled blit passes.
func EncodeBlitPass(
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
	rp.SetViewport(gputypes.Viewport{X: 0, Y: 0, Width: float32(w), Height: float32(h), MinDepth: 0, MaxDepth: 1})

	// Base layer: per-rect scissor when damage rects exist (ADR-028).
	RecordBaseBlitDraws(rp, baseRecorder, hasDamage, damageRects, w, h)

	// Overlay draws: per-overlay scissor intersected with damage union.
	RecordOverlayBlitDraws(rp, overlayRecorders, damageRects, w, h)

	if endErr := rp.End(); endErr != nil {
		slog.Warn("compositor blit pass End failed", "err", endErr)
	}
	return nil
}
