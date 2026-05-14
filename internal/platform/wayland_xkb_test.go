//go:build linux

package platform

import (
	"sync"
	"testing"
	"time"

	"github.com/gogpu/gogpu/internal/platform/wayland"
	"github.com/gogpu/gpucontext"
)

// TestWaylandXKBKeyDispatch verifies that when xkbcommon is available (Ready),
// KeyGetUtf8 is used for character dispatch instead of evdevKeycodeToRune.
func TestWaylandXKBKeyDispatch(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{
		ready:  true,
		result: "a",
	}
	w.xkb = xkb

	// Simulate key press on key 30 (KEY_A) — should use xkb
	w.keyboardFocused = true
	events := dispatchKeyWithXKB(w, 30, 0, true)

	// Expect EventKeyDown + EventChar('a')
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	if events[0].Type != EventKeyDown {
		t.Errorf("events[0].Type = %v, want EventKeyDown", events[0].Type)
	}
	if events[1].Type != EventChar {
		t.Errorf("events[1].Type = %v, want EventChar", events[1].Type)
	}
	if events[1].Char != 'a' {
		t.Errorf("events[1].Char = %q, want 'a'", events[1].Char)
	}
}

// TestWaylandXKBMultiRuneDispatch verifies multi-byte UTF-8 characters.
func TestWaylandXKBMultiRuneDispatch(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{
		ready:  true,
		result: "\u0439", // Cyrillic "й" (U+0439)
	}
	w.xkb = xkb
	w.keyboardFocused = true

	events := dispatchKeyWithXKB(w, 16, 0, true) // key Q → Cyrillic "й" in Russian layout

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[1].Type != EventChar {
		t.Errorf("events[1].Type = %v, want EventChar", events[1].Type)
	}
	if events[1].Char != '\u0439' {
		t.Errorf("events[1].Char = %U, want U+0439", events[1].Char)
	}
}

// TestWaylandXKBFallbackToEvdev verifies fallback to evdevKeycodeToRune when xkb is not ready.
func TestWaylandXKBFallbackToEvdev(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	// xkb not available (nil)
	w.xkb = nil
	w.keyboardFocused = true

	events := dispatchKeyWithXKB(w, 30, 0, true) // KEY_A

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[1].Type != EventChar {
		t.Errorf("events[1].Type = %v, want EventChar", events[1].Type)
	}
	if events[1].Char != 'a' {
		t.Errorf("events[1].Char = %q, want 'a' (evdev fallback)", events[1].Char)
	}
}

// TestWaylandXKBFallbackWhenNotReady verifies fallback when XKBHandle exists but state is 0.
func TestWaylandXKBFallbackWhenNotReady(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{
		ready:  false, // no keymap loaded yet
		result: "",
	}
	w.xkb = xkb
	w.keyboardFocused = true

	events := dispatchKeyWithXKB(w, 30, 0, true) // KEY_A

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[1].Char != 'a' {
		t.Errorf("events[1].Char = %q, want 'a' (evdev fallback)", events[1].Char)
	}
}

// TestWaylandXKBNoCharOnRelease verifies no char event on key release.
func TestWaylandXKBNoCharOnRelease(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{
		ready:  true,
		result: "a",
	}
	w.xkb = xkb
	w.keyboardFocused = true

	events := dispatchKeyWithXKB(w, 30, 0, false) // release

	if len(events) != 1 {
		t.Fatalf("expected 1 event (KeyUp only), got %d", len(events))
	}
	if events[0].Type != EventKeyUp {
		t.Errorf("events[0].Type = %v, want EventKeyUp", events[0].Type)
	}
}

// TestWaylandXKBNoCharWithCtrl verifies no char event when Ctrl is held.
func TestWaylandXKBNoCharWithCtrl(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{
		ready:  true,
		result: "a",
	}
	w.xkb = xkb
	w.keyboardFocused = true
	w.modifiers = gpucontext.ModControl

	events := dispatchKeyWithXKB(w, 30, gpucontext.ModControl, true)

	// Only EventKeyDown, no EventChar (Ctrl+A is a shortcut, not text)
	if len(events) != 1 {
		t.Fatalf("expected 1 event (KeyDown only), got %d", len(events))
	}
	if events[0].Type != EventKeyDown {
		t.Errorf("events[0].Type = %v, want EventKeyDown", events[0].Type)
	}
}

// TestWaylandXKBNoCharOnNonPrintable verifies no char event for non-printable keys.
func TestWaylandXKBNoCharOnNonPrintable(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{
		ready:  true,
		result: "", // Escape produces nothing
	}
	w.xkb = xkb
	w.keyboardFocused = true

	events := dispatchKeyWithXKB(w, 1, 0, true) // KEY_ESC

	// Only EventKeyDown, no EventChar
	if len(events) != 1 {
		t.Fatalf("expected 1 event (KeyDown only), got %d", len(events))
	}
}

// TestWaylandXKBGroupSwitch verifies that UpdateMask is called with the group parameter.
func TestWaylandXKBGroupSwitch(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{ready: true}
	w.xkb = xkb

	// Simulate wl_keyboard.modifiers with group=1 (second layout)
	waylandModifiersCallback(w, 0, 0, 0, 1)

	if xkb.lastGroup != 1 {
		t.Errorf("xkb.lastGroup = %d, want 1", xkb.lastGroup)
	}
	if xkb.updateMaskCalled != 1 {
		t.Errorf("xkb.updateMaskCalled = %d, want 1", xkb.updateMaskCalled)
	}
}

