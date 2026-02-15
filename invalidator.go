package gogpu

// Invalidator provides goroutine-safe redraw request coalescing.
//
// Uses a buffered channel (capacity 1) as a lock-free coalescing signal.
// Multiple concurrent Invalidate() calls produce exactly one wakeup.
// This follows the Gio pattern for cross-goroutine invalidation.
type Invalidator struct {
	ch     chan struct{}
	wakeup func()
}

func newInvalidator(wakeup func()) *Invalidator {
	return &Invalidator{
		ch:     make(chan struct{}, 1),
		wakeup: wakeup,
	}
}

// Invalidate requests a redraw. Safe to call from any goroutine.
// Multiple concurrent calls coalesce into a single redraw.
func (inv *Invalidator) Invalidate() {
	select {
	case inv.ch <- struct{}{}:
		// First signal — wake the event loop
		if inv.wakeup != nil {
			inv.wakeup()
		}
	default:
		// Already signaled — coalesced
	}
}

// Consume drains the pending redraw signal.
// Returns true if a redraw was requested since the last Consume call.
// Called from the main thread in the event loop.
func (inv *Invalidator) Consume() bool {
	select {
	case <-inv.ch:
		return true
	default:
		return false
	}
}
