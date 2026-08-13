package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/software"
)

type countingFrameWindow struct {
	mockWindow
	regularSyncs int
}

func (w *countingFrameWindow) SyncFrame() { w.regularSyncs++ }

type countingPresentationWindow struct {
	countingFrameWindow
	prepareCalls []bool
	prepareOK    bool
	cancelCalls  int
}

func (w *countingPresentationWindow) PrepareFrameSync(force bool) bool {
	w.prepareCalls = append(w.prepareCalls, force)
	return w.prepareOK
}

func (w *countingPresentationWindow) CancelFrameSync() { w.cancelCalls++ }

type steadyStatePresentationWindow struct{ mockWindow }

func (w *steadyStatePresentationWindow) PrepareFrameSync(bool) bool { return false }
func (w *steadyStatePresentationWindow) CancelFrameSync()           {}

func newPixelPresentationTestContext(
	t *testing.T,
	window platform.PlatformWindow,
) (*Context, *RenderTarget) {
	t.Helper()
	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	surface, err := instance.CreateSurfaceFromTarget(wgpu.HeadlessSurfaceTarget{})
	if err != nil {
		instance.Release()
		t.Fatalf("CreateSurfaceFromTarget: %v", err)
	}
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface:    surface,
		ForceFallbackAdapter: true,
	})
	if err != nil {
		surface.Release()
		instance.Release()
		t.Fatalf("RequestAdapter: %v", err)
	}
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		adapter.Release()
		surface.Release()
		instance.Release()
		t.Fatalf("RequestDevice: %v", err)
	}
	if err := surface.Configure(device, &wgpu.SurfaceConfiguration{
		Width:       1,
		Height:      1,
		Format:      gputypes.TextureFormatRGBA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: gputypes.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}); err != nil {
		adapter.Release()
		surface.Release()
		device.Release()
		instance.Release()
		t.Fatalf("Configure: %v", err)
	}
	renderer := &Renderer{device: device}
	target := &RenderTarget{
		renderer:   renderer,
		platWindow: window,
		surface:    surface,
		width:      1,
		height:     1,
		state:      SurfaceConfigured,
	}
	renderer.primary = target
	t.Cleanup(func() {
		surface.Unconfigure()
		device.Release()
		surface.Release()
		adapter.Release()
		instance.Release()
	})
	return newContext(renderer, 1), target
}

func TestContextRequestPresentationSyncTargetsActiveSurface(t *testing.T) {
	primary := &RenderTarget{}
	secondary := &RenderTarget{}
	renderer := &Renderer{primary: primary}

	primaryContext := newContext(renderer, 1)
	primaryContext.RequestPresentationSync()
	primaryContext.RequestPresentationSync()
	if !primary.presentationSyncRequested {
		t.Fatal("primary surface did not retain presentation sync request")
	}
	if secondary.presentationSyncRequested {
		t.Fatal("secondary surface unexpectedly retained presentation sync request")
	}

	newContextForSurface(renderer, secondary, 1).RequestPresentationSync()
	if !secondary.presentationSyncRequested {
		t.Fatal("target surface did not retain presentation sync request")
	}
}

func TestPresentationSyncRequestSurvivesEmptyFrame(t *testing.T) {
	target := newTestWindowSurface()
	newContext(target.renderer, 1).RequestPresentationSync()

	target.prepareLazyAcquire()
	target.resetLazyState()

	if !target.presentationSyncRequested {
		t.Fatal("empty frame consumed the presentation sync request")
	}
}

func TestPresentationSyncUsesOneShotPlatformPath(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	target := &RenderTarget{
		platWindow:                window,
		presentationSyncRequested: true,
	}

	attempt := syncFrameForPresent(target)

	if len(window.prepareCalls) != 1 || !window.prepareCalls[0] || window.regularSyncs != 0 {
		t.Fatalf("sync calls = regular %d, prepare %v; want 0, [true]", window.regularSyncs, window.prepareCalls)
	}
	if target.presentationSyncRequested {
		t.Fatal("consumed presentation sync request remained pending")
	}
	if !attempt.consumedRequest || !attempt.cancelable {
		t.Fatalf("sync attempt = %+v, want consumed and cancelable", attempt)
	}
}

func TestPresentationSyncUsesNormalFramePolicyWithoutRequest(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	target := &RenderTarget{platWindow: window}

	attempt := syncFrameForPresent(target)

	if len(window.prepareCalls) != 1 || window.prepareCalls[0] || window.regularSyncs != 0 {
		t.Fatalf("sync calls = regular %d, prepare %v; want 0, [false]", window.regularSyncs, window.prepareCalls)
	}
	if attempt.consumedRequest || !attempt.cancelable {
		t.Fatalf("sync attempt = %+v, want normal cancelable preparation", attempt)
	}
}

func TestPresentationSyncFallsBackToNormalPlatformSync(t *testing.T) {
	window := &countingFrameWindow{}
	target := &RenderTarget{
		platWindow:                window,
		presentationSyncRequested: true,
	}

	attempt := syncFrameForPresent(target)

	if window.regularSyncs != 1 {
		t.Fatalf("regular sync calls = %d, want 1", window.regularSyncs)
	}
	if target.presentationSyncRequested {
		t.Fatal("fallback presentation sync request remained pending")
	}
	if !attempt.consumedRequest || attempt.cancelable {
		t.Fatalf("sync attempt = %+v, want consumed fallback", attempt)
	}
}

