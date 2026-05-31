//go:build linux

package platform

import (
	"bytes"
	"encoding/binary"
	"net"
	"reflect"
	"slices"
	"testing"
)

func TestDBusHandlePath(t *testing.T) {
	tests := []struct {
		sender string
		token  string
		want   string
	}{
		{
			sender: ":1.42",
			token:  "gogpu_1",
			want:   "/org/freedesktop/portal/desktop/request/1_42/gogpu_1",
		},
		{
			sender: ":1.123",
			token:  "gogpu_2",
			want:   "/org/freedesktop/portal/desktop/request/1_123/gogpu_2",
		},
		{
			sender: ":2.0",
			token:  "mytoken",
			want:   "/org/freedesktop/portal/desktop/request/2_0/mytoken",
		},
	}

	for _, tt := range tests {
		got := dbusHandlePath(tt.sender, tt.token)
		if got != tt.want {
			t.Errorf("dbusHandlePath(%q, %q) = %q, want %q", tt.sender, tt.token, got, tt.want)
		}
	}
}

func TestDBusParseParams(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]string
	}{
		{
			input: "path=/run/user/1000/bus",
			want:  map[string]string{"path": "/run/user/1000/bus"},
		},
		{
			input: "abstract=/tmp/dbus-abc,guid=1234",
			want:  map[string]string{"abstract": "/tmp/dbus-abc", "guid": "1234"},
		},
		{
			input: "",
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		got := dbusParseParams(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("dbusParseParams(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDBusURIToPath(t *testing.T) {
	tests := []struct {
		uri    string
		want   string
		wantOK bool
	}{
		{"file:///home/user/file.txt", "/home/user/file.txt", true},
		{"file:///tmp/my%20file.txt", "/tmp/my file.txt", true},
		{"file:///path/to/%C3%A9t%C3%A9.txt", "/path/to/été.txt", true},
		{"https://example.com/file.txt", "", false},
		{"file://", "", true},
	}

	for _, tt := range tests {
		got, ok := dbusURIToPath(tt.uri)
		if ok != tt.wantOK {
			t.Errorf("dbusURIToPath(%q) ok = %v, want %v", tt.uri, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("dbusURIToPath(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestMsgBufAlignment(t *testing.T) {
	b := newMsgBuf(0)

	// Write byte at 0 → pos 1
	b.u8(0xAA)
	if b.pos != 1 {
		t.Fatalf("pos after u8: got %d, want 1", b.pos)
	}

	// Write u32 at pos 1 → align to 4, pad 3 bytes → write at pos 4
	b.u32(0xDEADBEEF)
	if b.pos != 8 {
		t.Fatalf("pos after u32: got %d, want 8", b.pos)
	}
	if b.data[1] != 0 || b.data[2] != 0 || b.data[3] != 0 {
		t.Error("expected 3 padding zero bytes before u32")
	}
	if binary.LittleEndian.Uint32(b.data[4:8]) != 0xDEADBEEF {
		t.Error("u32 value not written correctly")
	}
}

func TestMsgBufStr(t *testing.T) {
	b := newMsgBuf(0)
	b.str("hello")

	// u32(5) = 4 bytes + "hello"(5) + NUL(1) = 10 bytes
	if len(b.data) != 10 {
		t.Fatalf("str encoding length: got %d, want 10", len(b.data))
	}
	if binary.LittleEndian.Uint32(b.data[0:4]) != 5 {
		t.Error("str: length field incorrect")
	}
	if string(b.data[4:9]) != "hello" {
		t.Error("str: content incorrect")
	}
	if b.data[9] != 0 {
		t.Error("str: missing NUL terminator")
	}
}

func TestMsgBufSig(t *testing.T) {
	b := newMsgBuf(0)
	b.sig("ssa{sv}")

	// u8(7) + "ssa{sv}"(7) + NUL(1) = 9 bytes
	if len(b.data) != 9 {
		t.Fatalf("sig encoding length: got %d, want 9", len(b.data))
	}
	if b.data[0] != 7 {
		t.Error("sig: length byte incorrect")
	}
	if string(b.data[1:8]) != "ssa{sv}" {
		t.Error("sig: content incorrect")
	}
	if b.data[8] != 0 {
		t.Error("sig: missing NUL terminator")
	}
}

func TestMsgBufArrayRoundTrip(t *testing.T) {
	b := newMsgBuf(0)
	lp, cp := b.arrayStart(4) // array of u32
	b.u32(1)
	b.u32(2)
	b.arrayEnd(lp, cp)

	// Decode: u32 length = 8, then u32(1), u32(2)
	d := newMsgDecoder(b.data, 0)
	n, err := d.readU32()
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Errorf("array length: got %d, want 8", n)
	}
	v1, _ := d.readU32()
	v2, _ := d.readU32()
	if v1 != 1 || v2 != 2 {
		t.Errorf("array values: got %d %d, want 1 2", v1, v2)
	}
}

func TestMsgBufVariantByteArray(t *testing.T) {
	b := newMsgBuf(0)
	b.variantByteArray([]byte("/home/user\x00"))

	// sig("ay") = u8(2) + "ay" + NUL = 4 bytes
	// array: u32(len) + bytes
	if len(b.data) < 4 {
		t.Fatalf("variantByteArray too short: %d bytes", len(b.data))
	}
	// First byte is signature length = 2
	if b.data[0] != 2 {
		t.Errorf("variant sig length: got %d, want 2", b.data[0])
	}
	if string(b.data[1:3]) != "ay" {
		t.Errorf("variant sig: got %q, want %q", string(b.data[1:3]), "ay")
	}
	// Content must contain the path bytes
	if !bytes.Contains(b.data, []byte("/home/user\x00")) {
		t.Error("variantByteArray: path bytes not found in output")
	}
}

// TestDecodePortalResponse_Cancel verifies response code 1 returns nil, nil.
func TestDecodePortalResponse_Cancel(t *testing.T) {
	// Build body: u32(1) + empty a{sv}
	b := newMsgBuf(0)
	b.u32(1) // user canceled
	lp, cp := b.arrayStart(8)
	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil on cancel, got %v", paths)
	}
}

// TestDecodePortalResponse_Success verifies URI extraction from a success response.
func TestDecodePortalResponse_Success(t *testing.T) {
	// Build body: u32(0) + a{sv} with "uris" -> v(as) [file:///tmp/test.txt]
	b := newMsgBuf(0)
	b.u32(0) // success

	// a{sv}
	lp, cp := b.arrayStart(8)

	// dict entry: "uris" -> v(as)
	b.padTo(8)
	b.str("uris")
	b.sig("as") // variant signature
	// a(s): array of strings
	ilp, icp := b.arrayStart(4) // strings are 4-byte aligned
	b.str("file:///tmp/test.txt")
	b.arrayEnd(ilp, icp)

	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/tmp/test.txt"}
	if !slices.Equal(paths, want) {
		t.Errorf("decodePortalResponse() = %v, want %v", paths, want)
	}
}

// TestDecodePortalResponse_MultipleURIs verifies multiple URI extraction.
func TestDecodePortalResponse_MultipleURIs(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(0)

	lp, cp := b.arrayStart(8)

	b.padTo(8)
	b.str("uris")
	b.sig("as")
	ilp, icp := b.arrayStart(4)
	b.str("file:///home/user/a.png")
	b.str("file:///home/user/b.png")
	b.arrayEnd(ilp, icp)

	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/home/user/a.png", "/home/user/b.png"}
	if !slices.Equal(paths, want) {
		t.Errorf("decodePortalResponse() = %v, want %v", paths, want)
	}
}

// TestDecodePortalResponse_SkipsUnknownKeys verifies unknown dict entries are skipped.
func TestDecodePortalResponse_SkipsUnknownKeys(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(0)

	lp, cp := b.arrayStart(8)

	// unknown entry "writable" -> v(b) true
	b.padTo(8)
	b.str("writable")
	b.variantBool(true)

	// "uris" -> v(as)
	b.padTo(8)
	b.str("uris")
	b.sig("as")
	ilp, icp := b.arrayStart(4)
	b.str("file:///tmp/result.txt")
	b.arrayEnd(ilp, icp)

	b.arrayEnd(lp, cp)

	paths, err := decodePortalResponse(b.data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/tmp/result.txt"}
	if !slices.Equal(paths, want) {
		t.Errorf("decodePortalResponse() = %v, want %v", paths, want)
	}
}

// TestDecodePortalResponse_ErrorCode verifies non-zero non-cancel codes return error.
func TestDecodePortalResponse_ErrorCode(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(2) // error code
	lp, cp := b.arrayStart(8)
	b.arrayEnd(lp, cp)

	_, err := decodePortalResponse(b.data)
	if err == nil {
		t.Error("expected error for response code 2")
	}
}

// TestEncodeFileChooserBody verifies basic structure of the encoded portal body.
func TestEncodeFileChooserBody(t *testing.T) {
	body := encodeFileChooserBody("", "Open File", FileDialogOptions{
		Multiple: true,
		Filters:  []FileTypeFilter{{Name: "Images", Extensions: []string{"png", "jpg"}}},
	}, "gogpu_1", false)

	d := newMsgDecoder(body, 0)

	// parent_window
	pw, err := d.readStr()
	if err != nil || pw != "" {
		t.Errorf("parent_window = %q, want %q", pw, "")
	}

	// title
	title, err := d.readStr()
	if err != nil || title != "Open File" {
		t.Errorf("title = %q, want %q", title, "Open File")
	}

	// options a{sv} length (just verify it's non-zero)
	n, err := d.readU32()
	if err != nil || n == 0 {
		t.Errorf("options dict length = %d, want > 0 (err=%v)", n, err)
	}
}

// TestEncodeFileChooserBody_CurrentFolder verifies InitialDirectory encodes current_folder.
func TestEncodeFileChooserBody_CurrentFolder(t *testing.T) {
	body := encodeFileChooserBody("", "Open", FileDialogOptions{
		InitialDirectory: "/home/user/docs",
	}, "tok_1", false)

	if !bytes.Contains(body, []byte("current_folder")) {
		t.Error("expected current_folder key in portal body")
	}
	if !bytes.Contains(body, []byte("/home/user/docs")) {
		t.Error("expected InitialDirectory path in portal body")
	}
}

// TestEncodeFileChooserBody_NoCurrentFolderWhenEmpty verifies omission when empty.
func TestEncodeFileChooserBody_NoCurrentFolderWhenEmpty(t *testing.T) {
	body := encodeFileChooserBody("", "Open", FileDialogOptions{}, "tok_2", false)

	if bytes.Contains(body, []byte("current_folder")) {
		t.Error("current_folder must not appear when InitialDirectory is empty")
	}
}

// TestEncodePortalFilters verifies filter array encoding round-trips through the decoder.
func TestEncodePortalFilters(t *testing.T) {
	b := newMsgBuf(0)
	filters := []FileTypeFilter{
		{Name: "Images", Extensions: []string{"png", "jpg"}},
		{Name: "Docs", Extensions: []string{"pdf"}},
	}
	encodePortalFilters(b, filters)

	// Verify it decodes without error (structural smoke test).
	d := newMsgDecoder(b.data, 0)
	outerLen, err := d.readU32()
	if err != nil {
		t.Fatalf("read outer array len: %v", err)
	}
	if outerLen == 0 {
		t.Fatal("expected non-zero filter array length")
	}
}

// TestDBusEncodeMsg verifies the message fixed header fields.
func TestDBusEncodeMsg(t *testing.T) {
	msg := dbusEncodeMsg(
		dbusMsgCall, 7,
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.FileChooser",
		"OpenFile",
		"ssa{sv}",
		[]byte{0x01, 0x02, 0x03, 0x04},
	)

	if len(msg) < 16 {
		t.Fatalf("message too short: %d bytes", len(msg))
	}
	if msg[0] != 'l' {
		t.Errorf("endian: got %c, want 'l'", msg[0])
	}
	if msg[1] != dbusMsgCall {
		t.Errorf("type: got %d, want %d", msg[1], dbusMsgCall)
	}
	if msg[3] != 1 {
		t.Errorf("protocol version: got %d, want 1", msg[3])
	}
	bodyLen := binary.LittleEndian.Uint32(msg[4:8])
	if bodyLen != 4 {
		t.Errorf("body length: got %d, want 4", bodyLen)
	}
	serial := binary.LittleEndian.Uint32(msg[8:12])
	if serial != 7 {
		t.Errorf("serial: got %d, want 7", serial)
	}

	// Total length must be 8-byte aligned (header + padding + body)
	hdrArrayLen := binary.LittleEndian.Uint32(msg[12:16])
	headerTotal := 16 + int(hdrArrayLen)
	padLen := (8 - headerTotal%8) % 8
	expected := headerTotal + padLen + 4
	if len(msg) != expected {
		t.Errorf("total length: got %d, want %d", len(msg), expected)
	}
}

// TestDBusTypeAlign_Variant verifies variant alignment is 1, not 8.
func TestDBusTypeAlign_Variant(t *testing.T) {
	if got := dbusTypeAlign('v'); got != 1 {
		t.Errorf("dbusTypeAlign('v') = %d, want 1 (D-Bus spec)", got)
	}
	// Structs and dict entries must still be 8.
	if got := dbusTypeAlign('('); got != 8 {
		t.Errorf("dbusTypeAlign('(') = %d, want 8", got)
	}
	if got := dbusTypeAlign('{'); got != 8 {
		t.Errorf("dbusTypeAlign('{') = %d, want 8", got)
	}
}

// TestWaitResponse_EarlySignal verifies that a Response signal arriving before
// METHOD_RETURN is buffered and returned correctly. The D-Bus spec permits signals
// to be delivered before the method return that triggers them.
func TestWaitResponse_EarlySignal(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	const callSerial = uint32(3)
	handlePath := "/org/freedesktop/portal/desktop/request/1_42/gogpu_1"

	// Portal response body: success(0) + {"uris": ["file:///tmp/x.txt"]}
	var respBody []byte
	{
		b := newMsgBuf(0)
		b.u32(0)
		lp, cp := b.arrayStart(8)
		b.padTo(8)
		b.str("uris")
		b.sig("as")
		ilp, icp := b.arrayStart(4)
		b.str("file:///tmp/x.txt")
		b.arrayEnd(ilp, icp)
		b.arrayEnd(lp, cp)
		respBody = b.data
	}

	// METHOD_RETURN with REPLY_SERIAL = callSerial (empty body).
	var retMsg []byte
	{
		hdr := newMsgBuf(16)
		dbusWriteHdrField(hdr, dbusFieldReplySerial, "u", func() { hdr.u32(callSerial) })
		hdrBytes := hdr.data
		var fixed [16]byte
		fixed[0] = 'l'
		fixed[1] = dbusMsgReturn
		fixed[3] = 1
		binary.LittleEndian.PutUint32(fixed[8:], 99)
		binary.LittleEndian.PutUint32(fixed[12:], uint32(len(hdrBytes)))
		totalHdr := 16 + len(hdrBytes)
		padLen := (8 - totalHdr%8) % 8
		retMsg = append(retMsg, fixed[:]...)
		retMsg = append(retMsg, hdrBytes...)
		retMsg = append(retMsg, make([]byte, padLen)...)
	}

	go func() {
		defer server.Close()
		// Write Response signal FIRST — before METHOD_RETURN.
		sig := dbusEncodeMsg(dbusMsgSignal, 1, "",
			handlePath, "org.freedesktop.portal.Request", "Response", "ua{sv}", respBody)
		server.Write(sig)
		// Then METHOD_RETURN — client must not have missed the signal.
		server.Write(retMsg)
	}()

	conn := &dbusConn{rw: client}
	paths, err := conn.waitResponse(callSerial, handlePath)
	if err != nil {
		t.Fatalf("waitResponse: %v", err)
	}
	want := []string{"/tmp/x.txt"}
	if !slices.Equal(paths, want) {
		t.Errorf("got %v, want %v", paths, want)
	}
}

// TestWaitResponse_NormalOrder verifies the common case: METHOD_RETURN before signal.
func TestWaitResponse_NormalOrder(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	const callSerial = uint32(5)
	handlePath := "/org/freedesktop/portal/desktop/request/1_42/gogpu_2"

	var respBody []byte
	{
		b := newMsgBuf(0)
		b.u32(0)
		lp, cp := b.arrayStart(8)
		b.padTo(8)
		b.str("uris")
		b.sig("as")
		ilp, icp := b.arrayStart(4)
		b.str("file:///home/user/doc.pdf")
		b.arrayEnd(ilp, icp)
		b.arrayEnd(lp, cp)
		respBody = b.data
	}

	var retMsg []byte
	{
		hdr := newMsgBuf(16)
		dbusWriteHdrField(hdr, dbusFieldReplySerial, "u", func() { hdr.u32(callSerial) })
		hdrBytes := hdr.data
		var fixed [16]byte
		fixed[0] = 'l'
		fixed[1] = dbusMsgReturn
		fixed[3] = 1
		binary.LittleEndian.PutUint32(fixed[8:], 99)
		binary.LittleEndian.PutUint32(fixed[12:], uint32(len(hdrBytes)))
		totalHdr := 16 + len(hdrBytes)
		padLen := (8 - totalHdr%8) % 8
		retMsg = append(retMsg, fixed[:]...)
		retMsg = append(retMsg, hdrBytes...)
		retMsg = append(retMsg, make([]byte, padLen)...)
	}

	go func() {
		defer server.Close()
		// Normal order: METHOD_RETURN first, then signal.
		server.Write(retMsg)
		sig := dbusEncodeMsg(dbusMsgSignal, 2, "",
			handlePath, "org.freedesktop.portal.Request", "Response", "ua{sv}", respBody)
		server.Write(sig)
	}()

	conn := &dbusConn{rw: client}
	paths, err := conn.waitResponse(callSerial, handlePath)
	if err != nil {
		t.Fatalf("waitResponse: %v", err)
	}
	want := []string{"/home/user/doc.pdf"}
	if !slices.Equal(paths, want) {
		t.Errorf("got %v, want %v", paths, want)
	}
}

// TestMsgDecoderSkipValue verifies skipValue handles common portal response types.
func TestMsgDecoderSkipValue(t *testing.T) {
	tests := []struct {
		name   string
		sig    string
		encode func(*msgBuf)
	}{
		{
			name:   "bool",
			sig:    "b",
			encode: func(b *msgBuf) { b.bool32(true) },
		},
		{
			name:   "uint32",
			sig:    "u",
			encode: func(b *msgBuf) { b.u32(42) },
		},
		{
			name:   "string",
			sig:    "s",
			encode: func(b *msgBuf) { b.str("hello world") },
		},
		{
			name:   "variant bool",
			sig:    "v",
			encode: func(b *msgBuf) { b.variantBool(false) },
		},
		{
			name: "array of strings",
			sig:  "as",
			encode: func(b *msgBuf) {
				lp, cp := b.arrayStart(4)
				b.str("one")
				b.str("two")
				b.arrayEnd(lp, cp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := newMsgBuf(0)
			tt.encode(enc)

			// Append sentinel byte after the encoded value.
			enc.u8(0xFF)
			sentinelPos := len(enc.data) - 1

			d := newMsgDecoder(enc.data, 0)
			if err := d.skipValue(tt.sig); err != nil {
				t.Fatalf("skipValue(%q): %v", tt.sig, err)
			}
			// After skip, cursor must land exactly on the sentinel byte —
			// neither overrun (reads past it) nor underrun (stops early).
			if d.pos != sentinelPos {
				t.Errorf("skipValue(%q): pos=%d, want %d (sentinel)", tt.sig, d.pos, sentinelPos)
			}
		})
	}
}
