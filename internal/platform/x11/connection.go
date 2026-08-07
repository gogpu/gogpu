//go:build linux

package x11

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Connection errors.
var (
	ErrNotConnected     = errors.New("x11: not connected")
	ErrConnectionClosed = errors.New("x11: connection closed")
	ErrNoDisplay        = errors.New("x11: DISPLAY not set")
	ErrInvalidDisplay   = errors.New("x11: invalid DISPLAY format")
	ErrProtocolError    = errors.New("x11: protocol error")
)

// Connection represents a connection to an X11 server.
type eventQueue struct {
	mu     sync.Mutex
	events []Event
	signal chan struct{}
}

func newEventQueue() *eventQueue {
	return &eventQueue{
		signal: make(chan struct{}, 1),
	}
}

func (q *eventQueue) push(e Event) {
	q.mu.Lock()
	q.events = append(q.events, e)
	q.mu.Unlock()
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

func (q *eventQueue) pop() Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.events) == 0 {
		return nil
	}
	e := q.events[0]
	q.events[0] = nil
	q.events = q.events[1:]
	if len(q.events) > 0 {
		select {
		case q.signal <- struct{}{}:
		default:
		}
	}
	return e
}

// Connection represents a connection to an X11 server.
type Connection struct {
	conn   net.Conn
	reader *bufio.Reader

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
	mu      sync.Mutex
	closed  bool
	writeMu sync.Mutex
	replyMu sync.Mutex
	done    chan struct{}

	// Atom cache
	atomCache     map[string]Atom
	atomCacheLock sync.RWMutex

	// Screen number
	screenNum int

	// Pending replies
	pendingReplies map[uint16]chan []byte

	// Extension registry (major opcode → extension name)
	extensions     map[string]*ExtensionInfo
	extensionsLock sync.RWMutex

	// Event queue
	eventQ *eventQueue
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
		reader:         bufio.NewReader(conn),
		byteOrder:      LSBFirst,
		atomCache:      make(map[string]Atom),
		screenNum:      screenNum,
		pendingReplies: make(map[uint16]chan []byte),
		extensions:     make(map[string]*ExtensionInfo),
		eventQ:         newEventQueue(),
		done:           make(chan struct{}),
	}

	// Perform connection setup
	if err := c.performSetup(strconv.Itoa(displayNum)); err != nil {
		_ = conn.Close()
		return nil, err
	}

	go c.readerLoop()

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
func (c *Connection) performSetup(displayNum string) error {
	// Extract binary address from connected socket (libxcb getpeername pattern)
	family, address := connAddress(c.conn)

	// Get authentication data
	authName, authData, err := getAuth(family, address, displayNum)
	if err != nil {
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
	if _, err := io.ReadFull(c.reader, initialBuf); err != nil {
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
		if _, err := io.ReadFull(c.reader, additionalBuf); err != nil {
			return fmt.Errorf("x11: failed to read setup error data: %w", err)
		}

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
	if _, err := io.ReadFull(c.reader, remainingBuf); err != nil {
		return fmt.Errorf("x11: failed to read setup data: %w", err)
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
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.done)

	// Close pending reply channels
	c.replyMu.Lock()
	for _, ch := range c.pendingReplies {
		close(ch)
	}
	c.pendingReplies = nil
	c.replyMu.Unlock()

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
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return 0, ErrConnectionClosed
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	seq := c.getNextSeq()
	if _, err := c.conn.Write(data); err != nil {
		return 0, fmt.Errorf("x11: failed to send request: %w", err)
	}

	return seq, nil
}

// sendRequestWithReply sends a request and waits for a reply.
func (c *Connection) sendRequestWithReply(data []byte) ([]byte, error) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return nil, ErrConnectionClosed
	}

	ch := make(chan []byte, 1)

	c.writeMu.Lock()
	seq := c.getNextSeq()
	c.replyMu.Lock()
	c.pendingReplies[seq] = ch
	c.replyMu.Unlock()

	if _, err := c.conn.Write(data); err != nil {
		c.replyMu.Lock()
		delete(c.pendingReplies, seq)
		c.replyMu.Unlock()
		c.writeMu.Unlock()
		return nil, fmt.Errorf("x11: failed to send request: %w", err)
	}
	c.writeMu.Unlock()

	select {
	case reply := <-ch:
		if reply[0] == 0 {
			return nil, c.parseError(reply)
		}
		return reply, nil
	case <-c.done:
		return nil, ErrConnectionClosed
	}
}

