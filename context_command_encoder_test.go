package gogpu

import (
	"errors"
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/noop"
)

type countingSubmitQueue struct {
	noop.Queue
	submits   int
	submitErr error
}

type commandEncoderFailDevice struct{ noop.Device }

func (d *commandEncoderFailDevice) CreateCommandEncoder(
	_ *hal.CommandEncoderDescriptor,
) (hal.CommandEncoder, error) {
	return nil, errors.New("injected command encoder failure")
}

func (q *countingSubmitQueue) Submit(_ []hal.CommandBuffer) (uint64, error) {
	q.submits++
	if q.submitErr != nil {
		return 0, q.submitErr
	}
	return uint64(q.submits), nil
}

func newSharedEncoderTestContext(t *testing.T) (*Context, *RenderTarget, *countingSubmitQueue) {
	t.Helper()
	return newSharedEncoderTestContextWithDevice(t, &noop.Device{})
}

func newSharedEncoderTestContextWithDevice(
	t *testing.T,
	halDevice hal.Device,
) (*Context, *RenderTarget, *countingSubmitQueue) {
	t.Helper()
	queue := &countingSubmitQueue{}
	device, err := wgpu.NewDeviceFromHAL(
		halDevice, queue, 0, gputypes.DefaultLimits(), "shared-encoder-test",
	)
	if err != nil {
		t.Fatalf("NewDeviceFromHAL: %v", err)
	}
	renderer := &Renderer{device: device}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "shared-encoder-test-target",
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatBGRA8Unorm,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		device.Release()
		t.Fatalf("CreateTexture: %v", err)
	}
	view, err := device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		device.Release()
		t.Fatalf("CreateTextureView: %v", err)
	}
	target := &RenderTarget{
		renderer:     renderer,
		frameStarted: true,
		currentView:  view,
	}
	renderer.primary = target
	t.Cleanup(func() {
		target.discardFrameEncoder()
		if target.currentView != nil {
			target.currentView.Release()
		}
		texture.Release()
		device.Release()
	})
	return newContext(renderer, 1), target, queue
}

func TestContextCommandEncoderReusesAndSubmitsFrameEncoderOnce(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)

	first := ctx.CommandEncoder()
	if first == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	if second := ctx.CommandEncoder(); second != first {
		t.Fatal("CommandEncoder did not reuse the active frame encoder")
	}
	if queue.submits != 0 {
		t.Fatalf("encoder submitted before frame end: %d", queue.submits)
	}

	if !target.submitFrameEncoder(target.renderer) {
		t.Fatal("frame encoder submission failed")
	}
	if queue.submits != 1 {
		t.Fatalf("frame submissions = %d, want 1", queue.submits)
	}
	if target.frameEncoder != nil {
		t.Fatal("frame encoder retained after submission")
	}
}

func TestContextRenderTargetCommandEncoderForwardsActiveEncoder(t *testing.T) {
	ctx, _, _ := newSharedEncoderTestContext(t)

	encoder := ctx.CommandEncoder()
	opaque := ctx.RenderTarget().CommandEncoder()
	if opaque.IsNil() {
		t.Fatal("opaque CommandEncoder is nil")
	}
	if got := (*wgpu.CommandEncoder)(opaque.Pointer()); got != encoder {
		t.Fatal("RenderTarget returned a different command encoder")
	}
}

func TestContextCommandEncoderFlushesPendingClearFirst(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)
	target.clear(0.1, 0.2, 0.3, 1)

	if encoder := ctx.CommandEncoder(); encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	if target.hasPendingClear {
		t.Fatal("pending clear was not recorded before returning the shared encoder")
	}
	if !target.frameCleared {
		t.Fatal("frame was not marked cleared")
	}
	if queue.submits != 0 {
		t.Fatalf("clear submitted before frame end: %d", queue.submits)
	}
}

func TestContextCommandEncoderWithoutSurfaceReturnsNil(t *testing.T) {
	ctx := newContext(&Renderer{}, 1)
	if got := ctx.CommandEncoder(); got != nil {
		t.Fatalf("CommandEncoder() = %v, want nil", got)
	}
}

