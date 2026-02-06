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
	wmPaint            = 0x000F
	wmEraseBkgnd       = 0x0014
	wmSize             = 0x0005
	wmClose            = 0x0010
	wmSetCursor        = 0x0020
	wmEnterSizeMove    = 0x0231 // Start of resize/move modal loop
	wmExitSizeMove     = 0x0232 // End of resize/move modal loop
	wmKeydown          = 0x0100
	wmKeyup            = 0x0101
	wmSysKeydown       = 0x0104 // System key down (Alt, F10)
	wmSysKeyup         = 0x0105 // System key up (Alt, F10)
	htClient           = 1      // WM_SETCURSOR hit test code for client area
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

	// Keyboard lParam flags (GLFW/Ebiten pattern)
	kfExtended = 0x0100 // Extended key flag (bit 24 of lParam >> 16)

	// WM_TIMER for rendering during modal drag/resize loop.
	// Timer interval is 1ms so VSync naturally paces at ~60fps.
	// With 16ms, Windows' default 15.6ms timer resolution causes the timer
	// to fire every ~31ms (skips the first 15.6ms interrupt because 15.6 < 16),
	// resulting in ~30fps instead of 60fps.
	// With 1ms, the timer fires at the first system interrupt (~15.6ms),
	// and VSync blocks for ~16ms, giving a natural ~60fps cadence.
	wmTimer         = 0x0113
	wmNCLButtonDown = 0x00A1 // Non-client left button down (title bar, borders)
	renderTimerID   = 1      // Timer ID for modal-loop rendering
	renderTimerMS   = 1      // 1ms: fires at first system interrupt, VSync paces naturally

	// PeekMessage flags
	pmNoRemove = 0x0000

	// Scancodes for modifier keys (GLFW pattern)
	// Left-side keys use base scancode
	// Right-side keys use base scancode | 0x100 (extended)
	scLeftShift    = 0x2A
	scRightShift   = 0x36
	scLeftControl  = 0x1D
	scRightControl = 0x11D // 0x1D | 0x100
	scLeftAlt      = 0x38
	scRightAlt     = 0x138 // 0x38 | 0x100
	scLeftSuper    = 0x15B // Extended
	scRightSuper   = 0x15C // Extended
)

// msgStruct is the Windows MSG structure for PeekMessage.
type msgStruct struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

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
	procGetMessageTime     = user32.NewProc("GetMessageTime")
	procSetTimer           = user32.NewProc("SetTimer")
	procKillTimer          = user32.NewProc("KillTimer")
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

	// Callbacks for pointer, scroll, and keyboard events
	pointerCallback    func(gpucontext.PointerEvent)
	scrollCallback     func(gpucontext.ScrollEvent)
	keyboardCallback   func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)
	modalFrameCallback func() // Called on WM_TIMER during modal drag/resize
	callbackMu         sync.RWMutex

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
		style:         0, // No CS_HREDRAW|CS_VREDRAW: prevents full invalidation on resize
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

// SetKeyCallback registers a callback for keyboard events.
func (p *windowsPlatform) SetKeyCallback(fn func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)) {
	p.callbackMu.Lock()
	p.keyboardCallback = fn
	p.callbackMu.Unlock()
}

