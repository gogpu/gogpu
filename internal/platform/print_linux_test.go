//go:build linux

package platform

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestPortalPageRanges(t *testing.T) {
	tests := []struct {
		name   string
		ranges []PrintPageRange
		want   string
	}{
		{name: "all", want: ""},
		{name: "single page", ranges: []PrintPageRange{{From: 1, To: 1}}, want: "0"},
		{name: "inclusive conversion", ranges: []PrintPageRange{{From: 1, To: 3}, {From: 5, To: 5}, {From: 8, To: 9}}, want: "0-2,4,7-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portalPageRanges(tt.ranges); got != tt.want {
				t.Fatalf("portalPageRanges(%v) = %q, want %q", tt.ranges, got, tt.want)
			}
		})
	}
}

func TestLinuxPrintOutputBasename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "reports/invoice.pdf", want: "invoice.pdf"},
		{name: "", want: ""},
		{name: ".", want: ""},
		{name: "..", want: ""},
		{name: "/", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linuxPrintOutputBasename(tt.name); got != tt.want {
				t.Fatalf("linuxPrintOutputBasename(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestEncodePortalPreparePrintBody(t *testing.T) {
	request := PrintRequest{
		Document: PrintDocument{Name: "reports/invoice.pdf", MIMEType: linuxPrintMIMETypePDF, Data: []byte("pdf")},
		Options: PrintOptions{
			Parent:     12,
			Title:      "Invoice",
			PageRanges: []PrintPageRange{{From: 2, To: 4}},
			Copies:     2,
		},
	}
	body := encodePortalPreparePrintBody("x11:2a", "Invoice", request, "gogpu_prepare")
	d := newMsgDecoder(body, 0)
	parent, err := d.readStr()
	if err != nil || parent != "x11:2a" {
		t.Fatalf("parent = %q (err=%v), want x11:2a", parent, err)
	}
	title, err := d.readStr()
	if err != nil || title != "Invoice" {
		t.Fatalf("title = %q (err=%v), want Invoice", title, err)
	}
	settings, err := readPortalDict(d)
	if err != nil {
		t.Fatal(err)
	}
	wantSettings := map[string]string{
		"n-copies":        "2",
		"print-pages":     "ranges",
		"page-ranges":     "1-3",
		"output-basename": "invoice.pdf",
	}
	if !reflect.DeepEqual(settings, wantSettings) {
		t.Fatalf("settings = %#v, want %#v", settings, wantSettings)
	}
	pageSetup, err := readPortalDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(pageSetup) != 0 {
		t.Fatalf("page setup = %#v, want empty", pageSetup)
	}
	options, err := readPortalDict(d)
	if err != nil {
		t.Fatal(err)
	}
	if options["handle_token"] != "gogpu_prepare" {
		t.Fatalf("options = %#v, want handle_token", options)
	}
}

func TestEncodePortalPrintBody(t *testing.T) {
	body := encodePortalPrintBody("x11:2a", "Invoice", 0, "gogpu_print", 77)
	d := newMsgDecoder(body, 0)
	if got, _ := d.readStr(); got != "x11:2a" {
		t.Fatalf("parent = %q, want x11:2a", got)
	}
	if got, _ := d.readStr(); got != "Invoice" {
		t.Fatalf("title = %q, want Invoice", got)
	}
	fdIndex, err := d.readU32()
	if err != nil || fdIndex != 0 {
		t.Fatalf("fd index = %d (err=%v), want 0", fdIndex, err)
	}
	options, err := readPortalDictTyped(d)
	if err != nil {
		t.Fatal(err)
	}
	if options["handle_token"] != "gogpu_print" || options["token"] != "77" {
		t.Fatalf("options = %#v, want handle_token/token", options)
	}
}

func TestDecodePortalPreparePrintResponse(t *testing.T) {
	b := newMsgBuf(0)
	b.u32(0)
	lp, cp := b.arrayStart(8)
	b.padTo(8)
	b.str("settings")
	b.variantStr("ignored")
	b.padTo(8)
	b.str("token")
	b.variantU32(42)
	b.arrayEnd(lp, cp)

	token, err := decodePortalPreparePrintResponse(b.data)
	if err != nil || token != 42 {
		t.Fatalf("token = %d (err=%v), want 42", token, err)
	}

	canceled := newMsgBuf(0)
	canceled.u32(1)
	clp, ccp := canceled.arrayStart(8)
	canceled.arrayEnd(clp, ccp)
	if _, err := decodePortalPreparePrintResponse(canceled.data); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v, want context.Canceled", err)
	}
}

func TestDecodePortalPrintResponse(t *testing.T) {
	for _, code := range []uint32{0, 2} {
		b := newMsgBuf(0)
		b.u32(code)
		err := decodePortalPrintResponse(b.data)
		if code == 0 && err != nil {
			t.Fatalf("success response error = %v", err)
		}
		if code == 2 && err == nil {
			t.Fatal("error response unexpectedly succeeded")
		}
	}
	b := newMsgBuf(0)
	b.u32(1)
	if err := decodePortalPrintResponse(b.data); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v, want context.Canceled", err)
	}
}

