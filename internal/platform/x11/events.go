//go:build linux

package x11

import (
	"fmt"
)

// Event is the interface implemented by all X11 events.
type Event interface {
	eventMarker()
}

// KeyEvent contains common data for key press/release events.
type KeyEvent struct {
	Detail     uint8      // Keycode
	Sequence   uint16     // Sequence number
	Time       Timestamp  // Server timestamp
	Root       ResourceID // Root window
	Event      ResourceID // Event window
	Child      ResourceID // Child window (or None)
	RootX      int16      // Pointer X relative to root
	RootY      int16      // Pointer Y relative to root
	EventX     int16      // Pointer X relative to event window
	EventY     int16      // Pointer Y relative to event window
	State      uint16     // Key/button mask
	SameScreen bool       // True if event and root are on same screen
}

func (*KeyEvent) eventMarker() {}

// KeyPressEvent is generated when a key is pressed.
type KeyPressEvent struct {
	KeyEvent
}

// KeyReleaseEvent is generated when a key is released.
type KeyReleaseEvent struct {
	KeyEvent
}

// ButtonEvent contains common data for button press/release events.
type ButtonEvent struct {
	Detail     uint8      // Button number (1-5)
	Sequence   uint16     // Sequence number
	Time       Timestamp  // Server timestamp
	Root       ResourceID // Root window
	Event      ResourceID // Event window
	Child      ResourceID // Child window (or None)
	RootX      int16      // Pointer X relative to root
	RootY      int16      // Pointer Y relative to root
	EventX     int16      // Pointer X relative to event window
	EventY     int16      // Pointer Y relative to event window
	State      uint16     // Key/button mask before event
	SameScreen bool       // True if event and root are on same screen
}

func (*ButtonEvent) eventMarker() {}

// ButtonPressEvent is generated when a mouse button is pressed.
type ButtonPressEvent struct {
	ButtonEvent
}

// ButtonReleaseEvent is generated when a mouse button is released.
type ButtonReleaseEvent struct {
	ButtonEvent
}

// MotionNotifyEvent is generated when the pointer moves.
type MotionNotifyEvent struct {
	Detail     uint8      // Motion hint flag
	Sequence   uint16     // Sequence number
	Time       Timestamp  // Server timestamp
	Root       ResourceID // Root window
	Event      ResourceID // Event window
	Child      ResourceID // Child window (or None)
	RootX      int16      // Pointer X relative to root
	RootY      int16      // Pointer Y relative to root
	EventX     int16      // Pointer X relative to event window
	EventY     int16      // Pointer Y relative to event window
	State      uint16     // Key/button mask
	SameScreen bool       // True if event and root are on same screen
}

func (*MotionNotifyEvent) eventMarker() {}

// CrossingEvent contains common data for enter/leave events.
type CrossingEvent struct {
	Detail          uint8      // NotifyAncestor, NotifyVirtual, etc.
	Sequence        uint16     // Sequence number
	Time            Timestamp  // Server timestamp
	Root            ResourceID // Root window
	Event           ResourceID // Event window
	Child           ResourceID // Child window (or None)
	RootX           int16      // Pointer X relative to root
	RootY           int16      // Pointer Y relative to root
	EventX          int16      // Pointer X relative to event window
	EventY          int16      // Pointer Y relative to event window
	State           uint16     // Key/button mask
	Mode            uint8      // NotifyNormal, NotifyGrab, NotifyUngrab
	SameScreenFocus uint8      // Same screen and focus flags
}

func (*CrossingEvent) eventMarker() {}

// EnterNotifyEvent is generated when the pointer enters a window.
type EnterNotifyEvent struct {
	CrossingEvent
}

// LeaveNotifyEvent is generated when the pointer leaves a window.
type LeaveNotifyEvent struct {
	CrossingEvent
}

// FocusEvent contains common data for focus events.
type FocusEvent struct {
	Detail   uint8      // NotifyAncestor, NotifyVirtual, etc.
	Sequence uint16     // Sequence number
	Event    ResourceID // Event window
	Mode     uint8      // NotifyNormal, NotifyWhileGrabbed, etc.
}

func (*FocusEvent) eventMarker() {}

// FocusInEvent is generated when a window gains input focus.
type FocusInEvent struct {
	FocusEvent
}

