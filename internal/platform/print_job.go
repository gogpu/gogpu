package platform

import (
	"context"
	"sync"
)

// printJob is the backend-independent lifecycle half of a native print job.
// Platform implementations provide the cancellation hook and call complete
// only after their native operation has released all retained resources.
// Keeping this state machine in a common file makes cancellation and exactly
// one terminal Done value deterministic on every platform.
type printJob struct {
	done     chan error
	finished chan struct{}

	mu              sync.Mutex
	terminal        bool
	cancelRequested bool
	cancelFn        func()
}

func newPrintJob() *printJob {
	return &printJob{
		done:     make(chan error, 1),
		finished: make(chan struct{}),
	}
}

func (j *printJob) Done() <-chan error { return j.done }

// setCancel installs the native cancellation hook. If cancellation won the
// race with native setup, the hook is invoked immediately after installation.
func (j *printJob) setCancel(fn func()) {
	j.mu.Lock()
	if j.terminal {
		j.mu.Unlock()
		return
	}
	j.cancelFn = fn
	canceled := j.cancelRequested
	j.mu.Unlock()
	if canceled && fn != nil {
		fn()
	}
}

// clearCancel removes the native cancellation hook once the backend has
// released the resource it protects.  A terminal job ignores later Cancel
// calls, but clearing the hook before completion also prevents a concurrent
// cancellation from touching a recycled native handle during cleanup.
func (j *printJob) clearCancel() {
	j.mu.Lock()
	j.cancelFn = nil
	j.mu.Unlock()
}

func (j *printJob) Cancel() {
	j.mu.Lock()
	if j.terminal || j.cancelRequested {
		j.mu.Unlock()
		return
	}
	j.cancelRequested = true
	fn := j.cancelFn
	j.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (j *printJob) canceled() bool {
	j.mu.Lock()
	canceled := j.cancelRequested
	j.mu.Unlock()
	return canceled
}

// complete publishes exactly one terminal result and closes Done. A caller
// cancellation always wins over a concurrent native success callback.
func (j *printJob) complete(err error) {
	j.mu.Lock()
	if j.terminal {
		j.mu.Unlock()
		return
	}
	if j.cancelRequested {
		err = context.Canceled
	}
	j.terminal = true
	j.done <- err
	close(j.done)
	close(j.finished)
	j.mu.Unlock()
}

// watchPrintContext turns context cancellation into the same idempotent
// native cancellation path as PrintJob.Cancel. The watcher exits once the
// backend has published its terminal result.
func watchPrintContext(ctx context.Context, job *printJob) {
	if ctx == nil {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			job.Cancel()
		case <-job.finished:
		}
	}()
}