func TestDBusEncodeMsgWithFDs(t *testing.T) {
	raw := dbusEncodeMsgWithFDs(
		dbusMsgCall, 7,
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Print",
		"Print",
		"ssha{sv}",
		[]byte{0x01, 0x02, 0x03, 0x04},
		1,
	)
	if len(raw) < 16 {
		t.Fatalf("message too short: %d", len(raw))
	}
	hdrLen := int(binaryLE32(raw[12:16]))
	msg := &dbusMsg{}
	dbusParseHdrFields(raw[16:16+hdrLen], msg)
	if msg.Sig != "ssha{sv}" {
		t.Fatalf("signature = %q, want ssha{sv}", msg.Sig)
	}
	if msg.UnixFDs != 1 {
		t.Fatalf("UnixFDs = %d, want 1", msg.UnixFDs)
	}
}

func TestDBusSendCallWithFDsPassesDescriptor(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "bus.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: socketPath})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Net: "unix", Name: socketPath})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	document, err := os.CreateTemp(t.TempDir(), "document.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	defer os.Remove(document.Name())

	conn := &dbusConn{rw: client}
	_, err = conn.sendCallWithFDs("dest", "/path", "iface", "Print", "h", []byte{0, 0, 0, 0}, []int{int(document.Fd())})
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 4096)
	oob := make([]byte, 256)
	n, oobn, _, _, err := server.ReadMsgUnix(buf, oob)
	if err != nil {
		t.Fatal(err)
	}
	if n < 16 || oobn == 0 {
		t.Fatalf("ReadMsgUnix() = data %d, oob %d; want payload and descriptor", n, oobn)
	}
	msg := &dbusMsg{}
	hdrLen := int(binaryLE32(buf[12:16]))
	if 16+hdrLen > n {
		t.Fatalf("message header length %d exceeds payload %d", hdrLen, n)
	}
	dbusParseHdrFields(buf[16:16+hdrLen], msg)
	if msg.UnixFDs != 1 || msg.Sig != "h" {
		t.Fatalf("message metadata = UnixFDs:%d Sig:%q, want 1/h", msg.UnixFDs, msg.Sig)
	}
	control, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatal(err)
	}
	if len(control) != 1 {
		t.Fatalf("control messages = %d, want 1", len(control))
	}
	passed, err := unix.ParseUnixRights(&control[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(passed) != 1 {
		t.Fatalf("passed descriptors = %d, want 1", len(passed))
	}
	_ = unix.Close(passed[0])
}

func TestDBusSendCallContextClosesBlockedSetupWrite(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	conn := &dbusConn{rw: client}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := conn.sendCallContext(ctx, "dest", "/path", "iface", "PreparePrint", "s", make([]byte, 1<<20))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sendCallContext() error = %v, want context.Canceled", err)
	}
}

