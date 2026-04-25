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
	"runtime"
	"unsafe"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// textureCleanupHandle holds the data needed to destroy a texture's GPU
// resources from a runtime.AddCleanup callback. The handle stores wgpu
// pointers so the cleanup can run after the Texture is garbage collected.
type textureCleanupHandle struct {
	texture  *wgpu.Texture
	view     *wgpu.TextureView
	sampler  *wgpu.Sampler
	renderer *Renderer
}

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
	// GPU resources (wgpu public API types)
	texture *wgpu.Texture
	view    *wgpu.TextureView
	sampler *wgpu.Sampler

	// Metadata
	width         int
	height        int
	format        gputypes.TextureFormat
	premultiplied bool // true if pixel data uses premultiplied alpha

	// Reference to renderer for resource management
	renderer *Renderer

	// cleanup is the handle returned by runtime.AddCleanup.
	// Calling Stop() prevents the GC cleanup from running after explicit Destroy.
	cleanup runtime.Cleanup
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
func (t *Texture) SetPremultiplied(premultiplied bool) {
	t.premultiplied = premultiplied
}

// Format returns the texture format.
func (t *Texture) Format() gputypes.TextureFormat {
	return t.format
}

// Handle returns the underlying wgpu texture.
// For advanced use cases that need direct GPU access.
func (t *Texture) Handle() *wgpu.Texture {
	return t.texture
}

// View returns the texture view.
func (t *Texture) View() *wgpu.TextureView {
	return t.view
}

// TextureView returns the texture view as gpucontext.TextureView opaque handle.
// This enables duck-typed access from packages that cannot import gogpu
// (e.g., gg/ggcanvas uses structural typing to call this method).
func (t *Texture) TextureView() gpucontext.TextureView {
	if t.view == nil {
		return gpucontext.TextureView{}
	}
	return gpucontext.NewTextureView(unsafe.Pointer(t.view)) //nolint:gosec // Go spec Rule 1: *T → unsafe.Pointer (ADR-018 opaque handle)
}

// Sampler returns the sampler.
func (t *Texture) Sampler() *wgpu.Sampler {
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
		return 0
	}
}

