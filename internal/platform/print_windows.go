//go:build windows

package platform

// The Windows backend deliberately uses the Win32 print dialog and GDI
// passthrough path instead of adding a PDF renderer to gogpu.  The caller
// owns the complete document bytes; the selected printer driver is
// responsible for consuming the declared document format.  Drivers that
// advertise raw PDF/document support can thus receive the same bytes that the
// caller supplied without a second in-process renderer.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// PRINTDLGEX flags.
	pdPageNums                   = 0x00000002
	pdReturnDC                   = 0x00000100
	pdUseDevModeCopiesAndCollate = 0x00040000
	// PRINTDLGEX result actions.
	pdResultPrint  = 1
	pdResultCancel = 2

	// The PASSTHROUGH escape sends caller-owned bytes directly to the printer
	// driver's spool stream.  It is supported by the Win32 GDI print contract.
	passthroughEscape = 19

	// PrintDlgEx uses a DWORD page number.  A PDF's page count is intentionally
	// not parsed by this package, so expose the full practical dialog range.
	printDialogMaxPage = math.MaxUint32

	// Keep individual Escape calls below the size accepted by older printer
	// drivers.  The document itself may be arbitrarily larger.
	printEscapeChunkSize = 64 * 1024
)

// printDlgEx mirrors PRINTDLGEXW.  Pointer-sized fields are represented as
// uintptr so the struct has the same layout on 32- and 64-bit Windows.
type printDlgEx struct {
	lStructSize         uint32
	hwndOwner           uintptr
	hDevMode            uintptr
	hDevNames           uintptr
	hDC                 uintptr
	flags               uint32
	flags2              uint32
	exclusionFlags      uint32
	nPageRanges         uint32
	nMaxPageRanges      uint32
	lpPageRanges        uintptr
	nMinPage            uint32
	nMaxPage            uint32
	nCopies             uint16
	_                   uint16 // alignment before hInstance on 32/64-bit
	hInstance           uintptr
	lpPrintTemplateName *uint16
	lpCallback          uintptr
	nPropertyPages      uint32
	lphPropertyPages    uintptr
	nStartPage          uint32
	dwResultAction      uint32
}

// printPageRange mirrors PRINTPAGERANGE.
type printPageRange struct {
	nFromPage uint32
	nToPage   uint32
}

// printDocInfo mirrors DOCINFOW, used by StartDocW.
type printDocInfo struct {
	cbSize       int32
	lpszDocName  *uint16
	lpszOutput   *uint16
	lpszDatatype *uint16
	fwType       uint32
}

// windowsPrintSyscalls is the narrow syscall seam for deterministic tests.
// All methods are called from the worker's locked OS thread except AbortDoc,
// which may also be called by Cancel while a blocking print operation runs.
type windowsPrintSyscalls struct {
	printDlgEx   func(*printDlgEx) uint32
	globalFree   func(uintptr)
	deleteDC     func(uintptr) int32
	startDoc     func(uintptr, *printDocInfo) int32
	startPage    func(uintptr) int32
	endPage      func(uintptr) int32
	endDoc       func(uintptr) int32
	escape       func(uintptr, int32, uint32, uintptr, uintptr) int32
	abortDoc     func(uintptr) int32
	cancelDialog func(uintptr)
}

var (
	comdlg32Print               = windows.NewLazyDLL("comdlg32.dll")
	gdi32Print                  = windows.NewLazyDLL("gdi32.dll")
	procPrintDlgExW             = comdlg32Print.NewProc("PrintDlgExW")
	procGlobalFreePrint         = kernel32.NewProc("GlobalFree")
	procDeleteDCPrint           = gdi32Print.NewProc("DeleteDC")
	procStartDocWPrint          = gdi32Print.NewProc("StartDocW")
	procStartPagePrint          = gdi32Print.NewProc("StartPage")
	procEndPagePrint            = gdi32Print.NewProc("EndPage")
	procEndDocPrint             = gdi32Print.NewProc("EndDoc")
	procEscapePrint             = gdi32Print.NewProc("Escape")
	procAbortDocPrint           = gdi32Print.NewProc("AbortDoc")
	procGetLastActivePopupPrint = user32.NewProc("GetLastActivePopup")
)

