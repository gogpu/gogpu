//go:build linux

package wayland

import (
	"fmt"
	"sync"
)

// wl_shm opcodes (requests)
const (
	shmCreatePool Opcode = 0 // create_pool(id: new_id<wl_shm_pool>, fd: fd, size: int)
)

// wl_shm event opcodes
const (
	shmEventFormat Opcode = 0 // format(format: uint)
)

// wl_shm_pool opcodes (requests)
const (
	shmPoolCreateBuffer Opcode = 0 // create_buffer(id: new_id, offset: int, width: int, height: int, stride: int, format: uint)
	shmPoolDestroy      Opcode = 1 // destroy()
	shmPoolResize       Opcode = 2 // resize(size: int)
)

// wl_buffer opcodes (requests)
const (
	bufferDestroy Opcode = 0 // destroy()
)

// wl_buffer event opcodes
const (
	bufferEventRelease Opcode = 0 // release()
)

// ShmFormat represents a pixel format supported by wl_shm.
// These match the wl_shm_format enum from wayland.xml.
type ShmFormat uint32

// Common wl_shm_format values (subset).
const (
	// ShmFormatARGB8888 is 32-bit ARGB (8-8-8-8), little-endian.
	ShmFormatARGB8888 ShmFormat = 0

	// ShmFormatXRGB8888 is 32-bit RGB (8-8-8-8), little-endian, no alpha.
	ShmFormatXRGB8888 ShmFormat = 1

	// ShmFormatC8 is 8-bit color index.
	ShmFormatC8 ShmFormat = 0x20203843

	// ShmFormatRGB332 is 8-bit RGB (3-3-2).
	ShmFormatRGB332 ShmFormat = 0x38424752

	// ShmFormatBGR233 is 8-bit BGR (2-3-3).
	ShmFormatBGR233 ShmFormat = 0x38524742

	// ShmFormatXRGB4444 is 16-bit xRGB (4-4-4-4).
	ShmFormatXRGB4444 ShmFormat = 0x32315258

	// ShmFormatXBGR4444 is 16-bit xBGR (4-4-4-4).
	ShmFormatXBGR4444 ShmFormat = 0x32314258

	// ShmFormatRGBX4444 is 16-bit RGBx (4-4-4-4).
	ShmFormatRGBX4444 ShmFormat = 0x32315852

	// ShmFormatBGRX4444 is 16-bit BGRx (4-4-4-4).
	ShmFormatBGRX4444 ShmFormat = 0x32315842

	// ShmFormatARGB4444 is 16-bit ARGB (4-4-4-4).
	ShmFormatARGB4444 ShmFormat = 0x32315241

	// ShmFormatABGR4444 is 16-bit ABGR (4-4-4-4).
	ShmFormatABGR4444 ShmFormat = 0x32314241

	// ShmFormatRGBA4444 is 16-bit RGBA (4-4-4-4).
	ShmFormatRGBA4444 ShmFormat = 0x32314152

	// ShmFormatBGRA4444 is 16-bit BGRA (4-4-4-4).
	ShmFormatBGRA4444 ShmFormat = 0x32314142

	// ShmFormatXRGB1555 is 16-bit xRGB (1-5-5-5).
	ShmFormatXRGB1555 ShmFormat = 0x35315258

	// ShmFormatXBGR1555 is 16-bit xBGR (1-5-5-5).
	ShmFormatXBGR1555 ShmFormat = 0x35314258

	// ShmFormatRGBX5551 is 16-bit RGBx (5-5-5-1).
	ShmFormatRGBX5551 ShmFormat = 0x35315852

	// ShmFormatBGRX5551 is 16-bit BGRx (5-5-5-1).
	ShmFormatBGRX5551 ShmFormat = 0x35315842

	// ShmFormatARGB1555 is 16-bit ARGB (1-5-5-5).
	ShmFormatARGB1555 ShmFormat = 0x35315241

	// ShmFormatABGR1555 is 16-bit ABGR (1-5-5-5).
	ShmFormatABGR1555 ShmFormat = 0x35314241

	// ShmFormatRGBA5551 is 16-bit RGBA (5-5-5-1).
	ShmFormatRGBA5551 ShmFormat = 0x35314152

	// ShmFormatBGRA5551 is 16-bit BGRA (5-5-5-1).
	ShmFormatBGRA5551 ShmFormat = 0x35314142

	// ShmFormatRGB565 is 16-bit RGB (5-6-5).
	ShmFormatRGB565 ShmFormat = 0x36314752

	// ShmFormatBGR565 is 16-bit BGR (5-6-5).
	ShmFormatBGR565 ShmFormat = 0x36314742

	// ShmFormatRGB888 is 24-bit RGB (8-8-8).
	ShmFormatRGB888 ShmFormat = 0x34324752

	// ShmFormatBGR888 is 24-bit BGR (8-8-8).
	ShmFormatBGR888 ShmFormat = 0x34324742

	// ShmFormatXBGR8888 is 32-bit xBGR (8-8-8-8).
	ShmFormatXBGR8888 ShmFormat = 0x34324258

	// ShmFormatRGBX8888 is 32-bit RGBx (8-8-8-8).
	ShmFormatRGBX8888 ShmFormat = 0x34325852

	// ShmFormatBGRX8888 is 32-bit BGRx (8-8-8-8).
	ShmFormatBGRX8888 ShmFormat = 0x34325842

	// ShmFormatABGR8888 is 32-bit ABGR (8-8-8-8).
	ShmFormatABGR8888 ShmFormat = 0x34324241

	// ShmFormatRGBA8888 is 32-bit RGBA (8-8-8-8).
	ShmFormatRGBA8888 ShmFormat = 0x34324152

	// ShmFormatBGRA8888 is 32-bit BGRA (8-8-8-8).
	ShmFormatBGRA8888 ShmFormat = 0x34324142
)

