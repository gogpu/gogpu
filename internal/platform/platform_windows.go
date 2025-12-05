//go:build windows

package platform

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 constants
const (
	csHRedraw          = 0x0002
	csVRedraw          = 0x0001
	wmDestroy          = 0x0002
	wmSize             = 0x0005
	wmClose            = 0x0010
	wmKeydown          = 0x0100
	wmKeyup            = 0x0101
	idcArrow           = 32512
	swShowNormal       = 1
	pmRemove           = 0x0001
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	cwUseDefault       = 0x80000000
	vkEscape           = 0x1B
)

var (
	user32               = windows.NewLazyDLL("user32.dll")
	kernel32             = windows.NewLazyDLL("kernel32.dll")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procPeekMessageW     = user32.NewProc("PeekMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procGetClientRect    = user32.NewProc("GetClientRect")
)

// WNDCLASSEXW is the Win32 WNDCLASSEXW structure.
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

// MSG is the Win32 MSG structure.
type msg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// RECT is the Win32 RECT structure.
type rect struct {
	left, top, right, bottom int32
}

// windowsPlatform implements Platform for Windows.
type windowsPlatform struct {
	hwnd        windows.HWND
	hinstance   windows.Handle
	width       int
	height      int
	shouldClose bool
	events      []Event
	eventMu     sync.Mutex
}

// Global instance for window procedure callback
var globalPlatform *windowsPlatform

func newPlatform() Platform {
	return &windowsPlatform{}
}

func (p *windowsPlatform) Init(config Config) error {
	// Store global reference for callback
	globalPlatform = p

	// Get HINSTANCE
	ret, _, _ := procGetModuleHandleW.Call(0)
	p.hinstance = windows.Handle(ret)

	// Register window class
	className, err := windows.UTF16PtrFromString("GoGPUWindow")
	if err != nil {
		return fmt.Errorf("utf16 class name: %w", err)
	}

	wndClass := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     p.hinstance,
		lpszClassName: className,
	}

	// Load default cursor
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))
	wndClass.hCursor = windows.Handle(cursor)

	ret, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass)))
	if ret == 0 {
		return fmt.Errorf("RegisterClassExW failed")
	}

	// Create window
	titlePtr, err := windows.UTF16PtrFromString(config.Title)
	if err != nil {
		return fmt.Errorf("utf16 title: %w", err)
	}

	style := uintptr(wsOverlappedWindow | wsVisible)

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(titlePtr)),
		style,
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(config.Width),
		uintptr(config.Height),
		0, 0,
		uintptr(p.hinstance),
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed")
	}

	p.hwnd = windows.HWND(hwnd)
	p.width = config.Width
	p.height = config.Height

	// Show window
	procShowWindow.Call(uintptr(p.hwnd), swShowNormal)
	procUpdateWindow.Call(uintptr(p.hwnd))

	// Get actual client size
	p.updateSize()

	return nil
}

func (p *windowsPlatform) updateSize() {
	var r rect
	procGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&r)))
	p.width = int(r.right - r.left)
	p.height = int(r.bottom - r.top)
}

func (p *windowsPlatform) PollEvents() Event {
	// Process all pending Windows messages
	var m msg
	for {
		ret, _, _ := procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
			pmRemove,
		)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	// Return queued event if any
	p.eventMu.Lock()
	defer p.eventMu.Unlock()

	if len(p.events) > 0 {
		event := p.events[0]
		p.events = p.events[1:]
		return event
	}

	return Event{Type: EventNone}
}

func (p *windowsPlatform) ShouldClose() bool {
	return p.shouldClose
}

func (p *windowsPlatform) GetSize() (width, height int) {
	return p.width, p.height
}

func (p *windowsPlatform) GetHandle() (instance, window uintptr) {
	return uintptr(p.hinstance), uintptr(p.hwnd)
}

func (p *windowsPlatform) Destroy() {
	if p.hwnd != 0 {
		procDestroyWindow.Call(uintptr(p.hwnd))
		p.hwnd = 0
	}
	globalPlatform = nil
}

func (p *windowsPlatform) queueEvent(event Event) {
	p.eventMu.Lock()
	defer p.eventMu.Unlock()
	p.events = append(p.events, event)
}

// wndProc is the window procedure callback.
func wndProc(hwnd windows.HWND, message uint32, wParam, lParam uintptr) uintptr {
	p := globalPlatform
	if p == nil {
		ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
		return ret
	}

	switch message {
	case wmClose:
		p.shouldClose = true
		p.queueEvent(Event{Type: EventClose})
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0

	case wmSize:
		newWidth := int(lParam & 0xFFFF)
		newHeight := int((lParam >> 16) & 0xFFFF)
		if newWidth > 0 && newHeight > 0 && (newWidth != p.width || newHeight != p.height) {
			p.width = newWidth
			p.height = newHeight
			p.queueEvent(Event{
				Type:   EventResize,
				Width:  newWidth,
				Height: newHeight,
			})
		}
		return 0

	case wmKeydown:
		// ESC to close (convenience)
		if wParam == vkEscape {
			p.shouldClose = true
			p.queueEvent(Event{Type: EventClose})
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return ret
}
