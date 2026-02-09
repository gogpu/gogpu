package sdf

import (
	"errors"
	"testing"

	"github.com/gogpu/gg"
)

// ---------------------------------------------------------------------------
// packPixelData / unpackPixelData round-trip tests
// ---------------------------------------------------------------------------

func TestPackUnpackRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		stride int // 0 means stride == width*4
		data   []byte
	}{
		{
			name:   "single pixel opaque red",
			width:  1,
			height: 1,
			data:   []byte{255, 0, 0, 255},
		},
		{
			name:   "single pixel transparent black",
			width:  1,
			height: 1,
			data:   []byte{0, 0, 0, 0},
		},
		{
			name:   "single pixel full white",
			width:  1,
			height: 1,
			data:   []byte{255, 255, 255, 255},
		},
		{
			name:   "2x2 mixed colors no extra stride",
			width:  2,
			height: 2,
			data: []byte{
				255, 0, 0, 255, 0, 255, 0, 128,
				0, 0, 255, 64, 128, 128, 128, 255,
			},
		},
		{
			name:   "2x2 with extra stride padding",
			width:  2,
			height: 2,
			stride: 12, // 2*4=8, extra 4 bytes padding per row
			data: []byte{
				10, 20, 30, 40, 50, 60, 70, 80, 0, 0, 0, 0,
				90, 100, 110, 120, 130, 140, 150, 160, 0, 0, 0, 0,
			},
		},
		{
			name:   "3x1 single row",
			width:  3,
			height: 1,
			data: []byte{
				1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
			},
		},
		{
			name:   "1x3 single column",
			width:  1,
			height: 3,
			data: []byte{
				100, 200, 50, 255,
				0, 0, 0, 0,
				77, 88, 99, 128,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stride := tt.stride
			if stride == 0 {
				stride = tt.width * 4
			}

			target := gg.GPURenderTarget{
				Data:   make([]byte, len(tt.data)),
				Width:  tt.width,
				Height: tt.height,
				Stride: stride,
			}
			copy(target.Data, tt.data)

			packed := packPixelData(target)

			// Verify packed size: width * height * 4 (no stride padding)
			expectedPackedLen := tt.width * tt.height * 4
			if len(packed) != expectedPackedLen {
				t.Fatalf("packed length = %d, want %d", len(packed), expectedPackedLen)
			}

			// Now unpack back into a fresh target
			result := gg.GPURenderTarget{
				Data:   make([]byte, len(tt.data)),
				Width:  tt.width,
				Height: tt.height,
				Stride: stride,
			}
			unpackPixelData(result, packed)

			// Verify the pixel data matches (only the actual pixel area, not padding)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					srcIdx := y*stride + x*4
					dstIdx := y*stride + x*4
					for c := 0; c < 4; c++ {
						if result.Data[dstIdx+c] != tt.data[srcIdx+c] {
							t.Errorf("pixel (%d,%d) channel %d: got %d, want %d",
								x, y, c, result.Data[dstIdx+c], tt.data[srcIdx+c])
						}
					}
				}
			}
		})
	}
}

func TestPackPixelDataStrideHandling(t *testing.T) {
	// Verify that stride padding bytes are NOT included in the packed output.
	// Create a 2x2 target with stride=16 (2*4=8 active bytes, 8 padding bytes per row).
	stride := 16
	data := make([]byte, stride*2) // 2 rows
	// Row 0: pixel (0,0)=red, pixel (1,0)=green, padding=0xFF
	data[0], data[1], data[2], data[3] = 255, 0, 0, 255           // pixel (0,0)
	data[4], data[5], data[6], data[7] = 0, 255, 0, 255           // pixel (1,0)
	data[8], data[9], data[10], data[11] = 0xFF, 0xFF, 0xFF, 0xFF // padding
	data[12], data[13], data[14], data[15] = 0xFF, 0xFF, 0xFF, 0xFF
	// Row 1: pixel (0,1)=blue, pixel (1,1)=white, padding=0xAA
	data[16], data[17], data[18], data[19] = 0, 0, 255, 255     // pixel (0,1)
	data[20], data[21], data[22], data[23] = 255, 255, 255, 255 // pixel (1,1)
	data[24], data[25], data[26], data[27] = 0xAA, 0xAA, 0xAA, 0xAA
	data[28], data[29], data[30], data[31] = 0xAA, 0xAA, 0xAA, 0xAA

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  2,
		Height: 2,
		Stride: stride,
	}

	packed := packPixelData(target)

	// Packed should be exactly 2*2*4 = 16 bytes (no padding)
	if len(packed) != 16 {
		t.Fatalf("packed length = %d, want 16", len(packed))
	}

	// Verify packed contents: 4 pixels in sequence without padding
	expected := []byte{
		255, 0, 0, 255, // pixel (0,0)
		0, 255, 0, 255, // pixel (1,0)
		0, 0, 255, 255, // pixel (0,1)
		255, 255, 255, 255, // pixel (1,1)
	}

	for i := range expected {
		if packed[i] != expected[i] {
			t.Errorf("packed[%d] = %d, want %d", i, packed[i], expected[i])
		}
	}
}

