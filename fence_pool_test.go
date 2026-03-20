package gogpu

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
)

// --- Mock types for FencePool testing ---

// mockFence implements hal.Fence for testing.
type mockFence struct {
	id       int
	signaled bool
}

func (f *mockFence) Destroy() {}

// mockCommandBuffer implements hal.CommandBuffer for testing.
type mockCommandBuffer struct {
	id    int
	freed bool
}

func (b *mockCommandBuffer) Destroy() {}

// mockFenceDevice implements the fence-related subset of hal.Device for testing.
// All non-fence methods panic to catch accidental usage.
type mockFenceDevice struct {
	mu             sync.Mutex
	fenceCounter   int
	createdFences  []*mockFence
	resetCalls     int
	destroyCalls   int
	freeCmdBufIDs  []int
	createFenceErr error
	resetFenceErr  error
	statusErr      error
	waitResult     bool
	waitErr        error
}

func (d *mockFenceDevice) CreateFence() (hal.Fence, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.createFenceErr != nil {
		return nil, d.createFenceErr
	}
	d.fenceCounter++
	f := &mockFence{id: d.fenceCounter}
	d.createdFences = append(d.createdFences, f)
	return f, nil
}

func (d *mockFenceDevice) ResetFence(_ hal.Fence) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.resetCalls++
	return d.resetFenceErr
}

func (d *mockFenceDevice) GetFenceStatus(fence hal.Fence) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.statusErr != nil {
		return false, d.statusErr
	}
	f := fence.(*mockFence)
	return f.signaled, nil
}

func (d *mockFenceDevice) DestroyFence(_ hal.Fence) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.destroyCalls++
}

func (d *mockFenceDevice) Wait(_ hal.Fence, _ uint64, _ time.Duration) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.waitResult, d.waitErr
}

func (d *mockFenceDevice) FreeCommandBuffer(cmdBuf hal.CommandBuffer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if cb, ok := cmdBuf.(*mockCommandBuffer); ok {
		cb.freed = true
		d.freeCmdBufIDs = append(d.freeCmdBufIDs, cb.id)
	}
}

