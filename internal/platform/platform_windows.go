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
	wmChar             = 0x0102
	wmSysChar          = 0x0106 // System char (AltGr on European keyboards)
	wmUnichar          = 0x0109 // Unicode char from third-party IME
	unicodeNochar      = 0xFFFF // WM_UNICHAR sentinel: "do you support WM_UNICHAR?"
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

	// Pointer messages (WM_POINTER*, Windows 8+)
	wmPointerDown           = 0x0246
	wmPointerUp             = 0x0247
	wmPointerUpdate         = 0x0245
	wmPointerEnter          = 0x0249
	wmPointerLeave          = 0x024A
	wmPointerCaptureChanged = 0x024C

	// Pointer types (from GetPointerType)
	ptPointer = 0x00000001 // PT_POINTER (generic)
	ptTouch   = 0x00000002 // PT_TOUCH
	ptPen     = 0x00000003 // PT_PEN
	ptMouse   = 0x00000004 // PT_MOUSE

	// Pointer flags in POINTER_INFO
	pointerFlagInContact    = 0x00000004
	pointerFlagPrimary      = 0x00002000
	pointerFlagFirstButton  = 0x00000010
	pointerFlagSecondButton = 0x00000020
	pointerFlagThirdButton  = 0x00000040

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

	// Cursor IDs for LoadCursor
	idcHand     = 32649
	idcIBeam    = 32513
	idcCross    = 32515
	idcSizeAll  = 32646
	idcSizeNS   = 32645
	idcSizeWE   = 32644
	idcSizeNWSE = 32642
	idcSizeNESW = 32643
	idcNo       = 32648
	idcWait     = 32514

	// Clipboard format
	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	// SystemParametersInfo constants
	spiGetClientAreaAnimation = 0x1042
	spiGetHighContrast        = 0x0042
	hcfHighContrastOn         = 0x00000001

	// Frameless window constants
	wsPopup            = 0x80000000 // WS_POPUP
	wsThickFrame       = 0x00040000 // WS_THICKFRAME (for resize in frameless)
	wsCaption          = 0x00C00000 // WS_CAPTION (title bar)
	wmNCHitTest        = 0x0084     // WM_NCHITTEST
	wmNCCalcSize       = 0x0083     // WM_NCCALCSIZE
	wmNCPaint          = 0x0085     // WM_NCPAINT
	wmNCActivate       = 0x0086     // WM_NCACTIVATE
	wmNCUAHDrawCaption = 0x00AE     // Undocumented: UxTheme caption draw
	wmNCUAHDrawFrame   = 0x00AF     // Undocumented: UxTheme frame draw
	swMinimize         = 6          // SW_MINIMIZE
	swMaximize         = 3          // SW_MAXIMIZE

	// WM_NCHITTEST return values
	htCaption     = 2
	htSysMenu     = 3
	htMinButton   = 8
	htMaxButton   = 9
	htClose       = 20 // HTCLOSE
	htTop         = 12
	htBottom      = 15
	htLeft        = 10
	htRight       = 11
	htTopLeft     = 13
	htTopRight    = 14
	htBottomLeft  = 16
	htBottomRight = 17

	// SetWindowPos constants
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020

	// GetSystemMetrics / MonitorFromWindow constants
	smCXSizeFrame           = 32 // SM_CXSIZEFRAME
	smCYSizeFrame           = 33 // SM_CYSIZEFRAME
	smCXPaddedBorder        = 92 // SM_CXPADDEDBORDERWIDTH
	monitorDefaultToNearest = 2  // MONITOR_DEFAULTTONEAREST

	// DPI change message (Windows 8.1+)
	wmDpiChanged = 0x02E0 // WM_DPICHANGED

	// Focus messages
	wmActivate    = 0x0006 // WM_ACTIVATE
	waInactive    = 0      // WA_INACTIVE
	waActive      = 1      // WA_ACTIVE
	waClickActive = 2      // WA_CLICKACTIVE

	// WaitEvents / WakeUp constants
	wmWakeUp       = 0x0401     // WM_USER + 1 (custom wakeup message)
	qsAllinput     = 0x04FF     // QS_ALLINPUT
	mwmoInputAvail = 0x0004     // MWMO_INPUTAVAILABLE
	infinite       = 0xFFFFFFFF // INFINITE

	// Registry constants
	hkeyCurrentUser uintptr = 0x80000001
	keyRead         uintptr = 0x20019
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
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procTrackMouseEvent    = user32.NewProc("TrackMouseEvent")
	procGetMessageTime     = user32.NewProc("GetMessageTime")
	procSetTimer           = user32.NewProc("SetTimer")
	procKillTimer          = user32.NewProc("KillTimer")
	procSetWindowLongPtrW  = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos       = user32.NewProc("SetWindowPos")
	procIsZoomed           = user32.NewProc("IsZoomed")
	procScreenToClient     = user32.NewProc("ScreenToClient")
	procInvalidateRect     = user32.NewProc("InvalidateRect")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procMonitorFromWindow  = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW    = user32.NewProc("GetMonitorInfoW")

	// DWM (Desktop Window Manager) for frameless window shadow
	dwmapi                       = windows.NewLazyDLL("dwmapi.dll")
	procDwmExtendFrameIntoClient = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procDwmFlush                 = dwmapi.NewProc("DwmFlush")

	// WaitEvents / WakeUp
	procMsgWaitForMultipleObjectsEx = user32.NewProc("MsgWaitForMultipleObjectsEx")
	procPostMessageW                = user32.NewProc("PostMessageW")

	// DPI
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")

	// Clipboard
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	procGlobalFree       = kernel32.NewProc("GlobalFree")

	// Mouse capture (for drag tracking across window boundaries)
	procSetCapture     = user32.NewProc("SetCapture")
	procReleaseCapture = user32.NewProc("ReleaseCapture")

	// Cursor confinement and positioning (for CursorMode locked/confined)
	procClipCursor   = user32.NewProc("ClipCursor")
	procShowCursorW  = user32.NewProc("ShowCursor")
	procSetCursorPos = user32.NewProc("SetCursorPos")
	procGetCursorPos = user32.NewProc("GetCursorPos")

	// Pointer input (WM_POINTER*, Windows 8+)
	procGetPointerType    = user32.NewProc("GetPointerType")
	procGetPointerInfo    = user32.NewProc("GetPointerInfo")
	procGetPointerPenInfo = user32.NewProc("GetPointerPenInfo")

	// System preferences
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")

	// Registry (for dark mode)
	advapi32             = windows.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")

	// GDI32 (software backend pixel blitting)
	gdi32                 = windows.NewLazyDLL("gdi32.dll")
	procGetDC             = user32.NewProc("GetDC")
	procReleaseDC         = user32.NewProc("ReleaseDC")
	procSetDIBitsToDevice = gdi32.NewProc("SetDIBitsToDevice")
	procGetStockObject    = gdi32.NewProc("GetStockObject")
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

