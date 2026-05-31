//go:build linux

// Minimal hand-written D-Bus session bus client.
//
// Covers exactly the wire shapes needed by org.freedesktop.portal.FileChooser
// (ADR-036) and reusable for future D-Bus portals (ADR-040).
//
// Wire format summary:
//
//	Fixed header (16 bytes): endian, type, flags, version, bodyLen, serial, hdrArrayLen
//	Header fields  (a(yv)): each field is a (code: byte, value: variant) struct (8-byte aligned)
//	Padding to 8-byte boundary
//	Body
//
// Auth: SASL EXTERNAL — send NUL + "AUTH EXTERNAL <hex-uid>\r\nBEGIN\r\n",
// expect "OK <guid>\r\n".
package platform

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// errPortalUnavailable is returned by portalOpenFile/portalSaveFile when the
// xdg-desktop-portal cannot be reached (connect, auth, or sendCall failed)
// before any dialog was shown.  showOpenFileDialog treats it as a signal to
// fall back to the subprocess backend.  Any other error means the portal was
// reachable and the dialog may have been visible; those errors are surfaced
// directly rather than triggering a fallback that would open a second dialog.
var errPortalUnavailable = errors.New("portal: unavailable")

// D-Bus message type codes (first byte of fixed header).
const (
	dbusMsgCall   byte = 1
	dbusMsgReturn byte = 2
	dbusMsgError  byte = 3
	dbusMsgSignal byte = 4
)

// D-Bus header field codes used in the a(yv) header fields array.
const (
	dbusFieldPath        byte = 1
	dbusFieldInterface   byte = 2
	dbusFieldMember      byte = 3
	dbusFieldReplySerial byte = 5
	dbusFieldDest        byte = 6
	dbusFieldSender      byte = 7
	dbusFieldSignature   byte = 8
)

// dbusSerial is a process-global counter used to generate unique portal request tokens.
var dbusSerial atomic.Uint32

// dbusMsg holds the decoded fields of an incoming D-Bus message.
type dbusMsg struct {
	Type      byte
	Serial    uint32
	ReplyTo   uint32 // REPLY_SERIAL header field (0 if absent)
	Path      string
	Interface string
	Member    string
	Sender    string
	Sig       string // body signature
	Body      []byte
}

// dbusConn is a D-Bus session bus connection that has completed SASL auth and
// the Hello handshake.  Close rw when done.
type dbusConn struct {
	rw     net.Conn
	serial uint32
	name   string // unique bus name assigned by Hello (e.g. ":1.42")
}

// dbusConnect opens the session bus socket, performs SASL EXTERNAL authentication,
// and sends the mandatory Hello method call to obtain our unique bus name.
func dbusConnect() (*dbusConn, error) {
	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr == "" {
		return nil, fmt.Errorf("dbus: DBUS_SESSION_BUS_ADDRESS not set")
	}

	raw, err := dbusDialAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("dbus: dial: %w", err)
	}

	c := &dbusConn{rw: raw}
	if err := c.auth(); err != nil {
		raw.Close()
		return nil, fmt.Errorf("dbus: auth: %w", err)
	}

	// Bound the Hello round-trip; clear the deadline so that the caller
	// (portal dialog) can set its own longer deadline via waitResponse.
	c.rw.SetDeadline(time.Now().Add(5 * time.Second))
	name, err := c.hello()
	c.rw.SetDeadline(time.Time{})
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("dbus: hello: %w", err)
	}
	if name == "" {
		raw.Close()
		return nil, fmt.Errorf("dbus: Hello returned empty unique name")
	}
	c.name = name
	return c, nil
}

// dbusDialAddr parses DBUS_SESSION_BUS_ADDRESS and opens the first reachable
// Unix transport.  Supports unix:path=... (regular socket) and
// unix:abstract=... (Linux abstract namespace, '\x00' prefix).
// Multiple addresses separated by ';' are tried in order.
func dbusDialAddr(addr string) (net.Conn, error) {
	for _, transport := range strings.Split(addr, ";") {
		if !strings.HasPrefix(transport, "unix:") {
			continue
		}
		params := dbusParseParams(transport[len("unix:"):])

		if path, ok := params["path"]; ok {
			if conn, err := net.Dial("unix", path); err == nil {
				return conn, nil
			}
		}
		if abstract, ok := params["abstract"]; ok {
			uaddr := &net.UnixAddr{Net: "unix", Name: "\x00" + abstract}
			if conn, err := net.DialUnix("unix", nil, uaddr); err == nil {
				return conn, nil
			}
		}
	}
	return nil, fmt.Errorf("dbus: unreachable bus address %q", addr)
}

