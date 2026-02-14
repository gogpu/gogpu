package gogpu

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"io"
	"os"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// Texture update errors.
var (
	// ErrTextureUpdateDestroyed is returned when attempting to update a destroyed texture.
	ErrTextureUpdateDestroyed = errors.New("gogpu: cannot update destroyed texture")

	// ErrInvalidDataSize is returned when the data size doesn't match expected dimensions.
	ErrInvalidDataSize = errors.New("gogpu: invalid data size")

	// ErrRegionOutOfBounds is returned when the update region exceeds texture bounds.
	ErrRegionOutOfBounds = errors.New("gogpu: region out of bounds")

	// ErrInvalidRegion is returned when region parameters are invalid (negative or zero).
	ErrInvalidRegion = errors.New("gogpu: invalid region parameters")
)

// Texture represents a GPU texture resource with its associated view and sampler.
// It provides a high-level interface for working with textures in GoGPU.
type Texture struct {
	// GPU resources (HAL interfaces)
	texture hal.Texture
	view    hal.TextureView
	sampler hal.Sampler

	// Metadata
	width         int
	height        int
	format        gputypes.TextureFormat
	premultiplied bool // true if pixel data uses premultiplied alpha

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

// Premultiplied returns true if the texture data uses premultiplied alpha.
func (t *Texture) Premultiplied() bool {
	return t.premultiplied
}

// SetPremultiplied marks the texture as containing premultiplied alpha data.
// This controls the shader behavior: premultiplied textures scale all channels
// uniformly, while straight alpha textures are premultiplied in the shader.
func (t *Texture) SetPremultiplied(premultiplied bool) {
	t.premultiplied = premultiplied
}

// Format returns the texture format.
func (t *Texture) Format() gputypes.TextureFormat {
	return t.format
}

// Handle returns the underlying HAL texture.
// For advanced use cases that need direct GPU access.
func (t *Texture) Handle() hal.Texture {
	return t.texture
}

// View returns the texture view.
func (t *Texture) View() hal.TextureView {
	return t.view
}

// Sampler returns the sampler.
func (t *Texture) Sampler() hal.Sampler {
	return t.sampler
}

// BytesPerPixel returns the number of bytes per pixel for the texture format.
// Returns 4 for RGBA8/BGRA8 formats (the most common), 0 for unknown formats.
func (t *Texture) BytesPerPixel() int {
	return bytesPerPixelForFormat(t.format)
}

// bytesPerPixelForFormat returns bytes per pixel for a given texture format.
func bytesPerPixelForFormat(format gputypes.TextureFormat) int {
	switch format {
	case gputypes.TextureFormatRGBA8Unorm,
		gputypes.TextureFormatRGBA8UnormSrgb,
		gputypes.TextureFormatRGBA8Snorm,
		gputypes.TextureFormatRGBA8Uint,
		gputypes.TextureFormatRGBA8Sint,
		gputypes.TextureFormatBGRA8Unorm,
		gputypes.TextureFormatBGRA8UnormSrgb:
		return 4
	case gputypes.TextureFormatR8Unorm,
		gputypes.TextureFormatR8Snorm,
		gputypes.TextureFormatR8Uint,
		gputypes.TextureFormatR8Sint:
		return 1
	case gputypes.TextureFormatR16Uint,
		gputypes.TextureFormatR16Sint,
		gputypes.TextureFormatR16Float,
		gputypes.TextureFormatRG8Unorm,
		gputypes.TextureFormatRG8Snorm,
		gputypes.TextureFormatRG8Uint,
		gputypes.TextureFormatRG8Sint:
		return 2
	case gputypes.TextureFormatRG16Uint,
		gputypes.TextureFormatRG16Sint,
		gputypes.TextureFormatRG16Float,
		gputypes.TextureFormatRGBA16Uint,
		gputypes.TextureFormatRGBA16Sint,
		gputypes.TextureFormatRGBA16Float:
		return 8
	case gputypes.TextureFormatR32Uint,
		gputypes.TextureFormatR32Sint,
		gputypes.TextureFormatR32Float:
		return 4
	case gputypes.TextureFormatRG32Uint,
		gputypes.TextureFormatRG32Sint,
		gputypes.TextureFormatRG32Float:
		return 8
	case gputypes.TextureFormatRGBA32Uint,
		gputypes.TextureFormatRGBA32Sint,
		gputypes.TextureFormatRGBA32Float:
		return 16
	default:
		// Unknown format, return 0 (caller should handle)
		return 0
	}
}

// Destroy releases all GPU resources associated with this texture.
// After calling Destroy, the texture should not be used.
func (t *Texture) Destroy() {
	if t.renderer == nil || t.renderer.device == nil {
		return
	}

	// Evict from bind group cache before destroying the view.
	// The cached bind group references this texture's view and sampler,
	// so it must be destroyed before we destroy those resources.
	if t.view != nil && t.renderer.texBindGroupCache != nil {
		if bg, ok := t.renderer.texBindGroupCache[t.view]; ok {
			t.renderer.device.DestroyBindGroup(bg)
			delete(t.renderer.texBindGroupCache, t.view)
		}
	}

	if t.sampler != nil {
		t.renderer.device.DestroySampler(t.sampler)
		t.sampler = nil
	}
	if t.view != nil {
		t.renderer.device.DestroyTextureView(t.view)
		t.view = nil
	}
	if t.texture != nil {
		t.renderer.device.DestroyTexture(t.texture)
		t.texture = nil
	}
}

// TextureOptions configures texture creation.
type TextureOptions struct {
	// Label for debugging (optional)
	Label string

	// Filter mode for magnification (default: Linear)
	MagFilter gputypes.FilterMode

	// Filter mode for minification (default: Linear)
	MinFilter gputypes.FilterMode

	// Address mode for U coordinate (default: ClampToEdge)
	AddressModeU gputypes.AddressMode

	// Address mode for V coordinate (default: ClampToEdge)
	AddressModeV gputypes.AddressMode

	// Premultiplied indicates the texture data uses premultiplied alpha.
	// Controls shader behavior: premultiplied data is scaled uniformly,
	// straight alpha data is premultiplied in the shader before blending.
	Premultiplied bool
}

// DefaultTextureOptions returns sensible defaults for texture creation.
func DefaultTextureOptions() TextureOptions {
	return TextureOptions{
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
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
// The resulting texture is always marked as premultiplied because Go's image.RGBA
// stores premultiplied alpha data, and draw.Draw preserves this convention.
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

	// Go's image.RGBA stores premultiplied alpha by specification.
	// draw.Draw with draw.Src converts any source format to premultiplied.
	opts.Premultiplied = true
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

	// Create GPU texture via HAL device
	// Note: width/height validated above (expectedSize check ensures they are positive)
	texture, err := r.device.CreateTexture(&hal.TextureDescriptor{
		Label: opts.Label,
		Size: hal.Extent3D{
			Width:              uint32(width),  //nolint:gosec // G115: width validated positive above
			Height:             uint32(height), //nolint:gosec // G115: height validated positive above
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: failed to create texture: %w", err)
	}

	// Upload pixel data via HAL queue
	r.queue.WriteTexture(
		&hal.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   hal.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		data,
		&hal.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(width * 4), //nolint:gosec // G115: width validated positive above
			RowsPerImage: uint32(height),    //nolint:gosec // G115: height validated positive above
		},
		&hal.Extent3D{
			Width:              uint32(width),  //nolint:gosec // G115: width validated positive above
			Height:             uint32(height), //nolint:gosec // G115: height validated positive above
			DepthOrArrayLayers: 1,
		},
	)

	// Create texture view via HAL device
	view, err := r.device.CreateTextureView(texture, nil)
	if err != nil {
		r.device.DestroyTexture(texture)
		return nil, fmt.Errorf("gogpu: failed to create texture view: %w", err)
	}

	// Create sampler via HAL device
	sampler, err := r.device.CreateSampler(&hal.SamplerDescriptor{
		Label:        opts.Label,
		AddressModeU: opts.AddressModeU,
		AddressModeV: opts.AddressModeV,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    opts.MagFilter,
		MinFilter:    opts.MinFilter,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  32,
	})
	if err != nil {
		r.device.DestroyTextureView(view)
		r.device.DestroyTexture(texture)
		return nil, fmt.Errorf("gogpu: failed to create sampler: %w", err)
	}

	return &Texture{
		texture:       texture,
		view:          view,
		sampler:       sampler,
		width:         width,
		height:        height,
		format:        gputypes.TextureFormatRGBA8Unorm,
		premultiplied: opts.Premultiplied,
		renderer:      r,
	}, nil
}

// UpdateData uploads new pixel data to the entire texture.
// Data must be exactly width * height * bytesPerPixel bytes.
// This is more efficient than recreating the texture for dynamic content
// such as gg canvas rendering or video frames.
//
// Returns ErrTextureUpdateDestroyed if the texture has been destroyed.
// Returns ErrInvalidDataSize if the data size doesn't match texture dimensions.
func (t *Texture) UpdateData(data []byte) error {
	if t.renderer == nil || t.renderer.device == nil || t.texture == nil {
		return ErrTextureUpdateDestroyed
	}

	bpp := t.BytesPerPixel()
	if bpp == 0 {
		return fmt.Errorf("%w: unsupported texture format", ErrInvalidDataSize)
	}

	expectedSize := t.width * t.height * bpp
	if len(data) != expectedSize {
		return fmt.Errorf("%w: expected %d bytes (%dx%dx%d), got %d",
			ErrInvalidDataSize, expectedSize, t.width, t.height, bpp, len(data))
	}

	t.renderer.queue.WriteTexture(
		&hal.ImageCopyTexture{
			Texture:  t.texture,
			MipLevel: 0,
			Origin:   hal.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		data,
		&hal.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(t.width * bpp), //nolint:gosec // G115: width validated in constructor
			RowsPerImage: uint32(t.height),      //nolint:gosec // G115: height validated in constructor
		},
		&hal.Extent3D{
			Width:              uint32(t.width),  //nolint:gosec // G115: width validated in constructor
			Height:             uint32(t.height), //nolint:gosec // G115: height validated in constructor
			DepthOrArrayLayers: 1,
		},
	)

	return nil
}

// UpdateRegion uploads pixel data to a rectangular region of the texture.
// This is optimal for partial updates (dirty rectangles) where only a portion
// of the texture content has changed.
//
// Parameters:
//   - x, y: Top-left corner of the region in pixels (0,0 is top-left of texture)
//   - w, h: Width and height of the region in pixels
//   - data: Pixel data, must be exactly w * h * bytesPerPixel bytes
//
// The region must be within texture bounds.
//
// Returns ErrTextureUpdateDestroyed if the texture has been destroyed.
// Returns ErrInvalidRegion if x, y are negative or w, h are not positive.
// Returns ErrRegionOutOfBounds if the region exceeds texture dimensions.
// Returns ErrInvalidDataSize if the data size doesn't match region dimensions.
func (t *Texture) UpdateRegion(x, y, w, h int, data []byte) error {
	if t.renderer == nil || t.renderer.device == nil || t.texture == nil {
		return ErrTextureUpdateDestroyed
	}

	// Validate region parameters
	if x < 0 || y < 0 || w <= 0 || h <= 0 {
		return fmt.Errorf("%w: x=%d, y=%d, w=%d, h=%d (x,y must be non-negative; w,h must be positive)",
			ErrInvalidRegion, x, y, w, h)
	}

	// Validate region bounds
	if x+w > t.width || y+h > t.height {
		return fmt.Errorf("%w: region (%d,%d)+(%d,%d) exceeds texture size (%d,%d)",
			ErrRegionOutOfBounds, x, y, w, h, t.width, t.height)
	}

	bpp := t.BytesPerPixel()
	if bpp == 0 {
		return fmt.Errorf("%w: unsupported texture format", ErrInvalidDataSize)
	}

	// Validate data size
	expectedSize := w * h * bpp
	if len(data) != expectedSize {
		return fmt.Errorf("%w: expected %d bytes (%dx%dx%d), got %d",
			ErrInvalidDataSize, expectedSize, w, h, bpp, len(data))
	}

	t.renderer.queue.WriteTexture(
		&hal.ImageCopyTexture{
			Texture:  t.texture,
			MipLevel: 0,
			Origin: hal.Origin3D{
				X: uint32(x), //nolint:gosec // G115: x validated non-negative above
				Y: uint32(y), //nolint:gosec // G115: y validated non-negative above
				Z: 0,
			},
			Aspect: gputypes.TextureAspectAll,
		},
		data,
		&hal.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(w * bpp), //nolint:gosec // G115: w validated positive above
			RowsPerImage: uint32(h),       //nolint:gosec // G115: h validated positive above
		},
		&hal.Extent3D{
			Width:              uint32(w), //nolint:gosec // G115: w validated positive above
			Height:             uint32(h), //nolint:gosec // G115: h validated positive above
			DepthOrArrayLayers: 1,
		},
	)

	return nil
}