var windowsPrintCalls = windowsPrintSyscalls{
	printDlgEx: func(dlg *printDlgEx) uint32 {
		r, _, _ := procPrintDlgExW.Call(uintptr(unsafe.Pointer(dlg)))
		return uint32(r)
	},
	globalFree: func(handle uintptr) {
		procGlobalFreePrint.Call(handle)
	},
	deleteDC: func(hdc uintptr) int32 {
		r, _, _ := procDeleteDCPrint.Call(hdc)
		return int32(r)
	},
	startDoc: func(hdc uintptr, info *printDocInfo) int32 {
		r, _, _ := procStartDocWPrint.Call(hdc, uintptr(unsafe.Pointer(info)))
		return int32(r)
	},
	startPage: func(hdc uintptr) int32 {
		r, _, _ := procStartPagePrint.Call(hdc)
		return int32(r)
	},
	endPage: func(hdc uintptr) int32 {
		r, _, _ := procEndPagePrint.Call(hdc)
		return int32(r)
	},
	endDoc: func(hdc uintptr) int32 {
		r, _, _ := procEndDocPrint.Call(hdc)
		return int32(r)
	},
	escape: func(hdc uintptr, escape int32, inputCount uint32, input, output uintptr) int32 {
		r, _, _ := procEscapePrint.Call(hdc, uintptr(escape), uintptr(inputCount), input, output)
		return int32(r)
	},
	abortDoc: func(hdc uintptr) int32 {
		r, _, _ := procAbortDocPrint.Call(hdc)
		return int32(r)
	},
	cancelDialog: func(parent uintptr) {
		if parent == 0 {
			return
		}
		popup, _, _ := procGetLastActivePopupPrint.Call(parent)
		if popup != 0 && popup != parent {
			procPostMessageW.Call(popup, uintptr(wmClose), 0, 0)
		}
	},
}

// windowsPrintJob adds the native abort hook to the shared print lifecycle.
// The shared state machine makes cancellation and terminal publication match
// the macOS/Linux backends; abort is installed only while a native GDI job is
// active so a later Cancel cannot touch a recycled HDC.
type windowsPrintJob struct {
	*printJob
}

func newWindowsPrintJob() *windowsPrintJob {
	return &windowsPrintJob{
		printJob: newPrintJob(),
	}
}

func (j *windowsPrintJob) setAbort(abort func()) {
	j.setCancel(abort)
}

func (j *windowsPrintJob) clearAbort() {
	j.clearCancel()
}

func (j *windowsPrintJob) isCanceled() bool {
	return j.canceled()
}

func (j *windowsPrintJob) finish(err error) {
	if errors.Is(err, context.Canceled) {
		err = context.Canceled
	}
	j.clearAbort()
	j.complete(err)
}

