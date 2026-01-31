package gogpu

import (
	"sync"
	"testing"
)

func TestSubmissionTracker_NewSubmissionTracker(t *testing.T) {
	tracker := NewSubmissionTracker()
	if tracker == nil {
		t.Fatal("NewSubmissionTracker returned nil")
	}
	if tracker.LatestIndex() != 0 {
		t.Errorf("expected latest index 0, got %d", tracker.LatestIndex())
	}
	if tracker.CompletedIndex() != 0 {
		t.Errorf("expected completed index 0, got %d", tracker.CompletedIndex())
	}
	if tracker.ActiveCount() != 0 {
		t.Errorf("expected active count 0, got %d", tracker.ActiveCount())
	}
}

func TestSubmissionTracker_NextIndex(t *testing.T) {
	tracker := NewSubmissionTracker()

	// First index should be 1
	idx1 := tracker.NextIndex()
	if idx1 != 1 {
		t.Errorf("expected first index 1, got %d", idx1)
	}

	// Second index should be 2
	idx2 := tracker.NextIndex()
	if idx2 != 2 {
		t.Errorf("expected second index 2, got %d", idx2)
	}

	// LatestIndex should reflect the most recent
	if tracker.LatestIndex() != 2 {
		t.Errorf("expected latest index 2, got %d", tracker.LatestIndex())
	}
}

func TestSubmissionTracker_Track(t *testing.T) {
	tracker := NewSubmissionTracker()

	// Track some submissions
	tracker.Track(1)
	tracker.Track(2)
	tracker.Track(3)

	if tracker.ActiveCount() != 3 {
		t.Errorf("expected active count 3, got %d", tracker.ActiveCount())
	}
}

func TestSubmissionTracker_Triage(t *testing.T) {
	tracker := NewSubmissionTracker()

	// Track submissions 1, 2, 3
	tracker.Track(1)
	tracker.Track(2)
	tracker.Track(3)

	// Triage with fence value 0 (nothing completed)
	if tracker.Triage(0) {
		t.Error("expected no triaging for fence value 0")
	}
	if tracker.ActiveCount() != 3 {
		t.Errorf("expected active count 3 after triage(0), got %d", tracker.ActiveCount())
	}

	// Triage with fence value 2 (submissions 1 and 2 completed)
	if !tracker.Triage(2) {
		t.Error("expected triaging for fence value 2")
	}
	if tracker.ActiveCount() != 1 {
		t.Errorf("expected active count 1 after triage(2), got %d", tracker.ActiveCount())
	}
	if tracker.CompletedIndex() != 2 {
		t.Errorf("expected completed index 2, got %d", tracker.CompletedIndex())
	}

	// Triage with same fence value again (no new completions)
	if tracker.Triage(2) {
		t.Error("expected no triaging for same fence value")
	}

	// Triage with fence value 3 (submission 3 completed)
	if !tracker.Triage(3) {
		t.Error("expected triaging for fence value 3")
	}
	if tracker.ActiveCount() != 0 {
		t.Errorf("expected active count 0 after triage(3), got %d", tracker.ActiveCount())
	}
	if tracker.CompletedIndex() != 3 {
		t.Errorf("expected completed index 3, got %d", tracker.CompletedIndex())
	}
}

func TestSubmissionTracker_Concurrency(t *testing.T) {
	tracker := NewSubmissionTracker()

	var wg sync.WaitGroup
	numGoroutines := 10
	submissionsPerGoroutine := 100

	// Concurrent NextIndex and Track
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < submissionsPerGoroutine; j++ {
				idx := tracker.NextIndex()
				tracker.Track(idx)
			}
		}()
	}
	wg.Wait()

	expectedTotal := numGoroutines * submissionsPerGoroutine
	if int(tracker.LatestIndex()) != expectedTotal {
		t.Errorf("expected latest index %d, got %d", expectedTotal, tracker.LatestIndex())
	}
	if tracker.ActiveCount() != expectedTotal {
		t.Errorf("expected active count %d, got %d", expectedTotal, tracker.ActiveCount())
	}

	// Concurrent Triage - complete all
	if !tracker.Triage(uint64(expectedTotal)) {
		t.Error("expected triaging")
	}
	if tracker.ActiveCount() != 0 {
		t.Errorf("expected active count 0 after full triage, got %d", tracker.ActiveCount())
	}
}

func TestSubmissionTracker_TriageLowerValue(t *testing.T) {
	tracker := NewSubmissionTracker()

	tracker.Track(1)
	tracker.Track(2)

	// Triage up to 2
	tracker.Triage(2)

	// Track more
	tracker.Track(3)
	tracker.Track(4)

	// Triage with a value lower than completed should be no-op
	if tracker.Triage(1) {
		t.Error("expected no triaging for value lower than completed index")
	}
	if tracker.ActiveCount() != 2 {
		t.Errorf("expected active count 2, got %d", tracker.ActiveCount())
	}
}