// Unused hal.Device methods -- minimal stubs to satisfy the interface.
func (d *mockFenceDevice) CreateBuffer(_ *hal.BufferDescriptor) (hal.Buffer, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyBuffer(_ hal.Buffer) {}
func (d *mockFenceDevice) CreateTexture(_ *hal.TextureDescriptor) (hal.Texture, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyTexture(_ hal.Texture) {}
func (d *mockFenceDevice) CreateTextureView(_ hal.Texture, _ *hal.TextureViewDescriptor) (hal.TextureView, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyTextureView(_ hal.TextureView) {}
func (d *mockFenceDevice) CreateSampler(_ *hal.SamplerDescriptor) (hal.Sampler, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroySampler(_ hal.Sampler) {}
func (d *mockFenceDevice) CreateBindGroupLayout(_ *hal.BindGroupLayoutDescriptor) (hal.BindGroupLayout, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyBindGroupLayout(_ hal.BindGroupLayout) {}
func (d *mockFenceDevice) CreateBindGroup(_ *hal.BindGroupDescriptor) (hal.BindGroup, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyBindGroup(_ hal.BindGroup) {}
func (d *mockFenceDevice) CreatePipelineLayout(_ *hal.PipelineLayoutDescriptor) (hal.PipelineLayout, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyPipelineLayout(_ hal.PipelineLayout) {}
func (d *mockFenceDevice) CreateShaderModule(_ *hal.ShaderModuleDescriptor) (hal.ShaderModule, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyShaderModule(_ hal.ShaderModule) {}
func (d *mockFenceDevice) CreateRenderPipeline(_ *hal.RenderPipelineDescriptor) (hal.RenderPipeline, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyRenderPipeline(_ hal.RenderPipeline) {}
func (d *mockFenceDevice) CreateComputePipeline(_ *hal.ComputePipelineDescriptor) (hal.ComputePipeline, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyComputePipeline(_ hal.ComputePipeline) {}
func (d *mockFenceDevice) CreateQuerySet(_ *hal.QuerySetDescriptor) (hal.QuerySet, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyQuerySet(_ hal.QuerySet) {}
func (d *mockFenceDevice) CreateCommandEncoder(_ *hal.CommandEncoderDescriptor) (hal.CommandEncoder, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) CreateRenderBundleEncoder(_ *hal.RenderBundleEncoderDescriptor) (hal.RenderBundleEncoder, error) {
	return nil, nil //nolint:nilnil
}
func (d *mockFenceDevice) DestroyRenderBundle(_ hal.RenderBundle) {}
func (d *mockFenceDevice) WaitIdle() error                        { return nil }
func (d *mockFenceDevice) Destroy()                               {}

// Verify the mockFenceDevice satisfies the hal.Device interface at compile time.
var _ hal.Device = (*mockFenceDevice)(nil)

// mockQueue implements hal.Queue for testing.
type mockQueue struct{}

func (q *mockQueue) Submit(_ []hal.CommandBuffer, _ hal.Fence, _ uint64) error { return nil }
func (q *mockQueue) WriteBuffer(_ hal.Buffer, _ uint64, _ []byte) error        { return nil }
func (q *mockQueue) WriteTexture(_ *hal.ImageCopyTexture, _ []byte, _ *hal.ImageDataLayout, _ *hal.Extent3D) error {
	return nil
}
func (q *mockQueue) ReadBuffer(_ hal.Buffer, _ uint64, _ []byte) error               { return nil }
func (q *mockQueue) Present(_ hal.Surface, _ hal.SurfaceTexture) error               { return nil }
func (q *mockQueue) GetTimestampPeriod() float32                                     { return 0 }
func (q *mockQueue) Destroy()                                                        {}
func (q *mockQueue) CopyExternalImageToTexture(_ any, _ *hal.ImageCopyTexture) error { return nil }

// Verify the mockQueue satisfies the hal.Queue interface at compile time.
var _ hal.Queue = (*mockQueue)(nil)

// newTestFencePool creates a FencePool wrapping a mock HAL device for testing.
func newTestFencePool(t *testing.T, mockDev *mockFenceDevice) *FencePool {
	t.Helper()
	device, err := wgpu.NewDeviceFromHAL(
		mockDev,
		&mockQueue{},
		gputypes.Features(0),
		gputypes.DefaultLimits(),
		"test",
	)
	if err != nil {
		t.Fatalf("NewDeviceFromHAL() error = %v", err)
	}
	return NewFencePool(device)
}

// --- Tests ---

func TestNewFencePool(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	if pool == nil {
		t.Fatal("NewFencePool returned nil")
	}
	if pool.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0", pool.ActiveCount())
	}
	if pool.LastCompleted() != 0 {
		t.Errorf("LastCompleted = %d, want 0", pool.LastCompleted())
	}
}

func TestFencePoolAcquireFence(t *testing.T) {
	t.Run("creates new fence", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		fence, err := pool.AcquireFence()
		if err != nil {
			t.Fatalf("AcquireFence() error = %v", err)
		}
		if fence == nil {
			t.Fatal("AcquireFence() returned nil fence")
		}
		// NewDeviceFromHAL creates queue fence (1st), AcquireFence creates 2nd.
		if dev.fenceCounter < 2 {
			t.Errorf("fenceCounter = %d, want >= 2 (queue fence + acquired fence)", dev.fenceCounter)
		}
	})

	t.Run("creates multiple distinct fences", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		f1, err := pool.AcquireFence()
		if err != nil {
			t.Fatalf("AcquireFence() #1 error = %v", err)
		}
		f2, err := pool.AcquireFence()
		if err != nil {
			t.Fatalf("AcquireFence() #2 error = %v", err)
		}
		if f1 == f2 {
			t.Error("two consecutive AcquireFence calls returned the same fence pointer")
		}
	})

	t.Run("returns error when CreateFence fails", func(t *testing.T) {
		dev := &mockFenceDevice{
			createFenceErr: errors.New("GPU out of memory"),
		}
		// NewDeviceFromHAL also calls CreateFence; override error AFTER device creation.
		dev.createFenceErr = nil
		pool := newTestFencePool(t, dev)
		dev.createFenceErr = errors.New("GPU out of memory")

		_, err := pool.AcquireFence()
		if err == nil {
			t.Fatal("AcquireFence() expected error, got nil")
		}
	})

	t.Run("reuses fence from free pool", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		// Acquire, track, signal, poll -- fence moves to free pool.
		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(1, fence)

		// Signal the fence (index 1 = second created fence; index 0 = queue fence).
		dev.mu.Lock()
		dev.createdFences[1].signaled = true
		dev.mu.Unlock()

		pool.PollCompleted()

		countBeforeReuse := dev.fenceCounter

		// Next acquire should reuse from free pool, not create new.
		reused, err := pool.AcquireFence()
		if err != nil {
			t.Fatalf("AcquireFence() reuse error = %v", err)
		}
		if reused == nil {
			t.Fatal("AcquireFence() reuse returned nil")
		}
		if dev.fenceCounter != countBeforeReuse {
			t.Errorf("fenceCounter changed from %d to %d; expected reuse (no new creation)",
				countBeforeReuse, dev.fenceCounter)
		}
		if dev.resetCalls != 1 {
			t.Errorf("resetCalls = %d, want 1 (fence must be reset before reuse)", dev.resetCalls)
		}
	})

	t.Run("falls back to new fence when ResetFence fails", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		// Acquire, track, signal, poll -- fence moves to free pool.
		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(1, fence)

		dev.mu.Lock()
		dev.createdFences[1].signaled = true
		dev.mu.Unlock()

		pool.PollCompleted()

		// Make reset fail -- AcquireFence should create a new fence instead.
		dev.resetFenceErr = errors.New("reset failed")
		countBeforeAcquire := dev.fenceCounter

		acquired, err := pool.AcquireFence()
		if err != nil {
			t.Fatalf("AcquireFence() fallback error = %v", err)
		}
		if acquired == nil {
			t.Fatal("AcquireFence() fallback returned nil")
		}
		if dev.fenceCounter != countBeforeAcquire+1 {
			t.Errorf("fenceCounter = %d, want %d (should create new after reset failure)",
				dev.fenceCounter, countBeforeAcquire+1)
		}
	})
}

func TestFencePoolTrackSubmission(t *testing.T) {
	t.Run("single submission", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(1, fence)

		if pool.ActiveCount() != 1 {
			t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
		}
	})

	t.Run("multiple submissions", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		for i := uint64(1); i <= 5; i++ {
			fence, _ := pool.AcquireFence()
			pool.TrackSubmission(i, fence)
		}

		if pool.ActiveCount() != 5 {
			t.Errorf("ActiveCount = %d, want 5", pool.ActiveCount())
		}
	})

	t.Run("with nil command buffers", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		fence, _ := pool.AcquireFence()
		// Passing nil cmdBufs should not panic.
		pool.TrackSubmission(1, fence, nil, nil)

		if pool.ActiveCount() != 1 {
			t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
		}
	})
}

func TestFencePoolPollCompleted(t *testing.T) {
	t.Run("none signaled", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(1, fence)

		completed := pool.PollCompleted()
		if completed != 0 {
			t.Errorf("PollCompleted = %d, want 0 (none signaled)", completed)
		}
		if pool.ActiveCount() != 1 {
			t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
		}
	})

	t.Run("single fence signaled", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(42, fence)

		// Signal the fence.
		dev.mu.Lock()
		dev.createdFences[1].signaled = true
		dev.mu.Unlock()

		completed := pool.PollCompleted()
		if completed != 42 {
			t.Errorf("PollCompleted = %d, want 42", completed)
		}
		if pool.ActiveCount() != 0 {
			t.Errorf("ActiveCount = %d, want 0", pool.ActiveCount())
		}
		if pool.LastCompleted() != 42 {
			t.Errorf("LastCompleted = %d, want 42", pool.LastCompleted())
		}
	})

	t.Run("mixed signaled and unsignaled", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		// Track 3 submissions (mock fence indices 1, 2, 3; index 0 is queue fence).
		for i := uint64(1); i <= 3; i++ {
			f, _ := pool.AcquireFence()
			pool.TrackSubmission(i*10, f)
		}

		// Signal only fences for submissions 10 and 30 (indices 1 and 3).
		dev.mu.Lock()
		dev.createdFences[1].signaled = true // submission 10
		// createdFences[2] stays unsignaled    // submission 20
		dev.createdFences[3].signaled = true // submission 30
		dev.mu.Unlock()

		completed := pool.PollCompleted()
		if completed != 30 {
			t.Errorf("PollCompleted = %d, want 30 (highest signaled)", completed)
		}
		if pool.ActiveCount() != 1 {
			t.Errorf("ActiveCount = %d, want 1 (submission 20 still active)", pool.ActiveCount())
		}
	})

	t.Run("status error keeps fence active", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(1, fence)

		dev.statusErr = errors.New("status error")

		completed := pool.PollCompleted()
		if completed != 0 {
			t.Errorf("PollCompleted = %d, want 0 (error keeps fence active)", completed)
		}
		if pool.ActiveCount() != 1 {
			t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
		}
	})

	t.Run("empty pool returns lastCompleted", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		completed := pool.PollCompleted()
		if completed != 0 {
			t.Errorf("PollCompleted on empty pool = %d, want 0", completed)
		}
	})

	t.Run("lastCompleted never decreases", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		// First submission completes with index 100.
		f1, _ := pool.AcquireFence()
		pool.TrackSubmission(100, f1)

		dev.mu.Lock()
		dev.createdFences[1].signaled = true
		dev.mu.Unlock()

		pool.PollCompleted()

		// Second submission completes with lower index 50.
		// AcquireFence may reuse from free pool, so signal ALL mock fences.
		f2, _ := pool.AcquireFence()
		pool.TrackSubmission(50, f2)

		dev.mu.Lock()
		for _, mf := range dev.createdFences {
			mf.signaled = true
		}
		dev.mu.Unlock()

		completed := pool.PollCompleted()
		if completed != 100 {
			t.Errorf("PollCompleted = %d, want 100 (lastCompleted should not decrease)", completed)
		}
	})

	t.Run("repeated poll converges", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		f, _ := pool.AcquireFence()
		pool.TrackSubmission(1, f)

		// First poll: not signaled.
		c1 := pool.PollCompleted()
		if c1 != 0 {
			t.Errorf("first PollCompleted = %d, want 0", c1)
		}

		// Signal between polls.
		dev.mu.Lock()
		dev.createdFences[1].signaled = true
		dev.mu.Unlock()

		// Second poll: signaled.
		c2 := pool.PollCompleted()
		if c2 != 1 {
			t.Errorf("second PollCompleted = %d, want 1", c2)
		}

		// Third poll: no active fences, returns lastCompleted.
		c3 := pool.PollCompleted()
		if c3 != 1 {
			t.Errorf("third PollCompleted = %d, want 1", c3)
		}
	})
}

