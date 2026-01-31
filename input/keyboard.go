package input

import (
	"sync"
)

// Key represents a keyboard key.
type Key uint16

// Keyboard key codes
const (
	KeyUnknown Key = iota

	// Function keys
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12

	// Number keys
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9

	// Letter keys
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ

	// Special keys
	KeySpace
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeyCapsLock
	KeyShiftLeft
	KeyShiftRight
	KeyControlLeft
	KeyControlRight
	KeyAltLeft
	KeyAltRight
	KeySuperLeft  // Windows/Command key
	KeySuperRight // Windows/Command key

	// Arrow keys
	KeyUp
	KeyDown
	KeyLeft
	KeyRight

	// Navigation keys
	KeyInsert
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown

	// Punctuation
	KeyMinus
	KeyEqual
	KeyLeftBracket
	KeyRightBracket
	KeyBackslash
	KeySemicolon
	KeyApostrophe
	KeyGrave
	KeyComma
	KeyPeriod
	KeySlash

	// Numpad
	KeyNumpad0
	KeyNumpad1
	KeyNumpad2
	KeyNumpad3
	KeyNumpad4
	KeyNumpad5
	KeyNumpad6
	KeyNumpad7
	KeyNumpad8
	KeyNumpad9
	KeyNumpadAdd
	KeyNumpadSubtract
	KeyNumpadMultiply
	KeyNumpadDivide
	KeyNumpadEnter
	KeyNumpadDecimal
	KeyNumLock

	// Other
	KeyPrintScreen
	KeyScrollLock
	KeyPause

	KeyCount // Number of keys
)

// KeyboardState holds keyboard input state.
// All methods are thread-safe.
type KeyboardState struct {
	mu       sync.RWMutex
	current  [KeyCount]bool
	previous [KeyCount]bool
}

func newKeyboardState() KeyboardState {
	return KeyboardState{}
}

// SetKey sets key state (called by platform layer).
// Thread-safe.
func (k *KeyboardState) SetKey(key Key, pressed bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if key < KeyCount {
		k.current[key] = pressed
	}
}

// Pressed returns true if key is currently pressed.
// Thread-safe.
func (k *KeyboardState) Pressed(key Key) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if key >= KeyCount {
		return false
	}
	return k.current[key]
}

// JustPressed returns true if key was just pressed this frame.
// Thread-safe.
func (k *KeyboardState) JustPressed(key Key) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if key >= KeyCount {
		return false
	}
	return k.current[key] && !k.previous[key]
}

// JustReleased returns true if key was just released this frame.
// Thread-safe.
func (k *KeyboardState) JustReleased(key Key) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if key >= KeyCount {
		return false
	}
	return !k.current[key] && k.previous[key]
}

// AnyPressed returns true if any key is pressed.
// Thread-safe.
func (k *KeyboardState) AnyPressed() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	for _, pressed := range k.current {
		if pressed {
			return true
		}
	}
	return false
}

// Modifier returns true if a modifier key is pressed.
// Thread-safe.
func (k *KeyboardState) Modifier(mod Modifier) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	switch mod {
	case ModShift:
		return k.current[KeyShiftLeft] || k.current[KeyShiftRight]
	case ModControl:
		return k.current[KeyControlLeft] || k.current[KeyControlRight]
	case ModAlt:
		return k.current[KeyAltLeft] || k.current[KeyAltRight]
	case ModSuper:
		return k.current[KeySuperLeft] || k.current[KeySuperRight]
	}
	return false
}

// UpdateFrame advances the keyboard state to the next frame.
// Call this once per frame before processing new events.
// Thread-safe.
func (k *KeyboardState) UpdateFrame() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.previous = k.current
}

// Modifier represents keyboard modifiers.
type Modifier uint8

const (
	ModShift Modifier = 1 << iota
	ModControl
	ModAlt
	ModSuper
)
