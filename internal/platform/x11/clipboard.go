//go:build linux

package x11

import (
	"fmt"
	"time"
)

// ClipboardWrite stores text in the system clipboard by taking ownership
// of the CLIPBOARD and PRIMARY selections via the ICCCM protocol.
// Other applications receive the text when they request the selection.
func (p *Platform) ClipboardWrite(text string) error {
	if p.conn == nil || p.primary == nil {
		return fmt.Errorf("x11: clipboard write before init")
	}

	window := p.primary.window

	p.clipboardMu.Lock()
	p.clipboardText = text
	p.ownsClipboard = true
	p.clipboardMu.Unlock()

	// Take ownership of CLIPBOARD selection
	if err := p.conn.SetSelectionOwner(p.atoms.Clipboard, window, 0); err != nil {
		return fmt.Errorf("x11: clipboard SetSelectionOwner: %w", err)
	}

	// Verify we got ownership
	owner, err := p.conn.GetSelectionOwner(p.atoms.Clipboard)
	if err != nil {
		return fmt.Errorf("x11: clipboard GetSelectionOwner: %w", err)
	}
	if owner != window {
		p.clipboardMu.Lock()
		p.ownsClipboard = false
		p.clipboardMu.Unlock()
		return fmt.Errorf("x11: failed to acquire CLIPBOARD ownership")
	}

	// Also take PRIMARY for middle-click paste (best effort)
	_ = p.conn.SetSelectionOwner(AtomPrimary, window, 0)

	if err := p.conn.Flush(); err != nil {
		return fmt.Errorf("x11: clipboard flush: %w", err)
	}

	return nil
}

// ClipboardRead reads text from the system clipboard. If we own the clipboard,
// returns our stored text immediately. Otherwise, requests conversion from the
// current owner via the ICCCM selection protocol with a 1-second timeout.
func (p *Platform) ClipboardRead() (string, error) {
	if p.conn == nil || p.primary == nil {
		return "", fmt.Errorf("x11: clipboard read before init")
	}

	// Fast path: if we own the clipboard, return our stored text directly.
	// This avoids a round-trip and potential deadlock (requesting from ourselves).
	p.clipboardMu.Lock()
	if p.ownsClipboard {
		text := p.clipboardText
		p.clipboardMu.Unlock()
		return text, nil
	}
	p.clipboardReady = false
	p.clipboardMu.Unlock()

	window := p.primary.window

	// Request the CLIPBOARD owner to convert the selection to UTF8_STRING.
	// The owner will place the data in our GOGPU_SELECTION property and
	// send us a SelectionNotify event.
	if err := p.conn.ConvertSelection(
		window,
		p.atoms.Clipboard,
		p.atoms.UTF8String,
		p.atoms.GogpuSelection,
		0, // CurrentTime
	); err != nil {
		return "", fmt.Errorf("x11: clipboard ConvertSelection: %w", err)
	}

	if err := p.conn.Flush(); err != nil {
		return "", fmt.Errorf("x11: clipboard flush: %w", err)
	}

	// Pump events until we get SelectionNotify or timeout (1 second).
	// SDL3 uses the same 1-second timeout with event pumping.
	deadline := time.Now().Add(time.Second)
	for {
		p.clipboardMu.Lock()
		ready := p.clipboardReady
		p.clipboardMu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("x11: clipboard read timeout (no SelectionNotify within 1s)")
		}

		// Poll with a short timeout to avoid busy-waiting
		event, err := p.conn.PollEventTimeout(50 * time.Millisecond)
		if err != nil {
			return "", fmt.Errorf("x11: clipboard poll error: %w", err)
		}
		if event == nil {
			continue
		}

		// Check if this is our SelectionNotify
		if notify, ok := event.(*SelectionNotifyEvent); ok {
			if notify.Selection == p.atoms.Clipboard && notify.Requestor == window {
				p.clipboardMu.Lock()
				p.clipboardReady = true
				p.clipboardMu.Unlock()

				// Property is AtomNone if conversion was refused
				if notify.Property == AtomNone {
					return "", nil
				}
				break
			}
		}

		// Process other events normally — queue them for PollEvents to deliver.
		if pe := p.handleEvent(event); pe.Type != EventTypeNone {
			p.primary.queueEvent(pe)
		}
	}

	// Read the converted text from our property
	data, _, _, err := p.conn.GetProperty(
		window,
		p.atoms.GogpuSelection,
		p.atoms.UTF8String,
		0,     // offset
		65536, // max length (256KB)
		true,  // delete after read
	)
	if err != nil {
		return "", fmt.Errorf("x11: clipboard GetProperty: %w", err)
	}

	return string(data), nil
}