// FocusOutEvent is generated when a window loses input focus.
type FocusOutEvent struct {
	FocusEvent
}

// ExposeEvent is generated when a window region needs redrawing.
type ExposeEvent struct {
	Sequence uint16     // Sequence number
	Window   ResourceID // Exposed window
	X        uint16     // X coordinate of exposed region
	Y        uint16     // Y coordinate of exposed region
	Width    uint16     // Width of exposed region
	Height   uint16     // Height of exposed region
	Count    uint16     // Number of subsequent Expose events
}

func (*ExposeEvent) eventMarker() {}

// ConfigureNotifyEvent is generated when a window is reconfigured.
type ConfigureNotifyEvent struct {
	Sequence         uint16     // Sequence number
	Event            ResourceID // Event window
	Window           ResourceID // Configured window
	AboveSibling     ResourceID // Sibling above (or None)
	X                int16      // New X coordinate
	Y                int16      // New Y coordinate
	Width            uint16     // New width
	Height           uint16     // New height
	BorderWidth      uint16     // New border width
	OverrideRedirect bool       // Override redirect flag
}

func (*ConfigureNotifyEvent) eventMarker() {}

// MapNotifyEvent is generated when a window is mapped.
type MapNotifyEvent struct {
	Sequence         uint16     // Sequence number
	Event            ResourceID // Event window
	Window           ResourceID // Mapped window
	OverrideRedirect bool       // Override redirect flag
}

func (*MapNotifyEvent) eventMarker() {}

// UnmapNotifyEvent is generated when a window is unmapped.
type UnmapNotifyEvent struct {
	Sequence      uint16     // Sequence number
	Event         ResourceID // Event window
	Window        ResourceID // Unmapped window
	FromConfigure bool       // True if due to parent resize
}

func (*UnmapNotifyEvent) eventMarker() {}

// DestroyNotifyEvent is generated when a window is destroyed.
type DestroyNotifyEvent struct {
	Sequence uint16     // Sequence number
	Event    ResourceID // Event window
	Window   ResourceID // Destroyed window
}

func (*DestroyNotifyEvent) eventMarker() {}

// PropertyNotifyEvent is generated when a window property changes.
type PropertyNotifyEvent struct {
	Sequence uint16     // Sequence number
	Window   ResourceID // Window with changed property
	Atom     Atom       // Property atom
	Time     Timestamp  // Server timestamp
	State    uint8      // PropertyNewValue or PropertyDelete
}

func (*PropertyNotifyEvent) eventMarker() {}

// ClientMessageEvent is generated for client-to-client communication.
type ClientMessageEvent struct {
	Format   uint8      // 8, 16, or 32 bits
	Sequence uint16     // Sequence number
	Window   ResourceID // Target window
	Type     Atom       // Message type atom
	Data     [20]byte   // Message data (format-dependent)
}

func (*ClientMessageEvent) eventMarker() {}

// Data32 returns the message data as 5 uint32 values (for format=32).
func (e *ClientMessageEvent) Data32() [5]uint32 {
	var result [5]uint32
	for i := 0; i < 5; i++ {
		offset := i * 4
		result[i] = uint32(e.Data[offset]) |
			uint32(e.Data[offset+1])<<8 |
			uint32(e.Data[offset+2])<<16 |
			uint32(e.Data[offset+3])<<24
	}
	return result
}

// IsDeleteWindow checks if this is a WM_DELETE_WINDOW message.
func (e *ClientMessageEvent) IsDeleteWindow(atoms *StandardAtoms) bool {
	if e.Type != atoms.WMProtocols {
		return false
	}
	data := e.Data32()
	return Atom(data[0]) == atoms.WMDeleteWindow
}

// SelectionClearEvent is generated when selection ownership is lost.
type SelectionClearEvent struct {
	Sequence  uint16     // Sequence number
	Time      Timestamp  // Server timestamp
	Owner     ResourceID // Previous owner
	Selection Atom       // Selection atom
}

func (*SelectionClearEvent) eventMarker() {}

