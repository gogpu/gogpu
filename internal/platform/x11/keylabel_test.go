//go:build linux

package x11

import (
	"testing"

	xkbcommon "github.com/gogpu/gogpu/internal/platform/xkb"
	"github.com/gogpu/gpucontext"
)

// expectLabels checks p's label of each key against labels.
func expectLabels(t *testing.T, p *Platform, labels map[gpucontext.Key]rune) {
	t.Helper()
	for key, want := range labels {
		if got := p.KeyLabel(key); got != want {
			t.Errorf("KeyLabel(%v) = %q, want %q", key, got, want)
		}
	}
}

func TestKeyLabelReadsTheServersMappingAtTheGroup(t *testing.T) {
	// Two groups, US then French: X keycode 24 is the Q position, 11 the 2.
	keymap := &KeyboardMapping{MinKeycode: 8, MaxKeycode: 255, KeysymsPerCode: 4, Keysyms: make([]Keysym, 248*4)}
	copy(keymap.Keysyms[(24-8)*4:], []Keysym{'q', 'Q', 'a', 'A'})
	copy(keymap.Keysyms[(11-8)*4:], []Keysym{'2', '@', 0xe9, '2'})
	p := &Platform{keymap: keymap}

	expectLabels(t, p, map[gpucontext.Key]rune{gpucontext.KeyQ: 'q', gpucontext.Key2: '2', gpucontext.KeyF1: 0})
	p.xkbGroup = 1
	expectLabels(t, p, map[gpucontext.Key]rune{gpucontext.KeyQ: 'a', gpucontext.Key2: 'é'})
}

func TestKeyLabelReadsTheXkbcommonStateAtItsGroup(t *testing.T) {
	handle, err := xkbcommon.New()
	if err != nil {
		t.Skipf("libxkbcommon: %v", err)
	}
	defer handle.Close()
	if err := handle.SetKeymapFromRMLVO("evdev", "pc105", "us,fr", "", ""); err != nil {
		t.Skipf("no us,fr keymap: %v", err)
	}
	p := &Platform{xkbState: handle}

	expectLabels(t, p, map[gpucontext.Key]rune{gpucontext.KeyQ: 'q', gpucontext.Key2: '2', gpucontext.KeyEnter: 0})
	handle.UpdateMask(0, 0, 0, 0, 0, 1)
	expectLabels(t, p, map[gpucontext.Key]rune{gpucontext.KeyQ: 'a', gpucontext.Key2: 'é'})
}

func TestKeyLabelIsZeroBeforeAKeymap(t *testing.T) {
	expectLabels(t, &Platform{}, map[gpucontext.Key]rune{gpucontext.KeyQ: 0})
}

func TestAGroupSwitchReportsALayoutChange(t *testing.T) {
	p := newTestPlatformWithXkb(85, 0)

	if p.handleUnknownEvent(buildXkbStateNotifyEvent(85, 1)) != xkbcommon.LayoutChanged {
		t.Error("a switch to group 1 reported no layout change")
	}
	modifiers := buildXkbStateNotifyEvent(85, 1)
	modifiers.Data[9] = 1 // Shift
	if p.handleUnknownEvent(modifiers) != xkbcommon.LayoutUnchanged {
		t.Error("a modifier change within group 1 reported a layout change")
	}
}

func TestAKeymapChangeReportsALayoutChange(t *testing.T) {
	for _, xkbType := range []byte{0, 1} { // XkbNewKeyboardNotify, XkbMapNotify
		p := newTestPlatformWithXkb(85, 0)
		event := &UnknownEvent{Type: 85}
		event.Data[0] = xkbType
		if p.handleUnknownEvent(event) != xkbcommon.LayoutChanged {
			t.Errorf("XKB event %d reported no layout change", xkbType)
		}
	}

	p := &Platform{}
	if p.handleMappingNotify(&MappingNotifyEvent{Request: MappingKeyboard}) != xkbcommon.LayoutChanged {
		t.Error("a keyboard MappingNotify reported no layout change")
	}
	if p.handleMappingNotify(&MappingNotifyEvent{Request: MappingPointer}) != xkbcommon.LayoutUnchanged {
		t.Error("a pointer MappingNotify reported a layout change")
	}
}
