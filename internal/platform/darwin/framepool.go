//go:build darwin

package darwin

// PushFramePool opens a fresh NSAutoreleasePool for a frame split across two
// calls; PopFramePool drains it.
func PushFramePool() uintptr {
	initSelectors()
	initClasses()
	return uintptr(classes.NSAutoreleasePool.Send(selectors.new))
}

// PopFramePool drains the pool PushFramePool returned.
func PopFramePool(pool uintptr) {
	ID(pool).Send(selectors.drain)
}

// RunInFramePool executes fn inside a fresh NSAutoreleasePool.
//
// Use on the render thread once per frame. Metal drawables and transient ObjC
// objects are autoreleased; without per-frame draining, IOSurface memory grows
// during live window resize (Apple MTLBestPracticesGuide, imgui #2910).
func RunInFramePool(fn func()) {
	initSelectors()
	initClasses()

	pool := classes.NSAutoreleasePool.Send(selectors.new)
	defer pool.Send(selectors.drain)
	fn()
}
