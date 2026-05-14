//go:build linux

package x11

import (
	"fmt"
	"unicode"
)

// Keysym represents an X11 keysym.
type Keysym uint32

// Common keysyms (from X11/keysymdef.h).
const (
	KeysymVoidSymbol = 0xffffff

	// TTY function keys
	KeysymBackSpace  = 0xff08
	KeysymTab        = 0xff09
	KeysymLinefeed   = 0xff0a
	KeysymClear      = 0xff0b
	KeysymReturn     = 0xff0d
	KeysymPause      = 0xff13
	KeysymScrollLock = 0xff14
	KeysymSysReq     = 0xff15
	KeysymEscape     = 0xff1b
	KeysymDelete     = 0xffff

	// Cursor control & motion
	KeysymHome     = 0xff50
	KeysymLeft     = 0xff51
	KeysymUp       = 0xff52
	KeysymRight    = 0xff53
	KeysymDown     = 0xff54
	KeysymPageUp   = 0xff55
	KeysymPageDown = 0xff56
	KeysymEnd      = 0xff57
	KeysymBegin    = 0xff58

	// Misc functions
	KeysymSelect  = 0xff60
	KeysymPrint   = 0xff61
	KeysymExecute = 0xff62
	KeysymInsert  = 0xff63
	KeysymUndo    = 0xff65
	KeysymRedo    = 0xff66
	KeysymMenu    = 0xff67
	KeysymFind    = 0xff68
	KeysymCancel  = 0xff69
	KeysymHelp    = 0xff6a
	KeysymBreak   = 0xff6b
	KeysymNumLock = 0xff7f

	// Keypad
	KeysymKPSpace     = 0xff80
	KeysymKPTab       = 0xff89
	KeysymKPEnter     = 0xff8d
	KeysymKPF1        = 0xff91
	KeysymKPF2        = 0xff92
	KeysymKPF3        = 0xff93
	KeysymKPF4        = 0xff94
	KeysymKPHome      = 0xff95
	KeysymKPLeft      = 0xff96
	KeysymKPUp        = 0xff97
	KeysymKPRight     = 0xff98
	KeysymKPDown      = 0xff99
	KeysymKPPageUp    = 0xff9a
	KeysymKPPageDown  = 0xff9b
	KeysymKPEnd       = 0xff9c
	KeysymKPBegin     = 0xff9d
	KeysymKPInsert    = 0xff9e
	KeysymKPDelete    = 0xff9f
	KeysymKPEqual     = 0xffbd
	KeysymKPMultiply  = 0xffaa
	KeysymKPAdd       = 0xffab
	KeysymKPSeparator = 0xffac
	KeysymKPSubtract  = 0xffad
	KeysymKPDecimal   = 0xffae
	KeysymKPDivide    = 0xffaf
	KeysymKP0         = 0xffb0
	KeysymKP1         = 0xffb1
	KeysymKP2         = 0xffb2
	KeysymKP3         = 0xffb3
	KeysymKP4         = 0xffb4
	KeysymKP5         = 0xffb5
	KeysymKP6         = 0xffb6
	KeysymKP7         = 0xffb7
	KeysymKP8         = 0xffb8
	KeysymKP9         = 0xffb9

	// Function keys
	KeysymF1  = 0xffbe
	KeysymF2  = 0xffbf
	KeysymF3  = 0xffc0
	KeysymF4  = 0xffc1
	KeysymF5  = 0xffc2
	KeysymF6  = 0xffc3
	KeysymF7  = 0xffc4
	KeysymF8  = 0xffc5
	KeysymF9  = 0xffc6
	KeysymF10 = 0xffc7
	KeysymF11 = 0xffc8
	KeysymF12 = 0xffc9
	KeysymF13 = 0xffca
	KeysymF14 = 0xffcb
	KeysymF15 = 0xffcc
	KeysymF16 = 0xffcd
	KeysymF17 = 0xffce
	KeysymF18 = 0xffcf
	KeysymF19 = 0xffd0
	KeysymF20 = 0xffd1

	// Modifiers
	KeysymShiftL    = 0xffe1
	KeysymShiftR    = 0xffe2
	KeysymControlL  = 0xffe3
	KeysymControlR  = 0xffe4
	KeysymCapsLock  = 0xffe5
	KeysymShiftLock = 0xffe6
	KeysymMetaL     = 0xffe7
	KeysymMetaR     = 0xffe8
	KeysymAltL      = 0xffe9
	KeysymAltR      = 0xffea
	KeysymSuperL    = 0xffeb
	KeysymSuperR    = 0xffec
	KeysymHyperL    = 0xffed
	KeysymHyperR    = 0xffee

	// Latin-1
	KeysymSpace        = 0x0020
	KeysymExclam       = 0x0021
	KeysymQuoteDbl     = 0x0022
	KeysymNumberSign   = 0x0023
	KeysymDollar       = 0x0024
	KeysymPercent      = 0x0025
	KeysymAmpersand    = 0x0026
	KeysymApostrophe   = 0x0027
	KeysymParenLeft    = 0x0028
	KeysymParenRight   = 0x0029
	KeysymAsterisk     = 0x002a
	KeysymPlus         = 0x002b
	KeysymComma        = 0x002c
	KeysymMinus        = 0x002d
	KeysymPeriod       = 0x002e
	KeysymSlash        = 0x002f
	Keysym0            = 0x0030
	Keysym1            = 0x0031
	Keysym2            = 0x0032
	Keysym3            = 0x0033
	Keysym4            = 0x0034
	Keysym5            = 0x0035
	Keysym6            = 0x0036
	Keysym7            = 0x0037
	Keysym8            = 0x0038
	Keysym9            = 0x0039
	KeysymColon        = 0x003a
	KeysymSemicolon    = 0x003b
	KeysymLess         = 0x003c
	KeysymEqual        = 0x003d
	KeysymGreater      = 0x003e
	KeysymQuestion     = 0x003f
	KeysymAt           = 0x0040
	KeysymA            = 0x0041
	KeysymB            = 0x0042
	KeysymC            = 0x0043
	KeysymD            = 0x0044
	KeysymE            = 0x0045
	KeysymF            = 0x0046
	KeysymG            = 0x0047
	KeysymH            = 0x0048
	KeysymI            = 0x0049
	KeysymJ            = 0x004a
	KeysymK            = 0x004b
	KeysymL            = 0x004c
	KeysymM            = 0x004d
	KeysymN            = 0x004e
	KeysymO            = 0x004f
	KeysymP            = 0x0050
	KeysymQ            = 0x0051
	KeysymR            = 0x0052
	KeysymS            = 0x0053
	KeysymT            = 0x0054
	KeysymU            = 0x0055
	KeysymV            = 0x0056
	KeysymW            = 0x0057
	KeysymX            = 0x0058
	KeysymY            = 0x0059
	KeysymZ            = 0x005a
	KeysymBracketLeft  = 0x005b
	KeysymBackslash    = 0x005c
	KeysymBracketRight = 0x005d
	KeysymASCIICircum  = 0x005e
	KeysymUnderscore   = 0x005f
	KeysymGrave        = 0x0060
	Keysyma            = 0x0061
	Keysymb            = 0x0062
	Keysymc            = 0x0063
	Keysymd            = 0x0064
	Keysyme            = 0x0065
	Keysymf            = 0x0066
	Keysymg            = 0x0067
	Keysymh            = 0x0068
	Keysymi            = 0x0069
	Keysymj            = 0x006a
	Keysymk            = 0x006b
	Keysyml            = 0x006c
	Keysymm            = 0x006d
	Keysymn            = 0x006e
	Keysymo            = 0x006f
	Keysymp            = 0x0070
	Keysymq            = 0x0071
	Keysymr            = 0x0072
	Keysyms            = 0x0073
	Keysymt            = 0x0074
	Keysymu            = 0x0075
	Keysymv            = 0x0076
	Keysymw            = 0x0077
	Keysymx            = 0x0078
	Keysymy            = 0x0079
	Keysymz            = 0x007a
	KeysymBraceLeft    = 0x007b
	KeysymBar          = 0x007c
	KeysymBraceRight   = 0x007d
	KeysymASCIITilde   = 0x007e
)