// POINT is the Win32 POINT structure.
type point struct {
	x, y int32
}

// pointerInfo is the Win32 POINTER_INFO structure.
type pointerInfo struct {
	pointerType           uint32
	pointerID             uint32
	frameID               uint32
	pointerFlags          uint32
	sourceDevice          uintptr
	hwndTarget            uintptr
	ptPixelLocation       point
	ptHimetricLocation    point
	ptPixelLocationRaw    point
	ptHimetricLocationRaw point
	dwTime                uint32
	historyCount          uint32
	inputData             int32
	dwKeyStates           uint32
	performanceCount      uint64
	buttonChangeType      int32
}

// pointerPenInfo is the Win32 POINTER_PEN_INFO structure.
type pointerPenInfo struct {
	pointerInfo pointerInfo
	penFlags    uint32
	penMask     uint32
	pressure    uint32 // 0-1024
	rotation    uint32
	tiltX       int32 // -90 to +90
	tiltY       int32 // -90 to +90
}

// bitmapInfoHeader is the Win32 BITMAPINFOHEADER structure for DIB operations.
type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32 // negative = top-down DIB
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32 // BI_RGB = 0
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
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

	// Frameless window state
	frameless       bool
	hitTestCallback func(x, y float64) gpucontext.HitTestResult
	maximized       bool

	// Callbacks for pointer, scroll, keyboard, and character input events
	pointerCallback    func(gpucontext.PointerEvent)
	scrollCallback     func(gpucontext.ScrollEvent)
	keyboardCallback   func(key gpucontext.Key, mods gpucontext.Modifiers, pressed bool)
	charCallback       func(rune)
	highSurrogate      uint16 // UTF-16 high surrogate for emoji/CJK supplementary chars
	modalFrameCallback func() // Called on WM_TIMER during modal drag/resize
	callbackMu         sync.RWMutex

	// Timestamp reference for event timing
	startTime time.Time

	// Cursor mode state (0=normal, 1=locked, 2=confined)
	cursorMode    int
	savedCursorX  int32 // saved cursor position before locking
	savedCursorY  int32
	cursorCenterX int32 // window center in screen coords (for warp-back)
	cursorCenterY int32
	cursorHidden  bool // tracks ShowCursor balance
}

// Global instance for window procedure callback
var globalPlatform *windowsPlatform

func newPlatform() Platform {
	return &windowsPlatform{
		startTime: time.Now(),
	}
}

func (p *windowsPlatform) Init(config Config) error {
	// Enable per-monitor DPI awareness programmatically.
	// Without this, Windows bitmap-upscales the app on high-DPI displays (200%+),
	// causing blurry text and incorrect mouse coordinates.
	// Try PerMonitorV2 (Win10 1703+), fallback to basic DPI aware (Vista+).
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	if err := procSetProcessDpiAwarenessContext.Find(); err == nil {
		procSetProcessDpiAwarenessContext.Call(^uintptr(3)) // -4 as uintptr
	} else if err := procSetProcessDPIAware.Find(); err == nil {
		procSetProcessDPIAware.Call()
	}

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

	// Set black background brush to prevent gray flash during resize/focus loss.
	// Without this, Windows draws the system default background (gray) between
	// GPU frame renders, causing visible flicker.
	blackBrush, _, _ := procGetStockObject.Call(4) // BLACK_BRUSH = 4
	wndClass.hbrBackground = windows.Handle(blackBrush)

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

	// Both frameless and normal use WS_OVERLAPPEDWINDOW for native resize + DWM shadow.
	// For frameless: WM_NCCALCSIZE removes title bar, WM_NCACTIVATE(-1) prevents
	// border repaint, WM_NCUAHDRAW* blocks UxTheme painting.
	var style uintptr
	if config.Frameless {
		// Create hidden — show after DWM setup + WM_NCCALCSIZE to avoid first-frame artifact.
		style = uintptr(wsOverlappedWindow)
	} else {
		style = uintptr(wsOverlappedWindow | wsVisible)
	}

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
	p.frameless = config.Frameless

	// Enable DWM shadow for frameless windows.
	if config.Frameless {
		type margins struct {
			cxLeftWidth, cxRightWidth, cyTopHeight, cyBottomHeight int32
		}
		m := margins{0, 0, 0, 1}
		procDwmExtendFrameIntoClient.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&m)))
		// Force WM_NCCALCSIZE to remove NC area, then update cached size
		// so first frame renders at full window size.
		procSetWindowPos.Call(uintptr(p.hwnd), 0, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
		p.updateSize()
	}

	// Show window (frameless was created hidden to avoid first-frame artifact)
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

