package gogpu

import "image"

// computeDamageScissor computes the effective scissor rect as the intersection
// of group clip and frame damage rect, clamped to surface bounds.
// Returns (x, y, w, h, valid). When valid=false, the intersection is empty
// and all fragments should be discarded.
//
// This is the canonical surface-level damage scissor computation used by
// the compositor blit path (ADR-016). Content renderers (gg, g3d) produce
// damage rects; gogpu's compositor converts them to GPU scissor rects.
func computeDamageScissor(groupClip *[4]uint32, surfaceW, surfaceH uint32, damage image.Rectangle) (x, y, w, h uint32, valid bool) {
	sx, sy := damage.Min.X, damage.Min.Y
	sx2, sy2 := damage.Max.X, damage.Max.Y

	if groupClip != nil {
		gx := int(groupClip[0])
		gy := int(groupClip[1])
		gx2 := gx + int(groupClip[2])
		gy2 := gy + int(groupClip[3])
		sx = max(sx, gx)
		sy = max(sy, gy)
		sx2 = min(sx2, gx2)
		sy2 = min(sy2, gy2)
	}

	sx = max(sx, 0)
	sy = max(sy, 0)
	sx2 = min(sx2, int(surfaceW))
	sy2 = min(sy2, int(surfaceH))

	if sx2 <= sx || sy2 <= sy {
		return 0, 0, 0, 0, false
	}
	return uint32(sx), uint32(sy), uint32(sx2 - sx), uint32(sy2 - sy), true //nolint:gosec // clamped above
}

// damageRectsUnion returns the bounding box of all damage rects.
// Returns empty rect if slice is empty.
func damageRectsUnion(rects []image.Rectangle) image.Rectangle {
	if len(rects) == 0 {
		return image.Rectangle{}
	}
	u := rects[0]
	for _, r := range rects[1:] {
		u = u.Union(r)
	}
	return u
}
