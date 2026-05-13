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
