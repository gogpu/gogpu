//go:build linux

package x11

import (
	"fmt"
)

// XInput2 protocol constants.
const (
	// XIQueryVersion minor opcode.
	XIMinorOpcodeQueryVersion = 47
	// XISelectEvents minor opcode.
	XIMinorOpcodeSelectEvents = 46

	// XI2 event types (evtype field in GenericEvent).
	XITouchBegin  = 18
	XITouchUpdate = 19
	XITouchEnd    = 20

	// Device ID constants.
	XIAllDevices       = 0
	XIAllMasterDevices = 1

	// Touch event flags.
	XITouchEmulatingPointer = 1 << 17

	// Extension name for QueryExtension.
	XIExtensionName = "XInputExtension"
)

// XIExtension holds XInput2 extension state after initialization.
type XIExtension struct {
	MajorOpcode uint8  // Assigned opcode from QueryExtension
	EventBase   uint8  // Base event code
	ErrorBase   uint8  // Base error code
	MajorVer    uint16 // Negotiated major version
	MinorVer    uint16 // Negotiated minor version
}

// XIDeviceEvent represents an XI2 device event (touch, button, motion).
// Used for XI_TouchBegin, XI_TouchUpdate, XI_TouchEnd.
type XIDeviceEvent struct {
	EventType   uint16     // XI event type (XITouchBegin, etc.)
	Sequence    uint16     // Sequence number
	DeviceID    uint16     // Master device ID
	Time        Timestamp  // Server timestamp
	Detail      uint32     // Touch ID for touch events
	Root        ResourceID // Root window
	Event       ResourceID // Event window
	Child       ResourceID // Child window (or 0)
	RootX       float64    // Root-relative X (converted from FP1616)
	RootY       float64    // Root-relative Y
	EventX      float64    // Event-window-relative X
	EventY      float64    // Event-window-relative Y
	SourceID    uint16     // Physical device that generated this event
	Flags       uint32     // Event flags
	Mods        uint32     // Effective modifier mask
	ButtonsMask []byte     // Button mask
	AxisValues  []float64  // Valuator axis values (FP3232 converted)
}

func (*XIDeviceEvent) eventMarker() {}

// IsTouchEmulating returns true if this touch event is emulating pointer events.
func (e *XIDeviceEvent) IsTouchEmulating() bool {
	return e.Flags&XITouchEmulatingPointer != 0
}

// InitXInput2 queries and initializes the XInput2 extension.
// Returns nil if the extension is not available or version < 2.2.
func (c *Connection) InitXInput2() (*XIExtension, error) {
	// Step 1: QueryExtension for "XInputExtension"
	ext, err := c.QueryExtension(XIExtensionName)
	if err != nil {
		return nil, fmt.Errorf("xinput2: QueryExtension failed: %w", err)
	}
	if !ext.Present {
		return nil, fmt.Errorf("xinput2: extension not available")
	}

	xi := &XIExtension{
		MajorOpcode: ext.MajorOpcode,
		EventBase:   ext.FirstEvent,
		ErrorBase:   ext.FirstError,
	}

	// Step 2: XIQueryVersion requesting 2.2
	major, minor, err := c.xiQueryVersion(xi.MajorOpcode, 2, 2)
	if err != nil {
		return nil, fmt.Errorf("xinput2: XIQueryVersion failed: %w", err)
	}

	xi.MajorVer = major
	xi.MinorVer = minor

	if major < 2 || (major == 2 && minor < 2) {
		return nil, fmt.Errorf("xinput2: server version %d.%d < 2.2, touch not supported", major, minor)
	}

	return xi, nil
}

// xiQueryVersion sends an XIQueryVersion request and returns the server version.
func (c *Connection) xiQueryVersion(majorOpcode uint8, clientMajor, clientMinor uint16) (uint16, uint16, error) {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(majorOpcode)
	e.PutUint8(XIMinorOpcodeQueryVersion)
	e.PutUint16(2) // request length: 8 bytes / 4 = 2 units
	e.PutUint16(clientMajor)
	e.PutUint16(clientMinor)

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return 0, 0, err
	}

	if len(reply) < 12 {
		return 0, 0, fmt.Errorf("xinput2: XIQueryVersion reply too short")
	}

	d := NewDecoder(c.byteOrder, reply[8:12])
	serverMajor, _ := d.Uint16()
	serverMinor, _ := d.Uint16()

	return serverMajor, serverMinor, nil
}

// XISelectTouchEvents enables touch event delivery on the given window.
func (c *Connection) XISelectTouchEvents(xi *XIExtension, window ResourceID) error {
	// Build event mask for XI_TouchBegin(18), XI_TouchUpdate(19), XI_TouchEnd(20)
	mask := [4]byte{}
	xiSetMask(mask[:], XITouchBegin)
	xiSetMask(mask[:], XITouchUpdate)
	xiSetMask(mask[:], XITouchEnd)

	e := NewEncoder(c.byteOrder)
	e.PutUint8(xi.MajorOpcode)
	e.PutUint8(XIMinorOpcodeSelectEvents)
	e.PutUint16(5) // request length: (12 + 4 + 4) / 4 = 5 units
	e.PutUint32(uint32(window))
	e.PutUint16(1)                  // num_masks
	e.PutUint16(0)                  // padding
	e.PutUint16(XIAllMasterDevices) // deviceid
	e.PutUint16(1)                  // mask_len (1 x 4-byte unit)
	e.PutBytes(mask[:])

	_, err := c.sendRequest(e.Bytes())
	return err
}

