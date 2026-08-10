//go:build !rust && !(js && wasm) && !android

package gogpu

import (
	"fmt"
	"testing"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

type finiteFrameOverlay struct {
	draws int
}

func (*finiteFrameOverlay) Name() string { return "finite-frame" }

func (o *finiteFrameOverlay) Draw(gpucontext.DebugOverlayContext) bool {
	o.draws++
	return o.draws < 2
}

type namedIdleOverlay string

func (o namedIdleOverlay) Name() string { return string(o) }
func (namedIdleOverlay) Draw(gpucontext.DebugOverlayContext) bool {
	return false
}

func isolatedDebugOverlays(overlays ...gpucontext.DebugOverlay) []gpucontext.DebugOverlay {
	return append(overlays,
		namedIdleOverlay(overlayNameDamage),
		namedIdleOverlay(overlayNameFPS),
	)
}

type persistentFrameOverlay struct {
	draws      int
	beforeDone func()
}

func (*persistentFrameOverlay) Name() string { return "persistent-frame" }
func (o *persistentFrameOverlay) Draw(gpucontext.DebugOverlayContext) bool {
	o.draws++
	if o.beforeDone != nil {
		o.beforeDone()
	}
	return true
}

func newHeadlessRenderTarget(t *testing.T, configure bool) (*Renderer, *RenderTarget) {
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
	t.Cleanup(func() {
		device.Release()
		surface.Release()
		adapter.Release()
		instance.Release()
	})

	const size = 64
	renderer := &Renderer{device: device, adapter: adapter}
	ws := &RenderTarget{
		renderer: renderer,
		surface:  surface,
		state:    SurfaceConfigured,
		width:    size,
		height:   size,
		format:   gputypes.TextureFormatBGRA8Unorm,
		vsync:    true,
	}
	renderer.primary = ws
	if configure {
		if err := ws.configure(device, adapter); err != nil {
			t.Fatalf("Configure: %v", err)
		}
	}
	return renderer, ws
}

func TestRenderFrameGPU_OverlayAnimationContinuesWithoutContentDamage(t *testing.T) {
	renderer, ws := newHeadlessRenderTarget(t, true)
	overlay := &finiteFrameOverlay{}
	ws.debugOverlays = isolatedDebugOverlays(overlay)
	app := &App{
		renderLoop: &mockRenderLoop{},
		renderer:   renderer,
	}

	frame := windowFrame{window: &Window{surface: ws}, scale: 1}
	frame.onDraw = func(ctx *Context) { ctx.MarkPreserveContent() }
	app.renderFrameGPU([]windowFrame{frame})
	if overlay.draws != 1 {
		t.Fatalf("overlay draws after content frame = %d, want 1", overlay.draws)
	}
	if !app.pendingRedraw.Swap(false) {
		t.Fatal("animating overlay did not request its next frame")
	}

	// The UI has no new damage on the requested frame. The overlay must still
	// acquire, render, and present an overlay-only frame to finish its animation.
	frame.onDraw = func(*Context) {}
	app.renderFrameGPU([]windowFrame{frame})

	if overlay.draws != 2 {
		t.Errorf("overlay draws after idle frame = %d, want 2", overlay.draws)
	}
	if ws.frameNumber != 2 {
		t.Errorf("presented frames = %d, want 2", ws.frameNumber)
	}
	if ws.overlayNeedsRedraw {
		t.Error("finished overlay remained pending")
	}
	if app.pendingRedraw.Load() {
		t.Error("finished overlay requested another frame")
	}
}

func TestRenderFrameGPU_FailedOverlayAcquireWaitsForExternalInvalidation(t *testing.T) {
	renderer, ws := newHeadlessRenderTarget(t, false)
	overlay := &finiteFrameOverlay{}
	ws.debugOverlays = isolatedDebugOverlays(overlay)
	ws.overlayNeedsRedraw = true
	app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
	frame := windowFrame{
		window: &Window{surface: ws},
		onDraw: func(*Context) {},
		scale:  1,
	}

	// A stale configured state exercises a real failed GetCurrentTexture call.
	// Recovery configures the surface, but the overlay-only frame must not
	// self-schedule indefinitely if acquisition remains unavailable.
	app.renderFrameGPU([]windowFrame{frame})
	if app.pendingRedraw.Load() {
		t.Fatal("failed overlay-only acquire self-scheduled another frame")
	}
	if !ws.overlayNeedsRedraw {
		t.Fatal("failed overlay-only acquire discarded pending work")
	}
	if overlay.draws != 0 {
		t.Fatalf("overlay draws after failed acquire = %d, want 0", overlay.draws)
	}

	// A later external invalidation retries the preserved work. The recovery
	// from the first attempt made this acquire succeed.
	app.renderFrameGPU([]windowFrame{frame})
	if overlay.draws != 1 {
		t.Fatalf("overlay draws after external retry = %d, want 1", overlay.draws)
	}
	if !app.pendingRedraw.Swap(false) {
		t.Fatal("animating overlay did not request its next frame after recovery")
	}

	app.renderFrameGPU([]windowFrame{frame})
	if overlay.draws != 2 {
		t.Errorf("overlay draws after recovery animation = %d, want 2", overlay.draws)
	}
	if app.pendingRedraw.Load() {
		t.Error("finished overlay requested another frame")
	}
}

func TestRenderFrameGPU_FailedOverlayPresentDoesNotSelfSchedule(t *testing.T) {
	renderer, ws := newHeadlessRenderTarget(t, true)
	// Unconfiguring after the overlay records its commands makes the subsequent
	// PresentWithDamage fail while still exercising a successful frame acquire.
	overlay := &persistentFrameOverlay{beforeDone: ws.surface.Unconfigure}
	ws.debugOverlays = isolatedDebugOverlays(overlay)
	ws.overlayNeedsRedraw = true
	app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
	frame := windowFrame{
		window: &Window{surface: ws},
		onDraw: func(*Context) {},
		scale:  1,
	}

	for range 3 {
		if err := ws.configure(renderer.device, renderer.adapter); err != nil {
			t.Fatalf("reconfigure before external retry: %v", err)
		}
		app.renderFrameGPU([]windowFrame{frame})
		if app.pendingRedraw.Swap(false) {
			t.Error("failed overlay-only present scheduled an unbounded retry")
		}
		if !ws.overlayNeedsRedraw {
			t.Error("failed overlay-only present discarded pending work")
		}
	}
	if overlay.draws != 3 {
		t.Errorf("overlay draws after external retries = %d, want 3", overlay.draws)
	}
}

func TestRetryReconfiguredFrameReplaysAtLiveScale(t *testing.T) {
	renderer, ws := newHeadlessRenderTarget(t, true)
	ws.platWindow = &mockWindow{width: 64, height: 64, scaleFactor: 2}
	app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
	draws := 0
	frame := windowFrame{
		window: &Window{surface: ws},
		onDraw: func(ctx *Context) {
			draws++
			if ctx.ScaleFactor() != 2 {
				t.Errorf("retry scale = %v, want live scale 2", ctx.ScaleFactor())
			}
			ctx.MarkPreserveContent()
		},
		scale: 1,
	}

	result, overlayDeferred := app.retryReconfiguredFrame(frame, ws, false, false)

	if !result.completed || result.reconfigured {
		t.Errorf("retry result = %+v, want completed without another reconfigure", result)
	}
	if overlayDeferred {
		t.Fatal("GPU retry unexpectedly deferred an overlay")
	}
	if draws != 1 {
		t.Errorf("retry draw callbacks = %d, want 1", draws)
	}
}

func TestRetryReconfiguredFrameWithoutGPUWorkDefers(t *testing.T) {
	renderer, ws := newHeadlessRenderTarget(t, true)
	ws.platWindow = &mockWindow{width: 64, height: 64, scaleFactor: 1}
	app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
	frame := windowFrame{
		window: &Window{surface: ws},
		onDraw: func(*Context) {},
		scale:  1,
	}

	result, overlayDeferred := app.retryReconfiguredFrame(frame, ws, false, false)

	if !result.reconfigured || result.completed {
		t.Errorf("retry result = %+v, want reconfigured and incomplete", result)
	}
	if overlayDeferred {
		t.Fatal("retry without pixel presentation deferred an overlay")
	}
}

func TestRenderFrameGPU_UnavailablePendingOverlayDoesNotSelfSchedule(t *testing.T) {
	tests := []struct {
		name string
		ws   *RenderTarget
	}{
		{name: "unconfigured", ws: &RenderTarget{state: SurfaceReady}},
		{
			name: "minimized",
			ws: &RenderTarget{
				state:      SurfaceReady,
				platWindow: &mockWindow{width: 0, height: 0},
			},
		},
		{name: "lost", ws: &RenderTarget{state: SurfaceLost}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlay := &finiteFrameOverlay{}
			renderer := &Renderer{}
			tt.ws.renderer = renderer
			tt.ws.debugOverlays = isolatedDebugOverlays(overlay)
			tt.ws.overlayNeedsRedraw = true
			renderer.primary = tt.ws
			app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
			frame := windowFrame{
				window: &Window{surface: tt.ws},
				onDraw: func(*Context) {},
				scale:  1,
			}

			for range 3 {
				app.renderFrameGPU([]windowFrame{frame})
				if app.pendingRedraw.Swap(false) {
					t.Error("unavailable overlay-only frame scheduled an unbounded retry")
				}
			}
			if !tt.ws.overlayNeedsRedraw {
				t.Error("pending overlay work was discarded instead of deferred")
			}
			if overlay.draws != 0 {
				t.Errorf("overlay draws = %d without a surface, want 0", overlay.draws)
			}
		})
	}
}

