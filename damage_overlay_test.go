package gogpu

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/gogpu/gpucontext"
)

// --- parseDamageDebugMode tests ---

func TestParseDamageDebugMode(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantOvl bool
		wantLog bool
	}{
		{"empty", "", false, false},
		{"1 (legacy, not supported)", "1", false, false},
		{"overlay only", "overlay", true, false},
		{"log only", "log", false, true},
		{"overlay,log", "overlay,log", true, true},
		{"log,overlay", "log,overlay", true, true},
		{"overlay with spaces", "overlay , log", true, true},
		{"unknown value", "unknown", false, false},
		{"partial match", "overlay,unknown", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOGPU_DEBUG_DAMAGE", tt.envVal)
			// parseDamageDebugMode reads os.Getenv directly.
			mode := parseDamageDebugMode()
			if mode.overlay != tt.wantOvl {
				t.Errorf("overlay = %v, want %v", mode.overlay, tt.wantOvl)
			}
			if mode.log != tt.wantLog {
				t.Errorf("log = %v, want %v", mode.log, tt.wantLog)
			}
		})
	}
}

// --- fadeAlpha tests ---

func TestFadeAlpha(t *testing.T) {
	overlay := &damageDebugOverlay{}

	tests := []struct {
		name    string
		elapsed time.Duration
		wantMin float32
		wantMax float32
	}{
		{"at start", 0, 0.99, 1.01},
		{"half duration", damageFlashDuration / 2, 0.45, 0.55},
		{"at expiry", damageFlashDuration, -0.01, 0.01},
		{"past expiry", damageFlashDuration + time.Second, -0.01, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			flash := &damageFlash{time: now.Add(-tt.elapsed)}
			alpha := overlay.fadeAlpha(flash, now)
			if alpha < tt.wantMin || alpha > tt.wantMax {
				t.Errorf("fadeAlpha(elapsed=%v) = %f, want [%f, %f]",
					tt.elapsed, alpha, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// --- collectSnapshots tests ---

func TestCollectSnapshots_NilSources(t *testing.T) {
	overlay := &damageDebugOverlay{damageSources: nil}

	snapshots := overlay.collectSnapshots()
	if snapshots != nil {
		t.Errorf("snapshots = %v, want nil with nil damageSources", snapshots)
	}
}

func TestCollectSnapshots_EmptySources(t *testing.T) {
	sources := []*DamageSource{}
	overlay := &damageDebugOverlay{damageSources: &sources}

	snapshots := overlay.collectSnapshots()
	if snapshots != nil {
		t.Errorf("snapshots = %v, want nil with empty sources", snapshots)
	}
}

func TestCollectSnapshots_CopiesRects(t *testing.T) {
	ds := &DamageSource{
		name:  "gg",
		color: color.RGBA{R: 0, G: 200, B: 0, A: 80},
	}
	r := image.Rect(10, 20, 30, 40)
	ds.ReportDamage(r)

	sources := []*DamageSource{ds}
	overlay := &damageDebugOverlay{damageSources: &sources}

	snapshots := overlay.collectSnapshots()

	if len(snapshots) != 1 {
		t.Fatalf("snapshots len = %d, want 1", len(snapshots))
	}
	snap := snapshots[0]
	if snap.Name != "gg" {
		t.Errorf("Name = %q, want %q", snap.Name, "gg")
	}
	if snap.Color != ds.color {
		t.Errorf("Color = %v, want %v", snap.Color, ds.color)
	}
	if len(snap.Rects) != 1 || snap.Rects[0] != r {
		t.Errorf("Rects = %v, want [%v]", snap.Rects, r)
	}

	// Verify snapshot rects are a copy (modifying source doesn't affect snapshot).
	ds.rects[0] = image.Rect(99, 99, 999, 999)
	if snap.Rects[0] == ds.rects[0] {
		t.Error("snapshot rects should be a copy, not share underlying slice")
	}
}

func TestCollectSnapshots_FullDamage(t *testing.T) {
	ds := &DamageSource{name: "g3d"}
	ds.ReportDamage() // full damage

	sources := []*DamageSource{ds}
	overlay := &damageDebugOverlay{damageSources: &sources}

	snapshots := overlay.collectSnapshots()

	if len(snapshots) != 1 {
		t.Fatalf("snapshots len = %d, want 1", len(snapshots))
	}
	if !snapshots[0].Full {
		t.Error("Full = false, want true")
	}
}

func TestCollectSnapshots_Reason(t *testing.T) {
	ds := &DamageSource{name: "g3d"}
	reason := gpucontext.DamageReason{
		Category: gpucontext.DamageCategoryAnimation,
		Detail:   "camera pan",
	}
	ds.ReportDamageWithReason(reason, image.Rect(0, 0, 800, 600))

	sources := []*DamageSource{ds}
	overlay := &damageDebugOverlay{damageSources: &sources}

	snapshots := overlay.collectSnapshots()

	if snapshots[0].Reason != reason {
		t.Errorf("Reason = %+v, want %+v", snapshots[0].Reason, reason)
	}
}

// --- updateFlashes tests ---

func TestUpdateFlashes_AddsNewFlashes(t *testing.T) {
	overlay := &damageDebugOverlay{}
	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name:  "gg",
			Color: color.RGBA{R: 0, G: 200, B: 0, A: 80},
			Rects: []image.Rectangle{image.Rect(0, 0, 100, 100)},
		},
	}

	overlay.updateFlashes(snapshots)

	if len(overlay.flashes) != 1 {
		t.Fatalf("flashes len = %d, want 1", len(overlay.flashes))
	}
	if overlay.flashes[0].name != "gg" {
		t.Errorf("flash name = %q, want %q", overlay.flashes[0].name, "gg")
	}
	if overlay.flashes[0].rect != image.Rect(0, 0, 100, 100) {
		t.Errorf("flash rect = %v, want (0,0)-(100,100)", overlay.flashes[0].rect)
	}
}

func TestUpdateFlashes_FullDamageFlash(t *testing.T) {
	overlay := &damageDebugOverlay{}
	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name: "g3d",
			Full: true,
		},
	}

	overlay.updateFlashes(snapshots)

	if len(overlay.flashes) != 1 {
		t.Fatalf("flashes len = %d, want 1", len(overlay.flashes))
	}
	if !overlay.flashes[0].full {
		t.Error("flash full = false, want true")
	}
}

func TestUpdateFlashes_PrunesExpired(t *testing.T) {
	overlay := &damageDebugOverlay{}
	expired := time.Now().Add(-damageFlashDuration - time.Second)
	overlay.flashes = []damageFlash{
		{name: "old", rect: image.Rect(0, 0, 10, 10), time: expired},
	}

	overlay.updateFlashes(nil) // no new snapshots

	if len(overlay.flashes) != 0 {
		t.Errorf("flashes len = %d after pruning, want 0", len(overlay.flashes))
	}
}

func TestUpdateFlashes_RefreshesExisting(t *testing.T) {
	overlay := &damageDebugOverlay{}
	rect := image.Rect(0, 0, 100, 100)
	oldTime := time.Now().Add(-100 * time.Millisecond)
	overlay.flashes = []damageFlash{
		{name: "gg", rect: rect, time: oldTime},
	}

	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name:  "gg",
			Rects: []image.Rectangle{rect},
		},
	}

	overlay.updateFlashes(snapshots)

	// Should refresh existing flash, not add a new one.
	if len(overlay.flashes) != 1 {
		t.Fatalf("flashes len = %d, want 1 (refreshed, not duplicated)", len(overlay.flashes))
	}
	if !overlay.flashes[0].time.After(oldTime) {
		t.Error("flash time was not refreshed")
	}
}

