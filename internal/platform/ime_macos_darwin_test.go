//go:build darwin

package platform

import (
	"math"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"github.com/gogpu/gpucontext"
)

var macPlatformMainThreadTasks chan func()

func TestMain(m *testing.M) {
	macPlatformMainThreadTasks = make(chan func())
	done := make(chan int, 1)
	runtime.LockOSThread()
	go func() {
		done <- m.Run()
		close(macPlatformMainThreadTasks)
	}()
	for task := range macPlatformMainThreadTasks {
		task()
	}
	os.Exit(<-done)
}

func runOnMacPlatformMainThread(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	var panicValue any
	macPlatformMainThreadTasks <- func() {
		defer func() {
			panicValue = recover()
			close(done)
		}()
		fn()
	}
	<-done
	if panicValue != nil {
		panic(panicValue)
	}
}

func newMacIMEWindow() *darwinWindow {
	return &darwinWindow{id: 7, events: eventqueue.New[Event](16), ime: macIMEState{enabled: true}}
}

func macIMEString(t *testing.T, value string) platformdarwin.ID {
	t.Helper()
	text := platformdarwin.NewNSString(value)
	if text == nil || text.ID() == 0 {
		t.Fatalf("NewNSString(%q) failed", value)
	}
	t.Cleanup(text.Release)
	return text.ID()
}

func popMacIMEEvents(w *darwinWindow) []Event {
	var events []Event
	for {
		event, ok := w.events.Pop()
		if !ok {
			return events
		}
		events = append(events, event)
	}
}

func TestMacIMECompositionCommitAndCancelEvents(t *testing.T) {
	w := newMacIMEWindow()
	w.handleMacSetMarkedText(macIMEString(t, "你😀"), 1, 2, 0, 0)
	w.handleMacSetMarkedText(macIMEString(t, "你好"), 2, 0, 0, 0)
	got := popMacIMEEvents(w)
	if len(got) != 3 || got[0].Type != EventIMECompositionStart ||
		got[1].Type != EventIMECompositionUpdate || got[2].Type != EventIMECompositionUpdate {
		t.Fatalf("composition events = %#v, want start and two updates", got)
	}
	if got[1].IME == nil || got[1].IME.State.CompositionText != "你😀" ||
		got[1].IME.State.Cursor != (gpucontext.IMETextRange{Start: 3, End: 7}) {
		t.Fatalf("first composition update = %+v", got[1].IME)
	}

	w.handleMacInsertText(macIMEString(t, "你好"), ^uintptr(0), ^uintptr(0))
	got = popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMECompositionEnd || got[0].IME == nil || got[0].IME.Committed != "你好" {
		t.Fatalf("commit events = %#v, want one composition end with committed text", got)
	}

	w.handleMacSetMarkedText(macIMEString(t, "かな"), 2, 0, 0, 0)
	_ = popMacIMEEvents(w)
	w.handleMacUnmarkText()
	got = popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMECanceled {
		t.Fatalf("unmark events = %#v, want one cancellation", got)
	}
}

func TestMacIMEPrivacyAndOrdinaryTextDispatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		purpose gpucontext.ContentPurpose
		hints   gpucontext.ContentHint
	}{
		{name: "password purpose", purpose: gpucontext.ContentPurposePassword},
		{name: "hidden hint", hints: gpucontext.ContentHintHiddenText},
		{name: "sensitive hint", hints: gpucontext.ContentHintSensitiveData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newMacIMEWindow()
			w.ime.purpose, w.ime.hints = tc.purpose, tc.hints
			w.handleMacSetMarkedText(macIMEString(t, "secret"), 6, 0, 0, 0)
			w.handleMacInsertText(macIMEString(t, "secret"), 0, 0)
			if events := popMacIMEEvents(w); len(events) != 0 {
				t.Fatalf("sensitive input emitted events: %#v", events)
			}
		})
	}

	w := newMacIMEWindow()
	w.handleMacInsertText(macIMEString(t, "a\n😀\x01"), 0, 0)
	got := popMacIMEEvents(w)
	if len(got) != 2 || got[0].Type != EventChar || got[0].Char != 'a' || got[1].Type != EventChar || got[1].Char != '😀' {
		t.Fatalf("ordinary insert events = %#v, want printable runes only", got)
	}
}

