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

// ---------------------------------------------------------------------------
// FEAT-INPUT-020: Multi-Keyboard Layout Support (TDD Red Phase)
//
// These tests define the API contract for multi-keyboard layout support.
// They will FAIL with the current implementation because:
//   - KeycodeToKeysymGroup does not exist yet
//   - KeysymToString does not handle legacy Cyrillic keysyms
//   - KeysymToRune does not exist yet
//   - isLetter does not recognize Cyrillic letters
//
// Reference: ADR-027, gogpu#227
// ---------------------------------------------------------------------------

// Legacy Cyrillic keysym constants (from X11/keysymdef.h, XK_Cyrillic_*).
// These are defined inline for tests. The implementation will add them to
// keyboard.go as exported constants.
const (
	// Lowercase Cyrillic keysyms (XK_Cyrillic_*)
	testKeysymCyrillicA   Keysym = 0x6c1 // а U+0430
	testKeysymCyrillicBe  Keysym = 0x6c2 // б U+0431
	testKeysymCyrillicVe  Keysym = 0x6d7 // в U+0432
	testKeysymCyrillicGhe Keysym = 0x6c7 // г U+0433
	testKeysymCyrillicDe  Keysym = 0x6c4 // д U+0434
	testKeysymCyrillicIe  Keysym = 0x6c5 // е U+0435
	testKeysymCyrillicZhe Keysym = 0x6d6 // ж U+0436
	testKeysymCyrillicZe  Keysym = 0x6da // з U+0437
	testKeysymCyrillicI   Keysym = 0x6c9 // и U+0438
	testKeysymCyrillicSho Keysym = 0x6ca // й U+0439
	testKeysymCyrillicKa  Keysym = 0x6cb // к U+043A
	testKeysymCyrillicEl  Keysym = 0x6cc // л U+043B
	testKeysymCyrillicEm  Keysym = 0x6cd // м U+043C
	testKeysymCyrillicEn  Keysym = 0x6ce // н U+043D
	testKeysymCyrillicO   Keysym = 0x6cf // о U+043E
	testKeysymCyrillicPe  Keysym = 0x6d0 // п U+043F
	testKeysymCyrillicEr  Keysym = 0x6d2 // р U+0440
	testKeysymCyrillicEs  Keysym = 0x6d3 // с U+0441
	testKeysymCyrillicTe  Keysym = 0x6d4 // т U+0442
	testKeysymCyrillicU   Keysym = 0x6d5 // у U+0443
	testKeysymCyrillicEf  Keysym = 0x6c6 // ф U+0444
	testKeysymCyrillicHa  Keysym = 0x6c8 // х U+0445
	testKeysymCyrillicTse Keysym = 0x6c3 // ц U+0446
	testKeysymCyrillicChe Keysym = 0x6de // ч U+0447
	testKeysymCyrillicSha Keysym = 0x6db // ш U+0448
	testKeysymCyrillicSch Keysym = 0x6dd // щ U+0449
	testKeysymCyrillicHar Keysym = 0x6df // ъ U+044A
	testKeysymCyrillicYer Keysym = 0x6d9 // ы U+044B
	testKeysymCyrillicSof Keysym = 0x6d8 // ь U+044C
	testKeysymCyrillicE   Keysym = 0x6dc // э U+044D
	testKeysymCyrillicYu  Keysym = 0x6c0 // ю U+044E
	testKeysymCyrillicYa  Keysym = 0x6d1 // я U+044F

	// Uppercase Cyrillic keysyms
	testKeysymCyrillicAUp   Keysym = 0x6e1 // А U+0410
	testKeysymCyrillicBeUp  Keysym = 0x6e2 // Б U+0411
	testKeysymCyrillicVeUp  Keysym = 0x6f7 // В U+0412
	testKeysymCyrillicGheUp Keysym = 0x6e7 // Г U+0413
	testKeysymCyrillicDeUp  Keysym = 0x6e4 // Д U+0414
	testKeysymCyrillicIeUp  Keysym = 0x6e5 // Е U+0415
	testKeysymCyrillicEfUp  Keysym = 0x6e6 // Ф U+0424
	testKeysymCyrillicYuUp  Keysym = 0x6e0 // Ю U+042E
	testKeysymCyrillicYaUp  Keysym = 0x6f1 // Я U+042F

	// Special Cyrillic: IO (Ё/ё)
	testKeysymCyrillicIO   Keysym = 0x6b3 // Ё U+0401
	testKeysymCyrillicIoLo Keysym = 0x6a3 // ё U+0451
)