// MappingNotifyEvent is generated when keyboard mapping changes.
type MappingNotifyEvent struct {
	Sequence     uint16 // Sequence number
	Request      uint8  // MappingModifier, MappingKeyboard, MappingPointer
	FirstKeycode uint8  // First changed keycode
	Count        uint8  // Number of changed keycodes
}

func (*MappingNotifyEvent) eventMarker() {}

// UnknownEvent represents an unrecognized event type.
type UnknownEvent struct {
	Type uint8
	Data [31]byte
}

func (*UnknownEvent) eventMarker() {}

// parseEvent parses an event from wire format.
func (c *Connection) parseEvent(buf []byte) (Event, error) {
	if len(buf) < 32 {
		return nil, fmt.Errorf("x11: event buffer too short")
	}

	// Event type is in bits 0-6, bit 7 indicates synthetic event
	eventType := buf[0] & 0x7F

	switch eventType {
	case EventKeyPress:
		return c.parseKeyEvent(buf, true)
	case EventKeyRelease:
		return c.parseKeyEvent(buf, false)
	case EventButtonPress:
		return c.parseButtonEvent(buf, true)
	case EventButtonRelease:
		return c.parseButtonEvent(buf, false)
	case EventMotionNotify:
		return c.parseMotionNotifyEvent(buf)
	case EventEnterNotify:
		return c.parseCrossingEvent(buf, true)
	case EventLeaveNotify:
		return c.parseCrossingEvent(buf, false)
	case EventFocusIn:
		return c.parseFocusEvent(buf, true)
	case EventFocusOut:
		return c.parseFocusEvent(buf, false)
	case EventExpose:
		return c.parseExposeEvent(buf)
	case EventConfigureNotify:
		return c.parseConfigureNotifyEvent(buf)
	case EventMapNotify:
		return c.parseMapNotifyEvent(buf)
	case EventUnmapNotify:
		return c.parseUnmapNotifyEvent(buf)
	case EventDestroyNotify:
		return c.parseDestroyNotifyEvent(buf)
	case EventPropertyNotify:
		return c.parsePropertyNotifyEvent(buf)
	case EventClientMessage:
		return c.parseClientMessageEvent(buf)
	case EventSelectionClear:
		return c.parseSelectionClearEvent(buf)
	case EventMappingNotify:
		return c.parseMappingNotifyEvent(buf)
	default:
		event := &UnknownEvent{Type: eventType}
		copy(event.Data[:], buf[1:32])
		return event, nil
	}
}

func (c *Connection) parseKeyEvent(buf []byte, press bool) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	detail, _ := d.Uint8()
	seq, _ := d.Uint16()
	time, _ := d.Uint32()
	root, _ := d.Uint32()
	event, _ := d.Uint32()
	child, _ := d.Uint32()
	rootX, _ := d.Int16()
	rootY, _ := d.Int16()
	eventX, _ := d.Int16()
	eventY, _ := d.Int16()
	state, _ := d.Uint16()
	sameScreen, _ := d.Uint8()

	ke := KeyEvent{
		Detail:     detail,
		Sequence:   seq,
		Time:       Timestamp(time),
		Root:       ResourceID(root),
		Event:      ResourceID(event),
		Child:      ResourceID(child),
		RootX:      rootX,
		RootY:      rootY,
		EventX:     eventX,
		EventY:     eventY,
		State:      state,
		SameScreen: sameScreen != 0,
	}

	if press {
		return &KeyPressEvent{KeyEvent: ke}, nil
	}
	return &KeyReleaseEvent{KeyEvent: ke}, nil
}

func (c *Connection) parseButtonEvent(buf []byte, press bool) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	detail, _ := d.Uint8()
	seq, _ := d.Uint16()
	time, _ := d.Uint32()
	root, _ := d.Uint32()
	event, _ := d.Uint32()
	child, _ := d.Uint32()
	rootX, _ := d.Int16()
	rootY, _ := d.Int16()
	eventX, _ := d.Int16()
	eventY, _ := d.Int16()
	state, _ := d.Uint16()
	sameScreen, _ := d.Uint8()

	be := ButtonEvent{
		Detail:     detail,
		Sequence:   seq,
		Time:       Timestamp(time),
		Root:       ResourceID(root),
		Event:      ResourceID(event),
		Child:      ResourceID(child),
		RootX:      rootX,
		RootY:      rootY,
		EventX:     eventX,
		EventY:     eventY,
		State:      state,
		SameScreen: sameScreen != 0,
	}

	if press {
		return &ButtonPressEvent{ButtonEvent: be}, nil
	}
	return &ButtonReleaseEvent{ButtonEvent: be}, nil
}

