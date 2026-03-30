package gogpu

import (
	"math"
	"sync"

	"github.com/gogpu/wgpu"
)

// submissionTracker tracks GPU submissions for non-blocking resource recycling.
// Each Submit returns a monotonic submission index. Command buffers are stored
// and freed only when PollCompleted confirms the GPU has finished using them.
//
// This replaces the previous FencePool + SubmissionTracker combination with a
// simpler design: the HAL manages fences internally, and we only track
// (submissionIndex, commandBuffers) pairs.
type submissionTracker struct {
	mu     sync.Mutex
	active []trackedSubmission
}

// trackedSubmission pairs a submission index with the command buffers that
// must remain alive until the GPU finishes the submission.
type trackedSubmission struct {
	index   uint64
	cmdBufs []*wgpu.CommandBuffer
}

// track records a new in-flight submission with its command buffers.
func (t *submissionTracker) track(idx uint64, cmdBufs ...*wgpu.CommandBuffer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = append(t.active, trackedSubmission{index: idx, cmdBufs: cmdBufs})
}

// triage frees command buffers for submissions that have completed.
// completedIdx is the highest submission index known to be done (from Queue.Poll).
func (t *submissionTracker) triage(completedIdx uint64, device *wgpu.Device) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var remaining []trackedSubmission
	for _, s := range t.active {
		if s.index <= completedIdx {
			for _, cb := range s.cmdBufs {
				device.FreeCommandBuffer(cb)
			}
		} else {
			remaining = append(remaining, s)
		}
	}
	t.active = remaining
}

// waitAll waits for all GPU work to finish, then frees all tracked resources.
// Uses WaitIdle as a heavy-weight barrier; call only during shutdown.
func (t *submissionTracker) waitAll(device *wgpu.Device) {
	_ = device.WaitIdle()
	t.triage(math.MaxUint64, device)
}

// activeCount returns the number of submissions currently in-flight.
func (t *submissionTracker) activeCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.active)
}
