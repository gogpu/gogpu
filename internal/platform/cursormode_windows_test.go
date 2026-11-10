//go:build windows

package platform

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetClipCursor = user32.NewProc("GetClipCursor")

// newCursorModeTestWindow opens a hidden window, which is not active, on a
// platform of its own. When the test ends it releases the window's cursor
// mode and tears the platform down, its window class included, so a later
// test can register the class again.
func newCursorModeTestWindow(t *testing.T) *win32Window {
	t.Helper()
	runtime.LockOSThread()
	p := newPlatformManager().(*windowsPlatform)
	if err := p.Init(); err != nil {
		t.Fatalf("Init = %v", err)
	}
	t.Cleanup(func() {
		p.Destroy()
		name, err := windows.UTF16PtrFromString("GoGPUWindow")
		if err != nil {
			t.Fatal(err)
		}
		user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), uintptr(p.hinstance))
		windows.NewLazySystemDLL("ole32.dll").NewProc("OleUninitialize").Call()
		runtime.UnlockOSThread()
	})
	created, err := p.CreateWindow(Config{Title: "cursor mode", Width: 320, Height: 240})
	if err != nil {
		t.Fatalf("CreateWindow = %v", err)
	}
	window := created.(*win32Window)
	t.Cleanup(func() { window.SetCursorMode(0) })
	return window
}

// cursorClip returns the rectangle the cursor is confined to, the screen's
// while it is not.
func cursorClip() rect {
	var clip rect
	procGetClipCursor.Call(uintptr(unsafe.Pointer(&clip)))
	return clip
}

func TestCursorModeIsNormalWhileTheWindowIsInactive(t *testing.T) {
	window := newCursorModeTestWindow(t)
	unclipped := cursorClip()

	window.SetCursorMode(1)
	if mode := window.CursorMode(); mode != 0 || cursorClip() != unclipped {
		t.Errorf("a locked window not yet active answers %d, clip %+v, want Normal and the cursor free", mode, cursorClip())
	}
	wndProc(window.hwnd, wmActivate, waActive, 0)
	if mode := window.CursorMode(); mode != 1 || cursorClip() == unclipped {
		t.Errorf("a locked window made active answers %d, clip %+v, want Locked and the cursor clipped", mode, cursorClip())
	}
	wndProc(window.hwnd, wmActivate, waInactive, 0)
	if mode := window.CursorMode(); mode != 0 || cursorClip() != unclipped {
		t.Errorf("a locked window made inactive answers %d, clip %+v, want Normal and the cursor free", mode, cursorClip())
	}
	wndProc(window.hwnd, wmActivate, waActive, 0)
	if mode := window.CursorMode(); mode != 1 || cursorClip() == unclipped {
		t.Errorf("a locked window made active again answers %d, clip %+v, want Locked and the cursor clipped", mode, cursorClip())
	}
}

func TestMoveOverAnInactiveLockedWindowLeavesTheCursor(t *testing.T) {
	window := newCursorModeTestWindow(t)
	window.SetCursorMode(1)
	var before, after point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&before)))

	wndProc(window.hwnd, wmMouseMove, 0, 20|30<<16)

	procGetCursorPos.Call(uintptr(unsafe.Pointer(&after)))
	if after != before {
		t.Errorf("a move over an inactive locked window moved the cursor from %+v to %+v", before, after)
	}
}
