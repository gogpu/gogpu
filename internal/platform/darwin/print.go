//go:build darwin

package darwin

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

var (
	ErrPrintMainThread = errors.New("darwin: print operation requires the main thread")
	ErrPrintSetup      = errors.New("darwin: print operation setup failed")
)

// PrintPageRange is an inclusive, one-based range. Ranges are normalized and
// merged before PDFKit receives the document.
type PrintPageRange struct {
	From int
	To   int
}

// PrintRequest is the native-facing macOS print request. Data is copied by
// the caller-facing App.Print contract before this type is constructed.
type PrintRequest struct {
	Name       string
	Data       []byte
	Title      string
	Copies     int
	PageRanges []PrintPageRange
}

var pdfKitState struct {
	once sync.Once
	err  error
	lib  unsafe.Pointer
}

func initPDFKit() error {
	pdfKitState.once.Do(func() {
		if err := initRuntime(); err != nil {
			pdfKitState.err = err
			return
		}
		pdfKitState.lib, pdfKitState.err = ffi.LoadLibrary(
			"/System/Library/Frameworks/PDFKit.framework/PDFKit")
	})
	return pdfKitState.err
}

var printDelegateState struct {
	once      sync.Once
	class     Class
	didRun    SEL
	err       error
	callbacks sync.Map // uintptr(contextInfo) -> *PrintHandle
}

var nextPrintToken atomic.Uint64

func initPrintDelegate() error {
	printDelegateState.once.Do(func() {
		if err := initRuntime(); err != nil {
			printDelegateState.err = err
			return
		}
		initSelectors()
		initClasses()
		super := classes.NSObject
		if super == 0 {
			printDelegateState.err = ErrPrintSetup
			return
		}
		cls := AllocateClassPair(super, "GoGPUPrintDelegate")
		if cls == 0 {
			cls = GetClass("GoGPUPrintDelegate")
		}
		if cls == 0 {
			printDelegateState.err = ErrPrintSetup
			return
		}
		didRun := RegisterSelector("gogpuPrintOperationDidRun:success:contextInfo:")
		imp := ffi.NewCallback(func(_self, _sel, _operation, success, contextInfo uintptr) uintptr {
			value, ok := printDelegateState.callbacks.LoadAndDelete(contextInfo)
			if !ok {
				return 0
			}
			h := value.(*PrintHandle)
			h.mu.Lock()
			done := h.done
			h.mu.Unlock()
			if done != nil {
				done(success != 0)
			}
			return 0
		})
		if !ClassAddMethod(cls, didRun, imp, "v@:@B^v") {
			if GetClass("GoGPUPrintDelegate") == 0 {
				printDelegateState.err = ErrPrintSetup
				return
			}
		}
		if GetClass("GoGPUPrintDelegate") == 0 {
			RegisterClassPair(cls)
		}
		printDelegateState.class = cls
		printDelegateState.didRun = didRun
	})
	return printDelegateState.err
}

// PrintHandle owns all retained PDFKit/AppKit objects for one accepted print
// operation. Its callback runs after NSPrintOperation has finished or the
// user has dismissed the print panel. Close must happen before the caller's
// PrintJob.Done terminal value is published.
type PrintHandle struct {
	mu sync.Mutex

	document      ID
	filtered      ID
	printInfo     ID
	operation     ID
	delegate      ID
	parent        ID
	callbackToken uintptr
	started       bool
	closed        bool
	done          func(success bool)

	closeOnce sync.Once
}

