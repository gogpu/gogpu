//go:build linux

package x11

// xdnd_source.go — XDND source accessors and SelectionRequest handling for drag.
//
// Provides exported accessors needed by platform/xdnd_source_linux.go to
// drive the XDND drag source protocol without exposing internals.

import (
	"encoding/binary"
	"log/slog"
	"sync"
)

// XdndAtoms holds exported references to interned XDND atoms.
// XdndAtoms wraps the unexported xdndAtoms for cross-package access.
type XdndAtoms = xdndAtoms

// Conn returns the X11 connection for direct protocol operations.
func (p *Platform) Conn() *Connection {
	return p.conn
}

// XDNDAtoms returns the interned XDND atoms, or nil if not initialized.
func (p *Platform) XDNDAtoms() *XdndAtoms {
	return p.xdnd
}

// PrimaryWindow returns the ResourceID of the primary window.
func (p *Platform) PrimaryWindow() ResourceID {
	if p.primary != nil {
		return p.primary.window
	}
	return 0
}

// RootWindow returns the root window via the connection.
func (p *Platform) RootWindow() ResourceID {
	if p.conn != nil {
		return p.conn.RootWindow()
	}
	return 0
}

// dragSourceData holds the data being offered during an outgoing drag.
var dragSourceData struct {
	mu       sync.Mutex
	uriData  string
	mimeAtom Atom
}

// SetDragSourceData sets or clears the data for outgoing drag SelectionRequest handling.
// Pass empty uriData and atom=0 to clear.
func (p *Platform) SetDragSourceData(uriData string, mimeAtom Atom) {
	dragSourceData.mu.Lock()
	dragSourceData.uriData = uriData
	dragSourceData.mimeAtom = mimeAtom
	dragSourceData.mu.Unlock()
}

// HandleDragSelectionRequest handles SelectionRequest events during a drag session.
// The target requests the drag data (text/uri-list) via the XdndSelection.
func (p *Platform) HandleDragSelectionRequest(e *SelectionRequestEvent) {
	if p.xdnd == nil || p.conn == nil {
		return
	}

	// Only handle requests for XdndSelection.
	if e.Selection != p.xdnd.Selection {
		// Delegate to clipboard handler for non-XDND selections.
		p.handleSelectionRequest(e)
		return
	}

	property := e.Property
	if property == AtomNone {
		property = e.Target
	}

	dragSourceData.mu.Lock()
	data := dragSourceData.uriData
	mimeAtom := dragSourceData.mimeAtom
	dragSourceData.mu.Unlock()

	responded := false

	if data != "" {
		switch e.Target {
		case mimeAtom:
			// Respond with the URI list data.
			err := p.conn.ChangeProperty(
				e.Requestor,
				property,
				mimeAtom,
				8, // format: 8-bit bytes
				PropModeReplace,
				[]byte(data),
			)
			if err == nil {
				responded = true
			} else {
				slog.Warn("x11: drag ChangeProperty failed", "err", err)
			}

		case p.atoms.Targets:
			// Respond with supported targets: TARGETS + text/uri-list.
			targets := make([]byte, 8)
			putUint32LE(targets[0:4], uint32(p.atoms.Targets))
			putUint32LE(targets[4:8], uint32(mimeAtom))
			err := p.conn.ChangeProperty(
				e.Requestor,
				property,
				AtomAtom,
				32, // 32-bit atoms
				PropModeReplace,
				targets,
			)
			if err == nil {
				responded = true
			}
		}
	}

	// Send SelectionNotify response.
	p.sendSelectionNotify(e, property, responded)
}

// TranslateCoordinates translates coordinates from src to dst window.
// Returns (child, dstX, dstY, error).
func (c *Connection) TranslateCoordinates(src, dst ResourceID, srcX, srcY int16) (ResourceID, int16, int16, error) {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeTranslateCoords)
	e.PutUint8(0)  // unused
	e.PutUint16(4) // length = 4 32-bit units
	e.PutUint32(uint32(src))
	e.PutUint32(uint32(dst))
	e.PutInt16(srcX)
	e.PutInt16(srcY)

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return 0, 0, 0, err
	}

	if len(reply) < 16 {
		return 0, 0, 0, nil
	}

	// Reply format: [1:same_screen][1:unused][2:seq][4:len][4:child][2:dst_x][2:dst_y]
	child := ResourceID(binary.LittleEndian.Uint32(reply[8:12]))
	dstX := int16(binary.LittleEndian.Uint16(reply[12:14]))
	dstY := int16(binary.LittleEndian.Uint16(reply[14:16]))

	return child, dstX, dstY, nil
}
