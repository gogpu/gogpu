//go:build windows

package platform

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 menu message and flag constants.
const (
	wmCommand     = 0x0111 // WM_COMMAND: HIWORD(wParam)==0 means menu, 1 means accelerator
	wmRebuildMenu = 0x8001 // WM_APP+1: posted by SetApplicationMenu to trigger rebuild on OS thread

	mfSeparator = 0x0800 // MF_SEPARATOR
	mfPopup     = 0x0010 // MF_POPUP: uIDNewItem is a submenu HMENU
	mfGrayed    = 0x0001 // MF_GRAYED: disabled and grayed out
)

// Menu proc declarations; user32 DLL is already loaded in platform_windows.go.
var (
	procCreateMenu      = user32.NewProc("CreateMenu")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenuW     = user32.NewProc("AppendMenuW")
	procSetMenu         = user32.NewProc("SetMenu")
	procDrawMenuBar     = user32.NewProc("DrawMenuBar")
	procEnableMenuItem  = user32.NewProc("EnableMenuItem") // reserved for Phase 4 WM_INITMENUPOPUP
	procDestroyMenu     = user32.NewProc("DestroyMenu")
)

// menuCmdIDCounter allocates uint16 command IDs for menu items.
// Starts at 0x1000 (4096); range 0x0001–0x0FFF is reserved for systray IDs
// to prevent collision when both run in the same process.
var menuCmdIDCounter atomic.Uint32

func init() {
	menuCmdIDCounter.Store(0x1000)
}

// nextMenuCmdID allocates and returns the next command ID for a menu item.
// Thread-safe via atomic increment. IDs begin at 0x1000 to avoid colliding
// with the systray range 0x0001–0x0FFF when both subsystems run together.
func nextMenuCmdID() uint16 {
	// Add returns the post-increment value; subtract 1 to get the allocated slot.
	return uint16(menuCmdIDCounter.Add(1) - 1)
}

// menuActions maps command ID → action callback for WM_COMMAND dispatch.
var menuActions sync.Map // map[uint16]func()

// SetApplicationMenu implements PlatMenuManager.
// Stores items under menuMu and posts wmRebuildMenu so wndProc can rebuild
// the HMENU on the OS message thread (all Win32 menu calls must be on the
// thread that owns the window).
func (p *windowsPlatform) SetApplicationMenu(items []MenuItem) {
	p.menuMu.Lock()
	p.pendingMenu = items
	p.menuMu.Unlock()
	if p.primary != nil && p.primary.hwnd != 0 {
		procPostMessageW.Call(uintptr(p.primary.hwnd), wmRebuildMenu, 0, 0)
	}
}

// AddToSystemMenu implements PlatMenuManager.
// Returns false: Windows has no Apple-style application menu. The Win32 system
// menu (Alt+Space) is fundamentally different — items there would confuse users.
// Apps that want an application-level menu on Windows should use
// SetApplicationMenu with an explicit top-level entry.
func (p *windowsPlatform) AddToSystemMenu(_ SystemMenu, _ []MenuItem) bool {
	return false
}

// applyMenu rebuilds the Win32 HMENU from p.pendingMenu.
// Must be called on the OS message thread — from the wndProc wmRebuildMenu
// handler, or directly after CreateWindow before the message loop starts.
func (p *windowsPlatform) applyMenu() {
	p.menuMu.Lock()
	items := p.pendingMenu
	p.menuMu.Unlock()

	// Destroy the previous HMENU and clear all registered actions.
	if p.hMenu != 0 {
		procDestroyMenu.Call(uintptr(p.hMenu))
		p.hMenu = 0
	}
	menuActions.Range(func(k, _ any) bool {
		menuActions.Delete(k)
		return true
	})

	hwnd := uintptr(0)
	if p.primary != nil {
		hwnd = uintptr(p.primary.hwnd)
	}
	if hwnd == 0 {
		return
	}

	if len(items) == 0 {
		procSetMenu.Call(hwnd, 0)
		procDrawMenuBar.Call(hwnd)
		return
	}

	hMenuBar, _, _ := procCreateMenu.Call()
	if hMenuBar == 0 {
		return
	}

	for _, item := range items {
		appendMenuItem(hMenuBar, item)
	}

	p.hMenu = windows.Handle(hMenuBar)
	procSetMenu.Call(hwnd, hMenuBar)
	procDrawMenuBar.Call(hwnd)
}

// buildMenuPopup creates a Win32 popup HMENU from items.
// Submenus are created recursively. Returns 0 only if CreatePopupMenu fails,
// which cannot happen under normal conditions.
func buildMenuPopup(items []MenuItem) uintptr {
	hPopup, _, _ := procCreatePopupMenu.Call()
	if hPopup == 0 {
		return 0
	}
	for _, item := range items {
		appendMenuItem(hPopup, item)
	}
	return hPopup
}

// appendMenuItem adds one MenuItem to the given HMENU handle.
// Handles separators, nested submenus (MF_POPUP), and leaf items (MF_STRING).
// Disabled items receive MF_GRAYED. Actions and role-based callbacks are
// registered in menuActions for later WM_COMMAND dispatch.
func appendMenuItem(hMenu uintptr, item MenuItem) {
	if item.Separator {
		procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
		return
	}

	if len(item.Submenu) > 0 {
		subPopup := buildMenuPopup(item.Submenu)
		title, _ := windows.UTF16PtrFromString(item.Title)
		flags := uintptr(mfPopup)
		if item.Disabled {
			flags |= mfGrayed
		}
		procAppendMenuW.Call(hMenu, flags, subPopup, uintptr(unsafe.Pointer(title)))
		return
	}

	id := nextMenuCmdID()
	flags := uintptr(0) // MF_STRING = 0
	if item.Disabled {
		flags |= mfGrayed
	}

	if item.Role == MenuRoleQuit {
		action := item.Action
		menuActions.Store(id, func() {
			if action != nil {
				action()
			}
			// Use PostMessageW(WM_CLOSE) — same path as win32Window.Close() at
			// platform_windows.go:782 — to trigger the full ADR-026 lifecycle:
			// wmClose → shouldClose=true → EventClose → cleanup → DestroyWindow → PostQuitMessage.
			if globalPlatform != nil && globalPlatform.primary != nil {
				procPostMessageW.Call(uintptr(globalPlatform.primary.hwnd), wmClose, 0, 0)
			}
		})
	} else if item.Action != nil {
		menuActions.Store(id, item.Action)
	}

	title, _ := windows.UTF16PtrFromString(item.Title)
	procAppendMenuW.Call(hMenu, flags, uintptr(id), uintptr(unsafe.Pointer(title)))
}

// dispatchMenuCommand looks up the registered action for the given menu command
// ID and calls it. Called from wndProc when HIWORD(wParam) == 0 (menu source).
// No-ops silently for unknown IDs (accelerator pass-through, etc.).
func dispatchMenuCommand(id uint16) {
	if fn, ok := menuActions.Load(id); ok {
		fn.(func())()
	}
}

// Compile-time check: windowsPlatform must satisfy PlatMenuManager.
var _ PlatMenuManager = (*windowsPlatform)(nil)