// Destroy releases all GPU resources associated with this texture.
// After calling Destroy, the texture should not be used.
func (t *Texture) Destroy() {
	// Stop the GC cleanup -- we are destroying explicitly.
	t.cleanup.Stop()

	if t.renderer == nil || t.renderer.device == nil {
		return
	}

	// Evict from bind group cache before destroying the view.
	if t.view != nil && t.renderer.texBindGroupCache != nil {
		if bg, ok := t.renderer.texBindGroupCache[t.view]; ok {
			bg.Release()
			delete(t.renderer.texBindGroupCache, t.view)
		}
	}

	if t.sampler != nil {
		t.sampler.Release()
		t.sampler = nil
	}
	if t.view != nil {
		t.view.Release()
		t.view = nil
	}
	if t.texture != nil {
		t.texture.Release()
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
func (r *Renderer) NewTextureFromImageWithOptions(img image.Image, opts TextureOptions) (*Texture, error) {
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

	opts.Premultiplied = true
	return r.NewTextureFromRGBAWithOptions(width, height, rgba.Pix, opts)
}

// NewTextureFromRGBA creates a texture from raw RGBA pixel data.
func (r *Renderer) NewTextureFromRGBA(width, height int, data []byte) (*Texture, error) {
	return r.NewTextureFromRGBAWithOptions(width, height, data, DefaultTextureOptions())
}

// NewTextureFromRGBAWithOptions creates a texture from raw RGBA pixel data with custom options.
func (r *Renderer) NewTextureFromRGBAWithOptions(width, height int, data []byte, opts TextureOptions) (*Texture, error) {
	expectedSize := width * height * 4
	if len(data) != expectedSize {
		return nil, fmt.Errorf("gogpu: invalid data size: expected %d bytes, got %d", expectedSize, len(data))
	}

	// Create GPU texture via wgpu Device
	texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
		Label: opts.Label,
		Size: wgpu.Extent3D{
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

	// Upload pixel data via wgpu Queue
	if err := r.device.Queue().WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		data,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(width * 4), //nolint:gosec // G115: width validated positive above
			RowsPerImage: uint32(height),    //nolint:gosec // G115: height validated positive above
		},
		&wgpu.Extent3D{
			Width:              uint32(width),  //nolint:gosec // G115: width validated positive above
			Height:             uint32(height), //nolint:gosec // G115: height validated positive above
			DepthOrArrayLayers: 1,
		},
	); err != nil {
		texture.Release()
		return nil, fmt.Errorf("gogpu: failed to upload texture data: %w", err)
	}

	// Create texture view
	view, err := r.device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("gogpu: failed to create texture view: %w", err)
	}

	// Create sampler
	sampler, err := r.device.CreateSampler(&wgpu.SamplerDescriptor{
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
		view.Release()
		texture.Release()
		return nil, fmt.Errorf("gogpu: failed to create sampler: %w", err)
	}

	tex := &Texture{
		texture:       texture,
		view:          view,
		sampler:       sampler,
		width:         width,
		height:        height,
		format:        gputypes.TextureFormatRGBA8Unorm,
		premultiplied: opts.Premultiplied,
		renderer:      r,
	}

	// Safety net: if the texture is garbage collected without Destroy(),
	// enqueue deferred destruction on the render thread.
	handle := textureCleanupHandle{
		texture:  texture,
		view:     view,
		sampler:  sampler,
		renderer: r,
	}
	tex.cleanup = runtime.AddCleanup(tex, func(h textureCleanupHandle) {
		h.renderer.EnqueueDeferredDestroy(func() {
			if h.sampler != nil {
				h.sampler.Release()
			}
			if h.view != nil {
				h.view.Release()
			}
			if h.texture != nil {
				h.texture.Release()
			}
		})
	}, handle)

	return tex, nil
}

// UpdateData uploads new pixel data to the entire texture.
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

	if err := t.renderer.device.Queue().WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  t.texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		data,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(t.width * bpp), //nolint:gosec // G115: width validated in constructor
			RowsPerImage: uint32(t.height),      //nolint:gosec // G115: height validated in constructor
		},
		&wgpu.Extent3D{
			Width:              uint32(t.width),  //nolint:gosec // G115: width validated in constructor
			Height:             uint32(t.height), //nolint:gosec // G115: height validated in constructor
			DepthOrArrayLayers: 1,
		},
	); err != nil {
		return fmt.Errorf("gogpu: failed to upload texture data: %w", err)
	}

	return nil
}

// UpdateRegion uploads pixel data to a rectangular region of the texture.
func (t *Texture) UpdateRegion(x, y, w, h int, data []byte) error {
	if t.renderer == nil || t.renderer.device == nil || t.texture == nil {
		return ErrTextureUpdateDestroyed
	}

	if x < 0 || y < 0 || w <= 0 || h <= 0 {
		return fmt.Errorf("%w: x=%d, y=%d, w=%d, h=%d (x,y must be non-negative; w,h must be positive)",
			ErrInvalidRegion, x, y, w, h)
	}

	if x+w > t.width || y+h > t.height {
		return fmt.Errorf("%w: region (%d,%d)+(%d,%d) exceeds texture size (%d,%d)",
			ErrRegionOutOfBounds, x, y, w, h, t.width, t.height)
	}

	bpp := t.BytesPerPixel()
	if bpp == 0 {
		return fmt.Errorf("%w: unsupported texture format", ErrInvalidDataSize)
	}

	expectedSize := w * h * bpp
	if len(data) != expectedSize {
		return fmt.Errorf("%w: expected %d bytes (%dx%dx%d), got %d",
			ErrInvalidDataSize, expectedSize, w, h, bpp, len(data))
	}

	if err := t.renderer.device.Queue().WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  t.texture,
			MipLevel: 0,
			Origin: wgpu.Origin3D{
				X: uint32(x), //nolint:gosec // G115: x validated non-negative above
				Y: uint32(y), //nolint:gosec // G115: y validated non-negative above
				Z: 0,
			},
			Aspect: gputypes.TextureAspectAll,
		},
		data,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(w * bpp), //nolint:gosec // G115: w validated positive above
			RowsPerImage: uint32(h),       //nolint:gosec // G115: h validated positive above
		},
		&wgpu.Extent3D{
			Width:              uint32(w), //nolint:gosec // G115: w validated positive above
			Height:             uint32(h), //nolint:gosec // G115: h validated positive above
			DepthOrArrayLayers: 1,
		},
	); err != nil {
		return fmt.Errorf("gogpu: failed to upload texture region: %w", err)
	}

	return nil
}
