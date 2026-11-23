//go:build darwin

package darwin

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestRunInFramePool_HoldsThreadForPoolLifetime stresses the thread-locality
// of the NSAutoreleasePool: the pool belongs to the OS thread that created it,
// and draining it from another thread corrupts ObjC's thread-local pool stack
// (the macOS CI SIGSEGV of 2026-11-23 landed in that drain). The goroutine
// sleeps inside the pool, which releases its M, while other goroutines keep
// the other Ps busy, so it may be rescheduled onto a different OS thread
// before the deferred drain; with the pin in RunInFramePool the thread cannot
// change across the pair.
//
// This is a guard, not the reproducing case: on the pre-fix code it did not
// crash in a 1000-iteration run — the original crash needed the natural CI
// timing. The thread-locality argument is proved by code path: production
// callers hold a locked thread, the unfaithful test mock did not, and the
// drain received a message on a thread that did not own the pool.
func TestRunInFramePool_HoldsThreadForPoolLifetime(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		runtime.GOMAXPROCS(2)
	}

	stop := make(chan struct{})
	var spin sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		spin.Add(1)
		go func() {
			defer spin.Done()
			for {
				select {
				case <-stop:
					return
				default:
					runtime.Gosched()
				}
			}
		}()
	}
	defer func() {
		close(stop)
		spin.Wait()
	}()

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		RunInFramePool(func() {
			// Releasing the M here is the point: the goroutine may resume on
			// another OS thread, and the drain must still run on the thread
			// that owns the pool.
			time.Sleep(time.Millisecond)
		})
	}
}
