//go:build windows

package platform

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gogpu/gpucontext"
	"golang.org/x/sys/windows"
)

// Win32 constants
const (
	csHRedraw          = 0x0002
	csVRedraw          = 0x0001
	wmDestroy          = 0x0002
	wmSize             = 0x0005
	wmClose            = 0x0010
	wmSetCursor        = 0x0020
	wmEnterSizeMove    = 0x0231 // Start of resize/move modal loop
	wmExitSizeMove     = 0x0232 // End of resize/move modal loop
	wmKeydown          = 0x0100
	wmKeyup            = 0x0101
	htClient           = 1 // WM_SETCURSOR hit test code for client area
	idcArrow           = 32512
	swShowNormal       = 1
	swShow             = 5
	swRestore          = 9
	pmRemove           = 0x0001
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	cwUseDefault       = 0x80000000
	vkEscape           = 0x1B
	swpNoActivate      = 0x0010 // SWP_NOACTIVATE

	// Mouse messages
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmMButtonDown = 0x0207
	wmMButtonUp   = 0x0208
	wmMouseWheel  = 0x020A
	wmMouseHWheel = 0x020E
	wmXButtonDown = 0x020B
	wmXButtonUp   = 0x020C
	wmMouseLeave  = 0x02A3

	// Mouse button flags in wParam
	mkLButton  = 0x0001
	mkRButton  = 0x0002
	mkShift    = 0x0004
	mkControl  = 0x0008
	mkMButton  = 0x0010
	mkXButton1 = 0x0020
	mkXButton2 = 0x0040

	// XBUTTON identifiers in HIWORD of wParam for WM_XBUTTONDOWN/UP
	xButton1 = 0x0001
	xButton2 = 0x0002

	// Wheel delta constant
	wheelDelta = 120

	// TrackMouseEvent flags
	tmeLeave = 0x0002
)

var (
	user32                 = windows.NewLazyDLL("user32.dll")
	kernel32               = windows.NewLazyDLL("kernel32.dll")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procShowWindow         = user32.NewProc("ShowWindow")
	procUpdateWindow       = user32.NewProc("UpdateWindow")
	procSetForegroundWnd   = user32.NewProc("SetForegroundWindow")
	procGetForegroundWnd   = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput  = user32.NewProc("AttachThreadInput")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procPostQuitMessage    = user32.NewProc("PostQuitMessage")
	procLoadCursorW        = user32.NewProc("LoadCursorW")
	procSetCursor          = user32.NewProc("SetCursor")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procTrackMouseEvent    = user32.NewProc("TrackMouseEvent")
)

// trackMouseEventStruct is the TRACKMOUSEEVENT structure.
type trackMouseEventStruct struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   windows.HWND
	dwHoverTime uint32
}

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
	cursor      uintptr // Default arrow cursor for WM_SETCURSOR
	width       int
	height      int
	shouldClose bool
	inSizeMove  bool // True during modal resize/move loop
	events      []Event
	eventMu     sync.Mutex
	sizeMu      sync.RWMutex // Protects width, height, inSizeMove for thread-safe access

	// Mouse state tracking
	mouseX        float64
	mouseY        float64
	buttons       gpucontext.Buttons
	modifiers     gpucontext.Modifiers
	mouseInWindow bool
	mouseMu       sync.RWMutex // Protects mouse state

	// Callbacks for pointer and scroll events
	pointerCallback func(gpucontext.PointerEvent)
	scrollCallback  func(gpucontext.ScrollEvent)
	callbackMu      sync.RWMutex

	// Timestamp reference for event timing
	startTime time.Time
}

// Global instance for window procedure callback
var globalPlatform *windowsPlatform

func newPlatform() Platform {
	return &windowsPlatform{
		startTime: time.Now(),
	}
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
	p.cursor = cursor // Store for WM_SETCURSOR handling

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

	p.sizeMu.Lock()
	p.width = int(r.right - r.left)
	p.height = int(r.bottom - r.top)
	p.sizeMu.Unlock()
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
	p.sizeMu.RLock()
	defer p.sizeMu.RUnlock()
	return p.width, p.height
}

// InSizeMove returns true if the window is in a modal resize/move loop.
// During this time, rendering should continue but swapchain recreation
// should be deferred to prevent hangs.
func (p *windowsPlatform) InSizeMove() bool {
	p.sizeMu.RLock()
	defer p.sizeMu.RUnlock()
	return p.inSizeMove
}

func (p *windowsPlatform) GetHandle() (instance, window uintptr) {
	return uintptr(p.hinstance), uintptr(p.hwnd)
}

// SetPointerCallback registers a callback for pointer events.
func (p *windowsPlatform) SetPointerCallback(fn func(gpucontext.PointerEvent)) {
	p.callbackMu.Lock()
	p.pointerCallback = fn
	p.callbackMu.Unlock()
}

