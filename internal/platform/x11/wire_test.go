//go:build linux

package x11

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncoder_PutUint8(t *testing.T) {
	e := NewEncoder(LSBFirst)
	e.PutUint8(0x42)
	e.PutUint8(0xFF)

	got := e.Bytes()
	want := []byte{0x42, 0xFF}

	if !bytes.Equal(got, want) {
		t.Errorf("PutUint8: got %v, want %v", got, want)
	}
}

func TestEncoder_PutUint16_LittleEndian(t *testing.T) {
	e := NewEncoder(LSBFirst)
	e.PutUint16(0x1234)

	got := e.Bytes()
	want := []byte{0x34, 0x12} // Little-endian

	if !bytes.Equal(got, want) {
		t.Errorf("PutUint16 (LE): got %v, want %v", got, want)
	}
}

func TestEncoder_PutUint16_BigEndian(t *testing.T) {
	e := NewEncoder(MSBFirst)
	e.PutUint16(0x1234)

	got := e.Bytes()
	want := []byte{0x12, 0x34} // Big-endian

	if !bytes.Equal(got, want) {
		t.Errorf("PutUint16 (BE): got %v, want %v", got, want)
	}
}

func TestEncoder_PutUint32_LittleEndian(t *testing.T) {
	e := NewEncoder(LSBFirst)
	e.PutUint32(0x12345678)

	got := e.Bytes()
	want := []byte{0x78, 0x56, 0x34, 0x12} // Little-endian

	if !bytes.Equal(got, want) {
		t.Errorf("PutUint32 (LE): got %v, want %v", got, want)
	}
}

func TestEncoder_PutUint32_BigEndian(t *testing.T) {
	e := NewEncoder(MSBFirst)
	e.PutUint32(0x12345678)

	got := e.Bytes()
	want := []byte{0x12, 0x34, 0x56, 0x78} // Big-endian

	if !bytes.Equal(got, want) {
		t.Errorf("PutUint32 (BE): got %v, want %v", got, want)
	}
}

func TestEncoder_PutPad(t *testing.T) {
	tests := []struct {
		name    string
		initial int
		wantPad int
	}{
		{"aligned 0", 0, 0},
		{"aligned 4", 4, 0},
		{"aligned 8", 8, 0},
		{"needs 3", 1, 3},
		{"needs 2", 2, 2},
		{"needs 1", 3, 1},
		{"needs 3 from 5", 5, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEncoder(LSBFirst)
			for i := 0; i < tt.initial; i++ {
				e.PutUint8(0xFF)
			}
			e.PutPad()

			got := e.Len()
			want := tt.initial + tt.wantPad

			if got != want {
				t.Errorf("PutPad after %d bytes: got len %d, want %d", tt.initial, got, want)
			}
		})
	}
}

func TestEncoder_Reset(t *testing.T) {
	e := NewEncoder(LSBFirst)
	e.PutUint32(0x12345678)

	if e.Len() != 4 {
		t.Errorf("before Reset: got len %d, want 4", e.Len())
	}

	e.Reset()

	if e.Len() != 0 {
		t.Errorf("after Reset: got len %d, want 0", e.Len())
	}
}

func TestDecoder_Uint8(t *testing.T) {
	data := []byte{0x42, 0xFF}
	d := NewDecoder(LSBFirst, data)

	v1, err := d.Uint8()
	if err != nil {
		t.Fatalf("Uint8 (1): unexpected error: %v", err)
	}
	if v1 != 0x42 {
		t.Errorf("Uint8 (1): got %x, want %x", v1, 0x42)
	}

	v2, err := d.Uint8()
	if err != nil {
		t.Fatalf("Uint8 (2): unexpected error: %v", err)
	}
	if v2 != 0xFF {
		t.Errorf("Uint8 (2): got %x, want %x", v2, 0xFF)
	}
}

func TestDecoder_Uint16_LittleEndian(t *testing.T) {
	data := []byte{0x34, 0x12}
	d := NewDecoder(LSBFirst, data)

	v, err := d.Uint16()
	if err != nil {
		t.Fatalf("Uint16: unexpected error: %v", err)
	}
	if v != 0x1234 {
		t.Errorf("Uint16 (LE): got %x, want %x", v, 0x1234)
	}
}

func TestDecoder_Uint16_BigEndian(t *testing.T) {
	data := []byte{0x12, 0x34}
	d := NewDecoder(MSBFirst, data)

	v, err := d.Uint16()
	if err != nil {
		t.Fatalf("Uint16: unexpected error: %v", err)
	}
	if v != 0x1234 {
		t.Errorf("Uint16 (BE): got %x, want %x", v, 0x1234)
	}
}

func TestDecoder_Uint32_LittleEndian(t *testing.T) {
	data := []byte{0x78, 0x56, 0x34, 0x12}
	d := NewDecoder(LSBFirst, data)

	v, err := d.Uint32()
	if err != nil {
		t.Fatalf("Uint32: unexpected error: %v", err)
	}
	if v != 0x12345678 {
		t.Errorf("Uint32 (LE): got %x, want %x", v, 0x12345678)
	}
}

