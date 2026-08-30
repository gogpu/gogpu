package gogpu

import (
	"errors"
	"image"
	"testing"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	_ "github.com/gogpu/wgpu/hal/software"
)

// TestUpdateRegionStrideValidation covers layout validation with a live headless renderer.
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
		name    string
		region  image.Rectangle
		layout  gpucontext.ImageDataLayout
		dataLen int
		wantErr error
	}{
		{
			name:    "tight packed zero layout",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{},
			dataLen: 4 * 2 * 4,
		},
		{
			name:    "explicit packed stride",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{BytesPerRow: 16},
			dataLen: 4 * 2 * 4,
		},
		{
			name:    "strided full-frame buffer band",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{BytesPerRow: 32},
			dataLen: 48, // stride*(h-1)+packedRow = 32*1+16
		},
		{
			name:    "stride too small",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{BytesPerRow: 8},
			dataLen: 100,
			wantErr: ErrInvalidStride,
		},
		{
			name:    "data too short for stride",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{BytesPerRow: 32},
			dataLen: 40,
			wantErr: ErrInvalidDataSize,
		},
		{
			name:    "region out of bounds",
			region:  image.Rect(6, 0, 10, 1),
			layout:  gpucontext.ImageDataLayout{},
			dataLen: 16,
			wantErr: ErrRegionOutOfBounds,
		},
		{
			name:    "invalid region zero height",
			region:  image.Rect(0, 0, 4, 0),
			layout:  gpucontext.ImageDataLayout{},
			dataLen: 16,
			wantErr: ErrInvalidRegion,
		},
		{
			name:    "layout offset within buffer",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{BytesPerRow: 16, Offset: 8},
			dataLen: 8 + 4*2*4,
		},
		{
			name:    "layout offset past buffer",
			region:  image.Rect(0, 0, 4, 2),
			layout:  gpucontext.ImageDataLayout{Offset: 100},
			dataLen: 32,
			wantErr: ErrInvalidDataSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			err := tex.UpdateRegion(tt.region, data, tt.layout)
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

// TestUpdateRegionStrideZeroCopyBand verifies the ggcanvas-style use case:
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

	r.beginFrameStats()
	dirtyY, dirtyH := 5, 3
	stride := fw * bpp
	band := frame[dirtyY*stride:] // zero-copy slice into full frame
	region := image.Rect(0, dirtyY, fw, dirtyY+dirtyH)
	layout := gpucontext.ImageDataLayout{BytesPerRow: stride}
	if err := tex.UpdateRegion(region, band, layout); err != nil {
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
	r.EndFrame()

	last := r.GetFrameStats()
	if !last.PresentSkipped {
		t.Fatal("PresentSkipped = false, want true for idle EndFrame")
	}
}

func TestUpdateRegionValidationOrderDestroyedFirst(t *testing.T) {
	tex := &Texture{
		width:  10,
		height: 10,
		format: gputypes.TextureFormatRGBA8Unorm,
	}
	err := tex.UpdateRegion(image.Rect(0, 0, 5, 5), nil, gpucontext.ImageDataLayout{BytesPerRow: 1})
	if !errors.Is(err, ErrTextureUpdateDestroyed) {
		t.Fatalf("error = %v, want ErrTextureUpdateDestroyed", err)
	}
}