// dbusParseParams splits a "key=value,key=value" D-Bus address parameter string
// into a map.  Used by dbusDialAddr to extract path/abstract/guid fields.
func dbusParseParams(s string) map[string]string {
	m := make(map[string]string)
	for _, kv := range strings.Split(s, ",") {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			m[kv[:idx]] = kv[idx+1:]
		}
	}
	return m
}

// auth performs SASL EXTERNAL authentication on a newly opened D-Bus connection.
// Protocol: "\x00AUTH EXTERNAL <hex-uid>\r\n" → "OK <guid>\r\n" → "BEGIN\r\n".
// The hex-uid is the ASCII representation of os.Getuid() encoded as hex bytes.
func (c *dbusConn) auth() error {
	uid := hex.EncodeToString([]byte(strconv.Itoa(os.Getuid())))
	if _, err := fmt.Fprintf(c.rw, "\x00AUTH EXTERNAL %s\r\n", uid); err != nil {
		return err
	}
	line, err := dbusReadLine(c.rw)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "OK ") {
		return fmt.Errorf("auth rejected: %s", line)
	}
	_, err = fmt.Fprint(c.rw, "BEGIN\r\n")
	return err
}

// hello sends org.freedesktop.DBus.Hello and reads messages until the matching
// METHOD_RETURN arrives.  Returns the unique bus name (e.g. ":1.42").
func (c *dbusConn) hello() (string, error) {
	serial, err := c.sendCall(
		"org.freedesktop.DBus",
		"/org/freedesktop/DBus",
		"org.freedesktop.DBus",
		"Hello",
		"", nil,
	)
	if err != nil {
		return "", err
	}
	for {
		msg, err := c.readMsg()
		if err != nil {
			return "", err
		}
		if msg.ReplyTo != serial {
			continue
		}
		if msg.Type == dbusMsgError {
			return "", fmt.Errorf("hello error from bus")
		}
		if msg.Type == dbusMsgReturn {
			d := newMsgDecoder(msg.Body, 0)
			return d.readStr()
		}
	}
}

// sendCall encodes a D-Bus METHOD_CALL message, writes it to the connection, and
// returns the serial number used (needed to match the eventual METHOD_RETURN).
func (c *dbusConn) sendCall(dest, path, iface, member, sig string, body []byte) (uint32, error) {
	c.serial++
	raw := dbusEncodeMsg(dbusMsgCall, c.serial, dest, path, iface, member, sig, body)
	_, err := c.rw.Write(raw)
	return c.serial, err
}

// waitResponse reads incoming messages and waits in two stages:
//  1. Discard all messages until the METHOD_RETURN (or METHOD_ERROR) for callSerial.
//     Any Response signal that arrives during this phase is buffered — the D-Bus spec
//     permits signals to be delivered before their triggering method return.
//  2. After the return is received, look for the org.freedesktop.portal.Request.Response
//     signal on handlePath (or return the signal buffered in phase 1).
//
// Returns nil, nil if the portal reports that the user canceled (response code 1).
// A 5-minute deadline guards against a hung or crashed portal daemon.
func (c *dbusConn) waitResponse(callSerial uint32, handlePath string) ([]string, error) {
	c.rw.SetDeadline(time.Now().Add(5 * time.Minute))
	var early *dbusMsg // Response signal buffered before METHOD_RETURN
	gotReturn := false
	for {
		msg, err := c.readMsg()
		if err != nil {
			return nil, fmt.Errorf("dbus: read: %w", err)
		}

		if !gotReturn {
			if msg.ReplyTo == callSerial {
				if msg.Type == dbusMsgError {
					return nil, fmt.Errorf("portal: method error")
				}
				if msg.Type == dbusMsgReturn {
					gotReturn = true
					if early != nil {
						return decodePortalResponse(early.Body)
					}
				}
			} else if msg.Type == dbusMsgSignal &&
				msg.Path == handlePath &&
				msg.Member == "Response" {
				early = msg
			}
			continue
		}

		if msg.Type == dbusMsgSignal &&
			msg.Path == handlePath &&
			msg.Member == "Response" {
			return decodePortalResponse(msg.Body)
		}
	}
}

