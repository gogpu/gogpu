//go:build linux

package platform

// xdnd_source_linux.go — XDND (X Drag-and-Drop) source implementation.
//
// Implements the SOURCE side of the XDND v5 protocol for outgoing file
// drag-and-drop. The target side is in x11/xdnd.go.
//
// Protocol flow (source perspective):
//   1. Grab pointer (XGrabPointer)
//   2. Set XdndSelection owner to our window
//   3. On pointer motion: find target window under cursor, send XdndEnter then XdndPosition
//   4. Target replies with XdndStatus (accept/reject)
//   5. On button release: if target accepted, send XdndDrop; else cancel
//   6. Target requests data via SelectionRequest → we respond with text/uri-list
//   7. Target sends XdndFinished → we ungrab and report result
//
// Reference: https://freedesktop.org/wiki/Specifications/XDND/

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gogpu/gogpu/internal/platform/x11"
)

// startXDNDDrag initiates an XDND drag session from the given X11 platform.
// Blocks until the drag completes (grab loop). The done callback receives DragResult.
func startXDNDDrag(p *x11.Platform, paths []string, done func(DragResult)) {
	if p == nil {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	conn := p.Conn()
	if conn == nil {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	xdndAtoms := p.XDNDAtoms()
	if xdndAtoms == nil {
		slog.Warn("x11: XDND atoms not initialized, cannot start drag")
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	window := p.PrimaryWindow()
	if window == 0 {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	// Build the URI list data.
	uriData := buildURIList(paths)

	// Take ownership of XdndSelection.
	if err := conn.SetSelectionOwner(xdndAtoms.Selection, window, x11.CurrentTime); err != nil {
		slog.Warn("x11: SetSelectionOwner for XdndSelection failed", "err", err)
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	// Grab the pointer for the drag session.
	// EventMask: PointerMotion (bit 6) | ButtonRelease (bit 3) | ButtonPress (bit 2)
	const grabEventMask uint16 = (1 << 6) | (1 << 3) | (1 << 2)
	// GrabPointer(ownerEvents, grabWindow, eventMask, pointerMode, keyboardMode, confineTo, cursor, timestamp)
	// pointerMode=1 (Async), keyboardMode=1 (Async), confineTo=0 (None), cursor=0 (None)
	if _, err := conn.GrabPointer(true, window, grabEventMask, 1, 1, 0, 0, x11.CurrentTime); err != nil {
		slog.Warn("x11: GrabPointer for drag failed", "err", err)
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	// Register a handler for SelectionRequest events during the drag.
	p.SetDragSourceData(uriData, xdndAtoms.TextURIList)

	// Run the drag loop.
	result := runXDNDDragLoop(p, conn, xdndAtoms, window, uriData)

	// Ungrab pointer.
	if err := conn.UngrabPointer(x11.CurrentTime); err != nil {
		slog.Warn("x11: UngrabPointer after drag failed", "err", err)
	}

	// Clear selection ownership.
	if err := conn.SetSelectionOwner(xdndAtoms.Selection, 0, x11.CurrentTime); err != nil {
		slog.Warn("x11: clear XdndSelection owner failed", "err", err)
	}

	// Clear drag source data.
	p.SetDragSourceData("", 0)

	if err := conn.Flush(); err != nil {
		slog.Warn("x11: flush after drag cleanup failed", "err", err)
	}

	if done != nil {
		done(result)
	}
}

// xdndDragState tracks the state of the XDND drag loop.
type xdndDragState struct {
	currentTarget x11.ResourceID
	targetAccepts bool
	sourceWindow  x11.ResourceID
	rootWindow    x11.ResourceID
	started       bool
}

// runXDNDDragLoop runs the modal drag event loop.
func runXDNDDragLoop(p *x11.Platform, conn *x11.Connection, atoms *x11.XdndAtoms, sourceWindow x11.ResourceID, _ string) DragResult {
	ds := &xdndDragState{
		sourceWindow: sourceWindow,
		rootWindow:   conn.RootWindow(),
	}

	// Drain events queued before the grab — they belong to the application,
	// not to this drag session. A stale ButtonRelease would end the gesture
	// immediately (#431, @unxed).
	for {
		ev, err := conn.PollEventTimeout(0)
		if err != nil || ev == nil {
			break
		}
	}
	ds.started = true

	deadline := time.Now().Add(5 * time.Minute) // safety timeout

	for time.Now().Before(deadline) {
		event, err := conn.PollEventTimeout(50 * time.Millisecond)
		if err != nil || event == nil {
			continue
		}

		if result, done := ds.handleEvent(p, conn, atoms, event); done {
			return result
		}
	}

	return DragCancelled
}

// handleEvent processes a single event during the XDND drag loop.
// Returns (result, true) if the drag is finished, (0, false) to continue.
func (ds *xdndDragState) handleEvent(p *x11.Platform, conn *x11.Connection, atoms *x11.XdndAtoms, event x11.Event) (DragResult, bool) {
	switch e := event.(type) {
	case *x11.MotionNotifyEvent:
		ds.handleMotion(conn, atoms, e)

	case *x11.ClientMessageEvent:
		return ds.handleClientMessage(conn, atoms, e)

	case *x11.ButtonReleaseEvent:
		return ds.handleButtonRelease(p, conn, atoms)

	case *x11.KeyPressEvent:
		// Keycode 9 = Escape on most X11 systems.
		if e.Detail == 9 {
			if ds.currentTarget != 0 {
				sendXdndLeaveSource(conn, atoms, ds.sourceWindow, ds.currentTarget)
			}
			return DragCancelled, true
		}

	case *x11.SelectionRequestEvent:
		p.HandleDragSelectionRequest(e)
	}
	return 0, false
}

func (ds *xdndDragState) handleMotion(conn *x11.Connection, atoms *x11.XdndAtoms, e *x11.MotionNotifyEvent) {
	targetWin := findXDNDTargetWindow(conn, ds.rootWindow, e.RootX, e.RootY, atoms.Aware)

	if targetWin != ds.currentTarget {
		if ds.currentTarget != 0 {
			sendXdndLeaveSource(conn, atoms, ds.sourceWindow, ds.currentTarget)
		}
		ds.currentTarget = targetWin
		ds.targetAccepts = false

		if ds.currentTarget != 0 {
			sendXdndEnterSource(conn, atoms, ds.sourceWindow, ds.currentTarget)
		}
	}

	if ds.currentTarget != 0 {
		sendXdndPosition(conn, atoms, ds.sourceWindow, ds.currentTarget, e.RootX, e.RootY)
	}
}

func (ds *xdndDragState) handleClientMessage(conn *x11.Connection, atoms *x11.XdndAtoms, e *x11.ClientMessageEvent) (DragResult, bool) {
	switch e.Type {
	case atoms.Status:
		data := e.Data32()
		if !ds.ownsSession(x11.ResourceID(data[0])) {
			return 0, false
		}
		ds.targetAccepts = (data[1] & 1) != 0
		return 0, false

	case atoms.Finished:
		data := e.Data32()
		if !ds.ownsSession(x11.ResourceID(data[0])) {
			return 0, false
		}
		return dragResultFromFinished(atoms, data), true
	}
	return 0, false
}

// ownsSession checks that a reply belongs to the current drag target.
// Prevents stale XdndStatus/Finished from a previous session from ending this one.
func (ds *xdndDragState) ownsSession(window x11.ResourceID) bool {
	return ds.currentTarget != 0 && window == ds.currentTarget
}

// dragResultFromFinished interprets XdndFinished. We only offer ActionCopy,
// so an accepted drop with any other action is not a success.
func dragResultFromFinished(atoms *x11.XdndAtoms, data [5]uint32) DragResult {
	if data[1]&1 == 0 {
		return DragCancelled
	}
	if x11.Atom(data[2]) == atoms.ActionCopy {
		return DragCopied
	}
	slog.Warn("x11: drop accepted with unexpected action",
		"action", data[2], "offered", uint32(atoms.ActionCopy))
	return DragCancelled
}

func (ds *xdndDragState) handleButtonRelease(p *x11.Platform, conn *x11.Connection, atoms *x11.XdndAtoms) (DragResult, bool) {
	if !ds.started {
		return 0, false
	}
	if ds.currentTarget != 0 && ds.targetAccepts {
		sendXdndDrop(conn, atoms, ds.sourceWindow, ds.currentTarget)
		return ds.waitForXdndFinished(p, conn, atoms, 5*time.Second), true
	}
	if ds.currentTarget != 0 {
		sendXdndLeaveSource(conn, atoms, ds.sourceWindow, ds.currentTarget)
	}
	return DragCancelled, true
}

// findXDNDTargetWindow finds the deepest XDND-aware window under the cursor.
// awareAtom is the pre-interned XdndAware atom to avoid a sync roundtrip per call.
func findXDNDTargetWindow(conn *x11.Connection, root x11.ResourceID, rootX, rootY int16, awareAtom x11.Atom) x11.ResourceID {
	child, _, _, err := conn.TranslateCoordinates(root, root, rootX, rootY)
	if err != nil || child == 0 {
		return 0
	}

	target := child
	for {
		if hasXDNDAware(conn, target, awareAtom) {
			return target
		}
		deeper, _, _, err := conn.TranslateCoordinates(root, target, rootX, rootY)
		if err != nil || deeper == 0 || deeper == target {
			break
		}
		target = deeper
	}

	if hasXDNDAware(conn, child, awareAtom) {
		return child
	}
	return 0
}

// hasXDNDAware checks if a window has the XdndAware property using a cached atom.
func hasXDNDAware(conn *x11.Connection, window x11.ResourceID, awareAtom x11.Atom) bool {
	data, _, _, err := conn.GetProperty(window, awareAtom, x11.AtomAtom, 0, 1, false)
	if err != nil || len(data) < 4 {
		return false
	}
	return true
}

// sendXdndEnterSource sends XdndEnter to the target.
func sendXdndEnterSource(conn *x11.Connection, atoms *x11.XdndAtoms, source, target x11.ResourceID) {
	// data[0] = source window
	// data[1] = version (5) << 24 | flags (0 = types inline)
	// data[2] = first type = text/uri-list
	// data[3] = 0 (second type)
	// data[4] = 0 (third type)
	if err := conn.SendClientMessageDirect(target, target, atoms.Enter,
		uint32(source),
		5<<24, // version 5, types inline (bit 0 = 0)
		uint32(atoms.TextURIList),
		0, 0,
	); err != nil {
		slog.Warn("x11: send XdndEnter failed", "err", err)
	}
	if err := conn.Flush(); err != nil {
		slog.Warn("x11: flush after XdndEnter failed", "err", err)
	}
}

// sendXdndPosition sends XdndPosition to the target.
func sendXdndPosition(conn *x11.Connection, atoms *x11.XdndAtoms, source, target x11.ResourceID, rootX, rootY int16) {
	// data[0] = source window
	// data[1] = 0 (reserved)
	// data[2] = position: x << 16 | y
	// data[3] = timestamp (CurrentTime = 0)
	// data[4] = action = XdndActionCopy
	pos := uint32(uint16(rootX))<<16 | uint32(uint16(rootY))
	if err := conn.SendClientMessageDirect(target, target, atoms.Position,
		uint32(source),
		0,
		pos,
		0, // CurrentTime
		uint32(atoms.ActionCopy),
	); err != nil {
		slog.Warn("x11: send XdndPosition failed", "err", err)
	}
	if err := conn.Flush(); err != nil {
		slog.Warn("x11: flush after XdndPosition failed", "err", err)
	}
}

// sendXdndDrop sends XdndDrop to the target.
func sendXdndDrop(conn *x11.Connection, atoms *x11.XdndAtoms, source, target x11.ResourceID) {
	if err := conn.SendClientMessageDirect(target, target, atoms.Drop,
		uint32(source),
		0,
		0, // CurrentTime
		0, 0,
	); err != nil {
		slog.Warn("x11: send XdndDrop failed", "err", err)
	}
	if err := conn.Flush(); err != nil {
		slog.Warn("x11: flush after XdndDrop failed", "err", err)
	}
}

// sendXdndLeaveSource sends XdndLeave to the target.
func sendXdndLeaveSource(conn *x11.Connection, atoms *x11.XdndAtoms, source, target x11.ResourceID) {
	if err := conn.SendClientMessageDirect(target, target, atoms.Leave,
		uint32(source),
		0, 0, 0, 0,
	); err != nil {
		slog.Warn("x11: send XdndLeave failed", "err", err)
	}
	if err := conn.Flush(); err != nil {
		slog.Warn("x11: flush after XdndLeave failed", "err", err)
	}
}

// waitForXdndFinished polls for XdndFinished within a timeout, handling
// SelectionRequest events that arrive before it. The target requests the
// drag data (via ConvertSelection) as soon as it receives XdndDrop, so the
// SelectionRequest lands inside this wait. Discarding it caused the target
// to receive no data and report "invalid drag type" (#431, @unxed).
func (ds *xdndDragState) waitForXdndFinished(p *x11.Platform, conn *x11.Connection, atoms *x11.XdndAtoms, timeout time.Duration) DragResult {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		event, err := conn.PollEventTimeout(50 * time.Millisecond)
		if err != nil || event == nil {
			continue
		}
		switch e := event.(type) {
		case *x11.SelectionRequestEvent:
			p.HandleDragSelectionRequest(e)

		case *x11.ClientMessageEvent:
			if e.Type != atoms.Finished {
				continue
			}
			data := e.Data32()
			if !ds.ownsSession(x11.ResourceID(data[0])) {
				continue
			}
			return dragResultFromFinished(atoms, data)
		}
	}
	slog.Warn("x11: XdndFinished timeout — target did not respond")
	return DragCancelled
}

// buildURIList converts file paths to a text/uri-list string (RFC 2483).
func buildURIList(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		u := url.URL{Scheme: "file", Path: p}
		fmt.Fprintf(&b, "%s\r\n", u.String())
	}
	return b.String()
}
