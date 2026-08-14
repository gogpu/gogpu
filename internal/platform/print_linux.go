//go:build linux

package platform

// Linux native printing through xdg-desktop-portal.
//
// The portal print protocol is intentionally kept in this file instead of
// pulling in a D-Bus dependency.  StartPrint performs the short setup phase
// synchronously (session-bus connection and PreparePrint method call), then a
// job goroutine waits for the user's response, submits the caller-owned PDF via
// a temporary read-only file descriptor, and waits for the terminal response.
// A temporary file remains open until the Print request's response arrives;
// this is required because the portal may read the descriptor asynchronously.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const linuxPrintMIMETypePDF = "application/pdf"

var (
	errLinuxPrintUnsupported = ErrPrintUnavailable
	errLinuxPrintMIME        = errors.New("gogpu: Linux portal printing supports PDF documents only")
)

// StartPrint implements PrintManager for X11.  The X11 XID is the portal's
// parent-window token; an empty parent is used when the app has no window yet.
func (p *x11Platform) StartPrint(ctx context.Context, request PrintRequest) (PrintJob, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	parent, err := p.printParentWindow(request.Options.Parent)
	if err != nil {
		return nil, err
	}
	return startLinuxPortalPrint(ctx, request, parent)
}

// StartPrint implements PrintManager for Wayland.  This backend does not own
// an xdg-foreign exporter, so a valid parent window is intentionally passed as
// an empty portal identifier.  xdg-desktop-portal specifies that empty
// parent_window is valid when no suitable xdg-foreign handle is available.
func (p *waylandPlatform) StartPrint(ctx context.Context, request PrintRequest) (PrintJob, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	parent, err := p.printParentWindow(request.Options.Parent)
	if err != nil {
		return nil, err
	}
	return startLinuxPortalPrint(ctx, request, parent)
}

var (
	_ PrintManager = (*x11Platform)(nil)
	_ PrintManager = (*waylandPlatform)(nil)
)

func (p *x11Platform) printParentWindow(id WindowID) (string, error) {
	if p == nil || p.inner == nil {
		if id == 0 {
			return "", nil
		}
		return "", fmt.Errorf("x11: parent window %d is unavailable", id)
	}
	if id == 0 {
		id = p.primaryWindowID
	}
	if id == 0 {
		return "", nil
	}
	if id == p.primaryWindowID {
		_, xid := p.inner.GetHandle()
		if xid == 0 {
			return "", fmt.Errorf("x11: parent window %d has no XID", id)
		}
		return "x11:" + strconv.FormatUint(uint64(xid), 16), nil
	}

	p.secondaryMu.RLock()
	defer p.secondaryMu.RUnlock()
	for _, sec := range p.secondaries {
		if sec.winID != id || sec.platform == nil {
			continue
		}
		_, xid := sec.platform.GetHandle()
		if xid == 0 {
			return "", fmt.Errorf("x11: parent window %d has no XID", id)
		}
		return "x11:" + strconv.FormatUint(uint64(xid), 16), nil
	}
	return "", fmt.Errorf("x11: parent window %d is unknown", id)
}

func (p *waylandPlatform) printParentWindow(id WindowID) (string, error) {
	if p == nil {
		if id == 0 {
			return "", nil
		}
		return "", fmt.Errorf("wayland: parent window %d is unavailable", id)
	}
	if id == 0 {
		id = p.primaryWindowID
	}
	if id == 0 {
		return "", nil
	}
	if id == p.primaryWindowID {
		if p.libwl == nil || p.libwl.Surface() == 0 {
			return "", fmt.Errorf("wayland: parent window %d is unavailable", id)
		}
		return "", nil
	}

	p.secondaryMu.RLock()
	defer p.secondaryMu.RUnlock()
	for _, sec := range p.secondaries {
		if sec.winID == id && sec.libwl != nil && sec.libwl.Surface() != 0 {
			return "", nil
		}
	}
	return "", fmt.Errorf("wayland: parent window %d is unknown", id)
}

