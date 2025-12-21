//go:build linux

package wayland

import (
	"bytes"
	"errors"
	"testing"
)

func TestFixedConversion(t *testing.T) {
	tests := []struct {
		name     string
		float    float64
		expected float64
	}{
		{"zero", 0.0, 0.0},
		{"positive integer", 42.0, 42.0},
		{"negative integer", -42.0, -42.0},
		{"positive fraction", 3.5, 3.5},
		{"negative fraction", -3.5, -3.5},
		{"small positive", 0.125, 0.125},
		{"small negative", -0.125, -0.125},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := FixedFromFloat(tt.float)
			got := fixed.Float()

			// Allow small epsilon for floating point comparison
			epsilon := 0.004 // 24.8 fixed has ~0.004 precision
			if diff := got - tt.expected; diff < -epsilon || diff > epsilon {
				t.Errorf("FixedFromFloat(%v).Float() = %v, want %v", tt.float, got, tt.expected)
			}
		})
	}
}

func TestFixedFromInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int32
		expected int32
	}{
		{"zero", 0, 0},
		{"positive", 42, 42},
		{"negative", -42, -42},
		{"max", 8388607, 8388607},   // Max 24-bit signed
		{"min", -8388608, -8388608}, // Min 24-bit signed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := FixedFromInt(tt.input)
			got := fixed.Int()
			if got != tt.expected {
				t.Errorf("FixedFromInt(%d).Int() = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEncoderInt32(t *testing.T) {
	enc := NewEncoder(16)
	enc.PutInt32(0x12345678)
	enc.PutInt32(-1)

	expected := []byte{
		0x78, 0x56, 0x34, 0x12, // 0x12345678 little-endian
		0xFF, 0xFF, 0xFF, 0xFF, // -1
	}

	if !bytes.Equal(enc.Bytes(), expected) {
		t.Errorf("Int32 encoding: got %x, want %x", enc.Bytes(), expected)
	}
}

func TestEncoderUint32(t *testing.T) {
	enc := NewEncoder(16)
	enc.PutUint32(0xDEADBEEF)
	enc.PutUint32(0)

	expected := []byte{
		0xEF, 0xBE, 0xAD, 0xDE, // 0xDEADBEEF little-endian
		0x00, 0x00, 0x00, 0x00, // 0
	}

	if !bytes.Equal(enc.Bytes(), expected) {
		t.Errorf("Uint32 encoding: got %x, want %x", enc.Bytes(), expected)
	}
}

func TestEncoderString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:  "empty",
			input: "",
			expected: []byte{
				0x01, 0x00, 0x00, 0x00, // length = 1 (just null terminator)
				0x00, 0x00, 0x00, 0x00, // null + padding
			},
		},
		{
			name:  "abc",
			input: "abc",
			expected: []byte{
				0x04, 0x00, 0x00, 0x00, // length = 4 (abc + null)
				0x61, 0x62, 0x63, 0x00, // "abc\0"
			},
		},
		{
			name:  "hello",
			input: "hello",
			expected: []byte{
				0x06, 0x00, 0x00, 0x00, // length = 6 (hello + null)
				0x68, 0x65, 0x6c, 0x6c, // "hell"
				0x6f, 0x00, 0x00, 0x00, // "o\0" + 2 padding
			},
		},
		{
			name:  "ab",
			input: "ab",
			expected: []byte{
				0x03, 0x00, 0x00, 0x00, // length = 3 (ab + null)
				0x61, 0x62, 0x00, 0x00, // "ab\0" + 1 padding
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewEncoder(32)
			enc.PutString(tt.input)

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("String encoding %q: got %x, want %x", tt.input, enc.Bytes(), tt.expected)
			}
		})
	}
}

func TestEncoderArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:  "empty",
			input: nil,
			expected: []byte{
				0x00, 0x00, 0x00, 0x00, // length = 0
			},
		},
		{
			name:  "4 bytes",
			input: []byte{0x01, 0x02, 0x03, 0x04},
			expected: []byte{
				0x04, 0x00, 0x00, 0x00, // length = 4
				0x01, 0x02, 0x03, 0x04, // data
			},
		},
		{
			name:  "5 bytes",
			input: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			expected: []byte{
				0x05, 0x00, 0x00, 0x00, // length = 5
				0x01, 0x02, 0x03, 0x04, // data
				0x05, 0x00, 0x00, 0x00, // data + 3 padding
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewEncoder(32)
			enc.PutArray(tt.input)

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("Array encoding: got %x, want %x", enc.Bytes(), tt.expected)
			}
		})
	}
}

func TestDecoderInt32(t *testing.T) {
	data := []byte{
		0x78, 0x56, 0x34, 0x12,
		0xFF, 0xFF, 0xFF, 0xFF,
	}

	dec := NewDecoder(data)

	v1, err := dec.Int32()
	if err != nil {
		t.Fatalf("Int32 decode error: %v", err)
	}
	if v1 != 0x12345678 {
		t.Errorf("Int32 decode: got %x, want %x", v1, 0x12345678)
	}

	v2, err := dec.Int32()
	if err != nil {
		t.Fatalf("Int32 decode error: %v", err)
	}
	if v2 != -1 {
		t.Errorf("Int32 decode: got %d, want -1", v2)
	}
}

func TestDecoderUint32(t *testing.T) {
	data := []byte{
		0xEF, 0xBE, 0xAD, 0xDE,
		0x00, 0x00, 0x00, 0x00,
	}

	dec := NewDecoder(data)

	v1, err := dec.Uint32()
	if err != nil {
		t.Fatalf("Uint32 decode error: %v", err)
	}
	if v1 != 0xDEADBEEF {
		t.Errorf("Uint32 decode: got %x, want %x", v1, 0xDEADBEEF)
	}

	v2, err := dec.Uint32()
	if err != nil {
		t.Fatalf("Uint32 decode error: %v", err)
	}
	if v2 != 0 {
		t.Errorf("Uint32 decode: got %d, want 0", v2)
	}
}