// D-Bus message size limits — matches the default dbus-daemon caps.
const (
	dbusMaxHdrArrayLen = 1 << 20   // 1 MiB: header fields are tiny in practice
	dbusMaxBodyLen     = 128 << 20 // 128 MiB: D-Bus spec upper bound
)

// readMsg reads one complete D-Bus message from the connection.
// Layout: 16-byte fixed header, variable-length header fields, 8-byte-boundary
// padding, then the body.
func (c *dbusConn) readMsg() (*dbusMsg, error) {
	var fixed [16]byte
	if _, err := io.ReadFull(c.rw, fixed[:]); err != nil {
		return nil, err
	}
	if fixed[0] != 'l' {
		return nil, fmt.Errorf("dbus: big-endian messages not supported")
	}

	bodyLen := binary.LittleEndian.Uint32(fixed[4:8])
	serial := binary.LittleEndian.Uint32(fixed[8:12])
	hdrArrayLen := binary.LittleEndian.Uint32(fixed[12:16])

	// Guard against malformed messages that would cause multi-GiB allocations.
	if hdrArrayLen > dbusMaxHdrArrayLen {
		return nil, fmt.Errorf("dbus: header array length %d exceeds limit (%d)", hdrArrayLen, dbusMaxHdrArrayLen)
	}
	if bodyLen > dbusMaxBodyLen {
		return nil, fmt.Errorf("dbus: body length %d exceeds limit (%d)", bodyLen, dbusMaxBodyLen)
	}

	hdrData := make([]byte, hdrArrayLen)
	if _, err := io.ReadFull(c.rw, hdrData); err != nil {
		return nil, err
	}

	// Skip padding so that the body starts on an 8-byte boundary.
	if pad := (8 - (16+int(hdrArrayLen))%8) % 8; pad > 0 {
		if _, err := io.ReadFull(c.rw, make([]byte, pad)); err != nil {
			return nil, err
		}
	}

	var body []byte
	if bodyLen > 0 {
		body = make([]byte, bodyLen)
		if _, err := io.ReadFull(c.rw, body); err != nil {
			return nil, err
		}
	}

	msg := &dbusMsg{Type: fixed[1], Serial: serial, Body: body}
	dbusParseHdrFields(hdrData, msg)
	return msg, nil
}

// dbusParseHdrFields decodes the a(yv) header fields array and populates msg.
// hdrData starts at absolute message offset 16 (immediately after the fixed header).
func dbusParseHdrFields(hdrData []byte, msg *dbusMsg) {
	d := newMsgDecoder(hdrData, 16)
	for d.pos < len(d.data) {
		if err := d.alignTo(8); err != nil {
			break
		}
		if d.pos >= len(d.data) {
			break
		}
		code, err := d.readU8()
		if err != nil {
			break
		}
		typeSig, err := d.readSig()
		if err != nil {
			break
		}
		switch typeSig {
		case "s", "o":
			s, err := d.readStr()
			if err != nil {
				break
			}
			switch code {
			case dbusFieldPath:
				msg.Path = s
			case dbusFieldInterface:
				msg.Interface = s
			case dbusFieldMember:
				msg.Member = s
			case dbusFieldSender:
				msg.Sender = s
			}
		case "u":
			v, err := d.readU32()
			if err != nil {
				break
			}
			if code == dbusFieldReplySerial {
				msg.ReplyTo = v
			}
		case "g":
			g, err := d.readSig()
			if err != nil {
				break
			}
			if code == dbusFieldSignature {
				msg.Sig = g
			}
		default:
			_ = d.skipValue(typeSig)
		}
	}
}

// D-Bus message encoder (msgBuf)

// msgBuf accumulates bytes for a D-Bus message section while tracking the
// absolute byte position within the enclosing message.  The absolute position
// is required for correct D-Bus alignment (the spec measures alignment from the
// start of the message, not from the start of the current section).
type msgBuf struct {
	data []byte
	pos  int // absolute byte position in the message (used for alignment arithmetic)
}

// newMsgBuf creates a msgBuf whose absolute base offset is base.
// Use base=16 for header fields (they start at offset 16 in the message).
// Use base=0 for the body (the body section is always 8-byte aligned, so offset
// 0 satisfies every D-Bus alignment requirement).
func newMsgBuf(base int) *msgBuf { return &msgBuf{pos: base} }