// SetScrollCallback registers a callback for scroll events.
func (p *windowsPlatform) SetScrollCallback(fn func(gpucontext.ScrollEvent)) {
	p.callbackMu.Lock()
	p.scrollCallback = fn
	p.callbackMu.Unlock()
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

	// Coalesce resize events to avoid swapchain recreation storm.
	// During drag resize, Windows sends hundreds of WM_SIZE messages.
	// We only care about the final size.
	if event.Type == EventResize && len(p.events) > 0 {
		last := &p.events[len(p.events)-1]
		if last.Type == EventResize {
			// Update existing resize event with new dimensions
			last.Width = event.Width
			last.Height = event.Height
			return
		}
	}

	p.events = append(p.events, event)
}

// extractMousePos extracts mouse position from lParam.
// Returns signed coordinates (can be negative near screen edges).
func extractMousePos(lParam uintptr) (x, y float64) {
	// Low word is X, high word is Y (signed 16-bit values)
	xRaw := int16(lParam & 0xFFFF)
	yRaw := int16((lParam >> 16) & 0xFFFF)
	return float64(xRaw), float64(yRaw)
}

// extractModifiers extracts keyboard modifiers from wParam mouse flags.
func extractModifiers(wParam uintptr) gpucontext.Modifiers {
	var mods gpucontext.Modifiers
	if wParam&mkShift != 0 {
		mods |= gpucontext.ModShift
	}
	if wParam&mkControl != 0 {
		mods |= gpucontext.ModControl
	}
	// Note: Alt key state not available in mouse wParam,
	// would need GetKeyState(VK_MENU) for that
	return mods
}

// extractButtons extracts button state from wParam mouse flags.
func extractButtons(wParam uintptr) gpucontext.Buttons {
	var btns gpucontext.Buttons
	if wParam&mkLButton != 0 {
		btns |= gpucontext.ButtonsLeft
	}
	if wParam&mkRButton != 0 {
		btns |= gpucontext.ButtonsRight
	}
	if wParam&mkMButton != 0 {
		btns |= gpucontext.ButtonsMiddle
	}
	if wParam&mkXButton1 != 0 {
		btns |= gpucontext.ButtonsX1
	}
	if wParam&mkXButton2 != 0 {
		btns |= gpucontext.ButtonsX2
	}
	return btns
}

// extractWheelDelta extracts wheel delta from wParam.
// Returns normalized delta (positive = up/right).
func extractWheelDelta(wParam uintptr) float64 {
	// HIWORD is signed wheel delta
	delta := int16(wParam >> 16)
	return float64(delta) / wheelDelta
}

// extractXButton extracts which X button from wParam for WM_XBUTTONDOWN/UP.
func extractXButton(wParam uintptr) gpucontext.Button {
	xButton := (wParam >> 16) & 0xFFFF
	if xButton == xButton1 {
		return gpucontext.ButtonX1
	}
	if xButton == xButton2 {
		return gpucontext.ButtonX2
	}
	return gpucontext.ButtonNone
}

// dispatchPointerEvent dispatches a pointer event to the registered callback.
func (p *windowsPlatform) dispatchPointerEvent(ev gpucontext.PointerEvent) {
	p.callbackMu.RLock()
	callback := p.pointerCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// dispatchScrollEvent dispatches a scroll event to the registered callback.
func (p *windowsPlatform) dispatchScrollEvent(ev gpucontext.ScrollEvent) {
	p.callbackMu.RLock()
	callback := p.scrollCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(ev)
	}
}

// trackMouseLeave enables WM_MOUSELEAVE tracking.
func (p *windowsPlatform) trackMouseLeave() {
	tme := trackMouseEventStruct{
		cbSize:    uint32(unsafe.Sizeof(trackMouseEventStruct{})),
		dwFlags:   tmeLeave,
		hwndTrack: p.hwnd,
	}
	// TrackMouseEvent returns BOOL; we ignore the result as failure is non-fatal
	ret, _, _ := procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
	_ = ret // Ignore return value
}

// eventTimestamp returns the event timestamp as duration since start.
func (p *windowsPlatform) eventTimestamp() time.Duration {
	return time.Since(p.startTime)
}

// createPointerEvent creates a PointerEvent with common fields filled in.
func (p *windowsPlatform) createPointerEvent(
	eventType gpucontext.PointerEventType,
	button gpucontext.Button,
	x, y float64,
	wParam uintptr,
) gpucontext.PointerEvent {
	buttons := extractButtons(wParam)
	modifiers := extractModifiers(wParam)

	// For button down/up, set pressure based on button state
	var pressure float32
	if eventType == gpucontext.PointerDown || buttons != gpucontext.ButtonsNone {
		pressure = 0.5 // Default pressure for mouse
	}

	return gpucontext.PointerEvent{
		Type:        eventType,
		PointerID:   1, // Mouse always has ID 1
		X:           x,
		Y:           y,
		Pressure:    pressure,
		TiltX:       0,
		TiltY:       0,
		Twist:       0,
		Width:       1,
		Height:      1,
		PointerType: gpucontext.PointerTypeMouse,
		IsPrimary:   true,
		Button:      button,
		Buttons:     buttons,
		Modifiers:   modifiers,
		Timestamp:   p.eventTimestamp(),
	}
}