// String returns a human-readable name for common formats.
func (f ShmFormat) String() string {
	switch f {
	case ShmFormatARGB8888:
		return "ARGB8888"
	case ShmFormatXRGB8888:
		return "XRGB8888"
	case ShmFormatXBGR8888:
		return "XBGR8888"
	case ShmFormatRGBX8888:
		return "RGBX8888"
	case ShmFormatBGRX8888:
		return "BGRX8888"
	case ShmFormatABGR8888:
		return "ABGR8888"
	case ShmFormatRGBA8888:
		return "RGBA8888"
	case ShmFormatBGRA8888:
		return "BGRA8888"
	case ShmFormatRGB888:
		return "RGB888"
	case ShmFormatBGR888:
		return "BGR888"
	case ShmFormatRGB565:
		return "RGB565"
	case ShmFormatBGR565:
		return "BGR565"
	default:
		return fmt.Sprintf("0x%08X", uint32(f))
	}
}

// WlShm represents the wl_shm interface.
// It provides shared memory support for creating buffers.
type WlShm struct {
	display *Display
	id      ObjectID

	mu      sync.RWMutex
	formats []ShmFormat

	onFormat func(format ShmFormat)
}

// NewWlShm creates a WlShm from a bound object ID.
// The objectID should be obtained from Registry.BindShm().
func NewWlShm(display *Display, objectID ObjectID) *WlShm {
	return &WlShm{
		display: display,
		id:      objectID,
		formats: make([]ShmFormat, 0, 16),
	}
}

// ID returns the object ID of the shm.
func (s *WlShm) ID() ObjectID {
	return s.id
}

// CreatePool creates a new shared memory pool from a file descriptor.
// The fd should be a file descriptor to a shared memory object (e.g., from
// shm_open or memfd_create). The size is the size of the pool in bytes.
// The file descriptor is consumed by this call and should not be used afterward.
func (s *WlShm) CreatePool(fd int, size int32) (*WlShmPool, error) {
	poolID := s.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(poolID)
	builder.PutFD(fd)
	builder.PutInt32(size)
	msg := builder.BuildMessage(s.id, shmCreatePool)

	if err := s.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewWlShmPool(s.display, poolID, size), nil
}

// Formats returns a copy of the supported pixel formats.
// This list is populated by format events from the compositor.
// Call Display.Roundtrip() after binding to ensure formats are received.
func (s *WlShm) Formats() []ShmFormat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ShmFormat, len(s.formats))
	copy(result, s.formats)
	return result
}

// HasFormat returns true if the given format is supported.
func (s *WlShm) HasFormat(format ShmFormat) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, f := range s.formats {
		if f == format {
			return true
		}
	}
	return false
}