func TestUpdateFlashes_MultipleSnapshots(t *testing.T) {
	overlay := &damageDebugOverlay{}
	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name:  "gg",
			Color: color.RGBA{R: 0, G: 200, B: 0, A: 80},
			Rects: []image.Rectangle{
				image.Rect(0, 0, 50, 50),
				image.Rect(60, 60, 100, 100),
			},
		},
		{
			Name:  "g3d",
			Color: color.RGBA{R: 0, G: 100, B: 255, A: 80},
			Rects: []image.Rectangle{image.Rect(200, 200, 400, 400)},
		},
	}

	overlay.updateFlashes(snapshots)

	// 2 rects from gg + 1 rect from g3d = 3 flashes.
	if len(overlay.flashes) != 3 {
		t.Errorf("flashes len = %d, want 3", len(overlay.flashes))
	}
}

// --- damageDebugOverlay.Name test ---

func TestDamageDebugOverlay_Name(t *testing.T) {
	overlay := &damageDebugOverlay{}
	if name := overlay.Name(); name != "damage" {
		t.Errorf("Name() = %q, want %q", name, "damage")
	}
}

// --- damageFlashDuration constant test ---

func TestDamageFlashDuration(t *testing.T) {
	// Matches Chromium's ShowDebugBorders 400ms flash duration.
	if damageFlashDuration != 400*time.Millisecond {
		t.Errorf("damageFlashDuration = %v, want 400ms", damageFlashDuration)
	}
}

// --- overlay constant tests ---

func TestOverlayUniformSize(t *testing.T) {
	// screen(2f=8) + pad(2f=8) = 16 bytes.
	if overlayUniformSize != 16 {
		t.Errorf("overlayUniformSize = %d, want 16", overlayUniformSize)
	}
}

func TestOverlayInstanceStride(t *testing.T) {
	// rectXY(f32x2=8) + rectWH(f32x2=8) + color(f32x4=16) = 32 bytes.
	if overlayInstanceStride != 32 {
		t.Errorf("overlayInstanceStride = %d, want 32", overlayInstanceStride)
	}
}

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