func TestUnpackPixelDataWritesToCorrectStride(t *testing.T) {
	// Verify unpack writes pixels at the correct stride positions
	// and does not overwrite padding bytes.
	stride := 12 // 2*4=8 active + 4 padding per row
	dst := make([]byte, stride*2)
	// Fill with sentinel value
	for i := range dst {
		dst[i] = 0xDD
	}

	target := gg.GPURenderTarget{
		Data:   dst,
		Width:  2,
		Height: 2,
		Stride: stride,
	}

	packed := []byte{
		10, 20, 30, 40,
		50, 60, 70, 80,
		90, 100, 110, 120,
		130, 140, 150, 160,
	}

	unpackPixelData(target, packed)

	// Row 0: pixels should be overwritten, padding should remain 0xDD
	wantRow0 := []byte{10, 20, 30, 40, 50, 60, 70, 80, 0xDD, 0xDD, 0xDD, 0xDD}
	wantRow1 := []byte{90, 100, 110, 120, 130, 140, 150, 160, 0xDD, 0xDD, 0xDD, 0xDD}

	for i := 0; i < stride; i++ {
		if dst[i] != wantRow0[i] {
			t.Errorf("row 0 byte %d: got %d, want %d", i, dst[i], wantRow0[i])
		}
		if dst[stride+i] != wantRow1[i] {
			t.Errorf("row 1 byte %d: got %d, want %d", i, dst[stride+i], wantRow1[i])
		}
	}
}

// ---------------------------------------------------------------------------
// getColorFromPaint tests
// ---------------------------------------------------------------------------