func TestMacIMECommitAndCommandInOneKey(t *testing.T) {
	w := newMacIMEWindow()
	w.handleMacSetMarkedText(macIMEString(t, "한"), 1, 0, 0, 0)
	popMacIMEEvents(w)
	w.imeKey.begin(true)
	if !w.handleMacInsertText(macIMEString(t, "한"), 0, 0) {
		t.Fatal("commit was rejected")
	}
	w.handleMacIMECommand()
	got := popMacIMEEvents(w)
	if !w.imeKey.consumed || len(got) != 1 || got[0].Type != EventIMECompositionEnd || got[0].IME.Committed != "한" {
		t.Fatalf("commit then command: consumed=%v events=%#v", w.imeKey.consumed, got)
	}
}

func TestMacIMEUnhandledCommandCancelsPreedit(t *testing.T) {
	w := newMacIMEWindow()
	w.handleMacSetMarkedText(macIMEString(t, "ni"), 2, 0, 0, 0)
	popMacIMEEvents(w)
	w.imeKey.begin(true)
	if w.handleMacInsertText(macIMEString(t, "\r"), 0, 0) {
		t.Fatal("control-character insertion must not alter native storage")
	}
	if !w.ime.marked || len(popMacIMEEvents(w)) != 0 {
		t.Fatal("control character changed or committed the preedit")
	}
	w.handleMacIMECommand()
	got := popMacIMEEvents(w)
	if w.imeKey.consumed || w.ime.marked || !w.ime.nativeNeedsUnmark || len(got) != 1 || got[0].Type != EventIMECanceled {
		t.Fatalf("unhandled command: key=%+v state=%+v events=%#v", w.imeKey, w.ime, got)
	}
}

func TestMacIMEValidCursorArea(t *testing.T) {
	for _, tc := range []struct {
		name string
		area gpucontext.IMECursorArea
		want bool
	}{
		{name: "zero", want: true},
		{name: "valid", area: gpucontext.IMECursorArea{X: 1, Y: 2, Width: 3, Height: 4}, want: true},
		{name: "negative", area: gpucontext.IMECursorArea{X: -1}, want: false},
		{name: "nan", area: gpucontext.IMECursorArea{Y: math.NaN()}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validIMECursorArea(tc.area); got != tc.want {
				t.Fatalf("validIMECursorArea(%+v) = %v, want %v", tc.area, got, tc.want)
			}
		})
	}
}

func TestMacIMEControllerEnablePrivacy(t *testing.T) {
	w := newMacIMEWindow()
	w.ime.enabled = false
	if w.imeEnabled() {
		t.Fatal("IME must be disabled by default")
	}

	// Enabling requires a real NSTextInputClient view; set the state directly
	// here to exercise controller transitions without constructing an NSWindow.
	w.ime.enabled = true
	w.ime.setMarked("preedit", 7, 0)
	w.SetIMEEnabled(false)
	if w.imeEnabled() || w.ime.marked {
		t.Fatalf("disable retained IME state: %+v", w.ime)
	}
	got := popMacIMEEvents(w)
	if len(got) != 2 || got[0].Type != EventIMECanceled || got[1].Type != EventIMEDisabled {
		t.Fatalf("disable events = %#v, want canceled then disabled", got)
	}

	w.ime.enabled = true
	w.SetIMEContentType(gpucontext.ContentPurposePassword, gpucontext.ContentHintNone)
	if w.imeEnabled() {
		t.Fatal("password purpose did not disable IME")
	}
	got = popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMEDisabled {
		t.Fatalf("password transition events = %#v, want disabled", got)
	}
	w.SetIMEContentType(gpucontext.ContentPurposeNormal, gpucontext.ContentHintSensitiveData)
	w.ime.enabled = true
	w.SetIMEContentType(gpucontext.ContentPurposeNormal, gpucontext.ContentHintSensitiveData)
	if w.imeEnabled() {
		t.Fatal("sensitive hint did not prevent IME from enabling")
	}
}

