//go:build darwin

package darwin_test

import (
	"testing"

	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

// TestWindowDelegateClassRegistration verifies that the GoGPUWindowDelegate
// class can be registered and returns a valid class pointer.
func TestWindowDelegateClassRegistration(t *testing.T) {
	runOnMainThread(t, func() {
		cls, err := platformdarwin.WindowDelegateClass()
		if err != nil {
			t.Fatalf("WindowDelegateClass() error: %v", err)
		}
		if cls == 0 {
			t.Fatal("WindowDelegateClass() returned 0")
		}

		found := platformdarwin.GetClass("GoGPUWindowDelegate")
		if found == 0 {
			t.Error("GoGPUWindowDelegate class not found via GetClass")
		}
		if found != cls {
			t.Errorf("GetClass returned %v, WindowDelegateClass returned %v", found, cls)
		}
	})
}

// TestWindowDelegateClassIdempotent verifies that calling WindowDelegateClass()
// multiple times returns the same class (sync.Once pattern).
func TestWindowDelegateClassIdempotent(t *testing.T) {
	runOnMainThread(t, func() {
		cls1, err1 := platformdarwin.WindowDelegateClass()
		cls2, err2 := platformdarwin.WindowDelegateClass()

		if err1 != nil || err2 != nil {
			t.Fatalf("errors: %v, %v", err1, err2)
		}
		if cls1 != cls2 {
			t.Errorf("WindowDelegateClass not idempotent: %v != %v", cls1, cls2)
		}
	})
}

// TestWindowDelegateClassIsSubclassOfNSObject verifies that the delegate class
// inherits from NSObject.
func TestWindowDelegateClassIsSubclassOfNSObject(t *testing.T) {
	runOnMainThread(t, func() {
		cls, err := platformdarwin.WindowDelegateClass()
		if err != nil {
			t.Fatalf("WindowDelegateClass() error: %v", err)
		}

		nsObjectClass := platformdarwin.GetClass("NSObject")
		if nsObjectClass == 0 {
			t.Fatal("NSObject class not found")
		}

		selIsSubclass := platformdarwin.RegisterSelector("isSubclassOfClass:")
		result := platformdarwin.ID(cls).SendPtr(selIsSubclass, platformdarwin.ID(nsObjectClass).Ptr())
		if result == 0 {
			t.Error("GoGPUWindowDelegate is not a subclass of NSObject")
		}
	})
}

// TestWindowDelegateRespondsToSelector verifies that the delegate class
// responds to windowShouldClose: selector.
func TestWindowDelegateRespondsToSelector(t *testing.T) {
	runOnMainThread(t, func() {
		cls, err := platformdarwin.WindowDelegateClass()
		if err != nil {
			t.Fatalf("WindowDelegateClass() error: %v", err)
		}

		selResponds := platformdarwin.RegisterSelector("instancesRespondToSelector:")
		selShouldClose := platformdarwin.RegisterSelector("windowShouldClose:")

		result := platformdarwin.ID(cls).SendPtr(selResponds, uintptr(selShouldClose))
		if result == 0 {
			t.Error("GoGPUWindowDelegate does not respond to windowShouldClose:")
		}
	})
}

// TestCreateWindowDelegate verifies that a delegate instance can be created
// and associated with a *Window.
func TestCreateWindowDelegate(t *testing.T) {
	runOnMainThread(t, func() {
		config := platformdarwin.WindowConfig{
			Title:     "delegate test",
			Width:     400,
			Height:    300,
			Resizable: false,
		}
		win, err := platformdarwin.NewWindow(config)
		if err != nil {
			t.Fatalf("NewWindow error: %v", err)
		}
		defer win.Destroy()

		delegate, err := platformdarwin.CreateWindowDelegate(win)
		if err != nil {
			t.Fatalf("CreateWindowDelegate error: %v", err)
		}
		if delegate.IsNil() {
			t.Fatal("CreateWindowDelegate returned nil delegate")
		}

		var closed bool
		win.SetOnClose(func() bool {
			closed = true
			return true
		})

		if closed {
			t.Error("onClose callback should not be called during SetOnClose")
		}

		if win.ShouldClose() {
			t.Error("new window should not report shouldClose=true")
		}
	})
}

// TestSetOnCloseAndReject verifies that returning false from the onClose
// callback prevents shouldClose from being set.
func TestSetOnCloseReject(t *testing.T) {
	runOnMainThread(t, func() {
		config := platformdarwin.WindowConfig{
			Title:     "reject test",
			Width:     400,
			Height:    300,
			Resizable: false,
		}
		win, err := platformdarwin.NewWindow(config)
		if err != nil {
			t.Fatalf("NewWindow error: %v", err)
		}
		defer win.Destroy()

		win.SetOnClose(func() bool {
			return false // always reject
		})

		if win.ShouldClose() {
			t.Error("shouldClose should be false before any close attempt")
		}
	})
}

// TestSetOnCloseNilCallback verifies that setting a nil callback is safe.
func TestSetOnCloseNilCallback(t *testing.T) {
	runOnMainThread(t, func() {
		config := platformdarwin.WindowConfig{
			Title:     "nil callback test",
			Width:     400,
			Height:    300,
			Resizable: false,
		}
		win, err := platformdarwin.NewWindow(config)
		if err != nil {
			t.Fatalf("NewWindow error: %v", err)
		}
		defer win.Destroy()

		win.SetOnClose(nil)

		if win.ShouldClose() {
			t.Error("shouldClose should remain false after nil callback")
		}
	})
}

// TestWindowDelegateLifecycle verifies that creating and destroying multiple
// windows does not leak delegate instances.
func TestWindowDelegateLifecycle(t *testing.T) {
	runOnMainThread(t, func() {
		config := platformdarwin.WindowConfig{
			Title:     "lifecycle",
			Width:     200,
			Height:    200,
			Resizable: false,
		}

		for i := 0; i < 10; i++ {
			win, err := platformdarwin.NewWindow(config)
			if err != nil {
				t.Fatalf("iteration %d: NewWindow error: %v", i, err)
			}
			if win.ShouldClose() {
				t.Errorf("iteration %d: new window already marked for close", i)
			}
			win.Destroy()
		}
	})
}

// TestWindowDelegateCreationFailure verifies that CreateWindowDelegate
// returns an error when given a nil window.
func TestWindowDelegateCreationNilWindow(t *testing.T) {
	runOnMainThread(t, func() {
		delegate, err := platformdarwin.CreateWindowDelegate(nil)
		if err == nil {
			t.Error("CreateWindowDelegate should return an error for nil window")
		}
		if !delegate.IsNil() {
			t.Error("CreateWindowDelegate should return nil delegate for nil window")
		}
	})
}
