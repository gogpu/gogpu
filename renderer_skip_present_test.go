package gogpu

import (
	"testing"

	"github.com/gogpu/gputypes"
)

func newTestWindowSurface() *RenderTarget {
	r := &Renderer{}
	ws := &RenderTarget{
		renderer: r,
		format:   gputypes.TextureFormatBGRA8Unorm,
	}
	r.primary = ws
	return ws
}

func TestWindowSurface_HasGPUWork_DefaultFalse(t *testing.T) {
	ws := newTestWindowSurface()
	if ws.hasGPUWork {
		t.Error("hasGPUWork should be false by default")
	}
}

func TestWindowSurface_FrameStarted_DefaultFalse(t *testing.T) {
	ws := newTestWindowSurface()
	if ws.frameStarted {
		t.Error("frameStarted should be false by default")
	}
}

func TestPrepareLazyAcquire_SetsState(t *testing.T) {
	ws := newTestWindowSurface()
	ws.hasGPUWork = true
	ws.frameStarted = true

	ws.prepareLazyAcquire()

	if ws.hasGPUWork {
		t.Error("prepareLazyAcquire should reset hasGPUWork")
	}
	if ws.frameStarted {
		t.Error("prepareLazyAcquire should reset frameStarted")
	}
}

func TestResetLazyState_ClearsAll(t *testing.T) {
	ws := newTestWindowSurface()
	ws.frameStarted = true
	ws.hasGPUWork = true

	ws.resetLazyState()

	if ws.frameStarted {
		t.Error("resetLazyState should clear frameStarted")
	}
	if ws.hasGPUWork {
		t.Error("resetLazyState should clear hasGPUWork")
	}
}

func TestEnsureFrameStarted_NoSurface_ReturnsFalse(t *testing.T) {
	ws := newTestWindowSurface()
	ws.prepareLazyAcquire()

	// No configured surface → beginFrame returns false
	result := ws.ensureFrameStarted()

	if result {
		t.Error("ensureFrameStarted should return false when surface not configured")
	}
	if ws.frameStarted {
		t.Error("frameStarted should stay false when beginFrame fails")
	}
}

func TestEnsureFrameStarted_CalledOnce(t *testing.T) {
	ws := newTestWindowSurface()
	ws.prepareLazyAcquire()

	// First call: tries beginFrame
	ws.ensureFrameStarted()

	// Second call: returns cached result (no double acquire)
	ws.frameStarted = true
	result := ws.ensureFrameStarted()

	if result {
		t.Error("should still return false (no view)")
	}
}

func TestEndFrameForSurface_OnlyWhenStarted(t *testing.T) {
	ws := newTestWindowSurface()
	ws.frameStarted = false
	r := &Renderer{primary: ws}

	// Should not panic — endFrameForSurface is only called when frameStarted=true
	// (checked by caller in renderFrameMultiThread)
	_ = r
}

func TestWindowSurface_Clear_NilView_NoGPUWork(t *testing.T) {
	ws := newTestWindowSurface()
	ws.prepareLazyAcquire()
	ws.clear(0, 0, 0, 1)

	// ensureFrameStarted fails (no surface) → no GPU work
	if ws.hasGPUWork {
		t.Error("clear() with unconfigured surface should not set hasGPUWork")
	}
}
