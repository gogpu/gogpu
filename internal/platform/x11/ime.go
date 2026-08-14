//go:build linux

package x11

// XIM is an optional Xlib service. This file deliberately loads it through
// goffi rather than importing Xlib headers or using cgo, preserving gogpu's
// pure-Go/no-CGO platform policy. If any required symbol, locale, or context
// is unavailable, callers continue through the existing xkb text path.

import (
	"encoding/binary"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"github.com/gogpu/gpucontext"
)

const (
	ximPreeditArea      = 0x0001
	ximPreeditCallbacks = 0x0002
	ximPreeditPosition  = 0x0004
	ximPreeditNothing   = 0x0008
	ximPreeditNone      = 0x0010
	ximStatusNothing    = 0x0400

	xBufferOverflow   = -1
	xLookupNone       = 1
	xLookupChars      = 2
	xLookupKeySym     = 3
	xLookupBoth       = 4
	xKeyPressEvent    = 2
	xKeyReleaseEvent  = 3
	xNativeEventBytes = 192 // largest XEvent ABI; 32-bit callers use its prefix.
)

// ximCallback is the XIMCallback ABI: two pointer-sized fields.
type ximCallback struct {
	clientData uintptr
	callback   uintptr
}

// ximPreeditDrawData is the fixed prefix of XIMPreeditDrawCallbackStruct.
// The XIMText pointer is eight-byte aligned on LP64 and follows three C ints.
type ximPreeditDrawData struct {
	Caret        int32
	ChangeFirst  int32
	ChangeLength int32
	Text         uintptr
}

// xlibPoint is the XPoint shape used by XNSpotLocation.
type xlibPoint struct {
	X int16
	Y int16
}

// x11XIM stores loaded Xlib symbols and prepared call interfaces. A single
// value belongs to one Xlib Display* and owns its XIM/XIC lifetime.
type x11XIM struct {
	lib     unsafe.Pointer
	display uintptr

	fnSetLocaleModifiers unsafe.Pointer
	fnOpenIM             unsafe.Pointer
	fnCloseIM            unsafe.Pointer
	fnCreateIC           unsafe.Pointer
	fnDestroyIC          unsafe.Pointer
	fnSetICFocus         unsafe.Pointer
	fnUnsetICFocus       unsafe.Pointer
	fnFilterEvent        unsafe.Pointer
	fnUTF8Lookup         unsafe.Pointer
	fnUTF8Reset          unsafe.Pointer
	fnSetICValues        unsafe.Pointer
	fnCreateNested       unsafe.Pointer
	fnFree               unsafe.Pointer

	cifSetLocaleModifiers *types.CallInterface
	cifOpenIM             *types.CallInterface
	cifCloseIM            *types.CallInterface
	cifCreateIC           *types.CallInterface
	cifCreateICFallback   *types.CallInterface
	cifDestroyIC          *types.CallInterface
	cifSetICFocus         *types.CallInterface
	cifUnsetICFocus       *types.CallInterface
	cifFilterEvent        *types.CallInterface
	cifUTF8Lookup         *types.CallInterface
	cifUTF8Reset          *types.CallInterface
	cifSetICValues        *types.CallInterface
	cifCreateNested       *types.CallInterface
	cifCreateNestedSpot   *types.CallInterface
	cifFree               *types.CallInterface

	im uintptr
}

// x11IME is per-window state around an XIC. All calls occur on the event
// thread in normal operation, but setters may be called by a widget goroutine;
// the mutex protects state and prevents disabling from retaining context.
type x11IME struct {
	xlib       *x11XIM
	window     ResourceID
	callbackID uintptr
	ic         uintptr

	mu               sync.Mutex
	enabled          bool
	focused          bool
	composing        bool
	preeditDone      bool
	preedit          string
	preeditSupported bool
	cursorSupported  bool
	scale            float64
	queueFn          func(PlatformEvent)
	cursorArea       gpucontext.IMECursorArea
	cursorAreaSet    bool
	purpose          gpucontext.ContentPurpose
	hints            gpucontext.ContentHint
	surrounding      gpucontext.IMESurroundingText
}

var (
	ximCallbackOnce sync.Once
	ximStartCB      uintptr
	ximDrawCB       uintptr
	ximDoneCB       uintptr
	ximCaretCB      uintptr
	ximRegistry     sync.Map // map[uintptr]*x11IME, keyed by XIMCallback client data
	nextXIMID       atomic.Uintptr
)

