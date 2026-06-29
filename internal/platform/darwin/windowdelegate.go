//go:build darwin

package darwin

import (
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

var (
	goGPUWindowDelegateClass     Class
	goGPUWindowDelegateClassOnce sync.Once
	errGoGPUWindowDelegateClass  error
)

func WindowDelegateClass() (Class, error) {
	goGPUWindowDelegateClassOnce.Do(
		func() {
			goGPUWindowDelegateClass, errGoGPUWindowDelegateClass = registerWindowDelegateClass()
		},
	)
	return goGPUWindowDelegateClass, errGoGPUWindowDelegateClass
}

func registerWindowDelegateClass() (Class, error) {
	if err := initRuntime(); err != nil {
		return 0, err
	}
	nsObjectClass := GetClass("NSObject")
	if nsObjectClass == 0 {
		return 0, ErrClassNotFound
	}
	cls := AllocateClassPair(nsObjectClass, "GoGPUWindowDelegate")
	if cls == 0 {
		return 0, ErrClassNotFound
	}

	shouldCloseIMP := ffi.NewCallback(
		func(self, sel, sender uintptr) uintptr {
			win := getWindowFromDelegate(ID(self))

			if win == nil {
				return 1
			}
			win.mu.Lock()
			defer win.mu.Unlock()

			if win.onClose != nil && !win.onClose() {
				return 0
			}
			win.shouldClose = true
			return 1
		},
	)
	ClassAddMethod(cls, selectors.windowShouldClose, shouldCloseIMP, "B@:@")

	// windowDidChangeScreen: fires when the window moves to a display with
	// a different backing scale factor (e.g. Retina ↔ 1× transition).
	// Signal the per-window channel and wake the event loop so that the
	// physical-size check in pollEvents/checkResize runs promptly.
	screenChangedIMP := ffi.NewCallback(
		func(self, sel, notification uintptr) uintptr {
			win := getWindowFromDelegate(ID(self))
			if win != nil {
				select {
				case win.screenChangedCh <- struct{}{}:
				default:
				}
				WakeEventLoop()
			}
			return 0
		},
	)
	ClassAddMethod(cls, selectors.windowDidChangeScreen, screenChangedIMP, "v@:@")

	// windowDidResize: fires after each resize step, including during AppKit's
	// live-resize modal loop (inside sendEvent:) where our outer event loop is
	// blocked. Only invoke the hook while InLiveResize is true — this limits
	// the render trigger to user-drag resize and avoids spurious calls during
	// programmatic frame changes and window setup.
	didResizeIMP := ffi.NewCallback(
		func(self, sel, notification uintptr) uintptr {
			if win := getWindowFromDelegate(ID(self)); win != nil && win.InLiveResize() {
				if hook := win.liveResizeHookValue(); hook != nil {
					hook()
				}
			}
			return 0
		},
	)
	ClassAddMethod(cls, selectors.windowDidResize, didResizeIMP, "v@:@")

	// windowWillStartLiveResize: fires when the user begins dragging a resize handle.
	// Mark the window as in-resize so the app-level event filter can suppress
	// intermediate resize events until the user releases the mouse.
	willStartLiveResizeIMP := ffi.NewCallback(
		func(self, sel, notification uintptr) uintptr {
			if win := getWindowFromDelegate(ID(self)); win != nil {
				win.StartLiveResize()
			}
			return 0
		},
	)
	ClassAddMethod(cls, selectors.windowWillStartLiveResize, willStartLiveResizeIMP, "v@:@")

	// windowDidEndLiveResize: fires when the user releases the resize handle.
	// Clear the flag and wake the event loop so a final EventResize is emitted.
	didEndLiveResizeIMP := ffi.NewCallback(
		func(self, sel, notification uintptr) uintptr {
			if win := getWindowFromDelegate(ID(self)); win != nil {
				win.EndLiveResize()
			}
			return 0
		},
	)
	ClassAddMethod(cls, selectors.windowDidEndLiveResize, didEndLiveResizeIMP, "v@:@")

	RegisterClassPair(cls)
	return cls, nil
}

func CreateWindowDelegate(win *Window) (ID, error) {
	if win == nil {
		return 0, ErrWindowCreationFailed
	}

	cls, err := WindowDelegateClass()
	if err != nil {
		return 0, err
	}
	alloc := ID(cls).Send(selectors.alloc)
	if alloc.IsNil() {
		return 0, ErrWindowCreationFailed
	}
	delegate := alloc.Send(selectors.init)
	if delegate.IsNil() {
		return 0, ErrWindowCreationFailed
	}
	SetAssociatedObject(delegate, unsafe.Pointer(&delegateAssociatedKey), unsafe.Pointer(win), 0)

	return delegate, nil
}

var delegateAssociatedKey = "com.gpu.gowindow.delegate"

func getWindowFromDelegate(delegate ID) *Window {
	ptr := GetAssociatedObject(delegate, unsafe.Pointer(&delegateAssociatedKey))
	if ptr == nil {
		return nil
	}
	return (*Window)(ptr)
}
