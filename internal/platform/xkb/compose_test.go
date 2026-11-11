//go:build linux

package xkb

import (
	"os"
	"testing"
)

// enUSComposeFile is where libX11's locale data, which xkbcommon reads, keeps
// the en_US.UTF-8 compose table.
const enUSComposeFile = "/usr/share/X11/locale/en_US.UTF-8/Compose"

// Evdev keycodes of the keys the compose tests press.
const (
	evdevKeyE          uint32 = 18
	evdevKeyO          uint32 = 24
	evdevKeyA          uint32 = 30
	evdevKeyApostrophe uint32 = 40
	evdevKeyC          uint32 = 46
	evdevKeyRightAlt   uint32 = 100
)

// newComposeHandle loads libxkbcommon with the en_US.UTF-8 compose table and
// a keymap of layout, variant and options, skipping when the system has
// neither the library, the table nor the keymap.
func newComposeHandle(t *testing.T, layout, variant, options string) *Handle {
	t.Helper()
	t.Setenv("LC_ALL", "en_US.UTF-8")
	h, err := New()
	if err != nil {
		t.Skipf("libxkbcommon unavailable: %v", err)
	}
	t.Cleanup(h.Close)
	if h.composeState == 0 {
		if _, err := os.Stat(enUSComposeFile); err != nil {
			t.Skipf("no en_US.UTF-8 compose table on this system: %v", err)
		}
		t.Fatalf("the compose table was not loaded, though %s exists", enUSComposeFile)
	}
	if err := h.SetKeymapFromRMLVO("evdev", "pc105", layout, variant, options); err != nil {
		t.Skipf("no %s(%s) keymap on this system: %v", layout, variant, err)
	}
	return h
}

func TestADeadAcuteThenETypesEAcuteOnUSInternational(t *testing.T) {
	h := newComposeHandle(t, "us", "intl", "")

	if got := h.KeyGetComposedUtf8(evdevKeyApostrophe); got != "" {
		t.Errorf("dead acute typed %q, want nothing", got)
	}
	if got := h.KeyGetComposedUtf8(evdevKeyE); got != "é" {
		t.Errorf("dead acute, e typed %q, want %q", got, "é")
	}
}

func TestComposeOCTypesTheCopyrightSign(t *testing.T) {
	h := newComposeHandle(t, "us", "", "compose:ralt")

	if got := h.KeyGetComposedUtf8(evdevKeyRightAlt); got != "" {
		t.Errorf("Compose typed %q, want nothing", got)
	}
	if got := h.KeyGetComposedUtf8(evdevKeyO); got != "" {
		t.Errorf("Compose, o typed %q, want nothing", got)
	}
	if got := h.KeyGetComposedUtf8(evdevKeyC); got != "©" {
		t.Errorf("Compose, o, c typed %q, want %q", got, "©")
	}
}

func TestAKeyNoPartOfASequenceTypesItsOwnText(t *testing.T) {
	h := newComposeHandle(t, "us", "intl", "")

	if got := h.KeyGetComposedUtf8(evdevKeyA); got != "a" {
		t.Errorf("a typed %q, want %q", got, "a")
	}

	// After a completed sequence, the next key starts afresh.
	h.KeyGetComposedUtf8(evdevKeyApostrophe)
	h.KeyGetComposedUtf8(evdevKeyE)
	if got := h.KeyGetComposedUtf8(evdevKeyA); got != "a" {
		t.Errorf("a after a sequence typed %q, want %q", got, "a")
	}
}

func TestFeedComposeWithoutATableTakesNoKey(t *testing.T) {
	var h *Handle
	if composed, taken := h.FeedCompose(0xfe51); taken || composed != "" {
		t.Errorf("nil Handle.FeedCompose = %q, %v; want not taken", composed, taken)
	}
	h = &Handle{}
	if composed, taken := h.FeedCompose(0xfe51); taken || composed != "" {
		t.Errorf("empty Handle.FeedCompose = %q, %v; want not taken", composed, taken)
	}
}
