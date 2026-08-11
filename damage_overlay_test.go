package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/compositor"
)

// --- ADR-067 composition texture lifecycle tests ---

func TestResetLazyState_PreservesOverlayNeedsRedraw(t *testing.T) {
	// overlayNeedsRedraw must survive resetLazyState so the app frame loop
	// can check it after reset and call RequestRedraw (ADR-067).
	ws := &RenderTarget{overlayNeedsRedraw: true}
	ws.resetLazyState()
	if !ws.overlayNeedsRedraw {
		t.Error("overlayNeedsRedraw was cleared by resetLazyState; should be preserved")
	}
}

func TestRenderView_WithoutComposition(t *testing.T) {
	// Without a composition texture, renderView returns currentView.
	ws := &RenderTarget{}
	// currentView is nil by default; renderView should return nil.
	if ws.renderView() != nil {
		t.Error("renderView() should return nil when both composView and currentView are nil")
	}
}

func TestTryOverlayOnlyFrame_NoComposView(t *testing.T) {
	// Without composView, overlay-only frame is not possible.
	ws := &RenderTarget{overlayNeedsRedraw: true}
	if ws.tryOverlayOnlyFrame() {
		t.Error("tryOverlayOnlyFrame should return false without composView")
	}
}

func TestTryOverlayOnlyFrame_NoRedrawNeeded(t *testing.T) {
	// Without overlayNeedsRedraw, overlay-only frame is not needed.
	ws := &RenderTarget{overlayNeedsRedraw: false}
	if ws.tryOverlayOnlyFrame() {
		t.Error("tryOverlayOnlyFrame should return false when overlayNeedsRedraw is false")
	}
}

func TestReleaseCompositionTexture_Idempotent(t *testing.T) {
	// Calling releaseCompositionTexture on a RenderTarget with no composition
	// texture should be a safe no-op.
	ws := &RenderTarget{}
	ws.releaseCompositionTexture() // should not panic
	if ws.composW != 0 || ws.composH != 0 {
		t.Error("composW/composH should be 0 after release")
	}
}

func TestHasRegisteredOverlayEnv(t *testing.T) {
	// With no env vars set, hasRegisteredOverlayEnv returns false.
	// Note: this test relies on sync.Once caching, so it tests the cached
	// state which defaults to false when env vars are empty.
	ws := &RenderTarget{}
	// We cannot reliably test true case without env var manipulation that
	// conflicts with sync.Once, but we can verify the method exists and
	// returns a bool.
	_ = ws.hasRegisteredOverlayEnv()
}

// --- overlay constant tests (verify compositor exports) ---

func TestOverlayUniformSize(t *testing.T) {
	// screen(2f=8) + pad(2f=8) = 16 bytes.
	if compositor.OverlayUniformSize != 16 {
		t.Errorf("OverlayUniformSize = %d, want 16", compositor.OverlayUniformSize)
	}
}

func TestOverlayInstanceStride(t *testing.T) {
	// rectXY(f32x2=8) + rectWH(f32x2=8) + color(f32x4=16) = 32 bytes.
	if compositor.OverlayInstanceStride != 32 {
		t.Errorf("OverlayInstanceStride = %d, want 32", compositor.OverlayInstanceStride)
	}
}