// StartPrint implements PrintManager for the Win32 platform.  The request is
// accepted after synchronous validation/parent lookup; dialog and spooling
// run asynchronously on a locked OS thread so App.Print never blocks on UI.
func (p *windowsPlatform) StartPrint(ctx context.Context, request PrintRequest) (PrintJob, error) {
	if ctx == nil {
		return nil, errors.New("windows print: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrPrintUnavailable
	}
	if request.Document.MIMEType == "" || len(request.Document.Data) == 0 {
		return nil, errors.New("windows print: empty document")
	}
	if request.Options.Copies < 0 {
		return nil, errors.New("windows print: copies must not be negative")
	}
	if request.Options.Copies > math.MaxUint16 {
		return nil, fmt.Errorf("windows print: copies %d exceeds Windows limit %d", request.Options.Copies, math.MaxUint16)
	}
	for _, r := range request.Options.PageRanges {
		if r.From <= 0 || r.To < r.From {
			return nil, fmt.Errorf("windows print: invalid page range %d-%d", r.From, r.To)
		}
	}

	parent, err := p.printParent(request.Options.Parent)
	if err != nil {
		return nil, err
	}
	if parent == 0 {
		// PrintDlgExW requires a valid owner HWND (NULL is not accepted).  A
		// caller may submit a document before Run creates the primary window;
		// reject that request synchronously rather than launching an orphaned
		// dialog whose native setup error would arrive on PrintJob.Done.
		return nil, errors.New("windows print: parent window is unavailable")
	}

	// App.Print already copies the request, but keep this boundary explicit for
	// direct internal callers and for the worker's asynchronous lifetime.
	request.Document.Data = append([]byte(nil), request.Document.Data...)
	request.Options.PageRanges = append([]PrintPageRange(nil), request.Options.PageRanges...)

	job := newWindowsPrintJob()
	go p.runPrint(ctx, parent, request, job)
	watchPrintContext(ctx, job.printJob)
	return job, nil
}

// printParent resolves the backend-neutral WindowID to its owning HWND while
// retaining the parent relationship for the complete native operation.
func (p *windowsPlatform) printParent(id WindowID) (uintptr, error) {
	if p == nil {
		return 0, ErrPrintUnavailable
	}
	p.windowMu.RLock()
	defer p.windowMu.RUnlock()
	if id == 0 {
		if p.primary == nil || p.primary.hwnd == 0 {
			return 0, nil
		}
		return uintptr(p.primary.hwnd), nil
	}
	if p.primary != nil && p.primary.id == id && p.primary.hwnd != 0 {
		return uintptr(p.primary.hwnd), nil
	}
	for _, window := range p.windows {
		if window != nil && window.id == id && window.hwnd != 0 {
			return uintptr(window.hwnd), nil
		}
	}
	return 0, fmt.Errorf("windows print: parent window %d is unavailable", id)
}

func (p *windowsPlatform) runPrint(ctx context.Context, parent uintptr, request PrintRequest, job *windowsPrintJob) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// PrintDlgExW may create COM-backed property sheets.  This goroutine is
	// deliberately pinned to one OS thread, so initialize an STA on that same
	// thread and balance it before the thread is released.
	if hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded); hr == comSOK || hr == comSFalse {
		defer procCoUninitialize.Call()
	}

	if ctx.Err() != nil || job.isCanceled() {
		job.finish(context.Canceled)
		return
	}

	// A native print dialog is modal and Windows requires a message-aware OS
	// thread.  Serialize dialogs/spool setup so concurrent calls cannot compete
	// for the process's default printer UI.
	windowsPrintMu.Lock()
	defer windowsPrintMu.Unlock()

	// PrintDlgEx owns its modal loop.  GetLastActivePopup + WM_CLOSE is the
	// documented Win32 escape hatch for a caller that must cancel a common
	// dialog from another goroutine; the result is still checked against the
	// job/context after the dialog returns.
	if windowsPrintCalls.cancelDialog != nil {
		job.setAbort(func() { windowsPrintCalls.cancelDialog(parent) })
	}
	if ctx.Err() != nil || job.isCanceled() {
		job.finish(context.Canceled)
		return
	}
	result, err := showWindowsPrintDialog(parent, request, windowsPrintCalls)
	if err != nil {
		job.finish(err)
		return
	}
	if result.canceled || ctx.Err() != nil || job.isCanceled() {
		if result.hdc != 0 {
			windowsPrintCalls.deleteDC(result.hdc)
		}
		job.finish(context.Canceled)
		return
	}

	// AbortDoc is safe from the cancellation watcher while Escape/EndDoc is
	// blocked in a printer driver.  It is cleared before Done is signaled.
	job.setAbort(func() { windowsPrintCalls.abortDoc(result.hdc) })
	err = spoolWindowsDocument(ctx, request, result.hdc, job, windowsPrintCalls)
	job.finish(err)
}

type windowsPrintDialogResult struct {
	hdc      uintptr
	canceled bool
}

func showWindowsPrintDialog(parent uintptr, request PrintRequest, calls windowsPrintSyscalls) (result windowsPrintDialogResult, err error) {
	pageRanges := make([]printPageRange, maxPrintPageRanges(len(request.Options.PageRanges)))
	for i, r := range request.Options.PageRanges {
		if uint64(r.From) > math.MaxUint32 || uint64(r.To) > math.MaxUint32 {
			return result, fmt.Errorf("windows print: page range %d-%d exceeds Windows limit", r.From, r.To)
		}
		pageRanges[i] = printPageRange{nFromPage: uint32(r.From), nToPage: uint32(r.To)}
	}

	dlg := printDlgEx{
		lStructSize:    uint32(unsafe.Sizeof(printDlgEx{})),
		hwndOwner:      parent,
		flags:          pdReturnDC | pdUseDevModeCopiesAndCollate,
		nMaxPageRanges: uint32(len(pageRanges)),
		nMinPage:       1,
		nMaxPage:       printDialogMaxPage,
		nCopies:        1,
	}
	if len(request.Options.PageRanges) > 0 {
		dlg.flags |= pdPageNums
		dlg.nPageRanges = uint32(len(request.Options.PageRanges))
	}
	if request.Options.Copies > 0 {
		dlg.nCopies = uint16(request.Options.Copies)
	}
	if len(pageRanges) > 0 {
		dlg.lpPageRanges = uintptr(unsafe.Pointer(&pageRanges[0]))
	}

	hresult := calls.printDlgEx(&dlg)
	runtime.KeepAlive(pageRanges)
	if hresult != 0 {
		freePrintDialogResources(dlg, calls)
		return result, fmt.Errorf("windows print: PrintDlgExW failed (HRESULT 0x%08X)", hresult)
	}
	if dlg.dwResultAction == pdResultCancel {
		freePrintDialogResources(dlg, calls)
		return windowsPrintDialogResult{canceled: true}, nil
	}
	if dlg.dwResultAction != pdResultPrint {
		freePrintDialogResources(dlg, calls)
		return result, fmt.Errorf("windows print: PrintDlgExW returned unexpected action %d", dlg.dwResultAction)
	}
	if dlg.hDC == 0 {
		freePrintDialogResources(dlg, calls)
		return result, errors.New("windows print: PrintDlgExW returned no printer DC")
	}

	// The HDC owns the selected printer and DEVMODE.  Release the dialog's
	// memory blocks now; the HDC remains valid until spoolWindowsDocument ends.
	if dlg.hDevMode != 0 {
		calls.globalFree(dlg.hDevMode)
	}
	if dlg.hDevNames != 0 {
		calls.globalFree(dlg.hDevNames)
	}
	return windowsPrintDialogResult{hdc: dlg.hDC}, nil
}

