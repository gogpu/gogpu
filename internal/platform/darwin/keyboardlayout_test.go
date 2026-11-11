//go:build darwin

package darwin_test

import (
	"testing"

	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

// installedKeyboardLayout loads an installed layout, enabled or not, without
// selecting it. Call it on the main thread, where a failure can't end the test
// at once: it answers the zero layout, which labels every key 0.
func installedKeyboardLayout(t *testing.T, inputSourceID string) platformdarwin.KeyboardLayout {
	t.Helper()
	layout, err := platformdarwin.InstalledKeyboardLayout(inputSourceID)
	if err != nil {
		t.Errorf("InstalledKeyboardLayout(%q): %v", inputSourceID, err)
	}
	return layout
}

// TestKeyboardLayoutLabelsAKeyWithWhatItPrints asks layouts the user need not
// have enabled for the label of keys by their virtual key code.
func TestKeyboardLayoutLabelsAKeyWithWhatItPrints(t *testing.T) {
	const (
		keyCodeA            = 0x00
		keyCodeQ            = 0x0C
		keyCode2            = 0x13
		keyCodeLeftBracket  = 0x21
		keyCodeReturn       = 0x24
		frenchInputSourceID = "com.apple.keylayout.French"
	)
	tests := []struct {
		name          string
		inputSourceID string
		keyCode       uint16
		want          rune
	}{
		{"US Q", "com.apple.keylayout.US", keyCodeQ, 'q'},
		{"French Q", frenchInputSourceID, keyCodeQ, 'a'},
		{"French A", frenchInputSourceID, keyCodeA, 'q'},
		{"French 2", frenchInputSourceID, keyCode2, 'é'},
		{"French dead circumflex", frenchInputSourceID, keyCodeLeftBracket, '^'},
		{"Russian Q", "com.apple.keylayout.Russian", keyCodeQ, 'й'},
		{"US Return", "com.apple.keylayout.US", keyCodeReturn, '\r'},
	}
	runOnMainThread(t, func() {
		for _, tt := range tests {
			layout := installedKeyboardLayout(t, tt.inputSourceID)
			if got := layout.Label(tt.keyCode); got != tt.want {
				t.Errorf("%s: Label(%#x) = %q, want %q", tt.name, tt.keyCode, got, tt.want)
			}
		}
	})
}

// TestKeyboardLayoutsAreEqualWhenTheyPrintAlike compares layouts by what
// their keys print.
func TestKeyboardLayoutsAreEqualWhenTheyPrintAlike(t *testing.T) {
	runOnMainThread(t, func() {
		french := installedKeyboardLayout(t, "com.apple.keylayout.French")
		frenchAgain := installedKeyboardLayout(t, "com.apple.keylayout.French")
		us := installedKeyboardLayout(t, "com.apple.keylayout.US")
		if !french.Equal(frenchAgain) {
			t.Error("French is not equal to French loaded again")
		}
		if french.Equal(us) {
			t.Error("French is equal to US")
		}
		if us.Equal(platformdarwin.KeyboardLayout{}) {
			t.Error("US is equal to the zero layout")
		}
	})
}

// TestInstalledKeyboardLayoutAnswersAnErrorForAnUnknownSource asks for an
// input source no Mac has.
func TestInstalledKeyboardLayoutAnswersAnErrorForAnUnknownSource(t *testing.T) {
	runOnMainThread(t, func() {
		if _, err := platformdarwin.InstalledKeyboardLayout("com.example.keylayout.None"); err == nil {
			t.Error("InstalledKeyboardLayout of an unknown input source answered no error")
		}
	})
}

// TestCurrentKeyboardLayoutLabelsTheLetterKeys reads the layout in use,
// whichever it is: the key in the US A position prints a letter on every
// layout.
func TestCurrentKeyboardLayoutLabelsTheLetterKeys(t *testing.T) {
	runOnMainThread(t, func() {
		layout, err := platformdarwin.CurrentKeyboardLayout()
		if err != nil {
			t.Errorf("CurrentKeyboardLayout: %v", err)
			return
		}
		if label := layout.Label(0x00); label == 0 {
			t.Error("the current layout labels the key in the US A position 0")
		}
	})
}

// TestObserveInputSourceChangesCallsBackUntilStopped posts the notification
// macOS posts when the user selects another input source, which every process
// observing it hears, then runs the main thread's run loop, which delivers it.
func TestObserveInputSourceChangesCallsBackUntilStopped(t *testing.T) {
	runOnMainThread(t, func() {
		var changes int
		observer, err := platformdarwin.ObserveInputSourceChanges(func() { changes++ })
		if err != nil {
			t.Errorf("ObserveInputSourceChanges: %v", err)
			return
		}
		postInputSourceChanged()
		for range 20 {
			if changes > 0 {
				break
			}
			runRunLoop(0.1)
		}
		if changes != 1 {
			t.Errorf("one posted change called back %d times, want 1", changes)
		}

		observer.Stop()
		changesBeforeStop := changes
		postInputSourceChanged()
		runRunLoop(0.5)
		if changes != changesBeforeStop {
			t.Errorf("a change posted after Stop called back %d times", changes-changesBeforeStop)
		}
	})
}

// postInputSourceChanged posts the distributed notification macOS posts when
// the user selects another input source, kTISNotifySelectedKeyboardInputSourceChanged.
func postInputSourceChanged() {
	name := platformdarwin.NewNSString("com.apple.Carbon.TISNotifySelectedKeyboardInputSourceChanged")
	defer name.Release()
	center := platformdarwin.ID(platformdarwin.GetClass("NSDistributedNotificationCenter")).
		Send(platformdarwin.RegisterSelector("defaultCenter"))
	center.SendPtrs(platformdarwin.RegisterSelector("postNotificationName:object:userInfo:deliverImmediately:"),
		name.ID().Ptr(), 0, 0, 1)
}

// runRunLoop runs the current thread's run loop for seconds, or less when it
// has nothing to wait for.
func runRunLoop(seconds float64) {
	date := platformdarwin.ID(platformdarwin.GetClass("NSDate")).
		SendDouble(platformdarwin.RegisterSelector("dateWithTimeIntervalSinceNow:"), seconds)
	platformdarwin.ID(platformdarwin.GetClass("NSRunLoop")).Send(platformdarwin.RegisterSelector("currentRunLoop")).
		SendPtr(platformdarwin.RegisterSelector("runUntilDate:"), date.Ptr())
}
