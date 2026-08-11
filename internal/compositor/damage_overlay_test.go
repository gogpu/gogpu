package compositor

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/gogpu/gpucontext"
)

// --- ParseDamageDebugMode tests ---

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
			// ParseDamageDebugMode reads os.Getenv directly.
			mode := ParseDamageDebugMode()
			if mode.Overlay != tt.wantOvl {
				t.Errorf("overlay = %v, want %v", mode.Overlay, tt.wantOvl)
			}
			if mode.Log != tt.wantLog {
				t.Errorf("log = %v, want %v", mode.Log, tt.wantLog)
			}
		})
	}
}

// --- fadeAlpha tests ---

func TestFadeAlpha(t *testing.T) {
	overlay := &DamageDebugOverlay{}

	tests := []struct {
		name    string
		elapsed time.Duration
		wantMin float32
		wantMax float32
	}{
		{"at start", 0, 0.99, 1.01},
		{"half duration", DamageFlashDuration / 2, 0.45, 0.55},
		{"at expiry", DamageFlashDuration, -0.01, 0.01},
		{"past expiry", DamageFlashDuration + time.Second, -0.01, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			flash := &DamageFlash{Time: now.Add(-tt.elapsed)}
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
	overlay := &DamageDebugOverlay{DamageSources: nil}

	snapshots := overlay.collectSnapshots()
	if snapshots != nil {
		t.Errorf("snapshots = %v, want nil with nil damageSources", snapshots)
	}
}

func TestCollectSnapshots_EmptySources(t *testing.T) {
	sources := []*DamageSource{}
	overlay := &DamageDebugOverlay{DamageSources: &sources}

	snapshots := overlay.collectSnapshots()
	if snapshots != nil {
		t.Errorf("snapshots = %v, want nil with empty sources", snapshots)
	}
}

func TestCollectSnapshots_CopiesRects(t *testing.T) {
	ds := &DamageSource{
		Name:  "gg",
		Color: color.RGBA{R: 0, G: 200, B: 0, A: 80},
	}
	r := image.Rect(10, 20, 30, 40)
	ds.ReportDamage(r)

	sources := []*DamageSource{ds}
	overlay := &DamageDebugOverlay{DamageSources: &sources}

	snapshots := overlay.collectSnapshots()

	if len(snapshots) != 1 {
		t.Fatalf("snapshots len = %d, want 1", len(snapshots))
	}
	snap := snapshots[0]
	if snap.Name != "gg" {
		t.Errorf("Name = %q, want %q", snap.Name, "gg")
	}
	if snap.Color != ds.Color {
		t.Errorf("Color = %v, want %v", snap.Color, ds.Color)
	}
	if len(snap.Rects) != 1 || snap.Rects[0] != r {
		t.Errorf("Rects = %v, want [%v]", snap.Rects, r)
	}

	// Verify snapshot rects are a copy (modifying source doesn't affect snapshot).
	ds.Rects[0] = image.Rect(99, 99, 999, 999)
	if snap.Rects[0] == ds.Rects[0] {
		t.Error("snapshot rects should be a copy, not share underlying slice")
	}
}

func TestCollectSnapshots_FullDamage(t *testing.T) {
	ds := &DamageSource{Name: "g3d"}
	ds.ReportDamage() // full damage

	sources := []*DamageSource{ds}
	overlay := &DamageDebugOverlay{DamageSources: &sources}

	snapshots := overlay.collectSnapshots()

	if len(snapshots) != 1 {
		t.Fatalf("snapshots len = %d, want 1", len(snapshots))
	}
	if !snapshots[0].Full {
		t.Error("Full = false, want true")
	}
}

func TestCollectSnapshots_Reason(t *testing.T) {
	ds := &DamageSource{Name: "g3d"}
	reason := gpucontext.DamageReason{
		Category: gpucontext.DamageCategoryAnimation,
		Detail:   "camera pan",
	}
	ds.ReportDamageWithReason(reason, image.Rect(0, 0, 800, 600))

	sources := []*DamageSource{ds}
	overlay := &DamageDebugOverlay{DamageSources: &sources}

	snapshots := overlay.collectSnapshots()

	if snapshots[0].Reason != reason {
		t.Errorf("Reason = %+v, want %+v", snapshots[0].Reason, reason)
	}
}

// --- updateFlashes tests ---

func TestUpdateFlashes_AddsNewFlashes(t *testing.T) {
	overlay := &DamageDebugOverlay{}
	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name:  "gg",
			Color: color.RGBA{R: 0, G: 200, B: 0, A: 80},
			Rects: []image.Rectangle{image.Rect(0, 0, 100, 100)},
		},
	}

	overlay.updateFlashes(snapshots)

	if len(overlay.Flashes) != 1 {
		t.Fatalf("flashes len = %d, want 1", len(overlay.Flashes))
	}
	if overlay.Flashes[0].Name != "gg" {
		t.Errorf("flash name = %q, want %q", overlay.Flashes[0].Name, "gg")
	}
	if overlay.Flashes[0].Rect != image.Rect(0, 0, 100, 100) {
		t.Errorf("flash rect = %v, want (0,0)-(100,100)", overlay.Flashes[0].Rect)
	}
}