// Modifier mask bits.
const (
	ModifierShift   uint16 = 1 << 0
	ModifierLock    uint16 = 1 << 1 // Caps Lock
	ModifierControl uint16 = 1 << 2
	ModifierMod1    uint16 = 1 << 3 // Usually Alt
	ModifierMod2    uint16 = 1 << 4 // Usually Num Lock
	ModifierMod3    uint16 = 1 << 5
	ModifierMod4    uint16 = 1 << 6 // Usually Super/Windows
	ModifierMod5    uint16 = 1 << 7 // Usually Mode_switch/AltGr
	ModifierButton1 uint16 = 1 << 8
	ModifierButton2 uint16 = 1 << 9
	ModifierButton3 uint16 = 1 << 10
	ModifierButton4 uint16 = 1 << 11
	ModifierButton5 uint16 = 1 << 12
)

// KeyboardMapping holds the keyboard mapping for a connection.
type KeyboardMapping struct {
	MinKeycode     uint8
	MaxKeycode     uint8
	KeysymsPerCode int
	Keysyms        []Keysym
}

// GetKeyboardMapping requests the keyboard mapping from the server.
func (c *Connection) GetKeyboardMapping() (*KeyboardMapping, error) {
	if c.setup == nil {
		return nil, fmt.Errorf("x11: not connected")
	}

	minKeycode := c.setup.MinKeycode
	maxKeycode := c.setup.MaxKeycode
	count := int(maxKeycode - minKeycode + 1)

	e := NewEncoder(c.byteOrder)
	e.PutUint8(OpcodeGetKeyboardMapping)
	e.PutUint8(0)  // unused
	e.PutUint16(2) // length
	e.PutUint8(minKeycode)
	e.PutUint8(uint8(count))
	e.PutUint16(0) // unused

	reply, err := c.sendRequestWithReply(e.Bytes())
	if err != nil {
		return nil, fmt.Errorf("x11: GetKeyboardMapping failed: %w", err)
	}

	// Parse reply
	// Reply: [1][keysyms_per_keycode:1][seq:2][length:4][unused:24][keysyms...]
	if len(reply) < 32 {
		return nil, fmt.Errorf("x11: GetKeyboardMapping reply too short")
	}

	keysymsPerCode := int(reply[1])

	// Calculate total keysyms
	totalKeysyms := count * keysymsPerCode

	// Read keysyms from after the 32-byte header
	keysyms := make([]Keysym, totalKeysyms)
	d := NewDecoder(c.byteOrder, reply[32:])
	for i := range keysyms {
		sym, decErr := d.Uint32()
		if decErr != nil {
			// Truncated data - return partial result with what we have.
			// This is acceptable as keyboard mapping may still work for common keys.
			break
		}
		keysyms[i] = Keysym(sym)
	}

	//nolint:nilerr // Intentional: return partial mapping if data is truncated
	return &KeyboardMapping{
		MinKeycode:     minKeycode,
		MaxKeycode:     maxKeycode,
		KeysymsPerCode: keysymsPerCode,
		Keysyms:        keysyms,
	}, nil
}