// buildDualLayoutMapping creates a KeyboardMapping simulating en+ru layout.
//
// KeysymsPerCode=4: [group1_base, group1_shift, group2_base, group2_shift]
//
// Layout matches standard QWERTY (en) / ЙЦУКЕН (ru) on physical key positions.
// Uses X11 keycodes (evdev + 8): 'A' key = evdev 38 → X11 keycode 46.
func buildDualLayoutMapping() *KeyboardMapping {
	// Simulate keycodes 38-41 (physical keys A, S, D, F)
	// X11 keycode = evdev + 8, but for test simplicity we use raw values.
	minKeycode := uint8(38)
	maxKeycode := uint8(43)

	// Each keycode has 4 keysyms: [en_base, en_shift, ru_base, ru_shift]
	keysyms := []Keysym{
		// Keycode 38: A key → en: a/A, ru: ф/Ф
		Keysyma, KeysymA, testKeysymCyrillicEf, testKeysymCyrillicEfUp,
		// Keycode 39: S key → en: s/S, ru: ы/Ы (using Unicode keysyms for variety)
		Keysyms, KeysymS, 0x0100044B, 0x0100042B,
		// Keycode 40: D key → en: d/D, ru: в/В
		Keysymd, KeysymD, testKeysymCyrillicVe, testKeysymCyrillicVeUp,
		// Keycode 41: F key → en: f/F, ru: а/А
		Keysymf, KeysymF, testKeysymCyrillicA, testKeysymCyrillicAUp,
		// Keycode 42: 1 key → en: 1/!, ru: 1/! (same on both groups)
		Keysym1, KeysymExclam, Keysym1, KeysymExclam,
		// Keycode 43: Space → same on both groups
		KeysymSpace, KeysymSpace, KeysymSpace, KeysymSpace,
	}

	return &KeyboardMapping{
		MinKeycode:     minKeycode,
		MaxKeycode:     maxKeycode,
		KeysymsPerCode: 4,
		Keysyms:        keysyms,
	}
}

// TestKeyboardMapping_KeycodeToKeysymGroup tests group-aware keysym lookup.
// This is the core test for FEAT-INPUT-020.
//
// X11 keyboard model: KeysymsPerCode columns are laid out as:
//
//	[group1_base, group1_shift, group2_base, group2_shift, ...]
//
// Each group occupies 2 columns (base + shift).
//
// KeycodeToKeysymGroup signature:
//
//	func (km *KeyboardMapping) KeycodeToKeysymGroup(keycode uint8, shift, capsLock bool, group int) Keysym
func TestKeyboardMapping_KeycodeToKeysymGroup(t *testing.T) {
	km := buildDualLayoutMapping()

	tests := []struct {
		name     string
		keycode  uint8
		shift    bool
		capsLock bool
		group    int
		want     Keysym
	}{
		// --- Group 0 (English) — same behavior as before ---
		{"en/a base", 38, false, false, 0, Keysyma},
		{"en/a shift", 38, true, false, 0, KeysymA},
		{"en/a capslock", 38, false, true, 0, KeysymA},
		{"en/a shift+caps", 38, true, true, 0, Keysyma},
		{"en/s base", 39, false, false, 0, Keysyms},
		{"en/d base", 40, false, false, 0, Keysymd},
		{"en/f shift", 41, true, false, 0, KeysymF},
		{"en/1 base", 42, false, false, 0, Keysym1},
		{"en/1 shift", 42, true, false, 0, KeysymExclam},
		{"en/1 capslock", 42, false, true, 0, Keysym1}, // CapsLock does not affect digits
		{"en/space", 43, false, false, 0, KeysymSpace},

		// --- Group 1 (Russian) — the NEW behavior ---
		{"ru/a→ф base", 38, false, false, 1, testKeysymCyrillicEf},
		{"ru/a→ф shift", 38, true, false, 1, testKeysymCyrillicEfUp},
		{"ru/s→ы base (unicode keysym)", 39, false, false, 1, 0x0100044B},
		{"ru/s→ы shift (unicode keysym)", 39, true, false, 1, 0x0100042B},
		{"ru/d→в base", 40, false, false, 1, testKeysymCyrillicVe},
		{"ru/d→в shift", 40, true, false, 1, testKeysymCyrillicVeUp},
		{"ru/f→а base", 41, false, false, 1, testKeysymCyrillicA},
		{"ru/f→а shift", 41, true, false, 1, testKeysymCyrillicAUp},
		{"ru/1 same as en", 42, false, false, 1, Keysym1},
		{"ru/1 shift same as en", 42, true, false, 1, KeysymExclam},
		{"ru/space same as en", 43, false, false, 1, KeysymSpace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysymGroup(tt.keycode, tt.shift, tt.capsLock, tt.group)
			if got != tt.want {
				t.Errorf("KeycodeToKeysymGroup(keycode=%d, shift=%v, caps=%v, group=%d): got 0x%04x, want 0x%04x",
					tt.keycode, tt.shift, tt.capsLock, tt.group, got, tt.want)
			}
		})
	}
}

