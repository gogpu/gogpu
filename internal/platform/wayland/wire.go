//go:build linux

package wayland

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ObjectID represents a Wayland object identifier.
// Object ID 0 is null/invalid. Object ID 1 is always wl_display.
type ObjectID uint32

// Opcode represents a Wayland request or event opcode.
type Opcode uint16

// Fixed represents a Wayland fixed-point number (24.8 format).
// The upper 24 bits are the integer part, lower 8 bits are the fractional part.
type Fixed int32

// FixedFromFloat converts a float64 to Fixed (24.8 format).
func FixedFromFloat(f float64) Fixed {
	return Fixed(f * 256.0)
}

// Float returns the Fixed value as a float64.
func (f Fixed) Float() float64 {
	return float64(f) / 256.0
}

// FixedFromInt converts an integer to Fixed.
func FixedFromInt(i int32) Fixed {
	return Fixed(i << 8)
}

// Int returns the integer part of the Fixed value.
func (f Fixed) Int() int32 {
	return int32(f) >> 8
}

// Header size in bytes (object ID + size/opcode).
const headerSize = 8

// Maximum message size (64KB as per Wayland spec).
const maxMessageSize = 64 * 1024

// Errors returned by the wire protocol.
var (
	ErrMessageTooLarge    = errors.New("wayland: message exceeds maximum size")
	ErrMessageTooSmall    = errors.New("wayland: message smaller than header")
	ErrBufferTooSmall     = errors.New("wayland: buffer too small for message")
	ErrInvalidStringLen   = errors.New("wayland: invalid string length")
	ErrInvalidArrayLen    = errors.New("wayland: invalid array length")
	ErrUnexpectedEOF      = errors.New("wayland: unexpected end of message")
	ErrStringNotTerminated = errors.New("wayland: string not null-terminated")
)

// Message represents a Wayland wire protocol message.
// It can be either a request (client to server) or event (server to client).
type Message struct {
	// ObjectID is the target object for requests or source object for events.
	ObjectID ObjectID

	// Opcode identifies the specific request or event.
	Opcode Opcode

	// Args contains the message arguments as raw bytes.
	// Use Decoder to extract typed values.
	Args []byte

	// FDs contains file descriptors passed with this message (SCM_RIGHTS).
	FDs []int
}

// Size returns the total wire size of this message in bytes.
func (m *Message) Size() int {
	return headerSize + len(m.Args)
}

// Encoder encodes Wayland messages to the wire format.
type Encoder struct {
	buf []byte
}

// NewEncoder creates a new Encoder with the given initial buffer capacity.
func NewEncoder(capacity int) *Encoder {
	return &Encoder{
		buf: make([]byte, 0, capacity),
	}
}

// Reset clears the encoder's buffer for reuse.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

// Bytes returns the encoded message bytes.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// PutInt32 appends a signed 32-bit integer.
func (e *Encoder) PutInt32(v int32) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(v))
}

// PutUint32 appends an unsigned 32-bit integer.
func (e *Encoder) PutUint32(v uint32) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, v)
}

// PutFixed appends a fixed-point number.
func (e *Encoder) PutFixed(v Fixed) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(v))
}

// PutObject appends an object ID.
func (e *Encoder) PutObject(id ObjectID) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(id))
}

// PutNewID appends a new_id argument (just the object ID, no interface/version).
func (e *Encoder) PutNewID(id ObjectID) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(id))
}

// PutNewIDFull appends a new_id argument with interface name and version.
// This is used for wl_registry.bind and similar dynamic binding.
func (e *Encoder) PutNewIDFull(iface string, version uint32, id ObjectID) {
	e.PutString(iface)
	e.PutUint32(version)
	e.PutUint32(uint32(id))
}

