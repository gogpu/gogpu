package gogpu

import (
	"sync"
	"time"

	"github.com/gogpu/wgpu"
)

// FencePool manages a pool of GPU fences for non-blocking submission tracking.
// Following wgpu-rs FencePool pattern for Vulkan binary fences.
//
// Each submission gets its own fence. When the fence is signaled (GPU completed),
// the fence is returned to the free pool for reuse.
type FencePool struct {
	mu            sync.Mutex
	device        *wgpu.Device
	active        []activeFence // (submissionIndex, fence) pairs
	free          []*wgpu.Fence // fences ready for reuse
	lastCompleted uint64
}

// activeFence tracks a fence and its associated submission index.
// Also tracks resources that must be released when the submission completes.
type activeFence struct {
	index   uint64
	fence   *wgpu.Fence
	cmdBufs []*wgpu.CommandBuffer // Command buffers to release when complete
}

// NewFencePool creates a new fence pool.
func NewFencePool(device *wgpu.Device) *FencePool {
	return &FencePool{
		device: device,
	}
}

// AcquireFence gets a fence for a new submission.
// Returns a fence from the free pool or creates a new one.
func (p *FencePool) AcquireFence() (*wgpu.Fence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to get from free pool
	if len(p.free) > 0 {
		fence := p.free[len(p.free)-1]
		p.free = p.free[:len(p.free)-1]

		// Reset the fence for reuse
		if err := p.device.ResetFence(fence); err != nil {
			// If reset fails, create a new fence instead
			return p.device.CreateFence()
		}
		return fence, nil
	}

	// Create new fence
	return p.device.CreateFence()
}

// TrackSubmission records that a submission was made with the given fence.
// Resources (command buffers) are stored and released only when the fence signals.
func (p *FencePool) TrackSubmission(index uint64, fence *wgpu.Fence, cmdBufs ...*wgpu.CommandBuffer) {
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
func (p *FencePool) PollCompleted() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	var remaining []activeFence
	maxCompleted := p.lastCompleted

	for _, af := range p.active {
		signaled, err := p.device.GetFenceStatus(af.fence)
		if err != nil {
			remaining = append(remaining, af)
			continue
		}

		if signaled {
			// Submission complete - release resources
			for _, cmdBuf := range af.cmdBufs {
				p.device.FreeCommandBuffer(cmdBuf)
			}
			// Fence can be reused
			p.free = append(p.free, af.fence)
			if af.index > maxCompleted {
				maxCompleted = af.index
			}
		} else {
			remaining = append(remaining, af)
		}
	}

	p.active = remaining
	p.lastCompleted = maxCompleted
	return maxCompleted
}

// LastCompleted returns the last known completed submission index.
func (p *FencePool) LastCompleted() uint64 {
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
func (p *FencePool) WaitAll(timeout time.Duration) {
	p.mu.Lock()
	fences := make([]activeFence, len(p.active))
	copy(fences, p.active)
	p.mu.Unlock()

	for _, af := range fences {
		_, _ = p.device.WaitForFence(af.fence, af.index, timeout)
	}

	// Poll to update state
	p.PollCompleted()
}

// Destroy waits for all GPU work to complete, then releases all fences and resources.
func (p *FencePool) Destroy() {
	// Wait for all submissions to complete (1 second timeout).
	p.WaitAll(time.Second)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Release any remaining resources and fences
	for _, af := range p.active {
		for _, cmdBuf := range af.cmdBufs {
			p.device.FreeCommandBuffer(cmdBuf)
		}
		af.fence.Release()
	}
	p.active = nil

	// Destroy free fences
	for _, fence := range p.free {
		fence.Release()
	}
	p.free = nil
}
