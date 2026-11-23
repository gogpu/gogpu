//go:build darwin

package darwin

import "runtime"

// RunInFramePool executes fn inside a fresh NSAutoreleasePool.
//
// Use on the render thread once per frame. Metal drawables and transient ObjC
// objects are autoreleased; without per-frame draining, IOSurface memory grows
// during live window resize (Apple MTLBestPracticesGuide, imgui #2910).
//
// The pool belongs to the OS thread that creates it: draining it from another
// thread corrupts ObjC's thread-local pool stack (seen as a SIGSEGV inside
// drain when the goroutine migrated across the pair). The function therefore
// owns its affinity for the pool's lifetime: LockOSThread before the pool
// exists, UnlockOSThread after the drain. The lock is counted, so a caller
// that already pinned its thread — the render thread (internal/thread) and the
// main goroutine (init.go) — keeps its pin after the unlock. AppKit's
// main-thread requirement is separate and unchanged: this wraps the caller's
// work on the caller's thread; it never moves work between threads.
func RunInFramePool(fn func()) {
	initSelectors()
	initClasses()

	runtime.LockOSThread()
	pool := classes.NSAutoreleasePool.Send(selectors.new)
	if pool.IsNil() {
		// No pool to drain; the thread still must be released.
		defer runtime.UnlockOSThread()
		fn()
		return
	}
	// Defers run LIFO: the drain happens first, still on the locked thread,
	// and the unlock follows it.
	defer runtime.UnlockOSThread()
	defer pool.Send(selectors.drain)
	fn()
}