func TestUpdateFlashes_FullDamageFlash(t *testing.T) {
	overlay := &DamageDebugOverlay{}
	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name: "g3d",
			Full: true,
		},
	}

	overlay.updateFlashes(snapshots)

	if len(overlay.Flashes) != 1 {
		t.Fatalf("flashes len = %d, want 1", len(overlay.Flashes))
	}
	if !overlay.Flashes[0].Full {
		t.Error("flash full = false, want true")
	}
}

func TestUpdateFlashes_PrunesExpired(t *testing.T) {
	overlay := &DamageDebugOverlay{}
	expired := time.Now().Add(-DamageFlashDuration - time.Second)
	overlay.Flashes = []DamageFlash{
		{Name: "old", Rect: image.Rect(0, 0, 10, 10), Time: expired},
	}

	overlay.updateFlashes(nil) // no new snapshots

	if len(overlay.Flashes) != 0 {
		t.Errorf("flashes len = %d after pruning, want 0", len(overlay.Flashes))
	}
}

func TestUpdateFlashes_RefreshesExisting(t *testing.T) {
	overlay := &DamageDebugOverlay{}
	rect := image.Rect(0, 0, 100, 100)
	oldTime := time.Now().Add(-100 * time.Millisecond)
	overlay.Flashes = []DamageFlash{
		{Name: "gg", Rect: rect, Time: oldTime},
	}

	snapshots := []gpucontext.DamageSourceSnapshot{
		{
			Name:  "gg",
			Rects: []image.Rectangle{rect},
		},
	}

	overlay.updateFlashes(snapshots)

	// Should refresh existing flash, not add a new one.
	if len(overlay.Flashes) != 1 {
		t.Fatalf("flashes len = %d, want 1 (refreshed, not duplicated)", len(overlay.Flashes))
	}
	if !overlay.Flashes[0].Time.After(oldTime) {
		t.Error("flash time was not refreshed")
	}
}

func TestUpdateFlashes_MultipleSnapshots(t *testing.T) {
	overlay := &DamageDebugOverlay{}
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
	if len(overlay.Flashes) != 3 {
		t.Errorf("flashes len = %d, want 3", len(overlay.Flashes))
	}
}

// --- DamageDebugOverlay.Name test ---

func TestDamageDebugOverlay_Name(t *testing.T) {
	overlay := &DamageDebugOverlay{}
	if name := overlay.Name(); name != "damage" {
		t.Errorf("Name() = %q, want %q", name, "damage")
	}
}

// --- DamageFlashDuration constant test ---

func TestDamageFlashDuration(t *testing.T) {
	// Matches Chromium's ShowDebugBorders 400ms flash duration.
	if DamageFlashDuration != 400*time.Millisecond {
		t.Errorf("DamageFlashDuration = %v, want 400ms", DamageFlashDuration)
	}
}

// --- overlay constant tests ---

func TestOverlayUniformSize(t *testing.T) {
	// screen(2f=8) + pad(2f=8) = 16 bytes.
	if OverlayUniformSize != 16 {
		t.Errorf("OverlayUniformSize = %d, want 16", OverlayUniformSize)
	}
}

func TestOverlayInstanceStride(t *testing.T) {
	// rectXY(f32x2=8) + rectWH(f32x2=8) + color(f32x4=16) = 32 bytes.
	if OverlayInstanceStride != 32 {
		t.Errorf("OverlayInstanceStride = %d, want 32", OverlayInstanceStride)
	}
}