func maxPrintPageRanges(requested int) int {
	if requested < 1 {
		// Keep the page-range controls available when the caller did not provide
		// a preselected range.  PRINTDLGEX writes the user's selection back into
		// this buffer before returning.
		return 16
	}
	return requested
}

func freePrintDialogResources(dlg printDlgEx, calls windowsPrintSyscalls) {
	if dlg.hDC != 0 {
		calls.deleteDC(dlg.hDC)
	}
	if dlg.hDevMode != 0 {
		calls.globalFree(dlg.hDevMode)
	}
	if dlg.hDevNames != 0 {
		calls.globalFree(dlg.hDevNames)
	}
}

func spoolWindowsDocument(ctx context.Context, request PrintRequest, hdc uintptr, job *windowsPrintJob, calls windowsPrintSyscalls) error {
	if hdc == 0 {
		return errors.New("windows print: nil printer DC")
	}
	// The cancellation hook may call AbortDoc on this HDC from another
	// goroutine.  Clear it before DeleteDC so a cancellation racing cleanup
	// cannot touch a released (and potentially recycled) handle.
	defer func() {
		job.clearAbort()
		calls.deleteDC(hdc)
	}()
	if ctx.Err() != nil || job.isCanceled() {
		calls.abortDoc(hdc)
		return context.Canceled
	}

	docName := request.Document.Name
	if docName == "" {
		docName = request.Options.Title
	}
	if docName == "" {
		docName = "GoGPU document"
	}
	docNameW, err := windows.UTF16PtrFromString(docName)
	if err != nil {
		return fmt.Errorf("windows print: document name: %w", err)
	}
	datatypeW, _ := windows.UTF16PtrFromString("RAW")
	info := printDocInfo{
		cbSize:       int32(unsafe.Sizeof(printDocInfo{})),
		lpszDocName:  docNameW,
		lpszDatatype: datatypeW,
	}
	if calls.startDoc(hdc, &info) <= 0 {
		return fmt.Errorf("windows print: StartDocW failed: %w", printLastError())
	}

	if ctx.Err() != nil || job.isCanceled() {
		calls.abortDoc(hdc)
		return context.Canceled
	}
	if calls.startPage(hdc) <= 0 {
		calls.abortDoc(hdc)
		return fmt.Errorf("windows print: StartPage failed: %w", printLastError())
	}

	data := request.Document.Data
	for offset := 0; offset < len(data); {
		if ctx.Err() != nil || job.isCanceled() {
			calls.abortDoc(hdc)
			return context.Canceled
		}
		end := offset + printEscapeChunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		if len(chunk) == 0 {
			break
		}
		if calls.escape(hdc, passthroughEscape, uint32(len(chunk)), uintptr(unsafe.Pointer(&chunk[0])), 0) <= 0 {
			calls.abortDoc(hdc)
			return fmt.Errorf("windows print: PASSTHROUGH failed at byte %d: %w", offset, printLastError())
		}
		offset = end
	}
	runtime.KeepAlive(data)

	if calls.endPage(hdc) <= 0 {
		calls.abortDoc(hdc)
		return fmt.Errorf("windows print: EndPage failed: %w", printLastError())
	}
	if calls.endDoc(hdc) <= 0 {
		calls.abortDoc(hdc)
		return fmt.Errorf("windows print: EndDoc failed: %w", printLastError())
	}
	return nil
}

func printLastError() error {
	err := windows.GetLastError()
	if err == nil || err == syscall.Errno(0) {
		return syscall.EIO
	}
	return err
}

// Compile-time capability assertion.  The backend is additive: other
// platforms simply omit a PrintManager implementation and remain unsupported.
var _ PrintManager = (*windowsPlatform)(nil)

// windowsPrintMu serializes native print-dialog entry.  Win32 common dialogs
// and printer drivers are process-global; overlapping modal dialogs otherwise
// race the default printer selection and make cancellation nondeterministic.
var windowsPrintMu sync.Mutex
