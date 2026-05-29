//go:build windows

package platform

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COM constants
const (
	coinitApartmentThreaded uintptr = 0x2
	clsctxInprocServer      uintptr = 0x1
	comSOK                  uintptr = 0
	comSFalse               uintptr = 1
	hresultCancelled        uintptr = 0x800704C7

	// IFileDialog option flags
	fosPickFolders      uint32 = 0x00000020
	fosAllowMultiselect uint32 = 0x00000200
	fosForceFilesystem  uint32 = 0x00000040

	defaultFilterPattern = "*.*"

	// IFileDialog vtable indices (IUnknown=0-2, IModalWindow=3, IFileDialog=4-26)
	vtblDlgRelease      = 2
	vtblDlgShow         = 3
	vtblDlgSetFileTypes = 4
	vtblDlgSetOptions   = 9
	vtblDlgGetOptions   = 10
	vtblDlgSetFolder    = 12
	vtblDlgSetFileName  = 15
	vtblDlgSetTitle     = 17
	vtblDlgGetResult    = 20
	// IFileOpenDialog additional (index 27)
	vtblOpenDlgGetResults = 27
	// IShellItemArray vtable indices (IUnknown=0-2, then array methods)
	vtblSIAGetCount  = 7
	vtblSIAGetItemAt = 8
	// IShellItem vtable indices
	vtblSIGetDisplayName = 5

	// SIGDN_FILESYSPATH
	sigdnFileSysPath uintptr = 0x80058000
)

var (
	// CLSID_FileOpenDialog: {DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7}
	clsidFileOpenDialog = windows.GUID{
		Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE,
		Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7},
	}
	// CLSID_FileSaveDialog: {C0B4E2F3-BA21-4773-8DBA-335EC946EB8B}
	clsidFileSaveDialog = windows.GUID{
		Data1: 0xC0B4E2F3, Data2: 0xBA21, Data3: 0x4773,
		Data4: [8]byte{0x8D, 0xBA, 0x33, 0x5E, 0xC9, 0x46, 0xEB, 0x8B},
	}
	// IID_IFileOpenDialog: {D57C7288-D4AD-4768-BE02-9D969532D960}
	iidIFileOpenDialog = windows.GUID{
		Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768,
		Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60},
	}
	// IID_IFileSaveDialog: {84BCCD23-5FDE-4CDB-AEA4-AF64B83D78AB}
	iidIFileSaveDialog = windows.GUID{
		Data1: 0x84BCCD23, Data2: 0x5FDE, Data3: 0x4CDB,
		Data4: [8]byte{0xAE, 0xA4, 0xAF, 0x64, 0xB8, 0x3D, 0x78, 0xAB},
	}
	// IID_IShellItem: {43826D1E-E718-42EE-BC55-A1E261C37BFE}
	iidIShellItem = windows.GUID{
		Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE,
		Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE},
	}

	ole32                           = windows.NewLazyDLL("ole32.dll")
	shell32                         = windows.NewLazyDLL("shell32.dll")
	procCoInitializeEx              = ole32.NewProc("CoInitializeEx")
	procCoUninitialize              = ole32.NewProc("CoUninitialize")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree               = ole32.NewProc("CoTaskMemFree")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
)

// comdlgFilterSpec mirrors the Win32 COMDLG_FILTERSPEC struct.
type comdlgFilterSpec struct {
	pszName *uint16
	pszSpec *uint16
}

func showOpenFileDialog(hwnd uintptr, opts FileDialogOptions) ([]string, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hr == comSOK || hr == comSFalse {
		defer procCoUninitialize.Call()
	}

	var dialog uintptr
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if hr != comSOK {
		return nil, fmt.Errorf("file dialog: CoCreateInstance(FileOpenDialog) failed (0x%08X)", uint32(hr))
	}
	defer comRelease(dialog)

	setDlgCommonOptions(dialog, hwnd, opts, true)

	hr, _, _ = syscall.SyscallN(comVtbl(dialog)[vtblDlgShow], dialog, hwnd)
	if hr == hresultCancelled {
		return nil, nil
	}
	if hr != comSOK {
		return nil, fmt.Errorf("file dialog: Show failed (0x%08X)", uint32(hr))
	}

	var results uintptr
	hr, _, _ = syscall.SyscallN(comVtbl(dialog)[vtblOpenDlgGetResults], dialog, uintptr(unsafe.Pointer(&results)))
	if hr != comSOK {
		return nil, fmt.Errorf("file dialog: GetResults failed (0x%08X)", uint32(hr))
	}
	defer comRelease(results)

	var count uint32
	hr, _, _ = syscall.SyscallN(comVtbl(results)[vtblSIAGetCount], results, uintptr(unsafe.Pointer(&count)))
	if hr != comSOK {
		return nil, fmt.Errorf("file dialog: GetCount failed (0x%08X)", uint32(hr))
	}

	paths := make([]string, 0, int(count))
	for i := uint32(0); i < count; i++ {
		var item uintptr
		hr, _, _ = syscall.SyscallN(comVtbl(results)[vtblSIAGetItemAt], results, uintptr(i), uintptr(unsafe.Pointer(&item)))
		if hr != comSOK {
			continue
		}
		if p := shellItemPath(item); p != "" {
			paths = append(paths, p)
		}
		comRelease(item)
	}
	return paths, nil
}