func TestPresentationSyncRetainsRequestWhenPreparationFails(t *testing.T) {
	window := &countingPresentationWindow{}
	target := &RenderTarget{
		platWindow:                window,
		presentationSyncRequested: true,
	}

	attempt := syncFrameForPresent(target)

	if len(window.prepareCalls) != 1 || !window.prepareCalls[0] {
		t.Fatalf("prepare calls = %v, want [true]", window.prepareCalls)
	}
	if !target.presentationSyncRequested {
		t.Fatal("failed preparation consumed the presentation sync request")
	}
	if attempt != (presentationSyncAttempt{}) {
		t.Fatalf("sync attempt = %+v after failed preparation, want zero", attempt)
	}
}

func TestPresentationSyncPresentationFailureCancelsAndRestoresRequest(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	target := &RenderTarget{
		platWindow:                window,
		presentationSyncRequested: true,
	}

	attempt := syncFrameForPresent(target)
	finishPresentationSync(target, attempt, false)

	if window.cancelCalls != 1 {
		t.Fatalf("cancel calls = %d, want 1", window.cancelCalls)
	}
	if !target.presentationSyncRequested {
		t.Fatal("failed presentation did not restore the one-shot request")
	}
}

func TestNormalPresentationFailureCancelsPreparedFramePolicy(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	target := &RenderTarget{platWindow: window}

	attempt := syncFrameForPresent(target)
	finishPresentationSync(target, attempt, false)

	if window.cancelCalls != 1 {
		t.Fatalf("cancel calls = %d, want 1", window.cancelCalls)
	}
	if target.presentationSyncRequested {
		t.Fatal("normal frame policy created a one-shot request")
	}
}

func TestPresentationSyncSuccessfulPresentationConsumesRequest(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	target := &RenderTarget{
		platWindow:                window,
		presentationSyncRequested: true,
	}

	attempt := syncFrameForPresent(target)
	finishPresentationSync(target, attempt, true)

	if window.cancelCalls != 0 {
		t.Fatalf("cancel calls = %d, want 0", window.cancelCalls)
	}
	if target.presentationSyncRequested {
		t.Fatal("successful presentation restored a consumed request")
	}
}

func TestWriteSurfacePixelsUsesPrePresentSync(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	ctx, target := newPixelPresentationTestContext(t, window)
	damage := compositor.NewDamageSource("test", 0)
	damage.ReportDamage()
	target.damageSources = []*compositor.DamageSource{damage}
	texture, err := target.renderer.device.CreateTexture(&wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	t.Cleanup(texture.Release)
	target.currentView, err = target.renderer.device.CreateTextureView(texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView: %v", err)
	}
	ctx.RequestPresentationSync()

	if err := ctx.RenderTarget().WriteSurfacePixels([]byte{1, 2, 3, 4}, 1, 1); err != nil {
		t.Fatalf("WriteSurfacePixels: %v", err)
	}
	if len(window.prepareCalls) != 1 || !window.prepareCalls[0] {
		t.Fatalf("prepare calls = %v, want [true]", window.prepareCalls)
	}
	if target.presentationSyncRequested {
		t.Fatal("successful pixel presentation retained the request")
	}
	if !target.pixelPresented {
		t.Fatal("successful direct presentation did not update frame lifecycle")
	}
	if target.currentView != nil {
		t.Fatal("successful direct presentation retained the stale surface view")
	}
	if damage.Full || len(damage.Rects) != 0 {
		t.Fatal("successful direct presentation retained reported damage")
	}
}

func TestWriteSurfacePixelsRequiresActiveSurface(t *testing.T) {
	ctx := newContext(&Renderer{}, 1)
	if err := ctx.RenderTarget().WriteSurfacePixels(nil, 1, 1); err == nil {
		t.Fatal("WriteSurfacePixels succeeded without an active surface")
	}
}

func TestWriteSurfacePixelsFailureCancelsAndRestoresSync(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	ctx, target := newPixelPresentationTestContext(t, window)
	ctx.RequestPresentationSync()

	if err := ctx.RenderTarget().WriteSurfacePixels(nil, 1, 1); err == nil {
		t.Fatal("WriteSurfacePixels succeeded with an undersized pixel buffer")
	}
	if window.cancelCalls != 1 {
		t.Fatalf("cancel calls = %d, want 1", window.cancelCalls)
	}
	if !target.presentationSyncRequested {
		t.Fatal("failed pixel presentation did not restore the sync request")
	}
	if target.pixelPresented {
		t.Fatal("failed direct presentation updated frame lifecycle")
	}
}

func TestWriteSurfacePixelsUsesPostPresentFallback(t *testing.T) {
	window := &countingFrameWindow{}
	ctx, target := newPixelPresentationTestContext(t, window)
	ctx.RequestPresentationSync()

	if err := ctx.RenderTarget().WriteSurfacePixels([]byte{1, 2, 3, 4}, 1, 1); err != nil {
		t.Fatalf("WriteSurfacePixels: %v", err)
	}
	if window.regularSyncs != 1 {
		t.Fatalf("regular sync calls = %d, want 1", window.regularSyncs)
	}
	if target.presentationSyncRequested {
		t.Fatal("successful fallback pixel presentation retained the request")
	}
}

func TestPresentationSyncMarksSecondaryFrameGate(t *testing.T) {
	window := &countingPresentationWindow{prepareOK: true}
	renderer := &Renderer{}
	primary := &RenderTarget{renderer: renderer}
	secondary := &RenderTarget{renderer: renderer, platWindow: window}
	renderer.primary = primary

	syncFrameForPresent(secondary)

	if !renderer.secondaryFrameGatePending.Load() {
		t.Fatal("secondary compositor callback did not mark the application gate")
	}
}

func BenchmarkPresentationSyncSteadyState(b *testing.B) {
	window := &steadyStatePresentationWindow{}
	target := &RenderTarget{platWindow: window}
	b.ReportAllocs()

	for b.Loop() {
		syncFrameForPresent(target)
	}
}