// LogicalSize returns the window client area in DIP (device-independent pixels).
// On Windows with DPI awareness, this is the client rect divided by DPI scale.
// For most Windows apps at 100% scaling, this equals PhysicalSize.
func (p *windowsPlatform) LogicalSize() (width, height int) {
	p.sizeMu.RLock()
	defer p.sizeMu.RUnlock()

	scale := p.scaleFactor()
	if scale <= 0 || scale == 1.0 {
		return p.width, p.height
	}
	return int(float64(p.width) / scale), int(float64(p.height) / scale)
}

// PhysicalSize returns the GPU framebuffer size in device pixels.
// On Windows this is the actual client rect size (GetClientRect).
func (p *windowsPlatform) PhysicalSize() (width, height int) {
	p.sizeMu.RLock()
	defer p.sizeMu.RUnlock()
	return p.width, p.height
}

// scaleFactor returns the DPI scale factor for the window.
// Must NOT hold sizeMu (calls syscall).
func (p *windowsPlatform) scaleFactor() float64 {
	dpi, _, _ := procGetDpiForWindow.Call(uintptr(p.hwnd))
	if dpi == 0 {
		return 1.0
	}
	return float64(dpi) / 96.0
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

// SetCharCallback registers a callback for Unicode character input.
func (p *windowsPlatform) SetCharCallback(fn func(rune)) {
	p.callbackMu.Lock()
	p.charCallback = fn
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

// WaitEvents blocks until at least one OS event is available.
// MsgWaitForMultipleObjectsEx blocks the thread with zero CPU usage until
// input arrives. MWMO_INPUTAVAILABLE returns immediately if messages are
// already queued. Does NOT remove messages; PollEvents (PeekMessage) does that.
func (p *windowsPlatform) WaitEvents() {
	procMsgWaitForMultipleObjectsEx.Call(
		0,                       // nCount (no handles)
		0,                       // pHandles (nil)
		uintptr(infinite),       // dwMilliseconds
		uintptr(qsAllinput),     // dwWakeMask
		uintptr(mwmoInputAvail), // dwFlags
	)
}

// WakeUp unblocks WaitEvents from any goroutine.
// PostMessageW is thread-safe and wakes MsgWaitForMultipleObjectsEx.
func (p *windowsPlatform) WakeUp() {
	procPostMessageW.Call(uintptr(p.hwnd), uintptr(wmWakeUp), 0, 0)
}

// ScaleFactor returns the DPI scale factor for the window.
// 1.0 = 96 DPI (standard), 2.0 = 192 DPI (HiDPI).
// ScaleFactor returns the DPI scale factor.
// 1.0 = 96 DPI (standard), 1.25 = 120 DPI, 1.5 = 144 DPI, 2.0 = 192 DPI.
func (p *windowsPlatform) ScaleFactor() float64 {
	return p.scaleFactor()
}

// PrepareFrame returns current DPI state for the Windows platform.
func (p *windowsPlatform) PrepareFrame() PrepareFrameResult {
	w, h := p.PhysicalSize()
	return PrepareFrameResult{
		ScaleFactor:    p.scaleFactor(),
		PhysicalWidth:  uint32(w),
		PhysicalHeight: uint32(h),
	}
}

// ClipboardRead reads text from the system clipboard.
func (p *windowsPlatform) ClipboardRead() (string, error) {
	ret, _, _ := procOpenClipboard.Call(uintptr(p.hwnd))
	if ret == 0 {
		return "", fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil // clipboard empty or not text
	}

	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return "", fmt.Errorf("GlobalLock failed")
	}
	defer procGlobalUnlock.Call(h)

	// Read UTF-16 null-terminated string from locked global memory.
	// ptr is a valid address returned by GlobalLock; we must convert
	// uintptr -> unsafe.Pointer to read it.  go vet flags this pattern
	// but it is the standard way to work with Win32 memory APIs.
	p16 := (*uint16)(unsafe.Pointer(ptr)) //nolint:govet // syscall return value from GlobalLock
	text := windows.UTF16PtrToString(p16)
	return text, nil
}

// ClipboardWrite writes text to the system clipboard.
func (p *windowsPlatform) ClipboardWrite(text string) error {
	ret, _, _ := procOpenClipboard.Call(uintptr(p.hwnd))
	if ret == 0 {
		return fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}

	size := len(utf16) * 2 // UTF-16 = 2 bytes per char
	h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if h == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("GlobalLock failed")
	}

	// Copy UTF-16 data to locked global memory.
	// ptr is a valid address from GlobalLock; uintptr -> Pointer conversion
	// is the standard pattern for Win32 memory APIs.
	dst := unsafe.Pointer(ptr) //nolint:govet // syscall return value from GlobalLock
	for i := 0; i < len(utf16); i++ {
		*(*uint16)(unsafe.Add(dst, uintptr(i)*2)) = utf16[i]
	}

	procGlobalUnlock.Call(h)

	ret, _, _ = procSetClipboardData.Call(cfUnicodeText, h)
	if ret == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("SetClipboardData failed")
	}
	// After SetClipboardData succeeds, the system owns the handle
	return nil
}