// TestKeyboardMapping_KeycodeToKeysymGroup_CapsLockCyrillic tests CapsLock
// behavior with Cyrillic letters. CapsLock should toggle case for Cyrillic
// the same way it does for Latin — this requires isLetter to recognize
// Cyrillic keysyms.
func TestKeyboardMapping_KeycodeToKeysymGroup_CapsLockCyrillic(t *testing.T) {
	km := buildDualLayoutMapping()

	tests := []struct {
		name     string
		keycode  uint8
		shift    bool
		capsLock bool
		group    int
		want     Keysym
	}{
		// CapsLock alone → uppercase (shifted keysym)
		{"ru/ф caps → Ф", 38, false, true, 1, testKeysymCyrillicEfUp},
		{"ru/а caps → А", 41, false, true, 1, testKeysymCyrillicAUp},
		{"ru/в caps → В", 40, false, true, 1, testKeysymCyrillicVeUp},

		// Shift + CapsLock → lowercase (cancel out)
		{"ru/ф shift+caps → ф", 38, true, true, 1, testKeysymCyrillicEf},
		{"ru/а shift+caps → а", 41, true, true, 1, testKeysymCyrillicA},

		// CapsLock on digits is no-op (same for Russian layout)
		{"ru/1 caps unchanged", 42, false, true, 1, Keysym1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysymGroup(tt.keycode, tt.shift, tt.capsLock, tt.group)
			if got != tt.want {
				t.Errorf("KeycodeToKeysymGroup(keycode=%d, shift=%v, caps=%v, group=%d): got 0x%04x, want 0x%04x",
					tt.keycode, tt.shift, tt.capsLock, tt.group, got, tt.want)
			}
		})
	}
}

// TestKeyboardMapping_KeycodeToKeysymGroup_GroupOutOfRange tests that a group
// index beyond what the mapping supports falls back to group 0.
func TestKeyboardMapping_KeycodeToKeysymGroup_GroupOutOfRange(t *testing.T) {
	km := buildDualLayoutMapping() // KeysymsPerCode=4 → supports groups 0 and 1

	tests := []struct {
		name  string
		group int
		want  Keysym
	}{
		{"group 2 (out of range) → fallback to group 0", 2, Keysyma},
		{"group 99 → fallback to group 0", 99, Keysyma},
		{"negative group → fallback to group 0", -1, Keysyma},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysymGroup(38, false, false, tt.group)
			if got != tt.want {
				t.Errorf("KeycodeToKeysymGroup(keycode=38, group=%d): got 0x%04x, want 0x%04x (group 0 fallback)",
					tt.group, got, tt.want)
			}
		})
	}
}

// TestKeyboardMapping_KeycodeToKeysymGroup_SingleLayout tests that when
// KeysymsPerCode=2 (single layout, no group 2), the group parameter is
// effectively ignored and group 0 is always used.
func TestKeyboardMapping_KeycodeToKeysymGroup_SingleLayout(t *testing.T) {
	km := &KeyboardMapping{
		MinKeycode:     8,
		MaxKeycode:     9,
		KeysymsPerCode: 2,
		Keysyms: []Keysym{
			Keysyma, KeysymA, // keycode 8
			Keysymb, KeysymB, // keycode 9
		},
	}

	tests := []struct {
		name    string
		keycode uint8
		group   int
		want    Keysym
	}{
		{"group 0", 8, 0, Keysyma},
		{"group 1 (no data) → fallback to group 0", 8, 1, Keysyma},
		{"group 0 keycode 9", 9, 0, Keysymb},
		{"group 1 keycode 9 → fallback", 9, 1, Keysymb},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysymGroup(tt.keycode, false, false, tt.group)
			if got != tt.want {
				t.Errorf("KeycodeToKeysymGroup(keycode=%d, group=%d): got 0x%04x, want 0x%04x",
					tt.keycode, tt.group, got, tt.want)
			}
		})
	}
}