// SetModalFrameCallback registers a callback invoked via WM_TIMER during
// the Win32 modal drag/resize loop to keep rendering alive.
//
// When the user drags or resizes a window, DefWindowProc enters a modal
// message loop that blocks the application's main loop. A 16ms timer
// (~60fps) fires WM_TIMER messages inside this modal loop, invoking
// the callback to render frames and update application state.
//
// The callback runs on the main thread (same as the normal main loop),
// preserving the existing serialization between onUpdate and onDraw.
//
// Future: An independent render thread would eliminate this mechanism
// by decoupling the render loop from the message pump. See ROADMAP.md.
func (p *windowsPlatform) SetModalFrameCallback(fn func()) {
	p.callbackMu.Lock()
	p.modalFrameCallback = fn
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

// dispatchKeyEvent dispatches a keyboard event to the registered callback.
func (p *windowsPlatform) dispatchKeyEvent(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) {
	p.callbackMu.RLock()
	callback := p.keyboardCallback
	p.callbackMu.RUnlock()

	if callback != nil {
		callback(key, mods, pressed)
	}
}

// Virtual key code constants for keyboard handling.
const (
	vkBack      = 0x08
	vkTab       = 0x09
	vkReturn    = 0x0D
	vkShift     = 0x10
	vkControl   = 0x11
	vkMenu      = 0x12 // Alt
	vkPause     = 0x13
	vkCapital   = 0x14 // Caps Lock
	vkSpace     = 0x20
	vkPrior     = 0x21 // Page Up
	vkNext      = 0x22 // Page Down
	vkEnd       = 0x23
	vkHome      = 0x24
	vkLeftKey   = 0x25
	vkUpKey     = 0x26
	vkRightKey  = 0x27
	vkDownKey   = 0x28
	vkInsert    = 0x2D
	vkDeleteKey = 0x2E
	vkLWin      = 0x5B
	vkRWin      = 0x5C
	vkNumpad0   = 0x60
	vkNumpad1   = 0x61
	vkNumpad2   = 0x62
	vkNumpad3   = 0x63
	vkNumpad4   = 0x64
	vkNumpad5   = 0x65
	vkNumpad6   = 0x66
	vkNumpad7   = 0x67
	vkNumpad8   = 0x68
	vkNumpad9   = 0x69
	vkMultiply  = 0x6A
	vkAdd       = 0x6B
	vkSubtract  = 0x6D
	vkDecimal   = 0x6E
	vkDivide    = 0x6F
	vkF1        = 0x70
	vkF2        = 0x71
	vkF3        = 0x72
	vkF4        = 0x73
	vkF5        = 0x74
	vkF6        = 0x75
	vkF7        = 0x76
	vkF8        = 0x77
	vkF9        = 0x78
	vkF10       = 0x79
	vkF11       = 0x7A
	vkF12       = 0x7B
	vkNumLock   = 0x90
	vkScroll    = 0x91 // Scroll Lock
	vkLShift    = 0xA0
	vkRShift    = 0xA1
	vkLControl  = 0xA2
	vkRControl  = 0xA3
	vkLMenu     = 0xA4 // Left Alt
	vkRMenu     = 0xA5 // Right Alt
	vkOEM1      = 0xBA // ;:
	vkOEMPlus   = 0xBB // =+
	vkOEMComma  = 0xBC // ,<
	vkOEMMinus  = 0xBD // -_
	vkOEMPeriod = 0xBE // .>
	vkOEM2      = 0xBF // /?
	vkOEM3      = 0xC0 // `~
	vkOEM4      = 0xDB // [{
	vkOEM5      = 0xDC // \|
	vkOEM6      = 0xDD // ]}
	vkOEM7      = 0xDE // '"
)

// getKeyState calls GetKeyState to check if a key is pressed.
var procGetKeyState = user32.NewProc("GetKeyState")

// getKeyModifiers returns the current keyboard modifier state.
func getKeyModifiers() gpucontext.Modifiers {
	var mods gpucontext.Modifiers

	// Check shift
	ret, _, _ := procGetKeyState.Call(uintptr(vkShift))
	if int16(ret) < 0 {
		mods |= gpucontext.ModShift
	}

	// Check control
	ret, _, _ = procGetKeyState.Call(uintptr(vkControl))
	if int16(ret) < 0 {
		mods |= gpucontext.ModControl
	}

	// Check alt
	ret, _, _ = procGetKeyState.Call(uintptr(vkMenu))
	if int16(ret) < 0 {
		mods |= gpucontext.ModAlt
	}

	// Check super (Windows key)
	retL, _, _ := procGetKeyState.Call(uintptr(vkLWin))
	retR, _, _ := procGetKeyState.Call(uintptr(vkRWin))
	if int16(retL) < 0 || int16(retR) < 0 {
		mods |= gpucontext.ModSuper
	}

	// Check caps lock (toggle state)
	ret, _, _ = procGetKeyState.Call(uintptr(vkCapital))
	if ret&1 != 0 {
		mods |= gpucontext.ModCapsLock
	}

	// Check num lock (toggle state)
	ret, _, _ = procGetKeyState.Call(uintptr(vkNumLock))
	if ret&1 != 0 {
		mods |= gpucontext.ModNumLock
	}

	return mods
}

// vkCodeToKey converts a Windows virtual key code to gpucontext.Key.
//
// vkCodeToKey converts a Windows virtual key code to gpucontext.Key.
func vkCodeToKey(vkCode uintptr) gpucontext.Key {
	// Letters A-Z (0x41-0x5A)
	if vkCode >= 0x41 && vkCode <= 0x5A {
		return gpucontext.KeyA + gpucontext.Key(vkCode-0x41)
	}

	// Numbers 0-9 (0x30-0x39)
	if vkCode >= 0x30 && vkCode <= 0x39 {
		return gpucontext.Key0 + gpucontext.Key(vkCode-0x30)
	}

	// Function keys F1-F12
	if vkCode >= vkF1 && vkCode <= vkF12 {
		return gpucontext.KeyF1 + gpucontext.Key(vkCode-vkF1)
	}

	// Numpad 0-9
	if vkCode >= vkNumpad0 && vkCode <= vkNumpad9 {
		return gpucontext.KeyNumpad0 + gpucontext.Key(vkCode-vkNumpad0)
	}

	switch vkCode {
	// Navigation
	case vkEscape:
		return gpucontext.KeyEscape
	case vkTab:
		return gpucontext.KeyTab
	case vkBack:
		return gpucontext.KeyBackspace
	case vkReturn:
		return gpucontext.KeyEnter
	case vkSpace:
		return gpucontext.KeySpace
	case vkInsert:
		return gpucontext.KeyInsert
	case vkDeleteKey:
		return gpucontext.KeyDelete
	case vkHome:
		return gpucontext.KeyHome
	case vkEnd:
		return gpucontext.KeyEnd
	case vkPrior:
		return gpucontext.KeyPageUp
	case vkNext:
		return gpucontext.KeyPageDown
	case vkLeftKey:
		return gpucontext.KeyLeft
	case vkRightKey:
		return gpucontext.KeyRight
	case vkUpKey:
		return gpucontext.KeyUp
	case vkDownKey:
		return gpucontext.KeyDown

	// Modifiers
	case vkLShift:
		return gpucontext.KeyLeftShift
	case vkRShift:
		return gpucontext.KeyRightShift
	case vkLControl:
		return gpucontext.KeyLeftControl
	case vkRControl:
		return gpucontext.KeyRightControl
	case vkLMenu:
		return gpucontext.KeyLeftAlt
	case vkRMenu:
		return gpucontext.KeyRightAlt
	case vkLWin:
		return gpucontext.KeyLeftSuper
	case vkRWin:
		return gpucontext.KeyRightSuper

	// Punctuation
	case vkOEMMinus:
		return gpucontext.KeyMinus
	case vkOEMPlus:
		return gpucontext.KeyEqual
	case vkOEM4:
		return gpucontext.KeyLeftBracket
	case vkOEM6:
		return gpucontext.KeyRightBracket
	case vkOEM5:
		return gpucontext.KeyBackslash
	case vkOEM1:
		return gpucontext.KeySemicolon
	case vkOEM7:
		return gpucontext.KeyApostrophe
	case vkOEM3:
		return gpucontext.KeyGrave
	case vkOEMComma:
		return gpucontext.KeyComma
	case vkOEMPeriod:
		return gpucontext.KeyPeriod
	case vkOEM2:
		return gpucontext.KeySlash

	// Numpad operators
	case vkMultiply:
		return gpucontext.KeyNumpadMultiply
	case vkAdd:
		return gpucontext.KeyNumpadAdd
	case vkSubtract:
		return gpucontext.KeyNumpadSubtract
	case vkDecimal:
		return gpucontext.KeyNumpadDecimal
	case vkDivide:
		return gpucontext.KeyNumpadDivide

	// Lock keys
	case vkCapital:
		return gpucontext.KeyCapsLock
	case vkScroll:
		return gpucontext.KeyScrollLock
	case vkNumLock:
		return gpucontext.KeyNumLock
	case vkPause:
		return gpucontext.KeyPause
	}

	return gpucontext.KeyUnknown
}

// translateKey converts a Windows key event to gpucontext.Key using the GLFW/Ebiten pattern.
// It uses scancode and KF_EXTENDED flag for accurate Left/Right modifier detection,
// and handles AltGr (Ctrl+Alt on European keyboards) correctly.
//
// This is the enterprise-grade approach used by GLFW, SDL, and Ebiten.
func translateKey(wParam, lParam uintptr) gpucontext.Key {
	// Extract scancode with extended bit from lParam
	// Bits 16-23: scancode, bit 24: extended flag
	scancode := int((lParam >> 16) & (kfExtended | 0xFF))

	// Special handling for modifier keys (GLFW pattern)
	switch wParam {
	case vkShift:
		// Distinguish Left/Right Shift by scancode
		if scancode == scRightShift {
			return gpucontext.KeyRightShift
		}
		return gpucontext.KeyLeftShift

	case vkControl:
		// Check extended bit for Right Control
		if scancode&kfExtended != 0 {
			return gpucontext.KeyRightControl
		}
		// AltGr detection: Left Ctrl + Right Alt sent together
		// GLFW/Ebiten hack: check if next message is Right Alt with same timestamp
		if isAltGrSequence() {
			return gpucontext.KeyUnknown // Skip Ctrl part of AltGr
		}
		return gpucontext.KeyLeftControl

	case vkMenu: // Alt
		// Check extended bit for Right Alt
		if scancode&kfExtended != 0 {
			return gpucontext.KeyRightAlt
		}
		return gpucontext.KeyLeftAlt
	}

	// For non-modifier keys, use the standard vkCode mapping
	return vkCodeToKey(wParam)
}

// isAltGrSequence checks if the current Left Ctrl is part of an AltGr sequence.
// AltGr on European keyboards sends Left Ctrl + Right Alt with the same timestamp.
// We detect this by peeking at the next message in the queue.
//
// This is the standard GLFW/Ebiten approach for handling AltGr correctly.
func isAltGrSequence() bool {
	// Get current message timestamp
	currentTime, _, _ := procGetMessageTime.Call()

	// Peek at next message without removing it
	var next msgStruct
	ret, _, _ := procPeekMessageW.Call(
		uintptr(unsafe.Pointer(&next)),
		0, // NULL hwnd = all windows
		0, // wMsgFilterMin
		0, // wMsgFilterMax
		uintptr(pmNoRemove),
	)

	if ret == 0 {
		return false // No message in queue
	}

	// Check if next message is a key event
	if next.message != wmKeydown && next.message != wmSysKeydown &&
		next.message != wmKeyup && next.message != wmSysKeyup {
		return false
	}

	// Check if it's Right Alt (VK_MENU with extended bit) with same timestamp
	if next.wParam == vkMenu {
		nextScancode := int((next.lParam >> 16) & (kfExtended | 0xFF))
		if nextScancode&kfExtended != 0 && next.time == uint32(currentTime) {
			return true // This is AltGr sequence
		}
	}

	return false
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
//nolint:maintidx,gocognit // message dispatch functions inherently have high complexity
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

	case wmEraseBkgnd:
		// Suppress background erase during resize (Ebiten/GLFW pattern).
		// Without this, Windows fills the invalidated region with the window
		// class background brush, causing visible flicker during resize.
		// GPU-rendered apps handle all drawing; no GDI erase is needed.
		return 1

	case wmPaint:
		// Validate the paint region without drawing anything via GDI.
		// All rendering is done through the GPU pipeline (Vulkan/DX12).
		// We must call DefWindowProc to validate the region, otherwise
		// Windows sends WM_PAINT continuously.
		ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
		return ret

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

		// During modal resize loop, don't queue events - wait for WM_EXITSIZEMOVE.
		// DWM stretches the old swapchain content to the new window size; this is
		// standard GPU-app behavior on Windows (Chrome, VS Code, Electron all do this).
		// Resizing the swapchain during modal causes worse artifacts (flicker between
		// stretched and correctly-sized frames) because DWM stretches BEFORE our
		// render can complete.
		if sizeChanged && !inSizeMove {
			p.queueEvent(Event{
				Type:   EventResize,
				Width:  newWidth,
				Height: newHeight,
			})
		}
		return 0

	case wmNCLButtonDown:
		// Start render timer BEFORE DefWindowProc enters modal drag detection.
		// When the user clicks the title bar or resize border, DefWindowProc runs
		// a nested modal loop to distinguish click from drag (~500ms delay).
		// Starting the timer here keeps animation alive during that delay.
		procSetTimer.Call(uintptr(p.hwnd), renderTimerID, renderTimerMS, 0)

		// DefWindowProc handles the actual drag/resize detection (may block).
		ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)

		// If DefWindowProc returned without entering a modal loop, kill the timer.
		// WM_ENTERSIZEMOVE sets inSizeMove=true; if still false, no modal loop started.
		p.sizeMu.RLock()
		inModal := p.inSizeMove
		p.sizeMu.RUnlock()
		if !inModal {
			procKillTimer.Call(uintptr(p.hwnd), renderTimerID)
		}
		return ret

	case wmEnterSizeMove:
		p.sizeMu.Lock()
		p.inSizeMove = true
		p.sizeMu.Unlock()

		// Ensure render timer is running for the modal resize/move loop.
		// Timer may already be running from WM_NCLBUTTONDOWN; SetTimer with
		// the same ID safely replaces it (no duplicate timers).
		procSetTimer.Call(uintptr(p.hwnd), renderTimerID, renderTimerMS, 0)
		return 0

	case wmExitSizeMove:
		// Stop the render timer — normal main loop rendering resumes.
		procKillTimer.Call(uintptr(p.hwnd), renderTimerID)

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

	case wmTimer:
		if wParam == renderTimerID {
			// Invoke the modal frame callback to render a frame during
			// the modal drag/resize loop. The callback runs on the main
			// thread, preserving serialization with onUpdate/onDraw.
			p.callbackMu.RLock()
			callback := p.modalFrameCallback
			p.callbackMu.RUnlock()

			if callback != nil {
				callback()
			}
			return 0
		}

	case wmKeydown, wmSysKeydown:
		// Convert to Key using scancode-based translation (GLFW/Ebiten pattern)
		// This correctly handles Left/Right modifiers and AltGr
		key := translateKey(wParam, lParam)
		mods := getKeyModifiers()

		// Skip if key is unknown (e.g., Ctrl part of AltGr sequence)
		if key == gpucontext.KeyUnknown {
			return 0
		}

		// Dispatch keyboard event
		p.dispatchKeyEvent(key, mods, true)

		// ESC to close (convenience)
		if wParam == vkEscape {
			p.shouldClose = true
			p.queueEvent(Event{Type: EventClose})
		}

		// For WM_SYSKEYDOWN: let DefWindowProc handle Alt+F4, Alt+Tab
		// but suppress menu activation on Alt alone
		if message == wmSysKeydown {
			// Alt+F4 should still close the window
			if wParam == vkF4 && mods&gpucontext.ModAlt != 0 {
				ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
				return ret
			}
			// Suppress other system key behavior (menu activation)
			return 0
		}
		return 0

	case wmKeyup, wmSysKeyup:
		// Convert to Key using scancode-based translation (GLFW/Ebiten pattern)
		key := translateKey(wParam, lParam)
		mods := getKeyModifiers()

		// Skip if key is unknown (e.g., Ctrl part of AltGr sequence)
		if key == gpucontext.KeyUnknown {
			return 0
		}

		// Dispatch keyboard event
		p.dispatchKeyEvent(key, mods, false)

		// For WM_SYSKEYUP: suppress menu activation
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