// padTo inserts zero-padding bytes until the absolute position is a multiple of align.
func (b *msgBuf) padTo(align int) {
	if align <= 1 {
		return
	}
	n := (align - b.pos%align) % align
	for range n {
		b.data = append(b.data, 0)
		b.pos++
	}
}

// u8 appends a single byte without alignment.
func (b *msgBuf) u8(v byte) {
	b.data = append(b.data, v)
	b.pos++
}

// u32 aligns to 4 bytes then appends a little-endian uint32.
func (b *msgBuf) u32(v uint32) {
	b.padTo(4)
	b.data = binary.LittleEndian.AppendUint32(b.data, v)
	b.pos += 4
}

// str encodes a D-Bus string (type s) or object path (type o):
// u32 byte-length, UTF-8 content, NUL terminator.  No padding after NUL.
func (b *msgBuf) str(v string) {
	b.u32(uint32(len(v)))
	b.data = append(b.data, v...)
	b.data = append(b.data, 0)
	b.pos += len(v) + 1
}

// sig encodes a D-Bus signature (type g):
// u8 length, ASCII content, NUL terminator.  No alignment or padding.
func (b *msgBuf) sig(v string) {
	b.u8(byte(len(v)))
	b.data = append(b.data, v...)
	b.data = append(b.data, 0)
	b.pos += len(v) + 1
}

// bool32 encodes a D-Bus boolean as a 4-byte aligned uint32 (0 or 1).
func (b *msgBuf) bool32(v bool) {
	if v {
		b.u32(1)
	} else {
		b.u32(0)
	}
}

// variantStr encodes a D-Bus variant v(s): signature "s" followed by the string value.
func (b *msgBuf) variantStr(v string) { b.sig("s"); b.str(v) }

// variantBool encodes a D-Bus variant v(b): signature "b" followed by the bool value.
func (b *msgBuf) variantBool(v bool) { b.sig("b"); b.bool32(v) }

// variantByteArray encodes a D-Bus variant v(ay): signature "ay" followed by the byte slice.
func (b *msgBuf) variantByteArray(v []byte) {
	b.sig("ay")
	lp, cp := b.arrayStart(1)
	b.data = append(b.data, v...)
	b.pos += len(v)
	b.arrayEnd(lp, cp)
}

// arrayStart writes a zeroed array length placeholder (u32), then pads to
// elemAlign for the first element.
// Returns (lenFieldPos, contentPos): positions in b.data for use by arrayEnd.
func (b *msgBuf) arrayStart(elemAlign int) (lenFieldPos, contentPos int) {
	b.padTo(4)
	lenFieldPos = len(b.data)
	b.u32(0) // placeholder; patched by arrayEnd
	b.padTo(elemAlign)
	contentPos = len(b.data)
	return
}

// arrayEnd patches the uint32 length field written by arrayStart with the
// actual number of content bytes (len(b.data) - contentPos).
func (b *msgBuf) arrayEnd(lenFieldPos, contentPos int) {
	binary.LittleEndian.PutUint32(b.data[lenFieldPos:], uint32(len(b.data)-contentPos))
}

// dbusEncodeMsg assembles a complete little-endian D-Bus message.
// The fixed 16-byte header, variable header fields, 8-byte-boundary padding,
// and body are concatenated into a single slice ready to write to the socket.
func dbusEncodeMsg(msgType byte, serial uint32, dest, path, iface, member, bodySig string, body []byte) []byte {
	hdr := newMsgBuf(16) // header fields start at absolute offset 16
	dbusWriteHdrField(hdr, dbusFieldPath, "o", func() { hdr.str(path) })
	if iface != "" {
		dbusWriteHdrField(hdr, dbusFieldInterface, "s", func() { hdr.str(iface) })
	}
	dbusWriteHdrField(hdr, dbusFieldMember, "s", func() { hdr.str(member) })
	dbusWriteHdrField(hdr, dbusFieldDest, "s", func() { hdr.str(dest) })
	if bodySig != "" {
		dbusWriteHdrField(hdr, dbusFieldSignature, "g", func() { hdr.sig(bodySig) })
	}
	hdrBytes := hdr.data

	var fixed [16]byte
	fixed[0] = 'l'
	fixed[1] = msgType
	fixed[2] = 0 // flags
	fixed[3] = 1 // protocol version
	binary.LittleEndian.PutUint32(fixed[4:], uint32(len(body)))
	binary.LittleEndian.PutUint32(fixed[8:], serial)
	binary.LittleEndian.PutUint32(fixed[12:], uint32(len(hdrBytes)))

	totalHdr := 16 + len(hdrBytes)
	padLen := (8 - totalHdr%8) % 8

	out := make([]byte, 0, totalHdr+padLen+len(body))
	out = append(out, fixed[:]...)
	out = append(out, hdrBytes...)
	out = append(out, make([]byte, padLen)...)
	out = append(out, body...)
	return out
}

