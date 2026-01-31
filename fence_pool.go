package gogpu

import (
	"sync"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
)

// FencePool manages a pool of GPU fences for non-blocking submission tracking.
// Following wgpu-rs FencePool pattern for Vulkan binary fences.
//
// Each submission gets its own fence. When the fence is signaled (GPU completed),
// the fence is returned to the free pool for reuse.
//
// This enables non-blocking completion checks:
//   - Submit: get fence from pool, submit with fence, track (index, fence) pair
//   - PollCompleted: check all active fences, return max completed index
//   - Triage: move completed fences back to free pool
type FencePool struct {
	mu            sync.Mutex
	backend       gpu.Backend
	device        types.Device
	active        []activeFence // (submissionIndex, fence) pairs
	free          []types.Fence // fences ready for reuse
	lastCompleted types.SubmissionIndex
}

// activeFence tracks a fence and its associated submission index.
// Also tracks resources that must be released when the submission completes.
// Following wgpu-rs ActiveSubmission pattern: resources remain alive until GPU finishes.
type activeFence struct {
	index   types.SubmissionIndex
	fence   types.Fence
	cmdBufs []types.CommandBuffer // Command buffers to release when complete
}

// NewFencePool creates a new fence pool.
func NewFencePool(backend gpu.Backend, device types.Device) *FencePool {
	return &FencePool{
		backend: backend,
		device:  device,
	}
}

// AcquireFence gets a fence for a new submission.
// Returns a fence from the free pool or creates a new one.
func (p *FencePool) AcquireFence() (types.Fence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to get from free pool
	if len(p.free) > 0 {
		fence := p.free[len(p.free)-1]
		p.free = p.free[:len(p.free)-1]

		// Reset the fence for reuse
		if err := p.backend.ResetFence(p.device, fence); err != nil {
			// If reset fails, create a new fence instead
			return p.backend.CreateFence(p.device)
		}
		return fence, nil
	}

	// Create new fence
	return p.backend.CreateFence(p.device)
}

// TrackSubmission records that a submission was made with the given fence.
// Resources (command buffers) are stored and released only when the fence signals.
// This follows wgpu-rs pattern: resources must remain alive until GPU finishes.
func (p *FencePool) TrackSubmission(index types.SubmissionIndex, fence types.Fence, cmdBufs ...types.CommandBuffer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active = append(p.active, activeFence{
		index:   index,
		fence:   fence,
		cmdBufs: cmdBufs,
	})
}

// PollCompleted checks all active fences and returns the highest completed submission index.
// This is non-blocking - uses GetFenceStatus to poll without waiting.
// Also moves completed fences back to the free pool and releases associated resources.
// Following wgpu-rs pattern: resources are released only when GPU finishes using them.
func (p *FencePool) PollCompleted() types.SubmissionIndex {
	p.mu.Lock()
	defer p.mu.Unlock()

	var remaining []activeFence
	maxCompleted := p.lastCompleted

	for _, af := range p.active {
		signaled, err := p.backend.GetFenceStatus(af.fence)
		if err != nil {
			// On error, assume not signaled and keep tracking
			remaining = append(remaining, af)
			continue
		}

		if signaled {
			// Submission complete - release resources that were waiting
			for _, cmdBuf := range af.cmdBufs {
				p.backend.ReleaseCommandBuffer(cmdBuf)
			}
			// Fence can be reused
			p.free = append(p.free, af.fence)
			if af.index > maxCompleted {
				maxCompleted = af.index
			}
		} else {
			// Still in flight
			remaining = append(remaining, af)
		}
	}

	p.active = remaining
	p.lastCompleted = maxCompleted
	return maxCompleted
}

// LastCompleted returns the last known completed submission index.
func (p *FencePool) LastCompleted() types.SubmissionIndex {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastCompleted
}

// ActiveCount returns the number of submissions currently in-flight.
func (p *FencePool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.active)
}

// WaitAll waits for all active submissions to complete.
// Uses blocking wait on each active fence.
func (p *FencePool) WaitAll(timeoutNs uint64) {
	p.mu.Lock()
	fences := make([]activeFence, len(p.active))
	copy(fences, p.active)
	p.mu.Unlock()

	for _, af := range fences {
		_, _ = p.backend.WaitFence(p.device, af.fence, timeoutNs)
	}

	// Poll to update state
	p.PollCompleted()
}

// Destroy waits for all GPU work to complete, then releases all fences and resources.
// This is critical for proper cleanup - destroying active fences while
// GPU work is in-flight causes undefined behavior.
func (p *FencePool) Destroy() {
	// Wait for all submissions to complete (1 second timeout).
	// This must be done before taking the lock to avoid deadlock.
	p.WaitAll(1_000_000_000)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Release any remaining resources and fences (should be minimal after WaitAll)
	for _, af := range p.active {
		// Release pending command buffers
		for _, cmdBuf := range af.cmdBufs {
			p.backend.ReleaseCommandBuffer(cmdBuf)
		}
		p.backend.DestroyFence(p.device, af.fence)
	}
	p.active = nil

	// Destroy free fences
	for _, fence := range p.free {
		p.backend.DestroyFence(p.device, fence)
	}
	p.free = nil
}