// ParseXIDeviceEvent parses an XI2 device event from a GenericEvent.
// The data must contain the full event (32-byte header + payload).
func (c *Connection) ParseXIDeviceEvent(ge *GenericEvent) (*XIDeviceEvent, error) {
	buf := ge.Data
	if len(buf) < 96 { // 32 header + 64 fixed payload minimum
		return nil, fmt.Errorf("xinput2: device event too short (%d bytes)", len(buf))
	}

	d := NewDecoder(c.byteOrder, buf)

	// GenericEvent header (32 bytes)
	_, _ = d.Uint8()          // type (35)
	_, _ = d.Uint8()          // extension
	seq, _ := d.Uint16()      // sequence number
	_, _ = d.Uint32()         // length
	evtype, _ := d.Uint16()   // XI event type
	deviceID, _ := d.Uint16() // device ID
	time, _ := d.Uint32()     // timestamp

	// Remaining 16 bytes of GenericEvent header (padding/reserved)
	_ = d.Skip(16)

	// Fixed payload (64 bytes at offset 32)
	detail, _ := d.Uint32()     // touch ID
	root, _ := d.Uint32()       // root window
	event, _ := d.Uint32()      // event window
	child, _ := d.Uint32()      // child window
	rootXRaw, _ := d.Int32()    // FP1616 root_x
	rootYRaw, _ := d.Int32()    // FP1616 root_y
	eventXRaw, _ := d.Int32()   // FP1616 event_x
	eventYRaw, _ := d.Int32()   // FP1616 event_y
	buttonsLen, _ := d.Uint16() // button mask length (4-byte units)
	valuatorsLen, _ := d.Uint16()
	sourceID, _ := d.Uint16()
	_, _ = d.Uint16() // pad0
	flags, _ := d.Uint32()

	// Modifier info (16 bytes)
	_, _ = d.Uint32() // base_mods
	_, _ = d.Uint32() // latched_mods
	_, _ = d.Uint32() // locked_mods
	effectiveMods, _ := d.Uint32()

	// Group info (4 bytes)
	_ = d.Skip(4)

	// Variable: buttons mask (buttonsLen * 4 bytes)
	var buttonsMask []byte
	if buttonsLen > 0 {
		buttonsMask, _ = d.Bytes(int(buttonsLen) * 4)
	}

	// Variable: valuators mask (valuatorsLen * 4 bytes)
	var valuatorsMask []byte
	if valuatorsLen > 0 {
		valuatorsMask, _ = d.Bytes(int(valuatorsLen) * 4)
	}

	// Variable: axis values (FP3232, 8 bytes each, one per set bit in valuators)
	numValuators := popcount(valuatorsMask)
	var axisValues []float64
	if numValuators > 0 {
		axisValues = make([]float64, numValuators)
		for i := 0; i < numValuators; i++ {
			integral, _ := d.Int32()
			frac, _ := d.Uint32()
			axisValues[i] = fp3232ToFloat64(integral, frac)
		}
	}

	return &XIDeviceEvent{
		EventType:   evtype,
		Sequence:    seq,
		DeviceID:    deviceID,
		Time:        Timestamp(time),
		Detail:      detail,
		Root:        ResourceID(root),
		Event:       ResourceID(event),
		Child:       ResourceID(child),
		RootX:       fp1616ToFloat64(rootXRaw),
		RootY:       fp1616ToFloat64(rootYRaw),
		EventX:      fp1616ToFloat64(eventXRaw),
		EventY:      fp1616ToFloat64(eventYRaw),
		SourceID:    sourceID,
		Flags:       flags,
		Mods:        effectiveMods,
		ButtonsMask: buttonsMask,
		AxisValues:  axisValues,
	}, nil
}

// xiSetMask sets the bit for event type in the mask byte array.
func xiSetMask(mask []byte, event int) {
	mask[event>>3] |= 1 << (event & 7)
}

// fp1616ToFloat64 converts a 16.16 fixed-point value to float64.
func fp1616ToFloat64(v int32) float64 {
	return float64(v) / 65536.0
}

// fp3232ToFloat64 converts a 32.32 fixed-point value to float64.
func fp3232ToFloat64(integral int32, frac uint32) float64 {
	return float64(integral) + float64(frac)/4294967296.0
}

// popcount returns the number of set bits in a byte slice.
func popcount(mask []byte) int {
	count := 0
	for _, b := range mask {
		for b != 0 {
			count += int(b & 1)
			b >>= 1
		}
	}
	return count
}
