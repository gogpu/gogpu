package gogpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gpucontext"
)

// TestDamageSource_ReportDamage_WithRects verifies that rects are stored
// correctly on a DamageSource.
func TestDamageSource_ReportDamage_WithRects(t *testing.T) {
	ds := &compositor.DamageSource{Name: "test"}
	r1 := image.Rect(0, 0, 100, 100)
	r2 := image.Rect(200, 200, 300, 300)

	ds.ReportDamage(r1, r2)

	if len(ds.Rects) != 2 {
		t.Fatalf("rects len = %d, want 2", len(ds.Rects))
	}
	if ds.Rects[0] != r1 || ds.Rects[1] != r2 {
		t.Errorf("rects = %v, want [%v, %v]", ds.Rects, r1, r2)
	}
	if ds.Full {
		t.Error("full = true, want false when rects are provided")
	}
}

// TestDamageSource_ReportDamage_NoArgs signals full-surface damage.
func TestDamageSource_ReportDamage_NoArgs(t *testing.T) {
	ds := &compositor.DamageSource{Name: "test"}

	ds.ReportDamage()

	if !ds.Full {
		t.Error("full = false, want true when no rects provided")
	}
	if len(ds.Rects) != 0 {
		t.Errorf("rects len = %d, want 0", len(ds.Rects))
	}
}

// TestDamageSource_ReportDamage_Accumulates verifies that multiple calls
// accumulate rects rather than replacing them.
func TestDamageSource_ReportDamage_Accumulates(t *testing.T) {
	ds := &compositor.DamageSource{Name: "test"}

	ds.ReportDamage(image.Rect(0, 0, 10, 10))
	ds.ReportDamage(image.Rect(20, 20, 30, 30))

	if len(ds.Rects) != 2 {
		t.Fatalf("rects len = %d, want 2 after two ReportDamage calls", len(ds.Rects))
	}
}

// TestDamageSource_ReportDamageWithReason verifies reason is stored and rects
// are added via the delegate to ReportDamage.
func TestDamageSource_ReportDamageWithReason(t *testing.T) {
	ds := &compositor.DamageSource{Name: "test"}
	reason := gpucontext.DamageReason{
		Category: gpucontext.DamageCategoryAnimation,
		Detail:   "camera rotation",
	}
	r := image.Rect(0, 0, 800, 600)

	ds.ReportDamageWithReason(reason, r)

	if ds.Reason != reason {
		t.Errorf("reason = %+v, want %+v", ds.Reason, reason)
	}
	if len(ds.Rects) != 1 || ds.Rects[0] != r {
		t.Errorf("rects = %v, want [%v]", ds.Rects, r)
	}
	if ds.Full {
		t.Error("full = true, want false when rects provided with reason")
	}
}

// TestDamageSource_ReportDamageWithReason_NoRects signals full damage with a
// reason attached.
func TestDamageSource_ReportDamageWithReason_NoRects(t *testing.T) {
	ds := &compositor.DamageSource{Name: "test"}
	reason := gpucontext.DamageReason{
		Category: gpucontext.DamageCategoryFull,
		Detail:   "theme change",
	}

	ds.ReportDamageWithReason(reason)

	if !ds.Full {
		t.Error("full = false, want true when no rects provided with reason")
	}
	if ds.Reason != reason {
		t.Errorf("reason = %+v, want %+v", ds.Reason, reason)
	}
}

// TestDamageSource_Reset verifies that reset clears rects, full flag, and
// reason while retaining the slice capacity for reuse.
func TestDamageSource_Reset(t *testing.T) {
	ds := &compositor.DamageSource{Name: "test"}
	ds.ReportDamageWithReason(
		gpucontext.DamageReason{Category: gpucontext.DamageCategoryLayout, Detail: "widget moved"},
		image.Rect(10, 10, 50, 50),
		image.Rect(60, 60, 90, 90),
	)

	// Verify pre-reset state.
	if len(ds.Rects) != 2 {
		t.Fatalf("pre-reset: rects len = %d, want 2", len(ds.Rects))
	}

	ds.Reset()

	if len(ds.Rects) != 0 {
		t.Errorf("after reset: rects len = %d, want 0", len(ds.Rects))
	}
	if cap(ds.Rects) < 2 {
		t.Error("after reset: rects capacity should be retained for reuse")
	}
	if ds.Full {
		t.Error("after reset: full = true, want false")
	}
	if ds.Reason != (gpucontext.DamageReason{}) {
		t.Errorf("after reset: reason = %+v, want zero value", ds.Reason)
	}
}

