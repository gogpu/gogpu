package compositor

import (
	"image"
	"image/color"

	"github.com/gogpu/gpucontext"
)

// MaxDamageRects is the maximum number of individual damage rectangles sent to
// the compositor before falling back to a single bounding box.
const MaxDamageRects = 32

// DamagePalette assigns a visually distinct overlay color to each registered
// damage source. Colors are assigned by registration order (modulo palette
// length). Alpha is set for border rendering; fill alpha is computed dynamically.
var DamagePalette = [8]color.RGBA{
	{R: 0, G: 220, B: 0, A: 180},    // green
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
// registers one DamageSource and reports per-frame damage through it.
//
// DamageSource implements gpucontext.DamageReporter.
type DamageSource struct {
	Name   string
	Color  color.RGBA
	Rects  []image.Rectangle
	Full   bool
	Reason gpucontext.DamageReason
}

// NewDamageSource creates a DamageSource with the given name and palette color.
func NewDamageSource(name string, colorIdx int) *DamageSource {
	return &DamageSource{
		Name:  name,
		Color: DamagePalette[colorIdx%len(DamagePalette)],
	}
}

// ReportDamage reports damage rectangles for this frame.
func (ds *DamageSource) ReportDamage(rects ...image.Rectangle) {
	if len(rects) == 0 {
		ds.Full = true
	} else {
		ds.Rects = append(ds.Rects, rects...)
	}
}

// ReportDamageWithReason reports damage with a typed reason.
func (ds *DamageSource) ReportDamageWithReason(reason gpucontext.DamageReason, rects ...image.Rectangle) {
	ds.Reason = reason
	ds.ReportDamage(rects...)
}

// Reset clears per-frame state after present. Slice capacity is retained.
func (ds *DamageSource) Reset() {
	ds.Rects = ds.Rects[:0]
	ds.Full = false
	ds.Reason = gpucontext.DamageReason{}
}

// UnionAllSources unions damage from all registered sources into a single
// rect slice for the compositor (Chromium cc DamageTracker union pattern).
func UnionAllSources(sources []*DamageSource) []image.Rectangle {
	for _, ds := range sources {
		if ds.Full {
			return nil
		}
	}

	var all []image.Rectangle
	for _, ds := range sources {
		all = append(all, ds.Rects...)
	}

	if len(all) == 0 {
		return nil
	}
	if len(all) > MaxDamageRects {
		return []image.Rectangle{BoundingBox(all)}
	}
	return all
}

// BoundingBox computes the smallest rectangle enclosing all rects.
func BoundingBox(rects []image.Rectangle) image.Rectangle {
	bb := rects[0]
	for _, r := range rects[1:] {
		bb = bb.Union(r)
	}
	return bb
}