func initXIMCallbacks() {
	ximStartCB = ffi.NewCallback(func(_ uintptr, clientData uintptr, _ uintptr) int32 {
		if ime, ok := lookupXIM(clientData); ok {
			return ime.preeditStart()
		}
		return -1
	})
	ximDrawCB = ffi.NewCallback(func(_ uintptr, clientData uintptr, callData uintptr) {
		if ime, ok := lookupXIM(clientData); ok {
			ime.preeditDraw(callData)
		}
	})
	ximDoneCB = ffi.NewCallback(func(_ uintptr, clientData uintptr, _ uintptr) {
		if ime, ok := lookupXIM(clientData); ok {
			ime.preeditDoneCallback()
		}
	})
	ximCaretCB = ffi.NewCallback(func(_ uintptr, clientData uintptr, _ uintptr) {
		if ime, ok := lookupXIM(clientData); ok {
			ime.preeditCaret()
		}
	})
}

func lookupXIM(id uintptr) (*x11IME, bool) {
	value, ok := ximRegistry.Load(id)
	if !ok {
		return nil, false
	}
	ime, ok := value.(*x11IME)
	return ime, ok
}

func registerXIM(ime *x11IME) {
	id := nextXIMID.Add(1)
	ime.callbackID = id
	ximRegistry.Store(id, ime)
}

func unregisterXIM(ime *x11IME) {
	if ime != nil && ime.callbackID != 0 {
		ximRegistry.Delete(ime.callbackID)
		ime.callbackID = 0
	}
}

func nativeUnsignedLong() *types.TypeDescriptor {
	if strconv.IntSize == 32 {
		return types.UInt32TypeDescriptor
	}
	return types.UInt64TypeDescriptor
}

func prepareXIMCall(cif **types.CallInterface, ret *types.TypeDescriptor, args []*types.TypeDescriptor, variadicFixed int) bool {
	value := &types.CallInterface{}
	var err error
	if variadicFixed >= 0 {
		err = ffi.PrepareVariadicCallInterface(value, types.DefaultCall, variadicFixed, ret, args)
	} else {
		err = ffi.PrepareCallInterface(value, types.DefaultCall, ret, args)
	}
	if err != nil {
		logger().Debug("x11: XIM ffi interface unavailable", "error", err)
		return false
	}
	*cif = value
	return true
}

func lookupXIMSymbols(lib unsafe.Pointer) (*x11XIM, bool) {
	if lib == nil {
		return nil, false
	}
	lookup := func(name string) unsafe.Pointer {
		symbol, err := ffi.GetSymbol(lib, name)
		if err != nil {
			return nil
		}
		return symbol
	}
	x := &x11XIM{lib: lib}
	x.fnSetLocaleModifiers = lookup("XSetLocaleModifiers")
	x.fnOpenIM = lookup("XOpenIM")
	x.fnCloseIM = lookup("XCloseIM")
	x.fnCreateIC = lookup("XCreateIC")
	x.fnDestroyIC = lookup("XDestroyIC")
	x.fnSetICFocus = lookup("XSetICFocus")
	x.fnUnsetICFocus = lookup("XUnsetICFocus")
	x.fnFilterEvent = lookup("XFilterEvent")
	x.fnUTF8Lookup = lookup("Xutf8LookupString")
	x.fnUTF8Reset = lookup("Xutf8ResetIC")
	x.fnSetICValues = lookup("XSetICValues")
	x.fnCreateNested = lookup("XVaCreateNestedList")
	x.fnFree = lookup("XFree")
	// Keep the core XIM path independent from optional preedit/candidate
	// helpers. Some minimal Xlib/XIM deployments expose XOpenIM and lookup but
	// omit XVaCreateNestedList/XSetICValues; those contexts still provide useful
	// committed UTF-8 text through XIMPreeditNothing.
	if x.fnOpenIM == nil || x.fnCloseIM == nil || x.fnCreateIC == nil ||
		x.fnDestroyIC == nil || x.fnSetICFocus == nil || x.fnUnsetICFocus == nil ||
		x.fnFilterEvent == nil || x.fnUTF8Lookup == nil {
		return nil, false
	}

	ptr := types.PointerTypeDescriptor
	ulong := nativeUnsignedLong()
	intType := types.SInt32TypeDescriptor
	if !prepareXIMCall(&x.cifSetLocaleModifiers, ptr, []*types.TypeDescriptor{ptr}, -1) && x.fnSetLocaleModifiers != nil {
		// Locale modifiers are a best-effort helper; XOpenIM remains usable.
		x.fnSetLocaleModifiers = nil
	}
	if !prepareXIMCall(&x.cifOpenIM, ptr, []*types.TypeDescriptor{ptr, ptr, ptr, ptr}, -1) ||
		!prepareXIMCall(&x.cifCloseIM, intType, []*types.TypeDescriptor{ptr}, -1) ||
		!prepareXIMCall(&x.cifCreateIC, ptr, []*types.TypeDescriptor{
			ptr, ptr, ptr, ulong, ptr, ulong, ptr, ulong, ptr, ptr,
		}, 1) ||
		!prepareXIMCall(&x.cifCreateICFallback, ptr, []*types.TypeDescriptor{
			ptr, ptr, ptr, ulong, ptr, ulong, ptr, ulong,
		}, 1) ||
		!prepareXIMCall(&x.cifDestroyIC, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}, -1) ||
		!prepareXIMCall(&x.cifSetICFocus, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}, -1) ||
		!prepareXIMCall(&x.cifUnsetICFocus, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}, -1) ||
		!prepareXIMCall(&x.cifFilterEvent, intType, []*types.TypeDescriptor{ptr, ulong}, -1) ||
		!prepareXIMCall(&x.cifUTF8Lookup, intType, []*types.TypeDescriptor{ptr, ptr, ptr, intType, ulong, ptr}, -1) {
		return nil, false
	}
	if x.fnSetICValues != nil {
		_ = prepareXIMCall(&x.cifSetICValues, ptr, []*types.TypeDescriptor{ptr, ptr, ptr, ptr}, 1)
	}
	if x.fnCreateNested != nil {
		_ = prepareXIMCall(&x.cifCreateNested, ptr, []*types.TypeDescriptor{
			intType, ptr, ptr, ptr, ptr, ptr, ptr, ptr, ptr, ptr,
		}, 1)
		_ = prepareXIMCall(&x.cifCreateNestedSpot, ptr, []*types.TypeDescriptor{
			intType, ptr, ptr, ptr,
		}, 1)
	}
	if x.fnFree != nil {
		_ = prepareXIMCall(&x.cifFree, intType, []*types.TypeDescriptor{ptr}, -1)
	}
	if x.fnUTF8Reset != nil {
		if !prepareXIMCall(&x.cifUTF8Reset, ptr, []*types.TypeDescriptor{ptr}, -1) {
			x.fnUTF8Reset = nil
		}
	}
	return x, true
}

