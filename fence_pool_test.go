package gogpu

import (
	"sync"
	"testing"

	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/wgputest"
)

// newTestDevice returns a *wgpu.Device backed by the noop backend. The
// submission tracker only frees command buffers and waits idle, both of which
// the noop device performs without a GPU.
func newTestDevice(t *testing.T) *wgpu.Device {
	t.Helper()
	device, _, cleanup, err := wgputest.NewNoopDevice()
	if err != nil {
		t.Fatalf("wgputest.NewNoopDevice: %v", err)
	}
	t.Cleanup(cleanup)
	return device
}

func TestSubmissionTracker_Track(t *testing.T) {
	device := newTestDevice(t)

	var tracker submissionTracker
	tracker.track(1)
	tracker.track(2)

	if tracker.activeCount() != 2 {
		t.Errorf("expected 2 active, got %d", tracker.activeCount())
	}

	// Triage with index 1 — should free submission 1, keep 2
	tracker.triage(1, device)
	if tracker.activeCount() != 1 {
		t.Errorf("expected 1 active after triage(1), got %d", tracker.activeCount())
	}

	// Triage with index 2 — should free submission 2
	tracker.triage(2, device)
	if tracker.activeCount() != 0 {
		t.Errorf("expected 0 active after triage(2), got %d", tracker.activeCount())
	}
}

func TestSubmissionTracker_TriageFreesCmdBufs(t *testing.T) {
	device := newTestDevice(t)

	var tracker submissionTracker

	// Create mock command buffers via device (nil is fine for testing FreeCommandBuffer)
	tracker.track(1)
	tracker.track(2)

	// Triage all
	tracker.triage(3, device)
	if tracker.activeCount() != 0 {
		t.Errorf("expected 0 active, got %d", tracker.activeCount())
	}
}

func TestSubmissionTracker_WaitAll(t *testing.T) {
	device := newTestDevice(t)

	var tracker submissionTracker
	tracker.track(1)
	tracker.track(2)
	tracker.track(3)

	tracker.waitAll(device)
	if tracker.activeCount() != 0 {
		t.Errorf("expected 0 active after waitAll, got %d", tracker.activeCount())
	}
}

func TestSubmissionTracker_ConcurrentAccess(t *testing.T) {
	device := newTestDevice(t)

	var tracker submissionTracker
	var wg sync.WaitGroup

	// Concurrent tracks
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx uint64) {
			defer wg.Done()
			tracker.track(idx)
		}(uint64(i + 1))
	}
	wg.Wait()

	if tracker.activeCount() != 10 {
		t.Errorf("expected 10 active, got %d", tracker.activeCount())
	}

	// Triage all
	tracker.triage(10, device)
	if tracker.activeCount() != 0 {
		t.Errorf("expected 0 active, got %d", tracker.activeCount())
	}
}