// SetCursor changes the mouse cursor shape.
// cursorID maps to gpucontext.CursorShape values (0-11).
func (p *windowsPlatform) SetCursor(cursorID int) {
	var idc uintptr
	switch cursorID {
	case 0:
		idc = idcArrow // Default
	case 1:
		idc = idcHand // Pointer
	case 2:
		idc = idcIBeam // Text
	case 3:
		idc = idcCross // Crosshair
	case 4:
		idc = idcSizeAll // Move
	case 5:
		idc = idcSizeNS // ResizeNS
	case 6:
		idc = idcSizeWE // ResizeEW
	case 7:
		idc = idcSizeNWSE // ResizeNWSE
	case 8:
		idc = idcSizeNESW // ResizeNESW
	case 9:
		idc = idcNo // NotAllowed
	case 10:
		idc = idcWait // Wait
	case 11: // None — hide cursor
		p.cursor = 0
		procSetCursor.Call(0)
		return
	default:
		idc = idcArrow
	}
	cursor, _, _ := procLoadCursorW.Call(0, idc)
	if cursor != 0 {
		p.cursor = cursor
		procSetCursor.Call(cursor)
	}
}

// DarkMode returns true if the system dark mode is active.
// Reads AppsUseLightTheme from the Windows registry.
func (p *windowsPlatform) DarkMode() bool {
	keyPath, _ := windows.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	valueName, _ := windows.UTF16PtrFromString("AppsUseLightTheme")

	var key uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(keyPath)),
		0,
		keyRead,
		uintptr(unsafe.Pointer(&key)),
	)
	if ret != 0 {
		return false
	}
	defer procRegCloseKey.Call(key)

	var value uint32
	var valueSize uint32 = 4
	var valueType uint32
	ret, _, _ = procRegQueryValueExW.Call(
		key,
		uintptr(unsafe.Pointer(valueName)),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&valueSize)),
	)
	if ret != 0 {
		return false
	}

	return value == 0 // 0 = dark mode, 1 = light mode
}

// ReduceMotion returns true if the user prefers reduced animation.
// Checks if client area animation is disabled via SystemParametersInfo.
func (p *windowsPlatform) ReduceMotion() bool {
	var enabled uint32 // BOOL
	ret, _, _ := procSystemParametersInfoW.Call(
		spiGetClientAreaAnimation,
		0,
		uintptr(unsafe.Pointer(&enabled)),
		0,
	)
	if ret == 0 {
		return false
	}
	return enabled == 0 // animations disabled = reduce motion
}

// highContrastInfo matches the Windows HIGHCONTRAST structure layout.
type highContrastInfo struct {
	cbSize            uint32
	dwFlags           uint32
	lpszDefaultScheme *uint16
}

// HighContrast returns true if high contrast mode is active.
func (p *windowsPlatform) HighContrast() bool {
	var hc highContrastInfo
	hc.cbSize = uint32(unsafe.Sizeof(hc))
	ret, _, _ := procSystemParametersInfoW.Call(
		spiGetHighContrast,
		uintptr(hc.cbSize),
		uintptr(unsafe.Pointer(&hc)),
		0,
	)
	if ret == 0 {
		return false
	}
	return hc.dwFlags&hcfHighContrastOn != 0
}

// FontScale returns font size preference multiplier.
// On Windows, font scale is derived from the DPI scale factor.
func (p *windowsPlatform) FontScale() float32 {
	return float32(p.ScaleFactor())
}

// SetCursorMode sets cursor confinement/lock mode.
// 0=normal, 1=locked (hidden + confined + relative deltas), 2=confined (visible + confined).
func (p *windowsPlatform) SetCursorMode(mode int) {
	if mode == p.cursorMode {
		return
	}

	oldMode := p.cursorMode
	p.cursorMode = mode

	switch mode {
	case 1: // Locked
		// Save current cursor position for restoration
		var pt point
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		p.savedCursorX = pt.x
		p.savedCursorY = pt.y

		// Compute window center in screen coordinates
		p.updateCursorClipRect()

		// Clip cursor to window
		var r rect
		procGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&r)))
		var origin point
		procClientToScreen.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&origin)))
		clipRect := rect{
			left:   origin.x + r.left,
			top:    origin.y + r.top,
			right:  origin.x + r.right,
			bottom: origin.y + r.bottom,
		}
		procClipCursor.Call(uintptr(unsafe.Pointer(&clipRect)))

		// Hide cursor
		if !p.cursorHidden {
			procShowCursorW.Call(0) // FALSE = hide
			p.cursorHidden = true
		}

		// Warp to center
		procSetCursorPos.Call(uintptr(p.cursorCenterX), uintptr(p.cursorCenterY))

	case 2: // Confined
		// Clip cursor to window
		var r rect
		procGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&r)))
		var origin point
		procClientToScreen.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&origin)))
		clipRect := rect{
			left:   origin.x + r.left,
			top:    origin.y + r.top,
			right:  origin.x + r.right,
			bottom: origin.y + r.bottom,
		}
		procClipCursor.Call(uintptr(unsafe.Pointer(&clipRect)))

		// Show cursor if it was hidden
		if p.cursorHidden {
			procShowCursorW.Call(1) // TRUE = show
			p.cursorHidden = false
		}

		// Restore cursor position if coming from locked mode
		if oldMode == 1 {
			procSetCursorPos.Call(uintptr(p.savedCursorX), uintptr(p.savedCursorY))
		}

	default: // Normal (0)
		// Release clip
		procClipCursor.Call(0)

		// Show cursor if hidden
		if p.cursorHidden {
			procShowCursorW.Call(1) // TRUE = show
			p.cursorHidden = false
		}

		// Restore cursor position if coming from locked mode
		if oldMode == 1 {
			procSetCursorPos.Call(uintptr(p.savedCursorX), uintptr(p.savedCursorY))
		}
	}
}

// CursorMode returns the current cursor mode.
func (p *windowsPlatform) CursorMode() int {
	return p.cursorMode
}