func TestMacIMECursorAreaAndCancelController(t *testing.T) {
	w := newMacIMEWindow()
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16})
	if w.ime.area != (gpucontext.IMECursorArea{X: 10, Y: 20, Width: 1, Height: 16}) {
		t.Fatalf("cursor area = %+v", w.ime.area)
	}
	w.SetIMECursorArea(gpucontext.IMECursorArea{X: -1})
	if w.ime.area.X != 10 {
		t.Fatalf("invalid cursor area replaced valid value: %+v", w.ime.area)
	}
	w.ime.setMarked("preedit", 7, 0)
	w.CancelIME()
	if !w.imeEnabled() || w.ime.marked || !w.ime.nativeNeedsUnmark {
		t.Fatalf("CancelIME did not clear state and defer native unmark: %+v", w.ime)
	}
	got := popMacIMEEvents(w)
	if len(got) != 1 || got[0].Type != EventIMECanceled {
		t.Fatalf("CancelIME events = %#v, want one cancellation", got)
	}
	w.flushNativeIMEReset(0)
	if w.ime.nativeNeedsUnmark {
		t.Fatal("flushNativeIMEReset did not consume pending reset")
	}
}

func TestMacIMECapabilitiesDoNotAdvertiseSurroundingText(t *testing.T) {
	if caps := DefaultIMECapabilities(); caps&gpucontext.IMECapabilitySurroundingText != 0 {
		t.Fatalf("surrounding text is not supported, got capabilities %v", caps)
	}
	var window *darwinPlatformWindow
	if caps := window.IMECapabilities(); caps != 0 {
		t.Fatalf("missing window reported capabilities %v", caps)
	}
}

