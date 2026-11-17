package gogpu

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/wgputest"
)

type testContextHelper interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
}

// passLoadOps returns the load operations of every color attachment of every
// recorded render pass, in call order.
func passLoadOps(rec *wgputest.Recorder) []gputypes.LoadOp {
	var ops []gputypes.LoadOp
	for _, pass := range rec.Trace().Passes {
		if pass.Compute {
			continue
		}
		ops = append(ops, pass.LoadOps...)
	}
	return ops
}

// bindGroupOffsets returns the dynamic offsets of every recorded bind-group
// set at index 0, in call order.
func bindGroupOffsets(rec *wgputest.Recorder) [][]uint32 {
	var offsets [][]uint32
	for _, bind := range rec.Trace().BindGroups {
		if bind.Index != 0 {
			continue
		}
		offsets = append(offsets, bind.DynamicOffsets)
	}
	return offsets
}

func newSharedEncoderTestContext(t testContextHelper) (*Context, *RenderTarget, *wgputest.Recorder) {
	t.Helper()
	return newSharedEncoderTestContextWithOptions(t)
}

// newSharedEncoderTestContextWithOptions creates a context over a noop device
// whose public calls are recorded, so tests can assert on encoder, submit and
// pass counts and on the pass and bind-group data the context produced.
func newSharedEncoderTestContextWithOptions(
	t testContextHelper,
	opts ...wgputest.Option,
) (*Context, *RenderTarget, *wgputest.Recorder) {
	t.Helper()
	rec := wgputest.NewRecorder()
	opts = append(opts, wgputest.WithRecorder(rec))
	device, _, cleanup, err := wgputest.NewNoopDevice(opts...)
	if err != nil {
		t.Fatalf("wgputest.NewNoopDevice: %v", err)
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
		cleanup()
		t.Fatalf("CreateTexture: %v", err)
	}
	view, err := device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		cleanup()
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
		cleanup()
	})
	return newContext(renderer, 1), target, rec
}

func TestContextCommandEncoderReusesAndSubmitsFrameEncoderOnce(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContext(t)

	first := ctx.CommandEncoder()
	if first == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	if second := ctx.CommandEncoder(); second != first {
		t.Fatal("CommandEncoder did not reuse the active frame encoder")
	}
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("encoder submitted before frame end: %d", got)
	}

	target.submitFrameEncoder(target.renderer)
	if got := rec.Counts().Submits; got != 1 {
		t.Fatalf("frame submissions = %d, want 1", got)
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
	ctx, target, rec := newSharedEncoderTestContext(t)
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

	if got := rec.Counts().Encoders; got != 1 {
		t.Fatalf("frame command encoders created = %d, want 1", got)
	}
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("submissions before EndFrame = %d, want 0", got)
	}

	if got, want := passLoadOps(rec), []gputypes.LoadOp{gputypes.LoadOpClear}; !equalLoadOps(got, want) {
		t.Fatalf("render-pass load ops = %v, want %v", got, want)
	}
	if got, want := bindGroupOffsets(rec), [][]uint32{{0}, {256}, {512}}; !equalOffsets(got, want) {
		t.Fatalf("uniform dynamic offsets = %v, want %v", got, want)
	}

	target.submitFrameEncoder(target.renderer)
	if got := rec.Counts().Submits; got != 1 {
		t.Fatalf("submissions after EndFrame = %d, want 1", got)
	}
}

