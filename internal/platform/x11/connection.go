//go:build linux

package x11

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Connection errors.
var (
	ErrNotConnected    = errors.New("x11: not connected")
	ErrConnectionClosed = errors.New("x11: connection closed")
	ErrNoDisplay       = errors.New("x11: DISPLAY not set")
	ErrInvalidDisplay  = errors.New("x11: invalid DISPLAY format")
	ErrProtocolError   = errors.New("x11: protocol error")
)

// Connection represents a connection to an X11 server.
type Connection struct {
	conn     net.Conn
	connFile *os.File

	// Protocol settings
	byteOrder ByteOrder

	// Setup information
	setup *SetupInfo

	// Resource ID generation
	resourceIDBase uint32
	resourceIDMask uint32
	resourceIDLast uint32

	// Sequence number tracking
	nextSeq atomic.Uint32

	// Synchronization
	mu       sync.Mutex
	readBuf  []byte
	writeBuf []byte
	closed   bool

	// Atom cache
	atomCache     map[string]Atom
	atomCacheLock sync.RWMutex

	// Screen number
	screenNum int

	// Pending replies
	pendingReplies     map[uint16]chan []byte
	pendingRepliesLock sync.Mutex
}

// Connect establishes a connection to the X server using the DISPLAY environment variable.
func Connect() (*Connection, error) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		return nil, ErrNoDisplay
	}
	return ConnectTo(display)
}

// ConnectTo connects to the specified display.
// Display format: [host]:display[.screen]
// Examples: ":0", ":0.0", "localhost:0", "192.168.1.1:0"
func ConnectTo(display string) (*Connection, error) {
	host, displayNum, screenNum, err := parseDisplay(display)
	if err != nil {
		return nil, err
	}

	// Determine socket path or address
	var network, address string
	if host == "" {
		// Unix socket connection
		network = "unix"
		// Note: This is an intentional Unix-specific path, not using filepath.Join
		// because the X11 socket path is defined by the X.Org specification.
		address = "/tmp/.X11-unix/X" + strconv.Itoa(displayNum)
	} else {
		// TCP connection
		network = "tcp"
		port := 6000 + displayNum
		address = fmt.Sprintf("%s:%d", host, port)
	}

	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("x11: failed to connect to %s: %w", address, err)
	}

	c := &Connection{
		conn:           conn,
		byteOrder:      LSBFirst,
		readBuf:        make([]byte, 32*1024),
		writeBuf:       make([]byte, 0, 4096),
		atomCache:      make(map[string]Atom),
		screenNum:      screenNum,
		pendingReplies: make(map[uint16]chan []byte),
	}

	// Get file descriptor for raw socket operations
	switch tc := conn.(type) {
	case *net.UnixConn:
		c.connFile, _ = tc.File()
	case *net.TCPConn:
		c.connFile, _ = tc.File()
	}

	// Perform connection setup
	if err := c.performSetup(host, strconv.Itoa(displayNum)); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return c, nil
}

// parseDisplay parses an X11 display string.
// Returns host (empty for local), display number, and screen number.
func parseDisplay(display string) (host string, displayNum int, screenNum int, err error) {
	// Format: [host]:display[.screen]

	// Find the last colon (separating host from display)
	colonIdx := strings.LastIndex(display, ":")
	if colonIdx == -1 {
		return "", 0, 0, ErrInvalidDisplay
	}

	host = display[:colonIdx]
	rest := display[colonIdx+1:]

	// Parse display.screen
	dotIdx := strings.Index(rest, ".")
	var displayStr, screenStr string
	if dotIdx == -1 {
		displayStr = rest
		screenStr = "0"
	} else {
		displayStr = rest[:dotIdx]
		screenStr = rest[dotIdx+1:]
	}

	displayNum, err = strconv.Atoi(displayStr)
	if err != nil {
		return "", 0, 0, ErrInvalidDisplay
	}

	screenNum, err = strconv.Atoi(screenStr)
	if err != nil {
		return "", 0, 0, ErrInvalidDisplay
	}

	return host, displayNum, screenNum, nil
}

