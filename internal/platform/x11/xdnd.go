//go:build linux

// XDND (X Drag-and-Drop) Protocol v5 implementation.
//
// Implements the target (drop receiver) side of XDND for file drag-and-drop.
// Reference: https://freedesktop.org/wiki/Specifications/XDND/
//
// Protocol flow (target perspective):
//  1. Source sends XdndEnter → we store source window and supported MIME types
//  2. Source sends XdndPosition → we reply with XdndStatus (accept/reject)
//  3. Source sends XdndDrop → we request data via ConvertSelection
//  4. We receive SelectionNotify → parse file:// URIs → queue EventTypeDragDrop
//  5. We send XdndFinished to source
//
// Or: Source sends XdndLeave → we queue EventTypeDragLeave
package x11

import (
	"encoding/binary"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// XDND atom name constants.
const (
	AtomNameXdndAware     = "XdndAware"
	AtomNameXdndEnter     = "XdndEnter"
	AtomNameXdndLeave     = "XdndLeave"
	AtomNameXdndPosition  = "XdndPosition"
	AtomNameXdndStatus    = "XdndStatus"
	AtomNameXdndDrop      = "XdndDrop"
	AtomNameXdndFinished  = "XdndFinished"
	AtomNameXdndSelection = "XdndSelection"
	AtomNameXdndActionCopy = "XdndActionCopy"
	AtomNameXdndTypeList  = "XdndTypeList"
	AtomNameTextURIList   = "text/uri-list"
)

// xdndAtoms holds interned XDND atoms.
type xdndAtoms struct {
	Aware      Atom
	Enter      Atom
	Leave      Atom
	Position   Atom
	Status     Atom
	Drop       Atom
	Finished   Atom
	Selection  Atom
	ActionCopy Atom
	TypeList   Atom
	TextURIList Atom
}

// xdndState tracks per-window XDND drag session state.
type xdndState struct {
	sourceWindow ResourceID // source window that initiated the drag
	version      uint32     // XDND protocol version of source
	hasURIList   bool       // source supports text/uri-list
	lastX        float64    // last position (physical pixels)
	lastY        float64    // last position (physical pixels)
	dropPending  bool       // waiting for SelectionNotify after XdndDrop
}

// internXdndAtoms interns all XDND atoms.
func (p *Platform) internXdndAtoms() error {
	var err error
	a := &xdndAtoms{}

	a.Aware, err = p.conn.InternAtom(AtomNameXdndAware, false)
	if err != nil {
		return err
	}
	a.Enter, err = p.conn.InternAtom(AtomNameXdndEnter, false)
	if err != nil {
		return err
	}
	a.Leave, err = p.conn.InternAtom(AtomNameXdndLeave, false)
	if err != nil {
		return err
	}
	a.Position, err = p.conn.InternAtom(AtomNameXdndPosition, false)
	if err != nil {
		return err
	}
	a.Status, err = p.conn.InternAtom(AtomNameXdndStatus, false)
	if err != nil {
		return err
	}
	a.Drop, err = p.conn.InternAtom(AtomNameXdndDrop, false)
	if err != nil {
		return err
	}
	a.Finished, err = p.conn.InternAtom(AtomNameXdndFinished, false)
	if err != nil {
		return err
	}
	a.Selection, err = p.conn.InternAtom(AtomNameXdndSelection, false)
	if err != nil {
		return err
	}
	a.ActionCopy, err = p.conn.InternAtom(AtomNameXdndActionCopy, false)
	if err != nil {
		return err
	}
	a.TypeList, err = p.conn.InternAtom(AtomNameXdndTypeList, false)
	if err != nil {
		return err
	}
	a.TextURIList, err = p.conn.InternAtom(AtomNameTextURIList, false)
	if err != nil {
		return err
	}

	p.xdnd = a
	return nil
}

// setXdndAware sets the XdndAware property on a window to advertise XDND v5
// support. Must be called after window creation.
func (p *Platform) setXdndAware(window ResourceID) error {
	if p.xdnd == nil {
		return nil
	}
	// XdndAware property: single ATOM value with version number (5).
	version := make([]byte, 4)
	binary.LittleEndian.PutUint32(version, 5)
	return p.conn.ChangeProperty(
		window,
		p.xdnd.Aware,
		AtomAtom, // type = ATOM (version is stored as XA_ATOM format)
		32,       // format: 32-bit
		PropModeReplace,
		version,
	)
}

// handleXdndClientMessage handles XDND-related ClientMessage events.
// Returns a PlatformEvent if the message generated a user-visible event,
// or a zero-value PlatformEvent if handled internally.
func (p *Platform) handleXdndClientMessage(w *x11Window, e *ClientMessageEvent) PlatformEvent {
	if p.xdnd == nil {
		return PlatformEvent{Type: EventTypeNone}
	}

	data := e.Data32()

	switch e.Type {
	case p.xdnd.Enter:
		return p.handleXdndEnter(w, data)

	case p.xdnd.Position:
		return p.handleXdndPosition(w, data)

	case p.xdnd.Drop:
		return p.handleXdndDrop(w, data)

	case p.xdnd.Leave:
		return p.handleXdndLeave(w)
	}

	return PlatformEvent{Type: EventTypeNone}
}

// handleXdndEnter processes the XdndEnter message.
// data[0] = source window, data[1] = version (bits 24-31) | more-types flag (bit 0),
// data[2-4] = first 3 MIME type atoms (or 0 if more-types flag is set).
func (p *Platform) handleXdndEnter(w *x11Window, data [5]uint32) PlatformEvent {
	w.xdndState.sourceWindow = ResourceID(data[0])
	w.xdndState.version = (data[1] >> 24) & 0xFF
	w.xdndState.hasURIList = false
	w.xdndState.dropPending = false

	moreTypes := (data[1] & 1) != 0

	if moreTypes {
		// Source supports more than 3 types — read XdndTypeList property.
		w.xdndState.hasURIList = p.checkXdndTypeList(w.xdndState.sourceWindow)
	} else {
		// Check inline type atoms (data[2], data[3], data[4]).
		for i := 2; i <= 4; i++ {
			if Atom(data[i]) == p.xdnd.TextURIList {
				w.xdndState.hasURIList = true
				break
			}
		}
	}

	return PlatformEvent{
		Type:  EventTypeDragEnter,
		DragX: w.xdndState.lastX,
		DragY: w.xdndState.lastY,
	}
}

// handleXdndPosition processes the XdndPosition message and sends XdndStatus reply.
// data[0] = source window, data[2] = position (x<<16 | y),
// data[3] = timestamp, data[4] = action atom.
func (p *Platform) handleXdndPosition(w *x11Window, data [5]uint32) PlatformEvent {
	x := float64(data[2] >> 16)
	y := float64(data[2] & 0xFFFF)

	// Convert root coordinates to window-local coordinates.
	// XdndPosition sends root-relative coords; we need window-relative.
	if w.window != 0 {
		geom, err := p.conn.getGeometry(w.window)
		if err == nil {
			// Translate root coords to window coords by subtracting window origin.
			// For nested windows, we need TranslateCoordinates, but for
			// top-level windows getGeometry + border gives us the offset.
			x -= float64(geom.x)
			y -= float64(geom.y)
		}
	}

	w.xdndState.lastX = x
	w.xdndState.lastY = y

	// Send XdndStatus: accept if we support the type, reject otherwise.
	accept := w.xdndState.hasURIList
	p.sendXdndStatus(w, accept)

	return PlatformEvent{
		Type:  EventTypeDragMove,
		DragX: x,
		DragY: y,
	}
}

// handleXdndDrop processes the XdndDrop message.
// Requests the selection data via ConvertSelection.
// data[0] = source window, data[2] = timestamp.
func (p *Platform) handleXdndDrop(w *x11Window, data [5]uint32) PlatformEvent {
	if !w.xdndState.hasURIList {
		p.sendXdndFinished(w, false)
		return PlatformEvent{Type: EventTypeNone}
	}

	w.xdndState.dropPending = true

	// Request the drag data via the selection mechanism.
	timestamp := Timestamp(data[2])
	if err := p.conn.ConvertSelection(
		w.window,
		p.xdnd.Selection,
		p.xdnd.TextURIList,
		p.xdnd.Selection, // use XdndSelection as property
		timestamp,
	); err != nil {
		slog.Warn("xdnd: ConvertSelection failed", "err", err)
		w.xdndState.dropPending = false
		p.sendXdndFinished(w, false)
		return PlatformEvent{Type: EventTypeNone}
	}

	if err := p.conn.Flush(); err != nil {
		slog.Warn("xdnd: flush after ConvertSelection failed", "err", err)
	}

	// The actual data arrives via SelectionNotify — we handle it there.
	// For now, pump events with a timeout to receive the SelectionNotify.
	return p.waitForXdndSelectionNotify(w)
}

// handleXdndLeave processes the XdndLeave message.
func (p *Platform) handleXdndLeave(w *x11Window) PlatformEvent {
	w.xdndState = xdndState{} // reset
	return PlatformEvent{Type: EventTypeDragLeave}
}

// waitForXdndSelectionNotify pumps events until SelectionNotify arrives for
// the XDND selection, or until a 1-second timeout. This mirrors the clipboard
// read pattern (clipboard.go:ClipboardRead).
func (p *Platform) waitForXdndSelectionNotify(w *x11Window) PlatformEvent {
	deadline := time.Now().Add(time.Second)
	for {
		if time.Now().After(deadline) {
			slog.Warn("xdnd: SelectionNotify timeout")
			w.xdndState.dropPending = false
			p.sendXdndFinished(w, false)
			return PlatformEvent{Type: EventTypeNone}
		}

		event, err := p.conn.PollEventTimeout(50 * time.Millisecond)
		if err != nil || event == nil {
			continue
		}

		if notify, ok := event.(*SelectionNotifyEvent); ok {
			if notify.Selection == p.xdnd.Selection && notify.Requestor == w.window {
				return p.processXdndSelectionNotify(w, notify)
			}
		}
	}
}

// processXdndSelectionNotify reads the dropped file paths from the selection
// property, parses the text/uri-list, and returns the drop event.
func (p *Platform) processXdndSelectionNotify(w *x11Window, notify *SelectionNotifyEvent) PlatformEvent {
	defer func() {
		w.xdndState.dropPending = false
		w.xdndState = xdndState{} // reset for next drag
	}()

	if notify.Property == AtomNone {
		p.sendXdndFinished(w, false)
		return PlatformEvent{Type: EventTypeNone}
	}

	// Read the property data (text/uri-list)
	data, _, _, err := p.conn.GetProperty(
		w.window,
		notify.Property,
		Atom(0), // any type
		0, 65536, // up to 256KB of path data
		true, // delete after read
	)
	if err != nil {
		slog.Warn("xdnd: GetProperty failed", "err", err)
		p.sendXdndFinished(w, false)
		return PlatformEvent{Type: EventTypeNone}
	}

	paths := parseURIList(string(data))
	if len(paths) == 0 {
		p.sendXdndFinished(w, false)
		return PlatformEvent{Type: EventTypeNone}
	}

	p.sendXdndFinished(w, true)

	return PlatformEvent{
		Type:      EventTypeDragDrop,
		DragPaths: paths,
		DragX:     w.xdndState.lastX,
		DragY:     w.xdndState.lastY,
	}
}

// sendXdndStatus sends an XdndStatus reply to the source window.
// accept=true means we will accept the drop; accept=false rejects it.
func (p *Platform) sendXdndStatus(w *x11Window, accept bool) {
	var flags uint32
	if accept {
		flags = 1 // bit 0: accept drop
		// bit 1 (want position) = 0: no further XdndPosition for entire window
	}

	action := uint32(0)
	if accept {
		action = uint32(p.xdnd.ActionCopy)
	}

	if err := p.conn.SendClientMessage(
		w.xdndState.sourceWindow, // target for SendEvent
		w.xdndState.sourceWindow, // window field
		p.xdnd.Status,
		uint32(w.window), // data[0]: our window
		flags,            // data[1]: flags
		0,                // data[2]: x, y of skip rectangle (unused)
		0,                // data[3]: width, height of skip rectangle (unused)
		action,           // data[4]: accepted action
	); err != nil {
		slog.Warn("xdnd: SendClientMessage (Status) failed", "err", err)
	}
	if err := p.conn.Flush(); err != nil {
		slog.Warn("xdnd: flush after Status failed", "err", err)
	}
}

// sendXdndFinished sends an XdndFinished message to the source window,
// signaling that the drop operation is complete.
func (p *Platform) sendXdndFinished(w *x11Window, accepted bool) {
	var flags uint32
	if accepted {
		flags = 1
	}
	action := uint32(0)
	if accepted {
		action = uint32(p.xdnd.ActionCopy)
	}

	if err := p.conn.SendClientMessage(
		w.xdndState.sourceWindow,
		w.xdndState.sourceWindow,
		p.xdnd.Finished,
		uint32(w.window), // data[0]: target window
		flags,            // data[1]: accepted flag
		action,           // data[2]: accepted action
		0, 0,
	); err != nil {
		slog.Warn("xdnd: SendClientMessage (Finished) failed", "err", err)
	}
	if err := p.conn.Flush(); err != nil {
		slog.Warn("xdnd: flush after Finished failed", "err", err)
	}
}

// checkXdndTypeList reads the XdndTypeList property from the source window
// to check if text/uri-list is among the supported types.
func (p *Platform) checkXdndTypeList(source ResourceID) bool {
	data, _, _, err := p.conn.GetProperty(
		source,
		p.xdnd.TypeList,
		AtomAtom, // type = ATOM
		0, 256,   // up to 256 atoms
		false, // don't delete
	)
	if err != nil || len(data) == 0 {
		return false
	}

	// data contains 32-bit atom values
	for i := 0; i+3 < len(data); i += 4 {
		atom := Atom(binary.LittleEndian.Uint32(data[i : i+4]))
		if atom == p.xdnd.TextURIList {
			return true
		}
	}
	return false
}

// getGeometry is a simple wrapper to query window geometry (position + size).
func (c *Connection) getGeometry(window ResourceID) (struct{ x, y int16 }, error) {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeGetGeometry)
	e.PutUint8(0)  // unused
	e.PutUint16(2) // length = 2 4-byte units
	e.PutUint32(uint32(window))

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return struct{ x, y int16 }{}, err
	}

	if len(reply) < 24 {
		return struct{ x, y int16 }{}, nil
	}

	// Reply: [1:depth][1:unused][2:seq][4:len][4:root][2:x][2:y][2:w][2:h]...
	x := int16(binary.LittleEndian.Uint16(reply[12:14]))
	y := int16(binary.LittleEndian.Uint16(reply[14:16]))
	return struct{ x, y int16 }{x, y}, nil
}

// parseURIList parses a text/uri-list string (RFC 2483) into file paths.
// Each line is a URI; lines starting with # are comments.
// Only file:// URIs are converted to local paths; others are skipped.
// Percent-encoded characters (%XX) are decoded.
func parseURIList(data string) []string {
	var paths []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse as URL to handle percent-encoding.
		u, err := url.Parse(line)
		if err != nil {
			continue
		}

		if u.Scheme != "file" {
			continue
		}

		// url.Parse handles percent-decoding in Path.
		path := u.Path
		if path == "" {
			continue
		}

		paths = append(paths, path)
	}
	return paths
}