// One chunk holds texQuadUniformChunkSlots uniforms. The next quad must open a
// second chunk with its own buffer and bind group, start again at offset 0,
// and have its deferred write target the new buffer, all in the same pass and
// the same frame encoder.
func TestTexturedQuadUniformsSpillIntoSecondChunk(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContext(t)
	target.renderer.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
	target.format = gputypes.TextureFormatBGRA8Unorm
	target.width = 1
	target.height = 1

	tex, err := target.renderer.NewTextureFromRGBA(1, 1, []byte{255, 0, 0, 255})
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	t.Cleanup(tex.Destroy)

	n := texQuadUniformChunkSlots + 1
	for i := 0; i < n; i++ {
		if err := ctx.DrawTextureEx(tex, DrawTextureOptions{Width: 1, Height: 1, Alpha: 1}); err != nil {
			t.Fatalf("DrawTextureEx(%d): %v", i, err)
		}
	}

	chunks := target.texQuadUniformChunks
	if len(chunks) != 2 {
		t.Fatalf("uniform chunks = %d, want 2", len(chunks))
	}
	if chunks[0].nextSlot != texQuadUniformChunkSlots || chunks[1].nextSlot != 1 {
		t.Fatalf("chunk slots used = %d, %d; want %d, 1", chunks[0].nextSlot, chunks[1].nextSlot, texQuadUniformChunkSlots)
	}
	if chunks[0].buffer == chunks[1].buffer || chunks[0].bindGrp == chunks[1].bindGrp {
		t.Fatal("second chunk reuses the first chunk's buffer or bind group")
	}

	if got := rec.Counts().Encoders; got != 1 {
		t.Fatalf("frame command encoders created = %d, want 1", got)
	}
	offsets := bindGroupOffsets(rec)
	if len(offsets) != n {
		t.Fatalf("draws recorded = %d, want %d", len(offsets), n)
	}
	lastOfFirst := []uint32{uint32(texQuadUniformChunkSlots-1) * chunks[0].stride}
	if got := offsets[n-2]; !equalOffsets([][]uint32{got}, [][]uint32{lastOfFirst}) {
		t.Fatalf("offset of the last slot in chunk 0 = %v, want %v", got, lastOfFirst)
	}
	if got := offsets[n-1]; !equalOffsets([][]uint32{got}, [][]uint32{{0}}) {
		t.Fatalf("offset of the first slot in chunk 1 = %v, want [0]", got)
	}

	writes := target.pendingTexQuadUniformWrites
	if len(writes) != n {
		t.Fatalf("deferred uniform writes = %d, want %d", len(writes), n)
	}
	if writes[n-2].buffer != chunks[0].buffer || writes[n-1].buffer != chunks[1].buffer {
		t.Fatal("deferred writes do not target the chunk their slot came from")
	}

	target.submitFrameEncoder(target.renderer)
	if got := rec.Counts().Submits; got != 1 {
		t.Fatalf("submissions after EndFrame = %d, want 1", got)
	}
}

func TestTexturedQuadPassPreservesExternalAndClearBoundaries(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContext(t)
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

	if got := rec.Counts().Encoders; got != 1 {
		t.Fatalf("frame command encoders created = %d, want 1", got)
	}
	if got, want := passLoadOps(rec), []gputypes.LoadOp{
		gputypes.LoadOpClear,
		gputypes.LoadOpLoad,
		gputypes.LoadOpLoad,
		gputypes.LoadOpClear,
	}; !equalLoadOps(got, want) {
		t.Fatalf("render-pass load ops = %v, want %v", got, want)
	}
	if got, want := bindGroupOffsets(rec), [][]uint32{{0}, {256}, {512}}; !equalOffsets(got, want) {
		t.Fatalf("uniform dynamic offsets = %v, want %v", got, want)
	}
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("submissions before EndFrame = %d, want 0", got)
	}
	target.submitFrameEncoder(target.renderer)
	if got := rec.Counts().Submits; got != 1 {
		t.Fatalf("submissions after EndFrame = %d, want 1", got)
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
			ctx, target, rec := newSharedEncoderTestContext(b)
			target.renderer.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
			target.format = gputypes.TextureFormatBGRA8Unorm
			target.width = 1
			target.height = 1

			tex, err := target.renderer.NewTextureFromRGBA(1, 1, []byte{255, 0, 0, 255})
			if err != nil {
				b.Fatalf("NewTextureFromRGBA: %v", err)
			}
			b.Cleanup(tex.Destroy)

			// Only the frame loop's own work counts: the recorder is reset
			// after setup, so the counts below are the delta the loop created.
			rec.Reset()
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
				target.submitFrameEncoder(target.renderer)
				target.renderer.pollSubmissions()
				target.releaseTexQuadUniformChunks()
			}
			b.StopTimer()

			counts := rec.Counts()
			if counts.Encoders != b.N {
				b.Fatalf("frame command encoders created = %d, want %d (one per frame)", counts.Encoders, b.N)
			}
			if counts.Passes != b.N {
				b.Fatalf("render passes = %d, want %d (one per frame)", counts.Passes, b.N)
			}
			if counts.Submits != b.N {
				b.Fatalf("frame submissions = %d, want %d", counts.Submits, b.N)
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
	ctx, target, rec := newSharedEncoderTestContext(t)
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
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("clear submitted before frame end: %d", got)
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
	ctx, _, _ := newSharedEncoderTestContextWithOptions(t, wgputest.WithFaults(wgputest.Faults{
		CreateCommandEncoder: errors.New("injected command encoder failure"),
	}))
	if got := ctx.CommandEncoder(); got != nil {
		t.Fatalf("CommandEncoder() = %v after creation failure, want nil", got)
	}
}

