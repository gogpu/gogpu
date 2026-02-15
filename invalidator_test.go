package gogpu

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestInvalidator_BasicSignal(t *testing.T) {
	t.Run("Invalidate then Consume returns true", func(t *testing.T) {
		inv := newInvalidator(nil)
		inv.Invalidate()

		if !inv.Consume() {
			t.Error("expected Consume to return true after Invalidate")
		}
	})

	t.Run("second Consume returns false", func(t *testing.T) {
		inv := newInvalidator(nil)
		inv.Invalidate()
		inv.Consume() // drain

		if inv.Consume() {
			t.Error("expected Consume to return false after signal was drained")
		}
	})

	t.Run("Consume without Invalidate returns false", func(t *testing.T) {
		inv := newInvalidator(nil)

		if inv.Consume() {
			t.Error("expected Consume to return false without prior Invalidate")
		}
	})
}

func TestInvalidator_Coalescing(t *testing.T) {
	inv := newInvalidator(nil)

	// Multiple Invalidate calls before any Consume
	inv.Invalidate()
	inv.Invalidate()
	inv.Invalidate()

	// Should produce exactly one signal
	if !inv.Consume() {
		t.Error("expected Consume to return true after multiple Invalidate calls")
	}

	if inv.Consume() {
		t.Error("expected second Consume to return false — signals should coalesce")
	}
}

func TestInvalidator_WakeupCalled(t *testing.T) {
	var called atomic.Int32

	inv := newInvalidator(func() {
		called.Add(1)
	})

	inv.Invalidate()

	if got := called.Load(); got != 1 {
		t.Errorf("expected wakeup called 1 time, got %d", got)
	}
}

func TestInvalidator_WakeupNotCalledOnCoalesced(t *testing.T) {
	var called atomic.Int32

	inv := newInvalidator(func() {
		called.Add(1)
	})

	// First Invalidate — wakeup should fire
	inv.Invalidate()
	// Second Invalidate — channel full, wakeup should NOT fire
	inv.Invalidate()

	if got := called.Load(); got != 1 {
		t.Errorf("expected wakeup called exactly 1 time, got %d", got)
	}
}

func TestInvalidator_NilWakeup(t *testing.T) {
	inv := newInvalidator(nil)

	// Should not panic with nil wakeup
	inv.Invalidate()
	inv.Invalidate()

	if !inv.Consume() {
		t.Error("expected Consume to return true")
	}
}

func TestInvalidator_ConcurrentAccess(t *testing.T) {
	var wakeups atomic.Int32

	inv := newInvalidator(func() {
		wakeups.Add(1)
	})

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			inv.Invalidate()
		}()
	}

	wg.Wait()

	// After all goroutines complete, exactly one signal should be pending
	if !inv.Consume() {
		t.Error("expected Consume to return true after concurrent Invalidate calls")
	}

	if inv.Consume() {
		t.Error("expected second Consume to return false — all signals should coalesce")
	}

	// Wakeup should have been called at least once (first signal) but at most once
	// (channel blocks subsequent signals from reaching the wakeup branch).
	if got := wakeups.Load(); got != 1 {
		t.Errorf("expected wakeup called exactly 1 time, got %d", got)
	}
}

func TestInvalidator_InvalidateAfterConsume(t *testing.T) {
	inv := newInvalidator(nil)

	// First cycle
	inv.Invalidate()
	if !inv.Consume() {
		t.Error("expected Consume to return true on first cycle")
	}

	// Second cycle — fresh signal after drain
	inv.Invalidate()
	if !inv.Consume() {
		t.Error("expected Consume to return true on second cycle")
	}

	// No pending signal
	if inv.Consume() {
		t.Error("expected Consume to return false after draining second cycle")
	}
}