func TestContextRenderTargetCommandEncoderWithoutSurfaceReturnsNil(t *testing.T) {
	ctx := newContext(&Renderer{}, 1)
	if got := ctx.RenderTarget().CommandEncoder(); !got.IsNil() {
		t.Fatal("opaque CommandEncoder is non-nil without an active surface")
	}
}

func TestEnsureFrameEncoderWithoutFrameFails(t *testing.T) {
	target := &RenderTarget{renderer: &Renderer{}}
	if _, err := target.ensureFrameEncoder(); err == nil {
		t.Fatal("ensureFrameEncoder succeeded without an active frame")
	}
}

func TestContextCommandEncoderReturnsNilWhenCreationFails(t *testing.T) {
	ctx, _, _ := newSharedEncoderTestContextWithDevice(t, &commandEncoderFailDevice{})
	if got := ctx.CommandEncoder(); got != nil {
		t.Fatalf("CommandEncoder() = %v after creation failure, want nil", got)
	}
}

func TestFlushClearStandaloneSubmitsOnce(t *testing.T) {
	_, target, queue := newSharedEncoderTestContext(t)
	target.clear(0.1, 0.2, 0.3, 1)

	if !target.flushClear(target.renderer.device, target.renderer) {
		t.Fatal("flushClear failed")
	}
	if queue.submits != 1 {
		t.Fatalf("standalone clear submissions = %d, want 1", queue.submits)
	}
	if target.hasPendingClear || !target.frameCleared {
		t.Fatal("standalone clear did not update frame state")
	}
}

func TestFlushClearDiscardsStandaloneEncoderOnInvalidView(t *testing.T) {
	_, target, queue := newSharedEncoderTestContext(t)
	target.currentView.Release()
	target.clear(0.1, 0.2, 0.3, 1)

	if target.flushClear(target.renderer.device, target.renderer) {
		t.Fatal("flushClear succeeded with a released view")
	}
	if queue.submits != 0 {
		t.Fatalf("invalid clear submissions = %d, want 0", queue.submits)
	}
}

func TestContextCommandEncoderDiscardsSharedEncoderWhenClearFails(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)
	if encoder := ctx.CommandEncoder(); encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	target.currentView.Release()
	target.clear(0.1, 0.2, 0.3, 1)

	if encoder := ctx.CommandEncoder(); encoder != nil {
		t.Fatal("CommandEncoder returned encoder after clear failure")
	}
	if target.frameEncoder != nil {
		t.Fatal("failed shared encoder was retained")
	}
	if queue.submits != 0 {
		t.Fatalf("failed shared encoder submissions = %d, want 0", queue.submits)
	}
}

func TestSubmitFrameEncoderDropsFinishFailure(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)
	encoder := ctx.CommandEncoder()
	if encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	encoder.CopyBufferToBuffer(nil, 0, nil, 0, 1)

	if target.submitFrameEncoder(target.renderer) {
		t.Fatal("invalid frame encoder was reported submitted")
	}
	if target.frameEncoder != nil {
		t.Fatal("failed frame encoder was retained")
	}
	if queue.submits != 0 {
		t.Fatalf("invalid frame submissions = %d, want 0", queue.submits)
	}
}

func TestSubmitFrameEncoderReportsQueueFailure(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)
	if encoder := ctx.CommandEncoder(); encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	queue.submitErr = errors.New("injected submit failure")

	if target.submitFrameEncoder(target.renderer) {
		t.Fatal("failed queue submission was reported successful")
	}
	if target.frameEncoder != nil {
		t.Fatal("failed frame encoder was retained")
	}
	if queue.submits != 1 {
		t.Fatalf("submission attempts = %d, want 1", queue.submits)
	}
}

func TestEndFrameDiscardsEncoderAfterPixelPresent(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)
	if encoder := ctx.CommandEncoder(); encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	target.pixelPresented = true

	result := target.renderer.endFrameForSurface(target)
	if result.reconfigured {
		t.Fatal("pixel-presented frame unexpectedly reconfigured")
	}
	if !result.completed {
		t.Fatal("pixel-presented frame was not reported complete")
	}
	if target.frameEncoder != nil {
		t.Fatal("pixel-presented frame retained its encoder")
	}
	if queue.submits != 0 {
		t.Fatalf("pixel-presented frame submissions = %d, want 0", queue.submits)
	}
}