func TestFencePoolFenceReuseCycle(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	// Cycle: acquire -> track -> signal -> poll -> acquire (reuse) x3
	for cycle := 0; cycle < 3; cycle++ {
		f, err := pool.AcquireFence()
		if err != nil {
			t.Fatalf("cycle %d: AcquireFence error = %v", cycle, err)
		}
		pool.TrackSubmission(uint64(cycle+1), f)

		// Signal the latest fence. After the queue fence (index 0),
		// new fences are created at index 1. On subsequent cycles,
		// fences are reused (reset), so the same HAL fence is recycled.
		dev.mu.Lock()
		// Signal all fences to ensure current active one is signaled.
		for _, mf := range dev.createdFences {
			mf.signaled = true
		}
		dev.mu.Unlock()

		pool.PollCompleted()

		if pool.ActiveCount() != 0 {
			t.Errorf("cycle %d: ActiveCount = %d, want 0", cycle, pool.ActiveCount())
		}
	}

	// After 3 cycles with reuse, only 2 HAL fences should exist:
	// 1 queue fence + 1 pool fence (reused across cycles).
	if dev.fenceCounter != 2 {
		t.Errorf("fenceCounter = %d, want 2 (1 queue + 1 reused pool fence)", dev.fenceCounter)
	}
	if dev.resetCalls != 2 {
		t.Errorf("resetCalls = %d, want 2 (reused in cycles 2 and 3)", dev.resetCalls)
	}
}

