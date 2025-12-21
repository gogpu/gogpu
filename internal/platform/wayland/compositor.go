//go:build linux

package wayland

import (
	"sync"
)

// wl_compositor opcodes (requests)
const (
	compositorCreateSurface Opcode = 0 // create_surface(id: new_id<wl_surface>)
	compositorCreateRegion  Opcode = 1 // create_region(id: new_id<wl_region>)
)

// wl_surface opcodes (requests)
const (
	surfaceDestroy            Opcode = 0 // destroy()
	surfaceAttach             Opcode = 1 // attach(buffer: object<wl_buffer>, x: int, y: int)
	surfaceDamage             Opcode = 2 // damage(x: int, y: int, width: int, height: int)
	surfaceFrame              Opcode = 3 // frame(callback: new_id<wl_callback>)
	surfaceSetOpaqueRegion    Opcode = 4 // set_opaque_region(region: object<wl_region>)
	surfaceSetInputRegion     Opcode = 5 // set_input_region(region: object<wl_region>)
	surfaceCommit             Opcode = 6 // commit()
	surfaceSetBufferTransform Opcode = 7 // set_buffer_transform(transform: int) [v2]
	surfaceSetBufferScale     Opcode = 8 // set_buffer_scale(scale: int) [v3]
	surfaceDamageBuffer       Opcode = 9 // damage_buffer(x: int, y: int, width: int, height: int) [v4]
)

// wl_surface event opcodes
const (
	surfaceEventEnter Opcode = 0 // enter(output: object<wl_output>)
	surfaceEventLeave Opcode = 1 // leave(output: object<wl_output>)
)

// wl_callback event opcodes (already defined in display.go as callbackEventDone)
// Keeping reference here for documentation:
// const callbackEventDone Opcode = 0 // done(callback_data: uint)

// WlCompositor represents the wl_compositor interface.
// It is responsible for creating surfaces and regions.
type WlCompositor struct {
	display *Display
	id      ObjectID
}