func TestDecoderString(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name: "empty",
			data: []byte{
				0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
			expected: "",
		},
		{
			name: "abc",
			data: []byte{
				0x04, 0x00, 0x00, 0x00,
				0x61, 0x62, 0x63, 0x00,
			},
			expected: "abc",
		},
		{
			name: "hello",
			data: []byte{
				0x06, 0x00, 0x00, 0x00,
				0x68, 0x65, 0x6c, 0x6c,
				0x6f, 0x00, 0x00, 0x00,
			},
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(tt.data)
			got, err := dec.String()
			if err != nil {
				t.Fatalf("String decode error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("String decode: got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDecoderArray(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []byte
	}{
		{
			name: "empty",
			data: []byte{
				0x00, 0x00, 0x00, 0x00,
			},
			expected: nil,
		},
		{
			name: "4 bytes",
			data: []byte{
				0x04, 0x00, 0x00, 0x00,
				0x01, 0x02, 0x03, 0x04,
			},
			expected: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name: "5 bytes with padding",
			data: []byte{
				0x05, 0x00, 0x00, 0x00,
				0x01, 0x02, 0x03, 0x04,
				0x05, 0x00, 0x00, 0x00,
			},
			expected: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(tt.data)
			got, err := dec.Array()
			if err != nil {
				t.Fatalf("Array decode error: %v", err)
			}
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("Array decode: got %x, want %x", got, tt.expected)
			}
		})
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	// Create a test message
	builder := NewMessageBuilder()
	builder.PutUint32(42)
	builder.PutString("test")
	builder.PutInt32(-100)

	msg := builder.BuildMessage(1, 0)

	// Encode
	encoded, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("EncodeMessage error: %v", err)
	}

	// Decode header
	dec := NewDecoder(encoded)
	objectID, opcode, size, err := dec.DecodeHeader()
	if err != nil {
		t.Fatalf("DecodeHeader error: %v", err)
	}

	if objectID != 1 {
		t.Errorf("ObjectID: got %d, want 1", objectID)
	}
	if opcode != 0 {
		t.Errorf("Opcode: got %d, want 0", opcode)
	}
	if size != len(encoded) {
		t.Errorf("Size: got %d, want %d", size, len(encoded))
	}

	// Decode full message
	dec2 := NewDecoder(encoded)
	decoded, err := dec2.DecodeMessage()
	if err != nil {
		t.Fatalf("DecodeMessage error: %v", err)
	}

	if decoded.ObjectID != msg.ObjectID {
		t.Errorf("ObjectID: got %d, want %d", decoded.ObjectID, msg.ObjectID)
	}
	if decoded.Opcode != msg.Opcode {
		t.Errorf("Opcode: got %d, want %d", decoded.Opcode, msg.Opcode)
	}
	if !bytes.Equal(decoded.Args, msg.Args) {
		t.Errorf("Args: got %x, want %x", decoded.Args, msg.Args)
	}
}

func TestMessageBuilder(t *testing.T) {
	builder := NewMessageBuilder()
	builder.PutUint32(1).
		PutInt32(-1).
		PutString("hello").
		PutFixed(FixedFromFloat(1.5)).
		PutObject(ObjectID(42)).
		PutNewID(ObjectID(100))

	args, fds := builder.Build()

	if len(fds) != 0 {
		t.Errorf("FDs: got %d, want 0", len(fds))
	}

	// Verify by decoding
	dec := NewDecoder(args)

	v1, _ := dec.Uint32()
	if v1 != 1 {
		t.Errorf("Uint32: got %d, want 1", v1)
	}

	v2, _ := dec.Int32()
	if v2 != -1 {
		t.Errorf("Int32: got %d, want -1", v2)
	}

	v3, _ := dec.String()
	if v3 != "hello" {
		t.Errorf("String: got %q, want %q", v3, "hello")
	}

	v4, _ := dec.Fixed()
	if diff := v4.Float() - 1.5; diff < -0.01 || diff > 0.01 {
		t.Errorf("Fixed: got %f, want 1.5", v4.Float())
	}

	v5, _ := dec.Object()
	if v5 != 42 {
		t.Errorf("Object: got %d, want 42", v5)
	}

	v6, _ := dec.NewID()
	if v6 != 100 {
		t.Errorf("NewID: got %d, want 100", v6)
	}
}

func TestDecoderErrors(t *testing.T) {
	t.Run("Int32 EOF", func(t *testing.T) {
		dec := NewDecoder([]byte{0x01, 0x02}) // Only 2 bytes
		_, err := dec.Int32()
		if !errors.Is(err, ErrUnexpectedEOF) {
			t.Errorf("Expected ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("String EOF", func(t *testing.T) {
		dec := NewDecoder([]byte{
			0x10, 0x00, 0x00, 0x00, // length = 16
			0x61, 0x62, 0x63, // only 3 bytes of data
		})
		_, err := dec.String()
		if !errors.Is(err, ErrUnexpectedEOF) {
			t.Errorf("Expected ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("Array EOF", func(t *testing.T) {
		dec := NewDecoder([]byte{
			0x10, 0x00, 0x00, 0x00, // length = 16
			0x01, 0x02, 0x03, 0x04, // only 4 bytes of data
		})
		_, err := dec.Array()
		if !errors.Is(err, ErrUnexpectedEOF) {
			t.Errorf("Expected ErrUnexpectedEOF, got %v", err)
		}
	})
}

func TestPaddingFor(t *testing.T) {
	tests := []struct {
		length   int
		expected int
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
		got := paddingFor(tt.length)
		if got != tt.expected {
			t.Errorf("paddingFor(%d) = %d, want %d", tt.length, got, tt.expected)
		}
	}
}

func TestMessageSize(t *testing.T) {
	msg := &Message{
		ObjectID: 1,
		Opcode:   0,
		Args:     []byte{0x01, 0x02, 0x03, 0x04},
	}

	if msg.Size() != 12 { // 8 header + 4 args
		t.Errorf("Message.Size() = %d, want 12", msg.Size())
	}
}

func TestEncoderReset(t *testing.T) {
	enc := NewEncoder(16)
	enc.PutUint32(123)

	if len(enc.Bytes()) != 4 {
		t.Errorf("Before reset: len = %d, want 4", len(enc.Bytes()))
	}

	enc.Reset()

	if len(enc.Bytes()) != 0 {
		t.Errorf("After reset: len = %d, want 0", len(enc.Bytes()))
	}
}

func TestDecoderRemaining(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	dec := NewDecoder(data)

	if dec.Remaining() != 8 {
		t.Errorf("Initial remaining: got %d, want 8", dec.Remaining())
	}

	_, _ = dec.Uint32()

	if dec.Remaining() != 4 {
		t.Errorf("After Uint32: got %d, want 4", dec.Remaining())
	}

	if !dec.HasMore() {
		t.Error("HasMore should be true")
	}

	_, _ = dec.Uint32()

	if dec.HasMore() {
		t.Error("HasMore should be false")
	}
}

func TestFloatToFixedClamping(t *testing.T) {
	// Test values that would overflow
	large := FloatToFixed(1e10)
	if large.Float() > 8388607 {
		t.Errorf("Large value not clamped properly: %f", large.Float())
	}

	small := FloatToFixed(-1e10)
	if small.Float() < -8388608 {
		t.Errorf("Small value not clamped properly: %f", small.Float())
	}
}

func TestNewIDFull(t *testing.T) {
	enc := NewEncoder(64)
	enc.PutNewIDFull("wl_compositor", 4, ObjectID(2))

	// Decode and verify
	dec := NewDecoder(enc.Bytes())

	iface, err := dec.String()
	if err != nil {
		t.Fatalf("String decode error: %v", err)
	}
	if iface != "wl_compositor" {
		t.Errorf("Interface: got %q, want %q", iface, "wl_compositor")
	}

	version, err := dec.Uint32()
	if err != nil {
		t.Fatalf("Uint32 decode error: %v", err)
	}
	if version != 4 {
		t.Errorf("Version: got %d, want 4", version)
	}

	id, err := dec.Uint32()
	if err != nil {
		t.Fatalf("Uint32 decode error: %v", err)
	}
	if id != 2 {
		t.Errorf("ID: got %d, want 2", id)
	}
}

func TestDecodeHeaderErrors(t *testing.T) {
	t.Run("too small", func(t *testing.T) {
		dec := NewDecoder([]byte{0x01, 0x02, 0x03}) // Less than 8 bytes
		_, _, _, err := dec.DecodeHeader()
		if !errors.Is(err, ErrMessageTooSmall) {
			t.Errorf("Expected ErrMessageTooSmall, got %v", err)
		}
	})

	t.Run("invalid size in header", func(t *testing.T) {
		// Header with size < 8
		data := []byte{
			0x01, 0x00, 0x00, 0x00, // object ID
			0x00, 0x04, 0x00, 0x00, // size=4 (invalid), opcode=0
		}
		dec := NewDecoder(data)
		_, _, _, err := dec.DecodeHeader()
		if !errors.Is(err, ErrMessageTooSmall) {
			t.Errorf("Expected ErrMessageTooSmall, got %v", err)
		}
	})
}

func BenchmarkEncoderString(b *testing.B) {
	enc := NewEncoder(256)
	s := "wl_compositor"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Reset()
		enc.PutString(s)
	}
}

func BenchmarkDecoderString(b *testing.B) {
	data := []byte{
		0x0e, 0x00, 0x00, 0x00, // length = 14
		0x77, 0x6c, 0x5f, 0x63, // "wl_c"
		0x6f, 0x6d, 0x70, 0x6f, // "ompo"
		0x73, 0x69, 0x74, 0x6f, // "sito"
		0x72, 0x00, 0x00, 0x00, // "r\0" + padding
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := NewDecoder(data)
		_, _ = dec.String()
	}
}

func BenchmarkMessageEncode(b *testing.B) {
	builder := NewMessageBuilder()
	builder.PutUint32(42)
	builder.PutString("wl_compositor")
	builder.PutUint32(4)
	msg := builder.BuildMessage(1, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeMessage(msg)
	}
}
