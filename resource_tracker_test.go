package gogpu

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

// mockCloser tracks Close calls for testing.
type mockCloser struct {
	closed    bool
	closeErr  error
	closeFunc func() error
	label     string
}

func (m *mockCloser) Close() error {
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return m.closeErr
}

func newMockCloser(label string) *mockCloser {
	return &mockCloser{label: label}
}

func TestResourceTracker_TrackAndCloseAll_LIFOOrder(t *testing.T) {
	tracker := &resourceTracker{}

	var order []string
	a := &mockCloser{closeFunc: func() error { order = append(order, "a"); return nil }}
	b := &mockCloser{closeFunc: func() error { order = append(order, "b"); return nil }}
	c := &mockCloser{closeFunc: func() error { order = append(order, "c"); return nil }}

	tracker.Track(a, "a")
	tracker.Track(b, "b")
	tracker.Track(c, "c")

	err := tracker.CloseAll()
	if err != nil {
		t.Fatalf("CloseAll returned unexpected error: %v", err)
	}

	// LIFO order: c, b, a
	if len(order) != 3 {
		t.Fatalf("expected 3 closes, got %d", len(order))
	}
	if order[0] != "c" || order[1] != "b" || order[2] != "a" {
		t.Errorf("expected LIFO order [c, b, a], got %v", order)
	}
}

func TestResourceTracker_Untrack(t *testing.T) {
	tracker := &resourceTracker{}

	a := newMockCloser("a")
	b := newMockCloser("b")
	c := newMockCloser("c")

	tracker.Track(a, "a")
	tracker.Track(b, "b")
	tracker.Track(c, "c")

	// Untrack b
	tracker.Untrack(b)

	_ = tracker.CloseAll()

	if !a.closed {
		t.Error("a should have been closed")
	}
	if b.closed {
		t.Error("b should NOT have been closed (was untracked)")
	}
	if !c.closed {
		t.Error("c should have been closed")
	}
}

func TestResourceTracker_DoubleClose(t *testing.T) {
	tracker := &resourceTracker{}

	a := newMockCloser("a")
	tracker.Track(a, "a")

	err1 := tracker.CloseAll()
	if err1 != nil {
		t.Fatalf("first CloseAll returned error: %v", err1)
	}

	if !a.closed {
		t.Error("a should have been closed")
	}

	// Second CloseAll should be no-op
	a.closed = false // reset
	err2 := tracker.CloseAll()
	if err2 != nil {
		t.Fatalf("second CloseAll returned error: %v", err2)
	}
	if a.closed {
		t.Error("a should NOT have been closed again (tracker already closed)")
	}
}

func TestResourceTracker_TrackAfterShutdown(t *testing.T) {
	tracker := &resourceTracker{}

	// Close the tracker
	_ = tracker.CloseAll()

	// Track after shutdown should close immediately
	a := newMockCloser("a")
	tracker.Track(a, "a")

	if !a.closed {
		t.Error("resource tracked after shutdown should be closed immediately")
	}
}

func TestResourceTracker_ConcurrentTrackAndCloseAll(t *testing.T) {
	tracker := &resourceTracker{}

	var closeCount atomic.Int64
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines + 1) // goroutines for Track + 1 for CloseAll

	// Start goroutines that Track resources
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			m := &mockCloser{
				closeFunc: func() error {
					closeCount.Add(1)
					return nil
				},
			}
			tracker.Track(m, "concurrent")
		}()
	}

	// Start one goroutine that calls CloseAll
	go func() {
		defer wg.Done()
		_ = tracker.CloseAll()
	}()

	wg.Wait()

	// All resources should have been closed (either by CloseAll or immediate close)
	// The exact count depends on timing, but should be goroutines
	count := closeCount.Load()
	if count != goroutines {
		t.Errorf("expected %d closes, got %d", goroutines, count)
	}
}

func TestResourceTracker_CloseAllContinuesOnPanic(t *testing.T) {
	tracker := &resourceTracker{}

	a := newMockCloser("a")
	panicker := &mockCloser{
		closeFunc: func() error {
			panic("intentional panic in closer")
		},
	}
	c := newMockCloser("c")

	tracker.Track(a, "a")
	tracker.Track(panicker, "panicker")
	tracker.Track(c, "c")

	// CloseAll should NOT panic, should continue closing remaining resources.
	err := tracker.CloseAll()

	// First error is from the panicker (LIFO: c first, then panicker, then a)
	if err == nil {
		t.Error("expected error from panicking closer")
	}

	if !a.closed {
		t.Error("a should have been closed despite panic in another closer")
	}
	if !c.closed {
		t.Error("c should have been closed")
	}
}

func TestResourceTracker_CloseAllReturnsFirstError(t *testing.T) {
	tracker := &resourceTracker{}

	errFirst := errors.New("first error")
	errSecond := errors.New("second error")

	a := &mockCloser{closeErr: errSecond}
	b := &mockCloser{closeErr: errFirst}

	tracker.Track(a, "a")
	tracker.Track(b, "b")

	// LIFO: b closes first (returns errFirst), then a (returns errSecond)
	err := tracker.CloseAll()
	if !errors.Is(err, errFirst) {
		t.Errorf("expected first error %v, got %v", errFirst, err)
	}
}

func TestResourceTracker_TrackNil(t *testing.T) {
	tracker := &resourceTracker{}

	// Should not panic
	tracker.Track(nil, "nil")

	err := tracker.CloseAll()
	if err != nil {
		t.Fatalf("CloseAll returned error: %v", err)
	}
}

func TestResourceTracker_UntrackNil(t *testing.T) {
	tracker := &resourceTracker{}

	// Should not panic
	tracker.Untrack(nil)
}

func TestResourceTracker_UntrackNotTracked(t *testing.T) {
	tracker := &resourceTracker{}

	a := newMockCloser("a")
	b := newMockCloser("b")

	tracker.Track(a, "a")

	// Untrack something that was never tracked — should be no-op
	tracker.Untrack(b)

	_ = tracker.CloseAll()

	if !a.closed {
		t.Error("a should have been closed")
	}
	if b.closed {
		t.Error("b should NOT have been closed (was never tracked)")
	}
}

func TestResourceTracker_EmptyCloseAll(t *testing.T) {
	tracker := &resourceTracker{}

	err := tracker.CloseAll()
	if err != nil {
		t.Fatalf("CloseAll on empty tracker returned error: %v", err)
	}
}

// TestApp_ResourceTracker_Interface verifies App implements ResourceTracker.
func TestApp_ResourceTracker_Interface(t *testing.T) {
	var _ ResourceTracker = (*App)(nil)
}

// TestApp_TrackResource_LazyInit verifies tracker is initialized lazily.
func TestApp_TrackResource_LazyInit(t *testing.T) {
	app := &App{}

	if app.tracker != nil {
		t.Error("tracker should be nil initially")
	}

	m := newMockCloser("test")
	app.TrackResource(m)

	if app.tracker == nil {
		t.Error("tracker should be initialized after TrackResource")
	}
}

// TestApp_UntrackResource_NilTracker verifies UntrackResource is safe with nil tracker.
func TestApp_UntrackResource_NilTracker(t *testing.T) {
	app := &App{}

	// Should not panic
	app.UntrackResource(newMockCloser("test"))
}

// TestResourceTracker_SatisfiesInterface verifies the interface contract.
func TestResourceTracker_SatisfiesInterface(t *testing.T) {
	// Compile-time check
	var _ io.Closer = (*mockCloser)(nil)
}
