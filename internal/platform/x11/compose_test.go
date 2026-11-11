//go:build linux

package x11

import (
	"os"
	"testing"
	"time"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
	xkbcommon "github.com/gogpu/gogpu/internal/platform/xkb"
)

// X11 keycodes (evdev + 8) of the keys the compose tests press.
const (
	x11KeycodeE          uint8 = 26
	x11KeycodeA          uint8 = 38
	x11KeycodeApostrophe uint8 = 48
)

// testKeysymDeadAcute is the dead acute keysym of the server mapping a compose test builds.
const testKeysymDeadAcute Keysym = 0xfe51

// newComposePlatform answers a Platform whose xkbcommon handle has the
// en_US.UTF-8 compose table and a keymap of layout and variant, and a window
// for its events, skipping when the system has neither the library, the table
// nor the keymap.
func newComposePlatform(t *testing.T, layout, variant string) (*Platform, *x11Window) {
	t.Helper()
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if _, err := os.Stat("/usr/share/X11/locale/en_US.UTF-8/Compose"); err != nil {
		t.Skipf("no en_US.UTF-8 compose table on this system: %v", err)
	}
	handle, err := xkbcommon.New()
	if err != nil {
		t.Skipf("libxkbcommon unavailable: %v", err)
	}
	t.Cleanup(handle.Close)
	if err := handle.SetKeymapFromRMLVO("evdev", "pc105", layout, variant, ""); err != nil {
		t.Skipf("no %s(%s) keymap on this system: %v", layout, variant, err)
	}
	w := &x11Window{
		startTime: time.Now(),
		events:    eventqueue.New[PlatformEvent](eventqueue.DefaultCapacity),
	}
	return &Platform{xkbState: handle, primary: w}, w
}

// pressKey presses and releases keycode, and answers the text it typed.
func pressKey(p *Platform, w *x11Window, keycode uint8) string {
	p.handleKeyEvent(w, keycode, 0, true)
	p.handleKeyEvent(w, keycode, 0, false)
	var text []rune
	for {
		event, ok := w.dequeueEvent()
		if !ok {
			return string(text)
		}
		if event.Type == EventTypeChar {
			text = append(text, event.Char)
		}
	}
}

func TestX11ADeadAcuteThenETypesEAcuteOnUSInternational(t *testing.T) {
	p, w := newComposePlatform(t, "us", "intl")

	if got := pressKey(p, w, x11KeycodeApostrophe); got != "" {
		t.Errorf("dead acute typed %q, want nothing", got)
	}
	if got := pressKey(p, w, x11KeycodeE); got != "é" {
		t.Errorf("dead acute, e typed %q, want %q", got, "é")
	}
	if got := pressKey(p, w, x11KeycodeA); got != "a" {
		t.Errorf("a after the sequence typed %q, want %q", got, "a")
	}
}

func TestX11ADeadKeyOfAPreferredServerMappingComposes(t *testing.T) {
	// xkbcommon's keymap is plain US, where the apostrophe is no dead key:
	// the server's mapping, which comes first, makes it dead acute.
	p, w := newComposePlatform(t, "us", "")
	keysyms := make([]Keysym, int(x11KeycodeApostrophe-x11KeycodeE+1)*2)
	keysyms[0] = Keysyme
	keysyms[len(keysyms)-2] = testKeysymDeadAcute
	p.keymap = &KeyboardMapping{
		MinKeycode:     x11KeycodeE,
		MaxKeycode:     x11KeycodeApostrophe,
		KeysymsPerCode: 2,
		Keysyms:        keysyms,
	}
	p.preferServerKeymap = true

	if got := pressKey(p, w, x11KeycodeApostrophe); got != "" {
		t.Errorf("dead acute typed %q, want nothing", got)
	}
	if got := pressKey(p, w, x11KeycodeE); got != "é" {
		t.Errorf("dead acute, e typed %q, want %q", got, "é")
	}
}
