// Package eventqueue provides a fixed-capacity ring buffer for platform events.
//
// The ring buffer is pre-allocated at construction time and performs zero
// allocations after initialization. Push and Pop are O(1). Thread-safe via
// internal mutex. On overflow the oldest event is dropped (SDL3 pattern).
//
// Design follows SDL3 (fixed cap, drop oldest) and winit (bounded channel).
// See ADR-031 and EVENT-QUEUE-RING-BUFFER-RESEARCH.md.
package eventqueue

import "sync"

// DefaultCapacity is the default ring buffer capacity.
// 256 events handles ~4 seconds of 60 Hz trackpad scroll at full speed.
const DefaultCapacity = 256

// Queue is a fixed-capacity ring buffer for events of type T.
// Pre-allocated, zero allocations after init, O(1) push/pop.
// Thread-safe via internal mutex. Drops oldest event on overflow.
type Queue[T any] struct {
	mu    sync.Mutex
	buf   []T
	head  int // read position
	tail  int // write position
	count int // current number of events
}

// New creates a new Queue with the given capacity.
// If capacity <= 0, DefaultCapacity is used.
func New[T any](capacity int) *Queue[T] {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Queue[T]{buf: make([]T, capacity)}
}

// Push adds an event to the queue. If the queue is full, the oldest
// event is dropped to make room (SDL3 overflow policy).
func (q *Queue[T]) Push(e T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == len(q.buf) {
		// Queue full — drop oldest (SDL3 pattern).
		var zero T
		q.buf[q.head] = zero // zero out to release references
		q.head = (q.head + 1) % len(q.buf)
		q.count--
	}
	q.buf[q.tail] = e
	q.tail = (q.tail + 1) % len(q.buf)
	q.count++
}

// Pop removes and returns the oldest event from the queue.
// Returns the zero value of T and false if the queue is empty.
func (q *Queue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == 0 {
		var zero T
		return zero, false
	}
	e := q.buf[q.head]
	var zero T
	q.buf[q.head] = zero // zero out to prevent holding references
	q.head = (q.head + 1) % len(q.buf)
	q.count--
	return e, true
}

// Len returns the number of events currently in the queue.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.count
}

// CoalesceLast scans the queue from tail to head looking for an event that
// matches the predicate. If found, it replaces that event with the new one
// and returns true (coalesced). If not found, returns false (caller should Push).
// This is useful for resize event coalescing (Windows WM_SIZE storm).
func (q *Queue[T]) CoalesceLast(match func(T) bool, replacement T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == 0 {
		return false
	}

	// Scan from newest to oldest for efficiency (resize likely recent).
	for i := q.count - 1; i >= 0; i-- {
		idx := (q.head + i) % len(q.buf)
		if match(q.buf[idx]) {
			q.buf[idx] = replacement
			return true
		}
	}
	return false
}