// PutString appends a length-prefixed, null-terminated string.
// The string is padded to a 4-byte boundary.
func (e *Encoder) PutString(s string) {
	// Length includes null terminator
	length := uint32(len(s) + 1)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, length)

	// String data
	e.buf = append(e.buf, s...)

	// Null terminator
	e.buf = append(e.buf, 0)

	// Pad to 4-byte boundary
	padLen := paddingFor(int(length))
	for i := 0; i < padLen; i++ {
		e.buf = append(e.buf, 0)
	}
}

// PutArray appends a length-prefixed byte array.
// The array is padded to a 4-byte boundary.
func (e *Encoder) PutArray(data []byte) {
	length := uint32(len(data))
	e.buf = binary.LittleEndian.AppendUint32(e.buf, length)

	// Array data
	e.buf = append(e.buf, data...)

	// Pad to 4-byte boundary
	padLen := paddingFor(int(length))
	for i := 0; i < padLen; i++ {
		e.buf = append(e.buf, 0)
	}
}

// EncodeMessage encodes a complete message with header.
// The FDs field is not encoded here; FDs are passed via SCM_RIGHTS.
func (e *Encoder) EncodeMessage(objectID ObjectID, opcode Opcode, args []byte) ([]byte, error) {
	totalSize := headerSize + len(args)
	if totalSize > maxMessageSize {
		return nil, ErrMessageTooLarge
	}

	e.Reset()

	// Object ID
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(objectID))

	// Size (16 bits) | Opcode (16 bits)
	sizeAndOpcode := uint32(totalSize)<<16 | uint32(opcode)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, sizeAndOpcode)

	// Arguments
	e.buf = append(e.buf, args...)

	return e.buf, nil
}

// Decoder decodes Wayland messages from wire format.
type Decoder struct {
	buf    []byte
	offset int
	fds    []int
	fdIdx  int
}

// NewDecoder creates a new Decoder with the given buffer.
func NewDecoder(buf []byte) *Decoder {
	return &Decoder{
		buf:    buf,
		offset: 0,
	}
}

// Reset resets the decoder with a new buffer and file descriptors.
func (d *Decoder) Reset(buf []byte, fds []int) {
	d.buf = buf
	d.offset = 0
	d.fds = fds
	d.fdIdx = 0
}

// Remaining returns the number of unread bytes.
func (d *Decoder) Remaining() int {
	return len(d.buf) - d.offset
}

// HasMore returns true if there are more bytes to read.
func (d *Decoder) HasMore() bool {
	return d.offset < len(d.buf)
}

// Skip advances the offset by n bytes.
func (d *Decoder) Skip(n int) error {
	if d.offset+n > len(d.buf) {
		return ErrUnexpectedEOF
	}
	d.offset += n
	return nil
}

// Int32 reads a signed 32-bit integer.
func (d *Decoder) Int32() (int32, error) {
	if d.offset+4 > len(d.buf) {
		return 0, ErrUnexpectedEOF
	}
	v := int32(binary.LittleEndian.Uint32(d.buf[d.offset:]))
	d.offset += 4
	return v, nil
}

