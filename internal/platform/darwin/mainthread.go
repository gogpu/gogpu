//go:build darwin

package darwin

import (
	"errors"
	"sync"

	"github.com/go-webgpu/goffi/ffi"
)

// ErrMainThreadDispatch reports that a Cocoa operation could not be queued on
// the process' main thread.
var ErrMainThreadDispatch = errors.New("darwin: main-thread dispatch unavailable")

var mainThreadTasks struct {
	once    sync.Once
	class   Class
	invoke  SEL
	perform SEL
	err     error
	tasks   sync.Map // uintptr(NSObject) -> func()
}

// IsMainThread reports whether the current goroutine is running on AppKit's
// main thread. Unlike runtime.LockOSThread, this check is valid for callers
// that arrive from a background goroutine and is implemented by Foundation's
// NSThread class.
func IsMainThread() bool {
	threadClass := GetClass("NSThread")
	if threadClass == 0 {
		return false
	}
	return ID(threadClass).GetBool(RegisterSelector("isMainThread"))
}

func initMainThreadTasks() error {
	mainThreadTasks.once.Do(func() {
		if err := initRuntime(); err != nil {
			mainThreadTasks.err = err
			return
		}
		initSelectors()
		initClasses()
		super := classes.NSObject
		if super == 0 {
			mainThreadTasks.err = ErrMainThreadDispatch
			return
		}

		// A small NSObject subclass lets us use the documented
		// performSelectorOnMainThread:withObject:waitUntilDone: API without
		// introducing a C/CGO dispatch shim. The callback is retained in a Go
		// map keyed by the target object's address; no Go pointer crosses the
		// Objective-C boundary.
		cls := AllocateClassPair(super, "GoGPUMainThreadTask")
		if cls == 0 {
			// Tests and embedders can initialize this package more than once in
			// one process. Reuse a class registered by an earlier initializer.
			cls = GetClass("GoGPUMainThreadTask")
		}
		if cls == 0 {
			mainThreadTasks.err = ErrMainThreadDispatch
			return
		}

		invoke := RegisterSelector("gogpuPerform:")
		imp := ffi.NewCallback(func(self, _, _ uintptr) uintptr {
			if value, ok := mainThreadTasks.tasks.LoadAndDelete(self); ok {
				value.(func())()
			}
			// The object was allocated solely for this invocation. Release the
			// initial retain after the callback has run.
			ID(self).Send(selectors.release)
			return 0
		})
		if !ClassAddMethod(cls, invoke, imp, "v@:@") {
			// If another package registered the same class concurrently, the
			// existing method is sufficient; otherwise report the failure.
			if GetClass("GoGPUMainThreadTask") == 0 {
				mainThreadTasks.err = ErrMainThreadDispatch
				return
			}
		}
		if GetClass("GoGPUMainThreadTask") == 0 {
			RegisterClassPair(cls)
		}
		mainThreadTasks.class = cls
		mainThreadTasks.invoke = invoke
		mainThreadTasks.perform = RegisterSelector(
			"performSelectorOnMainThread:withObject:waitUntilDone:")
	})
	return mainThreadTasks.err
}

// PerformOnMain invokes fn on AppKit's main thread. If wait is true, the
// caller blocks until fn has returned; otherwise the function returns after
// the invocation has been queued. The asynchronous form is used for modal
// print operations so App.Print can return its PrintJob before the panel is
// dismissed. performSelectorOnMainThread also works while AppKit is inside a
// nested sheet/modal event loop, which is required for cancellation.
func PerformOnMain(fn func(), wait bool) error {
	if fn == nil {
		return nil
	}
	if wait && IsMainThread() {
		fn()
		return nil
	}
	if err := initMainThreadTasks(); err != nil {
		return err
	}
	if mainThreadTasks.class == 0 || mainThreadTasks.perform == 0 {
		return ErrMainThreadDispatch
	}

	target := ID(mainThreadTasks.class).Send(selectors.alloc).Send(selectors.init)
	if target == 0 {
		return ErrMainThreadDispatch
	}

	var done chan struct{}
	if wait {
		done = make(chan struct{})
		mainThreadTasks.tasks.Store(target.Ptr(), func() {
			defer close(done)
			fn()
		})
	} else {
		mainThreadTasks.tasks.Store(target.Ptr(), fn)
	}

	waitArg := uintptr(0)
	if wait {
		waitArg = 1
	}
	target.SendPtrs(mainThreadTasks.perform, mainThreadTasks.invoke.SELPtr(), 0, waitArg)
	if wait {
		<-done
	}
	return nil
}
