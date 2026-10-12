package gogpu

import "github.com/gogpu/gogpu/internal/platform"

// NativeKeyKind identifies a key press, release, or committed character.
type NativeKeyKind = platform.NativeKeyKind

const (
	// NativeKeyDown is a physical key press, before any character commit.
	NativeKeyDown = platform.NativeKeyDown
	// NativeKeyUp is a physical key release.
	NativeKeyUp = platform.NativeKeyUp
	// NativeTextCommit carries one Unicode scalar committed by the OS.
	NativeTextCommit = platform.NativeTextCommit
)

// NativeTextDisposition describes the source's expected key translation.
type NativeTextDisposition = platform.NativeTextDisposition

const (
	// NativeTextUnknown means translation could not be checked safely.
	NativeTextUnknown = platform.NativeTextUnknown
	// NativeTextNone means the key does not produce printable text.
	NativeTextNone = platform.NativeTextNone
	// NativeTextExpected means character messages are expected for the key.
	NativeTextExpected = platform.NativeTextExpected
	// NativeTextDeadKey means the key starts or continues composition.
	NativeTextDeadKey = platform.NativeTextDeadKey
)

// NativeKeyEvent preserves native key metadata for input adapters. Currently
// Windows supplies this stream. ControlKeyState is the Win32 KEY_EVENT_RECORD
// bit field (including left/right Ctrl and Alt), not gpucontext.Modifiers.
//
// A key's TextExpected is captured before TranslateMessage, so consumers may
// defer a text-producing press until NativeTextCommit. TextInput is true for
// confirmed modified/composed commits that must bypass application shortcuts;
// ordinary text keeps it false. Characters are Unicode scalars, including
// supplementary characters; multi-character commits are separate events.
// Physical metadata and RepeatCount remain available for terminal forwarding.
type NativeKeyEvent struct {
	Kind            NativeKeyKind
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	RepeatCount     uint16
	ControlKeyState uint32
	Char            rune
	TextExpected    NativeTextDisposition
	TextInput       bool
	WindowID        uint32
}

// OnNativeKey registers an optional native keyboard callback before Run.
// It returns false outside Windows 10 1607 or later. The callback runs in the
// same ordered dispatch as focus and pointer callbacks. EventSource callbacks
// are unchanged; use one keyboard stream to avoid processing each key twice.
func (a *App) OnNativeKey(fn func(NativeKeyEvent)) bool {
	if !platform.NativeKeysSupported {
		return false
	}
	a.onNativeKey = fn
	return true
}

func (a *App) dispatchNativeKey(event *platform.Event) {
	if a.onNativeKey == nil || event.NativeKey == nil {
		return
	}
	e := event.NativeKey
	a.onNativeKey(NativeKeyEvent{
		Kind: e.Kind, VirtualKeyCode: e.VirtualKeyCode, VirtualScanCode: e.VirtualScanCode,
		RepeatCount: e.RepeatCount, ControlKeyState: e.ControlKeyState, Char: e.Char,
		TextExpected: e.TextExpected, TextInput: e.TextInput, WindowID: uint32(event.WindowID),
	})
}
