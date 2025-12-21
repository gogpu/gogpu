//go:build linux

package wayland

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// wl_display opcodes (requests)
const (
	displaySync        Opcode = 0 // sync(callback: new_id)
	displayGetRegistry Opcode = 1 // get_registry(registry: new_id)
)

// wl_display event opcodes
const (
	displayEventError    Opcode = 0 // error(object_id: object, code: uint, message: string)
	displayEventDeleteID Opcode = 1 // delete_id(id: uint)
)

// Display error codes (from wayland.xml).
const (
	DisplayErrorInvalidObject  Opcode = 0 // server couldn't find object
	DisplayErrorInvalidMethod  Opcode = 1 // method doesn't exist on the specified interface
	DisplayErrorNoMemory       Opcode = 2 // server is out of memory
	DisplayErrorImplementation Opcode = 3 // implementation error in compositor
)

// Callback interface opcodes (wl_callback).
const (
	callbackEventDone Opcode = 0 // done(callback_data: uint)
)

// Errors returned by Display operations.
var (
	ErrDisplayNotConnected = errors.New("wayland: display not connected")
	ErrNoWaylandSocket     = errors.New("wayland: no wayland socket found")
	ErrProtocolError       = errors.New("wayland: protocol error from compositor")
	ErrConnectionClosed    = errors.New("wayland: connection closed")
	ErrNoMessage           = errors.New("wayland: no message available")
)

// Display represents a connection to the Wayland compositor.
// It is always object ID 1 in the Wayland protocol.
type Display struct {
	conn     net.Conn
	connFile *os.File

	// Object ID allocation
	nextID atomic.Uint32

	// Synchronization
	mu        sync.Mutex
	readBuf   []byte
	writeBuf  []byte
	fdBuf     []int
	callbacks map[ObjectID]chan uint32
	closed    bool

	// Protocol error state
	protocolError     error
	protocolErrorOnce sync.Once

	// Event handlers
	registry *Registry
	onError  func(objectID ObjectID, code uint32, message string)

	// Delete ID tracking
	deletedIDs []ObjectID
}

// Connect establishes a connection to the Wayland compositor.
// It looks for the socket at $XDG_RUNTIME_DIR/$WAYLAND_DISPLAY.
// If WAYLAND_DISPLAY is not set, it defaults to "wayland-0".
func Connect() (*Display, error) {
	socketPath, err := getSocketPath()
	if err != nil {
		return nil, err
	}

	return ConnectTo(socketPath)
}

// ConnectTo establishes a connection to the Wayland compositor at the given socket path.
func ConnectTo(socketPath string) (*Display, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("wayland: failed to connect to %s: %w", socketPath, err)
	}

	// Get the underlying file for sendmsg/recvmsg operations
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("wayland: expected unix socket, got %T", conn)
	}

	file, err := unixConn.File()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("wayland: failed to get socket file: %w", err)
	}

	d := &Display{
		conn:      conn,
		connFile:  file,
		readBuf:   make([]byte, maxMessageSize),
		writeBuf:  make([]byte, 0, 4096),
		fdBuf:     make([]int, 0, 16),
		callbacks: make(map[ObjectID]chan uint32),
	}

	// wl_display is always object ID 1, so start allocating from 2
	d.nextID.Store(2)

	return d, nil
}

// getSocketPath returns the path to the Wayland socket.
func getSocketPath() (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return "", fmt.Errorf("%w: XDG_RUNTIME_DIR not set", ErrNoWaylandSocket)
	}

	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = "wayland-0"
	}

	// Check if WAYLAND_DISPLAY is an absolute path
	if filepath.IsAbs(display) {
		return display, nil
	}

	return filepath.Join(runtimeDir, display), nil
}

// Close closes the connection to the compositor.
func (d *Display) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true

	// Close all pending callbacks
	for _, ch := range d.callbacks {
		close(ch)
	}
	d.callbacks = nil

	// Close file and connection
	if d.connFile != nil {
		_ = d.connFile.Close()
	}
	if d.conn != nil {
		return d.conn.Close()
	}

	return nil
}

// AllocID allocates a new object ID.
func (d *Display) AllocID() ObjectID {
	return ObjectID(d.nextID.Add(1) - 1)
}