func showSaveFileDialog(hwnd uintptr, opts FileDialogOptions) (string, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hr == comSOK || hr == comSFalse {
		defer procCoUninitialize.Call()
	}

	var dialog uintptr
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileSaveDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileSaveDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if hr != comSOK {
		return "", fmt.Errorf("file dialog: CoCreateInstance(FileSaveDialog) failed (0x%08X)", uint32(hr))
	}
	defer comRelease(dialog)

	setDlgCommonOptions(dialog, hwnd, opts, false)

	if opts.DefaultFilename != "" {
		nameW, err := windows.UTF16PtrFromString(opts.DefaultFilename)
		if err == nil {
			syscall.SyscallN(comVtbl(dialog)[vtblDlgSetFileName], dialog, uintptr(unsafe.Pointer(nameW)))
		}
	}

	hr, _, _ = syscall.SyscallN(comVtbl(dialog)[vtblDlgShow], dialog, hwnd)
	if hr == hresultCancelled {
		return "", nil
	}
	if hr != comSOK {
		return "", fmt.Errorf("file dialog: Show failed (0x%08X)", uint32(hr))
	}

	var item uintptr
	hr, _, _ = syscall.SyscallN(comVtbl(dialog)[vtblDlgGetResult], dialog, uintptr(unsafe.Pointer(&item)))
	if hr != comSOK {
		return "", fmt.Errorf("file dialog: GetResult failed (0x%08X)", uint32(hr))
	}
	defer comRelease(item)

	return shellItemPath(item), nil
}

// setDlgCommonOptions applies title, folder, option flags, and file type filters.
// isOpen must be true for IFileOpenDialog (Multiple selection is open-only).
func setDlgCommonOptions(dialog, hwnd uintptr, opts FileDialogOptions, isOpen bool) {
	_ = hwnd // reserved for future per-window modality

	if opts.Title != "" {
		titleW, err := windows.UTF16PtrFromString(opts.Title)
		if err == nil {
			syscall.SyscallN(comVtbl(dialog)[vtblDlgSetTitle], dialog, uintptr(unsafe.Pointer(titleW)))
		}
	}

	var flags uint32
	syscall.SyscallN(comVtbl(dialog)[vtblDlgGetOptions], dialog, uintptr(unsafe.Pointer(&flags)))
	flags |= fosForceFilesystem
	if opts.Directory {
		flags |= fosPickFolders
	}
	if isOpen && opts.Multiple {
		flags |= fosAllowMultiselect
	}
	syscall.SyscallN(comVtbl(dialog)[vtblDlgSetOptions], dialog, uintptr(flags))

	if opts.InitialDirectory != "" {
		pathW, err := windows.UTF16PtrFromString(opts.InitialDirectory)
		if err == nil {
			var si uintptr
			hr, _, _ := procSHCreateItemFromParsingName.Call(
				uintptr(unsafe.Pointer(pathW)),
				0,
				uintptr(unsafe.Pointer(&iidIShellItem)),
				uintptr(unsafe.Pointer(&si)),
			)
			if hr == comSOK && si != 0 {
				syscall.SyscallN(comVtbl(dialog)[vtblDlgSetFolder], dialog, si)
				comRelease(si)
			}
		}
	}

	if len(opts.Filters) > 0 {
		setDlgFileTypes(dialog, opts.Filters)
	}
}

// setDlgFileTypes sets the COMDLG_FILTERSPEC array on the dialog.
func setDlgFileTypes(dialog uintptr, filters []FileTypeFilter) {
	specs := make([]comdlgFilterSpec, 0, len(filters))
	// Keep UTF-16 backing slices alive for the duration of the vtable call.
	var backing [][]uint16

	for _, f := range filters {
		nameW, err := windows.UTF16FromString(f.Name)
		if err != nil {
			continue
		}
		specW, err := windows.UTF16FromString(dlgBuildFilterSpec(f.Extensions))
		if err != nil {
			continue
		}
		backing = append(backing, nameW, specW)
		specs = append(specs, comdlgFilterSpec{pszName: &nameW[0], pszSpec: &specW[0]})
	}
	if len(specs) == 0 {
		return
	}
	syscall.SyscallN(comVtbl(dialog)[vtblDlgSetFileTypes],
		dialog,
		uintptr(len(specs)),
		uintptr(unsafe.Pointer(&specs[0])),
	)
	runtime.KeepAlive(backing)
}

// dlgBuildFilterSpec converts extension slice to a Windows filter spec like "*.png;*.jpg".
func dlgBuildFilterSpec(exts []string) string {
	parts := make([]string, 0, len(exts))
	for _, e := range exts {
		e = strings.TrimPrefix(e, "*")
		e = strings.TrimPrefix(e, ".")
		if e != "" {
			parts = append(parts, "*."+e)
		}
	}
	if len(parts) == 0 {
		return defaultFilterPattern
	}
	return strings.Join(parts, ";")
}

// comVtbl returns the vtable of a COM interface pointer.
func comVtbl(obj uintptr) *[64]uintptr {
	return (*[64]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(obj)))) //nolint:govet // COM vtable dereference
}

// comRelease calls IUnknown::Release on a COM interface pointer.
func comRelease(obj uintptr) {
	if obj != 0 {
		syscall.SyscallN(comVtbl(obj)[vtblDlgRelease], obj)
	}
}

// shellItemPath extracts the filesystem path from an IShellItem via GetDisplayName.
func shellItemPath(item uintptr) string {
	if item == 0 {
		return ""
	}
	var pszPath uintptr
	hr, _, _ := syscall.SyscallN(comVtbl(item)[vtblSIGetDisplayName], item, sigdnFileSysPath, uintptr(unsafe.Pointer(&pszPath)))
	if hr != comSOK || pszPath == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pszPath)
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pszPath))) //nolint:govet // CoTaskMem pointer from IShellItem
}