// dbusWriteHdrField writes one (yv) header field struct into b.
// Each struct must be 8-byte aligned (D-Bus struct alignment rule).
func dbusWriteHdrField(b *msgBuf, code byte, typeSig string, writeVal func()) {
	b.padTo(8) // struct (yv) requires 8-byte alignment
	b.u8(code)
	b.sig(typeSig) // variant type signature
	writeVal()
}

// D-Bus message decoder (msgDecoder)

// msgDecoder reads fields from a D-Bus byte slice while tracking the absolute
// message offset for D-Bus alignment arithmetic.
type msgDecoder struct {
	data []byte
	pos  int // read cursor relative to data[0]
	base int // absolute offset of data[0] within the message
}

// newMsgDecoder creates a decoder for data whose first byte sits at absolute
// message offset base.  Use base=16 for header fields, base=0 for the body.
func newMsgDecoder(data []byte, base int) *msgDecoder {
	return &msgDecoder{data: data, base: base}
}

// absPos returns the current absolute position within the message.
func (d *msgDecoder) absPos() int { return d.base + d.pos }

// alignTo advances the read cursor to the next multiple of n from the absolute
// position.  Returns io.ErrUnexpectedEOF if the required padding exceeds the buffer.
func (d *msgDecoder) alignTo(n int) error {
	if n <= 1 {
		return nil
	}
	rem := d.absPos() % n
	if rem == 0 {
		return nil
	}
	skip := n - rem
	if d.pos+skip > len(d.data) {
		return io.ErrUnexpectedEOF
	}
	d.pos += skip
	return nil
}

