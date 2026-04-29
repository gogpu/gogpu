//go:build darwin

package darwin

import (
	"sync"

	"github.com/go-webgpu/goffi/ffi"
)

// GoGPUView is a custom NSView subclass that overrides keyDown:, keyUp:,
// doCommandBySelector:, and acceptsFirstResponder. This prevents the macOS
// system beep (NSBeep) that occurs when the default NSView receives unhandled
// keyboard events.
//
// Every enterprise macOS framework uses this pattern:
// Qt6 (QNSView), Chromium (BridgedContentView), Flutter (FlutterViewController),
// GTK4 (GdkMacosBaseView), winit (WinitView), GLFW (GLFWContentView), SDL3.
//
// See ADR-015 for full analysis.

var (
	goGPUViewClass     Class
	goGPUViewClassOnce sync.Once
	errGoGPUViewClass  error
)

// GoGPUViewClass returns the registered GoGPUView ObjC class.
// The class is created once and reused for all windows.
func GoGPUViewClass() (Class, error) {
	goGPUViewClassOnce.Do(func() {
		goGPUViewClass, errGoGPUViewClass = registerGoGPUViewClass()
	})
	return goGPUViewClass, errGoGPUViewClass
}

func registerGoGPUViewClass() (Class, error) {
	if err := initRuntime(); err != nil {
		return 0, err
	}

	nsViewClass := GetClass("NSView")
	if nsViewClass == 0 {
		return 0, ErrClassNotFound
	}

	cls := AllocateClassPair(nsViewClass, "GoGPUView")
	if cls == 0 {
		return 0, ErrClassNotFound
	}

	// keyDown: — handle keyboard event, prevent default NSBeep chain.
	// ObjC signature: -(void)keyDown:(NSEvent*)event → "v@:@"
	keyDownIMP := ffi.NewCallback(func(self, sel, event uintptr) uintptr {
		// Event is already handled by our handleEvent Go function
		// which runs before sendEvent:. Nothing to do here —
		// the override itself prevents [super keyDown:] from running.
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("keyDown:"), keyDownIMP, "v@:@")

	// keyUp: — same pattern, prevent super from running.
	keyUpIMP := ffi.NewCallback(func(self, sel, event uintptr) uintptr {
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("keyUp:"), keyUpIMP, "v@:@")

	// flagsChanged: — modifier key events (Shift, Cmd, etc.)
	flagsChangedIMP := ffi.NewCallback(func(self, sel, event uintptr) uintptr {
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("flagsChanged:"), flagsChangedIMP, "v@:@")

	// doCommandBySelector: — no-op. This is the method that calls NSBeep()
	// when no responder handles the command. GLFW uses empty {}, SDL3 same.
	doCommandIMP := ffi.NewCallback(func(self, sel, aSelector uintptr) uintptr {
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("doCommandBySelector:"), doCommandIMP, "v@::")

	// acceptsFirstResponder — return YES so the view receives key events.
	acceptsIMP := ffi.NewCallback(func(self, sel uintptr) uintptr {
		return 1 // YES
	})
	ClassAddMethod(cls, RegisterSelector("acceptsFirstResponder"), acceptsIMP, "B@:")

	RegisterClassPair(cls)
	return cls, nil
}

// CreateGoGPUView creates an instance of GoGPUView with the given frame rect.
// The returned ID is an allocated, initialized NSView subclass instance.
func CreateGoGPUView(frame NSRect) (ID, error) {
	cls, err := GoGPUViewClass()
	if err != nil {
		return 0, err
	}

	// [[GoGPUView alloc] initWithFrame:frame]
	alloc := ID(cls).Send(RegisterSelector("alloc"))
	if alloc.IsNil() {
		return 0, ErrViewCreationFailed
	}

	view := alloc.SendRect(RegisterSelector("initWithFrame:"), frame)
	if view.IsNil() {
		return 0, ErrViewCreationFailed
	}

	return view, nil
}
