//go:build windows

package platform

import (
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/gogpu/gpucontext"
	"golang.org/x/sys/windows"
)

var (
	procLoadKeyboardLayoutW   = user32.NewProc("LoadKeyboardLayoutW")
	procUnloadKeyboardLayout  = user32.NewProc("UnloadKeyboardLayout")
	procGetKeyboardLayoutList = user32.NewProc("GetKeyboardLayoutList")
	procUnregisterClassW      = user32.NewProc("UnregisterClassW")
	procOleUninitialize       = ole32.NewProc("OleUninitialize")
)

// newKeyLabelTestWindow opens a hidden window on a platform of its own, and
// tears the platform down, its window class included, when the test ends.
func newKeyLabelTestWindow(t *testing.T) *win32Window {
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
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(name)), uintptr(p.hinstance))
		procOleUninitialize.Call()
		runtime.UnlockOSThread()
	})
	created, err := p.CreateWindow(Config{Title: "key labels", Width: 320, Height: 240})
	if err != nil {
		t.Fatalf("CreateWindow = %v", err)
	}
	return created.(*win32Window)
}

// loadKeyboardLayout loads the layout named id, such as "0000040C" for
// French, without activating it. When the test ends it unloads the layout,
// unless the layout was loaded before.
func loadKeyboardLayout(t *testing.T, id string) uintptr {
	t.Helper()
	count, _, _ := procGetKeyboardLayoutList.Call(0, 0)
	loaded := make([]uintptr, count+1)
	procGetKeyboardLayoutList.Call(count, uintptr(unsafe.Pointer(&loaded[0])))
	name, err := windows.UTF16PtrFromString(id)
	if err != nil {
		t.Fatal(err)
	}
	const klfNoTellShell = 0x80
	layout, _, _ := procLoadKeyboardLayoutW.Call(uintptr(unsafe.Pointer(name)), klfNoTellShell)
	if layout == 0 {
		t.Skipf("layout %s cannot be loaded", id)
	}
	if !slices.Contains(loaded[:count], layout) {
		t.Cleanup(func() { procUnloadKeyboardLayout.Call(layout) })
	}
	return layout
}

// layoutChanges drains p's events and answers how many were layout changes.
func layoutChanges(p *windowsPlatform) int {
	count := 0
	for p.events.Len() > 0 {
		if p.dequeueEvent().Type == EventKeyboardLayoutChanged {
			count++
		}
	}
	return count
}

// expectLabels checks p's label of each key against labels.
func expectLabels(t *testing.T, p *windowsPlatform, labels map[gpucontext.Key]rune) {
	t.Helper()
	for key, want := range labels {
		if got := p.KeyLabel(key); got != want {
			t.Errorf("KeyLabel(%v) = %q, want %q", key, got, want)
		}
	}
}

func TestKeyLabelsFollowTheReportedLayout(t *testing.T) {
	window := newKeyLabelTestWindow(t)
	french := loadKeyboardLayout(t, "0000040C")
	us := loadKeyboardLayout(t, "00000409")
	p := window.platform
	wndProc(window.hwnd, wmInputLangChange, 0, us)
	layoutChanges(p)

	wndProc(window.hwnd, wmInputLangChange, 0, french)
	if got := layoutChanges(p); got != 1 {
		t.Errorf("a change to French reported %d layout changes, want 1", got)
	}
	expectLabels(t, p, map[gpucontext.Key]rune{gpucontext.KeyQ: 'a', gpucontext.KeyA: 'q', gpucontext.Key2: 'é'})

	wndProc(window.hwnd, wmInputLangChange, 0, french)
	if got := layoutChanges(p); got != 0 {
		t.Errorf("French reported again gave %d layout changes, want none", got)
	}

	wndProc(window.hwnd, wmInputLangChange, 0, us)
	if got := layoutChanges(p); got != 1 {
		t.Errorf("a change back to US reported %d layout changes, want 1", got)
	}
	expectLabels(t, p, map[gpucontext.Key]rune{gpucontext.KeyQ: 'q', gpucontext.KeyA: 'a', gpucontext.Key2: '2'})
}

func TestADeadKeysLabelLeavesNoDeadKeyPending(t *testing.T) {
	window := newKeyLabelTestWindow(t)
	wndProc(window.hwnd, wmInputLangChange, 0, loadKeyboardLayout(t, "0000040C"))

	// On French, the key right of P is the dead circumflex.
	expectLabels(t, window.platform, map[gpucontext.Key]rune{gpucontext.KeyLeftBracket: '^'})
	expectLabels(t, window.platform, map[gpucontext.Key]rune{gpucontext.KeyE: 'e'})
}
