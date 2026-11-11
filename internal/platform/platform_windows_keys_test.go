//go:build windows

package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

// keyMessageLParam builds a WM_KEYDOWN lParam: repeat count 1, the scancode
// in bits 16-23 and the extended flag in bit 24.
func keyMessageLParam(scancode uintptr) uintptr {
	return scancode<<16 | 1
}

func TestKeyFromMessage(t *testing.T) {
	tests := []struct {
		name     string
		wParam   uintptr
		scancode uintptr
		want     gpucontext.Key
	}{
		// The virtual key follows the layout; the key follows the position.
		{"Q position on QWERTY", 'Q', 0x10, gpucontext.KeyQ},
		{"Q position on AZERTY", 'A', 0x10, gpucontext.KeyQ},
		{"A position on AZERTY", 'Q', 0x1E, gpucontext.KeyA},
		{"W position on AZERTY", 'Z', 0x11, gpucontext.KeyW},
		{"Z position on AZERTY", 'W', 0x2C, gpucontext.KeyZ},
		{"M position on AZERTY", vkOEMComma, 0x32, gpucontext.KeyM},
		{"semicolon position on AZERTY", 'M', 0x27, gpucontext.KeySemicolon},
		{"Z position on QWERTZ", 'Y', 0x2C, gpucontext.KeyZ},
		{"Y position on QWERTZ", 'Z', 0x15, gpucontext.KeyY},
		{"digit 1 on AZERTY", 0x31, 0x02, gpucontext.Key1},
		{"digit 0", 0x30, 0x0B, gpucontext.Key0},

		// Letters by row.
		{"P", 'P', 0x19, gpucontext.KeyP},
		{"L", 'L', 0x26, gpucontext.KeyL},
		{"M", 'M', 0x32, gpucontext.KeyM},

		// Punctuation.
		{"minus", vkOEMMinus, 0x0C, gpucontext.KeyMinus},
		{"equal", vkOEMPlus, 0x0D, gpucontext.KeyEqual},
		{"left bracket", vkOEM4, 0x1A, gpucontext.KeyLeftBracket},
		{"right bracket", vkOEM6, 0x1B, gpucontext.KeyRightBracket},
		{"semicolon", vkOEM1, 0x27, gpucontext.KeySemicolon},
		{"apostrophe", vkOEM7, 0x28, gpucontext.KeyApostrophe},
		{"grave", vkOEM3, 0x29, gpucontext.KeyGrave},
		{"backslash", vkOEM5, 0x2B, gpucontext.KeyBackslash},
		{"comma", vkOEMComma, 0x33, gpucontext.KeyComma},
		{"period", vkOEMPeriod, 0x34, gpucontext.KeyPeriod},
		{"slash", vkOEM2, 0x35, gpucontext.KeySlash},

		// Editing and whitespace.
		{"escape", vkEscape, 0x01, gpucontext.KeyEscape},
		{"backspace", vkBack, 0x0E, gpucontext.KeyBackspace},
		{"tab", vkTab, 0x0F, gpucontext.KeyTab},
		{"enter", vkReturn, 0x1C, gpucontext.KeyEnter},
		{"space", vkSpace, 0x39, gpucontext.KeySpace},

		// Function keys.
		{"F1", vkF1, 0x3B, gpucontext.KeyF1},
		{"F10", vkF10, 0x44, gpucontext.KeyF10},
		{"F11", vkF11, 0x57, gpucontext.KeyF11},
		{"F12", vkF12, 0x58, gpucontext.KeyF12},

		// Modifiers: left and right by scancode and the extended bit.
		{"left shift", vkShift, 0x2A, gpucontext.KeyLeftShift},
		{"right shift", vkShift, 0x36, gpucontext.KeyRightShift},
		{"right shift with the extended bit a CJK IME sets", vkShift, 0x136, gpucontext.KeyRightShift},
		{"left control", vkControl, 0x1D, gpucontext.KeyLeftControl},
		{"right control", vkControl, 0x11D, gpucontext.KeyRightControl},
		{"left alt", vkMenu, 0x38, gpucontext.KeyLeftAlt},
		{"right alt", vkMenu, 0x138, gpucontext.KeyRightAlt},
		{"left super", vkLWin, 0x15B, gpucontext.KeyLeftSuper},
		{"right super", vkRWin, 0x15C, gpucontext.KeyRightSuper},

		// Navigation cluster: extended scancodes.
		{"insert", vkInsert, 0x152, gpucontext.KeyInsert},
		{"delete", vkDeleteKey, 0x153, gpucontext.KeyDelete},
		{"home", vkHome, 0x147, gpucontext.KeyHome},
		{"end", vkEnd, 0x14F, gpucontext.KeyEnd},
		{"page up", vkPrior, 0x149, gpucontext.KeyPageUp},
		{"page down", vkNext, 0x151, gpucontext.KeyPageDown},
		{"left", vkLeftKey, 0x14B, gpucontext.KeyLeft},
		{"right", vkRightKey, 0x14D, gpucontext.KeyRight},
		{"up", vkUpKey, 0x148, gpucontext.KeyUp},
		{"down", vkDownKey, 0x150, gpucontext.KeyDown},

		// Numpad: the same scancodes without the extended bit, whatever Num Lock says.
		{"numpad 7", vkNumpad7, 0x47, gpucontext.KeyNumpad7},
		{"numpad 7 with Num Lock off", vkHome, 0x47, gpucontext.KeyNumpad7},
		{"numpad 0", vkNumpad0, 0x52, gpucontext.KeyNumpad0},
		{"numpad decimal with Num Lock off", vkDeleteKey, 0x53, gpucontext.KeyNumpadDecimal},
		{"numpad multiply", vkMultiply, 0x37, gpucontext.KeyNumpadMultiply},
		{"numpad subtract", vkSubtract, 0x4A, gpucontext.KeyNumpadSubtract},
		{"numpad add", vkAdd, 0x4E, gpucontext.KeyNumpadAdd},
		{"numpad divide", vkDivide, 0x135, gpucontext.KeyNumpadDivide},
		{"numpad enter", vkReturn, 0x11C, gpucontext.KeyEnter},

		// Locks: Num Lock and Pause share scancode 0x45, told apart by the extended bit.
		{"caps lock", vkCapital, 0x3A, gpucontext.KeyCapsLock},
		{"scroll lock", vkScroll, 0x46, gpucontext.KeyScrollLock},
		{"num lock", vkNumLock, 0x145, gpucontext.KeyNumLock},
		{"pause", vkPause, 0x45, gpucontext.KeyPause},
		{"ctrl+break", vkCancel, 0x146, gpucontext.KeyCancel},

		// No scancode, as from injected input: fall back to the virtual key.
		{"injected letter without a scancode", 'A', 0, gpucontext.KeyA},

		// Keys the IME consumed carry no key of their own.
		{"IME-processed key", vkProcessKey, 0x10, gpucontext.KeyUnknown},

		{"unmapped scancode", 'A', 0x7F, gpucontext.KeyUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := keyFromMessage(test.wParam, keyMessageLParam(test.scancode))
			if got != test.want {
				t.Errorf("keyFromMessage(%#x, scancode %#x) = %v, want %v",
					test.wParam, test.scancode, got, test.want)
			}
		})
	}
}