// SetFormatHandler sets a callback for the format event.
// The handler is called when the compositor advertises a supported format.
func (s *WlShm) SetFormatHandler(handler func(format ShmFormat)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onFormat = handler
}

// dispatch handles wl_shm events.
func (s *WlShm) dispatch(msg *Message) error {
	if msg.Opcode == shmEventFormat {
		return s.handleFormat(msg)
	}
	return nil
}

func (s *WlShm) handleFormat(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	formatVal, err := decoder.Uint32()
	if err != nil {
		return err
	}

	format := ShmFormat(formatVal)

	s.mu.Lock()
	s.formats = append(s.formats, format)
	handler := s.onFormat
	s.mu.Unlock()

	if handler != nil {
		handler(format)
	}
	return nil
}

// WlShmPool represents the wl_shm_pool interface.
// A pool is a chunk of shared memory from which buffers can be created.
type WlShmPool struct {
	display *Display
	id      ObjectID
	size    int32
}

// NewWlShmPool creates a WlShmPool from an object ID.
func NewWlShmPool(display *Display, objectID ObjectID, size int32) *WlShmPool {
	return &WlShmPool{
		display: display,
		id:      objectID,
		size:    size,
	}
}

// ID returns the object ID of the pool.
func (p *WlShmPool) ID() ObjectID {
	return p.id
}

// Size returns the size of the pool in bytes.
func (p *WlShmPool) Size() int32 {
	return p.size
}

// CreateBuffer creates a buffer from this pool.
// Parameters:
//   - offset: byte offset within the pool
//   - width: width of the buffer in pixels
//   - height: height of the buffer in pixels
//   - stride: number of bytes per row
//   - format: pixel format
func (p *WlShmPool) CreateBuffer(offset, width, height, stride int32, format ShmFormat) (*WlBuffer, error) {
	bufferID := p.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(bufferID)
	builder.PutInt32(offset)
	builder.PutInt32(width)
	builder.PutInt32(height)
	builder.PutInt32(stride)
	builder.PutUint32(uint32(format))
	msg := builder.BuildMessage(p.id, shmPoolCreateBuffer)

	if err := p.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewWlBuffer(p.display, bufferID), nil
}

// Resize resizes the pool.
// This can only make the pool larger. The size parameter is the new size in bytes.
func (p *WlShmPool) Resize(size int32) error {
	if size < p.size {
		return fmt.Errorf("wayland: cannot shrink pool from %d to %d", p.size, size)
	}

	builder := NewMessageBuilder()
	builder.PutInt32(size)
	msg := builder.BuildMessage(p.id, shmPoolResize)

	if err := p.display.SendMessage(msg); err != nil {
		return err
	}

	p.size = size
	return nil
}

// Destroy destroys the pool.
// Buffers created from this pool remain valid after the pool is destroyed.
func (p *WlShmPool) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(p.id, shmPoolDestroy)

	return p.display.SendMessage(msg)
}

// WlBuffer represents the wl_buffer interface.
// A buffer contains pixel data that can be attached to a surface.
type WlBuffer struct {
	display *Display
	id      ObjectID

	mu        sync.Mutex
	onRelease func()
}

// NewWlBuffer creates a WlBuffer from an object ID.
func NewWlBuffer(display *Display, objectID ObjectID) *WlBuffer {
	return &WlBuffer{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID of the buffer.
func (b *WlBuffer) ID() ObjectID {
	return b.id
}

// Destroy destroys the buffer.
func (b *WlBuffer) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(b.id, bufferDestroy)

	return b.display.SendMessage(msg)
}

// SetReleaseHandler sets a callback for the release event.
// The release event is sent when the compositor is done using the buffer.
// After receiving this event, the client can safely modify or reuse the buffer.
func (b *WlBuffer) SetReleaseHandler(handler func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onRelease = handler
}

// dispatch handles wl_buffer events.
func (b *WlBuffer) dispatch(msg *Message) error {
	switch msg.Opcode {
	case bufferEventRelease:
		b.mu.Lock()
		handler := b.onRelease
		b.mu.Unlock()

		if handler != nil {
			handler()
		}
	default:
		// Unknown event - ignore
	}
	return nil
}
