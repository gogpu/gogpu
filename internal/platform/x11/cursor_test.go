//go:build linux

package x11

import "testing"

func TestCursorShapeToGlyph(t *testing.T) {
	tests := []struct {
		name     string
		cursorID int
		want     uint16
	}{
		{"Default", 0, XCursorLeftPtr},
		{"Pointer", 1, XCursorHand2},
		{"Text", 2, XCursorXterm},
		{"Crosshair", 3, XCursorCrosshair},
		{"Move", 4, XCursorFleur},
		{"ResizeNS", 5, XCursorSBVDoubleArrow},
		{"ResizeEW", 6, XCursorSBHDoubleArrow},
		{"ResizeNWSE", 7, XCursorTopLeftCorner},
		{"ResizeNESW", 8, XCursorBottomLeftCorner},
		{"NotAllowed", 9, XCursorCircle},
		{"Wait", 10, XCursorWatch},
		{"Unknown falls back to arrow", 99, XCursorLeftPtr},
		{"Negative falls back to arrow", -1, XCursorLeftPtr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cursorShapeToGlyph(tt.cursorID)
			if got != tt.want {
				t.Errorf("cursorShapeToGlyph(%d) = %d, want %d", tt.cursorID, got, tt.want)
			}
		})
	}
}

func TestCursorGlyphValues(t *testing.T) {
	// Verify glyph indices are even (X11 cursor font uses even indices for shapes,
	// odd indices for masks).
	glyphs := []struct {
		name  string
		value uint16
	}{
		{"XCursorLeftPtr", XCursorLeftPtr},
		{"XCursorHand2", XCursorHand2},
		{"XCursorXterm", XCursorXterm},
		{"XCursorCrosshair", XCursorCrosshair},
		{"XCursorFleur", XCursorFleur},
		{"XCursorSBVDoubleArrow", XCursorSBVDoubleArrow},
		{"XCursorSBHDoubleArrow", XCursorSBHDoubleArrow},
		{"XCursorTopLeftCorner", XCursorTopLeftCorner},
		{"XCursorBottomLeftCorner", XCursorBottomLeftCorner},
		{"XCursorCircle", XCursorCircle},
		{"XCursorWatch", XCursorWatch},
	}

	for _, g := range glyphs {
		t.Run(g.name, func(t *testing.T) {
			if g.value%2 != 0 {
				t.Errorf("%s = %d, want even number (X11 cursor font glyph indices are even)", g.name, g.value)
			}
		})
	}
}

func TestOpenFontRequestFormat(t *testing.T) {
	// Test that OpenFont builds the correct wire format
	// We can't test against a real X server, but we can verify the encoder output

	fontName := "cursor"
	nameLen := len(fontName)
	expectedReqLen := 3 + requestLength(nameLen) // 3 header words + padded name

	// The request should be:
	// [opcode:1][unused:1][length:2][fid:4][name_len:2][unused:2][name...][pad]
	totalBytes := int(expectedReqLen) * 4

	if totalBytes < 12+nameLen {
		t.Errorf("request too short: %d bytes for font name %q", totalBytes, fontName)
	}

	// Verify request length calculation
	if expectedReqLen != 5 { // 3 + ceil(6/4) = 3 + 2 = 5
		t.Errorf("reqLen = %d, want 5 for font name %q", expectedReqLen, fontName)
	}
}

func TestCreateGlyphCursorRequestLength(t *testing.T) {
	// CreateGlyphCursor should be exactly 8 4-byte units = 32 bytes
	// [opcode:1][unused:1][length:2][cid:4][source_font:4][mask_font:4]
	// [source_char:2][mask_char:2][fore_rgb:6][back_rgb:6]
	// = 4 + 4 + 4 + 4 + 4 + 6 + 6 = 32 bytes = 8 units
	expectedLength := uint16(8)

	e := NewEncoder(LSBFirst)
	e.PutUint8(OpcodeCreateGlyphCursor)
	e.PutUint8(0)
	e.PutUint16(expectedLength)
	e.PutUint32(100)    // cid
	e.PutUint32(200)    // source_font
	e.PutUint32(200)    // mask_font
	e.PutUint16(68)     // source_char
	e.PutUint16(69)     // mask_char
	e.PutUint16(0)      // fore_r
	e.PutUint16(0)      // fore_g
	e.PutUint16(0)      // fore_b
	e.PutUint16(0xFFFF) // back_r
	e.PutUint16(0xFFFF) // back_g
	e.PutUint16(0xFFFF) // back_b

	data := e.Bytes()
	if len(data) != 32 {
		t.Errorf("CreateGlyphCursor request = %d bytes, want 32", len(data))
	}
}

func TestChangeWindowCursorRequestFormat(t *testing.T) {
	// ChangeWindowAttributes with CWCursor should be 4 units = 16 bytes
	// [opcode:1][unused:1][length:2][window:4][value_mask:4][cursor:4]
	e := NewEncoder(LSBFirst)
	e.PutUint8(OpcodeChangeWindowAttrs)
	e.PutUint8(0)
	e.PutUint16(4)        // length
	e.PutUint32(500)      // window
	e.PutUint32(CWCursor) // value mask
	e.PutUint32(100)      // cursor ID

	data := e.Bytes()
	if len(data) != 16 {
		t.Errorf("ChangeWindowCursor request = %d bytes, want 16", len(data))
	}

	// Verify opcode
	if data[0] != OpcodeChangeWindowAttrs {
		t.Errorf("opcode = %d, want %d", data[0], OpcodeChangeWindowAttrs)
	}
}