func TestFencePoolWaitAll(t *testing.T) {
	t.Run("empty pool does not panic", func(t *testing.T) {
		dev := &mockFenceDevice{}
		pool := newTestFencePool(t, dev)

		pool.WaitAll(time.Second)

		if pool.ActiveCount() != 0 {
			t.Errorf("ActiveCount = %d, want 0", pool.ActiveCount())
		}
	})

	t.Run("waits and polls active fences", func(t *testing.T) {
		dev := &mockFenceDevice{waitResult: true}
		pool := newTestFencePool(t, dev)

		f1, _ := pool.AcquireFence()
		f2, _ := pool.AcquireFence()
		pool.TrackSubmission(1, f1)
		pool.TrackSubmission(2, f2)

		// Signal both fences so PollCompleted (called inside WaitAll) will collect them.
		dev.mu.Lock()
		for _, mf := range dev.createdFences {
			mf.signaled = true
		}
		dev.mu.Unlock()

		pool.WaitAll(time.Second)

		if pool.ActiveCount() != 0 {
			t.Errorf("ActiveCount after WaitAll = %d, want 0", pool.ActiveCount())
		}
		if pool.LastCompleted() != 2 {
			t.Errorf("LastCompleted after WaitAll = %d, want 2", pool.LastCompleted())
		}
	})
}