// Sync sends a sync request and returns a channel that receives the callback data.
// This is used for roundtrip synchronization with the compositor.
func (d *Display) Sync() (<-chan uint32, error) {
	callbackID := d.AllocID()

	ch := make(chan uint32, 1)
	d.mu.Lock()
	d.callbacks[callbackID] = ch
	d.mu.Unlock()

	// Build sync request: sync(callback: new_id<wl_callback>)
	builder := NewMessageBuilder()
	builder.PutNewID(callbackID)
	msg := builder.BuildMessage(1, displaySync) // wl_display is always object 1

	if err := d.SendMessage(msg); err != nil {
		d.mu.Lock()
		delete(d.callbacks, callbackID)
		d.mu.Unlock()
		close(ch)
		return nil, err
	}

	return ch, nil
}

// Roundtrip performs a synchronous roundtrip to the compositor.
// It sends a sync request and waits for the callback, ensuring all
// previous requests have been processed.
func (d *Display) Roundtrip() error {
	ch, err := d.Sync()
	if err != nil {
		return err
	}

	// Read events until we get our callback
	for {
		if err := d.DispatchOne(); err != nil {
			return err
		}

		select {
		case _, ok := <-ch:
			if !ok {
				return ErrConnectionClosed
			}
			return nil
		default:
			// Continue reading
		}
	}
}

// GetRegistry requests the global registry from the compositor.
// The Registry is used to bind to global interfaces like wl_compositor.
func (d *Display) GetRegistry() (*Registry, error) {
	if d.registry != nil {
		return d.registry, nil
	}

	registryID := d.AllocID()

	// Build get_registry request: get_registry(registry: new_id<wl_registry>)
	builder := NewMessageBuilder()
	builder.PutNewID(registryID)
	msg := builder.BuildMessage(1, displayGetRegistry)

	if err := d.SendMessage(msg); err != nil {
		return nil, err
	}

	d.registry = newRegistry(d, registryID)
	return d.registry, nil
}

// SendMessage sends a message to the compositor.
func (d *Display) SendMessage(msg *Message) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return ErrDisplayNotConnected
	}

	// Check for protocol error
	if d.protocolError != nil {
		return d.protocolError
	}

	// Encode message
	data, err := EncodeMessage(msg)
	if err != nil {
		return err
	}

	// Send with or without file descriptors
	if len(msg.FDs) > 0 {
		return d.sendWithFDs(data, msg.FDs)
	}

	_, err = d.conn.Write(data)
	return err
}

// sendWithFDs sends data with file descriptors via SCM_RIGHTS.
func (d *Display) sendWithFDs(data []byte, fds []int) error {
	fd := int(d.connFile.Fd())

	// Build control message for SCM_RIGHTS
	rights := unix.UnixRights(fds...)

	return unix.Sendmsg(fd, data, rights, nil, 0)
}

// RecvMessage receives a message from the compositor.
// It may block if no message is available.
func (d *Display) RecvMessage() (*Message, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil, ErrDisplayNotConnected
	}

	fd := int(d.connFile.Fd())

	// Prepare control message buffer for SCM_RIGHTS
	// Each fd is 4 bytes, allow for up to 28 fds
	// Control message header is 16 bytes (unix.Cmsghdr), data is 28*4 bytes
	// Total buffer size: 16 + 112 = 128 bytes, rounded up to 256 for safety
	oob := make([]byte, 256)

	n, oobn, _, _, err := unix.Recvmsg(fd, d.readBuf, oob, 0)
	if err != nil {
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrNoMessage
		}
		return nil, fmt.Errorf("wayland: recvmsg failed: %w", err)
	}

	if n == 0 {
		return nil, ErrConnectionClosed
	}

	// Parse received file descriptors
	fds, err := parseFileDescriptors(oob[:oobn])
	if err != nil {
		return nil, err
	}

	// Decode message
	decoder := NewDecoder(d.readBuf[:n])
	decoder.fds = fds

	msg, err := decoder.DecodeMessage()
	if err != nil {
		return nil, err
	}

	msg.FDs = fds
	return msg, nil
}

// DispatchOne reads and dispatches a single event from the compositor.
func (d *Display) DispatchOne() error {
	msg, err := d.RecvMessage()
	if err != nil {
		if errors.Is(err, ErrNoMessage) {
			return nil // No message available is not an error
		}
		return err
	}

	return d.dispatch(msg)
}

// Dispatch reads and dispatches all pending events from the compositor.
func (d *Display) Dispatch() error {
	for {
		msg, err := d.RecvMessage()
		if err != nil {
			if errors.Is(err, ErrNoMessage) {
				return nil // No more messages
			}
			return err
		}

		if err := d.dispatch(msg); err != nil {
			return err
		}
	}
}

