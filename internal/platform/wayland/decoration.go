//go:build linux

package wayland

import (
	"fmt"
	"sync"
)

// Decoration mode constants for zxdg_toplevel_decoration_v1.
const (
	DecorationModeClientSide uint32 = 1 // Client draws decorations
	DecorationModeServerSide uint32 = 2 // Server draws decorations
)

// zxdg_decoration_manager_v1 opcodes (requests)
const (
	zxdgDecorationManagerDestroy               Opcode = 0 // destroy()
	zxdgDecorationManagerGetToplevelDecoration Opcode = 1 // get_toplevel_decoration(new_id, toplevel)
)

// zxdg_toplevel_decoration_v1 opcodes (requests)
const (
	zxdgToplevelDecorationDestroy   Opcode = 0 // destroy()
	zxdgToplevelDecorationSetMode   Opcode = 1 // set_mode(mode: uint)
	zxdgToplevelDecorationUnsetMode Opcode = 2 // unset_mode()
)

// zxdg_toplevel_decoration_v1 event opcodes
const (
	zxdgToplevelDecorationEventConfigure Opcode = 0 // configure(mode: uint)
)

// ZxdgDecorationManager represents the zxdg_decoration_manager_v1 interface.
// It allows clients to request server-side or client-side window decorations.
type ZxdgDecorationManager struct {
	display *Display
	id      ObjectID
}

// NewZxdgDecorationManager creates a ZxdgDecorationManager from a bound object ID.
func NewZxdgDecorationManager(display *Display, objectID ObjectID) *ZxdgDecorationManager {
	return &ZxdgDecorationManager{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID.
func (m *ZxdgDecorationManager) ID() ObjectID {
	return m.id
}

// Destroy destroys the decoration manager.
func (m *ZxdgDecorationManager) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(m.id, zxdgDecorationManagerDestroy)

	return m.display.SendMessage(msg)
}

// GetToplevelDecoration creates a decoration object for the given xdg_toplevel.
func (m *ZxdgDecorationManager) GetToplevelDecoration(toplevel *XdgToplevel) (*ZxdgToplevelDecoration, error) {
	decorID := m.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(decorID)
	builder.PutObject(toplevel.ID())
	msg := builder.BuildMessage(m.id, zxdgDecorationManagerGetToplevelDecoration)

	if err := m.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewZxdgToplevelDecoration(m.display, decorID), nil
}

// ZxdgToplevelDecoration represents the zxdg_toplevel_decoration_v1 interface.
// It allows requesting a specific decoration mode for a toplevel window.
type ZxdgToplevelDecoration struct {
	display *Display
	id      ObjectID

	mu sync.Mutex

	// Current mode set by the compositor via configure event
	mode        uint32
	onConfigure func(mode uint32)
}

// NewZxdgToplevelDecoration creates a ZxdgToplevelDecoration from an object ID.
// It auto-registers with Display for event dispatch (configure events).
func NewZxdgToplevelDecoration(display *Display, objectID ObjectID) *ZxdgToplevelDecoration {
	d := &ZxdgToplevelDecoration{
		display: display,
		id:      objectID,
	}
	if display != nil {
		display.RegisterObject(objectID, d)
	}
	return d
}

// ID returns the object ID.
func (d *ZxdgToplevelDecoration) ID() ObjectID {
	return d.id
}

// Mode returns the current decoration mode set by the compositor.
func (d *ZxdgToplevelDecoration) Mode() uint32 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mode
}

// Destroy destroys the toplevel decoration object.
func (d *ZxdgToplevelDecoration) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(d.id, zxdgToplevelDecorationDestroy)

	return d.display.SendMessage(msg)
}

// SetMode requests a specific decoration mode.
// Use DecorationModeClientSide or DecorationModeServerSide.
func (d *ZxdgToplevelDecoration) SetMode(mode uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(mode)
	msg := builder.BuildMessage(d.id, zxdgToplevelDecorationSetMode)

	return d.display.SendMessage(msg)
}

// UnsetMode removes any previously set decoration mode preference.
// The compositor will choose the mode.
func (d *ZxdgToplevelDecoration) UnsetMode() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(d.id, zxdgToplevelDecorationUnsetMode)

	return d.display.SendMessage(msg)
}

// SetConfigureHandler sets a callback for the configure event.
// The handler receives the decoration mode chosen by the compositor.
func (d *ZxdgToplevelDecoration) SetConfigureHandler(handler func(mode uint32)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onConfigure = handler
}

// dispatch handles zxdg_toplevel_decoration_v1 events.
func (d *ZxdgToplevelDecoration) dispatch(msg *Message) error {
	switch msg.Opcode {
	case zxdgToplevelDecorationEventConfigure:
		return d.handleConfigure(msg)
	default:
		return nil
	}
}

// handleConfigure handles the configure event.
func (d *ZxdgToplevelDecoration) handleConfigure(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	mode, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: zxdg_toplevel_decoration.configure: failed to decode mode: %w", err)
	}

	d.mu.Lock()
	d.mode = mode
	handler := d.onConfigure
	d.mu.Unlock()

	if handler != nil {
		handler(mode)
	}

	return nil
}
