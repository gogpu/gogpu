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

// newCursorModeTestPlatform initializes a platform of its own on the test's
// thread, and tears it down when the test ends, its window class included,
// so a later test can register the class again.
func newCursorModeTestPlatform(t *testing.T) *windowsPlatform {
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
	return p
}

// newCursorModeTestWindow opens a hidden window, which is not active, on a
// platform of its own. When the test ends it releases the window's cursor
// mode and tears the platform down.
func newCursorModeTestWindow(t *testing.T) *win32Window {
	t.Helper()
	window := newCursorModeTestWindowOn(t, newCursorModeTestPlatform(t))
	t.Cleanup(func() { window.SetCursorMode(0) })
	return window
}

// newCursorModeTestWindowOn opens a hidden window, which is not active, on
// platform.
func newCursorModeTestWindowOn(t *testing.T, platform *windowsPlatform) *win32Window {
	t.Helper()
	created, err := platform.CreateWindow(Config{Title: "cursor mode", Width: 320, Height: 240})
	if err != nil {
		t.Fatalf("CreateWindow = %v", err)
	}
	return created.(*win32Window)
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

// cursorDisplayCount returns the thread's cursor display count, which shows
// the cursor while it is zero or more.
func cursorDisplayCount() int32 {
	count, _, _ := procShowCursorW.Call(1)
	procShowCursorW.Call(0)
	return int32(count) - 1
}

// windowDestructions are the ways a window is destroyed: on its own, or with
// the rest of its platform's windows when the platform is torn down.
var windowDestructions = []struct {
	name    string
	destroy func(window *win32Window)
}{
	{"window", func(window *win32Window) { window.Destroy() }},
	{"platform", func(window *win32Window) { window.platform.Destroy() }},
}

func TestDestroyingAnActiveWindowReleasesItsCursorMode(t *testing.T) {
	for _, mode := range []int{1, 2} {
		for _, destruction := range windowDestructions {
			t.Run(fmt.Sprintf("mode %d, %s", mode, destruction.name), func(t *testing.T) {
				// The clip is the whole system's: free it even when the test
				// fails, so the cursor is not left clipped after the run.
				t.Cleanup(func() { procClipCursor.Call(0) })
				platform := newCursorModeTestPlatform(t)
				// Read once Init has made the process DPI aware, in the
				// physical pixels the clip is read in after.
				unclipped := cursorClip()
				shownDisplayCount := cursorDisplayCount()
				window := newCursorModeTestWindowOn(t, platform)
				window.SetCursorMode(mode)
				wndProc(window.hwnd, wmActivate, waActive, 0)
				if cursorClip() == unclipped {
					t.Fatalf("the active window's cursor is not clipped")
				}

				destruction.destroy(window)

				if clip := cursorClip(); clip != unclipped {
					t.Errorf("the cursor is clipped to %+v after the window is destroyed, want %+v", clip, unclipped)
				}
				if displayCount := cursorDisplayCount(); displayCount != shownDisplayCount {
					t.Errorf("the cursor display count is %d after the window is destroyed, want %d", displayCount, shownDisplayCount)
				}
			})
		}
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