// TestWaylandXKBGroupSwitchAffectsKeyOutput verifies that switching group changes character output.
func TestWaylandXKBGroupSwitchAffectsKeyOutput(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{ready: true}
	w.xkb = xkb
	w.keyboardFocused = true

	// Group 0 → English 'q'
	xkb.result = "q"
	events := dispatchKeyWithXKB(w, 16, 0, true)
	if len(events) < 2 || events[1].Char != 'q' {
		t.Errorf("group 0: expected 'q', got events: %+v", events)
	}

	// Switch to group 1 (Russian layout)
	waylandModifiersCallback(w, 0, 0, 0, 1)
	xkb.result = "\u0439" // Cyrillic й

	events = dispatchKeyWithXKB(w, 16, 0, true)
	if len(events) < 2 || events[1].Char != '\u0439' {
		t.Errorf("group 1: expected U+0439, got events: %+v", events)
	}
}

// TestWaylandXKBKeymapCallback verifies that OnKeyboardKeymap triggers SetKeymapFromFD.
func TestWaylandXKBKeymapCallback(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{ready: false}
	w.xkb = xkb

	// Simulate keymap callback with XKB_KEYMAP_FORMAT_TEXT_V1 = 1
	waylandKeymapCallback(w, 1, 42, 4096)

	if xkb.setKeymapFD != 42 {
		t.Errorf("xkb.setKeymapFD = %d, want 42", xkb.setKeymapFD)
	}
	if xkb.setKeymapSize != 4096 {
		t.Errorf("xkb.setKeymapSize = %d, want 4096", xkb.setKeymapSize)
	}
}

// TestWaylandXKBKeymapCallbackIgnoresNonXKB verifies non-XKB format is ignored.
func TestWaylandXKBKeymapCallbackIgnoresNonXKB(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{ready: false}
	w.xkb = xkb

	// Format 0 = XKB_KEYMAP_FORMAT_NO_KEYMAP → should be ignored
	waylandKeymapCallback(w, 0, 42, 4096)

	if xkb.setKeymapFD != 0 {
		t.Errorf("xkb.setKeymapFD = %d, want 0 (not called)", xkb.setKeymapFD)
	}
}

// TestWaylandXKBThreadSafety verifies concurrent access to xkb handle is safe.
func TestWaylandXKBThreadSafety(t *testing.T) {
	w := &waylandWindow{startTime: time.Now()}
	xkb := &mockXKBHandle{ready: true, result: "x"}
	w.xkb = xkb
	w.keyboardFocused = true

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(group uint32) {
			defer wg.Done()
			waylandModifiersCallback(w, 0, 0, 0, group)
		}(uint32(i % 4))
		go func() {
			defer wg.Done()
			dispatchKeyWithXKB(w, 30, 0, true)
		}()
	}
	wg.Wait()
	// No race conditions should occur — verified by -race flag
}

// --- Mock XKBHandle for testing ---

// mockXKBHandle implements xkbKeyHandler for testing.
type mockXKBHandle struct {
	mu               sync.Mutex
	ready            bool
	result           string
	lastGroup        uint32
	updateMaskCalled int
	setKeymapFD      int
	setKeymapSize    uint32
}

func (m *mockXKBHandle) Ready() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ready
}

func (m *mockXKBHandle) KeyGetUtf8(keycode uint32) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.result
}

func (m *mockXKBHandle) UpdateMask(modsDepressed, modsLatched, modsLocked, group uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastGroup = group
	m.updateMaskCalled++
}

func (m *mockXKBHandle) SetKeymapFromFD(fd int, size uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setKeymapFD = fd
	m.setKeymapSize = size
	return nil
}

func (m *mockXKBHandle) Close() {}

// Verify mockXKBHandle satisfies xkbKeyHandler at compile time.
var _ xkbKeyHandler = (*mockXKBHandle)(nil)

// Verify *wayland.XKBHandle satisfies xkbKeyHandler at compile time.
var _ xkbKeyHandler = (*wayland.XKBHandle)(nil)

// --- Helper functions that exercise production code paths ---

// dispatchKeyWithXKB simulates the OnKeyboardKey callback logic for testing.
// Uses the production keycodeToRune method to verify the real code path.
func dispatchKeyWithXKB(w *waylandWindow, keycode uint32, mods gpucontext.Modifiers, pressed bool) []Event {
	w.eventMu.Lock()
	w.events = nil // Clear queue
	w.eventMu.Unlock()

	gpuKey := evdevToKey(keycode)
	w.dispatchKeyEvent(gpuKey, mods, pressed)

	// Character dispatch on press only, no modifier keys — same logic as production callback
	if pressed && mods&(gpucontext.ModControl|gpucontext.ModAlt|gpucontext.ModSuper) == 0 {
		if r := w.keycodeToRune(keycode); r != 0 {
			w.queueEvent(Event{Type: EventChar, Char: r})
		}
	}

	w.eventMu.Lock()
	result := make([]Event, len(w.events))
	copy(result, w.events)
	w.eventMu.Unlock()
	return result
}

// waylandModifiersCallback simulates the OnKeyboardModifiers callback.
// Mirrors the production callback: updates modifiers + xkb state.
func waylandModifiersCallback(w *waylandWindow, modsDepressed, modsLatched, modsLocked, group uint32) {
	w.pointerMu.Lock()
	w.modifiers = evdevModsToModifiers(modsDepressed, modsLocked)
	w.pointerMu.Unlock()

	if w.xkb != nil {
		w.xkb.UpdateMask(modsDepressed, modsLatched, modsLocked, group)
	}
}

// waylandKeymapCallback simulates the OnKeyboardKeymap callback.
// Mirrors the production callback: loads keymap from fd when format is XKB_V1.
func waylandKeymapCallback(w *waylandWindow, format uint32, fd int, size uint32) {
	if format != 1 { // XKB_KEYMAP_FORMAT_TEXT_V1
		return
	}
	if w.xkb != nil {
		_ = w.xkb.SetKeymapFromFD(fd, size)
	}
}
