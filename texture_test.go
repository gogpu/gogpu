package gogpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/gogpu/gpu/types"
)

func TestDefaultTextureOptions(t *testing.T) {
	opts := DefaultTextureOptions()

	if opts.MagFilter != types.FilterModeLinear {
		t.Errorf("MagFilter = %v, want FilterModeLinear", opts.MagFilter)
	}
	if opts.MinFilter != types.FilterModeLinear {
		t.Errorf("MinFilter = %v, want FilterModeLinear", opts.MinFilter)
	}
	if opts.AddressModeU != types.AddressModeClampToEdge {
		t.Errorf("AddressModeU = %v, want AddressModeClampToEdge", opts.AddressModeU)
	}
	if opts.AddressModeV != types.AddressModeClampToEdge {
		t.Errorf("AddressModeV = %v, want AddressModeClampToEdge", opts.AddressModeV)
	}
}

func TestTextureMetadata(t *testing.T) {
	// Create a texture with known metadata (without GPU resources)
	tex := &Texture{
		width:  128,
		height: 256,
		format: types.TextureFormatRGBA8Unorm,
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

	if tex.Format() != types.TextureFormatRGBA8Unorm {
		t.Errorf("Format() = %v, want TextureFormatRGBA8Unorm", tex.Format())
	}
}

func TestTextureHandles(t *testing.T) {
	// Create a texture with known handles (without GPU resources)
	tex := &Texture{
		texture: types.Texture(42),
		view:    types.TextureView(43),
		sampler: types.Sampler(44),
	}

	if tex.Handle() != 42 {
		t.Errorf("Handle() = %d, want 42", tex.Handle())
	}
	if tex.View() != 43 {
		t.Errorf("View() = %d, want 43", tex.View())
	}
	if tex.Sampler() != 44 {
		t.Errorf("Sampler() = %d, want 44", tex.Sampler())
	}
}

func TestTextureDestroyWithNilRenderer(t *testing.T) {
	// Destroy should be safe to call with nil renderer
	tex := &Texture{
		texture: types.Texture(42),
		view:    types.TextureView(43),
		sampler: types.Sampler(44),
	}

	// Should not panic
	tex.Destroy()
}

func TestTextureDestroyWithNilBackend(t *testing.T) {
	// Destroy should be safe to call with nil backend
	tex := &Texture{
		texture:  types.Texture(42),
		view:     types.TextureView(43),
		sampler:  types.Sampler(44),
		renderer: &Renderer{backend: nil},
	}

	// Should not panic
	tex.Destroy()
}

func TestTextureOptionsLabel(t *testing.T) {
	opts := TextureOptions{
		Label:        "test-texture",
		MagFilter:    types.FilterModeNearest,
		MinFilter:    types.FilterModeNearest,
		AddressModeU: types.AddressModeRepeat,
		AddressModeV: types.AddressModeMirrorRepeat,
	}

	if opts.Label != "test-texture" {
		t.Errorf("Label = %q, want %q", opts.Label, "test-texture")
	}
	if opts.MagFilter != types.FilterModeNearest {
		t.Errorf("MagFilter = %v, want FilterModeNearest", opts.MagFilter)
	}
	if opts.MinFilter != types.FilterModeNearest {
		t.Errorf("MinFilter = %v, want FilterModeNearest", opts.MinFilter)
	}
	if opts.AddressModeU != types.AddressModeRepeat {
		t.Errorf("AddressModeU = %v, want AddressModeRepeat", opts.AddressModeU)
	}
	if opts.AddressModeV != types.AddressModeMirrorRepeat {
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