// Uint32 reads an unsigned 32-bit integer.
func (d *Decoder) Uint32() (uint32, error) {
	if d.offset+4 > len(d.buf) {
		return 0, ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(d.buf[d.offset:])
	d.offset += 4
	return v, nil
}

// Fixed reads a fixed-point number.
func (d *Decoder) Fixed() (Fixed, error) {
	v, err := d.Uint32()
	if err != nil {
		return 0, err
	}
	return Fixed(v), nil
}

// Object reads an object ID.
func (d *Decoder) Object() (ObjectID, error) {
	v, err := d.Uint32()
	if err != nil {
		return 0, err
	}
	return ObjectID(v), nil
}

// NewID reads a new_id (just the object ID).
func (d *Decoder) NewID() (ObjectID, error) {
	return d.Object()
}

// String reads a length-prefixed, null-terminated string.
func (d *Decoder) String() (string, error) {
	length, err := d.Uint32()
	if err != nil {
		return "", err
	}

	// Length includes null terminator
	if length == 0 {
		return "", nil
	}

	// Validate length
	if length > maxMessageSize {
		return "", ErrInvalidStringLen
	}

	// Calculate padded length
	paddedLen := int(length) + paddingFor(int(length))
	if d.offset+paddedLen > len(d.buf) {
		return "", ErrUnexpectedEOF
	}

	// Read string (excluding null terminator)
	data := d.buf[d.offset : d.offset+int(length)-1]

	// Verify null terminator
	if d.buf[d.offset+int(length)-1] != 0 {
		return "", ErrStringNotTerminated
	}

	d.offset += paddedLen
	return string(data), nil
}

// Array reads a length-prefixed byte array.
func (d *Decoder) Array() ([]byte, error) {
	length, err := d.Uint32()
	if err != nil {
		return nil, err
	}

	if length == 0 {
		return nil, nil
	}

	// Validate length
	if length > maxMessageSize {
		return nil, ErrInvalidArrayLen
	}

	// Calculate padded length
	paddedLen := int(length) + paddingFor(int(length))
	if d.offset+paddedLen > len(d.buf) {
		return nil, ErrUnexpectedEOF
	}

	// Copy array data
	data := make([]byte, length)
	copy(data, d.buf[d.offset:d.offset+int(length)])

	d.offset += paddedLen
	return data, nil
}

// FD reads a file descriptor from the ancillary data.
func (d *Decoder) FD() (int, error) {
	if d.fdIdx >= len(d.fds) {
		return -1, fmt.Errorf("wayland: no more file descriptors available")
	}
	fd := d.fds[d.fdIdx]
	d.fdIdx++
	return fd, nil
}

// DecodeHeader decodes a message header from the buffer.
// Returns the object ID, opcode, total message size, and any error.
func (d *Decoder) DecodeHeader() (ObjectID, Opcode, int, error) {
	if d.Remaining() < headerSize {
		return 0, 0, 0, ErrMessageTooSmall
	}

	objectID, err := d.Object()
	if err != nil {
		return 0, 0, 0, err
	}

	sizeAndOpcode, err := d.Uint32()
	if err != nil {
		return 0, 0, 0, err
	}

	size := int(sizeAndOpcode >> 16)
	opcode := Opcode(sizeAndOpcode & 0xFFFF)

	if size < headerSize {
		return 0, 0, 0, ErrMessageTooSmall
	}
	if size > maxMessageSize {
		return 0, 0, 0, ErrMessageTooLarge
	}

	return objectID, opcode, size, nil
}

// DecodeMessage decodes a complete message from the buffer.
// The decoder must be positioned at the start of a message.
func (d *Decoder) DecodeMessage() (*Message, error) {
	startOffset := d.offset

	objectID, opcode, size, err := d.DecodeHeader()
	if err != nil {
		return nil, err
	}

	// Calculate args size
	argsSize := size - headerSize
	if d.offset+argsSize > len(d.buf) {
		return nil, ErrBufferTooSmall
	}

	// Extract args
	args := make([]byte, argsSize)
	copy(args, d.buf[d.offset:d.offset+argsSize])
	d.offset += argsSize

	msg := &Message{
		ObjectID: objectID,
		Opcode:   opcode,
		Args:     args,
	}

	// Attach consumed FDs
	if d.fdIdx < len(d.fds) {
		// Note: In practice, you'd need to know how many FDs this message expects
		// based on the interface and opcode. For now, we don't attach any here;
		// the caller should use FD() to consume them after decoding args.
		_ = startOffset // Silence unused variable
	}

	return msg, nil
}

// paddingFor returns the padding needed to align length to 4 bytes.
func paddingFor(length int) int {
	return (4 - (length % 4)) % 4
}

// MessageBuilder helps construct message arguments.
type MessageBuilder struct {
	encoder *Encoder
	fds     []int
}

// NewMessageBuilder creates a new MessageBuilder.
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		encoder: NewEncoder(256),
	}
}

