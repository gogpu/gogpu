//go:build windows

package platform

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsFullscreenStartup(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	p := newPlatformManager().(*windowsPlatform)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		p.Destroy()
		name, err := windows.UTF16PtrFromString("GoGPUWindow")
		if err != nil {
			t.Fatal(err)
		}
		user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), uintptr(p.hinstance))
		windows.NewLazySystemDLL("ole32.dll").NewProc("OleUninitialize").Call()
	}()

	// Exercise both the primary and secondary window creation paths.
	for _, name := range []string{"primary", "secondary"} {
		t.Log(name)
		created, err := p.CreateWindow(Config{Title: "Fullscreen startup test", Width: 320, Height: 240, Fullscreen: true})
		if err != nil {
			t.Fatal(err)
		}
		w := created.(*win32Window)
		if !w.IsFullscreen() {
			t.Fatal("fullscreen configuration was ignored during creation")
		}
		assertWindowVisible(t, w, false)
		assertFullscreenBounds(t, w)
		style, _, _ := procGetWindowLongPtrW.Call(uintptr(w.hwnd), gwlStyle)
		if style&wsOverlappedWindow != 0 {
			t.Errorf("fullscreen window retains decorations: style=%#x", style)
		}

		w.Show()
		assertWindowVisible(t, w, true)
		w.SetFullscreen(false)
		assertWindowVisible(t, w, true)
		if width, height := w.LogicalSize(); width != 320 || height != 240 {
			t.Errorf("restored client size = %dx%d, want 320x240", width, height)
		}
		style, _, _ = procGetWindowLongPtrW.Call(uintptr(w.hwnd), gwlStyle)
		if style&wsOverlappedWindow != wsOverlappedWindow {
			t.Errorf("restored window is missing decorations: style=%#x", style)
		}

		// Further toggles must preserve visibility and the original dimensions.
		w.SetFullscreen(true)
		assertFullscreenBounds(t, w)
		w.SetFullscreen(false)
		assertWindowVisible(t, w, true)
		if width, height := w.LogicalSize(); width != 320 || height != 240 {
			t.Errorf("client size after second toggle = %dx%d, want 320x240", width, height)
		}
		w.Hide()
	}
}

func assertWindowVisible(t *testing.T, w *win32Window, want bool) {
	t.Helper()
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(uintptr(w.hwnd))
	if (visible != 0) != want {
		t.Errorf("window visible = %t, want %t", visible != 0, want)
	}
}

func assertFullscreenBounds(t *testing.T, w *win32Window) {
	t.Helper()
	monitor, _, _ := procMonitorFromWindow.Call(uintptr(w.hwnd), monitorDefaultToNearest)
	info := monitorInfo{cbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatal("GetMonitorInfoW failed")
	}
	var bounds rect
	if ok, _, _ := user32.NewProc("GetWindowRect").Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		t.Fatal("GetWindowRect failed")
	}
	if bounds != info.rcMonitor {
		t.Errorf("fullscreen bounds = %+v, want %+v", bounds, info.rcMonitor)
	}
	width, height := w.PhysicalSize()
	if width != int(info.rcMonitor.right-info.rcMonitor.left) || height != int(info.rcMonitor.bottom-info.rcMonitor.top) {
		t.Errorf("cached physical size = %dx%d, does not cover monitor %+v", width, height, info.rcMonitor)
	}
}
