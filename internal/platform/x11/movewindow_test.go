//go:build linux

package x11

import (
	"bytes"
	"testing"
)

// TestMoveWindowRequest_LittleEndian verifies the ConfigureWindow wire
// encoding for a position-only move on a little-endian connection.
func TestMoveWindowRequest_LittleEndian(t *testing.T) {
	got := moveWindowRequest(LSBFirst, 0x11223344, 0x0012, -2)
	want := []byte{
		12, 0, 5, 0, // opcode, unused, length
		0x44, 0x33, 0x22, 0x11, // window
		0x03, 0x00, // value mask: X | Y
		0x00, 0x00, // unused
		0x12, 0x00, 0x00, 0x00, // x = 18
		0xFE, 0xFF, 0xFF, 0xFF, // y = -2
	}
	if !bytes.Equal(got, want) {
		t.Errorf("little-endian request = %v, want %v", got, want)
	}
}

// TestMoveWindowRequest_BigEndian verifies the ConfigureWindow wire
// encoding for a position-only move on a big-endian connection.
func TestMoveWindowRequest_BigEndian(t *testing.T) {
	got := moveWindowRequest(MSBFirst, 0x11223344, 0x0012, -2)
	want := []byte{
		12, 0, 0, 5, // opcode, unused, length
		0x11, 0x22, 0x33, 0x44, // window
		0x00, 0x03, // value mask: X | Y
		0x00, 0x00, // unused
		0x00, 0x00, 0x00, 0x12, // x = 18
		0xFF, 0xFF, 0xFF, 0xFE, // y = -2
	}
	if !bytes.Equal(got, want) {
		t.Errorf("big-endian request = %v, want %v", got, want)
	}
}