// readU8 reads one byte without alignment.
func (d *msgDecoder) readU8() (byte, error) {
	if d.pos >= len(d.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := d.data[d.pos]
	d.pos++
	return v, nil
}

// readU32 aligns to 4 bytes then reads a little-endian uint32.
func (d *msgDecoder) readU32() (uint32, error) {
	if err := d.alignTo(4); err != nil {
		return 0, err
	}
	if d.pos+4 > len(d.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(d.data[d.pos:])
	d.pos += 4
	return v, nil
}

// readStr decodes a D-Bus string (type s) or object path (type o):
// 4-byte-aligned u32 length, UTF-8 bytes, NUL terminator.
func (d *msgDecoder) readStr() (string, error) {
	n, err := d.readU32()
	if err != nil {
		return "", err
	}
	if d.pos+int(n)+1 > len(d.data) {
		return "", io.ErrUnexpectedEOF
	}
	s := string(d.data[d.pos : d.pos+int(n)])
	d.pos += int(n) + 1 // +1 skips the NUL terminator
	return s, nil
}

// readSig decodes a D-Bus signature (type g):
// u8 length, ASCII bytes, NUL terminator.  No alignment.
func (d *msgDecoder) readSig() (string, error) {
	n, err := d.readU8()
	if err != nil {
		return "", err
	}
	if d.pos+int(n)+1 > len(d.data) {
		return "", io.ErrUnexpectedEOF
	}
	s := string(d.data[d.pos : d.pos+int(n)])
	d.pos += int(n) + 1
	return s, nil
}

// skipValue advances the read cursor past one D-Bus value whose type is described
// by sig.  Used to skip unrecognized keys in the portal response a{sv} dict.
func (d *msgDecoder) skipValue(sig string) error {
	if sig == "" {
		return nil
	}
	switch sig[0] {
	case 'y':
		_, err := d.readU8()
		return err
	case 'b', 'u', 'i':
		_, err := d.readU32()
		return err
	case 'n', 'q':
		return d.skipFixed(2)
	case 'x', 't', 'd':
		return d.skipFixed(8)
	case 's', 'o':
		_, err := d.readStr()
		return err
	case 'g':
		_, err := d.readSig()
		return err
	case 'a':
		return d.skipArray(sig)
	case 'v':
		return d.skipVariant()
	case '(':
		return d.skipStruct(sig)
	case '{':
		return d.skipDictEntry(sig)
	}
	return fmt.Errorf("dbus: unknown type %q in skip", string(sig[0]))
}

// skipFixed aligns to size bytes then advances the cursor by size bytes.
func (d *msgDecoder) skipFixed(size int) error {
	if err := d.alignTo(size); err != nil {
		return err
	}
	if d.pos+size > len(d.data) {
		return io.ErrUnexpectedEOF
	}
	d.pos += size
	return nil
}

// skipArray reads the array length u32, skips the alignment gap, then advances
// past the content bytes.  Per the D-Bus spec the length excludes the alignment
// padding between the length field and the first element.
func (d *msgDecoder) skipArray(sig string) error {
	n, err := d.readU32()
	if err != nil {
		return err
	}
	elemAlign := 1
	if len(sig) > 1 {
		elemAlign = dbusTypeAlign(sig[1])
	}
	if err := d.alignTo(elemAlign); err != nil {
		return err
	}
	if d.pos+int(n) > len(d.data) {
		return io.ErrUnexpectedEOF
	}
	d.pos += int(n)
	return nil
}

// skipVariant reads the variant's type signature and then skips the value.
func (d *msgDecoder) skipVariant() error {
	vsig, err := d.readSig()
	if err != nil {
		return err
	}
	return d.skipValue(vsig)
}

// skipStruct aligns to 8 bytes (struct alignment) then skips each field in the
// struct by parsing the inner signature types.
func (d *msgDecoder) skipStruct(sig string) error {
	if err := d.alignTo(8); err != nil {
		return err
	}
	inner, _ := dbusFindMatchingParen(sig)
	for inner != "" {
		n, err := dbusSigTypeLen(inner)
		if err != nil {
			return err
		}
		if err := d.skipValue(inner[:n]); err != nil {
			return err
		}
		inner = inner[n:]
	}
	return nil
}

// skipDictEntry aligns to 8 bytes (dict entry is a struct) then skips the key
// and value fields.
func (d *msgDecoder) skipDictEntry(sig string) error {
	if err := d.alignTo(8); err != nil {
		return err
	}
	if len(sig) < 4 {
		return fmt.Errorf("dbus: truncated dict entry sig")
	}
	if err := d.skipValue(sig[1:2]); err != nil { // key type
		return err
	}
	return d.skipValue(sig[2 : len(sig)-1]) // value type(s)
}

// --- signature helpers ---

// dbusFindMatchingParen returns the content between the outermost "(" and ")" in
// sig, plus the total number of signature characters consumed (including the parens).
func dbusFindMatchingParen(sig string) (inner string, total int) {
	depth := 0
	for i, c := range sig {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return sig[1:i], i + 1
			}
		}
	}
	return sig[1:], len(sig)
}

// dbusSigTypeLen returns the number of signature characters that make up the first
// complete type in sig.  Examples: "u" → 1, "a(us)" → 5, "(sa(us))" → 8.
func dbusSigTypeLen(sig string) (int, error) {
	if sig == "" {
		return 0, nil
	}
	switch sig[0] {
	case 'a':
		if len(sig) < 2 {
			return 0, fmt.Errorf("dbus: truncated array sig")
		}
		n, err := dbusSigTypeLen(sig[1:])
		return 1 + n, err
	case '(':
		_, n := dbusFindMatchingParen(sig)
		return n, nil
	case '{':
		for i, c := range sig {
			if c == '}' {
				return i + 1, nil
			}
		}
		return len(sig), nil
	default:
		return 1, nil
	}
}

// dbusTypeAlign returns the D-Bus alignment requirement (in bytes) for the type
// whose first signature character is c.
func dbusTypeAlign(c byte) int {
	switch c {
	case 'y', 'g':
		return 1
	case 'n', 'q':
		return 2
	case 'b', 'u', 'i', 's', 'o', 'a', 'h':
		return 4
	case 'x', 't', 'd':
		return 8
	case '(', '{':
		return 8
	case 'v':
		return 1 // variant alignment is 1 per D-Bus spec (not 8)
	}
	return 4
}

// Portal response decoder