// updateCursorClipRect computes the window center in screen coordinates.
// Called when entering locked mode or when the window moves/resizes while locked.
func (p *windowsPlatform) updateCursorClipRect() {
	var r rect
	procGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&r)))
	var origin point
	procClientToScreen.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&origin)))
	p.cursorCenterX = origin.x + (r.right-r.left)/2
	p.cursorCenterY = origin.y + (r.bottom-r.top)/2
}

func (p *windowsPlatform) SetFrameless(frameless bool) {
	p.callbackMu.Lock()
	p.frameless = frameless
	p.callbackMu.Unlock()

	// Style is always WS_OVERLAPPEDWINDOW (Chrome approach).
	// WM_NCCALCSIZE removes the title bar when frameless=true.
	// Toggle DWM frame extension for shadow.
	type margins struct {
		cxLeftWidth, cxRightWidth, cyTopHeight, cyBottomHeight int32
	}
	var m margins
	if frameless {
		m = margins{0, 0, 0, 1} // 1px bottom = enable DWM shadow
	}
	procDwmExtendFrameIntoClient.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&m)))

	// Force WM_NCCALCSIZE recalculation
	procSetWindowPos.Call(uintptr(p.hwnd), 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
}

func (p *windowsPlatform) IsFrameless() bool {
	p.callbackMu.RLock()
	defer p.callbackMu.RUnlock()
	return p.frameless
}

func (p *windowsPlatform) SetHitTestCallback(fn func(x, y float64) gpucontext.HitTestResult) {
	p.callbackMu.Lock()
	defer p.callbackMu.Unlock()
	p.hitTestCallback = fn
}

func (p *windowsPlatform) SyncFrame() {
	// DwmFlush synchronizes with Desktop Window Manager composition.
	// During resize, this ensures our rendered frame and the DWM window
	// border update appear in the same composition cycle, reducing lag.
	p.sizeMu.RLock()
	resizing := p.inSizeMove
	p.sizeMu.RUnlock()
	if resizing {
		procDwmFlush.Call()
	}
}

func (p *windowsPlatform) Minimize() {
	procShowWindow.Call(uintptr(p.hwnd), swMinimize)
}

func (p *windowsPlatform) Maximize() {
	if p.IsMaximized() {
		procShowWindow.Call(uintptr(p.hwnd), swRestore)
	} else {
		procShowWindow.Call(uintptr(p.hwnd), swMaximize)
	}
}

func (p *windowsPlatform) IsMaximized() bool {
	ret, _, _ := procIsZoomed.Call(uintptr(p.hwnd))
	return ret != 0
}

