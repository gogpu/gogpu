//go:build linux

package x11

import (
	"testing"
)

func TestKeysymToString(t *testing.T) {
	tests := []struct {
		sym  Keysym
		want string
	}{
		{KeysymSpace, " "},
		{Keysyma, "a"},
		{KeysymA, "A"},
		{Keysym0, "0"},
		{KeysymBackSpace, ""},  // Non-printable
		{KeysymReturn, ""},     // Non-printable
		{KeysymF1, ""},         // Non-printable
		{0x00a9, "\u00a9"},     // Copyright symbol
		{0x01000041, "A"},      // Unicode keysym for 'A'
		{0x010003b1, "\u03b1"}, // Unicode keysym for Greek alpha
	}

	for _, tt := range tests {
		t.Run(KeysymName(tt.sym), func(t *testing.T) {
			got := KeysymToString(tt.sym)
			if got != tt.want {
				t.Errorf("KeysymToString(%x): got %q, want %q", tt.sym, got, tt.want)
			}
		})
	}
}

func TestKeysymName(t *testing.T) {
	tests := []struct {
		sym  Keysym
		want string
	}{
		{KeysymBackSpace, "BackSpace"},
		{KeysymTab, "Tab"},
		{KeysymReturn, "Return"},
		{KeysymEscape, "Escape"},
		{KeysymDelete, "Delete"},
		{KeysymHome, "Home"},
		{KeysymLeft, "Left"},
		{KeysymUp, "Up"},
		{KeysymRight, "Right"},
		{KeysymDown, "Down"},
		{KeysymF1, "F1"},
		{KeysymF12, "F12"},
		{KeysymShiftL, "Shift"},
		{KeysymControlL, "Control"},
		{KeysymAltL, "Alt"},
		{KeysymSuperL, "Super"},
		{KeysymSpace, "Space"},
		{Keysyma, "a"},
		{KeysymA, "A"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := KeysymName(tt.sym)
			if got != tt.want {
				t.Errorf("KeysymName(%x): got %q, want %q", tt.sym, got, tt.want)
			}
		})
	}
}

func TestIsLetter(t *testing.T) {
	tests := []struct {
		sym  Keysym
		want bool
	}{
		{Keysyma, true},
		{Keysymz, true},
		{KeysymA, true},
		{KeysymZ, true},
		{Keysym0, false},
		{KeysymSpace, false},
		{KeysymF1, false},
	}

	for _, tt := range tests {
		t.Run(KeysymName(tt.sym), func(t *testing.T) {
			got := isLetter(tt.sym)
			if got != tt.want {
				t.Errorf("isLetter(%x): got %v, want %v", tt.sym, got, tt.want)
			}
		})
	}
}

func TestKeyboardMapping_KeycodeToKeysym(t *testing.T) {
	// Create a simple keyboard mapping
	km := &KeyboardMapping{
		MinKeycode:     8,
		MaxKeycode:     11,
		KeysymsPerCode: 2,
		Keysyms: []Keysym{
			// Keycode 8: a, A
			Keysyma, KeysymA,
			// Keycode 9: b, B
			Keysymb, KeysymB,
			// Keycode 10: 1, !
			Keysym1, KeysymExclam,
			// Keycode 11: space, space
			KeysymSpace, KeysymSpace,
		},
	}

	tests := []struct {
		name     string
		keycode  uint8
		shift    bool
		capsLock bool
		want     Keysym
	}{
		{"a normal", 8, false, false, Keysyma},
		{"a shift", 8, true, false, KeysymA},
		{"a caps", 8, false, true, KeysymA},
		{"a shift+caps", 8, true, true, Keysyma}, // Shift + Caps = lowercase
		{"b normal", 9, false, false, Keysymb},
		{"1 normal", 10, false, false, Keysym1},
		{"1 shift", 10, true, false, KeysymExclam},
		{"1 caps", 10, false, true, Keysym1}, // Caps doesn't affect numbers
		{"space", 11, false, false, KeysymSpace},
		{"space shift", 11, true, false, KeysymSpace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysym(tt.keycode, tt.shift, tt.capsLock)
			if got != tt.want {
				t.Errorf("KeycodeToKeysym(%d, %v, %v): got %x, want %x",
					tt.keycode, tt.shift, tt.capsLock, got, tt.want)
			}
		})
	}
}

func TestKeyboardMapping_KeycodeOutOfRange(t *testing.T) {
	km := &KeyboardMapping{
		MinKeycode:     8,
		MaxKeycode:     10,
		KeysymsPerCode: 2,
		Keysyms:        []Keysym{Keysyma, KeysymA, Keysymb, KeysymB, Keysymc, KeysymC},
	}

	tests := []struct {
		name    string
		keycode uint8
	}{
		{"below min", 5},
		{"above max", 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysym(tt.keycode, false, false)
			if got != KeysymVoidSymbol {
				t.Errorf("KeycodeToKeysym(%d): got %x, want KeysymVoidSymbol", tt.keycode, got)
			}
		})
	}
}
