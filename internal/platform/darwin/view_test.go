//go:build darwin

package darwin_test

import (
	"testing"

	"github.com/go-webgpu/goffi/ffi"
	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

// TestAllocateClassPair verifies that a custom ObjC class can be created
// as a subclass of NSObject via the runtime API.
func TestAllocateClassPair(t *testing.T) {
	runOnMainThread(t, func() {
		nsObject := platformdarwin.GetClass("NSObject")
		if nsObject == 0 {
			t.Fatal("NSObject class not found")
		}

		cls := platformdarwin.AllocateClassPair(nsObject, "GoGPUTestClass")
		if cls == 0 {
			t.Fatal("AllocateClassPair returned 0")
		}

		platformdarwin.RegisterClassPair(cls)

		// Verify the class is now findable by name
		found := platformdarwin.GetClass("GoGPUTestClass")
		if found == 0 {
			t.Error("registered class not found via GetClass")
		}
	})
}

// TestAllocateClassPairDuplicate verifies that allocating a class with
// an already-registered name returns 0 (ObjC runtime rejects duplicates).
func TestAllocateClassPairDuplicate(t *testing.T) {
	runOnMainThread(t, func() {
		nsObject := platformdarwin.GetClass("NSObject")
		if nsObject == 0 {
			t.Fatal("NSObject class not found")
		}

		// NSObject already exists — should fail
		cls := platformdarwin.AllocateClassPair(nsObject, "NSObject")
		if cls != 0 {
			t.Error("AllocateClassPair should return 0 for duplicate class name")
		}
	})
}

// TestClassAddMethod verifies that a method can be added to a custom class.
func TestClassAddMethod(t *testing.T) {
	runOnMainThread(t, func() {
		nsObject := platformdarwin.GetClass("NSObject")
		if nsObject == 0 {
			t.Fatal("NSObject class not found")
		}

		cls := platformdarwin.AllocateClassPair(nsObject, "GoGPUTestMethodClass")
		if cls == 0 {
			t.Fatal("AllocateClassPair returned 0")
		}

		sel := platformdarwin.RegisterSelector("testMethod")

		// Use a no-op callback as IMP
		called := false
		imp := makeTestCallback(func() { called = true })

		ok := platformdarwin.ClassAddMethod(cls, sel, imp, "v@:")
		if !ok {
			t.Error("ClassAddMethod returned false")
		}

		platformdarwin.RegisterClassPair(cls)

		// Verify class was registered
		found := platformdarwin.GetClass("GoGPUTestMethodClass")
		if found == 0 {
			t.Error("class with added method not found")
		}
		_ = called // callback invocation requires msgSend to instance
	})
}

// TestGoGPUViewClassRegistration verifies that the GoGPUView class can be
// registered and returns a valid class pointer.
func TestGoGPUViewClassRegistration(t *testing.T) {
	runOnMainThread(t, func() {
		cls, err := platformdarwin.GoGPUViewClass()
		if err != nil {
			t.Fatalf("GoGPUViewClass() error: %v", err)
		}
		if cls == 0 {
			t.Fatal("GoGPUViewClass() returned 0")
		}

		// Verify class is findable by name
		found := platformdarwin.GetClass("GoGPUView")
		if found == 0 {
			t.Error("GoGPUView class not found via GetClass")
		}
		if found != cls {
			t.Errorf("GetClass returned %v, GoGPUViewClass returned %v", found, cls)
		}
	})
}

// TestGoGPUViewClassIdempotent verifies that calling GoGPUViewClass() multiple
// times returns the same class (sync.Once pattern).
func TestGoGPUViewClassIdempotent(t *testing.T) {
	runOnMainThread(t, func() {
		cls1, err1 := platformdarwin.GoGPUViewClass()
		cls2, err2 := platformdarwin.GoGPUViewClass()

		if err1 != nil || err2 != nil {
			t.Fatalf("errors: %v, %v", err1, err2)
		}
		if cls1 != cls2 {
			t.Errorf("GoGPUViewClass not idempotent: %v != %v", cls1, cls2)
		}
	})
}

// TestCreateGoGPUView verifies that a GoGPUView instance can be created
// with a given frame rect.
func TestCreateGoGPUView(t *testing.T) {
	runOnMainThread(t, func() {
		frame := platformdarwin.MakeRect(0, 0, 800, 600)
		view, err := platformdarwin.CreateGoGPUView(frame)
		if err != nil {
			t.Fatalf("CreateGoGPUView error: %v", err)
		}
		if view.IsNil() {
			t.Fatal("CreateGoGPUView returned nil view")
		}
	})
}

// makeTestCallback creates a no-op ObjC IMP that calls fn when invoked.
func makeTestCallback(fn func()) uintptr {
	return ffi.NewCallback(func(self, sel uintptr) uintptr {
		fn()
		return 0
	})
}