func (p *windowsPlatform) CloseWindow() {
	procPostMessageW.Call(uintptr(p.hwnd), wmClose, 0, 0)
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

	// Convert physical pixels → logical (DIP) coordinates.
	// With DPI awareness, WM_MOUSEMOVE reports physical pixels, but UI layout
	// uses logical coordinates (App.Size() returns LogicalSize).
	scale := p.scaleFactor()
	if scale > 1.0 {
		x /= scale
		y /= scale
	}

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

// mouseCapture calls SetCapture on the first button press to track mouse
// movement outside the window boundary during drag operations (sliders,
// scrollbars, text selection). Must be called BEFORE dispatching the event
// so the button state in p.buttons reflects the pre-press state.
func (p *windowsPlatform) mouseCapture(wParam uintptr) {
	p.mouseMu.Lock()
	wasPressedBefore := p.buttons != gpucontext.ButtonsNone
	p.buttons = extractButtons(wParam)
	p.mouseMu.Unlock()

	if !wasPressedBefore {
		procSetCapture.Call(uintptr(p.hwnd))
	}
}

// mouseRelease calls ReleaseCapture when the last button is released.
// Must be called AFTER dispatching the event so the PointerUp event
// still has correct button state.
func (p *windowsPlatform) mouseRelease(wParam uintptr) {
	newButtons := extractButtons(wParam)

	p.mouseMu.Lock()
	p.buttons = newButtons
	p.mouseMu.Unlock()

	if newButtons == gpucontext.ButtonsNone {
		procReleaseCapture.Call()
	}
}

// getPointerID extracts pointer ID from wParam (LOWORD).
func getPointerID(wParam uintptr) uint32 {
	return uint32(wParam & 0xFFFF)
}

// hitTestResultToWin32 converts gpucontext.HitTestResult to Win32 NCHITTEST values.
func hitTestResultToWin32(result gpucontext.HitTestResult) uintptr {
	switch result {
	case gpucontext.HitTestCaption:
		return htCaption
	case gpucontext.HitTestClose:
		return htClose
	case gpucontext.HitTestMaximize:
		return htMaxButton
	case gpucontext.HitTestMinimize:
		return htMinButton
	case gpucontext.HitTestResizeN:
		return htTop
	case gpucontext.HitTestResizeS:
		return htBottom
	case gpucontext.HitTestResizeW:
		return htLeft
	case gpucontext.HitTestResizeE:
		return htRight
	case gpucontext.HitTestResizeNW:
		return htTopLeft
	case gpucontext.HitTestResizeNE:
		return htTopRight
	case gpucontext.HitTestResizeSW:
		return htBottomLeft
	case gpucontext.HitTestResizeSE:
		return htBottomRight
	default:
		return htClient
	}
}

// mapWin32PointerType maps Win32 PT_* constants to gpucontext.PointerType.
func mapWin32PointerType(ptType uint32) gpucontext.PointerType {
	switch ptType {
	case ptTouch:
		return gpucontext.PointerTypeTouch
	case ptPen:
		return gpucontext.PointerTypePen
	default:
		return gpucontext.PointerTypeMouse
	}
}

// buttonsFromPointerFlags extracts button state from POINTER_INFO.pointerFlags.
func buttonsFromPointerFlags(flags uint32) gpucontext.Buttons {
	var btns gpucontext.Buttons
	if flags&pointerFlagFirstButton != 0 {
		btns |= gpucontext.ButtonsLeft
	}
	if flags&pointerFlagSecondButton != 0 {
		btns |= gpucontext.ButtonsRight
	}
	if flags&pointerFlagThirdButton != 0 {
		btns |= gpucontext.ButtonsMiddle
	}
	return btns
}

// buttonFromEventType determines which single button changed for down/up events.
func buttonFromEventType(eventType gpucontext.PointerEventType, flags uint32) gpucontext.Button {
	if eventType == gpucontext.PointerDown || eventType == gpucontext.PointerUp {
		if flags&pointerFlagFirstButton != 0 {
			return gpucontext.ButtonLeft
		}
		if flags&pointerFlagSecondButton != 0 {
			return gpucontext.ButtonRight
		}
		if flags&pointerFlagThirdButton != 0 {
			return gpucontext.ButtonMiddle
		}
	}
	return gpucontext.ButtonNone
}

// createPointerEventFromWMPointer creates a PointerEvent from WM_POINTER* message data.
// It calls GetPointerInfo to retrieve pointer details, and for pen input also
// calls GetPointerPenInfo to get pressure and tilt data.
func (p *windowsPlatform) createPointerEventFromWMPointer(
	eventType gpucontext.PointerEventType,
	wParam, lParam uintptr,
) gpucontext.PointerEvent {
	pointerID := getPointerID(wParam)

	// Get pointer info
	var info pointerInfo
	ret, _, _ := procGetPointerInfo.Call(uintptr(pointerID), uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		// Fallback: use lParam coordinates
		x, y := extractMousePos(lParam)
		return gpucontext.PointerEvent{
			Type:        eventType,
			PointerID:   int(pointerID),
			X:           x,
			Y:           y,
			Width:       1,
			Height:      1,
			PointerType: gpucontext.PointerTypeMouse,
			IsPrimary:   true,
			Timestamp:   p.eventTimestamp(),
		}
	}

	// Convert screen coordinates to client coordinates (physical pixels),
	// then to logical (DIP) coordinates for UI layout.
	var origin point
	procClientToScreen.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&origin)))
	x := float64(info.ptPixelLocation.x - origin.x)
	y := float64(info.ptPixelLocation.y - origin.y)
	if scale := p.scaleFactor(); scale > 1.0 {
		x /= scale
		y /= scale
	}

	pointerType := mapWin32PointerType(info.pointerType)
	isPrimary := info.pointerFlags&pointerFlagPrimary != 0
	buttons := buttonsFromPointerFlags(info.pointerFlags)
	button := buttonFromEventType(eventType, info.pointerFlags)
	modifiers := getKeyModifiers()

	// Default pressure
	var pressure float32
	if info.pointerFlags&pointerFlagInContact != 0 {
		pressure = 0.5
	}

	var tiltX, tiltY float32
	var width, height float32 = 1, 1

	// For pen input, get detailed pen info
	if pointerType == gpucontext.PointerTypePen {
		var penInfo pointerPenInfo
		ret, _, _ = procGetPointerPenInfo.Call(uintptr(pointerID), uintptr(unsafe.Pointer(&penInfo)))
		if ret != 0 {
			// Pressure: 0-1024 → 0.0-1.0
			pressure = float32(penInfo.pressure) / 1024.0
			tiltX = float32(penInfo.tiltX)
			tiltY = float32(penInfo.tiltY)
		}
	}

	// For touch input, pressure is 0.5 when in contact (already set above)
	// Width/Height could come from contact rect, but GetPointerTouchInfo
	// would be needed — keep defaults for now

	return gpucontext.PointerEvent{
		Type:        eventType,
		PointerID:   int(pointerID),
		X:           x,
		Y:           y,
		Pressure:    pressure,
		TiltX:       tiltX,
		TiltY:       tiltY,
		Width:       width,
		Height:      height,
		PointerType: pointerType,
		IsPrimary:   isPrimary,
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

	case wmNCUAHDrawCaption, wmNCUAHDrawFrame:
		// Block undocumented UxTheme caption/frame drawing messages.
		// These cause border artifacts on frameless windows.
		// Source: rossy/borderless-window, wangwenx190/framelesshelper
		p.callbackMu.RLock()
		frameless := p.frameless
		p.callbackMu.RUnlock()
		if frameless {
			return 0
		}

	case wmNCActivate:
		// THE KEY FIX: Prevent non-client area repaint on focus change.
		// DefWindowProc with lParam=-1 processes activation state change
		// but SKIPS repainting the non-client area. This eliminates the
		// visible border flash when the window gains/loses focus.
		// Source: Chromium, Electron, rossy/borderless-window, FramelessHelper
		p.callbackMu.RLock()
		frameless := p.frameless
		p.callbackMu.RUnlock()
		if frameless {
			// Invalidate client area to force GPU redraw over any NC artifacts.
			procInvalidateRect.Call(uintptr(p.hwnd), 0, 0)
			return 1
		}

	case wmNCPaint:
		// Let DefWindowProc handle WM_NCPAINT — DWM draws shadow + borders.
		// Our GPU renderer covers the borders. JBR approach.

	case wmNCCalcSize:
		// JBR approach: remove ONLY the title bar (top NC area).
		// Keep left/right/bottom NC borders so DWM shadow works.
		// GPU renderer draws over the thin NC borders.
		p.callbackMu.RLock()
		frameless := p.frameless
		p.callbackMu.RUnlock()

		if frameless && wParam != 0 {
			// Save original top before DefWindowProc adjusts it
			rgrc := (*rect)(unsafe.Pointer(lParam)) //nolint:govet // lParam is NCCALCSIZE_PARAMS*
			frameTop := rgrc.top

			// Let Windows calculate NC area (borders, title bar)
			procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)

			// Restore top — removes title bar but keeps side/bottom borders
			rgrc.top = frameTop

			if ret, _, _ := procIsZoomed.Call(uintptr(p.hwnd)); ret != 0 {
				// When maximized, add frame border to top to prevent
				// window from extending above the screen
				borderY, _, _ := procGetSystemMetrics.Call(smCYSizeFrame)
				padded, _, _ := procGetSystemMetrics.Call(smCXPaddedBorder)
				rgrc.top += int32(borderY + padded)
			}
			return 0
		}

	case wmNCHitTest:
		// Custom hit testing for frameless windows
		p.callbackMu.RLock()
		cb := p.hitTestCallback
		frameless := p.frameless
		p.callbackMu.RUnlock()

		if frameless && cb != nil {
			// Get cursor position in screen coordinates from lParam
			screenX := int16(lParam & 0xFFFF)
			screenY := int16((lParam >> 16) & 0xFFFF)

			// Convert screen to client coordinates
			pt := point{x: int32(screenX), y: int32(screenY)}
			procScreenToClient.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&pt)))

			// Convert to logical (DIP) coordinates
			scale := p.scaleFactor()
			logX := float64(pt.x)
			logY := float64(pt.y)
			if scale > 1.0 {
				logX /= scale
				logY /= scale
			}

			result := cb(logX, logY)
			return hitTestResultToWin32(result)
		}

	case wmActivate:
		// Release cursor grab on focus loss, re-grab on focus gain.
		activationState := wParam & 0xFFFF
		if activationState == waInactive {
			// Window lost focus — temporarily release cursor constraints
			if p.cursorMode != 0 {
				procClipCursor.Call(0)
				if p.cursorHidden {
					procShowCursorW.Call(1)
					p.cursorHidden = false
				}
			}
		} else {
			// Window gained focus — re-apply cursor mode
			if p.cursorMode != 0 {
				p.SetCursorMode(p.cursorMode)
			}
		}

	case wmWakeUp:
		// No-op: sole purpose is to unblock MsgWaitForMultipleObjectsEx in WaitEvents.
		return 0

	case wmDpiChanged:
		// Window moved to a monitor with different DPI.
		// lParam points to a RECT with the suggested new position/size.
		suggestedRect := (*rect)(unsafe.Pointer(lParam)) //nolint:govet // lParam is RECT*
		procSetWindowPos.Call(uintptr(p.hwnd), 0,
			uintptr(suggestedRect.left),
			uintptr(suggestedRect.top),
			uintptr(suggestedRect.right-suggestedRect.left),
			uintptr(suggestedRect.bottom-suggestedRect.top),
			swpNoZOrder|swpNoActivate)

		// Update cached client size after DPI-driven resize.
		p.updateSize()

		// Queue resize event with new DPI-adjusted dimensions.
		physW, physH := p.PhysicalSize()
		logW, logH := p.LogicalSize()
		p.queueEvent(Event{
			Type:           EventResize,
			Width:          logW,
			Height:         logH,
			PhysicalWidth:  physW,
			PhysicalHeight: physH,
		})
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
			// On Windows, WM_SIZE provides physical pixels (client rect).
			// Compute logical size from DPI scale for the event.
			logW, logH := newWidth, newHeight
			dpi, _, _ := procGetDpiForWindow.Call(uintptr(p.hwnd))
			if dpi > 0 && dpi != 96 {
				scale := float64(dpi) / 96.0
				logW = int(float64(newWidth) / scale)
				logH = int(float64(newHeight) / scale)
			}
			p.queueEvent(Event{
				Type:           EventResize,
				Width:          logW,
				Height:         logH,
				PhysicalWidth:  newWidth,
				PhysicalHeight: newHeight,
			})

			// Update cursor clip rect if locked or confined
			if p.cursorMode != 0 {
				p.SetCursorMode(p.cursorMode)
			}
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
		physW, physH := p.PhysicalSize()
		logW, logH := p.LogicalSize()
		p.queueEvent(Event{
			Type:           EventResize,
			Width:          logW,
			Height:         logH,
			PhysicalWidth:  physW,
			PhysicalHeight: physH,
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

	case wmChar, wmSysChar:
		// WM_CHAR/WM_SYSCHAR are generated by TranslateMessage().
		// wParam is a UTF-16 code unit — supplementary characters (emoji, CJK)
		// arrive as two consecutive messages: high surrogate then low surrogate.
		// Pattern: GLFW 3.4 win32_window.c + Ebiten textinput_windows.go
		w := uint16(wParam)
		if w >= 0xD800 && w <= 0xDBFF {
			// High surrogate — store and wait for low surrogate
			p.highSurrogate = w
			return 0
		}
		var char rune
		if w >= 0xDC00 && w <= 0xDFFF {
			// Low surrogate — combine with stored high surrogate
			if p.highSurrogate != 0 {
				char = (rune(p.highSurrogate)-0xD800)<<10 + (rune(w) - 0xDC00) + 0x10000
			}
		} else {
			char = rune(w)
		}
		p.highSurrogate = 0
		// Filter control characters (Ctrl+A..Z = 0x01..0x1A, DEL = 0x7F)
		if char >= 32 && char != 127 {
			p.callbackMu.RLock()
			callback := p.charCallback
			p.callbackMu.RUnlock()
			if callback != nil {
				callback(char)
			}
		}
		return 0

	case wmUnichar:
		// WM_UNICHAR from third-party IMEs — wParam is a full Unicode codepoint.
		if wParam == unicodeNochar {
			return 1 // "Yes, we support WM_UNICHAR"
		}
		char := rune(wParam)
		if char >= 32 && char != 127 {
			p.callbackMu.RLock()
			callback := p.charCallback
			p.callbackMu.RUnlock()
			if callback != nil {
				callback(char)
			}
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

	// Pointer input (touch/pen via WM_POINTER*, Windows 8+)
	// WM_POINTER* fires for touch and pen input by default.
	// Mouse continues via WM_MOUSE* messages (no EnableMouseInPointer).
	case wmPointerDown:
		ev := p.createPointerEventFromWMPointer(gpucontext.PointerDown, wParam, lParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmPointerUp:
		ev := p.createPointerEventFromWMPointer(gpucontext.PointerUp, wParam, lParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmPointerUpdate:
		ev := p.createPointerEventFromWMPointer(gpucontext.PointerMove, wParam, lParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmPointerEnter:
		ev := p.createPointerEventFromWMPointer(gpucontext.PointerEnter, wParam, lParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmPointerLeave:
		ev := p.createPointerEventFromWMPointer(gpucontext.PointerLeave, wParam, lParam)
		p.dispatchPointerEvent(ev)
		return 0

	// Mouse movement
	case wmMouseMove:
		x, y := extractMousePos(lParam)

		// In locked mode, compute delta from center and warp back
		if p.cursorMode == 1 {
			// Convert client coords to screen coords to compare with center
			var screenPt point
			screenPt.x = int32(x)
			screenPt.y = int32(y)
			procClientToScreen.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&screenPt)))

			deltaX := float64(screenPt.x - p.cursorCenterX)
			deltaY := float64(screenPt.y - p.cursorCenterY)

			// Skip the warp-back event (delta=0 means cursor is at center)
			if deltaX == 0 && deltaY == 0 {
				return 0
			}

			// Warp cursor back to center
			procSetCursorPos.Call(uintptr(p.cursorCenterX), uintptr(p.cursorCenterY))

			// Update mouse state
			p.mouseMu.Lock()
			p.buttons = extractButtons(wParam)
			p.modifiers = extractModifiers(wParam)
			p.mouseInWindow = true
			p.mouseMu.Unlock()

			// Emit move event with relative deltas
			ev := p.createPointerEvent(gpucontext.PointerMove, gpucontext.ButtonNone, x, y, wParam)
			ev.DeltaX = deltaX
			ev.DeltaY = deltaY
			p.dispatchPointerEvent(ev)
			return 0
		}

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
		p.mouseCapture(wParam)
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonLeft, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmLButtonUp:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonLeft, x, y, wParam)
		p.dispatchPointerEvent(ev)
		p.mouseRelease(wParam)
		return 0

	// Right button
	case wmRButtonDown:
		p.mouseCapture(wParam)
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonRight, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmRButtonUp:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonRight, x, y, wParam)
		p.dispatchPointerEvent(ev)
		p.mouseRelease(wParam)
		return 0

	// Middle button
	case wmMButtonDown:
		p.mouseCapture(wParam)
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerDown, gpucontext.ButtonMiddle, x, y, wParam)
		p.dispatchPointerEvent(ev)
		return 0

	case wmMButtonUp:
		x, y := extractMousePos(lParam)
		ev := p.createPointerEvent(gpucontext.PointerUp, gpucontext.ButtonMiddle, x, y, wParam)
		p.dispatchPointerEvent(ev)
		p.mouseRelease(wParam)
		return 0

	// X buttons (back/forward)
	case wmXButtonDown:
		p.mouseCapture(wParam)
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
		p.mouseRelease(wParam)
		return 1 // Must return TRUE for XBUTTON messages

	// Vertical scroll wheel
	case wmMouseWheel:
		// For wheel messages, coordinates are screen-relative
		// Convert to client coordinates using ScreenToClient
		screenX, screenY := extractMousePos(lParam)
		pt := point{x: int32(screenX), y: int32(screenY)}
		procScreenToClient.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&pt)))
		x, y := float64(pt.x), float64(pt.y)
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
		// For wheel messages, coordinates are screen-relative
		// Convert to client coordinates using ScreenToClient
		screenX, screenY := extractMousePos(lParam)
		pt := point{x: int32(screenX), y: int32(screenY)}
		procScreenToClient.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&pt)))
		x, y := float64(pt.x), float64(pt.y)
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