// NewPrintHandle parses the complete PDF with PDFKit and prepares an
// NSPrintOperation. It does not show the native print panel; Run starts the
// asynchronous modal operation. Call on AppKit's main thread.
//
//nolint:gocognit // PDFKit/AppKit setup must retain and unwind one native lifecycle.
func NewPrintHandle(request PrintRequest, parent ID) (*PrintHandle, error) {
	if !IsMainThread() {
		return nil, ErrPrintMainThread
	}
	if request.Copies < 0 {
		return nil, fmt.Errorf("%w: negative copies", ErrPrintSetup)
	}
	for _, r := range request.PageRanges {
		if r.From <= 0 || r.To < r.From {
			return nil, fmt.Errorf("%w: page range %d-%d", ErrPrintSetup, r.From, r.To)
		}
	}
	if len(request.Data) == 0 {
		return nil, fmt.Errorf("%w: empty PDF data", ErrPrintSetup)
	}
	if err := initPDFKit(); err != nil {
		return nil, fmt.Errorf("%w: PDFKit: %w", ErrPrintSetup, err)
	}
	initSelectors()
	initClasses()

	pdfClass := GetClass("PDFDocument")
	printInfoClass := GetClass("NSPrintInfo")
	if pdfClass == 0 || printInfoClass == 0 {
		return nil, fmt.Errorf("%w: PDFDocument/NSPrintInfo unavailable", ErrPrintSetup)
	}

	dataClass := GetClass("NSData")
	if dataClass == 0 {
		return nil, fmt.Errorf("%w: NSData unavailable", ErrPrintSetup)
	}
	dataSel := RegisterSelector("dataWithBytes:length:")
	data := ID(dataClass).SendPtrs(dataSel,
		uintptr(unsafe.Pointer(&request.Data[0])), uintptr(len(request.Data)))
	runtime.KeepAlive(request.Data)
	if data == 0 {
		return nil, fmt.Errorf("%w: NSData creation failed", ErrPrintSetup)
	}

	allocSel := selectors.alloc
	initWithDataSel := RegisterSelector("initWithData:")
	document := ID(pdfClass).Send(allocSel).SendPtr(initWithDataSel, data.Ptr())
	if document == 0 {
		return nil, fmt.Errorf("%w: PDFDocument rejected data", ErrPrintSetup)
	}

	selected := document
	filtered, err := filterPDFDocument(document, request.PageRanges)
	if err != nil {
		document.Send(selectors.release)
		return nil, err
	}
	if filtered != document {
		selected = filtered
	}

	printInfo, err := newPrintInfo(request.Copies)
	if err != nil {
		if filtered != document {
			filtered.Send(selectors.release)
		}
		document.Send(selectors.release)
		return nil, err
	}

	// kPDFPrintPageScaleToFit = 1; autoRotate = YES. PDFDocument's print
	// operation paginates every selected page and displays the native panel.
	operation := selected.SendPtrs(
		RegisterSelector("printOperationForPrintInfo:scalingMode:autoRotate:"),
		printInfo.Ptr(), 1, 1)
	if operation == 0 {
		printInfo.Send(selectors.release)
		if filtered != document {
			filtered.Send(selectors.release)
		}
		document.Send(selectors.release)
		return nil, fmt.Errorf("%w: PDFDocument could not create NSPrintOperation", ErrPrintSetup)
	}
	// The PDFDocument factory returns an autoreleased operation. Retain it for
	// the lifetime of the asynchronous print job.
	operation.Send(selectors.retain)

	if request.Title != "" || request.Name != "" {
		title := request.Title
		if title == "" {
			title = request.Name
		}
		if nsTitle := NewNSString(title); nsTitle != nil {
			operation.SendPtr(RegisterSelector("setJobTitle:"), nsTitle.ID().Ptr())
			nsTitle.Release()
		}
	}
	// Keep the callback on the main thread. Setting this to true would allow
	// AppKit to invoke the delegate on a detached printing thread, which would
	// move resource release and Done publication off the documented lifecycle.
	operation.SendBool(RegisterSelector("setCanSpawnSeparateThread:"), false)

	if err := initPrintDelegate(); err != nil {
		operation.Send(selectors.release)
		printInfo.Send(selectors.release)
		if filtered != document {
			filtered.Send(selectors.release)
		}
		document.Send(selectors.release)
		return nil, fmt.Errorf("%w: delegate: %w", ErrPrintSetup, err)
	}
	delegate := ID(printDelegateState.class).Send(selectors.alloc).Send(selectors.init)
	if delegate == 0 {
		operation.Send(selectors.release)
		printInfo.Send(selectors.release)
		if filtered != document {
			filtered.Send(selectors.release)
		}
		document.Send(selectors.release)
		return nil, fmt.Errorf("%w: delegate allocation failed", ErrPrintSetup)
	}

	h := &PrintHandle{
		document:  document,
		filtered:  filtered,
		printInfo: printInfo,
		operation: operation,
		delegate:  delegate,
		parent:    parent,
	}
	if parent != 0 {
		parent.Send(selectors.retain)
	}
	return h, nil
}

