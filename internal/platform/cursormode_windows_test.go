//go:build windows

package platform

import (
	"fmt"
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

// clientArea returns the window's client area in screen coordinates.
func clientArea(window *win32Window) rect {
	var client rect
	procGetClientRect.Call(uintptr(window.hwnd), uintptr(unsafe.Pointer(&client)))
	var origin point
	procClientToScreen.Call(uintptr(window.hwnd), uintptr(unsafe.Pointer(&origin)))
	return rect{left: origin.x, top: origin.y, right: origin.x + client.right, bottom: origin.y + client.bottom}
}

// windowResizes are the ways a window's client area changes size: a size
// request and a fullscreen toggle each resize it at once, while a resize
// inside the modal size loop takes effect when the loop ends.
var windowResizes = []struct {
	name   string
	resize func(window *win32Window)
}{
	{"size request", func(window *win32Window) { window.RequestSize(400, 300) }},
	{"fullscreen toggle", func(window *win32Window) { window.SetFullscreen(true) }},
	{"modal size loop", func(window *win32Window) {
		wndProc(window.hwnd, wmEnterSizeMove, 0, 0)
		window.RequestSize(400, 300)
		wndProc(window.hwnd, wmExitSizeMove, 0, 0)
	}},
}

func TestAResizeClipsTheCursorToTheNewClientArea(t *testing.T) {
	for _, mode := range []int{1, 2} {
		for _, resize := range windowResizes {
			t.Run(fmt.Sprintf("mode %d, %s", mode, resize.name), func(t *testing.T) {
				window := newCursorModeTestWindow(t)
				window.SetCursorMode(mode)
				wndProc(window.hwnd, wmActivate, waActive, 0)
				before := clientArea(window)

				resize.resize(window)

				after := clientArea(window)
				if after == before {
					t.Fatalf("the client area stayed %+v", after)
				}
				if clip := cursorClip(); clip != after {
					t.Errorf("the cursor is clipped to %+v, want the new client area %+v", clip, after)
				}
			})
		}
	}
}

func TestALockedWindowMeasuresItsDeltasFromTheNewCenterAfterAResize(t *testing.T) {
	for _, resize := range windowResizes {
		t.Run(resize.name, func(t *testing.T) {
			window := newCursorModeTestWindow(t)
			window.SetCursorMode(1)
			wndProc(window.hwnd, wmActivate, waActive, 0)
			resize.resize(window)
			for window.platform.events.Len() > 0 {
				window.platform.dequeueEvent()
			}

			width, height := window.PhysicalSize()
			x := width/2 + 3
			y := height/2 + 4
			wndProc(window.hwnd, wmMouseMove, 0, uintptr(x|y<<16))

			event := window.platform.dequeueEvent()
			if event.Type != EventPointerMove || event.Pointer.DeltaX != 3 || event.Pointer.DeltaY != 4 {
				t.Errorf("a move 3, 4 from the new center reported %+v, want a move by 3, 4", event)
			}
		})
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