// handleSelectionClear is called when another application takes ownership
// of a selection we owned. Clears our ownership flag.
func (p *Platform) handleSelectionClear(e *SelectionClearEvent) {
	if e.Selection == p.atoms.Clipboard {
		p.clipboardMu.Lock()
		p.ownsClipboard = false
		p.clipboardMu.Unlock()
	}
}

// handleSelectionRequest is called when another application requests our
// clipboard content. We respond by writing the data to the requestor's
// property and sending a SelectionNotify event.
func (p *Platform) handleSelectionRequest(e *SelectionRequestEvent) {
	// Determine the property to write to (use target as fallback if None)
	property := e.Property
	if property == AtomNone {
		property = e.Target
	}

	responded := false

	p.clipboardMu.Lock()
	text := p.clipboardText
	owns := p.ownsClipboard
	p.clipboardMu.Unlock()

	if owns {
		switch e.Target {
		case p.atoms.Targets:
			// Respond with list of supported targets
			responded = p.sendTargetsList(e.Requestor, property)

		case p.atoms.UTF8String:
			// Respond with clipboard text as UTF-8
			err := p.conn.ChangeProperty(
				e.Requestor,
				property,
				p.atoms.UTF8String,
				8, // format: 8-bit bytes
				PropModeReplace,
				[]byte(text),
			)
			if err == nil {
				responded = true
			}
		}
	}

	// Send SelectionNotify to the requestor
	p.sendSelectionNotify(e, property, responded)
}

// sendTargetsList writes the TARGETS atom list to the requestor's property.
func (p *Platform) sendTargetsList(requestor ResourceID, property Atom) bool {
	// Two targets: TARGETS itself and UTF8_STRING
	targets := make([]byte, 8)
	putUint32LE(targets[0:4], uint32(p.atoms.Targets))
	putUint32LE(targets[4:8], uint32(p.atoms.UTF8String))

	err := p.conn.ChangeProperty(
		requestor,
		property,
		AtomAtom, // type = ATOM
		32,       // format: 32-bit atoms
		PropModeReplace,
		targets,
	)
	return err == nil
}

// sendSelectionNotify sends a SelectionNotify event back to the requestor.
// If success is false, the property field is set to None (conversion refused).
func (p *Platform) sendSelectionNotify(req *SelectionRequestEvent, property Atom, success bool) {
	notifyProperty := property
	if !success {
		notifyProperty = AtomNone
	}

	// Build the 32-byte SelectionNotify event:
	// [1:type=31][1:unused][2:seq=0][4:time][4:requestor][4:selection][4:target][4:property][8:unused]
	eventData := make([]byte, 32)
	eventData[0] = EventSelectionNotify // type 31
	eventData[1] = 0                    // unused
	// seq (2 bytes) = 0
	putUint32LE(eventData[4:8], uint32(req.Time))
	putUint32LE(eventData[8:12], uint32(req.Requestor))
	putUint32LE(eventData[12:16], uint32(req.Selection))
	putUint32LE(eventData[16:20], uint32(req.Target))
	putUint32LE(eventData[20:24], uint32(notifyProperty))
	// remaining 8 bytes are zero (unused)

	// Send to the requestor (no propagation, no event mask filtering)
	_ = p.conn.SendEvent(req.Requestor, false, 0, eventData)
	_ = p.conn.Flush()
}

// putUint32LE writes a uint32 in little-endian to a byte slice.
func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