func TestDecoder_Uint32_BigEndian(t *testing.T) {
	data := []byte{0x12, 0x34, 0x56, 0x78}
	d := NewDecoder(MSBFirst, data)

	v, err := d.Uint32()
	if err != nil {
		t.Fatalf("Uint32: unexpected error: %v", err)
	}
	if v != 0x12345678 {
		t.Errorf("Uint32 (BE): got %x, want %x", v, 0x12345678)
	}
}

func TestDecoder_UnexpectedEOF(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		read func(*Decoder) error
	}{
		{"Uint8 empty", []byte{}, func(d *Decoder) error { _, err := d.Uint8(); return err }},
		{"Uint16 short", []byte{0x00}, func(d *Decoder) error { _, err := d.Uint16(); return err }},
		{"Uint32 short", []byte{0x00, 0x00, 0x00}, func(d *Decoder) error { _, err := d.Uint32(); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDecoder(LSBFirst, tt.data)
			err := tt.read(d)
			if !errors.Is(err, ErrUnexpectedEOF) {
				t.Errorf("%s: got error %v, want ErrUnexpectedEOF", tt.name, err)
			}
		})
	}
}

func TestDecoder_Skip(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	d := NewDecoder(LSBFirst, data)

	if err := d.Skip(2); err != nil {
		t.Fatalf("Skip: unexpected error: %v", err)
	}

	if d.Offset() != 2 {
		t.Errorf("Offset after Skip: got %d, want 2", d.Offset())
	}

	v, err := d.Uint8()
	if err != nil {
		t.Fatalf("Uint8 after Skip: unexpected error: %v", err)
	}
	if v != 0x03 {
		t.Errorf("Uint8 after Skip: got %x, want %x", v, 0x03)
	}
}

func TestDecoder_Skip_Error(t *testing.T) {
	data := []byte{0x01, 0x02}
	d := NewDecoder(LSBFirst, data)

	err := d.Skip(5)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Errorf("Skip beyond buffer: got error %v, want ErrUnexpectedEOF", err)
	}
}

func TestDecoder_Bytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	d := NewDecoder(LSBFirst, data)

	got, err := d.Bytes(3)
	if err != nil {
		t.Fatalf("Bytes: unexpected error: %v", err)
	}

	want := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(got, want) {
		t.Errorf("Bytes: got %v, want %v", got, want)
	}

	if d.Remaining() != 2 {
		t.Errorf("Remaining after Bytes: got %d, want 2", d.Remaining())
	}
}

func TestDecoder_String(t *testing.T) {
	data := []byte("hello")
	d := NewDecoder(LSBFirst, data)

	got, err := d.String(5)
	if err != nil {
		t.Fatalf("String: unexpected error: %v", err)
	}

	if got != "hello" {
		t.Errorf("String: got %q, want %q", got, "hello")
	}
}

func TestPad(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 3},
		{2, 2},
		{3, 1},
		{4, 0},
		{5, 3},
		{6, 2},
		{7, 1},
		{8, 0},
	}

	for _, tt := range tests {
		got := pad(tt.n)
		if got != tt.want {
			t.Errorf("pad(%d): got %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestRequestLength(t *testing.T) {
	tests := []struct {
		dataLen int
		want    uint16
	}{
		{0, 0},
		{1, 1},
		{4, 1},
		{5, 2},
		{8, 2},
		{9, 3},
		{12, 3},
	}

	for _, tt := range tests {
		got := requestLength(tt.dataLen)
		if got != tt.want {
			t.Errorf("requestLength(%d): got %d, want %d", tt.dataLen, got, tt.want)
		}
	}
}

func TestEncoderDecoder_Roundtrip(t *testing.T) {
	// Test encoding then decoding produces same values
	tests := []struct {
		name  string
		order ByteOrder
	}{
		{"LittleEndian", LSBFirst},
		{"BigEndian", MSBFirst},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEncoder(tt.order)

			// Encode various values
			e.PutUint8(0x42)
			e.PutUint16(0x1234)
			e.PutUint32(0xDEADBEEF)
			e.PutInt16(-1234)
			e.PutInt32(-0x12345678)

			data := e.Bytes()
			d := NewDecoder(tt.order, data)

			// Decode and verify
			v1, _ := d.Uint8()
			if v1 != 0x42 {
				t.Errorf("Uint8: got %x, want %x", v1, 0x42)
			}

			v2, _ := d.Uint16()
			if v2 != 0x1234 {
				t.Errorf("Uint16: got %x, want %x", v2, 0x1234)
			}

			v3, _ := d.Uint32()
			if v3 != 0xDEADBEEF {
				t.Errorf("Uint32: got %x, want %x", v3, 0xDEADBEEF)
			}

			v4, _ := d.Int16()
			if v4 != -1234 {
				t.Errorf("Int16: got %d, want %d", v4, -1234)
			}

			v5, _ := d.Int32()
			if v5 != -0x12345678 {
				t.Errorf("Int32: got %d, want %d", v5, -0x12345678)
			}
		})
	}
}
