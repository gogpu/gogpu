//go:build windows

package platform

// drag_windows.go — Outgoing drag-and-drop (drag source) via COM OLE2 DoDragDrop.
//
// Implementation uses the same COM vtable pattern as dialog_windows.go.
// We build minimal IDataObject and IDropSource COM objects inline using
// Go-allocated vtable arrays. CF_HDROP (DROPFILES) is used for file paths.
//
// Reference: Microsoft Win32 DoDragDrop documentation, SDL3 SDL_sysdnd.c.

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procDoDragDrop       = ole32.NewProc("DoDragDrop")
	procReleaseStgMedium = ole32.NewProc("ReleaseStgMedium") // reserved for STGMEDIUM cleanup
	procGlobalSize       = kernel32.NewProc("GlobalSize")
)

// COM/OLE constants for drag-and-drop.
const (
	gmemZeroinit = 0x0040
	gmemGHND     = gmemMoveable | gmemZeroinit

	cfHDROP uint16 = 15

	dropEffectNone = 0
	dropEffectCopy = 1
	dropEffectMove = 2

	tymedHGlobal    = 1
	dvaspectContent = 1

	dragdropSOK     uintptr = 0
	dragdropSCancel uintptr = 0x00040101
)

// FORMATETC mirrors the COM FORMATETC structure.
type formatETC struct {
	cfFormat uint16
	ptd      uintptr
	dwAspect uint32
	lindex   int32
	tymed    uint32
}

// STGMEDIUM mirrors the COM STGMEDIUM structure.
type stgMedium struct {
	tymed          uint32
	_pad           [4]byte // alignment on 64-bit
	hGlobal        uintptr
	pUnkForRelease uintptr
}

// DROPFILES mirrors the Win32 DROPFILES structure.
type dropFiles struct {
	pFiles uint32
	ptX    int32
	ptY    int32
	fNC    int32
	fWide  int32
}