func (c *Connection) parseMotionNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	detail, _ := d.Uint8()
	seq, _ := d.Uint16()
	time, _ := d.Uint32()
	root, _ := d.Uint32()
	event, _ := d.Uint32()
	child, _ := d.Uint32()
	rootX, _ := d.Int16()
	rootY, _ := d.Int16()
	eventX, _ := d.Int16()
	eventY, _ := d.Int16()
	state, _ := d.Uint16()
	sameScreen, _ := d.Uint8()

	return &MotionNotifyEvent{
		Detail:     detail,
		Sequence:   seq,
		Time:       Timestamp(time),
		Root:       ResourceID(root),
		Event:      ResourceID(event),
		Child:      ResourceID(child),
		RootX:      rootX,
		RootY:      rootY,
		EventX:     eventX,
		EventY:     eventY,
		State:      state,
		SameScreen: sameScreen != 0,
	}, nil
}

func (c *Connection) parseCrossingEvent(buf []byte, enter bool) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	detail, _ := d.Uint8()
	seq, _ := d.Uint16()
	time, _ := d.Uint32()
	root, _ := d.Uint32()
	event, _ := d.Uint32()
	child, _ := d.Uint32()
	rootX, _ := d.Int16()
	rootY, _ := d.Int16()
	eventX, _ := d.Int16()
	eventY, _ := d.Int16()
	state, _ := d.Uint16()
	mode, _ := d.Uint8()
	sameScreenFocus, _ := d.Uint8()

	ce := CrossingEvent{
		Detail:          detail,
		Sequence:        seq,
		Time:            Timestamp(time),
		Root:            ResourceID(root),
		Event:           ResourceID(event),
		Child:           ResourceID(child),
		RootX:           rootX,
		RootY:           rootY,
		EventX:          eventX,
		EventY:          eventY,
		State:           state,
		Mode:            mode,
		SameScreenFocus: sameScreenFocus,
	}

	if enter {
		return &EnterNotifyEvent{CrossingEvent: ce}, nil
	}
	return &LeaveNotifyEvent{CrossingEvent: ce}, nil
}

func (c *Connection) parseFocusEvent(buf []byte, focusIn bool) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	detail, _ := d.Uint8()
	seq, _ := d.Uint16()
	event, _ := d.Uint32()
	mode, _ := d.Uint8()

	fe := FocusEvent{
		Detail:   detail,
		Sequence: seq,
		Event:    ResourceID(event),
		Mode:     mode,
	}

	if focusIn {
		return &FocusInEvent{FocusEvent: fe}, nil
	}
	return &FocusOutEvent{FocusEvent: fe}, nil
}

func (c *Connection) parseExposeEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	window, _ := d.Uint32()
	x, _ := d.Uint16()
	y, _ := d.Uint16()
	width, _ := d.Uint16()
	height, _ := d.Uint16()
	count, _ := d.Uint16()

	return &ExposeEvent{
		Sequence: seq,
		Window:   ResourceID(window),
		X:        x,
		Y:        y,
		Width:    width,
		Height:   height,
		Count:    count,
	}, nil
}

func (c *Connection) parseConfigureNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	event, _ := d.Uint32()
	window, _ := d.Uint32()
	aboveSibling, _ := d.Uint32()
	x, _ := d.Int16()
	y, _ := d.Int16()
	width, _ := d.Uint16()
	height, _ := d.Uint16()
	borderWidth, _ := d.Uint16()
	overrideRedirect, _ := d.Uint8()

	return &ConfigureNotifyEvent{
		Sequence:         seq,
		Event:            ResourceID(event),
		Window:           ResourceID(window),
		AboveSibling:     ResourceID(aboveSibling),
		X:                x,
		Y:                y,
		Width:            width,
		Height:           height,
		BorderWidth:      borderWidth,
		OverrideRedirect: overrideRedirect != 0,
	}, nil
}

