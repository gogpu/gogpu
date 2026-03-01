package gogpu

import (
	"errors"
	"sync"
	"testing"
	"time"

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

// --- Tests ---

func TestNewFencePool(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

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

func TestFencePoolAcquireFenceCreatesNew(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	fence, err := pool.AcquireFence()
	if err != nil {
		t.Fatalf("AcquireFence() error = %v", err)
	}
	if fence == nil {
		t.Fatal("AcquireFence() returned nil fence")
	}
	if dev.fenceCounter != 1 {
		t.Errorf("fenceCounter = %d, want 1 (one fence created)", dev.fenceCounter)
	}
}

func TestFencePoolAcquireFenceReusesFromFreePool(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	// Acquire a fence, track it, signal it, poll to recycle
	fence1, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence1)

	// Signal the fence
	fence1.(*mockFence).signaled = true
	pool.PollCompleted()

	// Now acquire again -- should reuse the recycled fence
	fence2, err := pool.AcquireFence()
	if err != nil {
		t.Fatalf("AcquireFence() error = %v", err)
	}

	// Only 1 fence should have been created total (reused)
	if dev.fenceCounter != 1 {
		t.Errorf("fenceCounter = %d, want 1 (fence should be reused)", dev.fenceCounter)
	}
	if dev.resetCalls != 1 {
		t.Errorf("resetCalls = %d, want 1", dev.resetCalls)
	}
	if fence2 != fence1 {
		t.Error("Expected reused fence to be the same object")
	}
}

func TestFencePoolAcquireFenceResetFailsCreatesNew(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	// Acquire, track, signal, poll to recycle
	fence1, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence1)
	fence1.(*mockFence).signaled = true
	pool.PollCompleted()

	// Make reset fail so a new fence is created instead
	dev.resetFenceErr = errors.New("reset failed")

	fence2, err := pool.AcquireFence()
	if err != nil {
		t.Fatalf("AcquireFence() error = %v", err)
	}
	// Two fences created: original + fallback after reset failure
	if dev.fenceCounter != 2 {
		t.Errorf("fenceCounter = %d, want 2", dev.fenceCounter)
	}
	if fence2 == fence1 {
		t.Error("Expected a new fence, not the reset-failed one")
	}
}

func TestFencePoolAcquireFenceCreateError(t *testing.T) {
	dev := &mockFenceDevice{
		createFenceErr: errors.New("device lost"),
	}
	pool := NewFencePool(dev)

	_, err := pool.AcquireFence()
	if err == nil {
		t.Fatal("AcquireFence() expected error, got nil")
	}
	if err.Error() != "device lost" {
		t.Errorf("error = %q, want %q", err.Error(), "device lost")
	}
}

func TestFencePoolTrackSubmission(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence)

	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
	}
}

func TestFencePoolTrackMultipleSubmissions(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	for i := uint64(1); i <= 5; i++ {
		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(i, fence)
	}

	if pool.ActiveCount() != 5 {
		t.Errorf("ActiveCount = %d, want 5", pool.ActiveCount())
	}
}

func TestFencePoolTrackSubmissionWithCmdBufs(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	fence, _ := pool.AcquireFence()
	cmd1 := &mockCommandBuffer{id: 1}
	cmd2 := &mockCommandBuffer{id: 2}
	pool.TrackSubmission(1, fence, cmd1, cmd2)

	// Signal fence and poll -- should free command buffers
	fence.(*mockFence).signaled = true
	pool.PollCompleted()

	if !cmd1.freed || !cmd2.freed {
		t.Errorf("Command buffers not freed: cmd1.freed=%v, cmd2.freed=%v", cmd1.freed, cmd2.freed)
	}
	if len(dev.freeCmdBufIDs) != 2 {
		t.Errorf("freeCmdBufIDs count = %d, want 2", len(dev.freeCmdBufIDs))
	}
}

func TestFencePoolPollCompletedNoneSignaled(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence)

	completed := pool.PollCompleted()
	if completed != 0 {
		t.Errorf("PollCompleted = %d, want 0 (none signaled)", completed)
	}
	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
	}
}

func TestFencePoolPollCompletedSomeSignaled(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	f1, _ := pool.AcquireFence()
	f2, _ := pool.AcquireFence()
	f3, _ := pool.AcquireFence()
	pool.TrackSubmission(1, f1)
	pool.TrackSubmission(2, f2)
	pool.TrackSubmission(3, f3)

	// Signal only submissions 1 and 2
	f1.(*mockFence).signaled = true
	f2.(*mockFence).signaled = true

	completed := pool.PollCompleted()
	if completed != 2 {
		t.Errorf("PollCompleted = %d, want 2", completed)
	}
	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
	}
	if pool.LastCompleted() != 2 {
		t.Errorf("LastCompleted = %d, want 2", pool.LastCompleted())
	}
}

func TestFencePoolPollCompletedAllSignaled(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	f1, _ := pool.AcquireFence()
	f2, _ := pool.AcquireFence()
	pool.TrackSubmission(1, f1)
	pool.TrackSubmission(2, f2)

	f1.(*mockFence).signaled = true
	f2.(*mockFence).signaled = true

	completed := pool.PollCompleted()
	if completed != 2 {
		t.Errorf("PollCompleted = %d, want 2", completed)
	}
	if pool.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0", pool.ActiveCount())
	}
}

