package compositor

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDamageSource_ReportDamage_WithRects(t *testing.T) {
	ds := NewDamageSource("test", 0)
	r1 := image.Rect(0, 0, 100, 100)
	r2 := image.Rect(200, 200, 300, 300)

	ds.ReportDamage(r1, r2)

	if len(ds.Rects) != 2 {
		t.Fatalf("Rects len = %d, want 2", len(ds.Rects))
	}
	if ds.Full {
		t.Error("Full = true, want false when rects are provided")
	}
}

func TestDamageSource_ReportDamage_NoArgs(t *testing.T) {
	ds := NewDamageSource("test", 0)
	ds.ReportDamage()

	if !ds.Full {
		t.Error("Full = false, want true when no rects provided")
	}
}

func TestDamageSource_ReportDamage_Accumulates(t *testing.T) {
	ds := NewDamageSource("test", 0)
	ds.ReportDamage(image.Rect(0, 0, 10, 10))
	ds.ReportDamage(image.Rect(20, 20, 30, 30))

	if len(ds.Rects) != 2 {
		t.Fatalf("Rects len = %d, want 2", len(ds.Rects))
	}
}

func TestDamageSource_ReportDamageWithReason(t *testing.T) {
	ds := NewDamageSource("test", 0)
	reason := gpucontext.DamageReason{
		Category: gpucontext.DamageCategoryAnimation,
		Detail:   "camera rotation",
	}
	ds.ReportDamageWithReason(reason, image.Rect(0, 0, 800, 600))

	if ds.Reason != reason {
		t.Errorf("Reason = %+v, want %+v", ds.Reason, reason)
	}
	if len(ds.Rects) != 1 {
		t.Errorf("Rects len = %d, want 1", len(ds.Rects))
	}
}

func TestDamageSource_Reset(t *testing.T) {
	ds := NewDamageSource("test", 0)
	ds.ReportDamageWithReason(
		gpucontext.DamageReason{Category: gpucontext.DamageCategoryLayout},
		image.Rect(10, 10, 50, 50),
		image.Rect(60, 60, 90, 90),
	)

	ds.Reset()

	if len(ds.Rects) != 0 {
		t.Errorf("after Reset: Rects len = %d, want 0", len(ds.Rects))
	}
	if cap(ds.Rects) < 2 {
		t.Error("after Reset: Rects capacity should be retained")
	}
	if ds.Full {
		t.Error("after Reset: Full = true, want false")
	}
}

func TestDamageSource_ImplementsDamageReporter(t *testing.T) {
	var _ gpucontext.DamageReporter = (*DamageSource)(nil)
}

func TestNewDamageSource_PaletteColor(t *testing.T) {
	ds0 := NewDamageSource("gg", 0)
	ds1 := NewDamageSource("g3d", 1)
	dsWrap := NewDamageSource("overflow", len(DamagePalette))

	if ds0.Color != DamagePalette[0] {
		t.Errorf("color[0] = %v, want %v", ds0.Color, DamagePalette[0])
	}
	if ds1.Color != DamagePalette[1] {
		t.Errorf("color[1] = %v, want %v", ds1.Color, DamagePalette[1])
	}
	if dsWrap.Color != DamagePalette[0] {
		t.Errorf("wrapped color = %v, want %v (palette[0])", dsWrap.Color, DamagePalette[0])
	}
}

func TestUnionAllSources_SingleSource(t *testing.T) {
	ds := NewDamageSource("gg", 0)
	r1 := image.Rect(0, 0, 100, 100)
	r2 := image.Rect(200, 200, 300, 300)
	ds.ReportDamage(r1, r2)

	result := UnionAllSources([]*DamageSource{ds})
	if len(result) != 2 {
		t.Fatalf("result len = %d, want 2", len(result))
	}
}

func TestUnionAllSources_AnyFullSource_NilResult(t *testing.T) {
	full := NewDamageSource("g3d", 0)
	full.ReportDamage()
	partial := NewDamageSource("gg", 1)
	partial.ReportDamage(image.Rect(0, 0, 10, 10))

	result := UnionAllSources([]*DamageSource{full, partial})
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestUnionAllSources_NoRects_NilResult(t *testing.T) {
	result := UnionAllSources([]*DamageSource{{Name: "gg"}, {Name: "g3d"}})
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestUnionAllSources_BoundingBox(t *testing.T) {
	ds := NewDamageSource("gg", 0)
	for i := 0; i <= MaxDamageRects; i++ {
		ds.ReportDamage(image.Rect(i*10, i*10, i*10+5, i*10+5))
	}

	result := UnionAllSources([]*DamageSource{ds})
	if len(result) != 1 {
		t.Fatalf("result len = %d, want 1 (bounding box)", len(result))
	}
}

func TestDamagePalette_AllDistinct(t *testing.T) {
	seen := make(map[color.RGBA]int)
	for i, c := range DamagePalette {
		if prev, exists := seen[c]; exists {
			t.Errorf("palette[%d] = palette[%d] = %v (duplicate)", i, prev, c)
		}
		seen[c] = i
	}
}

func TestBoundingBox(t *testing.T) {
	got := BoundingBox([]image.Rectangle{
		image.Rect(0, 0, 10, 10),
		image.Rect(50, 50, 100, 100),
	})
	want := image.Rect(0, 0, 100, 100)
	if got != want {
		t.Errorf("BoundingBox = %v, want %v", got, want)
	}
}
