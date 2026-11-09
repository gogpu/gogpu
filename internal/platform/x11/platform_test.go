//go:build linux

package x11

import (
	"bytes"
	"net"
	"testing"
)

// writtenConn records what is written to it.
type writtenConn struct {
	net.Conn
	written bytes.Buffer
}

func (c *writtenConn) Write(b []byte) (int, error) { return c.written.Write(b) }

func TestDestroyWindow_DestroysTheWindowAndKeepsTheConnection(t *testing.T) {
	written := &writtenConn{}
	window := &x11Window{window: 0x00400001}
	p := &Platform{
		conn:    &Connection{conn: written, byteOrder: LSBFirst},
		primary: window,
		windows: map[ResourceID]*x11Window{window.window: window},
	}

	p.DestroyWindow()
	p.DestroyWindow()

	want := []byte{
		4, 0, 2, 0, // DestroyWindow opcode, unused, length
		0x01, 0x00, 0x40, 0x00, // window
	}
	if !bytes.Equal(written.written.Bytes(), want) {
		t.Errorf("requests sent by two DestroyWindow calls: %v, want one DestroyWindow %v", written.written.Bytes(), want)
	}
	if p.conn == nil || len(p.windows) != 0 || p.PrimaryWindow() != 0 {
		t.Errorf("after DestroyWindow: connection kept %v, %d windows registered, window %#x; want true, 0, 0",
			p.conn != nil, len(p.windows), p.PrimaryWindow())
	}
	if _, err := p.ClipboardRead(); err == nil {
		t.Error("ClipboardRead with no window open: got no error")
	}
}
