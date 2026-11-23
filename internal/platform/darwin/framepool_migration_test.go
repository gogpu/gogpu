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
// (observed as a SIGSEGV inside drain on macOS CI). The goroutine sleeps inside
// the pool, which releases its M, while other goroutines keep the other Ps
// busy, so it is likely rescheduled onto a different OS thread before the
// deferred drain. Without the pin in RunInFramePool this crashes or corrupts;
// with the pin the thread cannot change across the pair.
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
