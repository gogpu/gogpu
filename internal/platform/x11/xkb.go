//go:build linux

package x11

import (
	"fmt"
)

// XKB extension protocol constants.
const (
	// XkbExtensionName is the X11 extension name for XKEYBOARD.
	XkbExtensionName = "XKEYBOARD"

	// XKB minor opcodes.
	XkbMinorOpcodeUseExtension = 0 // negotiate version
	XkbMinorOpcodeSelectEvents = 1 // subscribe to events
	XkbMinorOpcodeGetState     = 4 // get current keyboard state

	// XKB event sub-types (byte 1 of the XKB event).
	XkbStateNotify = 2

	// XKB event type masks for SelectEvents (affectWhich field).
	XkbNewKeyboardNotifyMask = 0x0001 // bit 0: new keyboard attached
	XkbMapNotifyMask         = 0x0002 // bit 1: keymap changed
	XkbStateNotifyMask       = 0x0004 // bit 2: state changes (group, modifiers)

	// XKB state detail masks for per-event details (affectState/stateDetails).
	XkbGroupStateMask = 0x0010 // group changes specifically

	// XKB device spec: use the core keyboard.
	XkbUseCoreKbd = 0x0100
)

// XkbExtension holds XKB extension state after initialization.
type XkbExtension struct {
	MajorOpcode uint8  // Assigned opcode from QueryExtension
	EventBase   uint8  // Base event code for XKB events
	ErrorBase   uint8  // Base error code
	MajorVer    uint16 // Negotiated major version
	MinorVer    uint16 // Negotiated minor version
	Group       int    // Current keyboard group (0-3)
}

// InitXkb queries and initializes the XKB extension.
// Returns nil and an error if XKB is not available or version negotiation fails.
func (c *Connection) InitXkb() (*XkbExtension, error) {
	// Step 1: QueryExtension for "XKEYBOARD"
	ext, err := c.QueryExtension(XkbExtensionName)
	if err != nil {
		return nil, fmt.Errorf("xkb: QueryExtension failed: %w", err)
	}
	if !ext.Present {
		return nil, fmt.Errorf("xkb: extension not available")
	}

	xkb := &XkbExtension{
		MajorOpcode: ext.MajorOpcode,
		EventBase:   ext.FirstEvent,
		ErrorBase:   ext.FirstError,
	}

	// Step 2: XkbUseExtension — negotiate version 1.0
	major, minor, err := c.xkbUseExtension(xkb.MajorOpcode, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("xkb: UseExtension failed: %w", err)
	}
	xkb.MajorVer = major
	xkb.MinorVer = minor

	// Step 3: XkbGetState — get initial keyboard group
	group, err := c.xkbGetState(xkb.MajorOpcode)
	if err != nil {
		return nil, fmt.Errorf("xkb: GetState failed: %w", err)
	}
	xkb.Group = group

	// Step 4: XkbSelectEvents — subscribe to XkbStateNotify for group changes
	if err := c.xkbSelectEvents(xkb.MajorOpcode); err != nil {
		return nil, fmt.Errorf("xkb: SelectEvents failed: %w", err)
	}

	return xkb, nil
}

// xkbUseExtension sends an XkbUseExtension request (minor opcode 0) and returns
// the server-supported version. This negotiates the protocol version with the server.
func (c *Connection) xkbUseExtension(majorOpcode uint8, wantMajor, wantMinor uint16) (uint16, uint16, error) {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(majorOpcode)
	e.PutUint8(XkbMinorOpcodeUseExtension)
	e.PutUint16(2) // request length: 8 bytes / 4 = 2 units
	e.PutUint16(wantMajor)
	e.PutUint16(wantMinor)

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return 0, 0, err
	}

	if len(reply) < 14 {
		return 0, 0, fmt.Errorf("xkb: UseExtension reply too short (%d bytes)", len(reply))
	}

	// Reply layout:
	//   byte 1: supported (bool)
	//   bytes 2-3: sequence
	//   bytes 4-7: length
	//   bytes 8-9: server major
	//   bytes 10-11: server minor
	supported := reply[1]
	if supported == 0 {
		return 0, 0, fmt.Errorf("xkb: server does not support requested version %d.%d", wantMajor, wantMinor)
	}

	d := NewDecoder(c.byteOrder, reply[8:12])
	serverMajor, _ := d.Uint16()
	serverMinor, _ := d.Uint16()

	return serverMajor, serverMinor, nil
}

// xkbGetState sends an XkbGetState request (minor opcode 4) and returns the
// current keyboard group index (0-3).
func (c *Connection) xkbGetState(majorOpcode uint8) (int, error) {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(majorOpcode)
	e.PutUint8(XkbMinorOpcodeGetState)
	e.PutUint16(2) // request length: 8 bytes / 4 = 2 units
	e.PutUint16(XkbUseCoreKbd)
	e.PutUint16(0) // pad

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return 0, err
	}

	if len(reply) < 15 {
		return 0, fmt.Errorf("xkb: GetState reply too short (%d bytes)", len(reply))
	}

	// Reply layout (32 bytes):
	//   byte 1: device ID
	//   bytes 2-3: sequence
	//   bytes 4-7: length
	//   byte 8: mods
	//   byte 9: base mods
	//   byte 10: latched mods
	//   byte 11: locked mods
	//   byte 12: group
	//   byte 13: locked group
	//   byte 14: base group (uint16, but only need low byte)
	group := int(reply[12])

	return group, nil
}

// xkbSelectEvents sends an XkbSelectEvents request (minor opcode 1) to subscribe
// to XkbStateNotify events with group change details.
func (c *Connection) xkbSelectEvents(majorOpcode uint8) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(majorOpcode)
	e.PutUint8(XkbMinorOpcodeSelectEvents)
	e.PutUint16(5)                  // request length: 20 bytes / 4 = 5 units
	e.PutUint16(XkbUseCoreKbd)      // device spec
	e.PutUint16(XkbStateNotifyMask) // affectWhich: subscribe to state change events
	e.PutUint16(0)                  // clear: don't clear any event types
	e.PutUint16(0)                  // selectAll: 0 — use per-event details below (not auto-select all)
	e.PutUint16(0)                  // affectMap
	e.PutUint16(0)                  // map
	// Per-event details for StateNotify (included because StateNotify is in
	// affectWhich but NOT in selectAll — XKB wire protocol requires detail pair):
	e.PutUint16(XkbGroupStateMask) // affectState: we want group changes
	e.PutUint16(XkbGroupStateMask) // stateDetails: group changes

	_, err := c.sendRequest(e.Bytes())
	return err
}
