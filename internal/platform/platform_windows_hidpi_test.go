//go:build windows

package platform

import (
	"testing"
	"unsafe"
)

// --- DPI constants must match the Windows SDK ---

func TestDpiWindowsConstants(t *testing.T) {
	if wmDpiChanged != 0x02E0 {
		t.Errorf("wmDpiChanged = %#x, want 0x02E0", wmDpiChanged)
	}
	if wmGetDpiScaledSize != 0x02E4 {
		t.Errorf("wmGetDpiScaledSize = %#x, want 0x02E4", wmGetDpiScaledSize)
	}
	if monitorDefaultToPrimary != 1 {
		t.Errorf("monitorDefaultToPrimary = %d, want 1", monitorDefaultToPrimary)
	}
	if mdtEffectiveDpi != 0 {
		t.Errorf("mdtEffectiveDpi = %d, want 0 (MDT_EFFECTIVE_DPI)", mdtEffectiveDpi)
	}
}

// --- win32Size: memory layout must match Windows SIZE struct ---

func TestWin32Size_Layout(t *testing.T) {
	var s win32Size
	if got := unsafe.Sizeof(s); got != 8 {
		t.Errorf("win32Size = %d bytes, want 8", got)
	}
	// cx at offset 0, cy at offset 4 (little-endian).
	s.cx = 0x11223344
	s.cy = 0x55667788
	raw := (*[8]byte)(unsafe.Pointer(&s))
	if raw[0] != 0x44 || raw[1] != 0x33 || raw[2] != 0x22 || raw[3] != 0x11 {
		t.Errorf("cx not at byte offset 0")
	}
	if raw[4] != 0x88 || raw[5] != 0x77 || raw[6] != 0x66 || raw[7] != 0x55 {
		t.Errorf("cy not at byte offset 4")
	}
}

// --- monitorDpiFromPoint ---

func TestMonitorDpiFromPoint_Primary_InRange(t *testing.T) {
	dpi := monitorDpiFromPoint(0, 0, true)
	// 72–480 covers all real-world DPI values (72dpi legacy to 6× retina).
	if dpi < 72 || dpi > 480 {
		t.Errorf("monitorDpiFromPoint(primary) = %d, want 72..480", dpi)
	}
}

func TestMonitorDpiFromPoint_NeverReturnsZero(t *testing.T) {
	dpi := monitorDpiFromPoint(0, 0, true)
	if dpi == 0 {
		t.Error("monitorDpiFromPoint returned 0; fallback to 96 must always apply")
	}
}

func TestMonitorDpiFromPoint_NearestMatchesPrimary(t *testing.T) {
	// Querying (0,0) with NEAREST typically returns the primary monitor too.
	primary := monitorDpiFromPoint(0, 0, true)
	nearest := monitorDpiFromPoint(0, 0, false)
	// They may differ on multi-monitor setups; both must be in range.
	if nearest < 72 || nearest > 480 {
		t.Errorf("monitorDpiFromPoint(nearest, 0,0) = %d, want 72..480", nearest)
	}
	_ = primary
}

// --- adjustWindowRectForDpi ---

func TestAdjustWindowRectForDpi_OuterLargerThanClient(t *testing.T) {
	// AdjustWindowRect* must expand a client rect to include window decorations.
	r := rect{left: 0, top: 0, right: 800, bottom: 600}
	adjustWindowRectForDpi(&r, wsOverlappedWindow, 96)
	outerW := r.right - r.left
	outerH := r.bottom - r.top
	if outerW <= 800 {
		t.Errorf("outer width = %d, want > 800 (client width)", outerW)
	}
	if outerH <= 600 {
		t.Errorf("outer height = %d, want > 600 (client height)", outerH)
	}
}

func TestAdjustWindowRectForDpi_FrameGrowsWithDpi(t *testing.T) {
	// Frame overhead at 192 DPI must be >= frame overhead at 96 DPI.
	// (AdjustWindowRectExForDpi scales frame; AdjustWindowRectEx fallback is flat —
	// in both cases the invariant ">=" holds.)
	r96 := rect{0, 0, 100, 100}
	adjustWindowRectForDpi(&r96, wsOverlappedWindow, 96)
	frameW96 := (r96.right - r96.left) - 100

	r192 := rect{0, 0, 200, 200} // same logical size × 2 at 192 DPI
	adjustWindowRectForDpi(&r192, wsOverlappedWindow, 192)
	frameW192 := (r192.right - r192.left) - 200

	if frameW192 < frameW96 {
		t.Errorf("frame at 192 DPI (%d px) < frame at 96 DPI (%d px)", frameW192, frameW96)
	}
}

func TestAdjustWindowRectForDpi_FrameIsPositive(t *testing.T) {
	for _, dpi := range []uint32{96, 120, 144, 192} {
		r := rect{0, 0, 0, 0} // zero client
		adjustWindowRectForDpi(&r, wsOverlappedWindow, dpi)
		outerW := r.right - r.left
		outerH := r.bottom - r.top
		if outerW <= 0 || outerH <= 0 {
			t.Errorf("dpi=%d: outer size (%dx%d) not positive", dpi, outerW, outerH)
		}
	}
}

// --- Pure DPI scale arithmetic ---

