package gogpu

import (
	"errors"
	"fmt"
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

type commandEncoderFailDevice struct{ noop.Device }

func (d *commandEncoderFailDevice) CreateCommandEncoder(
	_ *hal.CommandEncoderDescriptor,
) (hal.CommandEncoder, error) {
	return nil, errors.New("injected command encoder failure")
}

func (q *countingSubmitQueue) Submit(_ []hal.CommandBuffer) (uint64, error) {
	q.submits++
	return uint64(q.submits), nil
}

// recordingCommandEncoderDevice counts frame encoder creation and records the
// load operation for each render pass. It wraps the noop backend so the test
// can prove both the submission boundary and clear/load ordering without a
// real display or GPU.
type recordingCommandEncoderDevice struct {
	noop.Device
	encoders []*recordingCommandEncoder
}

type recordingCommandEncoder struct {
	*noop.CommandEncoder
	loadOps        []gputypes.LoadOp
	uniformOffsets [][]uint32
}

type recordingRenderPassEncoder struct {
	hal.RenderPassEncoder
	owner *recordingCommandEncoder
}

func (d *recordingCommandEncoderDevice) CreateCommandEncoder(
	_ *hal.CommandEncoderDescriptor,
) (hal.CommandEncoder, error) {
	encoder := &recordingCommandEncoder{CommandEncoder: &noop.CommandEncoder{}}
	d.encoders = append(d.encoders, encoder)
	return encoder, nil
}

func (e *recordingCommandEncoder) BeginRenderPass(desc *hal.RenderPassDescriptor) hal.RenderPassEncoder {
	if len(desc.ColorAttachments) > 0 {
		e.loadOps = append(e.loadOps, desc.ColorAttachments[0].LoadOp)
	}
	return &recordingRenderPassEncoder{
		RenderPassEncoder: e.CommandEncoder.BeginRenderPass(desc),
		owner:             e,
	}
}

func (p *recordingRenderPassEncoder) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	if index == 0 {
		p.owner.uniformOffsets = append(p.owner.uniformOffsets, append([]uint32(nil), offsets...))
	}
	p.RenderPassEncoder.SetBindGroup(index, group, offsets)
}

type testContextHelper interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
}

func newSharedEncoderTestContext(t testContextHelper) (*Context, *RenderTarget, *countingSubmitQueue) {
	t.Helper()
	return newSharedEncoderTestContextWithDevice(t, &noop.Device{})
}

func newSharedEncoderTestContextWithDevice(
	t testContextHelper,
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
		renderer.Destroy()
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

func TestBeginFrameReleasesPreviousTexturedQuadUniformArena(t *testing.T) {
	ws := &RenderTarget{
		texQuadUniformChunks: []texQuadUniformChunk{{nextSlot: 7}},
	}
	r := &Renderer{primary: ws}

	if r.BeginFrame() {
		t.Fatal("BeginFrame succeeded for an unconfigured surface")
	}
	if got := len(ws.texQuadUniformChunks); got != 0 {
		t.Fatalf("textured-quad uniform chunks after new frame = %d, want 0", got)
	}
}

func TestDrawTexturedQuadsReuseFrameEncoderAndPreserveLoadOrder(t *testing.T) {
	halDevice := &recordingCommandEncoderDevice{}
	ctx, target, queue := newSharedEncoderTestContextWithDevice(t, halDevice)
	target.renderer.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
	target.format = gputypes.TextureFormatBGRA8Unorm
	target.width = 1
	target.height = 1

	tex, err := target.renderer.NewTextureFromRGBA(1, 1, []byte{255, 0, 0, 255})
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	t.Cleanup(tex.Destroy)

	// The first quad consumes the pending clear. Every subsequent quad must
	// load the prior result while remaining in the same frame submission.
	target.clear(0.1, 0.2, 0.3, 1)
	for i := 0; i < 3; i++ {
		if err := ctx.DrawTextureEx(tex, DrawTextureOptions{
			X:      float32(i),
			Y:      0,
			Width:  1,
			Height: 1,
			Alpha:  1,
		}); err != nil {
			t.Fatalf("DrawTextureEx(%d): %v", i, err)
		}
	}

	if got := len(halDevice.encoders); got != 1 {
		t.Fatalf("frame command encoders created = %d, want 1", got)
	}
	if queue.submits != 0 {
		t.Fatalf("submissions before EndFrame = %d, want 0", queue.submits)
	}

	if got, want := halDevice.encoders[0].loadOps, []gputypes.LoadOp{gputypes.LoadOpClear}; !equalLoadOps(got, want) {
		t.Fatalf("render-pass load ops = %v, want %v", got, want)
	}
	if got, want := halDevice.encoders[0].uniformOffsets, [][]uint32{{0}, {256}, {512}}; !equalOffsets(got, want) {
		t.Fatalf("uniform dynamic offsets = %v, want %v", got, want)
	}

	target.submitFrameEncoder(target.renderer)
	if queue.submits != 1 {
		t.Fatalf("submissions after EndFrame = %d, want 1", queue.submits)
	}
}

func TestTexturedQuadPassPreservesExternalAndClearBoundaries(t *testing.T) {
	halDevice := &recordingCommandEncoderDevice{}
	ctx, target, queue := newSharedEncoderTestContextWithDevice(t, halDevice)
	target.renderer.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
	target.format = gputypes.TextureFormatBGRA8Unorm
	target.width = 1
	target.height = 1

	tex, err := target.renderer.NewTextureFromRGBA(1, 1, []byte{255, 0, 0, 255})
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	t.Cleanup(tex.Destroy)

	target.clear(0.1, 0.2, 0.3, 1)
	if err := ctx.DrawTextureEx(tex, DrawTextureOptions{Width: 1, Height: 1, Alpha: 1}); err != nil {
		t.Fatalf("first DrawTextureEx: %v", err)
	}

	// Lending the frame encoder to an external renderer must close the current
	// quad pass, then the next quad resumes with LoadOpLoad in call order.
	encoder := ctx.CommandEncoder()
	if encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	externalPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    target.renderView(),
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
	})
	if err != nil {
		t.Fatalf("external BeginRenderPass: %v", err)
	}
	if err := externalPass.End(); err != nil {
		t.Fatalf("external End: %v", err)
	}
	if err := ctx.DrawTextureEx(tex, DrawTextureOptions{X: 1, Width: 1, Height: 1, Alpha: 1}); err != nil {
		t.Fatalf("second DrawTextureEx: %v", err)
	}

	// An explicit clear is another pass boundary. It must not be folded into
	// the already-open pass or reorder the clear relative to the next draw.
	target.clear(0.4, 0.5, 0.6, 1)
	if err := ctx.DrawTextureEx(tex, DrawTextureOptions{X: 2, Width: 1, Height: 1, Alpha: 1}); err != nil {
		t.Fatalf("third DrawTextureEx: %v", err)
	}

	if got := len(halDevice.encoders); got != 1 {
		t.Fatalf("frame command encoders created = %d, want 1", got)
	}
	if got, want := halDevice.encoders[0].loadOps, []gputypes.LoadOp{
		gputypes.LoadOpClear,
		gputypes.LoadOpLoad,
		gputypes.LoadOpLoad,
		gputypes.LoadOpClear,
	}; !equalLoadOps(got, want) {
		t.Fatalf("render-pass load ops = %v, want %v", got, want)
	}
	if got, want := halDevice.encoders[0].uniformOffsets, [][]uint32{{0}, {256}, {512}}; !equalOffsets(got, want) {
		t.Fatalf("uniform dynamic offsets = %v, want %v", got, want)
	}
	if queue.submits != 0 {
		t.Fatalf("submissions before EndFrame = %d, want 0", queue.submits)
	}
	target.submitFrameEncoder(target.renderer)
	if queue.submits != 1 {
		t.Fatalf("submissions after EndFrame = %d, want 1", queue.submits)
	}
}