// decodePortalResponse decodes the ua{sv} body of an
// org.freedesktop.portal.Request.Response signal.
// Response code 0 = success, 1 = user canceled, 2 = other error.
// Returns nil, nil on cancellation.
func decodePortalResponse(body []byte) ([]string, error) {
	d := newMsgDecoder(body, 0)

	code, err := d.readU32()
	if err != nil {
		return nil, fmt.Errorf("dbus: decode response code: %w", err)
	}
	if code == 1 { // user canceled
		return nil, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("dbus: portal response code %d", code)
	}

	uris, err := decodeURIsFromDict(d)
	if err != nil {
		return nil, fmt.Errorf("dbus: decode uris: %w", err)
	}

	paths := make([]string, 0, len(uris))
	for _, uri := range uris {
		if p, ok := dbusURIToPath(uri); ok {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	return paths, nil
}

// decodeURIsFromDict iterates a D-Bus a{sv} dictionary and extracts the "uris"
// key, which the portal returns as a variant containing an array of strings (as).
func decodeURIsFromDict(d *msgDecoder) ([]string, error) {
	arrayLen, err := d.readU32()
	if err != nil {
		return nil, err
	}
	if err := d.alignTo(8); err != nil { // {sv} entries are struct-aligned (8)
		return nil, err
	}
	end := d.pos + int(arrayLen)

	var uris []string
	for d.pos < end {
		if err := d.alignTo(8); err != nil {
			return nil, err
		}
		key, err := d.readStr()
		if err != nil {
			return nil, err
		}
		vsig, err := d.readSig()
		if err != nil {
			return nil, err
		}
		if key == "uris" && vsig == "as" {
			uris, err = decodeStringArray(d)
			if err != nil {
				return nil, err
			}
		} else {
			if err := d.skipValue(vsig); err != nil {
				return nil, err
			}
		}
	}
	return uris, nil
}

// decodeStringArray reads a D-Bus array of strings (as) and returns them as a
// Go string slice.
func decodeStringArray(d *msgDecoder) ([]string, error) {
	n, err := d.readU32()
	if err != nil {
		return nil, err
	}
	end := d.pos + int(n) // string elements are 4-byte aligned; readU32 already aligned
	var result []string
	for d.pos < end {
		s, err := d.readStr()
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

// dbusURIToPath converts a file:// URI to an absolute filesystem path.
// Percent-encoded characters (e.g. %20 for space) are decoded via url.PathUnescape.
// Returns ("", false) for non-file:// URIs.
func dbusURIToPath(uri string) (string, bool) {
	const prefix = "file://"
	if !strings.HasPrefix(uri, prefix) {
		return "", false
	}
	path, err := url.PathUnescape(uri[len(prefix):])
	if err != nil {
		return uri[len(prefix):], true // return raw path on decode error
	}
	return path, true
}

// Misc D-Bus utilities

// dbusReadLine reads bytes from r until a '\n' and returns the line without
// the trailing '\r\n'.  Used exclusively during SASL authentication.
// Returns an error if the line exceeds 4096 bytes (guard against malformed response).
func dbusReadLine(r io.Reader) (string, error) {
	var buf []byte
	b := [1]byte{}
	for {
		if _, err := r.Read(b[:]); err != nil {
			return "", err
		}
		if b[0] == '\n' {
			return strings.TrimRight(string(buf), "\r"), nil
		}
		buf = append(buf, b[0])
		if len(buf) > 4096 {
			return "", fmt.Errorf("dbus: auth response line too long")
		}
	}
}

// dbusNewToken returns a process-unique ASCII token string suitable for use as
// the portal "handle_token" option.  Format: "gogpu_<counter>".
func dbusNewToken() string {
	return "gogpu_" + strconv.FormatUint(uint64(dbusSerial.Add(1)), 10)
}

// dbusHandlePath computes the xdg-desktop-portal request object path from the
// caller's unique bus name and the handle token.
// Per the portal spec: remove the leading ':' from the sender name and replace
// '.' with '_'.  Example: ":1.42" + "gogpu_1" →
// "/org/freedesktop/portal/desktop/request/1_42/gogpu_1".
func dbusHandlePath(sender, token string) string {
	escaped := strings.TrimPrefix(sender, ":")
	escaped = strings.ReplaceAll(escaped, ".", "_")
	return "/org/freedesktop/portal/desktop/request/" + escaped + "/" + token
}