// TestDamageSource_ImplementsDamageReporter is a compile-time check that
// *DamageSource satisfies the gpucontext.DamageReporter interface.
func TestDamageSource_ImplementsDamageReporter(t *testing.T) {
	var _ gpucontext.DamageReporter = (*compositor.DamageSource)(nil)
}

// --- unionAllSources tests ---

// TestUnionAllSources_SingleSource verifies that rects from a single source
// pass through unchanged.
func TestUnionAllSources_SingleSource(t *testing.T) {
	ds := &compositor.DamageSource{Name: "gg"}
	r1 := image.Rect(0, 0, 100, 100)
	r2 := image.Rect(200, 200, 300, 300)
	ds.ReportDamage(r1, r2)

	result := compositor.UnionAllSources([]*compositor.DamageSource{ds})

	if len(result) != 2 {
		t.Fatalf("result len = %d, want 2", len(result))
	}
	if result[0] != r1 || result[1] != r2 {
		t.Errorf("result = %v, want [%v, %v]", result, r1, r2)
	}
}

// TestUnionAllSources_MultipleSources verifies that rects from all sources are
// unioned into a single slice.
func TestUnionAllSources_MultipleSources(t *testing.T) {
	gg := &compositor.DamageSource{Name: "gg"}
	gg.ReportDamage(image.Rect(0, 0, 100, 100))

	g3d := &compositor.DamageSource{Name: "g3d"}
	g3d.ReportDamage(image.Rect(200, 200, 400, 400))

	result := compositor.UnionAllSources([]*compositor.DamageSource{gg, g3d})

	if len(result) != 2 {
		t.Fatalf("result len = %d, want 2", len(result))
	}
}

// TestUnionAllSources_AnyFullSource_NilResult verifies that if any source
// reports full damage, the result is nil (full surface present).
func TestUnionAllSources_AnyFullSource_NilResult(t *testing.T) {
	tests := []struct {
		name    string
		sources []*compositor.DamageSource
	}{
		{
			name: "first source full",
			sources: func() []*compositor.DamageSource {
				full := &compositor.DamageSource{Name: "g3d"}
				full.ReportDamage() // no rects = full
				partial := &compositor.DamageSource{Name: "gg"}
				partial.ReportDamage(image.Rect(0, 0, 10, 10))
				return []*compositor.DamageSource{full, partial}
			}(),
		},
		{
			name: "second source full",
			sources: func() []*compositor.DamageSource {
				partial := &compositor.DamageSource{Name: "gg"}
				partial.ReportDamage(image.Rect(0, 0, 10, 10))
				full := &compositor.DamageSource{Name: "g3d"}
				full.ReportDamage() // no rects = full
				return []*compositor.DamageSource{partial, full}
			}(),
		},
		{
			name: "all sources full",
			sources: func() []*compositor.DamageSource {
				a := &compositor.DamageSource{Name: "a"}
				a.ReportDamage()
				b := &compositor.DamageSource{Name: "b"}
				b.ReportDamage()
				return []*compositor.DamageSource{a, b}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compositor.UnionAllSources(tt.sources)
			if result != nil {
				t.Errorf("result = %v, want nil (full present)", result)
			}
		})
	}
}