// startDragWindows initiates a COM OLE2 drag-and-drop operation with file paths.
// This blocks the calling thread until the drag completes (modal loop inside DoDragDrop).
func startDragWindows(paths []string, done func(DragResult)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Initialize COM on this thread (may already be initialized).
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hr == comSOK || hr == comSFalse {
		defer procCoUninitialize.Call()
	}

	hGlobal, err := buildDropFilesHGlobal(paths)
	if err != nil {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	// Build inline IDataObject and IDropSource COM objects.
	dataObj := newSimpleDataObject(hGlobal)
	dropSrc := newSimpleDropSource()

	var dwEffect uint32
	hr, _, _ = procDoDragDrop.Call(
		dataObj.comPtr(),
		dropSrc.comPtr(),
		uintptr(dropEffectCopy|dropEffectMove),
		uintptr(unsafe.Pointer(&dwEffect)),
	)

	var result DragResult
	if hr == dragdropSOK {
		switch dwEffect {
		case dropEffectCopy:
			result = DragCopied
		case dropEffectMove:
			result = DragMoved
		default:
			result = DragCancelled
		}
	} else {
		// DRAGDROP_S_CANCEL or error
		result = DragCancelled
	}

	// Clean up the HGLOBAL (DoDragDrop may or may not have freed it depending on dwEffect).
	// ReleaseStgMedium is the proper way; we handle it in IDataObject release.

	if done != nil {
		done(result)
	}
}

// buildDropFilesHGlobal builds an HGLOBAL containing a DROPFILES structure
// followed by the file paths as a double-null-terminated wide string list.
func buildDropFilesHGlobal(paths []string) (uintptr, error) {
	// Convert paths to UTF-16 and calculate total size.
	var utf16Paths [][]uint16
	totalChars := 0
	for _, p := range paths {
		u16, err := windows.UTF16FromString(p)
		if err != nil {
			return 0, fmt.Errorf("drag: invalid path %q: %w", p, err)
		}
		utf16Paths = append(utf16Paths, u16)
		totalChars += len(u16) // includes null terminator
	}
	totalChars++ // double-null terminator

	headerSize := uint32(unsafe.Sizeof(dropFiles{}))
	totalSize := uintptr(headerSize) + uintptr(totalChars*2) // 2 bytes per UTF-16 char

	hGlobal, _, _ := procGlobalAlloc.Call(uintptr(gmemGHND), totalSize)
	if hGlobal == 0 {
		return 0, fmt.Errorf("drag: GlobalAlloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(hGlobal)
	if ptr == 0 {
		procGlobalFree.Call(hGlobal)
		return 0, fmt.Errorf("drag: GlobalLock failed")
	}

	// Write DROPFILES header.
	df := (*dropFiles)(unsafe.Pointer(ptr)) //nolint:govet // GlobalLock returns locked memory pointer
	df.pFiles = headerSize
	df.fWide = 1 // Unicode

	// Write file paths after the header.
	offset := ptr + uintptr(headerSize)
	for _, u16 := range utf16Paths {
		for _, ch := range u16 {
			binary.LittleEndian.PutUint16(unsafe.Slice((*byte)(unsafe.Pointer(offset)), 2), ch) //nolint:govet // HGLOBAL offset arithmetic
			offset += 2
		}
	}
	// Double-null terminator (GMEM_ZEROINIT already zeroed, but be explicit).
	binary.LittleEndian.PutUint16(unsafe.Slice((*byte)(unsafe.Pointer(offset)), 2), 0) //nolint:govet // HGLOBAL offset arithmetic

	procGlobalUnlock.Call(hGlobal)
	return hGlobal, nil
}

// --- Minimal inline IDataObject ---

// simpleDataObject implements IDataObject with a single CF_HDROP format.
// The COM vtable is built as a Go array of function pointers.
type simpleDataObject struct {
	vtblPtr  uintptr // points to vtbl[0]
	refCount int32
	hGlobal  uintptr
	vtbl     [12]uintptr // IUnknown(3) + IDataObject(9)
}

func newSimpleDataObject(hGlobal uintptr) *simpleDataObject {
	obj := &simpleDataObject{
		refCount: 1,
		hGlobal:  hGlobal,
	}
	// IUnknown
	obj.vtbl[0] = syscall.NewCallback(dataObjQueryInterface)
	obj.vtbl[1] = syscall.NewCallback(dataObjAddRef)
	obj.vtbl[2] = syscall.NewCallback(dataObjRelease)
	// IDataObject
	obj.vtbl[3] = syscall.NewCallback(dataObjGetData)
	obj.vtbl[4] = syscall.NewCallback(dataObjGetDataHere)
	obj.vtbl[5] = syscall.NewCallback(dataObjQueryGetData)
	obj.vtbl[6] = syscall.NewCallback(dataObjGetCanonicalFormatEtc)
	obj.vtbl[7] = syscall.NewCallback(dataObjSetData)
	obj.vtbl[8] = syscall.NewCallback(dataObjEnumFormatEtc)
	obj.vtbl[9] = syscall.NewCallback(dataObjDAdvise)
	obj.vtbl[10] = syscall.NewCallback(dataObjDUnadvise)
	obj.vtbl[11] = syscall.NewCallback(dataObjEnumDAdvise)
	obj.vtblPtr = uintptr(unsafe.Pointer(&obj.vtbl[0]))
	// Pin the object to prevent GC.
	runtime.SetFinalizer(obj, nil)
	return obj
}

func (o *simpleDataObject) comPtr() uintptr {
	return uintptr(unsafe.Pointer(o))
}

// IUnknown::QueryInterface
func dataObjQueryInterface(this, riid, ppvObject uintptr) uintptr {
	// Accept IUnknown and IDataObject.
	if ppvObject == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	// For simplicity, accept any QI and return ourselves (DoDragDrop only needs IDataObject).
	*(*uintptr)(unsafe.Pointer(ppvObject)) = this //nolint:govet // COM out-pointer write
	dataObjAddRef(this)
	return 0 // S_OK
}

// IUnknown::AddRef
func dataObjAddRef(this uintptr) uintptr {
	obj := (*simpleDataObject)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.refCount++
	return uintptr(obj.refCount)
}

// IUnknown::Release
func dataObjRelease(this uintptr) uintptr {
	obj := (*simpleDataObject)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.refCount--
	if obj.refCount <= 0 {
		if obj.hGlobal != 0 {
			procGlobalFree.Call(obj.hGlobal)
			obj.hGlobal = 0
		}
	}
	return uintptr(obj.refCount)
}

// IDataObject::GetData
func dataObjGetData(this, pFormatEtc, pMedium uintptr) uintptr {
	if pFormatEtc == 0 || pMedium == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	obj := (*simpleDataObject)(unsafe.Pointer(this))   //nolint:govet // COM this pointer cast
	fmtEtc := (*formatETC)(unsafe.Pointer(pFormatEtc)) //nolint:govet // COM struct pointer cast

	if fmtEtc.cfFormat != cfHDROP || fmtEtc.tymed&tymedHGlobal == 0 {
		return 0x80040064 // DV_E_FORMATETC
	}

	if obj.hGlobal == 0 {
		return 0x80004005 // E_FAIL
	}

	// Duplicate the HGLOBAL for the caller.
	srcSize, _, _ := procGlobalSize.Call(obj.hGlobal)
	if srcSize == 0 {
		return 0x80004005 // E_FAIL
	}

	dst, _, _ := procGlobalAlloc.Call(uintptr(gmemGHND), srcSize)
	if dst == 0 {
		return 0x8007000E // E_OUTOFMEMORY
	}

	srcPtr, _, _ := procGlobalLock.Call(obj.hGlobal)
	dstPtr, _, _ := procGlobalLock.Call(dst)
	if srcPtr != 0 && dstPtr != 0 {
		// Copy memory.
		copy(
			unsafe.Slice((*byte)(unsafe.Pointer(dstPtr)), srcSize), //nolint:govet // GlobalLock pointer
			unsafe.Slice((*byte)(unsafe.Pointer(srcPtr)), srcSize), //nolint:govet // GlobalLock pointer
		)
	}
	procGlobalUnlock.Call(obj.hGlobal)
	procGlobalUnlock.Call(dst)

	med := (*stgMedium)(unsafe.Pointer(pMedium)) //nolint:govet // COM out-pointer cast
	med.tymed = tymedHGlobal
	med.hGlobal = dst
	med.pUnkForRelease = 0

	return 0 // S_OK
}

// IDataObject::GetDataHere — not supported.
func dataObjGetDataHere(this, pFormatEtc, pMedium uintptr) uintptr {
	return 0x80004001 // E_NOTIMPL
}

// IDataObject::QueryGetData
func dataObjQueryGetData(this, pFormatEtc uintptr) uintptr {
	if pFormatEtc == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	fmtEtc := (*formatETC)(unsafe.Pointer(pFormatEtc)) //nolint:govet // COM struct pointer cast
	if fmtEtc.cfFormat == cfHDROP && fmtEtc.tymed&tymedHGlobal != 0 {
		return 0 // S_OK
	}
	return 0x80040064 // DV_E_FORMATETC
}

// IDataObject::GetCanonicalFormatEtc — not supported.
func dataObjGetCanonicalFormatEtc(this, pFormatIn, pFormatOut uintptr) uintptr {
	return 0x80004001 // E_NOTIMPL
}

// IDataObject::SetData — not supported.
func dataObjSetData(this, pFormatEtc, pMedium, fRelease uintptr) uintptr {
	return 0x80004001 // E_NOTIMPL
}

// IDataObject::EnumFormatEtc — not supported (DoDragDrop does not require it for simple drops).
func dataObjEnumFormatEtc(this, dwDirection, ppEnumFormatEtc uintptr) uintptr {
	return 0x80004001 // E_NOTIMPL
}

// IDataObject::DAdvise — not supported.
func dataObjDAdvise(this, pFormatEtc, advf, pAdviseSink, pdwConnection uintptr) uintptr {
	return 0x80040003 // OLE_E_ADVISENOTSUPPORTED
}

// IDataObject::DUnadvise — not supported.
func dataObjDUnadvise(this, dwConnection uintptr) uintptr {
	return 0x80040003 // OLE_E_ADVISENOTSUPPORTED
}

// IDataObject::EnumDAdvise — not supported.
func dataObjEnumDAdvise(this, ppEnumAdvise uintptr) uintptr {
	return 0x80040003 // OLE_E_ADVISENOTSUPPORTED
}

// --- Minimal inline IDropSource ---

// simpleDropSource implements IDropSource.
type simpleDropSource struct {
	vtblPtr  uintptr
	refCount int32
	vtbl     [5]uintptr // IUnknown(3) + IDropSource(2)
}

func newSimpleDropSource() *simpleDropSource {
	obj := &simpleDropSource{refCount: 1}
	obj.vtbl[0] = syscall.NewCallback(dropSrcQueryInterface)
	obj.vtbl[1] = syscall.NewCallback(dropSrcAddRef)
	obj.vtbl[2] = syscall.NewCallback(dropSrcRelease)
	obj.vtbl[3] = syscall.NewCallback(dropSrcQueryContinueDrag)
	obj.vtbl[4] = syscall.NewCallback(dropSrcGiveFeedback)
	obj.vtblPtr = uintptr(unsafe.Pointer(&obj.vtbl[0]))
	runtime.SetFinalizer(obj, nil)
	return obj
}

func (o *simpleDropSource) comPtr() uintptr {
	return uintptr(unsafe.Pointer(o))
}

// IUnknown methods for IDropSource
func dropSrcQueryInterface(this, riid, ppvObject uintptr) uintptr {
	if ppvObject == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = this //nolint:govet // COM out-pointer write
	dropSrcAddRef(this)
	return 0 // S_OK
}

func dropSrcAddRef(this uintptr) uintptr {
	obj := (*simpleDropSource)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.refCount++
	return uintptr(obj.refCount)
}

func dropSrcRelease(this uintptr) uintptr {
	obj := (*simpleDropSource)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.refCount--
	return uintptr(obj.refCount)
}

// IDropSource::QueryContinueDrag
// Called by DoDragDrop to determine whether to continue, cancel, or complete.
func dropSrcQueryContinueDrag(this, fEscapePressed, grfKeyState uintptr) uintptr {
	if fEscapePressed != 0 {
		return dragdropSCancel // DRAGDROP_S_CANCEL
	}
	// MK_LBUTTON = 0x0001
	if grfKeyState&0x0001 == 0 {
		// Left button released — drop.
		return 0x00040100 // DRAGDROP_S_DROP
	}
	return 0 // S_OK — continue
}

// IDropSource::GiveFeedback — use default cursors.
func dropSrcGiveFeedback(this, dwEffect uintptr) uintptr {
	return 0x00040002 // DRAGDROP_S_USEDEFAULTCURSORS
}