func TestFencePoolPollCompletedStatusError(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence)

	// Make GetFenceStatus return an error -- fence should remain active
	dev.statusErr = errors.New("status error")

	completed := pool.PollCompleted()
	if completed != 0 {
		t.Errorf("PollCompleted = %d, want 0 (error keeps fence active)", completed)
	}
	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
	}
}

func TestFencePoolPollCompletedUpdatesLastCompleted(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	f1, _ := pool.AcquireFence()
	pool.TrackSubmission(5, f1)
	f1.(*mockFence).signaled = true

	pool.PollCompleted()

	f2, _ := pool.AcquireFence()
	pool.TrackSubmission(10, f2)
	f2.(*mockFence).signaled = true

	completed := pool.PollCompleted()
	if completed != 10 {
		t.Errorf("PollCompleted = %d, want 10", completed)
	}
}

func TestFencePoolLastCompleted(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	if pool.LastCompleted() != 0 {
		t.Errorf("LastCompleted = %d, want 0 (initial)", pool.LastCompleted())
	}

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(42, fence)
	fence.(*mockFence).signaled = true
	pool.PollCompleted()

	if pool.LastCompleted() != 42 {
		t.Errorf("LastCompleted = %d, want 42", pool.LastCompleted())
	}
}

func TestFencePoolWaitAll(t *testing.T) {
	dev := &mockFenceDevice{waitResult: true}
	pool := NewFencePool(dev)

	f1, _ := pool.AcquireFence()
	f2, _ := pool.AcquireFence()
	pool.TrackSubmission(1, f1)
	pool.TrackSubmission(2, f2)

	// Signal both fences so PollCompleted (called inside WaitAll) moves them to free
	f1.(*mockFence).signaled = true
	f2.(*mockFence).signaled = true

	pool.WaitAll(time.Second)

	if pool.ActiveCount() != 0 {
		t.Errorf("ActiveCount after WaitAll = %d, want 0", pool.ActiveCount())
	}
}

func TestFencePoolWaitAllEmpty(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	// WaitAll on empty pool should not panic
	pool.WaitAll(time.Second)

	if pool.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0", pool.ActiveCount())
	}
}

func TestFencePoolDestroy(t *testing.T) {
	dev := &mockFenceDevice{waitResult: true}
	pool := NewFencePool(dev)

	// Create fences in free pool and active pool
	f1, _ := pool.AcquireFence()
	f2, _ := pool.AcquireFence()
	f3, _ := pool.AcquireFence()
	cmd := &mockCommandBuffer{id: 1}
	pool.TrackSubmission(1, f1, cmd)
	pool.TrackSubmission(2, f2)

	// Signal f1, f2 so WaitAll + PollCompleted moves them to free
	f1.(*mockFence).signaled = true
	f2.(*mockFence).signaled = true

	// f3 is not tracked (it's just acquired, not submitted)
	// Put f3 into the free pool manually for testing
	pool.mu.Lock()
	pool.free = append(pool.free, f3)
	pool.mu.Unlock()

	pool.Destroy()

	// After Destroy, all slices should be nil
	if pool.active != nil {
		t.Error("active should be nil after Destroy")
	}
	if pool.free != nil {
		t.Error("free should be nil after Destroy")
	}
}

func TestFencePoolDestroyReleasesResources(t *testing.T) {
	dev := &mockFenceDevice{waitResult: true}
	pool := NewFencePool(dev)

	f1, _ := pool.AcquireFence()
	cmd1 := &mockCommandBuffer{id: 10}
	cmd2 := &mockCommandBuffer{id: 11}
	pool.TrackSubmission(1, f1, cmd1, cmd2)

	// Signal so WaitAll+PollCompleted frees them
	f1.(*mockFence).signaled = true

	pool.Destroy()

	// Verify command buffers were freed
	if !cmd1.freed {
		t.Error("cmd1 should be freed after Destroy")
	}
	if !cmd2.freed {
		t.Error("cmd2 should be freed after Destroy")
	}
}

func TestFencePoolConcurrentAccess(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

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

func TestFencePoolPollCompletedRecyclesFences(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	// Acquire and track a fence
	f1, _ := pool.AcquireFence()
	pool.TrackSubmission(1, f1)
	f1.(*mockFence).signaled = true

	pool.PollCompleted()

	// The fence should now be in the free pool
	pool.mu.Lock()
	freeCount := len(pool.free)
	pool.mu.Unlock()

	if freeCount != 1 {
		t.Errorf("free pool size = %d, want 1", freeCount)
	}
}

func TestFencePoolPollCompletedOutOfOrderSignaling(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

	f1, _ := pool.AcquireFence()
	f2, _ := pool.AcquireFence()
	f3, _ := pool.AcquireFence()
	pool.TrackSubmission(1, f1)
	pool.TrackSubmission(2, f2)
	pool.TrackSubmission(3, f3)

	// Signal out of order: 3 first, then 1
	f3.(*mockFence).signaled = true
	f1.(*mockFence).signaled = true

	completed := pool.PollCompleted()
	// maxCompleted should be 3 (the highest signaled index)
	if completed != 3 {
		t.Errorf("PollCompleted = %d, want 3 (highest signaled)", completed)
	}
	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1 (f2 still active)", pool.ActiveCount())
	}
}

func TestFencePoolActiveCountThreadSafe(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := NewFencePool(dev)

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

// Verify the mockFenceDevice satisfies the hal.Device interface at compile time.
var _ hal.Device = (*mockFenceDevice)(nil)