func TestDBusCloseRequestWritesPortalCloseMethod(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	conn := &dbusConn{rw: client}
	handlePath := "/org/freedesktop/portal/desktop/request/1_2/gogpu_close"
	done := make(chan struct{})
	go func() {
		conn.closeRequest(handlePath)
		close(done)
	}()
	var fixed [16]byte
	if _, err := io.ReadFull(server, fixed[:]); err != nil {
		t.Fatal(err)
	}
	if fixed[1] != dbusMsgCall {
		t.Fatalf("message type = %d, want method call", fixed[1])
	}
	hdrLen := int(binaryLE32(fixed[12:16]))
	hdr := make([]byte, hdrLen)
	if _, err := io.ReadFull(server, hdr); err != nil {
		t.Fatal(err)
	}
	if pad := (8 - (16+hdrLen)%8) % 8; pad > 0 {
		padding := make([]byte, pad)
		if _, err := io.ReadFull(server, padding); err != nil {
			t.Fatal(err)
		}
	}
	msg := &dbusMsg{}
	dbusParseHdrFields(hdr, msg)
	if msg.Path != handlePath || msg.Interface != "org.freedesktop.portal.Request" || msg.Member != "Close" {
		t.Fatalf("close request headers = path:%q iface:%q member:%q", msg.Path, msg.Interface, msg.Member)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("closeRequest did not return")
	}
}

func TestCreateLinuxPrintDocumentOwnsAndCleansFile(t *testing.T) {
	f, err := createLinuxPrintDocument([]byte("%PDF-test"))
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	defer closeLinuxPrintDocument(f)
	if filepath.Ext(name) != ".pdf" {
		t.Fatalf("temporary document name = %q, want .pdf extension", name)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Fatalf("temporary document mode = %#o, want 0600", mode)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "%PDF-test" {
		t.Fatalf("temporary document = %q, want original bytes", data)
	}
	closeLinuxPrintDocument(f)
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("temporary document still exists after cleanup: %v", err)
	}
}

func TestLinuxPrintJobCancelClosesConnectionAndDocument(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	document, err := createLinuxPrintDocument([]byte("pdf"))
	if err != nil {
		t.Fatal(err)
	}
	name := document.Name()
	ctx, cancel := context.WithCancel(context.Background())
	job := &linuxPrintJob{
		printJob:      newPrintJob(),
		ctx:           ctx,
		cancel:        cancel,
		conn:          &dbusConn{rw: client},
		document:      document,
		prepareSerial: 1,
		preparePath:   "/org/freedesktop/portal/desktop/request/1_1/gogpu_1",
	}
	job.setCancel(func() {
		cancel()
		job.closeConn()
	})
	go job.run()
	job.Cancel()

	select {
	case got := <-job.Done():
		if !errors.Is(got, context.Canceled) {
			t.Fatalf("Done() = %v, want context.Canceled", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for canceled print job")
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("temporary document still exists after canceled job: %v", err)
	}
}

// readPortalDict decodes the string-valued subset used by PreparePrint
// settings/options.  It intentionally rejects a non-string value so tests do
// not accidentally accept a malformed variant.
func readPortalDict(d *msgDecoder) (map[string]string, error) {
	result := make(map[string]string)
	n, err := d.readU32()
	if err != nil {
		return nil, err
	}
	if err := d.alignTo(8); err != nil {
		return nil, err
	}
	end := d.pos + int(n)
	for d.pos < end {
		if err := d.alignTo(8); err != nil {
			return nil, err
		}
		key, err := d.readStr()
		if err != nil {
			return nil, err
		}
		sig, err := d.readSig()
		if err != nil {
			return nil, err
		}
		if sig != "s" {
			return nil, errors.New("portal test: expected string variant")
		}
		value, err := d.readStr()
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

func readPortalDictTyped(d *msgDecoder) (map[string]string, error) {
	result := make(map[string]string)
	n, err := d.readU32()
	if err != nil {
		return nil, err
	}
	if err := d.alignTo(8); err != nil {
		return nil, err
	}
	end := d.pos + int(n)
	for d.pos < end {
		if err := d.alignTo(8); err != nil {
			return nil, err
		}
		key, err := d.readStr()
		if err != nil {
			return nil, err
		}
		sig, err := d.readSig()
		if err != nil {
			return nil, err
		}
		switch sig {
		case "s":
			value, err := d.readStr()
			if err != nil {
				return nil, err
			}
			result[key] = value
		case "u":
			value, err := d.readU32()
			if err != nil {
				return nil, err
			}
			result[key] = strconv.FormatUint(uint64(value), 10)
		default:
			if err := d.skipValue(sig); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func binaryLE32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