func newX11IME(handle *xlibHandle, window ResourceID, scale float64, queueFn func(PlatformEvent)) *x11IME {
	if handle == nil || handle.lib == nil || handle.display == 0 {
		return nil
	}
	xlib, ok := lookupXIMSymbols(handle.lib)
	if !ok {
		return nil
	}
	xlib.display = handle.display
	if xlib.fnSetLocaleModifiers != nil {
		empty := cString("")
		emptyPtr := uintptr(unsafe.Pointer(&empty[0]))
		args := []unsafe.Pointer{unsafe.Pointer(&emptyPtr)}
		var ignored uintptr
		_, _ = ffi.CallFunction(xlib.cifSetLocaleModifiers, xlib.fnSetLocaleModifiers, unsafe.Pointer(&ignored), args)
		runtime.KeepAlive(empty)
	}

	display := xlib.display
	var database, resourceName, resourceClass uintptr
	var im uintptr
	_, err := ffi.CallFunction(xlib.cifOpenIM, xlib.fnOpenIM, unsafe.Pointer(&im), []unsafe.Pointer{
		unsafe.Pointer(&display), unsafe.Pointer(&database), unsafe.Pointer(&resourceName), unsafe.Pointer(&resourceClass),
	})
	if err != nil || im == 0 {
		logger().Info("x11: XOpenIM unavailable", "error", err)
		return nil
	}
	xlib.im = im
	ime := &x11IME{xlib: xlib, window: window, scale: scale, queueFn: queueFn}
	registerXIM(ime)
	ximCallbackOnce.Do(initXIMCallbacks)

	ic, callbacks := xlib.createIC(window, ime.callbackID)
	if ic == 0 {
		unregisterXIM(ime)
		xlib.closeIM()
		return nil
	}
	ime.ic = ic
	ime.preeditSupported = callbacks
	ime.cursorSupported = xlib.cifSetICValues != nil && xlib.cifCreateNestedSpot != nil
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	ime.scale = scale
	return ime
}

func (x *x11XIM) closeIM() {
	if x == nil || x.im == 0 || x.fnCloseIM == nil {
		return
	}
	im := x.im
	var result int32
	_, _ = ffi.CallFunction(x.cifCloseIM, x.fnCloseIM, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&im)})
	x.im = 0
}

func (x *x11XIM) destroyIC(ic uintptr) {
	if x == nil || ic == 0 || x.fnDestroyIC == nil {
		return
	}
	ptr := ic
	_, _ = ffi.CallFunction(x.cifDestroyIC, x.fnDestroyIC, nil, []unsafe.Pointer{unsafe.Pointer(&ptr)})
}

func cString(value string) []byte {
	return append([]byte(value), 0)
}