// performSetup performs the X11 connection setup handshake.
func (c *Connection) performSetup(hostname, displayNum string) error {
	// Get authentication data
	authName, authData, err := getAuth(hostname, displayNum)
	if err != nil {
		// Try without auth
		authName = ""
		authData = nil
	}

	// Build and send setup request
	setupReq := buildSetupRequest(c.byteOrder, authName, authData)
	if _, err := c.conn.Write(setupReq); err != nil {
		return fmt.Errorf("x11: failed to send setup request: %w", err)
	}

	// Read initial response (8 bytes minimum)
	initialBuf := make([]byte, 8)
	if _, err := c.conn.Read(initialBuf); err != nil {
		return fmt.Errorf("x11: failed to read setup response: %w", err)
	}

	// Check status
	status := initialBuf[0]
	if status != SetupSuccess {
		// Read the rest for error message
		d := NewDecoder(c.byteOrder, initialBuf)
		_, _ = d.Uint8() // status
		reasonLen, _ := d.Uint8()
		_, _ = d.Uint16() // major
		_, _ = d.Uint16() // minor
		additionalLen, _ := d.Uint16()

		// Read additional data
		additionalBuf := make([]byte, additionalLen*4)
		_, _ = c.conn.Read(additionalBuf)

		if reasonLen > 0 && int(reasonLen) <= len(additionalBuf) {
			reason := string(additionalBuf[:reasonLen])
			return fmt.Errorf("%w: %s", ErrSetupFailed, reason)
		}
		return ErrSetupFailed
	}

	// Read additional data length (from bytes 6-7)
	d := NewDecoder(c.byteOrder, initialBuf[6:8])
	additionalLen, _ := d.Uint16()

	// Read remaining setup data
	remainingBuf := make([]byte, additionalLen*4)
	totalRead := 0
	for totalRead < len(remainingBuf) {
		n, err := c.conn.Read(remainingBuf[totalRead:])
		if err != nil {
			return fmt.Errorf("x11: failed to read setup data: %w", err)
		}
		totalRead += n
	}

	// Combine buffers for parsing
	fullResponse := make([]byte, len(initialBuf)+len(remainingBuf))
	copy(fullResponse, initialBuf)
	copy(fullResponse[len(initialBuf):], remainingBuf)

	// Parse setup response
	setup, err := parseSetupResponse(c.byteOrder, fullResponse)
	if err != nil {
		return err
	}

	c.setup = setup
	c.resourceIDBase = setup.ResourceIDBase
	c.resourceIDMask = setup.ResourceIDMask
	c.resourceIDLast = 0

	return nil
}

// Close closes the connection to the X server.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Close pending reply channels
	c.pendingRepliesLock.Lock()
	for _, ch := range c.pendingReplies {
		close(ch)
	}
	c.pendingReplies = nil
	c.pendingRepliesLock.Unlock()

	if c.connFile != nil {
		_ = c.connFile.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// GenerateID generates a new resource ID.
func (c *Connection) GenerateID() ResourceID {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.resourceIDLast
	c.resourceIDLast++
	return ResourceID((id << 0) | c.resourceIDBase) // Simplified - uses base directly
}

// getNextSeq returns the next sequence number and increments it.
func (c *Connection) getNextSeq() uint16 {
	return uint16(c.nextSeq.Add(1))
}

// Fd returns the connection file descriptor.
func (c *Connection) Fd() int {
	if c.connFile != nil {
		return int(c.connFile.Fd())
	}
	return -1
}

// Setup returns the setup information from the X server.
func (c *Connection) Setup() *SetupInfo {
	return c.setup
}

// DefaultScreen returns the default screen information.
func (c *Connection) DefaultScreen() *ScreenInfo {
	if c.setup == nil || len(c.setup.Screens) == 0 {
		return nil
	}
	if c.screenNum >= len(c.setup.Screens) {
		return &c.setup.Screens[0]
	}
	return &c.setup.Screens[c.screenNum]
}

// RootWindow returns the root window of the default screen.
func (c *Connection) RootWindow() ResourceID {
	screen := c.DefaultScreen()
	if screen == nil {
		return 0
	}
	return screen.Root
}

// sendRequest sends a request and returns the sequence number.
func (c *Connection) sendRequest(data []byte) (uint16, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, ErrConnectionClosed
	}

	seq := c.getNextSeq()
	if _, err := c.conn.Write(data); err != nil {
		return 0, fmt.Errorf("x11: failed to send request: %w", err)
	}

	return seq, nil
}