// Reset clears the builder for reuse.
func (b *MessageBuilder) Reset() {
	b.encoder.Reset()
	b.fds = b.fds[:0]
}

// PutInt32 appends a signed 32-bit integer.
func (b *MessageBuilder) PutInt32(v int32) *MessageBuilder {
	b.encoder.PutInt32(v)
	return b
}

// PutUint32 appends an unsigned 32-bit integer.
func (b *MessageBuilder) PutUint32(v uint32) *MessageBuilder {
	b.encoder.PutUint32(v)
	return b
}

// PutFixed appends a fixed-point number.
func (b *MessageBuilder) PutFixed(v Fixed) *MessageBuilder {
	b.encoder.PutFixed(v)
	return b
}

// PutObject appends an object ID.
func (b *MessageBuilder) PutObject(id ObjectID) *MessageBuilder {
	b.encoder.PutObject(id)
	return b
}

// PutNewID appends a new_id argument.
func (b *MessageBuilder) PutNewID(id ObjectID) *MessageBuilder {
	b.encoder.PutNewID(id)
	return b
}

// PutNewIDFull appends a new_id with interface name and version.
func (b *MessageBuilder) PutNewIDFull(iface string, version uint32, id ObjectID) *MessageBuilder {
	b.encoder.PutNewIDFull(iface, version, id)
	return b
}

// PutString appends a string.
func (b *MessageBuilder) PutString(s string) *MessageBuilder {
	b.encoder.PutString(s)
	return b
}

// PutArray appends a byte array.
func (b *MessageBuilder) PutArray(data []byte) *MessageBuilder {
	b.encoder.PutArray(data)
	return b
}

// PutFD queues a file descriptor to be passed with the message.
func (b *MessageBuilder) PutFD(fd int) *MessageBuilder {
	b.fds = append(b.fds, fd)
	return b
}

// Build returns the encoded arguments and file descriptors.
func (b *MessageBuilder) Build() ([]byte, []int) {
	return b.encoder.Bytes(), b.fds
}

// BuildMessage returns a complete Message with the given header and built arguments.
func (b *MessageBuilder) BuildMessage(objectID ObjectID, opcode Opcode) *Message {
	args := make([]byte, len(b.encoder.Bytes()))
	copy(args, b.encoder.Bytes())

	fds := make([]int, len(b.fds))
	copy(fds, b.fds)

	return &Message{
		ObjectID: objectID,
		Opcode:   opcode,
		Args:     args,
		FDs:      fds,
	}
}

// EncodeMessage encodes a Message to wire format.
// Returns the encoded bytes (FDs must be passed separately via SCM_RIGHTS).
func EncodeMessage(msg *Message) ([]byte, error) {
	totalSize := headerSize + len(msg.Args)
	if totalSize > maxMessageSize {
		return nil, ErrMessageTooLarge
	}

	buf := make([]byte, totalSize)

	// Object ID
	binary.LittleEndian.PutUint32(buf[0:4], uint32(msg.ObjectID))

	// Size (16 bits) | Opcode (16 bits)
	sizeAndOpcode := uint32(totalSize)<<16 | uint32(msg.Opcode)
	binary.LittleEndian.PutUint32(buf[4:8], sizeAndOpcode)

	// Arguments
	copy(buf[8:], msg.Args)

	return buf, nil
}

// FixedToFloat is a helper that safely converts Fixed to float without overflow.
func FixedToFloat(f Fixed) float64 {
	return float64(f) / 256.0
}

// FloatToFixed converts a float64 to Fixed with clamping to valid range.
func FloatToFixed(f float64) Fixed {
	// Clamp to valid range for 24.8 fixed point
	const maxVal = float64(math.MaxInt32) / 256.0
	const minVal = float64(math.MinInt32) / 256.0

	if f > maxVal {
		f = maxVal
	} else if f < minVal {
		f = minVal
	}

	return Fixed(f * 256.0)
}
