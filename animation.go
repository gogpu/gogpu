package gogpu

import "sync/atomic"

// AnimationController tracks active animations to determine rendering mode.
// When animations are active, the main loop renders at VSync rate (continuous).
// When all animations complete, the loop returns to idle (0% CPU).
//
// Thread-safe: StartAnimation and IsAnimating can be called from any goroutine.
type AnimationController struct {
	count int32 // atomic: number of active animations
}

// StartAnimation signals that an animation has started.
// The main loop will render continuously until all animations complete.
// Call Stop() on the returned token when the animation is done.
func (ac *AnimationController) StartAnimation() *AnimationToken {
	atomic.AddInt32(&ac.count, 1)
	return &AnimationToken{ac: ac}
}

// IsAnimating returns true if any animations are currently active.
func (ac *AnimationController) IsAnimating() bool {
	return atomic.LoadInt32(&ac.count) > 0
}

func (ac *AnimationController) stopAnimation() {
	atomic.AddInt32(&ac.count, -1)
}

// AnimationToken represents an active animation.
// Call Stop() when the animation completes to allow the render loop to idle.
type AnimationToken struct {
	ac      *AnimationController
	stopped atomic.Bool
}

// Stop signals the animation is complete. Idempotent -- safe to call multiple times.
func (t *AnimationToken) Stop() {
	if t.stopped.CompareAndSwap(false, true) {
		t.ac.stopAnimation()
	}
}
