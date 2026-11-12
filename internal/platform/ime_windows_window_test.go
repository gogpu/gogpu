//go:build windows

package platform

import (
	"math"
	"runtime"
	"testing"
	"unsafe"

	"github.com/gogpu/gpucontext"
	"golang.org/x/sys/windows"
)

const smIMMEnabled = 82 // SM_IMMENABLED

var procSendMessageWTest = user32.NewProc("SendMessageW")

// newIMETestWindow creates a real window on the calling thread, which must be
// locked to its OS thread. IMM32 association is a property of the HWND, so the
// native default can only be observed on a real window.
func newIMETestWindow(t *testing.T) (*windowsPlatform, *win32Window) {
	t.Helper()
	if enabled, _, _ := procGetSystemMetrics.Call(smIMMEnabled); enabled == 0 {
		t.Skip("IMM32 is disabled on this system (SM_IMMENABLED = 0)")
	}
	p := newPlatformManager().(*windowsPlatform)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Destroy()
		name, err := windows.UTF16PtrFromString("GoGPUWindow")
		if err != nil {
			t.Fatal(err)
		}
		user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), uintptr(p.hinstance))
		windows.NewLazySystemDLL("ole32.dll").NewProc("OleUninitialize").Call()
	})
	created, err := p.CreateWindow(Config{Title: "IME test", Width: 320, Height: 240})
	if err != nil {
		t.Fatal(err)
	}
	return p, created.(*win32Window)
}

// imeAssociated reports whether the window currently has an input context,
// i.e. whether the system IME can compose into it.
func imeAssociated(w *win32Window) bool {
	himc, _, _ := procImmGetContext.Call(uintptr(w.hwnd))
	if himc == 0 {
		return false
	}
	procImmReleaseContext.Call(uintptr(w.hwnd), himc)
	return true
}

// drainEvents pumps the window's messages and returns the queued events.
func drainEvents(p *windowsPlatform) []Event {
	var events []Event
	for {
		ev := p.PollEvents()
		if ev.Type == EventNone {
			return events
		}
		events = append(events, ev)
	}
}

// An application that never touches the IME controller keeps the system IME:
// the window keeps its input context, composition goes through
// DefWindowProc, and the committed text arrives as WM_CHAR (LEAD-023 F1).
func TestWin32NewWindowKeepsTheSystemIME(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p, w := newIMETestWindow(t)
	drainEvents(p)

	if !imeAssociated(w) {
		t.Fatal("a new window has no input context: the system IME cannot compose into it")
	}

	// Unmanaged windows leave composition to the system: no IME lifecycle
	// events reach the app, which gets the result as text input instead.
	procSendMessageWTest.Call(uintptr(w.hwnd), wmIMEStartComposition, 0, 0)
	procSendMessageWTest.Call(uintptr(w.hwnd), wmIMEEndComposition, 0, 0)
	for _, ev := range drainEvents(p) {
		switch ev.Type {
		case EventIMECompositionStart, EventIMECompositionUpdate, EventIMECompositionEnd, EventIMECanceled:
			t.Errorf("unmanaged window queued IME event %v", ev.Type)
		}
	}

	// Focus loss does not detach the IME.
	procSendMessageWTest.Call(uintptr(w.hwnd), wmKillFocus, 0, 0)
	drainEvents(p)
	if !imeAssociated(w) {
		t.Error("focus loss detached the system IME from an unmanaged window")
	}
}

// Once the app configures IME, enabling and disabling attach and detach the
// input context, and focus loss keeps what the app asked for.
func TestWin32ManagedIMEFollowsTheApp(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p, w := newIMETestWindow(t)
	drainEvents(p)

	w.SetIMEEnabled(false)
	drainEvents(p)
	if imeAssociated(w) {
		t.Fatal("SetIMEEnabled(false) left the input context attached")
	}

	w.SetIMEEnabled(true)
	drainEvents(p)
	if !imeAssociated(w) {
		t.Fatal("SetIMEEnabled(true) did not attach the input context")
	}

	procSendMessageWTest.Call(uintptr(w.hwnd), wmKillFocus, 0, 0)
	drainEvents(p)
	if !imeAssociated(w) {
		t.Error("focus loss detached the IME the app enabled")
	}
	w.imeMu.Lock()
	enabled := w.ime.enabled
	w.imeMu.Unlock()
	if !enabled {
		t.Error("focus loss turned off the IME the app enabled")
	}
}

var procImmGetCandidateWindowTest = imm32.NewProc("ImmGetCandidateWindow")

// The candidate window is placed in the client coordinates of the window that
// owns the input context, as CANDIDATEFORM requires (winit, SDL3). A window
// away from the screen origin shows the difference (LEAD-023 F2).
func TestWin32IMECandidateWindowUsesClientCoordinates(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p, w := newIMETestWindow(t)
	w.SetPosition(300, 200)
	w.Show()
	drainEvents(p)

	area := gpucontext.IMECursorArea{X: 10, Y: 20, Height: 16}
	w.SetIMEEnabled(true)
	w.SetIMECursorArea(area)
	drainEvents(p)

	himc, _, _ := procImmGetContext.Call(uintptr(w.hwnd))
	if himc == 0 {
		t.Fatal("no input context after SetIMEEnabled(true)")
	}
	defer procImmReleaseContext.Call(uintptr(w.hwnd), himc)
	var form candidateForm
	if ok, _, _ := procImmGetCandidateWindowTest.Call(himc, 0, uintptr(unsafe.Pointer(&form))); ok == 0 {
		t.Fatal("ImmGetCandidateWindow failed")
	}

	scale := w.scaleFactor()
	want := point{x: int32(math.Round(area.X * scale)), y: int32(math.Round(area.Y * scale))}
	if form.ptCurrentPos != want {
		t.Errorf("candidate position = %+v, want client %+v", form.ptCurrentPos, want)
	}
}