func TestMacIMEAppKitCallbackToPlatformEventsE2E(t *testing.T) {
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("native AppKit text-input integration test requires an interactive macOS session")
	}

	var libobjc unsafe.Pointer
	var objcMsgSend unsafe.Pointer
	runOnMacPlatformMainThread(t, func() {
		var err error
		libobjc, err = ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
		if err != nil {
			t.Fatalf("LoadLibrary(libobjc): %v", err)
		}
		objcMsgSend, err = ffi.GetSymbol(libobjc, "objc_msgSend")
		if err != nil {
			t.Fatalf("GetSymbol(objc_msgSend): %v", err)
		}

		poolClass := platformdarwin.GetClass("NSAutoreleasePool")
		pool := poolClass.Send(platformdarwin.RegisterSelector("new"))
		if pool.IsNil() {
			t.Fatal("failed to create autorelease pool")
		}
		defer pool.Send(platformdarwin.RegisterSelector("drain"))

		view, err := platformdarwin.CreateGoGPUView(platformdarwin.MakeRect(0, 0, 320, 200))
		if err != nil {
			t.Fatalf("CreateGoGPUView(): %v", err)
		}
		defer view.Send(platformdarwin.RegisterSelector("release"))
		if !platformdarwin.ViewSupportsTextInput(view) {
			t.Fatal("GoGPUView does not inherit NSTextInputClient")
		}

		w := newMacIMEWindow()
		platformdarwin.SetViewTextInputHandler(view, platformdarwin.TextInputHandler{
			SetMarkedText: func(text platformdarwin.ID, selectedLocation, selectedLength,
				_, _ uintptr) {
				w.handleMacSetMarkedText(text, selectedLocation, selectedLength, 0, 0)
			},
			InsertText: func(text platformdarwin.ID, _, _ uintptr) bool {
				return w.handleMacInsertText(text, 0, 0)
			},
			UnmarkText: w.handleMacUnmarkText,
		})
		defer platformdarwin.ClearViewTextInputHandler(view)

		preedit := platformdarwin.NewNSString("かな")
		if preedit == nil || preedit.ID().IsNil() {
			t.Fatal("NewNSString(preedit) failed")
		}
		callMacIMETextRangeSelector(t, objcMsgSend, view, "setMarkedText:selectedRange:replacementRange:",
			preedit.ID(), [2]uintptr{2, 0}, [2]uintptr{^uintptr(0), ^uintptr(0)})
		preedit.Release()
		got := popMacIMEEvents(w)
		if len(got) != 2 || got[0].Type != EventIMECompositionStart ||
			got[1].Type != EventIMECompositionUpdate || got[1].IME == nil ||
			got[1].IME.State.CompositionText != "かな" ||
			got[1].IME.State.Cursor != (gpucontext.IMETextRange{Start: 6, End: 6}) {
			t.Fatalf("native preedit events = %#v, want start and UTF-8 update", got)
		}

		commit := platformdarwin.NewNSString("仮名")
		if commit == nil || commit.ID().IsNil() {
			t.Fatal("NewNSString(commit) failed")
		}
		callMacIMETextRangeSelector(t, objcMsgSend, view, "insertText:replacementRange:",
			commit.ID(), [2]uintptr{^uintptr(0), ^uintptr(0)})
		commit.Release()
		got = popMacIMEEvents(w)
		if len(got) != 1 || got[0].Type != EventIMECompositionEnd || got[0].IME == nil ||
			got[0].IME.Committed != "仮名" {
			t.Fatalf("native commit events = %#v, want exactly one committed IME event", got)
		}

		preedit = platformdarwin.NewNSString("に")
		callMacIMETextRangeSelector(t, objcMsgSend, view, "setMarkedText:selectedRange:replacementRange:",
			preedit.ID(), [2]uintptr{1, 0}, [2]uintptr{^uintptr(0), ^uintptr(0)})
		preedit.Release()
		_ = popMacIMEEvents(w)
		view.Send(platformdarwin.RegisterSelector("unmarkText"))
		got = popMacIMEEvents(w)
		if len(got) != 1 || got[0].Type != EventIMECanceled {
			t.Fatalf("native cancel events = %#v, want one cancellation", got)
		}
	})
}

func callMacIMETextRangeSelector(t *testing.T, objcMsgSend unsafe.Pointer, view platformdarwin.ID, selectorName string, text platformdarwin.ID, ranges ...[2]uintptr) {
	t.Helper()
	rangeType := &types.TypeDescriptor{
		Size:      16,
		Alignment: 8,
		Kind:      types.StructType,
		Members: []*types.TypeDescriptor{
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
		},
	}
	argTypes := make([]*types.TypeDescriptor, 3+len(ranges))
	argTypes[0] = types.PointerTypeDescriptor
	argTypes[1] = types.PointerTypeDescriptor
	argTypes[2] = types.PointerTypeDescriptor
	for i := range ranges {
		argTypes[3+i] = rangeType
	}
	cif := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(cif, types.DefaultCall, types.VoidTypeDescriptor, argTypes); err != nil {
		t.Fatalf("PrepareCallInterface(): %v", err)
	}
	selector := uintptr(platformdarwin.RegisterSelector(selectorName))
	self := view.Ptr()
	object := text.Ptr()
	values := make([]unsafe.Pointer, 0, 3+len(ranges))
	values = append(values, unsafe.Pointer(&self), unsafe.Pointer(&selector), unsafe.Pointer(&object))
	for i := range ranges {
		values = append(values, unsafe.Pointer(&ranges[i]))
	}
	if _, err := ffi.CallFunction(cif, objcMsgSend, nil, values); err != nil {
		t.Fatalf("objc_msgSend(%s): %v", selectorName, err)
	}
	runtime.KeepAlive(ranges)
}