func (x *x11XIM) createNestedCallbacks(clientData uintptr) uintptr {
	if x == nil || x.fnCreateNested == nil || x.cifCreateNested == nil ||
		x.fnFree == nil || x.cifFree == nil {
		return 0
	}
	ximCallbackOnce.Do(initXIMCallbacks)
	start := ximCallback{clientData: clientData, callback: ximStartCB}
	draw := ximCallback{clientData: clientData, callback: ximDrawCB}
	done := ximCallback{clientData: clientData, callback: ximDoneCB}
	caret := ximCallback{clientData: clientData, callback: ximCaretCB}
	nameStart := cString("preeditStartCallback")
	nameDraw := cString("preeditDrawCallback")
	nameDone := cString("preeditDoneCallback")
	nameCaret := cString("preeditCaretCallback")
	startName, drawName, doneName, caretName := uintptr(unsafe.Pointer(&nameStart[0])), uintptr(unsafe.Pointer(&nameDraw[0])), uintptr(unsafe.Pointer(&nameDone[0])), uintptr(unsafe.Pointer(&nameCaret[0]))
	startPtr, drawPtr, donePtr, caretPtr := uintptr(unsafe.Pointer(&start)), uintptr(unsafe.Pointer(&draw)), uintptr(unsafe.Pointer(&done)), uintptr(unsafe.Pointer(&caret))
	unused := int32(0)
	var result uintptr
	args := []unsafe.Pointer{
		unsafe.Pointer(&unused),
		unsafe.Pointer(&startName), unsafe.Pointer(&startPtr),
		unsafe.Pointer(&drawName), unsafe.Pointer(&drawPtr),
		unsafe.Pointer(&doneName), unsafe.Pointer(&donePtr),
		unsafe.Pointer(&caretName), unsafe.Pointer(&caretPtr),
		unsafe.Pointer(new(uintptr)),
	}
	_, err := ffi.CallFunction(x.cifCreateNested, x.fnCreateNested, unsafe.Pointer(&result), args)
	runtime.KeepAlive([]any{nameStart, nameDraw, nameDone, nameCaret, start, draw, done, caret})
	if err != nil {
		return 0
	}
	return result
}

func (x *x11XIM) free(value uintptr) {
	if x == nil || value == 0 || x.fnFree == nil || x.cifFree == nil {
		return
	}
	var result int32
	ptr := value
	_, _ = ffi.CallFunction(x.cifFree, x.fnFree, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&ptr)})
}

func (x *x11XIM) createIC(window ResourceID, clientData uintptr) (uintptr, bool) {
	if x == nil || x.im == 0 {
		return 0, false
	}
	styleName := cString("inputStyle")
	clientName := cString("clientWindow")
	focusName := cString("focusWindow")
	preeditName := cString("preeditAttributes")
	style := uintptr(ximPreeditCallbacks | ximStatusNothing)
	clientWindow := uintptr(window)
	focusWindow := clientWindow
	stylePtr, clientPtr, focusPtr := uintptr(unsafe.Pointer(&styleName[0])), uintptr(unsafe.Pointer(&clientName[0])), uintptr(unsafe.Pointer(&focusName[0]))
	preeditNamePtr := uintptr(unsafe.Pointer(&preeditName[0]))
	im := x.im
	windowStyle, windowClient, windowFocus := style, clientWindow, focusWindow
	try := func(preeditAttrs uintptr) uintptr {
		args := make([]unsafe.Pointer, 0, 10)
		args = append(args,
			unsafe.Pointer(&im),
			unsafe.Pointer(&stylePtr), unsafe.Pointer(&windowStyle),
			unsafe.Pointer(&clientPtr), unsafe.Pointer(&windowClient),
			unsafe.Pointer(&focusPtr), unsafe.Pointer(&windowFocus),
			unsafe.Pointer(&preeditNamePtr), unsafe.Pointer(&preeditAttrs),
			unsafe.Pointer(new(uintptr)),
		)
		var result uintptr
		_, _ = ffi.CallFunction(x.cifCreateIC, x.fnCreateIC, unsafe.Pointer(&result), args)
		return result
	}
	tryFallback := func() uintptr {
		args := []unsafe.Pointer{
			unsafe.Pointer(&im),
			unsafe.Pointer(&stylePtr), unsafe.Pointer(&windowStyle),
			unsafe.Pointer(&clientPtr), unsafe.Pointer(&windowClient),
			unsafe.Pointer(&focusPtr), unsafe.Pointer(&windowFocus),
			unsafe.Pointer(new(uintptr)),
		}
		var result uintptr
		_, _ = ffi.CallFunction(x.cifCreateICFallback, x.fnCreateIC, unsafe.Pointer(&result), args)
		return result
	}

	preeditAttrs := x.createNestedCallbacks(clientData)
	ic := uintptr(0)
	if preeditAttrs != 0 {
		ic = try(preeditAttrs)
		x.free(preeditAttrs)
	}
	if ic != 0 {
		return ic, true
	}

	// Some XIMs do not implement callback preedit styles. Keep commit/UTF-8
	// lookup available with the standard no-preedit fallback.
	style = uintptr(ximPreeditNothing | ximStatusNothing)
	windowStyle = style
	ic = tryFallback()
	return ic, false
}