// KeycodeToKeysym converts a keycode to a keysym.
// group is typically 0 for the primary group, shift indicates shift state.
func (km *KeyboardMapping) KeycodeToKeysym(keycode uint8, shift, capsLock bool) Keysym {
	if keycode < km.MinKeycode || keycode > km.MaxKeycode {
		return KeysymVoidSymbol
	}

	// Calculate index into keysym array
	idx := int(keycode-km.MinKeycode) * km.KeysymsPerCode

	if idx >= len(km.Keysyms) {
		return KeysymVoidSymbol
	}

	// Get base keysym and shifted keysym
	baseSym := km.Keysyms[idx]
	var shiftedSym Keysym
	if km.KeysymsPerCode > 1 && idx+1 < len(km.Keysyms) {
		shiftedSym = km.Keysyms[idx+1]
	} else {
		shiftedSym = baseSym
	}

	// Handle shift and caps lock
	if shift {
		if capsLock && isLetter(baseSym) {
			return baseSym // Shift + Caps = lowercase
		}
		return shiftedSym
	}

	if capsLock && isLetter(baseSym) {
		return shiftedSym // Caps = uppercase for letters
	}

	return baseSym
}

// isLetter checks if a keysym is a letter (Latin, legacy Cyrillic, or Unicode Cyrillic).
func isLetter(sym Keysym) bool {
	if (sym >= Keysyma && sym <= Keysymz) || (sym >= KeysymA && sym <= KeysymZ) {
		return true
	}
	// Legacy Cyrillic: lowercase 0x6c0-0x6df, uppercase 0x6e0-0x6ff, special Ё 0x6b3, ё 0x6a3
	if (sym >= 0x6c0 && sym <= 0x6df) || (sym >= 0x6e0 && sym <= 0x6ff) || sym == 0x6b3 || sym == 0x6a3 {
		return true
	}
	// Unicode keysyms: check via unicode.Cyrillic table
	if sym >= 0x01000000 && sym <= 0x01ffffff {
		r := rune(sym - 0x01000000)
		return unicode.Is(unicode.Cyrillic, r)
	}
	return false
}

