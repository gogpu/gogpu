//go:build windows

package platform

// drag_windows.go — Outgoing drag-and-drop (drag source) via COM OLE2 DoDragDrop.

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COM GUIDs for QueryInterface validation.
// DoDragDrop queries for optional interfaces (IAsyncOperation, etc.) — we must
// return E_NOINTERFACE for anything we don't implement, otherwise COM calls
// methods at vtable offsets we haven't populated.
type comGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	iidIUnknown    = comGUID{0x00000000, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDataObject = comGUID{0x0000010E, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDropSource = comGUID{0x00000121, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

func guidEqual(a, b *comGUID) bool {
	return a.Data1 == b.Data1 && a.Data2 == b.Data2 && a.Data3 == b.Data3 && a.Data4 == b.Data4
}

var (
	procOleInitialize = ole32.NewProc("OleInitialize")
	procDoDragDrop    = ole32.NewProc("DoDragDrop")
	procGlobalSize    = kernel32.NewProc("GlobalSize")
)

// COM/OLE constants for drag-and-drop.
const (
	gmemFixed    = 0x0000
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
// MUST be called from the HWND owner thread (main thread) — DoDragDrop requires the
// message pump that owns the window. Do NOT call LockOSThread here; the goroutine is
// already pinned to the main OS thread by gogpu.App.Run().
func startDragWindows(paths []string, done func(DragResult)) {
	hGlobal, err := buildDropFilesHGlobal(paths)
	if err != nil {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	dataObj := newSimpleDataObject(hGlobal)
	dropSrc := newSimpleDropSource()

	var dwEffect uint32
	hr, _, _ := procDoDragDrop.Call(
		dataObj.comPtr(),
		dropSrc.comPtr(),
		uintptr(dropEffectCopy|dropEffectMove),
		uintptr(unsafe.Pointer(&dwEffect)),
	)
	runtime.KeepAlive(dataObj)
	runtime.KeepAlive(dropSrc)

	var result DragResult
	switch hr {
	case dragdropSOK, 0x00040100: // S_OK or DRAGDROP_S_DROP
		switch dwEffect {
		case dropEffectCopy:
			result = DragCopied
		case dropEffectMove:
			result = DragMoved
		default:
			result = DragCancelled
		}
	default:
		result = DragCancelled
	}

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
// vtblPtr (first field) points to a GlobalAlloc'd vtable — COM reads *this to get it.
type simpleDataObject struct {
	vtblPtr  uintptr
	refCount int32
	hGlobal  uintptr
}

func newSimpleDataObject(hGlobal uintptr) *simpleDataObject {
	// Allocate vtable in non-GC memory so COM can safely dereference it.
	vtblMem, _, _ := procGlobalAlloc.Call(uintptr(gmemFixed), 12*unsafe.Sizeof(uintptr(0)))
	if vtblMem == 0 {
		return nil
	}
	vtbl := unsafe.Slice((*uintptr)(unsafe.Pointer(vtblMem)), 12) //nolint:govet // GlobalAlloc returns non-GC memory
	// IUnknown
	vtbl[0] = syscall.NewCallback(dataObjQueryInterface)
	vtbl[1] = syscall.NewCallback(dataObjAddRef)
	vtbl[2] = syscall.NewCallback(dataObjRelease)
	// IDataObject
	vtbl[3] = syscall.NewCallback(dataObjGetData)
	vtbl[4] = syscall.NewCallback(dataObjGetDataHere)
	vtbl[5] = syscall.NewCallback(dataObjQueryGetData)
	vtbl[6] = syscall.NewCallback(dataObjGetCanonicalFormatEtc)
	vtbl[7] = syscall.NewCallback(dataObjSetData)
	vtbl[8] = syscall.NewCallback(dataObjEnumFormatEtc)
	vtbl[9] = syscall.NewCallback(dataObjDAdvise)
	vtbl[10] = syscall.NewCallback(dataObjDUnadvise)
	vtbl[11] = syscall.NewCallback(dataObjEnumDAdvise)

	obj := &simpleDataObject{
		vtblPtr:  vtblMem,
		refCount: 1,
		hGlobal:  hGlobal,
	}
	// Prevent GC from collecting the object while COM holds a reference.
	runtime.SetFinalizer(obj, nil)
	return obj
}

func (o *simpleDataObject) comPtr() uintptr {
	return uintptr(unsafe.Pointer(o))
}

// IUnknown::QueryInterface — only accept IUnknown and IDataObject.
func dataObjQueryInterface(this, riid, ppvObject uintptr) uintptr {
	if ppvObject == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	guid := (*comGUID)(unsafe.Pointer(riid)) //nolint:govet // COM REFIID pointer cast
	if guidEqual(guid, &iidIUnknown) || guidEqual(guid, &iidIDataObject) {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this //nolint:govet // COM out-pointer write
		dataObjAddRef(this)
		return 0 // S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0 //nolint:govet // COM out-pointer null on failure
	return 0x80004002                          // E_NOINTERFACE
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

// IDataObject::EnumFormatEtc — returns an enumerator for our single CF_HDROP format.
// Explorer calls this to discover available clipboard formats during drag-over.
func dataObjEnumFormatEtc(this, dwDirection, ppEnumFormatEtc uintptr) uintptr {
	if ppEnumFormatEtc == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	// DATADIR_GET = 1, DATADIR_SET = 2
	if dwDirection != 1 {
		return 0x80004001 // E_NOTIMPL (we only support DATADIR_GET)
	}
	enum := newFormatEnumerator()
	if enum == nil {
		return 0x8007000E // E_OUTOFMEMORY
	}
	*(*uintptr)(unsafe.Pointer(ppEnumFormatEtc)) = enum.comPtr() //nolint:govet // COM out-pointer write
	return 0                                                     // S_OK
}

// --- IEnumFORMATETC ---

var iidIEnumFORMATETC = comGUID{0x00000103, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

// formatEnumerator implements IEnumFORMATETC with a single CF_HDROP entry.
type formatEnumerator struct {
	vtblPtr  uintptr
	refCount int32
	index    int32 // current enumeration position (0 or 1)
}

func newFormatEnumerator() *formatEnumerator {
	vtblMem, _, _ := procGlobalAlloc.Call(uintptr(gmemFixed), 7*unsafe.Sizeof(uintptr(0)))
	if vtblMem == 0 {
		return nil
	}
	vtbl := unsafe.Slice((*uintptr)(unsafe.Pointer(vtblMem)), 7) //nolint:govet // GlobalAlloc returns non-GC memory
	// IUnknown
	vtbl[0] = syscall.NewCallback(enumFmtQueryInterface)
	vtbl[1] = syscall.NewCallback(enumFmtAddRef)
	vtbl[2] = syscall.NewCallback(enumFmtRelease)
	// IEnumFORMATETC
	vtbl[3] = syscall.NewCallback(enumFmtNext)
	vtbl[4] = syscall.NewCallback(enumFmtSkip)
	vtbl[5] = syscall.NewCallback(enumFmtReset)
	vtbl[6] = syscall.NewCallback(enumFmtClone)

	obj := &formatEnumerator{
		vtblPtr:  vtblMem,
		refCount: 1,
	}
	runtime.SetFinalizer(obj, nil)
	return obj
}

func (o *formatEnumerator) comPtr() uintptr {
	return uintptr(unsafe.Pointer(o))
}

func enumFmtQueryInterface(this, riid, ppvObject uintptr) uintptr {
	if ppvObject == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	guid := (*comGUID)(unsafe.Pointer(riid)) //nolint:govet // COM REFIID pointer cast
	if guidEqual(guid, &iidIUnknown) || guidEqual(guid, &iidIEnumFORMATETC) {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this //nolint:govet // COM out-pointer write
		enumFmtAddRef(this)
		return 0 // S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0 //nolint:govet // COM out-pointer null on failure
	return 0x80004002                          // E_NOINTERFACE
}

func enumFmtAddRef(this uintptr) uintptr {
	obj := (*formatEnumerator)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.refCount++
	return uintptr(obj.refCount)
}

func enumFmtRelease(this uintptr) uintptr {
	obj := (*formatEnumerator)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.refCount--
	return uintptr(obj.refCount)
}

// IEnumFORMATETC::Next — returns the next N FORMATETC entries.
func enumFmtNext(this, celt, rgelt, pceltFetched uintptr) uintptr {
	if rgelt == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	obj := (*formatEnumerator)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast

	fetched := uint32(0)
	for i := uintptr(0); i < celt; i++ {
		if obj.index >= 1 { // we have exactly one format
			break
		}
		dst := (*formatETC)(unsafe.Pointer(rgelt + i*unsafe.Sizeof(formatETC{}))) //nolint:govet // COM array element
		dst.cfFormat = cfHDROP
		dst.ptd = 0
		dst.dwAspect = dvaspectContent
		dst.lindex = -1
		dst.tymed = tymedHGlobal
		obj.index++
		fetched++
	}
	if pceltFetched != 0 {
		*(*uint32)(unsafe.Pointer(pceltFetched)) = fetched //nolint:govet // COM out-pointer
	}
	if fetched < uint32(celt) {
		return 1 // S_FALSE — fewer items returned than requested
	}
	return 0 // S_OK
}

// IEnumFORMATETC::Skip
func enumFmtSkip(this, celt uintptr) uintptr {
	obj := (*formatEnumerator)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.index += int32(celt)
	if obj.index > 1 {
		obj.index = 1
		return 1 // S_FALSE
	}
	return 0 // S_OK
}

// IEnumFORMATETC::Reset
func enumFmtReset(this uintptr) uintptr {
	obj := (*formatEnumerator)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	obj.index = 0
	return 0 // S_OK
}

// IEnumFORMATETC::Clone
func enumFmtClone(this, ppEnum uintptr) uintptr {
	if ppEnum == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	obj := (*formatEnumerator)(unsafe.Pointer(this)) //nolint:govet // COM this pointer cast
	clone := newFormatEnumerator()
	if clone == nil {
		return 0x8007000E // E_OUTOFMEMORY
	}
	clone.index = obj.index
	*(*uintptr)(unsafe.Pointer(ppEnum)) = clone.comPtr() //nolint:govet // COM out-pointer write
	return 0                                             // S_OK
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
}

func newSimpleDropSource() *simpleDropSource {
	vtblMem, _, _ := procGlobalAlloc.Call(uintptr(gmemFixed), 5*unsafe.Sizeof(uintptr(0)))
	if vtblMem == 0 {
		return nil
	}
	vtbl := unsafe.Slice((*uintptr)(unsafe.Pointer(vtblMem)), 5) //nolint:govet // GlobalAlloc returns non-GC memory
	vtbl[0] = syscall.NewCallback(dropSrcQueryInterface)
	vtbl[1] = syscall.NewCallback(dropSrcAddRef)
	vtbl[2] = syscall.NewCallback(dropSrcRelease)
	vtbl[3] = syscall.NewCallback(dropSrcQueryContinueDrag)
	vtbl[4] = syscall.NewCallback(dropSrcGiveFeedback)

	obj := &simpleDropSource{
		vtblPtr:  vtblMem,
		refCount: 1,
	}
	runtime.SetFinalizer(obj, nil)
	return obj
}

func (o *simpleDropSource) comPtr() uintptr {
	return uintptr(unsafe.Pointer(o))
}

// IUnknown methods for IDropSource — only accept IUnknown and IDropSource.
func dropSrcQueryInterface(this, riid, ppvObject uintptr) uintptr {
	if ppvObject == 0 {
		return 0x80070057 // E_INVALIDARG
	}
	guid := (*comGUID)(unsafe.Pointer(riid)) //nolint:govet // COM REFIID pointer cast
	if guidEqual(guid, &iidIUnknown) || guidEqual(guid, &iidIDropSource) {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this //nolint:govet // COM out-pointer write
		dropSrcAddRef(this)
		return 0 // S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0 //nolint:govet // COM out-pointer null on failure
	return 0x80004002                          // E_NOINTERFACE
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
		return 0x00040100 // DRAGDROP_S_DROP
	}
	return 0 // S_OK — continue
}

// IDropSource::GiveFeedback — use system default drag cursors.
func dropSrcGiveFeedback(this, dwEffect uintptr) uintptr {
	return 0x00040102 // DRAGDROP_S_USEDEFAULTCURSORS
}
