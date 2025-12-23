//go:build linux

package x11

import (
	"fmt"
	"os"
)

// WindowConfig holds configuration for creating a window.
type WindowConfig struct {
	Title      string
	Width      uint16
	Height     uint16
	X          int16
	Y          int16
	Resizable  bool
	Fullscreen bool
}

// CreateWindow creates a new X11 window.
func (c *Connection) CreateWindow(config WindowConfig) (ResourceID, error) {
	screen := c.DefaultScreen()
	if screen == nil {
		return 0, fmt.Errorf("x11: no default screen")
	}

	// Generate window ID
	windowID := c.GenerateID()

	// Set up window attributes
	valueMask := uint32(CWBackPixel | CWEventMask)

	// Event mask - listen for common events
	eventMask := uint32(
		EventMaskKeyPress |
			EventMaskKeyRelease |
			EventMaskButtonPress |
			EventMaskButtonRelease |
			EventMaskPointerMotion |
			EventMaskExposure |
			EventMaskStructureNotify |
			EventMaskFocusChange |
			EventMaskEnterWindow |
			EventMaskLeaveWindow |
			EventMaskPropertyChange)

	// Value list (order matters - must match bit order in valueMask)
	valueList := []uint32{
		screen.BlackPixel, // CWBackPixel
		eventMask,         // CWEventMask
	}

	// Build request
	// Request length = 8 + len(valueList) in 4-byte units
	reqLen := uint16(8 + len(valueList))

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeCreateWindow)
	e.PutUint8(screen.RootDepth) // depth
	e.PutUint16(reqLen)
	e.PutUint32(uint32(windowID))
	e.PutUint32(uint32(screen.Root))
	e.PutInt16(config.X)
	e.PutInt16(config.Y)
	e.PutUint16(config.Width)
	e.PutUint16(config.Height)
	e.PutUint16(0) // border width
	e.PutUint16(WindowClassInputOutput)
	e.PutUint32(screen.RootVisual)
	e.PutUint32(valueMask)
	for _, v := range valueList {
		e.PutUint32(v)
	}

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return 0, fmt.Errorf("x11: CreateWindow failed: %w", err)
	}

	return windowID, nil
}

// MapWindow makes a window visible.
func (c *Connection) MapWindow(window ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeMapWindow)
	e.PutUint8(0) // unused
	e.PutUint16(2) // length
	e.PutUint32(uint32(window))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: MapWindow failed: %w", err)
	}
	return nil
}

// UnmapWindow hides a window.
func (c *Connection) UnmapWindow(window ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeUnmapWindow)
	e.PutUint8(0) // unused
	e.PutUint16(2) // length
	e.PutUint32(uint32(window))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: UnmapWindow failed: %w", err)
	}
	return nil
}

// DestroyWindow destroys a window.
func (c *Connection) DestroyWindow(window ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeDestroyWindow)
	e.PutUint8(0) // unused
	e.PutUint16(2) // length
	e.PutUint32(uint32(window))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: DestroyWindow failed: %w", err)
	}
	return nil
}

// ChangeProperty changes a window property.
func (c *Connection) ChangeProperty(window ResourceID, property, propType Atom, format uint8, mode uint8, data []byte) error {
	dataLen := len(data)
	// Number of data elements (based on format)
	var numElements uint32
	switch format {
	case 8:
		numElements = uint32(dataLen)
	case 16:
		numElements = uint32(dataLen / 2)
	case 32:
		numElements = uint32(dataLen / 4)
	default:
		return fmt.Errorf("x11: invalid property format %d", format)
	}

	// Request length in 4-byte units
	reqLen := uint16(6 + (dataLen+3)/4)

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeChangeProperty)
	e.PutUint8(mode)
	e.PutUint16(reqLen)
	e.PutUint32(uint32(window))
	e.PutUint32(uint32(property))
	e.PutUint32(uint32(propType))
	e.PutUint8(format)
	e.PutPadN(3) // unused
	e.PutUint32(numElements)
	e.PutBytes(data)
	e.PutPad()

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: ChangeProperty failed: %w", err)
	}
	return nil
}

// SetWindowTitle sets the window title using both WM_NAME and _NET_WM_NAME.
func (c *Connection) SetWindowTitle(window ResourceID, title string, atoms *StandardAtoms) error {
	titleBytes := []byte(title)

	// Set WM_NAME (legacy, STRING type)
	if err := c.ChangeProperty(window, AtomWMName, AtomString, 8, PropModeReplace, titleBytes); err != nil {
		return err
	}

	// Set _NET_WM_NAME (modern, UTF8_STRING type)
	if atoms.NetWMName != AtomNone && atoms.UTF8String != AtomNone {
		if err := c.ChangeProperty(window, atoms.NetWMName, atoms.UTF8String, 8, PropModeReplace, titleBytes); err != nil {
			return err
		}
	}

	return nil
}

