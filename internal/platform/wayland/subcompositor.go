//go:build linux

package wayland

// wl_subcompositor opcodes (requests)
const (
	subcompositorDestroy       Opcode = 0 // destroy()
	subcompositorGetSubsurface Opcode = 1 // get_subsurface(id: new_id, surface: object, parent: object)
)

// wl_subsurface opcodes (requests)
const (
	subsurfaceDestroy     Opcode = 0 // destroy()
	subsurfaceSetPosition Opcode = 1 // set_position(x: int, y: int)
	subsurfacePlaceAbove  Opcode = 2 // place_above(sibling: object<wl_surface>)
	subsurfacePlaceBelow  Opcode = 3 // place_below(sibling: object<wl_surface>)
	subsurfaceSetSync     Opcode = 4 // set_sync()
	subsurfaceSetDesync   Opcode = 5 // set_desync()
)

// WlSubcompositor represents the wl_subcompositor interface.
// It creates subsurfaces that are composited relative to a parent surface.
type WlSubcompositor struct {
	display *Display
	id      ObjectID
}

// NewWlSubcompositor creates a WlSubcompositor from a bound object ID.
func NewWlSubcompositor(display *Display, objectID ObjectID) *WlSubcompositor {
	return &WlSubcompositor{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID.
func (sc *WlSubcompositor) ID() ObjectID {
	return sc.id
}

// GetSubsurface creates a subsurface for the given surface, attached to parent.
// The surface must not already be a subsurface or have a role.
// The subsurface is initially in sync mode and positioned at (0, 0).
func (sc *WlSubcompositor) GetSubsurface(surface, parent *WlSurface) (*WlSubsurface, error) {
	subsurfaceID := sc.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(subsurfaceID)
	builder.PutObject(surface.ID())
	builder.PutObject(parent.ID())
	msg := builder.BuildMessage(sc.id, subcompositorGetSubsurface)

	if err := sc.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return &WlSubsurface{
		display: sc.display,
		id:      subsurfaceID,
		surface: surface,
		parent:  parent,
	}, nil
}

// Destroy destroys the subcompositor.
func (sc *WlSubcompositor) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(sc.id, subcompositorDestroy)
	return sc.display.SendMessage(msg)
}

// WlSubsurface represents the wl_subsurface interface.
// A subsurface is composited relative to its parent surface.
// It has its own wl_surface for content (buffers, damage, commit).
type WlSubsurface struct {
	display *Display
	id      ObjectID
	surface *WlSurface // the child surface
	parent  *WlSurface // the parent surface
}

// ID returns the object ID.
func (ss *WlSubsurface) ID() ObjectID {
	return ss.id
}

// Surface returns the child wl_surface associated with this subsurface.
func (ss *WlSubsurface) Surface() *WlSurface {
	return ss.surface
}

// Parent returns the parent wl_surface.
func (ss *WlSubsurface) Parent() *WlSurface {
	return ss.parent
}

// SetPosition sets the position of the subsurface relative to its parent.
// The position is applied on the next parent surface commit (double-buffered).
func (ss *WlSubsurface) SetPosition(x, y int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(x)
	builder.PutInt32(y)
	msg := builder.BuildMessage(ss.id, subsurfaceSetPosition)
	return ss.display.SendMessage(msg)
}

// PlaceAbove places this subsurface above the given sibling surface.
// The sibling must be a sibling subsurface or the parent surface.
func (ss *WlSubsurface) PlaceAbove(sibling *WlSurface) error {
	builder := NewMessageBuilder()
	builder.PutObject(sibling.ID())
	msg := builder.BuildMessage(ss.id, subsurfacePlaceAbove)
	return ss.display.SendMessage(msg)
}

// PlaceBelow places this subsurface below the given sibling surface.
func (ss *WlSubsurface) PlaceBelow(sibling *WlSurface) error {
	builder := NewMessageBuilder()
	builder.PutObject(sibling.ID())
	msg := builder.BuildMessage(ss.id, subsurfacePlaceBelow)
	return ss.display.SendMessage(msg)
}

// SetSync sets the subsurface to synchronized mode.
// In sync mode, the subsurface's state (position, buffer) is committed
// atomically with the parent surface's state. This is the default mode.
func (ss *WlSubsurface) SetSync() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(ss.id, subsurfaceSetSync)
	return ss.display.SendMessage(msg)
}

// SetDesync sets the subsurface to desynchronized mode.
// In desync mode, the subsurface's commits take effect immediately.
func (ss *WlSubsurface) SetDesync() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(ss.id, subsurfaceSetDesync)
	return ss.display.SendMessage(msg)
}

// Destroy destroys the subsurface. The wl_surface is not destroyed.
func (ss *WlSubsurface) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(ss.id, subsurfaceDestroy)
	return ss.display.SendMessage(msg)
}