// dispatch routes a message to the appropriate handler.
func (d *Display) dispatch(msg *Message) error {
	switch msg.ObjectID {
	case 1: // wl_display
		return d.dispatchDisplayEvent(msg)

	default:
		// Check if it's a callback
		d.mu.Lock()
		ch, ok := d.callbacks[msg.ObjectID]
		d.mu.Unlock()

		if ok && msg.Opcode == callbackEventDone {
			decoder := NewDecoder(msg.Args)
			data, err := decoder.Uint32()
			if err != nil {
				return err
			}

			d.mu.Lock()
			delete(d.callbacks, msg.ObjectID)
			d.mu.Unlock()

			ch <- data
			close(ch)
			return nil
		}

		// Check if it's a registry event
		if d.registry != nil && msg.ObjectID == d.registry.id {
			return d.registry.dispatch(msg)
		}

		// Unknown object - this is not necessarily an error
		// The object might have been created by application code
		return nil
	}
}

// dispatchDisplayEvent handles wl_display events.
func (d *Display) dispatchDisplayEvent(msg *Message) error {
	switch msg.Opcode {
	case displayEventError:
		return d.handleError(msg)

	case displayEventDeleteID:
		return d.handleDeleteID(msg)

	default:
		// Unknown event - ignore
		return nil
	}
}

// handleError handles the wl_display.error event.
func (d *Display) handleError(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	objectID, err := decoder.Object()
	if err != nil {
		return err
	}

	code, err := decoder.Uint32()
	if err != nil {
		return err
	}

	message, err := decoder.String()
	if err != nil {
		return err
	}

	// Store protocol error
	d.protocolErrorOnce.Do(func() {
		d.protocolError = fmt.Errorf("%w: object %d code %d: %s",
			ErrProtocolError, objectID, code, message)
	})

	// Call user error handler if set
	if d.onError != nil {
		d.onError(objectID, code, message)
	}

	return d.protocolError
}

// handleDeleteID handles the wl_display.delete_id event.
func (d *Display) handleDeleteID(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	id, err := decoder.Uint32()
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.deletedIDs = append(d.deletedIDs, ObjectID(id))
	d.mu.Unlock()

	// Note: In a full implementation, you would recycle these IDs
	// and clean up any local objects with this ID.

	return nil
}

// SetErrorHandler sets a callback for protocol errors.
// The handler receives the object ID, error code, and error message.
func (d *Display) SetErrorHandler(handler func(objectID ObjectID, code uint32, message string)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onError = handler
}

// GetProtocolError returns any protocol error received from the compositor.
// Returns nil if no error has occurred.
func (d *Display) GetProtocolError() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.protocolError
}

// Flush sends any buffered data to the compositor.
// This is typically not needed as messages are sent immediately.
func (d *Display) Flush() error {
	// Currently messages are sent immediately, so this is a no-op.
	// In a production implementation, you might want to buffer
	// messages and flush them together for efficiency.
	return nil
}

// DisplayID returns the object ID of the display (always 1).
func (d *Display) DisplayID() ObjectID {
	return 1
}

// Fd returns the file descriptor of the socket connection.
// This can be used with poll/epoll for event loop integration.
func (d *Display) Fd() int {
	if d.connFile == nil {
		return -1
	}
	return int(d.connFile.Fd())
}

// Ptr returns the file descriptor as a uintptr for use with Vulkan surface creation.
// This is used with VK_KHR_wayland_surface extension.
// Note: In Wayland, we pass the fd as the "display pointer" since the Display
// struct wraps a Unix socket connection, not a C pointer.
func (d *Display) Ptr() uintptr {
	return uintptr(d.Fd())
}

// parseFileDescriptors extracts file descriptors from socket control messages.
func parseFileDescriptors(oob []byte) ([]int, error) {
	if len(oob) == 0 {
		return nil, nil
	}

	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, fmt.Errorf("wayland: parse control message failed: %w", err)
	}

	var fds []int
	for _, scm := range scms {
		if scm.Header.Level != unix.SOL_SOCKET || scm.Header.Type != unix.SCM_RIGHTS {
			continue
		}
		gotFDs, err := unix.ParseUnixRights(&scm)
		if err != nil {
			return nil, fmt.Errorf("wayland: parse unix rights failed: %w", err)
		}
		fds = append(fds, gotFDs...)
	}

	return fds, nil
}
