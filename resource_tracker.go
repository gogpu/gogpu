package gogpu

import (
	"fmt"
	"io"
	"sync"
)

// ResourceTracker is an optional interface that providers can implement
// to support automatic GPU resource lifecycle management.
//
// When a provider implements ResourceTracker, resources like ggcanvas.Canvas
// can register themselves for automatic cleanup during application shutdown.
// This eliminates the need for manual OnClose callbacks in most cases.
//
// App implements this interface.
type ResourceTracker interface {
	// TrackResource registers an io.Closer for automatic cleanup during shutdown.
	// Resources are closed in LIFO (reverse) order when the application exits.
	TrackResource(io.Closer)

	// UntrackResource removes a resource from automatic cleanup tracking.
	// Use this when a resource is closed manually before shutdown.
	UntrackResource(io.Closer)
}

// resourceTracker manages the lifecycle of tracked GPU resources.
// Resources are closed in LIFO order during shutdown to respect
// dependency ordering (resources created later may depend on earlier ones).
//
// Thread-safe: all methods are protected by a mutex.
type resourceTracker struct {
	mu        sync.Mutex
	resources []trackedResource
	closed    bool
}

// trackedResource holds a reference to a tracked resource.
type trackedResource struct {
	closer io.Closer
	label  string
}

// Track registers a resource for automatic cleanup.
// If the tracker is already closed, the resource is closed immediately.
func (t *resourceTracker) Track(c io.Closer, label string) {
	if c == nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		// Tracker already shut down — close the resource immediately.
		_ = c.Close()
		return
	}
	t.resources = append(t.resources, trackedResource{closer: c, label: label})
	t.mu.Unlock()
}

// Untrack removes a resource from tracking.
// Use this when a resource is closed manually before shutdown
// to prevent double-close.
func (t *resourceTracker) Untrack(c io.Closer) {
	if c == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for i, r := range t.resources {
		if r.closer == c {
			// Remove by swapping with last element (order doesn't matter for removal,
			// CloseAll iterates in reverse anyway).
			t.resources[i] = t.resources[len(t.resources)-1]
			t.resources[len(t.resources)-1] = trackedResource{} // Clear for GC
			t.resources = t.resources[:len(t.resources)-1]
			return
		}
	}
}

// CloseAll closes all tracked resources in LIFO (reverse) order.
// Each Close is wrapped in a deferred recover to continue on panic.
// Returns the first error encountered.
func (t *resourceTracker) CloseAll() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	// Copy and clear the slice under lock.
	resources := t.resources
	t.resources = nil
	t.mu.Unlock()

	var firstErr error

	// Close in LIFO order (last tracked = first closed).
	for i := len(resources) - 1; i >= 0; i-- {
		r := resources[i]
		if err := safeClose(r.closer); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// safeClose calls Close on the closer, recovering from panics.
func safeClose(c io.Closer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during Close: %v", r)
		}
	}()
	return c.Close()
}