// NewWlCompositor creates a WlCompositor from a bound object ID.
// The objectID should be obtained from Registry.BindCompositor().
func NewWlCompositor(display *Display, objectID ObjectID) *WlCompositor {
	return &WlCompositor{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID of the compositor.
func (c *WlCompositor) ID() ObjectID {
	return c.id
}

// CreateSurface creates a new surface.
// The returned surface is used for creating a window via xdg_shell or for
// Vulkan/EGL surface creation.
func (c *WlCompositor) CreateSurface() (*WlSurface, error) {
	surfaceID := c.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(surfaceID)
	msg := builder.BuildMessage(c.id, compositorCreateSurface)

	if err := c.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewWlSurface(c.display, surfaceID), nil
}

// WlSurface represents the wl_surface interface.
// A surface is a rectangular area used to display content.
// Surfaces are used as the basis for windows, popups, and subsurfaces.
type WlSurface struct {
	display *Display
	id      ObjectID

	mu sync.Mutex

	// Event handlers
	onEnter func(outputID ObjectID)
	onLeave func(outputID ObjectID)
}

// NewWlSurface creates a WlSurface from an object ID.
func NewWlSurface(display *Display, objectID ObjectID) *WlSurface {
	return &WlSurface{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID of the surface.
func (s *WlSurface) ID() ObjectID {
	return s.id
}

// Ptr returns the object ID as a uintptr for use with Vulkan surface creation.
// This is used with VK_KHR_wayland_surface extension.
// Note: In pure Go Wayland implementation, we return the object ID since we don't
// have a C wl_surface pointer. The GPU backend will need to handle this appropriately.
func (s *WlSurface) Ptr() uintptr {
	return uintptr(s.id)
}

// Attach attaches a buffer to the surface.
// The x and y arguments specify the offset from the new buffer's position
// to the current surface position.
// If buffer is 0, the surface is unmapped.
func (s *WlSurface) Attach(buffer ObjectID, x, y int32) error {
	builder := NewMessageBuilder()
	builder.PutObject(buffer)
	builder.PutInt32(x)
	builder.PutInt32(y)
	msg := builder.BuildMessage(s.id, surfaceAttach)

	return s.display.SendMessage(msg)
}

// Damage marks a region of the surface as damaged.
// This tells the compositor which parts of the surface have changed.
// The compositor uses this to optimize repainting.
func (s *WlSurface) Damage(x, y, width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(s.id, surfaceDamage)

	return s.display.SendMessage(msg)
}

// DamageBuffer marks a region of the buffer as damaged (version 4+).
// This is similar to Damage but uses buffer coordinates instead of
// surface coordinates.
func (s *WlSurface) DamageBuffer(x, y, width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(s.id, surfaceDamageBuffer)

	return s.display.SendMessage(msg)
}

// Frame requests a frame callback for animation synchronization.
// The returned callback will be triggered when it's time to draw the next frame.
func (s *WlSurface) Frame() (*WlCallback, error) {
	callbackID := s.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(callbackID)
	msg := builder.BuildMessage(s.id, surfaceFrame)

	if err := s.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewWlCallback(s.display, callbackID), nil
}

// SetOpaqueRegion sets the opaque region of the surface.
// The compositor may optimize painting by not drawing content behind opaque regions.
// Pass 0 to unset the opaque region.
func (s *WlSurface) SetOpaqueRegion(region ObjectID) error {
	builder := NewMessageBuilder()
	builder.PutObject(region)
	msg := builder.BuildMessage(s.id, surfaceSetOpaqueRegion)

	return s.display.SendMessage(msg)
}

// SetInputRegion sets the input region of the surface.
// Only input events within this region will be delivered to the client.
// Pass 0 to accept input on the entire surface.
func (s *WlSurface) SetInputRegion(region ObjectID) error {
	builder := NewMessageBuilder()
	builder.PutObject(region)
	msg := builder.BuildMessage(s.id, surfaceSetInputRegion)

	return s.display.SendMessage(msg)
}

// Commit commits the pending surface state.
// This atomically applies all pending changes (buffer, damage, etc.)
// and submits them to the compositor.
func (s *WlSurface) Commit() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(s.id, surfaceCommit)

	return s.display.SendMessage(msg)
}

// SetBufferTransform sets the buffer transformation (version 2+).
// The transform specifies how to rotate/flip the buffer contents.
func (s *WlSurface) SetBufferTransform(transform int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(transform)
	msg := builder.BuildMessage(s.id, surfaceSetBufferTransform)

	return s.display.SendMessage(msg)
}

// SetBufferScale sets the buffer scale factor (version 3+).
// This is used for HiDPI displays. A scale of 2 means each buffer pixel
// covers 2x2 surface pixels.
func (s *WlSurface) SetBufferScale(scale int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(scale)
	msg := builder.BuildMessage(s.id, surfaceSetBufferScale)

	return s.display.SendMessage(msg)
}

// Destroy destroys the surface.
// All resources associated with this surface are released.
func (s *WlSurface) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(s.id, surfaceDestroy)

	return s.display.SendMessage(msg)
}

// SetEnterHandler sets a callback for the enter event.
// The handler is called when the surface enters an output (monitor).
func (s *WlSurface) SetEnterHandler(handler func(outputID ObjectID)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEnter = handler
}

// SetLeaveHandler sets a callback for the leave event.
// The handler is called when the surface leaves an output (monitor).
func (s *WlSurface) SetLeaveHandler(handler func(outputID ObjectID)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onLeave = handler
}

// dispatch handles wl_surface events.
func (s *WlSurface) dispatch(msg *Message) error {
	switch msg.Opcode {
	case surfaceEventEnter:
		return s.handleEnter(msg)
	case surfaceEventLeave:
		return s.handleLeave(msg)
	default:
		return nil
	}
}

func (s *WlSurface) handleEnter(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	outputID, err := decoder.Object()
	if err != nil {
		return err
	}

	s.mu.Lock()
	handler := s.onEnter
	s.mu.Unlock()

	if handler != nil {
		handler(outputID)
	}
	return nil
}

func (s *WlSurface) handleLeave(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	outputID, err := decoder.Object()
	if err != nil {
		return err
	}

	s.mu.Lock()
	handler := s.onLeave
	s.mu.Unlock()

	if handler != nil {
		handler(outputID)
	}
	return nil
}

// WlCallback represents the wl_callback interface.
// Callbacks are used for one-shot notifications, typically for frame synchronization.
type WlCallback struct {
	display *Display
	id      ObjectID

	mu   sync.Mutex
	done chan uint32
}

// NewWlCallback creates a WlCallback from an object ID.
func NewWlCallback(display *Display, objectID ObjectID) *WlCallback {
	return &WlCallback{
		display: display,
		id:      objectID,
		done:    make(chan uint32, 1),
	}
}

// ID returns the object ID of the callback.
func (c *WlCallback) ID() ObjectID {
	return c.id
}

// Done returns a channel that receives the callback data when the callback fires.
// The channel is closed after the callback fires.
func (c *WlCallback) Done() <-chan uint32 {
	return c.done
}

// dispatch handles wl_callback events.
func (c *WlCallback) dispatch(msg *Message) error {
	if msg.Opcode == callbackEventDone {
		decoder := NewDecoder(msg.Args)
		data, err := decoder.Uint32()
		if err != nil {
			return err
		}

		c.mu.Lock()
		if c.done != nil {
			c.done <- data
			close(c.done)
			c.done = nil
		}
		c.mu.Unlock()
	}
	return nil
}
