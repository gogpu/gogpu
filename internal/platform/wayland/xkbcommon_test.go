//go:build linux

package wayland

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/xkb"
)

// TestXKBConstants verifies xkbcommon constant values match the C header.
// Constants are now defined in the shared xkb package.
func TestXKBConstants(t *testing.T) {
	tests := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"XKB_CONTEXT_NO_FLAGS", xkb.XKBContextNoFlags, 0},
		{"XKB_KEYMAP_FORMAT_TEXT_V1", xkb.XKBKeymapFormatTextV1, 1},
		{"XKB_KEYMAP_COMPILE_NO_FLAGS", xkb.XKBKeymapCompileNoFlags, 0},
		{"evdev offset", xkb.XKBEvdevOffset, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestXKBHandleNilSafety verifies nil handle methods do not panic.
func TestXKBHandleNilSafety(t *testing.T) {
	var h *XKBHandle

	// Ready should return false for nil handle
	if h.Ready() {
		t.Error("nil XKBHandle.Ready() = true, want false")
	}

	// KeyGetUtf8 should return empty string for nil handle
	if got := h.KeyGetUtf8(30); got != "" {
		t.Errorf("nil XKBHandle.KeyGetUtf8(30) = %q, want empty", got)
	}

	// UpdateMask should not panic on nil handle
	h.UpdateMask(0, 0, 0, 0, 0, 0)

	// Close should not panic on nil handle
	h.Close()
}

// TestXKBHandleNoKeymap verifies methods work when no keymap is loaded.
func TestXKBHandleNoKeymap(t *testing.T) {
	h := &XKBHandle{} // No library loaded, no context

	// Ready should return false (state is 0)
	if h.Ready() {
		t.Error("empty XKBHandle.Ready() = true, want false")
	}

	// KeyGetUtf8 should return empty string (state is 0)
	if got := h.KeyGetUtf8(30); got != "" {
		t.Errorf("empty XKBHandle.KeyGetUtf8(30) = %q, want empty", got)
	}

	// UpdateMask should not panic (state is 0)
	h.UpdateMask(1, 0, 0, 0, 0, 1)

	// Close should not panic (nothing to destroy)
	h.Close()
}

// TestXKBHandleSetKeymapFromFD_NilHandle verifies error on nil handle.
func TestXKBHandleSetKeymapFromFD_NilHandle(t *testing.T) {
	var h *XKBHandle
	err := h.SetKeymapFromFD(3, 100)
	if err == nil {
		t.Error("nil XKBHandle.SetKeymapFromFD() = nil, want error")
	}
}

// TestXKBEvdevOffset verifies the evdev-to-xkb keycode offset.
// XKB keycodes = evdev keycodes + 8 (historical X11 heritage).
// Evdev key 'a' = 30, XKB key 'a' = 38.
func TestXKBEvdevOffset(t *testing.T) {
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
			got := tt.evdev + xkb.XKBEvdevOffset
			if got != tt.wantXKB {
				t.Errorf("evdev %d + offset = %d, want %d", tt.evdev, got, tt.wantXKB)
			}
		})
	}
}
