package ggrender

import (
	"github.com/gogpu/gg"
	"github.com/gogpu/gogpu"
)

// Renderer implements gg.Renderer using GPU acceleration via gogpu.
//
// The renderer uses gogpu's DeviceProvider to access GPU resources
// for hardware-accelerated path rendering.
type Renderer struct {
	provider gogpu.DeviceProvider

	// Fallback software renderer (used until GPU pipeline is ready)
	software *gg.SoftwareRenderer

	// Dimensions
	width  int
	height int
}

// New creates a new GPU-accelerated renderer.
//
// The provider parameter supplies GPU resources (device, queue, etc.)
// from a gogpu.App instance.
//
// Example:
//
//	app := gogpu.NewApp(config)
//	// ... in OnDraw callback:
//	gpuRenderer := ggrender.New(app.DeviceProvider())
//	dc := gg.NewContext(800, 600, gg.WithRenderer(gpuRenderer))
func New(provider gogpu.DeviceProvider) *Renderer {
	return &Renderer{
		provider: provider,
	}
}

// NewWithSize creates a new GPU-accelerated renderer with specific dimensions.
//
// This is useful when you know the rendering dimensions upfront.
func NewWithSize(provider gogpu.DeviceProvider, width, height int) *Renderer {
	return &Renderer{
		provider: provider,
		width:    width,
		height:   height,
		software: gg.NewSoftwareRenderer(width, height),
	}
}

// Fill fills a path with the given paint using GPU acceleration.
//
// Current implementation: Uses software rendering as fallback.
// Future: GPU compute shader path tessellation and rasterization.
func (r *Renderer) Fill(pixmap *gg.Pixmap, path *gg.Path, paint *gg.Paint) error {
	// Ensure software renderer is initialized
	r.ensureSoftware(pixmap.Width(), pixmap.Height())

	// TODO: Implement GPU path tessellation
	// 1. Flatten path to line segments (GPU compute)
	// 2. Tile binning (GPU compute)
	// 3. Fine rasterization (GPU compute)
	// 4. Composite to pixmap (GPU or CPU readback)

	// For now, delegate to software renderer
	return r.software.Fill(pixmap, path, paint)
}

// Stroke strokes a path with the given paint using GPU acceleration.
//
// Current implementation: Uses software rendering as fallback.
// Future: GPU stroke expansion and rasterization.
func (r *Renderer) Stroke(pixmap *gg.Pixmap, path *gg.Path, paint *gg.Paint) error {
	// Ensure software renderer is initialized
	r.ensureSoftware(pixmap.Width(), pixmap.Height())

	// TODO: Implement GPU stroke expansion
	// 1. Expand stroke to filled path (GPU or CPU)
	// 2. Use Fill pipeline for rasterization

	// For now, delegate to software renderer
	return r.software.Stroke(pixmap, path, paint)
}

// ensureSoftware initializes the software renderer if needed.
func (r *Renderer) ensureSoftware(width, height int) {
	if r.software == nil || r.width != width || r.height != height {
		r.width = width
		r.height = height
		r.software = gg.NewSoftwareRenderer(width, height)
	}
}

// Provider returns the underlying DeviceProvider.
// This can be used for advanced GPU operations.
func (r *Renderer) Provider() gogpu.DeviceProvider {
	return r.provider
}

// Ensure Renderer implements gg.Renderer.
var _ gg.Renderer = (*Renderer)(nil)
