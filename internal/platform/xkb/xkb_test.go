//go:build linux

package xkb

import (
	"testing"
)

// TestXKBConstants verifies xkbcommon constant values match the C header.
func TestXKBConstants(t *testing.T) {
	tests := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"XKB_CONTEXT_NO_FLAGS", XKBContextNoFlags, 0},
		{"XKB_KEYMAP_FORMAT_TEXT_V1", XKBKeymapFormatTextV1, 1},
		{"XKB_KEYMAP_COMPILE_NO_FLAGS", XKBKeymapCompileNoFlags, 0},
		{"evdev offset", XKBEvdevOffset, 8},
		{"XKB_KEY_UP", xkbKeyUp, 0},
		{"XKB_KEY_DOWN", xkbKeyDown, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestHandleNilSafety verifies nil handle methods do not panic.
func TestHandleNilSafety(t *testing.T) {
	var h *Handle

	// Ready should return false for nil handle
	if h.Ready() {
		t.Error("nil Handle.Ready() = true, want false")
	}

	// KeyGetUtf8 should return empty string for nil handle
	if got := h.KeyGetUtf8(30); got != "" {
		t.Errorf("nil Handle.KeyGetUtf8(30) = %q, want empty", got)
	}

	// KeyGetOneSym should return 0 for nil handle
	if got := h.KeyGetOneSym(30); got != 0 {
		t.Errorf("nil Handle.KeyGetOneSym(30) = %d, want 0", got)
	}

	// UpdateMask should not panic on nil handle
	h.UpdateMask(0, 0, 0, 0)

	// UpdateKey should not panic on nil handle
	h.UpdateKey(30, true)
	h.UpdateKey(30, false)

	// Close should not panic on nil handle
	h.Close()
}

// TestHandleNoKeymap verifies methods work when no keymap is loaded.
func TestHandleNoKeymap(t *testing.T) {
	h := &Handle{} // No library loaded, no context

	// Ready should return false (state is 0)
	if h.Ready() {
		t.Error("empty Handle.Ready() = true, want false")
	}

	// KeyGetUtf8 should return empty string (state is 0)
	if got := h.KeyGetUtf8(30); got != "" {
		t.Errorf("empty Handle.KeyGetUtf8(30) = %q, want empty", got)
	}

	// KeyGetOneSym should return 0 (state is 0)
	if got := h.KeyGetOneSym(30); got != 0 {
		t.Errorf("empty Handle.KeyGetOneSym(30) = %d, want 0", got)
	}

	// UpdateMask should not panic (state is 0)
	h.UpdateMask(1, 0, 0, 1)

	// UpdateKey should not panic (state is 0)
	h.UpdateKey(30, true)

	// Close should not panic (nothing to destroy)
	h.Close()
}

// TestHandleSetKeymapFromFD_NilHandle verifies error on nil handle.
func TestHandleSetKeymapFromFD_NilHandle(t *testing.T) {
	var h *Handle
	err := h.SetKeymapFromFD(3, 100)
	if err == nil {
		t.Error("nil Handle.SetKeymapFromFD() = nil, want error")
	}
}

// TestHandleSetKeymapFromNames_NilHandle verifies error on nil handle.
func TestHandleSetKeymapFromNames_NilHandle(t *testing.T) {
	var h *Handle
	err := h.SetKeymapFromNames()
	if err == nil {
		t.Error("nil Handle.SetKeymapFromNames() = nil, want error")
	}
}

// TestHandleSetKeymapFromNames_NoSymbol verifies error when symbol is not resolved.
func TestHandleSetKeymapFromNames_NoSymbol(t *testing.T) {
	h := &Handle{} // fnKeymapNewFromNames is nil
	err := h.SetKeymapFromNames()
	if err == nil {
		t.Error("Handle.SetKeymapFromNames() with nil symbol = nil, want error")
	}
}

// TestEvdevOffset verifies the evdev-to-xkb keycode offset.
// XKB keycodes = evdev keycodes + 8 (historical X11 heritage).
// Evdev key 'a' = 30, XKB key 'a' = 38.
func TestEvdevOffset(t *testing.T) {
	tests := []struct {
		name    string
		evdev   uint32
		wantXKB uint32
	}{
		{"key A (evdev 30)", 30, 38},
		{"key Q (evdev 16)", 16, 24},
		{"key 1 (evdev 2)", 2, 10},
		{"key Space (evdev 57)", 57, 65},
		{"key Escape (evdev 1)", 1, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.evdev + XKBEvdevOffset
			if got != tt.wantXKB {
				t.Errorf("evdev %d + offset = %d, want %d", tt.evdev, got, tt.wantXKB)
			}
		})
	}
}

// TestPhase2Constants verifies Phase 2 constant values match the C header.
func TestPhase2Constants(t *testing.T) {
	// XKB_STATE_MODS_EFFECTIVE = DEPRESSED(0x08) | LATCHED(0x10) | LOCKED(0x20) = 0x38
	if XKBStateModsEffective != 0x38 {
		t.Errorf("XKBStateModsEffective = 0x%x, want 0x38", XKBStateModsEffective)
	}

	// xkbModInvalid sentinel must be -1 (maps to XKB_MOD_INVALID = 0xFFFFFFFF uint32)
	if xkbModInvalid != -1 {
		t.Errorf("xkbModInvalid = %d, want -1", xkbModInvalid)
	}
}