// SetWMProtocols sets the WM_PROTOCOLS property to receive WM_DELETE_WINDOW events.
func (c *Connection) SetWMProtocols(window ResourceID, atoms *StandardAtoms) error {
	// Build array of atoms we want to receive
	var protocols []byte

	// Add WM_DELETE_WINDOW
	protocols = append(protocols, byte(atoms.WMDeleteWindow), byte(atoms.WMDeleteWindow>>8),
		byte(atoms.WMDeleteWindow>>16), byte(atoms.WMDeleteWindow>>24))

	return c.ChangeProperty(window, atoms.WMProtocols, AtomAtom, 32, PropModeReplace, protocols)
}

// SetWMClass sets the WM_CLASS property (instance name and class name).
func (c *Connection) SetWMClass(window ResourceID, instanceName, className string) error {
	// WM_CLASS is two null-terminated strings concatenated
	data := []byte(instanceName)
	data = append(data, 0)
	data = append(data, []byte(className)...)
	data = append(data, 0)

	return c.ChangeProperty(window, AtomWMClass, AtomString, 8, PropModeReplace, data)
}

// SetWMPID sets the _NET_WM_PID property.
func (c *Connection) SetWMPID(window ResourceID, atoms *StandardAtoms) error {
	if atoms.NetWMPID == AtomNone {
		return nil
	}

	pid := uint32(os.Getpid())
	data := []byte{
		byte(pid),
		byte(pid >> 8),
		byte(pid >> 16),
		byte(pid >> 24),
	}

	return c.ChangeProperty(window, atoms.NetWMPID, AtomCardinal, 32, PropModeReplace, data)
}

// SetNetWMWindowType sets the _NET_WM_WINDOW_TYPE property.
func (c *Connection) SetNetWMWindowType(window ResourceID, windowType Atom, atoms *StandardAtoms) error {
	if atoms.NetWMWindowType == AtomNone {
		return nil
	}

	data := []byte{
		byte(windowType),
		byte(windowType >> 8),
		byte(windowType >> 16),
		byte(windowType >> 24),
	}

	return c.ChangeProperty(window, atoms.NetWMWindowType, AtomAtom, 32, PropModeReplace, data)
}

// ConfigureWindow configures window position and size.
func (c *Connection) ConfigureWindow(window ResourceID, x, y int16, width, height uint16) error {
	// Value mask bits
	const (
		ConfigX      = 1 << 0
		ConfigY      = 1 << 1
		ConfigWidth  = 1 << 2
		ConfigHeight = 1 << 3
	)

	valueMask := uint16(ConfigX | ConfigY | ConfigWidth | ConfigHeight)

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeConfigureWindow)
	e.PutUint8(0) // unused
	e.PutUint16(8) // length: 3 + 1 + 4 values = 8 4-byte units
	e.PutUint32(uint32(window))
	e.PutUint16(valueMask)
	e.PutUint16(0) // unused
	e.PutUint32(uint32(x))
	e.PutUint32(uint32(y))
	e.PutUint32(uint32(width))
	e.PutUint32(uint32(height))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: ConfigureWindow failed: %w", err)
	}
	return nil
}

// GetGeometry gets window geometry.
func (c *Connection) GetGeometry(drawable ResourceID) (x, y int16, width, height uint16, err error) {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeGetGeometry)
	e.PutUint8(0) // unused
	e.PutUint16(2) // length
	e.PutUint32(uint32(drawable))

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("x11: GetGeometry failed: %w", err)
	}

	// Parse reply
	// Reply: [1][depth:1][seq:2][length:4][root:4][x:2][y:2][width:2][height:2][border:2][unused:2]
	if len(reply) < 24 {
		return 0, 0, 0, 0, fmt.Errorf("x11: GetGeometry reply too short")
	}

	d := NewDecoder(c.byteOrder, reply[12:])
	x, _ = d.Int16()
	y, _ = d.Int16()
	width, _ = d.Uint16()
	height, _ = d.Uint16()

	return x, y, width, height, nil
}