func (x *x11XIM) setFocus(ic uintptr, focused bool) {
	if x == nil || ic == 0 {
		return
	}
	ptr := ic
	fn, cif := x.fnUnsetICFocus, x.cifUnsetICFocus
	if focused {
		fn, cif = x.fnSetICFocus, x.cifSetICFocus
	}
	if fn != nil {
		_, _ = ffi.CallFunction(cif, fn, nil, []unsafe.Pointer{unsafe.Pointer(&ptr)})
	}
}

func (x *x11XIM) filterEvent(ic uintptr, raw *[xNativeEventBytes]byte, window ResourceID) bool {
	if x == nil || ic == 0 || x.fnFilterEvent == nil {
		return false
	}
	eventPtr := uintptr(unsafe.Pointer(&raw[0]))
	nativeWindow := uintptr(window)
	var result int32
	_, _ = ffi.CallFunction(x.cifFilterEvent, x.fnFilterEvent, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&eventPtr), unsafe.Pointer(&nativeWindow)})
	return result != 0
}

func (x *x11XIM) lookupString(ic uintptr, raw *[xNativeEventBytes]byte) string {
	if x == nil || ic == 0 || x.fnUTF8Lookup == nil {
		return ""
	}
	capacity := 256
	for attempt := 0; attempt < 2; attempt++ {
		buffer := make([]byte, capacity)
		bufferPtr := uintptr(unsafe.Pointer(&buffer[0]))
		eventPtr := uintptr(unsafe.Pointer(&raw[0]))
		icPtr := ic
		length := int32(capacity)
		keysym := uintptr(0)
		status := int32(xLookupNone)
		var result int32
		_, _ = ffi.CallFunction(x.cifUTF8Lookup, x.fnUTF8Lookup, unsafe.Pointer(&result), []unsafe.Pointer{
			unsafe.Pointer(&icPtr), unsafe.Pointer(&eventPtr), unsafe.Pointer(&bufferPtr),
			unsafe.Pointer(&length), unsafe.Pointer(&keysym), unsafe.Pointer(&status),
		})
		runtime.KeepAlive(buffer)
		if status == xBufferOverflow {
			if result <= 0 || int(result) > 1<<20 {
				return ""
			}
			capacity = int(result)
			continue
		}
		if result < 0 {
			return ""
		}
		if status != xLookupChars && status != xLookupBoth {
			return ""
		}
		if int(result) > len(buffer) {
			result = int32(len(buffer))
		}
		value := string(buffer[:result])
		if !utf8.ValidString(value) {
			return strings.ToValidUTF8(value, "")
		}
		return value
	}
	return ""
}

func (x *x11XIM) reset(ic uintptr) {
	if x == nil || ic == 0 || x.fnUTF8Reset == nil {
		return
	}
	ptr := ic
	var result uintptr
	_, _ = ffi.CallFunction(x.cifUTF8Reset, x.fnUTF8Reset, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&ptr)})
}

func (x *x11XIM) setSpot(ic uintptr, area gpucontext.IMECursorArea, scale float64) {
	if x == nil || ic == 0 || x.fnSetICValues == nil || x.cifSetICValues == nil ||
		x.fnCreateNested == nil || x.cifCreateNestedSpot == nil {
		return
	}
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	xPos := math.Round(area.X * scale)
	yPos := math.Round((area.Y + area.Height) * scale)
	if xPos < math.MinInt16 {
		xPos = math.MinInt16
	}
	if xPos > math.MaxInt16 {
		xPos = math.MaxInt16
	}
	if yPos < math.MinInt16 {
		yPos = math.MinInt16
	}
	if yPos > math.MaxInt16 {
		yPos = math.MaxInt16
	}
	point := xlibPoint{X: int16(xPos), Y: int16(yPos)}
	name := cString("spotLocation")
	namePtr := uintptr(unsafe.Pointer(&name[0]))
	pointPtr := uintptr(unsafe.Pointer(&point))
	unused := int32(0)
	var nested uintptr
	_, _ = ffi.CallFunction(x.cifCreateNestedSpot, x.fnCreateNested, unsafe.Pointer(&nested), []unsafe.Pointer{
		unsafe.Pointer(&unused), unsafe.Pointer(&namePtr), unsafe.Pointer(&pointPtr), unsafe.Pointer(new(uintptr)),
	})
	if nested == 0 {
		return
	}
	attr := cString("preeditAttributes")
	attrPtr := uintptr(unsafe.Pointer(&attr[0]))
	icPtr := ic
	nestedPtr := nested
	var result uintptr
	_, _ = ffi.CallFunction(x.cifSetICValues, x.fnSetICValues, unsafe.Pointer(&result), []unsafe.Pointer{
		unsafe.Pointer(&icPtr), unsafe.Pointer(&attrPtr), unsafe.Pointer(&nestedPtr), unsafe.Pointer(new(uintptr)),
	})
	x.free(nested)
	runtime.KeepAlive([]any{name, attr, point})
}