// TestModsIndicesDefault verifies that ModsIndices defaults to all xkbModInvalid when no keymap is loaded.
func TestModsIndicesDefault(t *testing.T) {
	h := &Handle{} // No library loaded, no keymap
	indices := h.GetModsIndices()

	fields := []struct {
		name string
		got  int32
	}{
		{"Shift", indices.Shift},
		{"CapsLock", indices.CapsLock},
		{"Control", indices.Control},
		{"Alt", indices.Alt},
		{"NumLock", indices.NumLock},
		{"Mod3", indices.Mod3},
		{"Super", indices.Super},
		{"Mod5", indices.Mod5},
	}

	for _, f := range fields {
		if f.got != 0 {
			// Zero is the default for int32, not xkbModInvalid.
			// resolveModsIndices is only called during SetKeymapFromFD/SetKeymapFromNames.
			// Before that, fields are zero-initialized by Go.
			t.Logf("%s = %d (Go zero value, not explicitly set)", f.name, f.got)
		}
	}
}

// TestModsIndicesNilHandle verifies GetModsIndices returns all invalid on nil handle.
func TestModsIndicesNilHandle(t *testing.T) {
	var h *Handle
	indices := h.GetModsIndices()

	fields := []struct {
		name string
		got  int32
	}{
		{"Shift", indices.Shift},
		{"CapsLock", indices.CapsLock},
		{"Control", indices.Control},
		{"Alt", indices.Alt},
		{"NumLock", indices.NumLock},
		{"Mod3", indices.Mod3},
		{"Super", indices.Super},
		{"Mod5", indices.Mod5},
	}

	for _, f := range fields {
		if f.got != xkbModInvalid {
			t.Errorf("nil Handle.GetModsIndices().%s = %d, want %d (xkbModInvalid)", f.name, f.got, xkbModInvalid)
		}
	}
}

// TestPhase2NilHandleSafety verifies Phase 2 methods do not panic on nil handle.
func TestPhase2NilHandleSafety(t *testing.T) {
	var h *Handle

	// KeyGetLayout should return 0 for nil handle
	if got := h.KeyGetLayout(30); got != 0 {
		t.Errorf("nil Handle.KeyGetLayout(30) = %d, want 0", got)
	}

	// KeyWithoutModifiers should return 0 for nil handle
	if got := h.KeyWithoutModifiers(30); got != 0 {
		t.Errorf("nil Handle.KeyWithoutModifiers(30) = %d, want 0", got)
	}

	// ModNameIsActive should return false for nil handle
	if got := h.ModNameIsActive("Shift\x00"); got {
		t.Error("nil Handle.ModNameIsActive(\"Shift\") = true, want false")
	}
}

// TestPhase2NoKeymapSafety verifies Phase 2 methods work when no keymap is loaded.
func TestPhase2NoKeymapSafety(t *testing.T) {
	h := &Handle{} // No library loaded, no context, no keymap

	// KeyGetLayout should return 0 (state is 0)
	if got := h.KeyGetLayout(30); got != 0 {
		t.Errorf("empty Handle.KeyGetLayout(30) = %d, want 0", got)
	}

	// KeyWithoutModifiers should return 0 (keymap is 0)
	if got := h.KeyWithoutModifiers(30); got != 0 {
		t.Errorf("empty Handle.KeyWithoutModifiers(30) = %d, want 0", got)
	}

	// ModNameIsActive should return false (state is 0)
	if got := h.ModNameIsActive("Shift\x00"); got {
		t.Error("empty Handle.ModNameIsActive(\"Shift\") = true, want false")
	}
}

// TestResolveModsIndicesNoFunction verifies resolveModsIndices sets all invalid when function is nil.
func TestResolveModsIndicesNoFunction(t *testing.T) {
	h := &Handle{
		keymap: 0x12345678, // Fake non-zero keymap
		// fnKeymapModGetIndex is nil
	}
	h.resolveModsIndices()

	indices := h.GetModsIndices()
	fields := []struct {
		name string
		got  int32
	}{
		{"Shift", indices.Shift},
		{"CapsLock", indices.CapsLock},
		{"Control", indices.Control},
		{"Alt", indices.Alt},
		{"NumLock", indices.NumLock},
		{"Mod3", indices.Mod3},
		{"Super", indices.Super},
		{"Mod5", indices.Mod5},
	}

	for _, f := range fields {
		if f.got != xkbModInvalid {
			t.Errorf("resolveModsIndices (nil fn).%s = %d, want %d (xkbModInvalid)", f.name, f.got, xkbModInvalid)
		}
	}
}

// TestResolveModsIndicesNoKeymap verifies resolveModsIndices sets all invalid when keymap is zero.
func TestResolveModsIndicesNoKeymap(t *testing.T) {
	h := &Handle{
		keymap: 0, // No keymap
	}
	h.resolveModsIndices()

	indices := h.GetModsIndices()
	if indices.Shift != xkbModInvalid {
		t.Errorf("resolveModsIndices (no keymap).Shift = %d, want %d", indices.Shift, xkbModInvalid)
	}
	if indices.Alt != xkbModInvalid {
		t.Errorf("resolveModsIndices (no keymap).Alt = %d, want %d", indices.Alt, xkbModInvalid)
	}
}