// Run starts the modal print panel and returns immediately after AppKit has
// accepted it. The callback receives true only when the native operation
// completed successfully. Run must execute on the main thread.
func (h *PrintHandle) Run(done func(success bool)) error {
	if h == nil {
		return ErrPrintSetup
	}
	if !IsMainThread() {
		return ErrPrintMainThread
	}
	h.mu.Lock()
	if h.closed || h.started {
		h.mu.Unlock()
		return fmt.Errorf("%w: operation already started", ErrPrintSetup)
	}
	h.started = true
	h.done = done
	token := nextPrintToken.Add(1)
	if token == 0 {
		token = nextPrintToken.Add(1)
	}
	h.callbackToken = uintptr(token)
	printDelegateState.callbacks.Store(h.callbackToken, h)
	h.mu.Unlock()

	// runOperationModalForWindow:delegate:didRunSelector:contextInfo: is a
	// void-returning method; SendPtrs supplies its four pointer-sized args.
	h.operation.SendPtrs(
		RegisterSelector("runOperationModalForWindow:delegate:didRunSelector:contextInfo:"),
		h.parent.Ptr(), h.delegate.Ptr(), printDelegateState.didRun.SELPtr(), h.callbackToken)
	return nil
}

// Cancel asks the current print panel to close. NSPrintOperation does not
// expose a cancellation method; once the panel is gone the terminal callback
// still reports false and the platform job maps a caller cancellation to
// context.Canceled. Callers may invoke this from any goroutine.
func (h *PrintHandle) Cancel() {
	if h == nil {
		return
	}
	if !IsMainThread() {
		_ = PerformOnMain(h.Cancel, false)
		return
	}
	h.mu.Lock()
	if h.closed || !h.started {
		h.mu.Unlock()
		return
	}
	op := h.operation
	h.mu.Unlock()
	if op == 0 {
		return
	}
	panel := op.Send(RegisterSelector("printPanel"))
	if panel != 0 {
		panel.SendPtr(RegisterSelector("cancel:"), 0)
	}
}

// Close releases all native resources retained by NewPrintHandle. It is
// idempotent and must run before the platform publishes PrintJob.Done.
func (h *PrintHandle) Close() {
	if h == nil {
		return
	}
	if !IsMainThread() {
		_ = PerformOnMain(h.Close, true)
		return
	}
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closed = true
		token := h.callbackToken
		document, filtered, info, operation, delegate, parent :=
			h.document, h.filtered, h.printInfo, h.operation, h.delegate, h.parent
		h.document, h.filtered, h.printInfo, h.operation, h.delegate, h.parent = 0, 0, 0, 0, 0, 0
		h.mu.Unlock()
		if token != 0 {
			printDelegateState.callbacks.Delete(token)
		}
		if operation != 0 {
			operation.Send(selectors.release)
		}
		if info != 0 {
			info.Send(selectors.release)
		}
		if filtered != 0 && filtered != document {
			filtered.Send(selectors.release)
		}
		if document != 0 {
			document.Send(selectors.release)
		}
		if delegate != 0 {
			delegate.Send(selectors.release)
		}
		if parent != 0 {
			parent.Send(selectors.release)
		}
	})
}