func nativeKeyEvent(keyEvent *KeyEvent, display uintptr) [xNativeEventBytes]byte {
	return nativeKeyEventType(keyEvent, display, true)
}

func nativeKeyEventType(keyEvent *KeyEvent, display uintptr, pressed bool) [xNativeEventBytes]byte {
	var raw [xNativeEventBytes]byte
	put32 := func(offset int, value uint32) {
		binary.NativeEndian.PutUint32(raw[offset:offset+4], value)
	}
	put64 := func(offset int, value uint64) {
		binary.NativeEndian.PutUint64(raw[offset:offset+8], value)
	}
	eventType := uint32(xKeyReleaseEvent)
	if pressed {
		eventType = xKeyPressEvent
	}
	put32(0, eventType)
	if strconv.IntSize == 32 {
		put32(4, uint32(keyEvent.Sequence))
		put32(8, 0) // send_event = False
		put32(12, uint32(display))
		put32(16, uint32(keyEvent.Event))
		put32(20, uint32(keyEvent.Root))
		put32(24, uint32(keyEvent.Child))
		put32(28, uint32(keyEvent.Time))
		put32(32, uint32(int32(keyEvent.EventX)))
		put32(36, uint32(int32(keyEvent.EventY)))
		put32(40, uint32(int32(keyEvent.RootX)))
		put32(44, uint32(int32(keyEvent.RootY)))
		put32(48, uint32(keyEvent.State))
		put32(52, uint32(keyEvent.Detail))
		put32(56, boolToUint32(keyEvent.SameScreen))
		return raw
	}
	put64(8, uint64(keyEvent.Sequence))
	put32(16, 0) // send_event = False
	put64(24, uint64(display))
	put64(32, uint64(keyEvent.Event))
	put64(40, uint64(keyEvent.Root))
	put64(48, uint64(keyEvent.Child))
	put64(56, uint64(keyEvent.Time))
	put32(64, uint32(int32(keyEvent.EventX)))
	put32(68, uint32(int32(keyEvent.EventY)))
	put32(72, uint32(int32(keyEvent.RootX)))
	put32(76, uint32(int32(keyEvent.RootY)))
	put32(80, uint32(keyEvent.State))
	put32(84, uint32(keyEvent.Detail))
	put32(88, boolToUint32(keyEvent.SameScreen))
	return raw
}

