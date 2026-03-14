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

func TestFencePoolAcquireFenceCreatesNew(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	fence, err := pool.AcquireFence()
	if err != nil {
		t.Fatalf("AcquireFence() error = %v", err)
	}
	if fence == nil {
		t.Fatal("AcquireFence() returned nil fence")
	}
	// Note: mockFenceDevice.CreateFence is called by both NewDeviceFromHAL (for Queue fence)
	// and by AcquireFence. The queue fence is the first one, AcquireFence creates the second.
	if dev.fenceCounter < 2 {
		t.Errorf("fenceCounter = %d, want >= 2 (queue fence + acquired fence)", dev.fenceCounter)
	}
}

func TestFencePoolTrackSubmission(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	fence, _ := pool.AcquireFence()
	pool.TrackSubmission(1, fence)

	if pool.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", pool.ActiveCount())
	}
}

func TestFencePoolTrackMultipleSubmissions(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	for i := uint64(1); i <= 5; i++ {
		fence, _ := pool.AcquireFence()
		pool.TrackSubmission(i, fence)
	}

	if pool.ActiveCount() != 5 {
		t.Errorf("ActiveCount = %d, want 5", pool.ActiveCount())
	}
}

func TestFencePoolPollCompletedNoneSignaled(t *testing.T) {
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
}

func TestFencePoolPollCompletedStatusError(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

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

func TestFencePoolWaitAllEmpty(t *testing.T) {
	dev := &mockFenceDevice{}
	pool := newTestFencePool(t, dev)

	// WaitAll on empty pool should not panic
	pool.WaitAll(time.Second)

	if pool.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d, want 0", pool.ActiveCount())
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