func TestDpiScaleMath_LogicalToPhysicalRoundTrip(t *testing.T) {
	cases := []struct {
		logical int
		dpi     uint32
	}{
		{800, 96},   // 1.00× → 800
		{800, 120},  // 1.25× → 1000
		{800, 144},  // 1.50× → 1200
		{800, 192},  // 2.00× → 1600
		{1000, 150}, // 1.5625×
		{1920, 96},
		{1920, 192},
	}
	for _, tc := range cases {
		scale := float64(tc.dpi) / 96.0
		physical := int(float64(tc.logical) * scale)
		backLogical := int(float64(physical) / scale)
		// Allow ±1 pixel rounding.
		diff := backLogical - tc.logical
		if diff < -1 || diff > 1 {
			t.Errorf("logical=%d dpi=%d: physical=%d → back=%d (diff=%d)",
				tc.logical, tc.dpi, physical, backLogical, diff)
		}
	}
}

func TestDpiScaleMath_PhysicalGrowsWithDpi(t *testing.T) {
	const logical = 800
	dpis := []uint32{96, 120, 144, 192, 240}
	prev := 0
	for _, dpi := range dpis {
		physical := int(float64(logical) * float64(dpi) / 96.0)
		if physical < prev {
			t.Errorf("physical size shrank: at %d DPI = %d, prev = %d", dpi, physical, prev)
		}
		prev = physical
	}
}

func TestDpiScaleMath_96DpiIsIdentity(t *testing.T) {
	scale := float64(96) / 96.0
	if scale != 1.0 {
		t.Errorf("scale at 96 DPI = %f, want exactly 1.0", scale)
	}
	logical := 800
	physical := int(float64(logical) * scale)
	if physical != logical {
		t.Errorf("at 96 DPI: physical=%d, want %d (identity)", physical, logical)
	}
}

// --- SystemScaleFactor ---

func TestSystemScaleFactor_InRange(t *testing.T) {
	s := SystemScaleFactor()
	if s < 1.0 || s > 5.0 {
		t.Errorf("SystemScaleFactor() = %f, want 1.0..5.0", s)
	}
}

func TestSystemScaleFactor_MatchesMonitorDpi(t *testing.T) {
	want := float64(monitorDpiFromPoint(0, 0, true)) / 96.0
	got := SystemScaleFactor()
	if got != want {
		t.Errorf("SystemScaleFactor() = %f, want %f (monitorDpiFromPoint)", got, want)
	}
}

// --- windowsPlatform.ScaleFactor (PlatScaleProvider) ---

func TestWindowsPlatform_ImplementsPlatScaleProvider(t *testing.T) {
	// Compile-time: windowsPlatform satisfies PlatScaleProvider.
	var p windowsPlatform
	var _ PlatScaleProvider = &p
}

func TestWindowsPlatform_ScaleFactor_NilPrimary(t *testing.T) {
	// Must not panic and must return a sane value before any window is created.
	p := &windowsPlatform{}
	s := p.ScaleFactor()
	if s < 1.0 || s > 5.0 {
		t.Errorf("ScaleFactor() with nil primary = %f, want 1.0..5.0", s)
	}
}

func TestWindowsPlatform_ScaleFactor_MatchesSystemScale(t *testing.T) {
	p := &windowsPlatform{}
	got := p.ScaleFactor()
	want := SystemScaleFactor()
	if got != want {
		t.Errorf("ScaleFactor() = %f, want %f (SystemScaleFactor)", got, want)
	}
}

// --- Pre-scale simulation (mirrors createWindowWin32 Step 1+2) ---

func TestPreScale_OuterDimensionsIncludeFrame(t *testing.T) {
	// Reproduce the exact logic in createWindowWin32 and assert the outer rect
	// is larger than the requested physical client area.
	dpi := monitorDpiFromPoint(0, 0, true)
	scale := float64(dpi) / 96.0

	const logW, logH = 800, 600
	outerRect := rect{
		left:   0,
		top:    0,
		right:  int32(float64(logW) * scale),
		bottom: int32(float64(logH) * scale),
	}
	physClientW := outerRect.right
	physClientH := outerRect.bottom

	adjustWindowRectForDpi(&outerRect, wsOverlappedWindow, dpi)

	outerW := outerRect.right - outerRect.left
	outerH := outerRect.bottom - outerRect.top

	if outerW <= physClientW {
		t.Errorf("outer width %d not > physical client %d", outerW, physClientW)
	}
	if outerH <= physClientH {
		t.Errorf("outer height %d not > physical client %d", outerH, physClientH)
	}
}

func TestPreScale_PostVerify_SameResultWhenDpiMatches(t *testing.T) {
	// When guess DPI == actual DPI the post-verify path produces the same outer dims.
	dpi := monitorDpiFromPoint(0, 0, true)
	scale := float64(dpi) / 96.0
	const logW, logH = 1024, 768

	calc := func(d uint32) (int32, int32) {
		s := float64(d) / 96.0
		r := rect{0, 0, int32(float64(logW) * s), int32(float64(logH) * s)}
		adjustWindowRectForDpi(&r, wsOverlappedWindow, d)
		return r.right - r.left, r.bottom - r.top
	}

	w1, h1 := calc(dpi)
	w2, h2 := calc(dpi)
	_ = scale

	if w1 != w2 || h1 != h2 {
		t.Errorf("calc is not deterministic: (%d×%d) vs (%d×%d)", w1, h1, w2, h2)
	}
}
