//go:build windows

package platform

import (
	"reflect"
	"runtime"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	"golang.org/x/sys/windows"
)

func TestNativeTextBatchedPressesRetainOrigin(t *testing.T) {
	var state nativeKeyboardState
	q := NativeKeyEvent{Kind: NativeKeyDown, VirtualKeyCode: 0x51, VirtualScanCode: 16,
		ControlKeyState: 9, RepeatCount: 1, TextExpected: NativeTextExpected}
	e := q
	e.VirtualKeyCode, e.VirtualScanCode, e.ControlKeyState = 0x45, 18, 0
	state.key(q, 1)
	state.key(e, 1)
	// Both presses and modifier releases can already be queued when the
	// first WM_CHAR arrives. The commit must not borrow the latest key.
	for _, tc := range []struct {
		unit uint16
		scan uint16
		vk   uint16
		text bool
		mods uint32
	}{
		{'@', 16, 0x51, true, 9}, {'e', 18, 0x45, false, 0},
	} {
		got, ok := state.utf16(tc.unit, NativeKeyEvent{VirtualScanCode: tc.scan, RepeatCount: 1}, false)
		if !ok || got.VirtualKeyCode != tc.vk || got.TextInput != tc.text || got.ControlKeyState != tc.mods {
			t.Fatalf("commit %q = %+v", tc.unit, got)
		}
	}
	if len(state.pending) != 0 {
		t.Fatal("consumed presses retained")
	}
}

func TestNativeTextDeadKeyMultiScalarAndSupplementary(t *testing.T) {
	var state nativeKeyboardState
	dead := NativeKeyEvent{Kind: NativeKeyDown, VirtualKeyCode: 0xc0, VirtualScanCode: 41,
		TextExpected: NativeTextDeadKey, RepeatCount: 1}
	state.key(dead, 1)
	state.unitSource(NativeKeyEvent{VirtualScanCode: 41}) // WM_DEADCHAR, not committed text.
	base := NativeKeyEvent{Kind: NativeKeyDown, VirtualKeyCode: 0x51, VirtualScanCode: 16,
		TextExpected: NativeTextExpected, RepeatCount: 1}
	state.key(base, 2) // Uncombinable dead key followed by q commits two scalars.
	plain := base
	plain.VirtualKeyCode, plain.VirtualScanCode = 0x58, 45
	state.key(plain, 1)
	for _, ch := range []uint16{'~', 'q'} {
		got, ok := state.utf16(ch, NativeKeyEvent{VirtualScanCode: 16, RepeatCount: 1}, false)
		if !ok || !got.TextInput || got.VirtualKeyCode != 0x51 {
			t.Fatalf("composition = %+v", got)
		}
	}
	got, ok := state.utf16('x', NativeKeyEvent{VirtualScanCode: 45, RepeatCount: 1}, false)
	if !ok || got.TextInput {
		t.Fatalf("later plain text inherited composition: %+v", got)
	}
	base.ControlKeyState = 9
	state.key(base, 2)
	units := utf16.Encode([]rune("😀"))
	if _, ok := state.utf16(units[0], NativeKeyEvent{VirtualScanCode: 16, RepeatCount: 3}, false); ok {
		t.Fatal("high surrogate emitted")
	}
	got, ok = state.utf16(units[1], NativeKeyEvent{VirtualScanCode: 16, RepeatCount: 3}, false)
	if !ok || got.Char != '😀' || got.RepeatCount != 3 || !got.TextInput {
		t.Fatalf("supplementary commit = %+v", got)
	}
}

func TestNativeKeyModifiersPreserveSidesAndLocks(t *testing.T) {
	var state [256]byte
	for _, vk := range []byte{0xa5, 0xa4, 0xa3, 0xa2, 0x10} {
		state[vk] = 0x80
	}
	for _, vk := range []byte{0x90, 0x91, 0x14} {
		state[vk] = 1
	}
	if got := nativeControlKeyState(&state, 1<<24); got != 0x1ff {
		t.Fatalf("mods = %#x", got)
	}
}

func TestNativeRightAltOnlyTextAndShortcut(t *testing.T) {
	var state nativeKeyboardState
	event := NativeKeyEvent{Kind: NativeKeyDown, VirtualKeyCode: 0x51, VirtualScanCode: 16,
		ControlKeyState: 1, TextExpected: NativeTextExpected}
	state.key(event, 1)
	got, ok := state.utf16('@', NativeKeyEvent{VirtualScanCode: 16}, false)
	if !ok || !got.TextInput {
		t.Fatalf("RightAlt commit = %+v", got)
	}
	state.key(event, 1)
	got, ok = state.utf16('q', NativeKeyEvent{VirtualScanCode: 16}, true)
	if !ok || got.TextInput || got.ControlKeyState != 1 {
		t.Fatalf("RightAlt system accelerator = %+v", got)
	}
	event.TextExpected = NativeTextNone
	if got := state.key(event, 0); got.TextInput {
		t.Fatal("nontext RightAlt became text")
	}
}