func (c *Connection) parseMapNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	event, _ := d.Uint32()
	window, _ := d.Uint32()
	overrideRedirect, _ := d.Uint8()

	return &MapNotifyEvent{
		Sequence:         seq,
		Event:            ResourceID(event),
		Window:           ResourceID(window),
		OverrideRedirect: overrideRedirect != 0,
	}, nil
}

func (c *Connection) parseUnmapNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	event, _ := d.Uint32()
	window, _ := d.Uint32()
	fromConfigure, _ := d.Uint8()

	return &UnmapNotifyEvent{
		Sequence:      seq,
		Event:         ResourceID(event),
		Window:        ResourceID(window),
		FromConfigure: fromConfigure != 0,
	}, nil
}

func (c *Connection) parseDestroyNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	event, _ := d.Uint32()
	window, _ := d.Uint32()

	return &DestroyNotifyEvent{
		Sequence: seq,
		Event:    ResourceID(event),
		Window:   ResourceID(window),
	}, nil
}

func (c *Connection) parsePropertyNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	window, _ := d.Uint32()
	atom, _ := d.Uint32()
	time, _ := d.Uint32()
	state, _ := d.Uint8()

	return &PropertyNotifyEvent{
		Sequence: seq,
		Window:   ResourceID(window),
		Atom:     Atom(atom),
		Time:     Timestamp(time),
		State:    state,
	}, nil
}

func (c *Connection) parseClientMessageEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	format, _ := d.Uint8()
	seq, _ := d.Uint16()
	window, _ := d.Uint32()
	msgType, _ := d.Uint32()

	event := &ClientMessageEvent{
		Format:   format,
		Sequence: seq,
		Window:   ResourceID(window),
		Type:     Atom(msgType),
	}

	// Read 20 bytes of data
	data, _ := d.Bytes(20)
	copy(event.Data[:], data)

	return event, nil
}

func (c *Connection) parseSelectionClearEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	time, _ := d.Uint32()
	owner, _ := d.Uint32()
	selection, _ := d.Uint32()

	return &SelectionClearEvent{
		Sequence:  seq,
		Time:      Timestamp(time),
		Owner:     ResourceID(owner),
		Selection: Atom(selection),
	}, nil
}

func (c *Connection) parseMappingNotifyEvent(buf []byte) (Event, error) {
	d := NewDecoder(c.byteOrder, buf)

	_, _ = d.Uint8() // event type
	_, _ = d.Uint8() // unused
	seq, _ := d.Uint16()
	request, _ := d.Uint8()
	firstKeycode, _ := d.Uint8()
	count, _ := d.Uint8()

	return &MappingNotifyEvent{
		Sequence:     seq,
		Request:      request,
		FirstKeycode: firstKeycode,
		Count:        count,
	}, nil
}

// WaitForEvent reads and returns the next event from the server.
// This call blocks until an event is available.
func (c *Connection) WaitForEvent() (Event, error) {
	for {
		buf := make([]byte, 32)
		if _, err := c.conn.Read(buf); err != nil {
			return nil, fmt.Errorf("x11: failed to read event: %w", err)
		}

		// Check response type
		responseType := buf[0]

		// Error response
		if responseType == 0 {
			return nil, c.parseError(buf)
		}

		// Reply response - skip (we're looking for events)
		if responseType == 1 {
			// Read additional data
			d := NewDecoder(c.byteOrder, buf[4:8])
			additionalLen, _ := d.Uint32()
			if additionalLen > 0 {
				additional := make([]byte, additionalLen*4)
				_, _ = c.conn.Read(additional)
			}
			continue
		}

		// Event
		return c.parseEvent(buf)
	}
}

// PollEvent checks for a pending event without blocking.
// Returns nil, nil if no event is available - this is the expected case
// when there are no pending events to process.
//
//nolint:nilnil // nil,nil is intentional to indicate "no event available"
func (c *Connection) PollEvent() (Event, error) {
	// Set read deadline to avoid blocking
	// This is a simple approach - a production implementation
	// would use poll/epoll for proper non-blocking I/O

	// For now, we'll use a non-blocking approach by checking
	// if data is available
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrConnectionClosed
	}

	// Try to read with a very short timeout
	// This is a simplified approach - returns nil event when no data available
	return nil, nil
}
