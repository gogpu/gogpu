//go:build linux

package x11

import "fmt"

// PutImage format values.
const (
	// ImageFormatBitmap is XY bitmap format (1 bit per pixel).
	ImageFormatBitmap = 0
	// ImageFormatXYPixmap is XY pixmap format.
	ImageFormatXYPixmap = 1
	// ImageFormatZPixmap is Z (packed) pixmap format.
	ImageFormatZPixmap = 2
)

// CreateGC creates a graphics context for the given drawable.
// Returns the GC resource ID.
func (c *Connection) CreateGC(drawable ResourceID) (ResourceID, error) {
	gcID := c.GenerateID()

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeCreateGC)
	e.PutUint8(0)  // unused
	e.PutUint16(4) // request length in 4-byte units (header only, no values)
	e.PutUint32(uint32(gcID))
	e.PutUint32(uint32(drawable))
	e.PutUint32(0) // value mask (no attributes)

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return 0, fmt.Errorf("x11: CreateGC failed: %w", err)
	}

	return gcID, nil
}

// FreeGC frees a graphics context.
func (c *Connection) FreeGC(gc ResourceID) error {
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeFreeGC)
	e.PutUint8(0)  // unused
	e.PutUint16(2) // request length
	e.PutUint32(uint32(gc))

	if _, err := c.sendRequest(e.Bytes()); err != nil {
		return fmt.Errorf("x11: FreeGC failed: %w", err)
	}
	return nil
}

// PutImage sends pixel data to a drawable using the PutImage request.
// format must be ImageFormatZPixmap for 32-bit BGRA data.
// The data slice must contain width*height*4 bytes in the X11-expected
// pixel format (typically BGRA on little-endian for 32-bit visuals).
//
// For images larger than the X11 maximum request size, PutImage
// automatically splits the data into multiple row-strip requests.
func (c *Connection) PutImage(drawable ResourceID, gc ResourceID,
	width, height uint16, dstX, dstY int16,
	depth uint8, format uint8, data []byte) error {
	// X11 PutImage header: opcode(1) + format(1) + length(2) +
	// drawable(4) + gc(4) + width(2) + height(2) +
	// dstX(2) + dstY(2) + leftPad(1) + depth(1) + pad(2) = 24 bytes
	const headerSize = 24

	// Maximum request size in bytes. MaxRequestLength is in 4-byte units.
	maxReqLen := int(c.setup.MaxRequestLength) * 4
	if maxReqLen <= headerSize {
		maxReqLen = 262140 // 65535 * 4 default
	}

	maxDataPerReq := maxReqLen - headerSize
	bytesPerRow := int(width) * 4 // 32-bit pixels = 4 bytes/pixel

	if bytesPerRow == 0 {
		return nil
	}

	// How many rows fit in one request?
	rowsPerReq := maxDataPerReq / bytesPerRow
	if rowsPerReq < 1 {
		rowsPerReq = 1
	}

	// Send row strips
	rowsSent := 0
	totalRows := int(height)

	for rowsSent < totalRows {
		rows := rowsPerReq
		if rowsSent+rows > totalRows {
			rows = totalRows - rowsSent
		}

		chunkSize := rows * bytesPerRow
		offset := rowsSent * bytesPerRow

		if offset+chunkSize > len(data) {
			chunkSize = len(data) - offset
		}
		if chunkSize <= 0 {
			break
		}

		chunk := data[offset : offset+chunkSize]

		// Pad to 4-byte boundary
		padLen := (4 - chunkSize%4) % 4
		reqLen := uint16((headerSize + chunkSize + padLen) / 4)

		e := NewEncoder(c.byteOrder)
		e.PutUint8(OpcodePutImage)
		e.PutUint8(format) // format in the "detail" byte
		e.PutUint16(reqLen)
		e.PutUint32(uint32(drawable))
		e.PutUint32(uint32(gc))
		e.PutUint16(width)
		e.PutUint16(uint16(rows))
		e.PutInt16(dstX)
		e.PutInt16(dstY + int16(rowsSent))
		e.PutUint8(0)     // left-pad (only for XYBitmap/XYPixmap)
		e.PutUint8(depth) // depth
		e.PutUint16(0)    // pad
		e.PutBytes(chunk)
		// Add padding bytes
		for i := 0; i < padLen; i++ {
			e.PutUint8(0)
		}

		if _, err := c.sendRequest(e.Bytes()); err != nil {
			return fmt.Errorf("x11: PutImage failed at row %d: %w", rowsSent, err)
		}

		rowsSent += rows
	}

	return nil
}