func boolToUint32(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

func (i *x11IME) capabilities() gpucontext.IMECapabilities {
	i.mu.Lock()
	defer i.mu.Unlock()
	features := gpucontext.IMECapabilityCommit | gpucontext.IMECapabilityCancel | gpucontext.IMECapabilityDisabled
	if i.preeditSupported {
		features |= gpucontext.IMECapabilityComposition
		if i.cursorSupported {
			features |= gpucontext.IMECapabilityCursorArea
		}
	}
	// XIM has no portable surrounding-text or delete-surrounding API. Content
	// purpose/hints are retained as advisory metadata but cannot be expressed
	// through Xlib, so those bits are intentionally not advertised.
	return gpucontext.IMECapabilities{Version: gpucontext.IMEContractVersion, Features: features}
}

func (i *x11IME) close() {
	if i == nil {
		return
	}
	i.mu.Lock()
	ic := i.ic
	i.ic = 0
	i.enabled = false
	i.composing = false
	i.preedit = ""
	i.preeditDone = false
	i.mu.Unlock()
	unregisterXIM(i)
	if i.xlib != nil {
		i.xlib.destroyIC(ic)
		i.xlib.closeIM()
	}
}

func (i *x11IME) setFocus(focused bool) {
	i.mu.Lock()
	i.focused = focused
	enabled := i.enabled
	ic := i.ic
	area, areaSet := i.cursorArea, i.cursorAreaSet
	scale := i.scale
	preeditSupported, cursorSupported := i.preeditSupported, i.cursorSupported
	canceled := !focused && i.composing
	if canceled {
		i.composing = false
		i.preedit = ""
		i.preeditDone = false
	}
	i.mu.Unlock()
	if ic != 0 {
		i.xlib.setFocus(ic, focused && enabled)
		if focused && enabled && areaSet && preeditSupported && cursorSupported {
			i.xlib.setSpot(ic, area, scale)
		}
	}
	if canceled {
		i.xlib.reset(ic)
		i.queue(gpucontextEventCanceled)
	}
}

const (
	gpucontextEventCanceled = EventTypeIMECanceled
	gpucontextEventDisabled = EventTypeIMEDisabled
)

func (i *x11IME) setEnabled(enabled bool) {
	i.mu.Lock()
	wasEnabled := i.enabled
	i.enabled = enabled
	ic := i.ic
	canceled := !enabled && i.composing
	if !enabled {
		i.surrounding = gpucontext.IMESurroundingText{}
		i.composing = false
		i.preedit = ""
		i.preeditDone = false
	}
	focused := i.focused
	area, areaSet := i.cursorArea, i.cursorAreaSet
	preeditSupported, cursorSupported := i.preeditSupported, i.cursorSupported
	i.mu.Unlock()

	if ic != 0 {
		i.xlib.setFocus(ic, enabled && focused)
	}
	if enabled && areaSet && preeditSupported && cursorSupported {
		i.mu.Lock()
		scale := i.scale
		i.mu.Unlock()
		i.xlib.setSpot(ic, area, scale)
	}
	if canceled {
		i.xlib.reset(ic)
		i.queue(gpucontextEventCanceled)
	}
	if !enabled && wasEnabled {
		i.queue(gpucontextEventDisabled)
	}
}

func (i *x11IME) setCursorArea(area gpucontext.IMECursorArea) {
	if area.X < 0 || area.Y < 0 || area.Width < 0 || area.Height < 0 ||
		math.IsNaN(area.X) || math.IsNaN(area.Y) || math.IsNaN(area.Width) || math.IsNaN(area.Height) ||
		math.IsInf(area.X, 0) || math.IsInf(area.Y, 0) || math.IsInf(area.Width, 0) || math.IsInf(area.Height, 0) {
		return
	}
	i.mu.Lock()
	i.cursorArea, i.cursorAreaSet = area, true
	enabled, focused, ic := i.enabled, i.focused, i.ic
	preeditSupported, cursorSupported := i.preeditSupported, i.cursorSupported
	i.mu.Unlock()
	if enabled && focused && preeditSupported && cursorSupported {
		i.mu.Lock()
		scale := i.scale
		i.mu.Unlock()
		i.xlib.setSpot(ic, area, scale)
	}
}

// setLegacyPosition preserves gpucontext.IMEController's historical pixel
// coordinates while the v2 IMECursorArea API uses logical DIP. Store the
// equivalent DIP point so the common spot update path converts it back to the
// physical X11 client coordinate exactly once.
func (i *x11IME) setLegacyPosition(x, y int) {
	if x < 0 || y < 0 {
		return
	}
	i.mu.Lock()
	scale := i.scale
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	area := gpucontext.IMECursorArea{X: float64(x) / scale, Y: float64(y) / scale}
	i.cursorArea, i.cursorAreaSet = area, true
	enabled, focused, ic := i.enabled, i.focused, i.ic
	preeditSupported, cursorSupported := i.preeditSupported, i.cursorSupported
	i.mu.Unlock()
	if enabled && focused && preeditSupported && cursorSupported {
		i.xlib.setSpot(ic, area, scale)
	}
}

func (i *x11IME) setContentType(purpose gpucontext.ContentPurpose, hints gpucontext.ContentHint) {
	i.mu.Lock()
	i.purpose, i.hints = purpose, hints
	i.mu.Unlock()
}

func (i *x11IME) setSurroundingText(text gpucontext.IMESurroundingText) {
	if !text.IsValid() {
		return
	}
	i.mu.Lock()
	if i.enabled {
		i.surrounding = text
	}
	i.mu.Unlock()
}

func (i *x11IME) cancel() {
	i.mu.Lock()
	if !i.composing {
		i.mu.Unlock()
		return
	}
	i.composing = false
	i.preedit = ""
	i.preeditDone = false
	ic := i.ic
	i.mu.Unlock()
	i.xlib.reset(ic)
	i.queue(gpucontextEventCanceled)
}

func (i *x11IME) handleKey(keyEvent *KeyEvent, pressed bool) bool {
	if i == nil || keyEvent == nil {
		return false
	}
	i.mu.Lock()
	enabled, ic := i.enabled, i.ic
	i.mu.Unlock()
	if !enabled || ic == 0 {
		return false
	}
	raw := nativeKeyEventType(keyEvent, i.xlib.display, pressed)
	// XFilterEvent updates the XIC's compose state and may synchronously invoke
	// preedit callbacks. Its boolean result is intentionally not used: a
	// no-preedit XIC still returns ordinary committed UTF-8 text below.
	i.xlib.filterEvent(ic, &raw, i.window)
	if !pressed {
		// XIM's back-end protocol forwards both KeyPress and KeyRelease events;
		// only KeyPress carries lookup text into the application.
		return true
	}
	text := i.xlib.lookupString(ic, &raw)
	i.mu.Lock()
	composing, preeditDone := i.composing, i.preeditDone
	i.preeditDone = false
	if text != "" {
		if composing || preeditDone {
			i.composing = false
			i.preedit = ""
			i.queueLocked(PlatformEvent{Type: EventTypeIMECompositionEnd, IMECommitted: text})
			i.mu.Unlock()
			return true
		}
		for _, r := range text {
			if r >= 32 {
				i.queueLocked(PlatformEvent{Type: EventTypeChar, Char: r})
			}
		}
		i.mu.Unlock()
		return true
	}
	if preeditDone {
		i.composing = false
		i.preedit = ""
		i.mu.Unlock()
		i.queue(gpucontextEventCanceled)
		return true
	}
	i.mu.Unlock()
	return true
}

func (i *x11IME) preeditStart() int32 {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.enabled || !i.focused {
		return -1
	}
	if !i.composing {
		i.composing = true
		i.preedit = ""
		i.preeditDone = false
		i.queueLocked(PlatformEvent{Type: EventTypeIMECompositionStart})
	}
	return -1
}

func (i *x11IME) preeditDoneCallback() {
	i.mu.Lock()
	if i.enabled && i.focused && i.composing {
		i.preeditDone = true
	}
	i.mu.Unlock()
}

func (i *x11IME) preeditCaret() {}

func (i *x11IME) preeditDraw(callData uintptr) {
	if callData == 0 {
		return
	}
	//nolint:govet // XIM callback data is an ABI-owned struct pointer.
	draw := (*ximPreeditDrawData)(unsafe.Pointer(callData))
	text := ximTextString(draw.Text)
	i.mu.Lock()
	if !i.enabled || !i.focused {
		i.mu.Unlock()
		return
	}
	if !i.composing {
		i.composing = true
		i.preedit = ""
		i.queueLocked(PlatformEvent{Type: EventTypeIMECompositionStart})
	}
	updated, startByte, endByte := replaceXIMPreedit(i.preedit, int(draw.ChangeFirst), int(draw.ChangeLength), text)
	i.preedit = updated
	caret := int(draw.Caret)
	if caret < 0 {
		caret = 0
	}
	if caret > utf8.RuneCountInString(updated) {
		caret = utf8.RuneCountInString(updated)
	}
	caretByte := runeByteOffset(updated, caret)
	composition := gpucontext.IMEComposition{
		CompositionText: updated,
		CursorBegin:     caretByte,
		CursorEnd:       caretByte,
		SelectionStart:  startByte,
		SelectionEnd:    endByte,
	}
	if !composition.IsValid() {
		i.mu.Unlock()
		return
	}
	i.queueLocked(PlatformEvent{Type: EventTypeIMECompositionUpdate, IMEComposition: composition})
	i.mu.Unlock()
}

func (i *x11IME) queue(eventType EventType) {
	if i == nil {
		return
	}
	i.queueLocked(PlatformEvent{Type: eventType})
}

// queueLocked is intentionally a method on x11IME; the owner window is found
// through the callback registry without storing a Go pointer in native memory.
//
//nolint:gocritic // PlatformEvent is queued by value to transfer ownership into the ring.
func (i *x11IME) queueLocked(event PlatformEvent) {
	if i.queueFn != nil {
		i.queueFn(event)
	}
}

func ximTextString(pointer uintptr) string {
	if pointer == 0 {
		return ""
	}
	//nolint:govet // XIM callback data is an ABI-owned struct pointer.
	text := (*ximText)(unsafe.Pointer(pointer))
	if text.String == 0 || text.Length == 0 || text.EncodingWide != 0 {
		return ""
	}
	// XIMText.Length counts characters while the multi-byte arm is NUL
	// terminated. Bound the read to avoid trusting a malformed provider.
	maxBytes := int(text.Length) * 4
	if maxBytes < 1 {
		return ""
	}
	if maxBytes >= 4096 {
		maxBytes = 4095
	}
	//nolint:govet // XIM callback data points to an ABI-owned NUL-terminated buffer.
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(text.String)), maxBytes+1)
	limit := 0
	for limit < len(bytes) && bytes[limit] != 0 {
		limit++
	}
	value := string(bytes[:limit])
	if !utf8.ValidString(value) {
		return strings.ToValidUTF8(value, "")
	}
	return value
}

func runeByteOffset(text string, runeIndex int) int {
	if runeIndex <= 0 {
		return 0
	}
	if runeIndex >= utf8.RuneCountInString(text) {
		return len(text)
	}
	count := 0
	for offset := range text {
		if count == runeIndex {
			return offset
		}
		count++
	}
	return len(text)
}

func replaceXIMPreedit(current string, first, length int, replacement string) (updated string, startByte, endByte int) {
	runes := []rune(current)
	if first < 0 {
		first = 0
	}
	if first > len(runes) {
		first = len(runes)
	}
	if length < 0 {
		length = 0
	}
	end := first + length
	if end > len(runes) {
		end = len(runes)
	}
	next := make([]rune, 0, len(runes)-(end-first)+utf8.RuneCountInString(replacement))
	next = append(next, runes[:first]...)
	next = append(next, []rune(replacement)...)
	next = append(next, runes[end:]...)
	updated = string(next)
	startByte = runeByteOffset(updated, first)
	endByte = runeByteOffset(updated, first+utf8.RuneCountInString(replacement))
	return updated, startByte, endByte
}