func TestContextCommandEncoderReturnsNilWhenTexturedQuadPassEndFails(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContextWithOptions(t, wgputest.WithFaults(wgputest.Faults{
		WriteBuffer: errors.New("injected uniform write failure"),
	}))
	target.renderer.surfaceFormat = gputypes.TextureFormatBGRA8Unorm
	target.format = gputypes.TextureFormatBGRA8Unorm
	target.width = 1
	target.height = 1

	tex, err := target.renderer.NewTextureFromRGBA(1, 1, []byte{255, 0, 0, 255})
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	t.Cleanup(tex.Destroy)

	if err := ctx.DrawTextureEx(tex, DrawTextureOptions{Width: 1, Height: 1, Alpha: 1}); err != nil {
		t.Fatalf("DrawTextureEx: %v", err)
	}
	if target.texturedQuadPass == nil {
		t.Fatal("DrawTextureEx did not leave an active textured-quad pass")
	}

	if got := ctx.CommandEncoder(); got != nil {
		t.Fatal("CommandEncoder returned an encoder after textured-quad pass failure")
	}
	if target.texturedQuadPass != nil {
		t.Fatal("failed textured-quad pass was retained")
	}
	if target.frameEncoder != nil {
		t.Fatal("failed shared encoder was retained")
	}
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("failed shared encoder submissions = %d, want 0", got)
	}
}

func TestFlushClearStandaloneSubmitsOnce(t *testing.T) {
	_, target, rec := newSharedEncoderTestContext(t)
	target.clear(0.1, 0.2, 0.3, 1)

	if !target.flushClear(target.renderer.device, target.renderer) {
		t.Fatal("flushClear failed")
	}
	if got := rec.Counts().Submits; got != 1 {
		t.Fatalf("standalone clear submissions = %d, want 1", got)
	}
	if target.hasPendingClear || !target.frameCleared {
		t.Fatal("standalone clear did not update frame state")
	}
}

func TestFlushClearDiscardsStandaloneEncoderOnInvalidView(t *testing.T) {
	_, target, rec := newSharedEncoderTestContext(t)
	target.currentView.Release()
	target.clear(0.1, 0.2, 0.3, 1)

	if target.flushClear(target.renderer.device, target.renderer) {
		t.Fatal("flushClear succeeded with a released view")
	}
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("invalid clear submissions = %d, want 0", got)
	}
}

func TestContextCommandEncoderDiscardsSharedEncoderWhenClearFails(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContext(t)
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
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("failed shared encoder submissions = %d, want 0", got)
	}
}

func TestSubmitFrameEncoderDropsFinishFailure(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContext(t)
	encoder := ctx.CommandEncoder()
	if encoder == nil {
		t.Fatal("CommandEncoder returned nil")
	}
	encoder.CopyBufferToBuffer(nil, 0, nil, 0, 1)

	target.submitFrameEncoder(target.renderer)
	if target.frameEncoder != nil {
		t.Fatal("failed frame encoder was retained")
	}
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("invalid frame submissions = %d, want 0", got)
	}
}

func TestEndFrameDiscardsEncoderAfterPixelPresent(t *testing.T) {
	ctx, target, rec := newSharedEncoderTestContext(t)
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
	if got := rec.Counts().Submits; got != 0 {
		t.Fatalf("pixel-presented frame submissions = %d, want 0", got)
	}
}