// startLinuxPortalPrint performs the synchronous setup portion of the portal
// operation.  If the session bus or portal cannot be reached before a request
// is submitted, no PrintJob is returned; callers can safely report unsupported
// printing rather than opening a second, unrelated dialog fallback.
func startLinuxPortalPrint(ctx context.Context, request PrintRequest, parent string) (PrintJob, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: nil context", errLinuxPrintUnsupported)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Document.MIMEType != linuxPrintMIMETypePDF {
		return nil, fmt.Errorf("%w: %q", errLinuxPrintMIME, request.Document.MIMEType)
	}
	if len(request.Document.Data) == 0 {
		return nil, fmt.Errorf("%w: empty document", errLinuxPrintMIME)
	}
	if request.Options.Copies < 0 {
		return nil, errors.New("print: copies must not be negative")
	}
	for _, r := range request.Options.PageRanges {
		if r.From <= 0 || r.To < r.From {
			return nil, fmt.Errorf("print: invalid page range %d-%d", r.From, r.To)
		}
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return nil, fmt.Errorf("%w: DBUS_SESSION_BUS_ADDRESS is not set", errLinuxPrintUnsupported)
	}

	document, err := createLinuxPrintDocument(request.Document.Data)
	if err != nil {
		return nil, err
	}

	operationCtx, cancel := context.WithCancel(ctx)
	conn, err := dbusConnectContext(operationCtx)
	if err != nil {
		cancel()
		closeLinuxPrintDocument(document)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: connect session bus: %v", errLinuxPrintUnsupported, err)
	}

	prepareToken := dbusNewToken()
	prepareBody := encodePortalPreparePrintBody(parent, request.Options.Title, request, prepareToken)
	prepareSerial, err := conn.sendCall(
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Print",
		"PreparePrint",
		"ssa{sv}a{sv}a{sv}",
		prepareBody,
	)
	if err != nil {
		conn.rw.Close()
		cancel()
		closeLinuxPrintDocument(document)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: send PreparePrint: %v", errLinuxPrintUnsupported, err)
	}

	job := &linuxPrintJob{
		printJob:      newPrintJob(),
		ctx:           operationCtx,
		cancel:        cancel,
		conn:          conn,
		document:      document,
		prepareSerial: prepareSerial,
		preparePath:   dbusHandlePath(conn.name, prepareToken),
		parent:        parent,
		title:         request.Options.Title,
	}
	job.setCancel(func() {
		cancel()
		job.closeConn()
	})
	watchPrintContext(ctx, job.printJob)
	go job.run()
	return job, nil
}