// TestKeyboardMapping_KeycodeToKeysymGroup_EmptyMapping tests edge case with
// zero-length keysym array.
func TestKeyboardMapping_KeycodeToKeysymGroup_EmptyMapping(t *testing.T) {
	km := &KeyboardMapping{
		MinKeycode:     8,
		MaxKeycode:     10,
		KeysymsPerCode: 4,
		Keysyms:        []Keysym{}, // empty
	}

	got := km.KeycodeToKeysymGroup(8, false, false, 0)
	if got != KeysymVoidSymbol {
		t.Errorf("empty mapping: got 0x%04x, want KeysymVoidSymbol", got)
	}

	got = km.KeycodeToKeysymGroup(8, false, false, 1)
	if got != KeysymVoidSymbol {
		t.Errorf("empty mapping group 1: got 0x%04x, want KeysymVoidSymbol", got)
	}
}

// TestKeyboardMapping_KeycodeToKeysymGroup_ThreeLayouts tests 3 layouts
// (KeysymsPerCode=6: 3 groups x 2 columns each).
func TestKeyboardMapping_KeycodeToKeysymGroup_ThreeLayouts(t *testing.T) {
	// Simulate en + ru + de (German) for the 'Y' key:
	// en: y/Y, ru: н/Н, de: z/Z (QWERTZ layout)
	km := &KeyboardMapping{
		MinKeycode:     52,
		MaxKeycode:     52,
		KeysymsPerCode: 6,
		Keysyms: []Keysym{
			Keysymy, KeysymY, // group 0: en
			testKeysymCyrillicEn, 0x6ee, // group 1: ru (н/Н, 0x6ee = XK_Cyrillic_EN)
			Keysymz, KeysymZ, // group 2: de
		},
	}

	tests := []struct {
		name  string
		group int
		shift bool
		want  Keysym
	}{
		{"en base", 0, false, Keysymy},
		{"en shift", 0, true, KeysymY},
		{"ru base", 1, false, testKeysymCyrillicEn},
		{"ru shift", 1, true, 0x6ee},
		{"de base", 2, false, Keysymz},
		{"de shift", 2, true, KeysymZ},
		{"group 3 (out of range) → fallback group 0", 3, false, Keysymy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := km.KeycodeToKeysymGroup(52, tt.shift, false, tt.group)
			if got != tt.want {
				t.Errorf("3-layout KeycodeToKeysymGroup(group=%d, shift=%v): got 0x%04x, want 0x%04x",
					tt.group, tt.shift, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// KeysymToString: Cyrillic support
// ---------------------------------------------------------------------------

// TestKeysymToString_Cyrillic tests that legacy Cyrillic keysyms (0x6a0-0x6ff)
// are converted to their Unicode string representation.
// Current implementation handles Latin-1 and Unicode keysyms, but NOT legacy
// Cyrillic — these tests will FAIL until the conversion table is added.
func TestKeysymToString_Cyrillic(t *testing.T) {
	tests := []struct {
		name string
		sym  Keysym
		want string
	}{
		// Lowercase Cyrillic
		{"а (0x6c1)", testKeysymCyrillicA, "а"},
		{"б (0x6c2)", testKeysymCyrillicBe, "б"},
		{"в (0x6d7)", testKeysymCyrillicVe, "в"},
		{"г (0x6c7)", testKeysymCyrillicGhe, "г"},
		{"д (0x6c4)", testKeysymCyrillicDe, "д"},
		{"е (0x6c5)", testKeysymCyrillicIe, "е"},
		{"ф (0x6c6)", testKeysymCyrillicEf, "ф"},
		{"ю (0x6c0)", testKeysymCyrillicYu, "ю"},
		{"я (0x6d1)", testKeysymCyrillicYa, "я"},
		{"ж (0x6d6)", testKeysymCyrillicZhe, "ж"},
		{"з (0x6da)", testKeysymCyrillicZe, "з"},
		{"и (0x6c9)", testKeysymCyrillicI, "и"},
		{"й (0x6ca)", testKeysymCyrillicSho, "й"},
		{"к (0x6cb)", testKeysymCyrillicKa, "к"},
		{"л (0x6cc)", testKeysymCyrillicEl, "л"},
		{"м (0x6cd)", testKeysymCyrillicEm, "м"},
		{"н (0x6ce)", testKeysymCyrillicEn, "н"},
		{"о (0x6cf)", testKeysymCyrillicO, "о"},
		{"п (0x6d0)", testKeysymCyrillicPe, "п"},
		{"р (0x6d2)", testKeysymCyrillicEr, "р"},
		{"с (0x6d3)", testKeysymCyrillicEs, "с"},
		{"т (0x6d4)", testKeysymCyrillicTe, "т"},
		{"у (0x6d5)", testKeysymCyrillicU, "у"},
		{"х (0x6c8)", testKeysymCyrillicHa, "х"},
		{"ц (0x6c3)", testKeysymCyrillicTse, "ц"},
		{"ч (0x6de)", testKeysymCyrillicChe, "ч"},
		{"ш (0x6db)", testKeysymCyrillicSha, "ш"},
		{"щ (0x6dd)", testKeysymCyrillicSch, "щ"},
		{"ъ (0x6df)", testKeysymCyrillicHar, "ъ"},
		{"ы (0x6d9)", testKeysymCyrillicYer, "ы"},
		{"ь (0x6d8)", testKeysymCyrillicSof, "ь"},
		{"э (0x6dc)", testKeysymCyrillicE, "э"},

		// Uppercase Cyrillic
		{"А (0x6e1)", testKeysymCyrillicAUp, "А"},
		{"Б (0x6e2)", testKeysymCyrillicBeUp, "Б"},
		{"В (0x6f7)", testKeysymCyrillicVeUp, "В"},
		{"Г (0x6e7)", testKeysymCyrillicGheUp, "Г"},
		{"Д (0x6e4)", testKeysymCyrillicDeUp, "Д"},
		{"Е (0x6e5)", testKeysymCyrillicIeUp, "Е"},
		{"Ф (0x6e6)", testKeysymCyrillicEfUp, "Ф"},
		{"Ю (0x6e0)", testKeysymCyrillicYuUp, "Ю"},
		{"Я (0x6f1)", testKeysymCyrillicYaUp, "Я"},

		// Special: Ё/ё
		{"Ё (0x6b3)", testKeysymCyrillicIO, "Ё"},
		{"ё (0x6a3)", testKeysymCyrillicIoLo, "ё"},

		// Unicode Cyrillic keysyms (should ALREADY work via 0x01000000+ path)
		{"ф unicode (0x01000444)", 0x01000444, "ф"},
		{"Ф unicode (0x01000424)", 0x01000424, "Ф"},
		{"ё unicode (0x01000451)", 0x01000451, "ё"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeysymToString(tt.sym)
			if got != tt.want {
				t.Errorf("KeysymToString(0x%04x): got %q, want %q", tt.sym, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// KeysymToRune: complete keysym → rune conversion
// ---------------------------------------------------------------------------

// TestKeysymToRune tests the new KeysymToRune function that converts any
// keysym to a Unicode rune. This covers:
//   - Latin-1 (0x20-0xff) → same codepoint
//   - Legacy Cyrillic (0x6a0-0x6ff) → Unicode U+0400-U+04FF via lookup table
//   - Unicode keysyms (0x01000000+) → direct subtraction
//   - Non-printable (function keys, modifiers) → (0, false)
func TestKeysymToRune(t *testing.T) {
	tests := []struct {
		name   string
		sym    Keysym
		want   rune
		wantOK bool
	}{
		// Latin-1 printable
		{"space", KeysymSpace, ' ', true},
		{"a", Keysyma, 'a', true},
		{"A", KeysymA, 'A', true},
		{"z", Keysymz, 'z', true},
		{"0", Keysym0, '0', true},
		{"!", KeysymExclam, '!', true},
		{"~", KeysymASCIITilde, '~', true},
		{"copyright (0xa9)", 0x00a9, '\u00a9', true},
		{"umlaut-u (0xfc)", 0x00fc, '\u00fc', true},

		// Legacy Cyrillic (must be converted via lookup table)
		{"Cyrillic а", testKeysymCyrillicA, 'а', true},    // U+0430
		{"Cyrillic б", testKeysymCyrillicBe, 'б', true},   // U+0431
		{"Cyrillic ф", testKeysymCyrillicEf, 'ф', true},   // U+0444
		{"Cyrillic А", testKeysymCyrillicAUp, 'А', true},  // U+0410
		{"Cyrillic Ф", testKeysymCyrillicEfUp, 'Ф', true}, // U+0424
		{"Cyrillic ё", testKeysymCyrillicIoLo, 'ё', true}, // U+0451
		{"Cyrillic Ё", testKeysymCyrillicIO, 'Ё', true},   // U+0401
		{"Cyrillic ю", testKeysymCyrillicYu, 'ю', true},   // U+044E
		{"Cyrillic Ю", testKeysymCyrillicYuUp, 'Ю', true}, // U+042E
		{"Cyrillic я", testKeysymCyrillicYa, 'я', true},   // U+044F
		{"Cyrillic Я", testKeysymCyrillicYaUp, 'Я', true}, // U+042F

		// Unicode keysyms (0x01000000 + codepoint)
		{"unicode а", 0x01000430, 'а', true},
		{"unicode Ф", 0x01000424, 'Ф', true},
		{"unicode Greek alpha", 0x010003B1, 'α', true},
		{"unicode CJK", 0x01004E2D, '中', true},
		{"unicode emoji", 0x0101F600, '😀', true},

		// Non-printable keysyms → (0, false)
		{"BackSpace", KeysymBackSpace, 0, false},
		{"Return", KeysymReturn, 0, false},
		{"Escape", KeysymEscape, 0, false},
		{"F1", KeysymF1, 0, false},
		{"Shift_L", KeysymShiftL, 0, false},
		{"Control_L", KeysymControlL, 0, false},
		{"VoidSymbol", KeysymVoidSymbol, 0, false},

		// Edge cases
		{"null keysym (0x0000)", 0, 0, false},
		{"DEL (0x7f)", 0x7f, 0, false},                        // DEL is not printable
		{"gap between Latin-1 ranges (0x80)", 0x80, 0, false}, // 0x80-0x9f not printable in Latin-1
		{"gap (0x9f)", 0x9f, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := KeysymToRune(tt.sym)
			if ok != tt.wantOK {
				t.Errorf("KeysymToRune(0x%04x): ok=%v, want ok=%v", tt.sym, ok, tt.wantOK)
				return
			}
			if r != tt.want {
				t.Errorf("KeysymToRune(0x%04x): got %q (U+%04X), want %q (U+%04X)",
					tt.sym, r, r, tt.want, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isLetter: Cyrillic letter recognition
// ---------------------------------------------------------------------------

// TestIsLetter_Cyrillic tests that isLetter recognizes Cyrillic letters
// for proper CapsLock handling. Current implementation only checks Latin a-z/A-Z.
func TestIsLetter_Cyrillic(t *testing.T) {
	tests := []struct {
		name string
		sym  Keysym
		want bool
	}{
		// Latin letters (should still work)
		{"Latin a", Keysyma, true},
		{"Latin Z", KeysymZ, true},

		// Legacy Cyrillic lowercase — must be recognized as letters
		{"Cyrillic а (0x6c1)", testKeysymCyrillicA, true},
		{"Cyrillic ф (0x6c6)", testKeysymCyrillicEf, true},
		{"Cyrillic ю (0x6c0)", testKeysymCyrillicYu, true},
		{"Cyrillic я (0x6d1)", testKeysymCyrillicYa, true},
		{"Cyrillic ё (0x6a3)", testKeysymCyrillicIoLo, true},

		// Legacy Cyrillic uppercase — must be recognized as letters
		{"Cyrillic А (0x6e1)", testKeysymCyrillicAUp, true},
		{"Cyrillic Ф (0x6e6)", testKeysymCyrillicEfUp, true},
		{"Cyrillic Ю (0x6e0)", testKeysymCyrillicYuUp, true},
		{"Cyrillic Ё (0x6b3)", testKeysymCyrillicIO, true},

		// Unicode Cyrillic keysyms — also letters
		{"Unicode а (0x01000430)", 0x01000430, true},
		{"Unicode Ф (0x01000424)", 0x01000424, true},
		{"Unicode ё (0x01000451)", 0x01000451, true},

		// Non-letters (must remain false)
		{"digit 0", Keysym0, false},
		{"space", KeysymSpace, false},
		{"exclamation", KeysymExclam, false},
		{"F1 key", KeysymF1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLetter(tt.sym)
			if got != tt.want {
				t.Errorf("isLetter(0x%04x): got %v, want %v", tt.sym, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End-to-end: keycode → rune pipeline for Russian layout
// ---------------------------------------------------------------------------

// TestEndToEnd_KeycodeToRune_RussianLayout tests the complete pipeline:
// keycode → KeycodeToKeysymGroup → KeysymToRune for Russian layout.
// This simulates what happens when a user types on a Russian keyboard layout.
func TestEndToEnd_KeycodeToRune_RussianLayout(t *testing.T) {
	km := buildDualLayoutMapping()

	tests := []struct {
		name     string
		keycode  uint8
		shift    bool
		capsLock bool
		group    int
		wantRune rune
		wantOK   bool
	}{
		// Physical 'A' key, Russian layout → ф
		{"A key, group 1, base → ф", 38, false, false, 1, 'ф', true},
		// Physical 'A' key, Russian layout, shift → Ф
		{"A key, group 1, shift → Ф", 38, true, false, 1, 'Ф', true},
		// Physical 'A' key, English layout → a
		{"A key, group 0, base → a", 38, false, false, 0, 'a', true},
		// Physical 'A' key, English layout, shift → A
		{"A key, group 0, shift → A", 38, true, false, 0, 'A', true},
		// Physical 'F' key, Russian layout → а
		{"F key, group 1, base → а", 41, false, false, 1, 'а', true},
		// Physical 'F' key, Russian layout, shift → А
		{"F key, group 1, shift → А", 41, true, false, 1, 'А', true},
		// Physical '1' key, same on both layouts
		{"1 key, group 1, base → 1", 42, false, false, 1, '1', true},
		{"1 key, group 1, shift → !", 42, true, false, 1, '!', true},
		// Space is space regardless of layout
		{"space, group 0", 43, false, false, 0, ' ', true},
		{"space, group 1", 43, false, false, 1, ' ', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := km.KeycodeToKeysymGroup(tt.keycode, tt.shift, tt.capsLock, tt.group)
			r, ok := KeysymToRune(sym)
			if ok != tt.wantOK {
				t.Errorf("pipeline(keycode=%d, group=%d): KeysymToRune ok=%v, want ok=%v (keysym=0x%04x)",
					tt.keycode, tt.group, ok, tt.wantOK, sym)
				return
			}
			if r != tt.wantRune {
				t.Errorf("pipeline(keycode=%d, group=%d): got %q (U+%04X), want %q (U+%04X) (keysym=0x%04x)",
					tt.keycode, tt.group, r, r, tt.wantRune, tt.wantRune, sym)
			}
		})
	}
}

// TestEndToEnd_KeycodeToString_RussianLayout tests the complete pipeline
// using KeysymToString (which is what OnTextInput ultimately uses).
func TestEndToEnd_KeycodeToString_RussianLayout(t *testing.T) {
	km := buildDualLayoutMapping()

	tests := []struct {
		name    string
		keycode uint8
		shift   bool
		group   int
		want    string
	}{
		{"A→ф", 38, false, 1, "ф"},
		{"A→Ф (shift)", 38, true, 1, "Ф"},
		{"S→ы (unicode keysym)", 39, false, 1, "ы"},
		{"S→Ы (unicode shift)", 39, true, 1, "Ы"},
		{"D→в", 40, false, 1, "в"},
		{"D→В (shift)", 40, true, 1, "В"},
		{"F→а", 41, false, 1, "а"},
		{"F→А (shift)", 41, true, 1, "А"},
		// English layout for comparison
		{"A→a (en)", 38, false, 0, "a"},
		{"A→A (en shift)", 38, true, 0, "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := km.KeycodeToKeysymGroup(tt.keycode, tt.shift, false, tt.group)
			got := KeysymToString(sym)
			if got != tt.want {
				t.Errorf("KeysymToString(KeycodeToKeysymGroup(%d, shift=%v, group=%d)): got %q, want %q (keysym=0x%04x)",
					tt.keycode, tt.shift, tt.group, got, tt.want, sym)
			}
		})
	}
}
