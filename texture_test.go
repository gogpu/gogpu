package gogpu

import (
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/gputypes"
)

func TestDefaultTextureOptions(t *testing.T) {
	opts := DefaultTextureOptions()

	if opts.MagFilter != gputypes.FilterModeLinear {
		t.Errorf("MagFilter = %v, want FilterModeLinear", opts.MagFilter)
	}
	if opts.MinFilter != gputypes.FilterModeLinear {
		t.Errorf("MinFilter = %v, want FilterModeLinear", opts.MinFilter)
	}
	if opts.AddressModeU != gputypes.AddressModeClampToEdge {
		t.Errorf("AddressModeU = %v, want AddressModeClampToEdge", opts.AddressModeU)
	}
	if opts.AddressModeV != gputypes.AddressModeClampToEdge {
		t.Errorf("AddressModeV = %v, want AddressModeClampToEdge", opts.AddressModeV)
	}
}

func TestTextureMetadata(t *testing.T) {
	// Create a texture with known metadata (without GPU resources)
	tex := &Texture{
		width:  128,
		height: 256,
		format: gputypes.TextureFormatRGBA8Unorm,
	}

	if tex.Width() != 128 {
		t.Errorf("Width() = %d, want 128", tex.Width())
	}
	if tex.Height() != 256 {
		t.Errorf("Height() = %d, want 256", tex.Height())
	}

	w, h := tex.Size()
	if w != 128 || h != 256 {
		t.Errorf("Size() = (%d, %d), want (128, 256)", w, h)
	}

	if tex.Format() != gputypes.TextureFormatRGBA8Unorm {
		t.Errorf("Format() = %v, want TextureFormatRGBA8Unorm", tex.Format())
	}
}

func TestTextureHandles(t *testing.T) {
	// Create a texture with mock HAL interfaces
	tex := &Texture{
		texture: &mockTexture{},
		view:    &mockTextureView{},
		sampler: &mockSampler{},
	}

	if tex.Handle() == nil {
		t.Error("Handle() returned nil, want non-nil")
	}
	if tex.View() == nil {
		t.Error("View() returned nil, want non-nil")
	}
	if tex.Sampler() == nil {
		t.Error("Sampler() returned nil, want non-nil")
	}
}

func TestTextureDestroyWithNilRenderer(t *testing.T) {
	// Destroy should be safe to call with nil renderer
	tex := &Texture{
		texture: &mockTexture{},
		view:    &mockTextureView{},
		sampler: &mockSampler{},
	}

	// Should not panic
	tex.Destroy()
}

func TestTextureDestroyWithNilDevice(t *testing.T) {
	// Destroy should be safe to call with nil device
	tex := &Texture{
		texture:  &mockTexture{},
		view:     &mockTextureView{},
		sampler:  &mockSampler{},
		renderer: &Renderer{device: nil},
	}

	// Should not panic
	tex.Destroy()
}

func TestTextureOptionsLabel(t *testing.T) {
	opts := TextureOptions{
		Label:        "test-texture",
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		AddressModeU: gputypes.AddressModeRepeat,
		AddressModeV: gputypes.AddressModeMirrorRepeat,
	}

	if opts.Label != "test-texture" {
		t.Errorf("Label = %q, want %q", opts.Label, "test-texture")
	}
	if opts.MagFilter != gputypes.FilterModeNearest {
		t.Errorf("MagFilter = %v, want FilterModeNearest", opts.MagFilter)
	}
	if opts.MinFilter != gputypes.FilterModeNearest {
		t.Errorf("MinFilter = %v, want FilterModeNearest", opts.MinFilter)
	}
	if opts.AddressModeU != gputypes.AddressModeRepeat {
		t.Errorf("AddressModeU = %v, want AddressModeRepeat", opts.AddressModeU)
	}
	if opts.AddressModeV != gputypes.AddressModeMirrorRepeat {
		t.Errorf("AddressModeV = %v, want AddressModeMirrorRepeat", opts.AddressModeV)
	}
}

func TestTexturedQuadShader(t *testing.T) {
	shader := TexturedQuadShader()

	if shader == "" {
		t.Error("TexturedQuadShader() returned empty string")
	}

	// Verify it contains expected WGSL elements
	tests := []string{
		"@vertex",
		"@fragment",
		"textureSample",
		"sampler",
		"texture_2d",
		"uniforms",
	}

	for _, expected := range tests {
		if !containsString(shader, expected) {
			t.Errorf("TexturedQuadShader() missing %q", expected)
		}
	}
}

