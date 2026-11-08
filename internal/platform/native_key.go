package platform

// NativeKeyKind identifies the phase of a native keyboard message.
type NativeKeyKind uint8

const (
	NativeKeyDown NativeKeyKind = iota + 1
	NativeKeyUp
	NativeTextCommit
)

// NativeTextDisposition describes the translation expected from a key press.
type NativeTextDisposition uint8

const (
	NativeTextUnknown NativeTextDisposition = iota
	NativeTextNone
	NativeTextExpected
	NativeTextDeadKey
)

// NativeKeyEvent is captured at the source, before native text translation.
// ControlKeyState uses Win32 KEY_EVENT_RECORD flags, including modifier sides.
type NativeKeyEvent struct {
	Kind            NativeKeyKind
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	RepeatCount     uint16
	ControlKeyState uint32
	Char            rune
	TextExpected    NativeTextDisposition
	TextInput       bool
}
