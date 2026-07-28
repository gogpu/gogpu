package gogpu

import (
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/noop"
)

type countingSubmitQueue struct {
	noop.Queue
	submits int
}

func (q *countingSubmitQueue) Submit(_ []hal.CommandBuffer) (uint64, error) {
	q.submits++
	return uint64(q.submits), nil
}

func newSharedEncoderTestContext(t *testing.T) (*Context, *RenderTarget, *countingSubmitQueue) {
	t.Helper()
	queue := &countingSubmitQueue{}
	device, err := wgpu.NewDeviceFromHAL(
		&noop.Device{}, queue, 0, gputypes.DefaultLimits(), "shared-encoder-test",
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
		view.Release()
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

	target.submitFrameEncoder(target.renderer)
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