// SetInputFocus sets the input focus to a window.
func (c *Connection) SetInputFocus(window ResourceID, revertTo uint8, time Timestamp) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeSetInputFocus)
	e.PutUint8(revertTo)
	e.PutUint16(3) // length
	e.PutUint32(uint32(window))
	e.PutUint32(uint32(time))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: SetInputFocus failed: %w", err)
	}
	return nil
}

// Revert to values for SetInputFocus.
const (
	RevertToNone        = 0
	RevertToPointerRoot = 1
	RevertToParent      = 2
)

// MotifWMHints structure for setting window decorations.
type MotifWMHints struct {
	Flags       uint32
	Functions   uint32
	Decorations uint32
	InputMode   int32
	Status      uint32
}

// Motif hint flags.
const (
	MotifHintsFunctions   = 1 << 0
	MotifHintsDecorations = 1 << 1
	MotifHintsInputMode   = 1 << 2
	MotifHintsStatus      = 1 << 3
)

// Motif decoration flags.
const (
	MotifDecorAll      = 1 << 0
	MotifDecorBorder   = 1 << 1
	MotifDecorResizeH  = 1 << 2
	MotifDecorTitle    = 1 << 3
	MotifDecorMenu     = 1 << 4
	MotifDecorMinimize = 1 << 5
	MotifDecorMaximize = 1 << 6
)

// SetMotifWMHints sets the _MOTIF_WM_HINTS property for window decorations.
func (c *Connection) SetMotifWMHints(window ResourceID, hints *MotifWMHints, atoms *StandardAtoms) error {
	if atoms.MotifWMHints == AtomNone {
		return nil
	}

	data := make([]byte, 20)
	c.putUint32LE(data[0:4], hints.Flags)
	c.putUint32LE(data[4:8], hints.Functions)
	c.putUint32LE(data[8:12], hints.Decorations)
	c.putUint32LE(data[12:16], uint32(hints.InputMode))
	c.putUint32LE(data[16:20], hints.Status)

	return c.ChangeProperty(window, atoms.MotifWMHints, atoms.MotifWMHints, 32, PropModeReplace, data)
}

// putUint32LE writes a uint32 in little-endian format.
func (c *Connection) putUint32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

// SetWindowBorderless removes window decorations (borderless window).
func (c *Connection) SetWindowBorderless(window ResourceID, atoms *StandardAtoms) error {
	hints := &MotifWMHints{
		Flags:       MotifHintsDecorations,
		Decorations: 0, // No decorations
	}
	return c.SetMotifWMHints(window, hints, atoms)
}

// SetFullscreen sets window to fullscreen state using _NET_WM_STATE.
func (c *Connection) SetFullscreen(window ResourceID, fullscreen bool, atoms *StandardAtoms) error {
	if atoms.NetWMState == AtomNone || atoms.NetWMStateFullscreen == AtomNone {
		return nil
	}

	// Send _NET_WM_STATE client message to window manager
	var action uint32
	if fullscreen {
		action = 1 // _NET_WM_STATE_ADD
	} else {
		action = 0 // _NET_WM_STATE_REMOVE
	}

	return c.SendClientMessage(window, c.RootWindow(), atoms.NetWMState,
		action, uint32(atoms.NetWMStateFullscreen), 0, 0, 0)
}

// SendClientMessage sends a ClientMessage event to a window.
func (c *Connection) SendClientMessage(window, target ResourceID, msgType Atom, data0, data1, data2, data3, data4 uint32) error {
	// Build event data
	eventData := make([]byte, 32)

	// Event type (ClientMessage = 33) + synthetic flag
	eventData[0] = EventClientMessage | 0x80 // Set synthetic flag
	// Format (32-bit)
	eventData[1] = 32
	// Sequence (unused for synthetic events)
	eventData[2] = 0
	eventData[3] = 0
	// Window
	c.putUint32LE(eventData[4:8], uint32(window))
	// Type
	c.putUint32LE(eventData[8:12], uint32(msgType))
	// Data
	c.putUint32LE(eventData[12:16], data0)
	c.putUint32LE(eventData[16:20], data1)
	c.putUint32LE(eventData[20:24], data2)
	c.putUint32LE(eventData[24:28], data3)
	c.putUint32LE(eventData[28:32], data4)

	// Build SendEvent request
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeSendEvent)
	e.PutUint8(0) // propagate = false
	e.PutUint16(11) // length = 11 4-byte units
	e.PutUint32(uint32(target))
	e.PutUint32(EventMaskSubstructureNotify | EventMaskSubstructureRedirect)
	e.PutBytes(eventData)

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: SendEvent failed: %w", err)
	}
	return nil
}
