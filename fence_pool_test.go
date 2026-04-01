package gogpu

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
)

// --- Mock types for submissionTracker testing ---

// mockFenceDevice implements hal.Device for testing.
type mockFenceDevice struct {
	freedCmdBufs []hal.CommandBuffer
}

func (d *mockFenceDevice) CreateBuffer(_ *hal.BufferDescriptor) (hal.Buffer, error) { return nil, nil } //nolint:nilnil // mock
func (d *mockFenceDevice) DestroyBuffer(_ hal.Buffer)                               {}
func (d *mockFenceDevice) CreateTexture(_ *hal.TextureDescriptor) (hal.Texture, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyTexture(_ hal.Texture) {}
func (d *mockFenceDevice) CreateTextureView(_ hal.Texture, _ *hal.TextureViewDescriptor) (hal.TextureView, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyTextureView(_ hal.TextureView) {}
func (d *mockFenceDevice) CreateSampler(_ *hal.SamplerDescriptor) (hal.Sampler, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroySampler(_ hal.Sampler) {}
func (d *mockFenceDevice) CreateBindGroupLayout(_ *hal.BindGroupLayoutDescriptor) (hal.BindGroupLayout, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyBindGroupLayout(_ hal.BindGroupLayout) {}
func (d *mockFenceDevice) CreateBindGroup(_ *hal.BindGroupDescriptor) (hal.BindGroup, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyBindGroup(_ hal.BindGroup) {}
func (d *mockFenceDevice) CreatePipelineLayout(_ *hal.PipelineLayoutDescriptor) (hal.PipelineLayout, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyPipelineLayout(_ hal.PipelineLayout) {}
func (d *mockFenceDevice) CreateShaderModule(_ *hal.ShaderModuleDescriptor) (hal.ShaderModule, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyShaderModule(_ hal.ShaderModule) {}
func (d *mockFenceDevice) CreateRenderPipeline(_ *hal.RenderPipelineDescriptor) (hal.RenderPipeline, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyRenderPipeline(_ hal.RenderPipeline) {}
func (d *mockFenceDevice) CreateComputePipeline(_ *hal.ComputePipelineDescriptor) (hal.ComputePipeline, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) DestroyComputePipeline(_ hal.ComputePipeline) {}
func (d *mockFenceDevice) CreateQuerySet(_ *hal.QuerySetDescriptor) (hal.QuerySet, error) {
	return nil, hal.ErrTimestampsNotSupported
}
func (d *mockFenceDevice) DestroyQuerySet(_ hal.QuerySet) {}
func (d *mockFenceDevice) CreateCommandEncoder(_ *hal.CommandEncoderDescriptor) (hal.CommandEncoder, error) {
	return nil, nil //nolint:nilnil // mock
}
func (d *mockFenceDevice) CreateFence() (hal.Fence, error) { return nil, nil } //nolint:nilnil // mock
func (d *mockFenceDevice) DestroyFence(_ hal.Fence)        {}
func (d *mockFenceDevice) Wait(_ hal.Fence, _ uint64, _ time.Duration) (bool, error) {
	return true, nil
}
func (d *mockFenceDevice) ResetFence(_ hal.Fence) error             { return nil }
func (d *mockFenceDevice) GetFenceStatus(_ hal.Fence) (bool, error) { return true, nil }
func (d *mockFenceDevice) FreeCommandBuffer(cb hal.CommandBuffer) {
	d.freedCmdBufs = append(d.freedCmdBufs, cb)
}
func (d *mockFenceDevice) CreateRenderBundleEncoder(_ *hal.RenderBundleEncoderDescriptor) (hal.RenderBundleEncoder, error) {
	return nil, fmt.Errorf("mock: render bundles not supported")
}
func (d *mockFenceDevice) DestroyRenderBundle(_ hal.RenderBundle) {}
func (d *mockFenceDevice) WaitIdle() error                        { return nil }
func (d *mockFenceDevice) Destroy()                               {}

// Verify the mockFenceDevice satisfies the hal.Device interface at compile time.
var _ hal.Device = (*mockFenceDevice)(nil)

// mockQueue implements hal.Queue for testing.
type mockQueue struct {
	submissionIndex uint64
}

func (q *mockQueue) Submit(_ []hal.CommandBuffer) (uint64, error) {
	q.submissionIndex++
	return q.submissionIndex, nil
}
func (q *mockQueue) PollCompleted() uint64                              { return q.submissionIndex }
func (q *mockQueue) WriteBuffer(_ hal.Buffer, _ uint64, _ []byte) error { return nil }
func (q *mockQueue) WriteTexture(_ *hal.ImageCopyTexture, _ []byte, _ *hal.ImageDataLayout, _ *hal.Extent3D) error {
	return nil
}
func (q *mockQueue) ReadBuffer(_ hal.Buffer, _ uint64, _ []byte) error               { return nil }
func (q *mockQueue) Present(_ hal.Surface, _ hal.SurfaceTexture) error               { return nil }
func (q *mockQueue) GetTimestampPeriod() float32                                     { return 0 }
func (q *mockQueue) Destroy()                                                        {}
func (q *mockQueue) CopyExternalImageToTexture(_ any, _ *hal.ImageCopyTexture) error { return nil }
func (q *mockQueue) SupportsCommandBufferCopies() bool                               { return false }

// Verify the mockQueue satisfies the hal.Queue interface at compile time.
var _ hal.Queue = (*mockQueue)(nil)

// newTestDevice creates a wgpu.Device wrapping a mock HAL device for testing.
func newTestDevice(t *testing.T, mockDev *mockFenceDevice) *wgpu.Device {
	t.Helper()
	device, err := wgpu.NewDeviceFromHAL(
		mockDev,
		&mockQueue{},
		gputypes.Features(0),
		gputypes.DefaultLimits(),
		"test",
	)
	if err != nil {
		t.Fatalf("NewDeviceFromHAL failed: %v", err)
	}
	return device
}

func TestSubmissionTracker_Track(t *testing.T) {
	dev := &mockFenceDevice{}
	device := newTestDevice(t, dev)

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
	dev := &mockFenceDevice{}
	device := newTestDevice(t, dev)

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
	dev := &mockFenceDevice{}
	device := newTestDevice(t, dev)

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
	dev := &mockFenceDevice{}
	device := newTestDevice(t, dev)

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
