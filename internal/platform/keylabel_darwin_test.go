//go:build darwin

package platform

import (
	"os"
	"testing"
	"time"

	"github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gpucontext"
)

// keyboardLayoutChanges polls p until no event is left, and counts the layout
// changes it reported.
func keyboardLayoutChanges(p *darwinPlatform) int {
	events := pollAll(p)
	var changes int
	for i := range events {
		if events[i].Type == EventKeyboardLayoutChanged {
			changes++
		}
	}
	return changes
}

// TestKeyLabelsFollowTheKeyboardLayout gives the platform layouts the user
// need not have enabled, as an input source change gives the one in use.
func TestKeyLabelsFollowTheKeyboardLayout(t *testing.T) {
	runOnMain(func() {
		french, err := darwin.InstalledKeyboardLayout("com.apple.keylayout.French")
		if err != nil {
			t.Errorf("InstalledKeyboardLayout(French): %v", err)
			return
		}
		us, err := darwin.InstalledKeyboardLayout("com.apple.keylayout.US")
		if err != nil {
			t.Errorf("InstalledKeyboardLayout(US): %v", err)
			return
		}
		p := &darwinPlatform{}

		p.updateKeyboardLayout(french)
		if changes := keyboardLayoutChanges(p); changes != 1 {
			t.Errorf("French reported %d layout changes, want 1", changes)
		}
		for key, want := range map[gpucontext.Key]rune{
			gpucontext.KeyQ:           'a',
			gpucontext.KeyA:           'q',
			gpucontext.Key2:           'é',
			gpucontext.KeyLeftBracket: '^',
			gpucontext.KeyUnknown:     0,
		} {
			if got := p.KeyLabel(key); got != want {
				t.Errorf("French: KeyLabel(%v) = %q, want %q", key, got, want)
			}
		}

		p.updateKeyboardLayout(french)
		if changes := keyboardLayoutChanges(p); changes != 0 {
			t.Errorf("French again reported %d layout changes, want 0", changes)
		}

		p.updateKeyboardLayout(us)
		if changes := keyboardLayoutChanges(p); changes != 1 {
			t.Errorf("US reported %d layout changes, want 1", changes)
		}
		if got := p.KeyLabel(gpucontext.KeyQ); got != 'q' {
			t.Errorf("US: KeyLabel(KeyQ) = %q, want 'q'", got)
		}
	})
}

// TestAnInputSourceChangeReportsTheLayoutInUse holds another layout than the
// one in use, then posts the notification macOS posts when the user selects
// another input source: polling reports the change, and labels follow the
// layout in use.
func TestAnInputSourceChangeReportsTheLayoutInUse(t *testing.T) {
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("opens a window, which CI runners skip")
	}
	runOnMain(func() {
		current, err := darwin.CurrentKeyboardLayout()
		if err != nil {
			t.Errorf("CurrentKeyboardLayout: %v", err)
			return
		}
		other, err := darwin.InstalledKeyboardLayout("com.apple.keylayout.French")
		if err == nil && other.Equal(current) {
			other, err = darwin.InstalledKeyboardLayout("com.apple.keylayout.US")
		}
		if err != nil {
			t.Errorf("InstalledKeyboardLayout: %v", err)
			return
		}

		// PollEvents drains AppKit's events, and with them the notification,
		// while a window is open.
		p := &darwinPlatform{}
		if err := p.Init(); err != nil {
			t.Errorf("Init: %v", err)
			return
		}
		defer p.Destroy()
		if _, err := p.CreateWindow(Config{Title: "key labels", Width: 320, Height: 240}); err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		pollAll(p)
		p.keyboardLayout = other

		postInputSourceChanged()
		var changes int
		for deadline := time.Now().Add(2 * time.Second); changes == 0 && time.Now().Before(deadline); {
			changes = keyboardLayoutChanges(p)
			time.Sleep(10 * time.Millisecond)
		}
		if changes != 1 {
			t.Errorf("an input source change reported %d layout changes, want 1", changes)
		}
		const keyCodeQ = 0x0C
		if got, want := p.KeyLabel(gpucontext.KeyQ), current.Label(keyCodeQ); got != want {
			t.Errorf("KeyLabel(KeyQ) = %q after the change, want %q, the layout in use's", got, want)
		}
	})
}

// postInputSourceChanged posts the distributed notification macOS posts when
// the user selects another input source, which every process observing it
// hears: kTISNotifySelectedKeyboardInputSourceChanged.
func postInputSourceChanged() {
	name := darwin.NewNSString("com.apple.Carbon.TISNotifySelectedKeyboardInputSourceChanged")
	defer name.Release()
	center := darwin.ID(darwin.GetClass("NSDistributedNotificationCenter")).Send(darwin.RegisterSelector("defaultCenter"))
	center.SendPtrs(darwin.RegisterSelector("postNotificationName:object:userInfo:deliverImmediately:"),
		name.ID().Ptr(), 0, 0, 1)
}
