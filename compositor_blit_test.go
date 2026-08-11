package gogpu

import (
	"image"
	"testing"

	"github.com/gogpu/wgpu"
)

func TestCompositorBlitResources_ReleaseBinding(t *testing.T) {
	r := &CompositorBlitResources{
		BoundView: &wgpu.TextureView{}, // non-nil sentinel
	}
	// BindGroup is nil — ReleaseBinding should not panic.
	r.ReleaseBinding()
	if r.BoundView != nil {
		t.Error("BoundView should be nil after ReleaseBinding")
	}
}

func TestCompositorBlitResources_Destroy(t *testing.T) {
	r := &CompositorBlitResources{}
	// Calling Destroy on zero-value should not panic.
	r.Destroy()
}

func TestApplyOverlayScissorWithDamage_ViaScissorCompute(t *testing.T) {
	// ApplyOverlayScissorWithDamage requires a *wgpu.RenderPassEncoder
	// (not mockable without GPU). Verify the underlying damage scissor
	// computation that drives the overlay scissor logic.
	x, y, w, h, valid := computeDamageScissor(nil, 800, 600, image.Rect(0, 0, 800, 600))
	if !valid {
		t.Error("full-surface damage should be valid")
	}
	if x != 0 || y != 0 || w != 800 || h != 600 {
		t.Errorf("expected full surface scissor, got (%d, %d, %d, %d)", x, y, w, h)
	}
}

func TestCompositorShouldPreserve(t *testing.T) {
	tests := []struct {
		name         string
		frameCleared bool
		want         bool
	}{
		{"no previous content", false, false},
		{"has previous content", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &RenderTarget{
				frameCleared: tt.frameCleared,
			}
			if got := ws.CompositorShouldPreserve(); got != tt.want {
				t.Errorf("CompositorShouldPreserve() = %v, want %v", got, tt.want)
			}
		})
	}
}