func TestGetColorFromPaint(t *testing.T) {
	red := gg.RGBA{R: 1, G: 0, B: 0, A: 1}
	green := gg.RGBA{R: 0, G: 1, B: 0, A: 1}

	tests := []struct {
		name  string
		paint *gg.Paint
		want  gg.RGBA
	}{
		{
			name:  "solid brush returns brush color",
			paint: &gg.Paint{Brush: gg.Solid(red)},
			want:  red,
		},
		{
			name:  "custom brush returns ColorAt(0,0)",
			paint: &gg.Paint{Brush: gg.CustomBrush{Func: func(_, _ float64) gg.RGBA { return green }, Name: "test"}},
			want:  green,
		},
		{
			name:  "nil brush returns black",
			paint: &gg.Paint{},
			want:  gg.Black,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getColorFromPaint(tt.paint)
			if got != tt.want {
				t.Errorf("getColorFromPaint() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetColorFromPaintSemiTransparent(t *testing.T) {
	semiRed := gg.RGBA{R: 1, G: 0, B: 0, A: 0.5}
	paint := &gg.Paint{Brush: gg.Solid(semiRed)}
	got := getColorFromPaint(paint)
	if got != semiRed {
		t.Errorf("getColorFromPaint() = %+v, want %+v", got, semiRed)
	}
}

// ---------------------------------------------------------------------------
// NewAccelerator and Name tests
// ---------------------------------------------------------------------------

func TestNewAcceleratorCreatesValidStruct(t *testing.T) {
	a := NewAccelerator(nil, 0, 0)
	if a == nil {
		t.Fatal("NewAccelerator returned nil")
	}
	if a.initialized {
		t.Error("new accelerator should not be initialized")
	}
}

func TestAcceleratorName(t *testing.T) {
	a := NewAccelerator(nil, 0, 0)
	if got := a.Name(); got != "sdf-gpu" {
		t.Errorf("Name() = %q, want %q", got, "sdf-gpu")
	}
}

// ---------------------------------------------------------------------------
// CanAccelerate tests
// ---------------------------------------------------------------------------

func TestCanAccelerate(t *testing.T) {
	a := &Accelerator{}

	tests := []struct {
		name string
		op   gg.AcceleratedOp
		want bool
	}{
		{"circle SDF", gg.AccelCircleSDF, true},
		{"rrect SDF", gg.AccelRRectSDF, true},
		{"circle + rrect combined", gg.AccelCircleSDF | gg.AccelRRectSDF, true},
		{"fill op", gg.AccelFill, false},
		{"stroke op", gg.AccelStroke, false},
		{"scene op", gg.AccelScene, false},
		{"text op", gg.AccelText, false},
		{"image op", gg.AccelImage, false},
		{"gradient op", gg.AccelGradient, false},
		{"zero value", 0, false},
		{"all non-SDF ops", gg.AccelFill | gg.AccelStroke | gg.AccelScene | gg.AccelText, false},
		{"mixed: fill + circle", gg.AccelFill | gg.AccelCircleSDF, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.CanAccelerate(tt.op); got != tt.want {
				t.Errorf("CanAccelerate(%d) = %v, want %v", tt.op, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FillPath / StrokePath tests — always return ErrFallbackToCPU
// ---------------------------------------------------------------------------

func TestFillPathAlwaysFallsBack(t *testing.T) {
	a := &Accelerator{}
	target := gg.GPURenderTarget{}

	err := a.FillPath(target, nil, nil)
	if !errors.Is(err, gg.ErrFallbackToCPU) {
		t.Errorf("FillPath() = %v, want ErrFallbackToCPU", err)
	}
}

func TestStrokePathAlwaysFallsBack(t *testing.T) {
	a := &Accelerator{}
	target := gg.GPURenderTarget{}

	err := a.StrokePath(target, nil, nil)
	if !errors.Is(err, gg.ErrFallbackToCPU) {
		t.Errorf("StrokePath() = %v, want ErrFallbackToCPU", err)
	}
}

// ---------------------------------------------------------------------------
// FillShape / StrokeShape when not initialized — CPU fallback
// ---------------------------------------------------------------------------

func TestFillShapeNotInitializedFallsBackToSDFCPU(t *testing.T) {
	// When not initialized, FillShape delegates to the embedded cpuFallback
	// (gg.SDFAccelerator). For a circle shape with a valid target, the CPU
	// fallback should render without error.
	a := &Accelerator{initialized: false}

	// Create a small target with enough data for the CPU SDF fallback
	width, height := 4, 4
	stride := width * 4
	data := make([]byte, stride*height)

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  width,
		Height: height,
		Stride: stride,
	}

	shape := gg.DetectedShape{
		Kind:    gg.ShapeCircle,
		CenterX: 2,
		CenterY: 2,
		RadiusX: 1.5,
		RadiusY: 1.5,
	}

	paint := gg.NewPaint()

	err := a.FillShape(target, shape, paint)
	// CPU fallback should succeed for a valid circle
	if err != nil {
		t.Errorf("FillShape (not initialized, circle) returned error: %v", err)
	}
}

func TestStrokeShapeNotInitializedFallsBackToSDFCPU(t *testing.T) {
	a := &Accelerator{initialized: false}

	width, height := 4, 4
	stride := width * 4
	data := make([]byte, stride*height)

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  width,
		Height: height,
		Stride: stride,
	}

	shape := gg.DetectedShape{
		Kind:    gg.ShapeCircle,
		CenterX: 2,
		CenterY: 2,
		RadiusX: 1.5,
		RadiusY: 1.5,
	}

	paint := gg.NewPaint()

	err := a.StrokeShape(target, shape, paint)
	if err != nil {
		t.Errorf("StrokeShape (not initialized, circle) returned error: %v", err)
	}
}

func TestFillShapeNotInitializedUnknownShapeFallsBack(t *testing.T) {
	a := &Accelerator{initialized: false}

	target := gg.GPURenderTarget{
		Data:   make([]byte, 16),
		Width:  1,
		Height: 1,
		Stride: 4,
	}

	shape := gg.DetectedShape{Kind: gg.ShapeUnknown}
	paint := gg.NewPaint()

	err := a.FillShape(target, shape, paint)
	if !errors.Is(err, gg.ErrFallbackToCPU) {
		t.Errorf("FillShape (not initialized, unknown shape) = %v, want ErrFallbackToCPU", err)
	}
}

func TestStrokeShapeNotInitializedUnknownShapeFallsBack(t *testing.T) {
	a := &Accelerator{initialized: false}

	target := gg.GPURenderTarget{
		Data:   make([]byte, 16),
		Width:  1,
		Height: 1,
		Stride: 4,
	}

	shape := gg.DetectedShape{Kind: gg.ShapeUnknown}
	paint := gg.NewPaint()

	err := a.StrokeShape(target, shape, paint)
	if !errors.Is(err, gg.ErrFallbackToCPU) {
		t.Errorf("StrokeShape (not initialized, unknown shape) = %v, want ErrFallbackToCPU", err)
	}
}

// ---------------------------------------------------------------------------
// FillShape / StrokeShape when initialized (requires GPU - skip in CI)
// ---------------------------------------------------------------------------

func TestFillShapeInitializedRequiresGPU(t *testing.T) {
	t.Skip("requires GPU hardware")
}

func TestStrokeShapeInitializedRequiresGPU(t *testing.T) {
	t.Skip("requires GPU hardware")
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

func TestAcceleratorImplementsGPUAccelerator(t *testing.T) {
	// Compile-time check is already in accelerator.go via:
	//   var _ gg.GPUAccelerator = (*Accelerator)(nil)
	// This test documents the contract for readers.
	var _ gg.GPUAccelerator = (*Accelerator)(nil)
}

// ---------------------------------------------------------------------------
// Shader constant sizes
// ---------------------------------------------------------------------------

func TestCircleUniformSize(t *testing.T) {
	// SDFCircleParams has 12 f32/u32 fields: 12 * 4 = 48 bytes
	if circleUniformSize != 48 {
		t.Errorf("circleUniformSize = %d, want 48", circleUniformSize)
	}
}

func TestRRectUniformSize(t *testing.T) {
	// SDFRRectParams has 14 f32/u32 fields: 14 * 4 = 56 bytes
	if rrectUniformSize != 56 {
		t.Errorf("rrectUniformSize = %d, want 56", rrectUniformSize)
	}
}

// ---------------------------------------------------------------------------
// FillShape / StrokeShape shape dispatch (not initialized)
// ---------------------------------------------------------------------------

func TestFillShapeNotInitializedRRect(t *testing.T) {
	a := &Accelerator{initialized: false}

	width, height := 8, 8
	stride := width * 4
	data := make([]byte, stride*height)

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  width,
		Height: height,
		Stride: stride,
	}

	shape := gg.DetectedShape{
		Kind:         gg.ShapeRRect,
		CenterX:      4,
		CenterY:      4,
		Width:        6,
		Height:       6,
		CornerRadius: 1,
	}

	paint := gg.NewPaint()

	err := a.FillShape(target, shape, paint)
	if err != nil {
		t.Errorf("FillShape (not initialized, rrect) returned error: %v", err)
	}
}

func TestFillShapeNotInitializedRect(t *testing.T) {
	a := &Accelerator{initialized: false}

	width, height := 8, 8
	stride := width * 4
	data := make([]byte, stride*height)

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  width,
		Height: height,
		Stride: stride,
	}

	shape := gg.DetectedShape{
		Kind:    gg.ShapeRect,
		CenterX: 4,
		CenterY: 4,
		Width:   6,
		Height:  6,
	}

	paint := gg.NewPaint()

	err := a.FillShape(target, shape, paint)
	if err != nil {
		t.Errorf("FillShape (not initialized, rect) returned error: %v", err)
	}
}

func TestFillShapeNotInitializedEllipse(t *testing.T) {
	a := &Accelerator{initialized: false}

	width, height := 8, 8
	stride := width * 4
	data := make([]byte, stride*height)

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  width,
		Height: height,
		Stride: stride,
	}

	shape := gg.DetectedShape{
		Kind:    gg.ShapeEllipse,
		CenterX: 4,
		CenterY: 4,
		RadiusX: 3,
		RadiusY: 2,
	}

	paint := gg.NewPaint()

	err := a.FillShape(target, shape, paint)
	if err != nil {
		t.Errorf("FillShape (not initialized, ellipse) returned error: %v", err)
	}
}

func TestStrokeShapeNotInitializedRRect(t *testing.T) {
	a := &Accelerator{initialized: false}

	width, height := 8, 8
	stride := width * 4
	data := make([]byte, stride*height)

	target := gg.GPURenderTarget{
		Data:   data,
		Width:  width,
		Height: height,
		Stride: stride,
	}

	shape := gg.DetectedShape{
		Kind:         gg.ShapeRRect,
		CenterX:      4,
		CenterY:      4,
		Width:        6,
		Height:       6,
		CornerRadius: 1,
	}

	paint := gg.NewPaint()

	err := a.StrokeShape(target, shape, paint)
	if err != nil {
		t.Errorf("StrokeShape (not initialized, rrect) returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Close on non-initialized accelerator (no-op, should not panic)
// ---------------------------------------------------------------------------

func TestCloseNonInitialized(t *testing.T) {
	a := NewAccelerator(nil, 0, 0)
	// Should not panic when backend is nil and handles are zero
	a.Close()
	if a.initialized {
		t.Error("Close should set initialized to false")
	}
}

// ---------------------------------------------------------------------------
// packPixelData edge cases
// ---------------------------------------------------------------------------

func TestPackPixelDataZeroSize(t *testing.T) {
	target := gg.GPURenderTarget{
		Data:   nil,
		Width:  0,
		Height: 0,
		Stride: 0,
	}

	packed := packPixelData(target)
	if len(packed) != 0 {
		t.Errorf("packed length for zero-size target = %d, want 0", len(packed))
	}
}

func TestPackPixelDataAllZeroAlpha(t *testing.T) {
	// All pixels fully transparent
	target := gg.GPURenderTarget{
		Data:   []byte{0, 0, 0, 0, 0, 0, 0, 0},
		Width:  2,
		Height: 1,
		Stride: 8,
	}

	packed := packPixelData(target)
	for i, b := range packed {
		if b != 0 {
			t.Errorf("packed[%d] = %d, want 0 for all-transparent pixels", i, b)
		}
	}
}
