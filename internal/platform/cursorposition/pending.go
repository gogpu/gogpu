// Package cursorposition tells the pointer motion that setting the cursor's
// position caused from the motion the user made.
//
// Moving the cursor with SetCursorPos (Windows) or XWarpPointer (X11) makes
// the system report a motion event at the new position, as if the user had
// moved there. Setting the position produces no pointer event of its own, so
// that event is recognized by its position and dropped.
package cursorposition

// limit bounds the positions kept for motion to match, should their motion
// never arrive; the oldest go first.
const limit = 16

// Pending is the positions the cursor was set to whose motion event has not
// arrived yet, oldest first. Positions are in the window's device pixels, as
// its motion events carry them. The zero value is empty and ready to use.
type Pending struct {
	positions []position
}

// position is where the cursor was set.
type position struct {
	x int32
	y int32
}

// Add remembers that the cursor was set to (x, y), whose motion is yet to
// arrive.
func (p *Pending) Add(x, y int32) {
	if len(p.positions) == limit {
		p.positions = append(p.positions[:0], p.positions[1:]...)
	}
	p.positions = append(p.positions, position{x: x, y: y})
}

// Arrived reports whether motion at (x, y) is a pending position's. When it
// is, that position and every earlier one are forgotten: their motion came,
// or was merged into this. Motion anywhere else is the user's and changes
// nothing, since a pending position's motion may still be queued behind it.
func (p *Pending) Arrived(x, y int32) bool {
	for i, pending := range p.positions {
		if pending == (position{x: x, y: y}) {
			p.positions = append(p.positions[:0], p.positions[i+1:]...)
			return true
		}
	}
	return false
}

// Clear forgets every pending position.
func (p *Pending) Clear() {
	p.positions = p.positions[:0]
}
