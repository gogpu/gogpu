//go:build darwin

package darwin_test

import (
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

type testNSRange struct {
	location uint64
	length   uint64
}

var testNSRangeType = &types.TypeDescriptor{
	Size:      16,
	Alignment: 8,
	Kind:      types.StructType,
	Members: []*types.TypeDescriptor{
		types.UInt64TypeDescriptor,
		types.UInt64TypeDescriptor,
	},
}

func objcArgNSRange(value testNSRange) objcArg {
	return objcArg{typ: testNSRangeType, ptr: unsafe.Pointer(&value), keepAlive: &value}
}

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

// TestGoGPUViewTextInputBridge exercises the registered AppKit methods on a
// real NSTextView subclass. It verifies that native setMarkedText, insertText
// and unmarkText dispatch into the Go handlers while forwarding to NSTextView.
func TestGoGPUViewTextInputBridge(t *testing.T) {
	rt := loadObjcRuntime(t)
	var marked, committed string
	var unmarked bool
	runOnMainThread(t, func() {
		withAutoreleasePool(t, rt, func() {
			view, err := platformdarwin.CreateGoGPUView(platformdarwin.MakeRect(0, 0, 320, 200))
			if err != nil {
				t.Fatalf("CreateGoGPUView() error: %v", err)
			}
			defer view.Send(platformdarwin.SEL(rt.sel(t, "release")))
			if !platformdarwin.ViewSupportsTextInput(view) {
				t.Fatal("GoGPUView did not inherit NSTextInputClient support")
			}
			platformdarwin.SetViewTextInputHandler(view, platformdarwin.TextInputHandler{
				SetMarkedText: func(text platformdarwin.ID, _, _, _, _ uintptr) {
					marked = platformdarwin.ObjectString(text)
				},
				InsertText: func(text platformdarwin.ID, _, _ uintptr) {
					committed = platformdarwin.ObjectString(text)
				},
				UnmarkText: func() { unmarked = true },
			})
			defer platformdarwin.ClearViewTextInputHandler(view)

			preedit := platformdarwin.NewNSString("かな")
			if preedit == nil || preedit.ID() == 0 {
				t.Fatal("NewNSString(preedit) returned nil")
			}
			objcCallVoid(t, rt, view.Ptr(), rt.sel(t, "setMarkedText:selectedRange:replacementRange:"),
				objcArgPtr(preedit.ID().Ptr()),
				objcArgNSRange(testNSRange{location: 2}),
				objcArgNSRange(testNSRange{location: ^uint64(0), length: ^uint64(0)}),
			)
			preedit.Release()
			if marked != "かな" {
				t.Fatalf("setMarkedText callback = %q, want %q", marked, "かな")
			}

			result := platformdarwin.NewNSString("仮名")
			if result == nil || result.ID() == 0 {
				t.Fatal("NewNSString(commit) returned nil")
			}
			objcCallVoid(t, rt, view.Ptr(), rt.sel(t, "insertText:replacementRange:"),
				objcArgPtr(result.ID().Ptr()),
				objcArgNSRange(testNSRange{location: ^uint64(0), length: ^uint64(0)}),
			)
			result.Release()
			if committed != "仮名" {
				t.Fatalf("insertText callback = %q, want %q", committed, "仮名")
			}
			if got := platformdarwin.ObjectString(view.Send(platformdarwin.RegisterSelector("string"))); got != "" {
				t.Fatalf("native text storage after commit = %q, want empty", got)
			}
			preedit = platformdarwin.NewNSString("に")
			objcCallVoid(t, rt, view.Ptr(), rt.sel(t, "setMarkedText:selectedRange:replacementRange:"),
				objcArgPtr(preedit.ID().Ptr()), objcArgNSRange(testNSRange{location: 1}),
				objcArgNSRange(testNSRange{location: ^uint64(0), length: ^uint64(0)}))
			preedit.Release()
			if got := platformdarwin.ObjectString(view.Send(platformdarwin.RegisterSelector("string"))); got != "に" {
				t.Fatalf("native storage for second preedit = %q, want only the new preedit", got)
			}

			objcCallVoid(t, rt, view.Ptr(), rt.sel(t, "unmarkText"))
			if !unmarked {
				t.Fatal("unmarkText callback was not delivered")
			}
			if got := platformdarwin.ObjectString(view.Send(platformdarwin.RegisterSelector("string"))); got != "" {
				t.Fatalf("native text storage after unmark = %q, want empty", got)
			}
		})
	})
}

// makeTestCallback creates a no-op ObjC IMP that calls fn when invoked.
func makeTestCallback(fn func()) uintptr {
	return ffi.NewCallback(func(self, sel uintptr) uintptr {
		fn()
		return 0
	})
}
