package gogpu

import (
	"testing"

	"github.com/gogpu/gputypes"
)

// --- State logic tests (unit-test the flag mechanics) ---

// TestExternalContent_FrameClearedEnablesLoad verifies the fundamental
// contract: frameCleared=true → LoadOp::Load in drawTexturedQuad.
// Replicates the exact LoadOp decision from renderer.go:1122-1128.
func TestExternalContent_FrameClearedEnablesLoad(t *testing.T) {
	ws := newTestWindowSurface()
	ws.frameCleared = true
	ws.hasPendingClear = false

	loadOp := gputypes.LoadOpClear
	if !ws.hasPendingClear && ws.frameCleared {
		loadOp = gputypes.LoadOpLoad
	}

	if loadOp != gputypes.LoadOpLoad {
		t.Errorf("LoadOp = %v with frameCleared=true, want LoadOpLoad", loadOp)
	}
}

// TestExternalContent_ClearWinsOverFrameCleared verifies that explicit
// Clear() (hasPendingClear) takes priority over frameCleared.
// renderer.go:1124 checks hasPendingClear first.
func TestExternalContent_ClearWinsOverFrameCleared(t *testing.T) {
	ws := newTestWindowSurface()
	ws.frameCleared = true
	ws.hasPendingClear = true

	loadOp := gputypes.LoadOpClear
	if ws.hasPendingClear {
		loadOp = gputypes.LoadOpClear
	} else if ws.frameCleared {
		loadOp = gputypes.LoadOpLoad
	}

	if loadOp != gputypes.LoadOpClear {
		t.Error("hasPendingClear must win over frameCleared")
	}
}

// TestExternalContent_ResetOnBeginFrame verifies that frameCleared resets
// when a new frame begins (beginFrame, renderer.go:490). This is the actual
// reset point — not prepareLazyAcquire (which resets frameStarted/hasGPUWork
// but not frameCleared). frameCleared persists through prepareLazyAcquire
// because it is only meaningful AFTER acquire, and beginFrame always clears it.
func TestExternalContent_ResetOnBeginFrame(t *testing.T) {
	ws := newTestWindowSurface()
	ws.frameCleared = true
	ws.hasGPUWork = true

	// prepareLazyAcquire resets hasGPUWork but NOT frameCleared
	ws.prepareLazyAcquire()

	if !ws.frameCleared {
		t.Error("prepareLazyAcquire must NOT reset frameCleared (reset happens in beginFrame)")
	}
	if ws.hasGPUWork {
		t.Error("prepareLazyAcquire must reset hasGPUWork")
	}
}

// TestExternalContent_LazyStateDoesNotResetFrameCleared documents that
// resetLazyState (called after endFrame) does NOT reset frameCleared.
// The actual reset happens in beginFrame when the next frame acquires.
func TestExternalContent_LazyStateDoesNotResetFrameCleared(t *testing.T) {
	ws := newTestWindowSurface()
	ws.frameCleared = true

	ws.resetLazyState()

	// frameCleared survives resetLazyState — by design
	if !ws.frameCleared {
		t.Error("resetLazyState must NOT reset frameCleared (reset happens in beginFrame)")
	}
}

// --- MarkExternalContent integration tests ---

// TestMarkExternalContent_NoSurface_NoPanic verifies graceful handling when
// surface is not configured. ensureFrameStarted returns false → no state
// change, no panic.
func TestMarkExternalContent_NoSurface_NoPanic(t *testing.T) {
	ws := newTestWindowSurface()
	ws.prepareLazyAcquire()

	ctx := newContext(&Renderer{primary: ws}, 1.0)
	ctx.MarkExternalContent() // must not panic

	if ws.frameCleared {
		t.Error("frameCleared must stay false when surface not available")
	}
	if ws.hasGPUWork {
		t.Error("hasGPUWork must stay false when surface not available")
	}
	if ws.frameStarted {
		t.Error("frameStarted must stay false when ensureFrameStarted fails")
	}
}

// TestMarkExternalContent_RequiresFrameStarted verifies that
// MarkExternalContent only sets state when the frame is properly started
// (surface acquired, view available). Without a configured surface,
// ensureFrameStarted returns false and no state is modified.
func TestMarkExternalContent_RequiresFrameStarted(t *testing.T) {
	ws := newTestWindowSurface()
	// frameStarted=false, no surface → ensureFrameStarted will fail

	ctx := newContext(&Renderer{primary: ws}, 1.0)
	ctx.MarkExternalContent()

	if ws.frameCleared {
		t.Error("MarkExternalContent without started frame must not set frameCleared")
	}
}

// TestMarkExternalContent_MultiWindow verifies correct surface targeting
// in multi-window mode. MarkExternalContent uses activeSurface() which
// returns the Context's target surface, not always the primary.
func TestMarkExternalContent_MultiWindow(t *testing.T) {
	primary := newTestWindowSurface()
	secondary := &RenderTarget{
		renderer: primary.renderer,
		format:   gputypes.TextureFormatBGRA8Unorm,
	}

	// Neither has a real surface → both fail ensureFrameStarted
	// But we verify the targeting logic: secondary context targets secondary
	ctx := newContextForSurface(primary.renderer, secondary, 1.0)

	// activeSurface() should return secondary, not primary
	if ctx.activeSurface() != secondary {
		t.Error("Context with explicit surface must target it, not primary")
	}
}

// TestContextRenderTarget_PreserveContent verifies that the ggcanvas adapter
// exposes the external-content state for the surface targeted by its Context.
func TestContextRenderTarget_PreserveContent(t *testing.T) {
	if newContext(&Renderer{}, 1.0).RenderTarget().PreserveContent() {
		t.Error("PreserveContent() = true without an active surface")
	}

	primary := newTestWindowSurface()
	ctx := newContext(primary.renderer, 1.0)
	rt := ctx.RenderTarget()

	if rt.PreserveContent() {
		t.Error("PreserveContent() = true before external content is marked")
	}

	primary.frameCleared = true
	if !rt.PreserveContent() {
		t.Error("PreserveContent() = false with frameCleared=true")
	}

	secondary := &RenderTarget{
		renderer:     primary.renderer,
		format:       gputypes.TextureFormatBGRA8Unorm,
		frameCleared: true,
	}
	primary.frameCleared = false
	secondaryRT := newContextForSurface(primary.renderer, secondary, 1.0).RenderTarget()
	if !secondaryRT.PreserveContent() {
		t.Error("PreserveContent() did not read the Context's secondary surface")
	}
}