func equalLoadOps(got, want []gputypes.LoadOp) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func equalOffsets(got, want [][]uint32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			return false
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				return false
			}
		}
	}
	return true
}

func BenchmarkDrawTexturedQuadsSharedFrameEncoder(b *testing.B) {
	for _, draws := range []int{1, 20, 100} {
		b.Run(fmt.Sprintf("draws=%d", draws), func(b *testing.B) {
			halDevice := &recordingCommandEncoderDevice{}
			ctx, target, queue := newSharedEncoderTestContextWithDevice(b, halDevice)
			target.renderer.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
			target.format = gputypes.TextureFormatBGRA8Unorm
			target.width = 1
			target.height = 1

			tex, err := target.renderer.NewTextureFromRGBA(1, 1, []byte{255, 0, 0, 255})
			if err != nil {
				b.Fatalf("NewTextureFromRGBA: %v", err)
			}
			b.Cleanup(tex.Destroy)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target.frameCleared = false
				for j := 0; j < draws; j++ {
					if err := ctx.DrawTextureEx(tex, DrawTextureOptions{
						X:      float32(j),
						Width:  1,
						Height: 1,
						Alpha:  1,
					}); err != nil {
						b.Fatalf("DrawTextureEx(%d): %v", j, err)
					}
				}
				if got := len(halDevice.encoders[i].loadOps); got != 1 {
					b.Fatalf("render passes for %d draws = %d, want 1", draws, got)
				}
				target.submitFrameEncoder(target.renderer)
				target.renderer.pollSubmissions()
				target.releaseTexQuadUniformChunks()
			}
			b.StopTimer()

			if got := len(halDevice.encoders); got != b.N {
				b.Fatalf("frame command encoders created = %d, want %d", got, b.N)
			}
			if got := queue.submits; got != b.N {
				b.Fatalf("frame submissions = %d, want %d", got, b.N)
			}
			b.ReportMetric(float64(draws), "draws/frame")
			b.ReportMetric(1, "passes/frame")
			b.ReportMetric(1, "encoders/frame")
			b.ReportMetric(1, "submits/frame")
		})
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

	target.submitFrameEncoder(target.renderer)
	if target.frameEncoder != nil {
		t.Fatal("failed frame encoder was retained")
	}
	if queue.submits != 0 {
		t.Fatalf("invalid frame submissions = %d, want 0", queue.submits)
	}
}

func TestEndFrameDiscardsEncoderAfterPixelPresent(t *testing.T) {
	ctx, target, queue := newSharedEncoderTestContext(t)
	if encoder := ctx.CommandEncoder(); encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	target.pixelPresented = true

	if target.renderer.endFrameForSurface(target) {
		t.Fatal("pixel-presented frame unexpectedly reconfigured")
	}
	if target.frameEncoder != nil {
		t.Fatal("pixel-presented frame retained its encoder")
	}
	if queue.submits != 0 {
		t.Fatalf("pixel-presented frame submissions = %d, want 0", queue.submits)
	}
}
