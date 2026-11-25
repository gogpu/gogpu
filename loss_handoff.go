package gogpu

import (
	"fmt"
	"sync/atomic"
)

// maxSurfaceRecoverAttempts is the number of consecutive surface-loss episodes
// without a successful configure after which the loss escalates to device loss.
// Without it, "recoverable, keep going" would never end on a backend that keeps
// reporting a lost surface.
const maxSurfaceRecoverAttempts = 3

// lossSignal carries error identity from the render thread to the event-loop
// goroutine (P2-API-009).
//
// Producer (render thread): the record* helpers store the wrapped error and
// bump the surface sequence; device loss is terminal and subsumes any pending
// surface loss.
//
// Consumer (event-loop goroutine): ConsumeLoss delivers the device error once
// (it is terminal), or one surface episode per sequence bump. Device loss
// arriving before a pending surface episode is delivered means only the device
// callback fires — the surface episode is dropped, by rule, not racing.
//
// Re-arm is producer-side only: an episode ends when the surface configures
// again (SurfaceLost → SurfaceConfigured); the next transition into SurfaceLost
// bumps the sequence again. The consumer never resets producer state, so a
// dispatch in flight cannot race a re-arm — the worst case is one extra
// episode observed after the app already saw recovery, which is exactly the
// next loss.
type lossSignal struct {
	surfaceEpisode atomic.Uint64
	surfaceErr     atomic.Pointer[error]
	deviceErr      atomic.Pointer[error]

	// deliveredEpisode is consumer-owned: written only on the event-loop
	// goroutine (ConsumeLoss), read only there.
	deliveredEpisode uint64
	deviceDelivered  bool
}

// recordSurfaceLost is called on the render thread when a surface-loss episode
// begins. It stores the episode and wakes the event loop so delivery does not
// wait for an unrelated event.
func (r *Renderer) recordSurfaceLost(cause error) {
	if r.loss.deviceErr.Load() != nil {
		return // terminal: nothing after device loss
	}
	err := fmt.Errorf("%w: %w", ErrSurfaceLost, cause)
	r.loss.surfaceErr.Store(&err)
	r.loss.surfaceEpisode.Add(1)
	r.wakeLoop()
}

// recordDeviceLost is called on the render thread when the device is lost. It
// is terminal: later surface losses are ignored and a pending surface episode
// is dropped by the consumer's dominance rule.
func (r *Renderer) recordDeviceLost(cause error) {
	err := fmt.Errorf("%w: %w", ErrDeviceLost, cause)
	if r.loss.deviceErr.Swap(&err) != nil {
		return
	}
	r.wakeLoop()
}

// wakeLoop nudges the event loop after a loss is recorded. Set by App before
// the main loop starts; RequestRedraw is safe from any goroutine.
func (r *Renderer) wakeLoop() {
	if r.wake != nil {
		r.wake()
	}
}

// ConsumeLoss returns the pending device loss once, or the next undelivered
// surface episode. It must be called from the event-loop goroutine only.
func (r *Renderer) ConsumeLoss() (surface, device error) {
	if !r.loss.deviceDelivered {
		if e := r.loss.deviceErr.Load(); e != nil {
			r.loss.deviceDelivered = true
			// Device loss subsumes every pending surface episode — mark them
			// delivered so a stale slot can never surface after it.
			r.loss.deliveredEpisode = r.loss.surfaceEpisode.Load()
			return nil, *e
		}
	}
	if seq := r.loss.surfaceEpisode.Load(); seq != r.loss.deliveredEpisode {
		r.loss.deliveredEpisode = seq
		if e := r.loss.surfaceErr.Load(); e != nil {
			return *e, nil
		}
	}
	return nil, nil
}