// createLinuxPrintDocument materializes the complete document in a private
// temporary file.  The portal receives the open descriptor, not a path, so the
// file remains inaccessible to unrelated users and works in sandboxes.
func createLinuxPrintDocument(data []byte) (*os.File, error) {
	f, err := os.CreateTemp("", "gogpu-print-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("print: create temporary document: %w", err)
	}
	if err := f.Chmod(0600); err != nil {
		closeLinuxPrintDocument(f)
		return nil, fmt.Errorf("print: protect temporary document: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		closeLinuxPrintDocument(f)
		return nil, fmt.Errorf("print: write temporary document: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		closeLinuxPrintDocument(f)
		return nil, fmt.Errorf("print: rewind temporary document: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return nil, fmt.Errorf("print: close writable temporary document: %w", err)
	}
	readOnly, err := os.Open(name)
	if err != nil {
		_ = os.Remove(name)
		return nil, fmt.Errorf("print: reopen temporary document: %w", err)
	}
	return readOnly, nil
}

func closeLinuxPrintDocument(f *os.File) {
	if f == nil {
		return
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
}

type linuxPrintJob struct {
	*printJob
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	conn     *dbusConn
	document *os.File

	prepareSerial uint32
	preparePath   string
	parent        string
	title         string

	finishOnce sync.Once
}

func (j *linuxPrintJob) closeConn() {
	j.mu.Lock()
	conn := j.conn
	j.mu.Unlock()
	if conn != nil && conn.rw != nil {
		_ = conn.rw.Close()
	}
}

func (j *linuxPrintJob) run() {
	defer func() {
		if err := j.ctx.Err(); err != nil {
			j.finish(err)
		}
	}()

	j.mu.Lock()
	conn := j.conn
	document := j.document
	prepareSerial := j.prepareSerial
	preparePath := j.preparePath
	j.mu.Unlock()

	body, err := conn.waitResponseBody(prepareSerial, preparePath)
	if err != nil {
		j.finish(j.contextualError(err))
		return
	}
	preparedToken, err := decodePortalPreparePrintResponse(body)
	if err != nil {
		j.finish(j.contextualError(err))
		return
	}
	if err := j.ctx.Err(); err != nil {
		j.finish(err)
		return
	}

	printToken := dbusNewToken()
	printBody := encodePortalPrintBody(j.parent, j.title, 0, printToken, preparedToken)
	printSerial, err := conn.sendCallWithFDs(
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Print",
		"Print",
		"ssha{sv}",
		printBody,
		[]int{int(document.Fd())},
	)
	if err != nil {
		j.finish(j.contextualError(fmt.Errorf("print: send Print: %w", err)))
		return
	}
	printPath := dbusHandlePath(conn.name, printToken)
	body, err = conn.waitResponseBody(printSerial, printPath)
	if err != nil {
		j.finish(j.contextualError(err))
		return
	}
	j.finish(j.contextualError(decodePortalPrintResponse(body)))
}

func (j *linuxPrintJob) contextualError(err error) error {
	if ctxErr := j.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (j *linuxPrintJob) finish(err error) {
	j.finishOnce.Do(func() {
		j.mu.Lock()
		conn := j.conn
		document := j.document
		j.conn = nil
		j.document = nil
		j.mu.Unlock()
		if conn != nil && conn.rw != nil {
			_ = conn.rw.Close()
		}
		closeLinuxPrintDocument(document)
		if j.cancel != nil {
			j.cancel()
		}
		j.clearCancel()
		j.complete(err)
	})
}

// encodePortalPreparePrintBody produces the portal's
// ssa{sv}a{sv}a{sv} arguments: parent, title, initial print settings, page
// setup, and options.  Settings are hints only; PreparePrint's response token
// is authoritative for the subsequent Print call.
func encodePortalPreparePrintBody(parent, title string, request PrintRequest, token string) []byte {
	b := newMsgBuf(0)
	b.str(parent)
	b.str(title)
	encodePortalPrintSettings(b, request)
	encodeEmptyPortalDict(b)
	encodePortalDict(b, func() {
		portalDictEntryStr(b, "handle_token", token)
	})
	return b.data
}

// encodePortalPrintBody produces the portal's ssha{sv} arguments.  fdIndex is
// the descriptor index in the SCM_RIGHTS control message (always zero for the
// single document descriptor used here).
func encodePortalPrintBody(parent, title string, fdIndex uint32, handleToken string, preparedToken uint32) []byte {
	b := newMsgBuf(0)
	b.str(parent)
	b.str(title)
	b.u32(fdIndex) // `h`: index into the attached Unix FD array
	encodePortalDict(b, func() {
		portalDictEntryStr(b, "handle_token", handleToken)
		portalDictEntryU32(b, "token", preparedToken)
	})
	return b.data
}

func encodePortalPrintSettings(b *msgBuf, request PrintRequest) {
	lp, cp := b.arrayStart(8)
	if request.Options.Copies > 0 {
		portalDictEntryStr(b, "n-copies", strconv.Itoa(request.Options.Copies))
	}
	if ranges := portalPageRanges(request.Options.PageRanges); ranges != "" {
		portalDictEntryStr(b, "print-pages", "ranges")
		portalDictEntryStr(b, "page-ranges", ranges)
	}
	if name := linuxPrintOutputBasename(request.Document.Name); name != "" {
		portalDictEntryStr(b, "output-basename", name)
	}
	b.arrayEnd(lp, cp)
}

// linuxPrintOutputBasename keeps the user-facing name as a safe basename for
// the portal's print-to-file hint.  Empty, root, and dot path names carry no
// useful basename and are omitted rather than sending an invalid hint.
func linuxPrintOutputBasename(name string) string {
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func encodeEmptyPortalDict(b *msgBuf) {
	lp, cp := b.arrayStart(8)
	b.arrayEnd(lp, cp)
}

func encodePortalDict(b *msgBuf, entries func()) {
	lp, cp := b.arrayStart(8)
	entries()
	b.arrayEnd(lp, cp)
}

func portalDictEntryStr(b *msgBuf, key, value string) {
	b.padTo(8)
	b.str(key)
	b.variantStr(value)
}

func portalDictEntryU32(b *msgBuf, key string, value uint32) {
	b.padTo(8)
	b.str(key)
	b.variantU32(value)
}

// portalPageRanges translates the public one-based inclusive ranges to the
// portal's comma-separated zero-based representation.  A one-page range is
// emitted as a single number (rather than "n-n").
func portalPageRanges(ranges []PrintPageRange) string {
	if len(ranges) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		from, to := r.From-1, r.To-1
		if from == to {
			parts = append(parts, strconv.Itoa(from))
		} else {
			parts = append(parts, strconv.Itoa(from)+"-"+strconv.Itoa(to))
		}
	}
	return strings.Join(parts, ",")
}

// decodePortalPreparePrintResponse extracts the token from a successful
// ua{sv} response.  Response code 1 is the portal's user-canceled outcome and
// is normalized to context.Canceled for PrintJob.Done.
func decodePortalPreparePrintResponse(body []byte) (uint32, error) {
	d := newMsgDecoder(body, 0)
	code, err := d.readU32()
	if err != nil {
		return 0, fmt.Errorf("print: decode PreparePrint response code: %w", err)
	}
	if code == 1 {
		return 0, context.Canceled
	}
	if code != 0 {
		return 0, fmt.Errorf("print: portal PreparePrint response code %d", code)
	}
	arrayLen, err := d.readU32()
	if err != nil {
		return 0, fmt.Errorf("print: decode PreparePrint results: %w", err)
	}
	if err := d.alignTo(8); err != nil {
		return 0, err
	}
	end := d.pos + int(arrayLen)
	var token uint32
	foundToken := false
	for d.pos < end {
		if err := d.alignTo(8); err != nil {
			return 0, err
		}
		key, err := d.readStr()
		if err != nil {
			return 0, err
		}
		vsig, err := d.readSig()
		if err != nil {
			return 0, err
		}
		if key == "token" && vsig == "u" {
			token, err = d.readU32()
			if err != nil {
				return 0, err
			}
			foundToken = true
			continue
		}
		if err := d.skipValue(vsig); err != nil {
			return 0, err
		}
	}
	if !foundToken {
		return 0, errors.New("print: PreparePrint response did not contain a token")
	}
	return token, nil
}

func decodePortalPrintResponse(body []byte) error {
	d := newMsgDecoder(body, 0)
	code, err := d.readU32()
	if err != nil {
		return fmt.Errorf("print: decode Print response code: %w", err)
	}
	switch code {
	case 0:
		return nil
	case 1:
		return context.Canceled
	default:
		return fmt.Errorf("print: portal Print response code %d", code)
	}
}
