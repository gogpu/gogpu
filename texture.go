package gogpu

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"io"
	"os"

	"github.com/gogpu/gogpu/gpu/types"
)

// Texture represents a GPU texture resource with its associated view and sampler.
// It provides a high-level interface for working with textures in GoGPU.
type Texture struct {
	// GPU resources
	texture types.Texture
	view    types.TextureView
	sampler types.Sampler

	// Metadata
	width  int
	height int
	format types.TextureFormat

	// Reference to renderer for resource management
	renderer *Renderer
}

// Width returns the texture width in pixels.
func (t *Texture) Width() int {
	return t.width
}

// Height returns the texture height in pixels.
func (t *Texture) Height() int {
	return t.height
}

// Size returns the texture dimensions.
func (t *Texture) Size() (width, height int) {
	return t.width, t.height
}

// Format returns the texture format.
func (t *Texture) Format() types.TextureFormat {
	return t.format
}

// Handle returns the underlying GPU texture handle.
// For advanced use cases that need direct GPU access.
func (t *Texture) Handle() types.Texture {
	return t.texture
}

// View returns the texture view handle.
func (t *Texture) View() types.TextureView {
	return t.view
}

// Sampler returns the sampler handle.
func (t *Texture) Sampler() types.Sampler {
	return t.sampler
}

// Destroy releases all GPU resources associated with this texture.
// After calling Destroy, the texture should not be used.
func (t *Texture) Destroy() {
	if t.renderer == nil || t.renderer.backend == nil {
		return
	}

	if t.sampler != 0 {
		t.renderer.backend.ReleaseSampler(t.sampler)
		t.sampler = 0
	}
	if t.view != 0 {
		t.renderer.backend.ReleaseTextureView(t.view)
		t.view = 0
	}
	if t.texture != 0 {
		t.renderer.backend.ReleaseTexture(t.texture)
		t.texture = 0
	}
}

// TextureOptions configures texture creation.
type TextureOptions struct {
	// Label for debugging (optional)
	Label string

	// Filter mode for magnification (default: Linear)
	MagFilter types.FilterMode

	// Filter mode for minification (default: Linear)
	MinFilter types.FilterMode

	// Address mode for U coordinate (default: ClampToEdge)
	AddressModeU types.AddressMode

	// Address mode for V coordinate (default: ClampToEdge)
	AddressModeV types.AddressMode
}

// DefaultTextureOptions returns sensible defaults for texture creation.
func DefaultTextureOptions() TextureOptions {
	return TextureOptions{
		MagFilter:    types.FilterModeLinear,
		MinFilter:    types.FilterModeLinear,
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
	}
}

// LoadTexture loads a texture from a file path.
// Supports PNG and JPEG formats.
func (r *Renderer) LoadTexture(path string) (*Texture, error) {
	return r.LoadTextureWithOptions(path, DefaultTextureOptions())
}

// LoadTextureWithOptions loads a texture with custom options.
//
//nolint:gosec // G304: File path comes from user - intentional for texture loading.
func (r *Renderer) LoadTextureWithOptions(path string, opts TextureOptions) (*Texture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gogpu: failed to open texture file: %w", err)
	}
	defer func() { _ = file.Close() }()

	return r.LoadTextureFromReaderWithOptions(file, opts)
}

// LoadTextureFromReader loads a texture from an io.Reader.
func (r *Renderer) LoadTextureFromReader(reader io.Reader) (*Texture, error) {
	return r.LoadTextureFromReaderWithOptions(reader, DefaultTextureOptions())
}

// LoadTextureFromReaderWithOptions loads a texture from an io.Reader with custom options.
func (r *Renderer) LoadTextureFromReaderWithOptions(reader io.Reader, opts TextureOptions) (*Texture, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("gogpu: failed to decode image: %w", err)
	}

	return r.NewTextureFromImageWithOptions(img, opts)
}

// NewTextureFromImage creates a texture from a Go image.Image.
func (r *Renderer) NewTextureFromImage(img image.Image) (*Texture, error) {
	return r.NewTextureFromImageWithOptions(img, DefaultTextureOptions())
}

// NewTextureFromImageWithOptions creates a texture from a Go image.Image with custom options.
func (r *Renderer) NewTextureFromImageWithOptions(img image.Image, opts TextureOptions) (*Texture, error) {
	// Convert to RGBA if needed
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var rgba *image.RGBA
	if r, ok := img.(*image.RGBA); ok {
		rgba = r
	} else {
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	return r.NewTextureFromRGBAWithOptions(width, height, rgba.Pix, opts)
}

// NewTextureFromRGBA creates a texture from raw RGBA pixel data.
// The data must be width * height * 4 bytes (RGBA8).
func (r *Renderer) NewTextureFromRGBA(width, height int, data []byte) (*Texture, error) {
	return r.NewTextureFromRGBAWithOptions(width, height, data, DefaultTextureOptions())
}

// NewTextureFromRGBAWithOptions creates a texture from raw RGBA pixel data with custom options.
func (r *Renderer) NewTextureFromRGBAWithOptions(width, height int, data []byte, opts TextureOptions) (*Texture, error) {
	expectedSize := width * height * 4
	if len(data) != expectedSize {
		return nil, fmt.Errorf("gogpu: invalid data size: expected %d bytes, got %d", expectedSize, len(data))
	}

	// Create GPU texture
	// Note: width/height validated above (expectedSize check ensures they are positive)
	texture, err := r.backend.CreateTexture(r.device, &types.TextureDescriptor{
		Label: opts.Label,
		Size: types.Extent3D{
			Width:              uint32(width),  //nolint:gosec // G115: width validated positive above
			Height:             uint32(height), //nolint:gosec // G115: height validated positive above
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatRGBA8Unorm,
		Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: failed to create texture: %w", err)
	}

	// Upload pixel data
	r.backend.WriteTexture(
		r.queue,
		&types.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   types.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   types.TextureAspectAll,
		},
		data,
		&types.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(width * 4), //nolint:gosec // G115: width validated positive above
			RowsPerImage: uint32(height),    //nolint:gosec // G115: height validated positive above
		},
		&types.Extent3D{
			Width:              uint32(width),  //nolint:gosec // G115: width validated positive above
			Height:             uint32(height), //nolint:gosec // G115: height validated positive above
			DepthOrArrayLayers: 1,
		},
	)

	// Create texture view
	view := r.backend.CreateTextureView(texture, nil)
	if view == 0 {
		r.backend.ReleaseTexture(texture)
		return nil, fmt.Errorf("gogpu: failed to create texture view")
	}

	// Create sampler
	sampler, err := r.backend.CreateSampler(r.device, &types.SamplerDescriptor{
		Label:        opts.Label,
		AddressModeU: opts.AddressModeU,
		AddressModeV: opts.AddressModeV,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    opts.MagFilter,
		MinFilter:    opts.MinFilter,
		MipmapFilter: types.MipmapFilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  32,
	})
	if err != nil {
		r.backend.ReleaseTextureView(view)
		r.backend.ReleaseTexture(texture)
		return nil, fmt.Errorf("gogpu: failed to create sampler: %w", err)
	}

	return &Texture{
		texture:  texture,
		view:     view,
		sampler:  sampler,
		width:    width,
		height:   height,
		format:   types.TextureFormatRGBA8Unorm,
		renderer: r,
	}, nil
}