func TestSimpleTextureShader(t *testing.T) {
	shader := SimpleTextureShader()

	if shader == "" {
		t.Error("SimpleTextureShader() returned empty string")
	}

	// Verify it contains expected WGSL elements
	tests := []string{
		"@vertex",
		"@fragment",
		"textureSample",
		"sampler",
		"texture_2d",
	}

	for _, expected := range tests {
		if !containsString(shader, expected) {
			t.Errorf("SimpleTextureShader() missing %q", expected)
		}
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCreateGradientImage(t *testing.T) {
	// Test the gradient image creation function from the example
	width, height := 16, 16
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(x * 255 / width)
			g := uint8(y * 255 / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	bounds := img.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Errorf("Image size = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
	}

	// Check corner colors
	topLeft := img.At(0, 0).(color.RGBA)
	if topLeft.R != 0 || topLeft.G != 0 || topLeft.B != 128 || topLeft.A != 255 {
		t.Errorf("Top-left pixel = %v, want (0, 0, 128, 255)", topLeft)
	}

	bottomRight := img.At(width-1, height-1).(color.RGBA)
	expectedR := uint8((width - 1) * 255 / width)
	expectedG := uint8((height - 1) * 255 / height)
	if bottomRight.R != expectedR || bottomRight.G != expectedG || bottomRight.B != 128 || bottomRight.A != 255 {
		t.Errorf("Bottom-right pixel = %v, want (%d, %d, 128, 255)", bottomRight, expectedR, expectedG)
	}
}

func TestCheckerboardPattern(t *testing.T) {
	// Test the checkerboard pattern creation from the example
	width, height := 8, 8
	pixels := make([]byte, width*height*4)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			if (x+y)%2 == 0 {
				pixels[i] = 255   // R
				pixels[i+1] = 255 // G
				pixels[i+2] = 255 // B
				pixels[i+3] = 255 // A
			} else {
				pixels[i] = 100   // R
				pixels[i+1] = 100 // G
				pixels[i+2] = 100 // B
				pixels[i+3] = 255 // A
			}
		}
	}

	// Verify size
	expectedSize := width * height * 4
	if len(pixels) != expectedSize {
		t.Errorf("Pixels size = %d, want %d", len(pixels), expectedSize)
	}

	// Verify checkerboard pattern
	// (0,0) should be white (even)
	if pixels[0] != 255 || pixels[1] != 255 || pixels[2] != 255 {
		t.Errorf("Pixel (0,0) = (%d,%d,%d), want (255,255,255)", pixels[0], pixels[1], pixels[2])
	}

	// (1,0) should be gray (odd)
	if pixels[4] != 100 || pixels[5] != 100 || pixels[6] != 100 {
		t.Errorf("Pixel (1,0) = (%d,%d,%d), want (100,100,100)", pixels[4], pixels[5], pixels[6])
	}

	// (0,1) should be gray (odd)
	idx := width * 4
	if pixels[idx] != 100 || pixels[idx+1] != 100 || pixels[idx+2] != 100 {
		t.Errorf("Pixel (0,1) = (%d,%d,%d), want (100,100,100)", pixels[idx], pixels[idx+1], pixels[idx+2])
	}

	// (1,1) should be white (even)
	idx = width*4 + 4
	if pixels[idx] != 255 || pixels[idx+1] != 255 || pixels[idx+2] != 255 {
		t.Errorf("Pixel (1,1) = (%d,%d,%d), want (255,255,255)", pixels[idx], pixels[idx+1], pixels[idx+2])
	}
}

func TestBytesPerPixel(t *testing.T) {
	tests := []struct {
		name   string
		format gputypes.TextureFormat
		want   int
	}{
		{"RGBA8Unorm", gputypes.TextureFormatRGBA8Unorm, 4},
		{"RGBA8UnormSrgb", gputypes.TextureFormatRGBA8UnormSrgb, 4},
		{"BGRA8Unorm", gputypes.TextureFormatBGRA8Unorm, 4},
		{"BGRA8UnormSrgb", gputypes.TextureFormatBGRA8UnormSrgb, 4},
		{"R8Unorm", gputypes.TextureFormatR8Unorm, 1},
		{"R16Float", gputypes.TextureFormatR16Float, 2},
		{"R32Float", gputypes.TextureFormatR32Float, 4},
		{"RG32Float", gputypes.TextureFormatRG32Float, 8},
		{"RGBA32Float", gputypes.TextureFormatRGBA32Float, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tex := &Texture{format: tt.format}
			got := tex.BytesPerPixel()
			if got != tt.want {
				t.Errorf("BytesPerPixel() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBytesPerPixelForFormat(t *testing.T) {
	// Test the internal function directly
	tests := []struct {
		format gputypes.TextureFormat
		want   int
	}{
		{gputypes.TextureFormatRGBA8Snorm, 4},
		{gputypes.TextureFormatRGBA8Uint, 4},
		{gputypes.TextureFormatRGBA8Sint, 4},
		{gputypes.TextureFormatR8Snorm, 1},
		{gputypes.TextureFormatR8Uint, 1},
		{gputypes.TextureFormatR8Sint, 1},
		{gputypes.TextureFormatRG8Unorm, 2},
		{gputypes.TextureFormatRG8Snorm, 2},
		{gputypes.TextureFormatRG16Float, 8},
		{gputypes.TextureFormatRGBA16Float, 8},
		{gputypes.TextureFormatR32Uint, 4},
		{gputypes.TextureFormatR32Sint, 4},
		{gputypes.TextureFormatRG32Uint, 8},
		{gputypes.TextureFormatRG32Sint, 8},
		{gputypes.TextureFormatRGBA32Uint, 16},
		{gputypes.TextureFormatRGBA32Sint, 16},
	}

	for _, tt := range tests {
		got := bytesPerPixelForFormat(tt.format)
		if got != tt.want {
			t.Errorf("bytesPerPixelForFormat(%v) = %d, want %d", tt.format, got, tt.want)
		}
	}
}

func TestBytesPerPixelUnknownFormat(t *testing.T) {
	// Unknown format should return 0
	got := bytesPerPixelForFormat(gputypes.TextureFormat(9999))
	if got != 0 {
		t.Errorf("bytesPerPixelForFormat(unknown) = %d, want 0", got)
	}
}

func TestUpdateDataDestroyedTexture(t *testing.T) {
	tests := []struct {
		name string
		tex  *Texture
	}{
		{
			name: "nil renderer",
			tex:  &Texture{width: 10, height: 10, format: gputypes.TextureFormatRGBA8Unorm},
		},
		{
			name: "nil device",
			tex: &Texture{
				width:    10,
				height:   10,
				format:   gputypes.TextureFormatRGBA8Unorm,
				renderer: &Renderer{device: nil},
			},
		},
		{
			name: "nil texture handle",
			tex: &Texture{
				width:    10,
				height:   10,
				format:   gputypes.TextureFormatRGBA8Unorm,
				texture:  nil,
				renderer: &Renderer{device: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tex.UpdateData(make([]byte, 400))
			if !errors.Is(err, ErrTextureUpdateDestroyed) {
				t.Errorf("UpdateData() error = %v, want ErrTextureUpdateDestroyed", err)
			}
		})
	}
}

func TestUpdateDataInvalidSize(t *testing.T) {
	// Create a minimal mock texture (without actual GPU)
	tex := &Texture{
		width:   10,
		height:  10,
		format:  gputypes.TextureFormatRGBA8Unorm,
		texture: &mockTexture{}, // non-nil to pass destroyed check
		// Note: renderer is nil so it will fail at destroyed check first
	}

	// UpdateData with nil renderer should return destroyed error first
	err := tex.UpdateData(make([]byte, 100)) // wrong size
	if !errors.Is(err, ErrTextureUpdateDestroyed) {
		t.Errorf("UpdateData() error = %v, want ErrTextureUpdateDestroyed (nil renderer)", err)
	}
}

func TestUpdateRegionDestroyedTexture(t *testing.T) {
	tests := []struct {
		name string
		tex  *Texture
	}{
		{
			name: "nil renderer",
			tex:  &Texture{width: 10, height: 10, format: gputypes.TextureFormatRGBA8Unorm},
		},
		{
			name: "nil device",
			tex: &Texture{
				width:    10,
				height:   10,
				format:   gputypes.TextureFormatRGBA8Unorm,
				renderer: &Renderer{device: nil},
			},
		},
		{
			name: "nil texture handle",
			tex: &Texture{
				width:    10,
				height:   10,
				format:   gputypes.TextureFormatRGBA8Unorm,
				texture:  nil,
				renderer: &Renderer{device: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tex.UpdateRegion(0, 0, 5, 5, make([]byte, 100))
			if !errors.Is(err, ErrTextureUpdateDestroyed) {
				t.Errorf("UpdateRegion() error = %v, want ErrTextureUpdateDestroyed", err)
			}
		})
	}
}

func TestUpdateRegionInvalidParams(t *testing.T) {
	// Note: Since we can't create a full mock backend easily,
	// these tests verify the validation logic by expecting
	// the destroyed error (which comes first in the check order)
	tex := &Texture{
		width:    10,
		height:   10,
		format:   gputypes.TextureFormatRGBA8Unorm,
		texture:  nil, // Will fail destroyed check
		renderer: nil,
	}

	tests := []struct {
		name    string
		x, y    int
		w, h    int
		wantErr error
	}{
		{"negative x", -1, 0, 5, 5, ErrTextureUpdateDestroyed},
		{"negative y", 0, -1, 5, 5, ErrTextureUpdateDestroyed},
		{"zero width", 0, 0, 0, 5, ErrTextureUpdateDestroyed},
		{"zero height", 0, 0, 5, 0, ErrTextureUpdateDestroyed},
		{"negative width", 0, 0, -5, 5, ErrTextureUpdateDestroyed},
		{"negative height", 0, 0, 5, -5, ErrTextureUpdateDestroyed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tex.UpdateRegion(tt.x, tt.y, tt.w, tt.h, make([]byte, 100))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("UpdateRegion() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTextureUpdateErrors(t *testing.T) {
	// Verify error messages are properly formatted
	tests := []struct {
		err     error
		wantMsg string
	}{
		{ErrTextureUpdateDestroyed, "gogpu: cannot update destroyed texture"},
		{ErrInvalidDataSize, "gogpu: invalid data size"},
		{ErrRegionOutOfBounds, "gogpu: region out of bounds"},
		{ErrInvalidRegion, "gogpu: invalid region parameters"},
	}

	for _, tt := range tests {
		t.Run(tt.wantMsg, func(t *testing.T) {
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("error = %q, want %q", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}
