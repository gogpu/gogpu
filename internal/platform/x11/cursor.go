//go:build linux

package x11

import "fmt"

// Standard X11 cursor font glyph indices.
// These are glyph indices in the "cursor" font, defined in X11/cursorfont.h.
// Each cursor shape occupies two consecutive glyphs: the shape and its mask.
const (
	XCursorLeftPtr          uint16 = 68  // Standard left-pointing arrow
	XCursorHand2            uint16 = 60  // Hand/pointer cursor
	XCursorXterm            uint16 = 152 // I-beam text cursor
	XCursorCrosshair        uint16 = 34  // Crosshair
	XCursorFleur            uint16 = 52  // Four-way move
	XCursorSBVDoubleArrow   uint16 = 116 // Vertical double arrow (NS resize)
	XCursorSBHDoubleArrow   uint16 = 108 // Horizontal double arrow (EW resize)
	XCursorTopLeftCorner    uint16 = 134 // Top-left corner (NWSE resize)
	XCursorBottomLeftCorner uint16 = 12  // Bottom-left corner (NESW resize)
	XCursorCircle           uint16 = 24  // Circle (not-allowed, closest match)
	XCursorWatch            uint16 = 150 // Watch/hourglass (wait)
)

// OpenFont opens a font by name and returns a font ID.
// X11 opcode 45: OpenFont(fid: FONT, name: STRING8).
func (c *Connection) OpenFont(name string) (ResourceID, error) {
	fontID := c.GenerateID()
	nameLen := len(name)
	reqLen := 3 + requestLength(nameLen)

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeOpenFont)
	e.PutUint8(0) // unused
	e.PutUint16(reqLen)
	e.PutUint32(uint32(fontID))
	e.PutUint16(uint16(nameLen))
	e.PutUint16(0) // unused
	e.PutBytes([]byte(name))
	e.PutPad()

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return 0, fmt.Errorf("x11: OpenFont failed: %w", err)
	}
	return fontID, nil
}

// CloseFont closes a font.
// X11 opcode 46: CloseFont(font: FONT).
func (c *Connection) CloseFont(fontID ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeCloseFont)
	e.PutUint8(0)  // unused
	e.PutUint16(2) // length
	e.PutUint32(uint32(fontID))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: CloseFont failed: %w", err)
	}
	return nil
}

// CreateGlyphCursor creates a cursor from font glyphs.
// X11 opcode 94: CreateGlyphCursor(cid, source_font, mask_font,
//
//	source_char, mask_char, fore_r, fore_g, fore_b, back_r, back_g, back_b).
//
// The source font glyph defines the shape, mask font glyph defines the mask.
// For the standard cursor font, mask_char = source_char + 1.
func (c *Connection) CreateGlyphCursor(sourceFont, maskFont ResourceID, sourceChar, maskChar uint16,
	foreR, foreG, foreB, backR, backG, backB uint16) (ResourceID, error) {
	cursorID := c.GenerateID()

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeCreateGlyphCursor)
	e.PutUint8(0)  // unused
	e.PutUint16(8) // length = 8 4-byte units
	e.PutUint32(uint32(cursorID))
	e.PutUint32(uint32(sourceFont))
	e.PutUint32(uint32(maskFont))
	e.PutUint16(sourceChar)
	e.PutUint16(maskChar)
	e.PutUint16(foreR)
	e.PutUint16(foreG)
	e.PutUint16(foreB)
	e.PutUint16(backR)
	e.PutUint16(backG)
	e.PutUint16(backB)

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return 0, fmt.Errorf("x11: CreateGlyphCursor failed: %w", err)
	}
	return cursorID, nil
}

// FreeCursor frees a cursor resource.
// X11 opcode 95: FreeCursor(cursor: CURSOR).
func (c *Connection) FreeCursor(cursorID ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeFreeCursor)
	e.PutUint8(0)  // unused
	e.PutUint16(2) // length
	e.PutUint32(uint32(cursorID))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: FreeCursor failed: %w", err)
	}
	return nil
}

// ChangeWindowCursor changes the cursor attribute of a window.
// X11 opcode 2: ChangeWindowAttributes with CWCursor value mask.
// Pass cursorID=0 to revert to parent window's cursor.
func (c *Connection) ChangeWindowCursor(window, cursorID ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeChangeWindowAttrs)
	e.PutUint8(0)  // unused
	e.PutUint16(4) // length = 3 header + 1 value
	e.PutUint32(uint32(window))
	e.PutUint32(CWCursor)
	e.PutUint32(uint32(cursorID))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: ChangeWindowAttributes(cursor) failed: %w", err)
	}
	return nil
}