func newPrintInfo(copies int) (ID, error) {
	infoClass := GetClass("NSPrintInfo")
	if infoClass == 0 {
		return 0, fmt.Errorf("%w: NSPrintInfo unavailable", ErrPrintSetup)
	}
	shared := ID(infoClass).Send(RegisterSelector("sharedPrintInfo"))
	if shared == 0 {
		return 0, fmt.Errorf("%w: shared NSPrintInfo unavailable", ErrPrintSetup)
	}
	if copies == 0 {
		printInfoCopy := shared.Send(RegisterSelector("copy"))
		if printInfoCopy == 0 {
			return 0, fmt.Errorf("%w: NSPrintInfo copy failed", ErrPrintSetup)
		}
		return printInfoCopy, nil
	}

	dictionary := shared.Send(RegisterSelector("dictionary"))
	mutable := dictionary.Send(RegisterSelector("mutableCopy"))
	if mutable == 0 {
		return 0, fmt.Errorf("%w: NSPrintInfo dictionary copy failed", ErrPrintSetup)
	}
	number := ID(GetClass("NSNumber")).SendPtrs(RegisterSelector("numberWithInteger:"), uintptr(copies))
	key := NewNSString("NSPrintCopies")
	if number == 0 || key == nil {
		mutable.Send(selectors.release)
		return 0, fmt.Errorf("%w: NSPrintCopies value failed", ErrPrintSetup)
	}
	mutable.SendPtrs(RegisterSelector("setObject:forKey:"), number.Ptr(), key.ID().Ptr())
	key.Release()
	info := ID(infoClass).Send(selectors.alloc).SendPtrs(
		RegisterSelector("initWithDictionary:"), mutable.Ptr())
	mutable.Send(selectors.release)
	if info == 0 {
		return 0, fmt.Errorf("%w: NSPrintInfo initialization failed", ErrPrintSetup)
	}
	return info, nil
}

func filterPDFDocument(document ID, ranges []PrintPageRange) (ID, error) {
	if len(ranges) == 0 {
		return document, nil
	}
	count := document.GetUint64(RegisterSelector("pageCount"))
	merged := normalizePrintRanges(ranges)
	if count == 0 {
		return 0, fmt.Errorf("%w: PDF contains no pages", ErrPrintSetup)
	}

	selectedCount := uint64(0)
	for page := uint64(1); page <= count; page++ {
		if printPageSelected(int(page), merged) {
			selectedCount++
		}
	}
	if selectedCount == 0 {
		return 0, fmt.Errorf("%w: page ranges select no pages", ErrPrintSetup)
	}
	if selectedCount == count && printPageSelected(1, merged) {
		all := true
		for page := uint64(1); page <= count; page++ {
			if !printPageSelected(int(page), merged) {
				all = false
				break
			}
		}
		if all {
			return document, nil
		}
	}

	pdfClass := GetClass("PDFDocument")
	filtered := ID(pdfClass).Send(selectors.alloc).Send(selectors.init)
	if filtered == 0 {
		return 0, fmt.Errorf("%w: filtered PDFDocument allocation failed", ErrPrintSetup)
	}
	pageAt := RegisterSelector("pageAtIndex:")
	insert := RegisterSelector("insertPage:atIndex:")
	index := uint64(0)
	for page := uint64(1); page <= count; page++ {
		if !printPageSelected(int(page), merged) {
			continue
		}
		pdfPage := document.SendPtrs(pageAt, uintptr(page-1))
		if pdfPage == 0 {
			filtered.Send(selectors.release)
			return 0, fmt.Errorf("%w: PDF page %d unavailable", ErrPrintSetup, page)
		}
		filtered.SendPtrs(insert, pdfPage.Ptr(), uintptr(index))
		index++
	}
	return filtered, nil
}

func normalizePrintRanges(ranges []PrintPageRange) []PrintPageRange {
	copyRanges := append([]PrintPageRange(nil), ranges...)
	sort.Slice(copyRanges, func(i, j int) bool {
		if copyRanges[i].From == copyRanges[j].From {
			return copyRanges[i].To < copyRanges[j].To
		}
		return copyRanges[i].From < copyRanges[j].From
	})
	merged := make([]PrintPageRange, 0, len(copyRanges))
	for _, r := range copyRanges {
		if r.To < r.From {
			continue
		}
		if len(merged) == 0 {
			merged = append(merged, r)
			continue
		}
		last := &merged[len(merged)-1]
		if r.From > last.To {
			// Avoid last.To+1 overflow for a direct internal caller passing the
			// largest representable page number.
			maxInt := int(^uint(0) >> 1)
			if last.To == maxInt || r.From != last.To+1 {
				merged = append(merged, r)
				continue
			}
		}
		if r.To > last.To {
			last.To = r.To
		}
	}
	return merged
}

func printPageSelected(page int, ranges []PrintPageRange) bool {
	for _, r := range ranges {
		if page < r.From {
			return false
		}
		if page <= r.To {
			return true
		}
	}
	return false
}
