//go:build linux

package x11

import (
	"fmt"
)

// Common atom names used by window managers.
const (
	AtomNameWMProtocols    = "WM_PROTOCOLS"
	AtomNameWMDeleteWindow = "WM_DELETE_WINDOW"
	AtomNameWMTakeFocus    = "WM_TAKE_FOCUS"
	AtomNameWMState        = "WM_STATE"
	AtomNameNetWMName      = "_NET_WM_NAME"
	AtomNameNetWMState     = "_NET_WM_STATE"
	AtomNameNetWMStateFullscreen = "_NET_WM_STATE_FULLSCREEN"
	AtomNameNetWMStateMaximizedVert = "_NET_WM_STATE_MAXIMIZED_VERT"
	AtomNameNetWMStateMaximizedHorz = "_NET_WM_STATE_MAXIMIZED_HORZ"
	AtomNameNetWMStateHidden = "_NET_WM_STATE_HIDDEN"
	AtomNameNetWMWindowType = "_NET_WM_WINDOW_TYPE"
	AtomNameNetWMWindowTypeNormal = "_NET_WM_WINDOW_TYPE_NORMAL"
	AtomNameNetWMPID        = "_NET_WM_PID"
	AtomNameNetWMIcon       = "_NET_WM_ICON"
	AtomNameNetFrameExtents = "_NET_FRAME_EXTENTS"
	AtomNameUTF8String      = "UTF8_STRING"
	AtomNameMotifWMHints    = "_MOTIF_WM_HINTS"
)

// InternAtom interns an atom name and returns its ID.
// If onlyIfExists is true, returns AtomNone if the atom doesn't exist.
func (c *Connection) InternAtom(name string, onlyIfExists bool) (Atom, error) {
	// Check cache first
	c.atomCacheLock.RLock()
	if atom, ok := c.atomCache[name]; ok {
		c.atomCacheLock.RUnlock()
		return atom, nil
	}
	c.atomCacheLock.RUnlock()

	// Build request
	nameLen := len(name)
	reqLen := 2 + requestLength(nameLen) // 2 for header, rest for name

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeInternAtom)
	if onlyIfExists {
		e.PutUint8(1)
	} else {
		e.PutUint8(0)
	}
	e.PutUint16(reqLen)
	e.PutUint16(uint16(nameLen))
	e.PutUint16(0) // unused
	e.PutBytes([]byte(name))
	e.PutPad()

	// Send request and wait for reply
	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return AtomNone, fmt.Errorf("x11: InternAtom failed: %w", err)
	}

	// Parse reply
	// Reply format: [1][unused][seq:2][length:4][atom:4][unused:20]
	if len(reply) < 12 {
		return AtomNone, fmt.Errorf("x11: InternAtom reply too short")
	}

	d := NewDecoder(c.byteOrder, reply[8:12])
	atomID, err := d.Uint32()
	if err != nil {
		return AtomNone, err
	}

	atom := Atom(atomID)

	// Cache the result
	if atom != AtomNone {
		c.atomCacheLock.Lock()
		c.atomCache[name] = atom
		c.atomCacheLock.Unlock()
	}

	return atom, nil
}

// GetAtomName returns the name of an atom.
func (c *Connection) GetAtomName(atom Atom) (string, error) {
	// Check cache first (reverse lookup)
	c.atomCacheLock.RLock()
	for name, a := range c.atomCache {
		if a == atom {
			c.atomCacheLock.RUnlock()
			return name, nil
		}
	}
	c.atomCacheLock.RUnlock()

	// Build request
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeGetAtomName)
	e.PutUint8(0) // unused
	e.PutUint16(2) // length in 4-byte units
	e.PutUint32(uint32(atom))

	// Send request and wait for reply
	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return "", fmt.Errorf("x11: GetAtomName failed: %w", err)
	}

	// Parse reply
	// Reply format: [1][unused][seq:2][length:4][name_len:2][unused:22][name...]
	if len(reply) < 32 {
		return "", fmt.Errorf("x11: GetAtomName reply too short")
	}

	d := NewDecoder(c.byteOrder, reply[8:10])
	nameLen, err := d.Uint16()
	if err != nil {
		return "", err
	}

	if len(reply) < 32+int(nameLen) {
		return "", fmt.Errorf("x11: GetAtomName reply truncated")
	}

	name := string(reply[32 : 32+nameLen])

	// Cache the result
	c.atomCacheLock.Lock()
	c.atomCache[name] = atom
	c.atomCacheLock.Unlock()

	return name, nil
}

// InternAtoms interns multiple atom names at once.
// This is more efficient than calling InternAtom for each name.
func (c *Connection) InternAtoms(names []string) (map[string]Atom, error) {
	result := make(map[string]Atom)

	// Check cache and build list of atoms to request
	var toRequest []string
	c.atomCacheLock.RLock()
	for _, name := range names {
		if atom, ok := c.atomCache[name]; ok {
			result[name] = atom
		} else {
			toRequest = append(toRequest, name)
		}
	}
	c.atomCacheLock.RUnlock()

	// Request remaining atoms
	for _, name := range toRequest {
		atom, err := c.InternAtom(name, false)
		if err != nil {
			return nil, err
		}
		result[name] = atom
	}

	return result, nil
}

// StandardAtoms contains commonly used atoms that are interned at connection time.
type StandardAtoms struct {
	WMProtocols    Atom
	WMDeleteWindow Atom
	WMTakeFocus    Atom
	WMState        Atom
	NetWMName      Atom
	NetWMState     Atom
	NetWMStateFullscreen Atom
	NetWMStateMaximizedVert Atom
	NetWMStateMaximizedHorz Atom
	NetWMWindowType Atom
	NetWMWindowTypeNormal Atom
	NetWMPID       Atom
	UTF8String     Atom
	MotifWMHints   Atom
}

// InternStandardAtoms interns all standard atoms needed for windowing.
func (c *Connection) InternStandardAtoms() (*StandardAtoms, error) {
	atoms := &StandardAtoms{}

	var err error

	atoms.WMProtocols, err = c.InternAtom(AtomNameWMProtocols, false)
	if err != nil {
		return nil, err
	}

	atoms.WMDeleteWindow, err = c.InternAtom(AtomNameWMDeleteWindow, false)
	if err != nil {
		return nil, err
	}

	atoms.WMTakeFocus, err = c.InternAtom(AtomNameWMTakeFocus, false)
	if err != nil {
		return nil, err
	}

	atoms.WMState, err = c.InternAtom(AtomNameWMState, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMName, err = c.InternAtom(AtomNameNetWMName, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMState, err = c.InternAtom(AtomNameNetWMState, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMStateFullscreen, err = c.InternAtom(AtomNameNetWMStateFullscreen, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMStateMaximizedVert, err = c.InternAtom(AtomNameNetWMStateMaximizedVert, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMStateMaximizedHorz, err = c.InternAtom(AtomNameNetWMStateMaximizedHorz, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMWindowType, err = c.InternAtom(AtomNameNetWMWindowType, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMWindowTypeNormal, err = c.InternAtom(AtomNameNetWMWindowTypeNormal, false)
	if err != nil {
		return nil, err
	}

	atoms.NetWMPID, err = c.InternAtom(AtomNameNetWMPID, false)
	if err != nil {
		return nil, err
	}

	atoms.UTF8String, err = c.InternAtom(AtomNameUTF8String, false)
	if err != nil {
		return nil, err
	}

	atoms.MotifWMHints, err = c.InternAtom(AtomNameMotifWMHints, false)
	if err != nil {
		return nil, err
	}

	return atoms, nil
}
