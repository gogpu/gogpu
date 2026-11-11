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
