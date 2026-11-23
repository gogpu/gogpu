//go:build windows

package platform

import (
	"runtime"
	"testing"
)

// A platform initialized after another one was destroyed in the same process
// registers the window class again and opens its windows, as HostLoop does
// when it is started again after a Stop.
func TestWindowsPlatformInitializedAfterAnotherIsDestroyedOpensAWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for _, name := range []string{"first", "second"} {
		p := newPlatformManager().(*windowsPlatform)
		if err := p.Init(); err != nil {
			t.Fatalf("%s Init = %v", name, err)
		}
		_, err := p.CreateWindow(Config{Title: name, Width: 320, Height: 240})
		p.Destroy()
		if err != nil {
			t.Fatalf("%s CreateWindow = %v", name, err)
		}
	}
}

// Two platforms alive at once share the process-wide window class: the second
// Init registers the class again, which must not fail, and its window opens.
// The order of Init and Destroy must not matter (winit's model: the class
// lives as long as the process).
func TestWindowsTwoPlatformsInOneProcess(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	first := newPlatformManager().(*windowsPlatform)
	if err := first.Init(); err != nil {
		t.Fatalf("first Init = %v", err)
	}
	defer first.Destroy()

	second := newPlatformManager().(*windowsPlatform)
	if err := second.Init(); err != nil {
		t.Fatalf("second Init = %v", err)
	}
	defer second.Destroy()

	if _, err := second.CreateWindow(Config{Title: "second", Width: 320, Height: 240}); err != nil {
		t.Fatalf("second CreateWindow = %v", err)
	}
}