func TestNativeSurrogatesCannotBorrowAnotherKey(t *testing.T) {
	var state nativeKeyboardState
	if _, ok := state.utf16(0xd83d, NativeKeyEvent{VirtualScanCode: 16}, false); ok {
		t.Fatal("emitted high")
	}
	if _, ok := state.utf16(0xde00, NativeKeyEvent{VirtualScanCode: 18}, false); ok {
		t.Fatal("combined different source messages")
	}
	if _, ok := state.utf16(0xde00, NativeKeyEvent{VirtualScanCode: 16}, false); ok {
		t.Fatal("retained invalid high surrogate")
	}
}

func TestNativeKeyWindowsTranslationSnapshot(t *testing.T) {
	if !NativeKeysSupported {
		t.Skip("nonmutating layout lookup requires Windows 10 1607")
	}
	load := user32.NewProc("LoadKeyboardLayoutW")
	name, err := windows.UTF16PtrFromString("00000407")
	if err != nil {
		t.Fatal(err)
	}
	layout, _, _ := load.Call(uintptr(unsafe.Pointer(name)), 0x80)
	if layout == 0 {
		t.Skip("German keyboard layout unavailable")
	}
	var keys [256]byte
	keys[0x11], keys[0xa2], keys[0x12], keys[0xa5] = 0x80, 0x80, 0x80, 0x80
	event := NativeKeyEvent{VirtualKeyCode: 0x51, VirtualScanCode: 16}
	original := keys
	disposition, units := nativeTranslation(event, &keys, layout)
	if disposition != NativeTextExpected || units != 1 {
		t.Fatalf("German AltGr+Q: %v %d", disposition, units)
	}
	if !reflect.DeepEqual(keys, original) {
		t.Fatal("translation changed keyboard snapshot")
	}
	event.VirtualKeyCode = vkF4
	if disposition, _ := nativeTranslation(event, &keys, layout); disposition != NativeTextNone {
		t.Fatalf("F4 became text: %v", disposition)
	}
}

func TestNativeSourceQueuePreservesPressReleaseCommitAcrossLayoutChange(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state, _, ok := snapshotNativeKeyboard()
	if !ok {
		t.Skip("keyboard snapshot unavailable")
	}
	set := user32.NewProc("SetKeyboardState")
	defer func() { set.Call(uintptr(unsafe.Pointer(&state[0]))) }()
	var keys [256]byte
	keys[0x51] = 0x80
	if ret, _, _ := set.Call(uintptr(unsafe.Pointer(&keys[0]))); ret == 0 {
		t.Skip("cannot prepare keyboard state")
	}
	p := &windowsPlatform{windows: make(map[windows.HWND]*win32Window), events: eventqueue.New[Event](16)}
	w := &win32Window{id: 1, platform: p, hwnd: 1}
	p.windows[w.hwnd] = w
	previous := globalPlatform
	globalPlatform = p
	t.Cleanup(func() { globalPlatform = previous })
	m := msg{hwnd: w.hwnd, message: wmKeydown, wParam: 0x51, lParam: 2 | 16<<16}
	p.captureNativeKey(&m)
	wndProc(w.hwnd, m.message, m.wParam, m.lParam)
	m.message = wmKeyup
	p.captureNativeKey(&m)
	wndProc(w.hwnd, m.message, m.wParam, m.lParam)
	// The new layout notification may be ahead of a character that Windows
	// already translated with the previous layout. Its source still applies.
	wndProc(w.hwnd, wmInputLangChange, 0, 0)
	wndProc(w.hwnd, wmChar, '@', 2|16<<16)
	for _, kind := range []NativeKeyKind{NativeKeyDown, NativeKeyUp, NativeTextCommit} {
		event := p.dequeueEvent()
		if event.NativeKey == nil || event.NativeKey.Kind != kind {
			t.Fatalf("source event = %+v, want native %d", event, kind)
		}
		if event.NativeKey.VirtualKeyCode != 0x51 || event.NativeKey.RepeatCount != 2 {
			t.Fatalf("lost source metadata: %+v", event.NativeKey)
		}
	}
	if got := p.dequeueEvent(); got.Type != EventNone {
		t.Fatalf("duplicate event: %+v", got)
	}
}

func TestNativeControlCharacterRetiresSourceBeforeSameScanText(t *testing.T) {
	var state nativeKeyboardState
	control := NativeKeyEvent{Kind: NativeKeyDown, VirtualKeyCode: 0x43, VirtualScanCode: 46,
		ControlKeyState: 8, TextExpected: NativeTextNone, RepeatCount: 1}
	plain := control
	plain.ControlKeyState = 0
	plain.TextExpected = NativeTextExpected
	state.key(control, 1) // Ctrl+C translates to U+0003, which is filtered.
	state.key(plain, 1)   // C is already queued before either WM_CHAR.
	if _, ok := state.utf16(3, NativeKeyEvent{VirtualScanCode: 46}, false); ok {
		t.Fatal("control character emitted as text")
	}
	got, ok := state.utf16('c', NativeKeyEvent{VirtualScanCode: 46}, false)
	if !ok || got.ControlKeyState != 0 || got.TextInput || got.VirtualKeyCode != 0x43 {
		t.Fatalf("plain C borrowed preceding Ctrl+C: %+v", got)
	}
	if len(state.pending) != 0 {
		t.Fatal("consumed control/text sources retained")
	}
}
