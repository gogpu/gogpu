//go:build windows

package platform

import (
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var (
	procNativeGetKeyboardState  = user32.NewProc("GetKeyboardState")
	procNativeGetKeyboardLayout = user32.NewProc("GetKeyboardLayout")
	procNativeToUnicodeEx       = user32.NewProc("ToUnicodeEx")
)

const (
	wmDeadChar        = 0x0103
	wmSysDeadChar     = 0x0107
	wmInputLangChange = 0x0051
)

type nativeTextPending struct {
	event NativeKeyEvent
	units int
}

// Windows posts translated character messages behind already queued messages.
// Keep each source snapshot until its UTF-16 units arrive, even if modifiers
// have been released or another key has already been dequeued in this batch.
type nativeKeyboardState struct {
	pending    []nativeTextPending
	composing  bool
	high       uint16
	highSource NativeKeyEvent
}

func (s *nativeKeyboardState) key(event NativeKeyEvent, units int) NativeKeyEvent {
	if event.Kind != NativeKeyDown {
		return event
	}
	switch event.TextExpected {
	case NativeTextDeadKey:
		s.composing = true
	case NativeTextExpected:
		event.TextInput = s.composing || nativeTextChord(event.ControlKeyState)
		s.composing = false
	}
	if units > 0 {
		s.pending = append(s.pending, nativeTextPending{event: event, units: units})
	}
	return event
}

func nativeTextChord(mods uint32) bool {
	return mods&(0x0004|0x0008) != 0 && mods&(0x0001|0x0002) != 0
}

func (s *nativeKeyboardState) unitSource(fallback NativeKeyEvent) NativeKeyEvent {
	for i := range s.pending {
		pending := &s.pending[i]
		if pending.event.VirtualScanCode != fallback.VirtualScanCode {
			continue
		}
		source := pending.event
		pending.units--
		if pending.units == 0 {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
		}
		if fallback.RepeatCount != 0 {
			source.RepeatCount = fallback.RepeatCount
		}
		return source
	}
	// An IME may commit text without a preceding physical key. Do not invent
	// a key or read the current layout to reconstruct one.
	fallback.TextInput = true
	return fallback
}

func (s *nativeKeyboardState) utf16(unit uint16, fallback NativeKeyEvent, systemChar bool) (NativeKeyEvent, bool) {
	source := s.unitSource(fallback)
	// Right Alt on a non-AltGr layout can generate WM_SYSCHAR for an
	// ordinary accelerator. Only an actual WM_CHAR confirms its text intent.
	if !systemChar && source.ControlKeyState&0x0001 != 0 && source.ControlKeyState&(0x0002|0x0004) == 0 {
		source.TextInput = true
	}
	if unit >= 0xd800 && unit <= 0xdbff {
		s.high, s.highSource = unit, source
		return NativeKeyEvent{}, false
	}
	char := rune(unit)
	if unit >= 0xdc00 && unit <= 0xdfff {
		if s.high == 0 || !sameNativeTextSource(s.highSource, source) {
			s.high = 0
			return NativeKeyEvent{}, false
		}
		char = utf16.DecodeRune(rune(s.high), rune(unit))
		source = s.highSource
	}
	s.high = 0
	return nativeCommit(source, char)
}

func sameNativeTextSource(a, b NativeKeyEvent) bool {
	return a.VirtualKeyCode == b.VirtualKeyCode && a.VirtualScanCode == b.VirtualScanCode &&
		a.ControlKeyState == b.ControlKeyState && a.RepeatCount == b.RepeatCount && a.TextInput == b.TextInput
}

func nativeCommit(source NativeKeyEvent, char rune) (NativeKeyEvent, bool) {
	if char < ' ' || char == 0x7f || !utf8.ValidRune(char) {
		return NativeKeyEvent{}, false
	}
	source.Kind = NativeTextCommit
	source.Char = char
	return source, true
}

func nativeControlKeyState(state *[256]byte, lParam uintptr) uint32 {
	var mods uint32
	for _, key := range []struct {
		vk   byte
		flag uint32
	}{
		{0xa5, 0x0001}, {0xa4, 0x0002}, {0xa3, 0x0004}, {0xa2, 0x0008}, {0x10, 0x0010},
	} {
		if state[key.vk]&0x80 != 0 {
			mods |= key.flag
		}
	}
	for _, key := range []struct {
		vk   byte
		flag uint32
	}{
		{0x90, 0x0020}, {0x91, 0x0040}, {0x14, 0x0080},
	} {
		if state[key.vk]&1 != 0 {
			mods |= key.flag
		}
	}
	if lParam&(1<<24) != 0 {
		mods |= 0x0100
	}
	return mods
}

func nativeMessageFields(m *msg, state *[256]byte) NativeKeyEvent {
	return NativeKeyEvent{
		VirtualKeyCode:  uint16(m.wParam & 0xffff),
		VirtualScanCode: uint16((m.lParam >> 16) & 0xff),
		RepeatCount:     uint16(m.lParam & 0xffff),
		ControlKeyState: nativeControlKeyState(state, m.lParam),
		TextExpected:    NativeTextNone,
	}
}

func snapshotNativeKeyboard() ([256]byte, uintptr, bool) {
	var state [256]byte
	ok, _, _ := procNativeGetKeyboardState.Call(uintptr(unsafe.Pointer(&state[0])))
	layout, _, _ := procNativeGetKeyboardLayout.Call(0)
	return state, layout, ok != 0 && layout != 0
}

func nativeTranslation(event NativeKeyEvent, state *[256]byte, layout uintptr) (NativeTextDisposition, int) {
	if !NativeKeysSupported || layout == 0 {
		return NativeTextUnknown, 0
	}
	var units [16]uint16
	n, _, _ := procNativeToUnicodeEx.Call(uintptr(event.VirtualKeyCode), uintptr(event.VirtualScanCode),
		uintptr(unsafe.Pointer(&state[0])), uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)),
		4, layout) // Windows 10 1607+: inspect without changing dead-key state.
	count := int32(n)
	if count < 0 {
		return NativeTextDeadKey, 1
	}
	if count == 0 {
		return NativeTextNone, 0
	}
	if count > int32(len(units)) {
		return NativeTextUnknown, 0
	}
	for _, char := range utf16.Decode(units[:count]) {
		if char >= ' ' && char != 0x7f {
			return NativeTextExpected, int(count)
		}
	}
	return NativeTextNone, int(count)
}