func TestRenderFrameGPU_PixelPresentedFrameDefersPendingOverlay(t *testing.T) {
	for _, frameStarted := range []bool{false, true} {
		t.Run(fmt.Sprintf("frame_started_%t", frameStarted), func(t *testing.T) {
			renderer, ws := newHeadlessRenderTarget(t, true)
			overlay := &finiteFrameOverlay{}
			ws.debugOverlays = isolatedDebugOverlays(overlay)
			ws.overlayNeedsRedraw = true
			app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
			frame := windowFrame{
				window: &Window{surface: ws},
				onDraw: func(*Context) {
					// WriteSurfacePixels sets this after bypassing the GPU frame path.
					ws.pixelPresented = true
					ws.frameStarted = frameStarted
				},
				scale: 1,
			}

			for range 3 {
				app.renderFrameGPU([]windowFrame{frame})
				if app.pendingRedraw.Swap(false) {
					t.Error("pixel presentation scheduled an overlay frame it cannot composite")
				}
			}
			if !ws.overlayNeedsRedraw {
				t.Error("pixel presentation discarded pending overlay work")
			}
			if overlay.draws != 0 {
				t.Errorf("overlay draws on pixel-present path = %d, want 0", overlay.draws)
			}
		})
	}
}

func TestRenderFrameGPU_UnavailableOverlayDoesNotSuppressOtherSurfaceRetry(t *testing.T) {
	renderer := &Renderer{}
	overlaySurface := &RenderTarget{
		renderer:           renderer,
		state:              SurfaceReady,
		debugOverlays:      isolatedDebugOverlays(new(finiteFrameOverlay)),
		overlayNeedsRedraw: true,
	}
	contentSurface := &RenderTarget{renderer: renderer, state: SurfaceReady}
	app := &App{renderLoop: &mockRenderLoop{}, renderer: renderer}
	frames := []windowFrame{
		{
			window: &Window{surface: overlaySurface},
			onDraw: func(*Context) {},
			scale:  1,
		},
		{
			window: &Window{surface: contentSurface},
			onDraw: func(ctx *Context) { ctx.MarkPreserveContent() },
			scale:  1,
		},
	}

	app.renderFrameGPU(frames)

	if !app.pendingRedraw.Load() {
		t.Error("content acquire failure on another surface did not retain its retry")
	}
	if !overlaySurface.overlayNeedsRedraw {
		t.Error("unavailable surface discarded its pending overlay")
	}
}
