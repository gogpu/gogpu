package gogpu

import (
	"sync"

	"github.com/gogpu/gogpu/gpu/types"
)

// SubmissionTracker tracks GPU submissions for non-blocking resource recycling.
// Follows wgpu-rs LifetimeTracker pattern where each Submit() returns a SubmissionIndex,
// and the fence is signaled with that value when the GPU completes the work.
//
// This enables efficient resource management without per-frame blocking:
// - Submit() increments and returns the submission index
// - Track() records that a submission is active
// - Triage() processes completed submissions based on fence value
//
// Usage:
//
//	tracker := NewSubmissionTracker()
//	subIdx := tracker.NextIndex()
//	backend.Submit(queue, commands, fence, subIdx)
//	tracker.Track(subIdx)
//	// Later, after some frames:
//	fenceValue, _ := backend.GetFenceValue(fence)
//	tracker.Triage(fenceValue)  // Recycles completed submissions
type SubmissionTracker struct {
	mu           sync.Mutex
	nextIndex    types.SubmissionIndex
	active       []activeSubmission
	completedIdx types.SubmissionIndex
}

// activeSubmission represents a submission that is still in-flight on the GPU.
type activeSubmission struct {
	index types.SubmissionIndex
	// Future: Resources to recycle when submission completes.
	// For now, we just track the index for the infrastructure.
}

// NewSubmissionTracker creates a new submission tracker.
// The tracker starts with submission index 0, so the first submission is 1.
func NewSubmissionTracker() *SubmissionTracker {
	return &SubmissionTracker{
		nextIndex: 0,
	}
}

// NextIndex returns the next submission index and increments the counter.
// This should be called before Submit() to get the value to pass to the fence.
func (t *SubmissionTracker) NextIndex() types.SubmissionIndex {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nextIndex++
	return t.nextIndex
}

// Track records a new submission as active.
// Call this after Submit() succeeds.
func (t *SubmissionTracker) Track(index types.SubmissionIndex) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = append(t.active, activeSubmission{index: index})
}

// Triage processes completed submissions based on the current fence value.
// Returns true if any submissions were triaged (completed).
//
// This is the key method for non-blocking resource recycling:
// - Call GetFenceValue() to get the current fence value (non-blocking)
// - Pass that value to Triage() to mark submissions as complete
// - All submissions with index <= fenceValue are considered complete
func (t *SubmissionTracker) Triage(fenceValue uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	fv := types.SubmissionIndex(fenceValue)
	if fv <= t.completedIdx {
		return false // No new completions
	}

	var remaining []activeSubmission
	for _, sub := range t.active {
		if sub.index > fv {
			// Still in-flight, keep tracking
			remaining = append(remaining, sub)
		}
		// Submissions with index <= fv are complete.
		// Future: Call resource cleanup callbacks here.
	}

	triaged := len(t.active) != len(remaining)
	t.active = remaining
	t.completedIdx = fv
	return triaged
}

// CompletedIndex returns the last known completed submission index.
// All submissions with index <= this value have been processed by the GPU.
func (t *SubmissionTracker) CompletedIndex() types.SubmissionIndex {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.completedIdx
}

// LatestIndex returns the most recent submission index.
// This is the index that will be signaled when the latest submission completes.
func (t *SubmissionTracker) LatestIndex() types.SubmissionIndex {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.nextIndex
}

// ActiveCount returns the number of submissions currently in-flight.
func (t *SubmissionTracker) ActiveCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.active)
}