// KeysymToString converts a keysym to a printable string.
// Returns empty string for non-printable keysyms.
func KeysymToString(sym Keysym) string {
	// Latin-1 printable range
	if sym >= 0x20 && sym <= 0x7e {
		return string(rune(sym))
	}

	// Latin-1 extended range
	if sym >= 0xa0 && sym <= 0xff {
		return string(rune(sym))
	}

	// Legacy Cyrillic keysyms (0x6a0-0x6ff)
	if r, ok := legacyCyrillicToRune[sym]; ok {
		return string(r)
	}

	// Unicode keysyms (0x01000000 + unicode codepoint)
	if sym >= 0x01000000 && sym <= 0x01ffffff {
		return string(rune(sym - 0x01000000))
	}

	return ""
}

// KeysymName returns a human-readable name for a keysym.
//
//nolint:goconst // display names intentionally match constant names
func KeysymName(sym Keysym) string {
	switch sym {
	case KeysymBackSpace:
		return "BackSpace"
	case KeysymTab:
		return "Tab"
	case KeysymReturn:
		return "Return"
	case KeysymEscape:
		return "Escape"
	case KeysymDelete:
		return "Delete"
	case KeysymHome:
		return "Home"
	case KeysymLeft:
		return "Left"
	case KeysymUp:
		return "Up"
	case KeysymRight:
		return "Right"
	case KeysymDown:
		return "Down"
	case KeysymPageUp:
		return "PageUp"
	case KeysymPageDown:
		return "PageDown"
	case KeysymEnd:
		return "End"
	case KeysymInsert:
		return "Insert"
	case KeysymF1:
		return "F1"
	case KeysymF2:
		return "F2"
	case KeysymF3:
		return "F3"
	case KeysymF4:
		return "F4"
	case KeysymF5:
		return "F5"
	case KeysymF6:
		return "F6"
	case KeysymF7:
		return "F7"
	case KeysymF8:
		return "F8"
	case KeysymF9:
		return "F9"
	case KeysymF10:
		return "F10"
	case KeysymF11:
		return "F11"
	case KeysymF12:
		return "F12"
	case KeysymShiftL, KeysymShiftR:
		return "Shift"
	case KeysymControlL, KeysymControlR:
		return "Control"
	case KeysymAltL, KeysymAltR:
		return "Alt"
	case KeysymSuperL, KeysymSuperR:
		return "Super"
	case KeysymCapsLock:
		return "CapsLock"
	case KeysymNumLock:
		return "NumLock"
	case KeysymSpace:
		return "Space"
	default:
		// For printable characters
		if s := KeysymToString(sym); s != "" {
			return s
		}
		return fmt.Sprintf("0x%04x", sym)
	}
}

// KeycodeToKeysymGroup converts a keycode to a keysym for a given keyboard group.
// Group 0 uses columns 0,1; group 1 uses columns 2,3; group N uses columns N*2, N*2+1.
// If the group is out of range, falls back to group 0.
func (km *KeyboardMapping) KeycodeToKeysymGroup(keycode uint8, shift, capsLock bool, group int) Keysym {
	if keycode < km.MinKeycode || keycode > km.MaxKeycode {
		return KeysymVoidSymbol
	}

	baseIdx := int(keycode-km.MinKeycode) * km.KeysymsPerCode
	if baseIdx >= len(km.Keysyms) {
		return KeysymVoidSymbol
	}

	maxGroups := km.KeysymsPerCode / 2
	if group < 0 || group >= maxGroups {
		group = 0
	}

	colBase := baseIdx + group*2
	if colBase >= len(km.Keysyms) {
		colBase = baseIdx
	}

	baseSym := km.Keysyms[colBase]
	var shiftedSym Keysym
	if colBase+1 < len(km.Keysyms) {
		shiftedSym = km.Keysyms[colBase+1]
	} else {
		shiftedSym = baseSym
	}

	if shift {
		if capsLock && isLetter(baseSym) {
			return baseSym
		}
		return shiftedSym
	}

	if capsLock && isLetter(baseSym) {
		return shiftedSym
	}

	return baseSym
}