// TestUnionAllSources_NoRects_NilResult verifies that when no sources have
// reported any damage, the result is nil (full present as safe default).
func TestUnionAllSources_NoRects_NilResult(t *testing.T) {
	tests := []struct {
		name    string
		sources []*compositor.DamageSource
	}{
		{
			name:    "empty sources",
			sources: []*compositor.DamageSource{},
		},
		{
			name: "sources with no damage",
			sources: []*compositor.DamageSource{
				{Name: "gg"},
				{Name: "g3d"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compositor.UnionAllSources(tt.sources)
			if result != nil {
				t.Errorf("result = %v, want nil (full present)", result)
			}
		})
	}
}

// TestUnionAllSources_BoundingBox verifies that when combined rect count
// exceeds compositor.MaxDamageRects, a single bounding box is returned.
func TestUnionAllSources_BoundingBox(t *testing.T) {
	ds := &compositor.DamageSource{Name: "gg"}
	// Create compositor.MaxDamageRects+1 rects to trigger bounding box fallback.
	for i := 0; i <= compositor.MaxDamageRects; i++ {
		ds.ReportDamage(image.Rect(i*10, i*10, i*10+5, i*10+5))
	}

	result := compositor.UnionAllSources([]*compositor.DamageSource{ds})

	if len(result) != 1 {
		t.Fatalf("result len = %d, want 1 (bounding box)", len(result))
	}
	// Bounding box should encompass all rects.
	bb := result[0]
	if bb.Min.X != 0 || bb.Min.Y != 0 {
		t.Errorf("bounding box Min = %v, want (0,0)", bb.Min)
	}
	lastRect := image.Rect(compositor.MaxDamageRects*10, compositor.MaxDamageRects*10,
		compositor.MaxDamageRects*10+5, compositor.MaxDamageRects*10+5)
	if bb.Max.X != lastRect.Max.X || bb.Max.Y != lastRect.Max.Y {
		t.Errorf("bounding box Max = %v, want %v", bb.Max, lastRect.Max)
	}
}

// TestUnionAllSources_ExactlyMaxRects verifies that exactly compositor.MaxDamageRects
// rects are returned as-is (no bounding box).
func TestUnionAllSources_ExactlyMaxRects(t *testing.T) {
	ds := &compositor.DamageSource{Name: "gg"}
	for i := 0; i < compositor.MaxDamageRects; i++ {
		ds.ReportDamage(image.Rect(i, i, i+1, i+1))
	}

	result := compositor.UnionAllSources([]*compositor.DamageSource{ds})

	if len(result) != compositor.MaxDamageRects {
		t.Errorf("result len = %d, want %d (individual rects)", len(result), compositor.MaxDamageRects)
	}
}

// --- boundingBox tests ---

// TestBoundingBox verifies correct bounding box computation.
func TestBoundingBox(t *testing.T) {
	tests := []struct {
		name  string
		rects []image.Rectangle
		want  image.Rectangle
	}{
		{
			name:  "single rect",
			rects: []image.Rectangle{image.Rect(10, 20, 30, 40)},
			want:  image.Rect(10, 20, 30, 40),
		},
		{
			name: "two non-overlapping",
			rects: []image.Rectangle{
				image.Rect(0, 0, 10, 10),
				image.Rect(50, 50, 100, 100),
			},
			want: image.Rect(0, 0, 100, 100),
		},
		{
			name: "overlapping",
			rects: []image.Rectangle{
				image.Rect(0, 0, 50, 50),
				image.Rect(25, 25, 75, 75),
			},
			want: image.Rect(0, 0, 75, 75),
		},
		{
			name: "negative coordinates",
			rects: []image.Rectangle{
				image.Rect(-10, -20, 5, 5),
				image.Rect(0, 0, 100, 100),
			},
			want: image.Rect(-10, -20, 100, 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compositor.BoundingBox(tt.rects)
			if got != tt.want {
				t.Errorf("compositor.BoundingBox(%v) = %v, want %v", tt.rects, got, tt.want)
			}
		})
	}
}

// --- RegisterDamageSource tests ---

// TestRegisterDamageSource_PaletteColor verifies that colors are assigned by
// registration order from the compositor.DamagePalette.
func TestRegisterDamageSource_PaletteColor(t *testing.T) {
	ws := newTestWindowSurface()
	ctx := newContext(ws.renderer, 1.0)

	ctx.RegisterDamageSource("gg")
	if ws.damageSources[0].Color != compositor.DamagePalette[0] {
		t.Errorf("first source color = %v, want %v (palette[0])", ws.damageSources[0].Color, compositor.DamagePalette[0])
	}

	ctx.RegisterDamageSource("g3d")
	if ws.damageSources[1].Color != compositor.DamagePalette[1] {
		t.Errorf("second source color = %v, want %v (palette[1])", ws.damageSources[1].Color, compositor.DamagePalette[1])
	}

	ctx.RegisterDamageSource("video")
	if ws.damageSources[2].Color != compositor.DamagePalette[2] {
		t.Errorf("third source color = %v, want %v (palette[2])", ws.damageSources[2].Color, compositor.DamagePalette[2])
	}
}

// TestRegisterDamageSource_PaletteWraps verifies that palette colors wrap
// around when more sources than palette entries are registered.
func TestRegisterDamageSource_PaletteWraps(t *testing.T) {
	ws := newTestWindowSurface()
	ctx := newContext(ws.renderer, 1.0)

	// Register len(compositor.DamagePalette) sources to exhaust palette.
	for i := range len(compositor.DamagePalette) {
		ctx.RegisterDamageSource("src" + string(rune('A'+i)))
	}

	// Next source wraps to palette[0].
	ctx.RegisterDamageSource("overflow")
	wrappedDS := ws.damageSources[len(ws.damageSources)-1]
	if wrappedDS.Color != compositor.DamagePalette[0] {
		t.Errorf("wrapped source color = %v, want %v (palette[0])", wrappedDS.Color, compositor.DamagePalette[0])
	}
}

// TestRegisterDamageSource_Multiple verifies that multiple sources coexist
// on the same surface.
func TestRegisterDamageSource_Multiple(t *testing.T) {
	ws := newTestWindowSurface()
	ctx := newContext(ws.renderer, 1.0)

	ctx.RegisterDamageSource("gg")
	ctx.RegisterDamageSource("g3d")

	if ws.damageSources[0].Name != "gg" {
		t.Errorf("gg.Name = %q, want %q", ws.damageSources[0].Name, "gg")
	}
	if ws.damageSources[1].Name != "g3d" {
		t.Errorf("g3d.Name = %q, want %q", ws.damageSources[1].Name, "g3d")
	}
	if len(ws.damageSources) != 2 {
		t.Errorf("damageSources len = %d, want 2", len(ws.damageSources))
	}
}

// TestRegisterDamageSource_MultiWindow verifies that sources are registered
// on the correct surface in multi-window mode.
func TestRegisterDamageSource_MultiWindow(t *testing.T) {
	primary := newTestWindowSurface()
	secondary := &RenderTarget{renderer: primary.renderer}

	// Register on secondary surface via context targeting.
	ctx := newContextForSurface(primary.renderer, secondary, 1.0)
	ctx.RegisterDamageSource("g3d")

	if len(secondary.damageSources) != 1 {
		t.Errorf("secondary.damageSources len = %d, want 1", len(secondary.damageSources))
	}
	if len(primary.damageSources) != 0 {
		t.Errorf("primary.damageSources len = %d, want 0 (registered on secondary)", len(primary.damageSources))
	}
}

// --- compositor.DamagePalette tests ---

// TestDamagePalette_AllDistinct verifies that all palette colors are visually
// distinct (no duplicates).
func TestDamagePalette_AllDistinct(t *testing.T) {
	seen := make(map[color.RGBA]int)
	for i, c := range compositor.DamagePalette {
		if prev, exists := seen[c]; exists {
			t.Errorf("palette[%d] = palette[%d] = %v (duplicate)", i, prev, c)
		}
		seen[c] = i
	}
}

// TestDamagePalette_AllSemiTransparent verifies that all palette colors have
// non-zero alpha for border rendering. The palette stores BORDER alpha (high
// visibility). Fill alpha is computed dynamically as borderAlpha * fraction.
func TestDamagePalette_AllSemiTransparent(t *testing.T) {
	for i, c := range compositor.DamagePalette {
		if c.A == 0 || c.A == 255 {
			t.Errorf("palette[%d].A = %d, want semi-transparent (1-254)", i, c.A)
		}
	}
}