// wndProc is the window procedure callback.
//
//nolint:maintidx // message dispatch functions inherently have high complexity
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

		p.sizeMu.Lock()
		sizeChanged := newWidth > 0 && newHeight > 0 && (newWidth != p.width || newHeight != p.height)
		inSizeMove := p.inSizeMove
		if sizeChanged {
			p.width = newWidth
			p.height = newHeight
		}
		p.sizeMu.Unlock()

		// During modal resize loop, don't queue events - wait for WM_EXITSIZEMOVE
		if sizeChanged && !inSizeMove {
			p.queueEvent(Event{
				Type:   EventResize,
				Width:  newWidth,
				Height: newHeight,
			})
		}
		return 0

	case wmEnterSizeMove:
		p.sizeMu.Lock()
		p.inSizeMove = true
		p.sizeMu.Unlock()
		return 0

	case wmExitSizeMove:
		p.sizeMu.Lock()
		p.inSizeMove = false
		p.sizeMu.Unlock()

		// Queue final resize event when resize ends
		p.updateSize()
		width, height := p.GetSize()
		p.queueEvent(Event{
			Type:   EventResize,
			Width:  width,
			Height: height,
		})
		return 0

	case wmKeydown:
		// ESC to close (convenience)
		if wParam == vkEscape {
			p.shouldClose = true
			p.queueEvent(Event{Type: EventClose})
		}
		return 0

	case wmSetCursor:
		// Restore cursor to arrow when in client area.
		// This fixes resize cursor staying after resize ends.
		hitTest := lParam & 0xFFFF
		if hitTest == htClient {
			_, _, _ = procSetCursor.Call(p.cursor)
			return 1 // Cursor was set
		}
		// Let Windows handle non-client area cursors
		ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
		return ret

	// Mouse movement
	case wmMouseMove:
		x, y := extractMousePos(lParam)

		// Track mouse enter/leave
		p.mouseMu.Lock()
		wasInWindow := p.mouseInWindow
		p.mouseX = x
		p.mouseY = y
		p.buttons = extractButtons(wParam)
		p.modifiers = extractModifiers(wParam)
		p.mouseInWindow = true
		p.mouseMu.Unlock()

		// First move in window - send PointerEnter
		if !wasInWindow {
			p.trackMouseLeave()
			ev := p.createPointerEvent(gpucontext.PointerEnter, gpucontext.ButtonNone, x, y, wParam)
			p.dispatchPointerEvent(ev)
		}

		// Always send PointerMove
		ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmMouseLeave:
		p.mouseMu.Lock()
		x, y := p.mouseX, p.mouseY
		buttons := p.buttons
		modifiers := p.modifiers
		p.mouseInWindow = false
		p.mouseMu.Unlock()

		ev := gpucontext.PointerEvent{
			Type:        gpucontext.PointerLeave,
			PointerID:   1,
			X:           x,
			Y:           y,
			Pressure:    0,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeMouse,
			IsPrimary:   true,
			Button:      gpucontext.ButtonNone,
			Buttons:     buttons,
			Modifiers:   modifiers,
			Timestamp:   p.eventTimestamp(),
		}
		p.dispatchPointerEvent(ev)
		return 0

	// Left button
	case wmLButtonDown:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonLeft, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmLButtonUp:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonLeft, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	// Right button
	case wmRButtonDown:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonRight, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmRButtonUp:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonRight, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	// Middle button
	case wmMButtonDown:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonMiddle, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmMButtonUp:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonMiddle, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	// X buttons (back/forward)
	case wmXButtonDown:
		x, y := extractMousePos(lParam)
		button := extractXButton(wParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, button, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 1 // Must return TRUE for XBUTTON messages

	case wmXButtonUp:
		x, y := extractMousePos(lParam)
		button := extractXButton(wParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, button, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 1 // Must return TRUE for XBUTTON messages

	// Vertical scroll wheel
	case wmMouseWheel:
		// For wheel messages, coordinates are screen-relative
		// We need to convert to client coordinates
		x, y := extractMousePos(lParam)
		deltaY := extractWheelDelta(wParam)

		ev := gpucontext.ScrollEvent{
			X:         x,
			Y:         y,
			DeltaX:    0,
			DeltaY:    -deltaY, // Invert: wheel up = scroll content up = negative deltaY
			DeltaMode: gpucontext.ScrollDeltaLine,
			Modifiers: extractModifiers(wParam),
			Timestamp: p.eventTimestamp(),
		}
		p.dispatchScrollEvent(ev)
		return 0

	// Horizontal scroll wheel
	case wmMouseHWheel:
		x, y := extractMousePos(lParam)
		deltaX := extractWheelDelta(wParam)

		ev := gpucontext.ScrollEvent{
			X:         x,
			Y:         y,
			DeltaX:    deltaX, // Positive = scroll content right
			DeltaY:    0,
			DeltaMode: gpucontext.ScrollDeltaLine,
			Modifiers: extractModifiers(wParam),
			Timestamp: p.eventTimestamp(),
		}
		p.dispatchScrollEvent(ev)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return ret
}