// KeysymToRune converts a keysym to a Unicode rune.
// Returns (0, false) for non-printable keysyms (function keys, modifiers, etc.).
func KeysymToRune(sym Keysym) (rune, bool) {
	if sym == 0 {
		return 0, false
	}

	// Latin-1 printable range (0x20-0x7e)
	if sym >= 0x20 && sym <= 0x7e {
		return rune(sym), true
	}

	// 0x7f (DEL) and 0x80-0x9f (C1 control characters) are not printable
	if sym >= 0x7f && sym <= 0x9f {
		return 0, false
	}

	// Latin-1 extended (0xa0-0xff)
	if sym >= 0xa0 && sym <= 0xff {
		return rune(sym), true
	}

	// Legacy Cyrillic keysyms (0x6a0-0x6ff)
	if r, ok := legacyCyrillicToRune[sym]; ok {
		return r, true
	}

	// Unicode keysyms (0x01000000 + codepoint)
	if sym >= 0x01000000 && sym <= 0x01ffffff {
		return rune(sym - 0x01000000), true
	}

	return 0, false
}

// legacyCyrillicToRune maps X11 legacy Cyrillic keysyms (XK_Cyrillic_*, 0x6a0-0x6ff)
// to their Unicode codepoints. The mapping is NOT a simple offset — it follows the
// scattered layout defined in X11/keysymdef.h.
var legacyCyrillicToRune = map[Keysym]rune{
	// Lowercase
	0x6c0: 0x044E, // ю
	0x6c1: 0x0430, // а
	0x6c2: 0x0431, // б
	0x6c3: 0x0446, // ц
	0x6c4: 0x0434, // д
	0x6c5: 0x0435, // е
	0x6c6: 0x0444, // ф
	0x6c7: 0x0433, // г
	0x6c8: 0x0445, // х
	0x6c9: 0x0438, // и
	0x6ca: 0x0439, // й
	0x6cb: 0x043A, // к
	0x6cc: 0x043B, // л
	0x6cd: 0x043C, // м
	0x6ce: 0x043D, // н
	0x6cf: 0x043E, // о
	0x6d0: 0x043F, // п
	0x6d1: 0x044F, // я
	0x6d2: 0x0440, // р
	0x6d3: 0x0441, // с
	0x6d4: 0x0442, // т
	0x6d5: 0x0443, // у
	0x6d6: 0x0436, // ж
	0x6d7: 0x0432, // в
	0x6d8: 0x044C, // ь
	0x6d9: 0x044B, // ы
	0x6da: 0x0437, // з
	0x6db: 0x0448, // ш
	0x6dc: 0x044D, // э
	0x6dd: 0x0449, // щ
	0x6de: 0x0447, // ч
	0x6df: 0x044A, // ъ

	// Uppercase
	0x6e0: 0x042E, // Ю
	0x6e1: 0x0410, // А
	0x6e2: 0x0411, // Б
	0x6e3: 0x0426, // Ц
	0x6e4: 0x0414, // Д
	0x6e5: 0x0415, // Е
	0x6e6: 0x0424, // Ф
	0x6e7: 0x0413, // Г
	0x6e8: 0x0425, // Х
	0x6e9: 0x0418, // И
	0x6ea: 0x0419, // Й
	0x6eb: 0x041A, // К
	0x6ec: 0x041B, // Л
	0x6ed: 0x041C, // М
	0x6ee: 0x041D, // Н
	0x6ef: 0x041E, // О
	0x6f0: 0x041F, // П
	0x6f1: 0x042F, // Я
	0x6f2: 0x0420, // Р
	0x6f3: 0x0421, // С
	0x6f4: 0x0422, // Т
	0x6f5: 0x0423, // У
	0x6f6: 0x0417, // З
	0x6f7: 0x0412, // В
	0x6f8: 0x042C, // Ь
	0x6f9: 0x042B, // Ы
	0x6fa: 0x0416, // Ж
	0x6fb: 0x0428, // Ш
	0x6fc: 0x042D, // Э
	0x6fd: 0x0429, // Щ
	0x6fe: 0x0427, // Ч
	0x6ff: 0x042A, // Ъ

	// Special: Ё/ё and Ukrainian/Belarusian
	0x6a3: 0x0451, // ё
	0x6b3: 0x0401, // Ё
	0x6a4: 0x0454, // є (Ukrainian)
	0x6b4: 0x0404, // Є (Ukrainian)
	0x6a6: 0x0456, // і (Ukrainian)
	0x6b6: 0x0406, // І (Ukrainian)
	0x6a7: 0x0457, // ї (Ukrainian)
	0x6b7: 0x0407, // Ї (Ukrainian)
	0x6a8: 0x045E, // ў (Belarusian)
	0x6b8: 0x040E, // Ў (Belarusian)
}