// captureNativeKey runs immediately after PeekMessage and BEFORE
// TranslateMessage changes the thread's keyboard/dead-key state.
func (p *windowsPlatform) captureNativeKey(m *msg) {
	if m.message != wmKeydown && m.message != wmSysKeydown && m.message != wmKeyup && m.message != wmSysKeyup {
		return
	}
	p.windowMu.RLock()
	w := p.windows[m.hwnd]
	p.windowMu.RUnlock()
	if w == nil {
		return
	}
	state, layout, ok := snapshotNativeKeyboard()
	event := nativeMessageFields(m, &state)
	event.Kind = NativeKeyUp
	if m.message == wmKeydown || m.message == wmSysKeydown {
		event.Kind = NativeKeyDown
		var units int
		if ok {
			event.TextExpected, units = nativeTranslation(event, &state, layout)
		} else {
			event.TextExpected = NativeTextUnknown
		}
		event = w.nativeKeyboard.key(event, units)
	}
	w.nativeCurrent = &event
}

func (w *win32Window) nativeCharFallback(lParam uintptr) NativeKeyEvent {
	state, _, _ := snapshotNativeKeyboard()
	event := nativeMessageFields(&msg{lParam: lParam}, &state)
	event.VirtualKeyCode = 0
	return event
}

func (w *win32Window) queueNativeCommit(event NativeKeyEvent) {
	// Preserve the existing legacy character callback alongside the optional
	// native stream. Neither subscription changes what the other receives.
	w.platform.queueEvent(Event{WindowID: w.id, Type: EventChar, Char: event.Char, NativeKey: &event})
}

func (w *win32Window) queueUnknownNativeKey() {
	if w.nativeCurrent != nil {
		w.platform.queueEvent(Event{WindowID: w.id, Type: EventNativeKey, NativeKey: w.nativeCurrent})
		w.nativeCurrent = nil
	}
}
