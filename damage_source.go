package gogpu

import (
	"image"
	"image/color"

	"github.com/gogpu/gpucontext"
)

// maxDamageRects is the maximum number of individual damage rectangles sent to
// the compositor before falling back to a single bounding box. Too many rects
// increase per-present overhead in Vulkan VK_KHR_incremental_present and
// Wayland wl_surface_damage_buffer without meaningful compositing savings.
const maxDamageRects = 32

// damagePalette assigns a visually distinct overlay color to each registered
// damage source. Colors are assigned by registration order (modulo palette
// length). Alpha is set low for subtle see-through overlay rendering.
//
// Enterprise references:
//   - Chromium PaintRect fill: alpha = 60/255 (~24%) at full step, fades to 0
//   - Chromium debug_colors.cc FadedGreen: initial_value=60 for fill, 255 for border
//   - GTK4 updatesoverlay.c: alpha = 0.4 * (1 - progress) (red, no per-source)
//
// Our palette stores the BORDER alpha (high). Fill alpha is computed as
// borderAlpha * 0.2 in drawRect (border=visible outline, fill=subtle tint).
var damagePalette = [8]color.RGBA{
	{R: 0, G: 220, B: 0, A: 180},    // green  — border 70%, fill 14%
	{R: 0, G: 100, B: 255, A: 180},  // blue
	{R: 255, G: 140, B: 0, A: 180},  // orange
	{R: 160, G: 32, B: 240, A: 180}, // purple
	{R: 0, G: 210, B: 210, A: 180},  // cyan
	{R: 255, G: 220, B: 0, A: 180},  // yellow
	{R: 220, G: 0, B: 180, A: 180},  // magenta
	{R: 50, G: 205, B: 50, A: 180},  // lime
}

// DamageSource is a concrete damage reporter for a single renderer registered
// with the compositor. Each independent renderer (gg, g3d, video, compose)
// registers one DamageSource via Context.RegisterDamageSource and reports
// per-frame damage through it.
//
// All fields are unexported — consumers interact through the
// gpucontext.DamageReporter interface (ReportDamage, ReportDamageWithReason).
//
// Thread safety: all damage operations happen on the render thread (same
// goroutine that calls OnDraw). No synchronization is required.
//
// DamageSource implements gpucontext.DamageReporter.
type DamageSource struct {
	name   string
	color  color.RGBA
	rects  []image.Rectangle
	full   bool
	reason gpucontext.DamageReason
}

// ReportDamage reports damage rectangles for this frame. Rects are in physical
// pixels (surface coordinates). Calling with no rects signals full-surface
// damage for this source. Damage is reset after present — the source must
// report damage every frame that content changes.
func (ds *DamageSource) ReportDamage(rects ...image.Rectangle) {
	if len(rects) == 0 {
		ds.full = true
	} else {
		ds.rects = append(ds.rects, rects...)
	}
}

// ReportDamageWithReason reports damage with a typed reason for debug overlay
// labels, structured logging, and future optimization heuristics. No rects
// signals full-surface damage. Reason is reset after present.
func (ds *DamageSource) ReportDamageWithReason(reason gpucontext.DamageReason, rects ...image.Rectangle) {
	ds.reason = reason
	ds.ReportDamage(rects...)
}

// reset clears per-frame state after present. The rects slice is reused
// (length zeroed, capacity retained) to avoid per-frame allocations.
func (ds *DamageSource) reset() {
	ds.rects = ds.rects[:0]
	ds.full = false
	ds.reason = gpucontext.DamageReason{}
}

// unionAllSources unions damage from all registered sources into a single
// rect slice for the compositor.
//
// Rules (following Chromium cc DamageTracker union pattern):
//   - If ANY source reports full damage (full=true) -> nil (full surface present).
//   - If no sources reported any damage -> nil (full present, safe default).
//   - If combined rect count > maxDamageRects -> single bounding box.
//   - Otherwise -> all rects from all sources.
func unionAllSources(sources []*DamageSource) []image.Rectangle {
	for _, ds := range sources {
		if ds.full {
			return nil
		}
	}

	var all []image.Rectangle
	for _, ds := range sources {
		all = append(all, ds.rects...)
	}

	if len(all) == 0 {
		return nil // no damage reported -> full present (safe default)
	}
	if len(all) > maxDamageRects {
		return []image.Rectangle{boundingBox(all)}
	}
	return all
}

// boundingBox computes the smallest rectangle enclosing all rects.
// Caller must ensure len(rects) > 0.
func boundingBox(rects []image.Rectangle) image.Rectangle {
	bb := rects[0]
	for _, r := range rects[1:] {
		bb = bb.Union(r)
	}
	return bb
}
