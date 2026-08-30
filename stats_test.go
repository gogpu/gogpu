package gogpu

import (
	"errors"
	"testing"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	_ "github.com/gogpu/wgpu/hal/software"
)

// TestUpdateRegionStrideValidation covers bytesPerRow validation without a live GPU
// after the destroyed check is bypassed via a non-nil texture handle stub is not
// available — these use a live headless renderer.
func TestUpdateRegionStrideValidation(t *testing.T) {
	EnableStats()
	t.Cleanup(DisableStats)

	r, err := NewHeadlessRenderer()
	if err != nil {
		t.Fatalf("NewHeadlessRenderer: %v", err)
	}
	t.Cleanup(func() {
		r.Destroy()
		r.ReleaseInstance()
	})

	const tw, th = 8, 8
	full := make([]byte, tw*th*4)
	tex, err := r.NewTextureFromRGBA(tw, th, full)
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	t.Cleanup(tex.Destroy)

	tests := []struct {
		name        string
		x, y, w, h  int
		bytesPerRow int
		dataLen     int
		wantErr     error
	}{
		{
			name: "tight packed zero stride",
			w:    4, h: 2, bytesPerRow: 0, dataLen: 4 * 2 * 4,
		},
		{
			name: "explicit packed stride",
			w:    4, h: 2, bytesPerRow: 16, dataLen: 4 * 2 * 4,
		},
		{
			name: "strided full-frame buffer band",
			// Region 4x2 from an 8-wide RGBA buffer: stride = 8*4 = 32.
			// Need stride*(h-1)+packedRow = 32*1+16 = 48 bytes minimum.
			w: 4, h: 2, bytesPerRow: 32, dataLen: 48,
		},
		{
			name: "stride too small",
			w:    4, h: 2, bytesPerRow: 8, dataLen: 100, wantErr: ErrInvalidStride,
		},
		{
			name: "data too short for stride",
			w:    4, h: 2, bytesPerRow: 32, dataLen: 40, wantErr: ErrInvalidDataSize,
		},
		{
			name: "region out of bounds",
			x:    6, w: 4, h: 1, bytesPerRow: 0, dataLen: 16, wantErr: ErrRegionOutOfBounds,
		},
		{
			name: "invalid region zero height",
			w:    4, h: 0, bytesPerRow: 0, dataLen: 16, wantErr: ErrInvalidRegion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			err := tex.UpdateRegion(tt.x, tt.y, tt.w, tt.h, tt.bytesPerRow, data)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("UpdateRegion: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("UpdateRegion error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestUpdateRegionStrideZeroCopyBand verifies the geckty/ggcanvas use case:
// upload a dirty horizontal band from a full-frame RGBA buffer without extractRegion.
func TestUpdateRegionStrideZeroCopyBand(t *testing.T) {
	EnableStats()
	defer DisableStats()

	r, err := NewHeadlessRenderer()
	if err != nil {
		t.Fatalf("NewHeadlessRenderer: %v", err)
	}
	defer r.Destroy()
	defer r.ReleaseInstance()
	r.ResetCounters()
	r.beginFrameStats()

	const (
		fw, fh = 80, 40 // grid where 3 dirty rows are < 10% of the frame (#484 acceptance)
		bpp    = 4
	)
	frame := make([]byte, fw*fh*bpp)
	// Paint dirty rows 5..7 with a recognizable pattern.
	for y := 5; y < 8; y++ {
		for x := 0; x < fw; x++ {
			i := (y*fw + x) * bpp
			frame[i] = 0x11
			frame[i+1] = 0x22
			frame[i+2] = 0x33
			frame[i+3] = 0xFF
		}
	}

	tex, err := r.NewTextureFromRGBA(fw, fh, frame)
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	defer tex.Destroy()

	r.beginFrameStats() // isolate band upload from create upload
	dirtyY, dirtyH := 5, 3
	bandOffset := dirtyY * fw * bpp
	band := frame[bandOffset:] // zero-copy slice into full frame
	if err := tex.UpdateRegion(0, dirtyY, fw, dirtyH, fw*bpp, band); err != nil {
		t.Fatalf("strided UpdateRegion: %v", err)
	}

	fs := (&Context{renderer: r}).FrameStats()
	fullBytes := int64(fw * fh * bpp)
	bandBytes := int64(fw * dirtyH * bpp)
	if fs.UploadBytes != bandBytes {
		t.Fatalf("FrameStats.UploadBytes = %d, want band %d", fs.UploadBytes, bandBytes)
	}
	if fs.UploadRegions != 1 {
		t.Fatalf("FrameStats.UploadRegions = %d, want 1", fs.UploadRegions)
	}
	// Acceptance from #484: dirty rows ≪ full frame.
	if fs.UploadBytes*10 >= fullBytes {
		t.Fatalf("upload %d bytes is not < 10%% of full frame %d", fs.UploadBytes, fullBytes)
	}

	counters := r.GetCounters()
	if counters.Textures != 1 {
		t.Fatalf("GetCounters.Textures = %d, want 1", counters.Textures)
	}
	if counters.TextureBytesAllocated != fullBytes {
		t.Fatalf("TextureBytesAllocated = %d, want %d", counters.TextureBytesAllocated, fullBytes)
	}
}

func TestUpdateRegionImplementsTextureRegionUpdater(t *testing.T) {
	var _ gpucontext.TextureRegionUpdater = (*Texture)(nil)
}

func TestStatsDisabledIsZeroCost(t *testing.T) {
	DisableStats()
	r, err := NewHeadlessRenderer()
	if err != nil {
		t.Fatalf("NewHeadlessRenderer: %v", err)
	}
	defer r.Destroy()
	defer r.ReleaseInstance()

	data := make([]byte, 4*4*4)
	tex, err := r.NewTextureFromRGBA(4, 4, data)
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	defer tex.Destroy()

	if got := r.GetCounters(); got != (DeviceCounters{}) {
		t.Fatalf("GetCounters with stats off = %+v, want zero", got)
	}
	ctx := &Context{renderer: r}
	if got := ctx.FrameStats(); got != (FrameStats{}) {
		t.Fatalf("FrameStats with stats off = %+v, want zero", got)
	}
}

func TestDeviceCountersTrackAllocFree(t *testing.T) {
	EnableStats()
	defer DisableStats()

	r, err := NewHeadlessRenderer()
	if err != nil {
		t.Fatalf("NewHeadlessRenderer: %v", err)
	}
	defer r.Destroy()
	defer r.ReleaseInstance()
	r.ResetCounters()

	data := make([]byte, 16*16*4)
	tex, err := r.NewTextureFromRGBA(16, 16, data)
	if err != nil {
		t.Fatalf("NewTextureFromRGBA: %v", err)
	}
	c := r.GetCounters()
	if c.Textures != 1 || c.TextureBytesAllocated != 16*16*4 {
		t.Fatalf("after create: %+v", c)
	}

	tex.Destroy()
	c = r.GetCounters()
	if c.Textures != 0 || c.TextureBytesAllocated != 0 {
		t.Fatalf("after destroy: %+v", c)
	}
}

func TestFrameStatsPresentSkippedOnIdleEndFrame(t *testing.T) {
	EnableStats()
	defer DisableStats()

	r := &Renderer{primary: &RenderTarget{}}
	r.primary.renderer = r
	r.beginFrameStats()
	r.EndFrame() // frameStarted=false → present skipped

	last := r.GetFrameStats()
	if !last.PresentSkipped {
		t.Fatal("PresentSkipped = false, want true for idle EndFrame")
	}
}

func TestUpdateRegionValidationOrderDestroyedFirst(t *testing.T) {
	// Destroyed check must precede region/stride validation (existing contract).
	tex := &Texture{
		width:  10,
		height: 10,
		format: gputypes.TextureFormatRGBA8Unorm,
	}
	err := tex.UpdateRegion(0, 0, 5, 5, 1, nil) // stride too small would be ErrInvalidStride if live
	if !errors.Is(err, ErrTextureUpdateDestroyed) {
		t.Fatalf("error = %v, want ErrTextureUpdateDestroyed", err)
	}
}