func (c *Connection) seqFromHeader(buf []byte) uint16 {
	if c.byteOrder == MSBFirst {
		return binary.BigEndian.Uint16(buf[2:4])
	}
	return binary.LittleEndian.Uint16(buf[2:4])
}

func (c *Connection) dispatchReply(seq uint16, data []byte) bool {
	c.replyMu.Lock()
	ch, ok := c.pendingReplies[seq]
	if ok {
		delete(c.pendingReplies, seq)
	}
	c.replyMu.Unlock()
	if ok {
		ch <- data
	}
	return ok
}

// readerLoop runs in its own goroutine and exclusively reads from the socket,
// demultiplexing events, replies, and errors to their respective destinations.
func (c *Connection) readerLoop() {
	defer c.Close()

	var demuxedEvents, demuxedReplies, demuxedErrors uint64

	defer func() {
		slog.Debug("x11: connection reader loop exited",
			"events", demuxedEvents,
			"replies", demuxedReplies,
			"errors", demuxedErrors)
	}()

	for {
		buf := make([]byte, 32)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return
		}

		responseType := buf[0]

		switch {
		case responseType == 0: // Error (type 0)
			demuxedErrors++
			seq := c.seqFromHeader(buf)
			if !c.dispatchReply(seq, buf) {
				slog.Warn("x11: asynchronous error", "err", c.parseError(buf))
			}

		case responseType == 1: // Reply (type 1)
			demuxedReplies++
			seq := c.seqFromHeader(buf)
			additional, err := c.readAdditional(buf)
			if err != nil {
				return
			}
			c.dispatchReply(seq, additional)

		case responseType&0x7F == EventGenericEvent:
			var err error
			buf, err = c.readAdditional(buf)
			if err != nil {
				return
			}
			fallthrough

		default:
			// Event
			event, err := c.parseEvent(buf)
			if err == nil && event != nil {
				demuxedEvents++
				c.eventQ.push(event)
			}
		}

		total := demuxedEvents + demuxedReplies + demuxedErrors
		if total%1000 == 0 {
			slog.Debug("x11: connection demux stats", "events", demuxedEvents, "replies", demuxedReplies, "errors", demuxedErrors)
		}
	}
}

// readAdditional reads additional data from the connection based on the length
// field at offset 4 of the 32-byte header. Returns the extended buffer if
// additional data was present, or the original buffer otherwise.
func (c *Connection) readAdditional(buf []byte) ([]byte, error) {
	d := NewDecoder(c.byteOrder, buf[4:8])
	additionalLen, _ := d.Uint32()
	if additionalLen == 0 {
		return buf, nil
	}

	additional := make([]byte, additionalLen*4)
	if _, err := io.ReadFull(c.reader, additional); err != nil {
		return nil, err
	}

	combined := make([]byte, 0, 32+len(additional))
	combined = append(combined, buf...)
	combined = append(combined, additional...)
	return combined, nil
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

// QueryExtension queries the X server for an extension by name.
// Returns ExtensionInfo with Present=false if the extension is not available.
// Results are cached so subsequent calls for the same extension are fast.
func (c *Connection) QueryExtension(name string) (*ExtensionInfo, error) {
	// Check cache first
	c.extensionsLock.RLock()
	if ext, ok := c.extensions[name]; ok {
		c.extensionsLock.RUnlock()
		return ext, nil
	}
	c.extensionsLock.RUnlock()

	// Build QueryExtension request
	nameBytes := []byte(name)
	nameLen := len(nameBytes)
	padLen := pad(nameLen)
	reqLen := requestLength(8 + nameLen + padLen)

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeQueryExtension)
	e.PutUint8(0) // unused
	e.PutUint16(reqLen)
	e.PutUint16(uint16(nameLen))
	e.PutUint16(0) // unused
	e.PutBytes(nameBytes)
	e.PutPad()

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return nil, fmt.Errorf("x11: QueryExtension(%q) failed: %w", name, err)
	}

	if len(reply) < 32 {
		return nil, fmt.Errorf("x11: QueryExtension(%q) reply too short", name)
	}

	ext := &ExtensionInfo{
		Present:     reply[8] != 0,
		MajorOpcode: reply[9],
		FirstEvent:  reply[10],
		FirstError:  reply[11],
	}

	// Cache the result
	c.extensionsLock.Lock()
	c.extensions[name] = ext
	c.extensionsLock.Unlock()

	return ext, nil
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
	e.PutUint8(0)  // unused
	e.PutUint16(1) // length in 4-byte units

	_, err := c.sendRequestWithReply(e.Bytes())
	return err
}