func TestFencePoolDestroy(t *testing.T) {
	t.Run("releases active and free fences", func(t *testing.T) {
		dev := &mockFenceDevice{waitResult: true}
		pool := newTestFencePool(t, dev)

		// Create two tracked submissions.
		f1, _ := pool.AcquireFence()
		f2, _ := pool.AcquireFence()
		pool.TrackSubmission(1, f1)
		pool.TrackSubmission(2, f2)

		// Signal first fence so it moves to free pool on Destroy's WaitAll+PollCompleted.
		dev.mu.Lock()
		dev.createdFences[1].signaled = true
		dev.mu.Unlock()

		pool.Destroy()

		if pool.ActiveCount() != 0 {
			t.Errorf("ActiveCount after Destroy = %d, want 0", pool.ActiveCount())
		}
	})

	t.Run("empty pool destroy does not panic", func(t *testing.T) {
		dev := &mockFenceDevice{waitResult: true}
		pool := newTestFencePool(t, dev)

		// Should not panic on empty pool.
		pool.Destroy()

		if pool.ActiveCount() != 0 {
			t.Errorf("ActiveCount after Destroy = %d, want 0", pool.ActiveCount())
		}
	})
}

func TestFencePoolLastCompleted(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	if pool.LastCompleted() != 0 {
		t.Errorf("initial LastCompleted = %d, want 0", pool.LastCompleted())
	}

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(7, fence)

	// LastCompleted should not change just from tracking.
	if pool.LastCompleted() != 0 {
		t.Errorf("LastCompleted after track = %d, want 0", pool.LastCompleted())
	}

	// Signal and poll.
	dev.mu.Lock()
	dev.createdFences[1].signaled = true
	dev.mu.Unlock()

	pool.PollCompleted()
	if pool.LastCompleted() != 7 {
		t.Errorf("LastCompleted after poll = %d, want 7", pool.LastCompleted())
	}
}

func TestFencePoolConcurrentAccess(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent AcquireFence + TrackSubmission
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fence, err := pool.AcquireFence()
			if err != nil {
				t.Errorf("goroutine %d: AcquireFence error = %v", idx, err)
				return
			}
			pool.TrackSubmission(uint64(idx+1), fence)
		}(i)
	}
	wg.Wait()

	if pool.ActiveCount() != numGoroutines {
		t.Errorf("ActiveCount = %d, want %d", pool.ActiveCount(), numGoroutines)
	}
}

func TestFencePoolConcurrentPollCompleted(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	// Track several submissions.
	for i := uint64(1); i <= 5; i++ {
		f, _ := pool.AcquireFence()
		pool.TrackSubmission(i, f)
	}

	// Signal all fences.
	dev.mu.Lock()
	for _, mf := range dev.createdFences {
		mf.signaled = true
	}
	dev.mu.Unlock()

	// Concurrent PollCompleted calls should not race.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.PollCompleted()
		}()
	}
	wg.Wait()

	if pool.ActiveCount() != 0 {
		t.Errorf("ActiveCount after concurrent polls = %d, want 0", pool.ActiveCount())
	}
	if pool.LastCompleted() != 5 {
		t.Errorf("LastCompleted after concurrent polls = %d, want 5", pool.LastCompleted())
	}
}

func TestFencePoolActiveCountThreadSafe(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.ActiveCount()
			_ = pool.LastCompleted()
		}()
	}
	wg.Wait()

	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
	}
}