// BlitPixels copies RGBA pixel data to the window using GDI SetDIBitsToDevice.
// Implements the PixelBlitter interface for software backend presentation.
func (p *windowsPlatform) BlitPixels(pixels []byte, width, height int) error {
	hdc, _, _ := procGetDC.Call(uintptr(p.hwnd))
	if hdc == 0 {
		return fmt.Errorf("gogpu: GetDC failed")
	}
	defer procReleaseDC.Call(uintptr(p.hwnd), hdc)

	bmi := bitmapInfoHeader{
		biSize:     40,
		biWidth:    int32(width),
		biHeight:   -int32(height), // negative = top-down
		biPlanes:   1,
		biBitCount: 32,
	}

	// Software backend stores RGBA, Windows DIB expects BGRA -- swap R<->B
	bgra := make([]byte, len(pixels))
	for i := 0; i < len(pixels)-3; i += 4 {
		bgra[i+0] = pixels[i+2] // B
		bgra[i+1] = pixels[i+1] // G
		bgra[i+2] = pixels[i+0] // R
		bgra[i+3] = pixels[i+3] // A
	}

	procSetDIBitsToDevice.Call(
		hdc,
		0, 0, // dest x, y
		uintptr(width), uintptr(height),
		0, 0, // src x, y
		0, uintptr(height), // start scan, num scans
		uintptr(unsafe.Pointer(&bgra[0])),
		uintptr(unsafe.Pointer(&bmi)),
		0, // DIB_RGB_COLORS
	)

	return nil
}