// sendRequestWithReply sends a request and waits for a reply.
func (c *Connection) sendRequestWithReply(data []byte) ([]byte, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrConnectionClosed
	}

	seq := c.getNextSeq()
	replyCh := make(chan []byte, 1)

	c.pendingRepliesLock.Lock()
	c.pendingReplies[seq] = replyCh
	c.pendingRepliesLock.Unlock()

	if _, err := c.conn.Write(data); err != nil {
		c.mu.Unlock()
		c.pendingRepliesLock.Lock()
		delete(c.pendingReplies, seq)
		c.pendingRepliesLock.Unlock()
		return nil, fmt.Errorf("x11: failed to send request: %w", err)
	}
	c.mu.Unlock()

	// Read responses until we get our reply
	for {
		reply, err := c.readResponse()
		if err != nil {
			return nil, err
		}

		// Check if this is our reply
		select {
		case data := <-replyCh:
			return data, nil
		default:
			// Not our reply, continue
			_ = reply
		}
	}
}

// readResponse reads a single response from the server.
func (c *Connection) readResponse() ([]byte, error) {
	// Read the first 32 bytes (fixed size for events/errors, base for replies)
	buf := make([]byte, 32)
	if _, err := c.conn.Read(buf); err != nil {
		return nil, fmt.Errorf("x11: failed to read response: %w", err)
	}

	responseType := buf[0]

	// Error (type 0)
	if responseType == 0 {
		// Parse error
		return nil, c.parseError(buf)
	}

	// Reply (type 1)
	if responseType == 1 {
		// Get additional data length
		d := NewDecoder(c.byteOrder, buf[4:8])
		additionalLen, _ := d.Uint32()

		if additionalLen > 0 {
			// Read additional data into a new combined buffer
			additional := make([]byte, additionalLen*4)
			totalRead := 0
			for totalRead < len(additional) {
				n, err := c.conn.Read(additional[totalRead:])
				if err != nil {
					return nil, fmt.Errorf("x11: failed to read reply data: %w", err)
				}
				totalRead += n
			}
			// Create new buffer with combined data (avoid appending to non-zero length slice)
			combined := make([]byte, 0, 32+len(additional))
			combined = append(combined, buf...)
			combined = append(combined, additional...)
			buf = combined
		}

		// Get sequence number and dispatch to waiting goroutine
		seqD := NewDecoder(c.byteOrder, buf[2:4])
		seq, _ := seqD.Uint16()

		c.pendingRepliesLock.Lock()
		ch, ok := c.pendingReplies[seq]
		if ok {
			delete(c.pendingReplies, seq)
		}
		c.pendingRepliesLock.Unlock()

		if ok {
			ch <- buf
		}

		return buf, nil
	}

	// Event (type 2-127)
	return buf, nil
}

// parseError parses an X11 error response.
func (c *Connection) parseError(buf []byte) error {
	d := NewDecoder(c.byteOrder, buf)
	_, _ = d.Uint8() // response type (0)
	errorCode, _ := d.Uint8()
	seq, _ := d.Uint16()
	resourceID, _ := d.Uint32()
	minorOpcode, _ := d.Uint16()
	majorOpcode, _ := d.Uint8()

	return fmt.Errorf("%w: code=%d seq=%d resource=%d major=%d minor=%d",
		ErrProtocolError, errorCode, seq, resourceID, majorOpcode, minorOpcode)
}

// Flush ensures all buffered data is sent to the server.
func (c *Connection) Flush() error {
	// Currently we send immediately, so this is a no-op
	return nil
}

// Sync performs a round-trip to ensure all requests have been processed.
func (c *Connection) Sync() error {
	// Send GetInputFocus request which always generates a reply
	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeGetInputFocus)
	e.PutUint8(0) // unused
	e.PutUint16(1) // length in 4-byte units

	_, err := c.sendRequestWithReply(e.Bytes())
	return err
}
