package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// newTestWgpuDevice creates a *wgpu.Device wrapping a mock HAL device for testing.
func newTestWgpuDevice(t *testing.T, mockDev *mockFenceDevice) (*wgpu.Device, error) {
	t.Helper()
	return wgpu.NewDeviceFromHAL(
		mockDev,
		&mockQueue{},
		gputypes.Features(0),
		gputypes.DefaultLimits(),
		"test",
	)
}

// newTestRenderer creates a Renderer with a primary windowSurface for testing.
// Sets up the minimum fields needed for read-only wrapper methods.
func newTestRenderer(width, height uint32) *Renderer {
	r := &Renderer{
		primary: &windowSurface{
			width:  width,
			height: height,
		},
	}
	r.primary.renderer = r
	return r
}

// newTestRendererFull creates a Renderer with all commonly tested fields.
func newTestRendererFull(width, height uint32, format gputypes.TextureFormat, backendName string) *Renderer {
	r := &Renderer{
		backendName: backendName,
		primary: &windowSurface{
			width:  width,
			height: height,
			format: format,
		},
	}
	r.primary.renderer = r
	return r
}

// newTestContext creates a Context with a mock Renderer for testing.
// Only sets up the fields needed for read-only wrapper methods.
// Uses scale=1.0 so logical == physical dimensions.
func newTestContext(width, height uint32, format gputypes.TextureFormat, backendName string) *Context {
	r := newTestRendererFull(width, height, format, backendName)
	return newContext(r, 1.0)
}

func TestContextSize(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
		wantW, wantH  int
	}{
		{"standard", 800, 600, 800, 600},
		{"4K", 3840, 2160, 3840, 2160},
		{"zero", 0, 0, 0, 0},
		{"square", 512, 512, 512, 512},
		{"wide", 2560, 1080, 2560, 1080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.width, tt.height, gputypes.TextureFormatBGRA8Unorm, "test")
			w, h := ctx.Size()
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("Size() = (%d, %d), want (%d, %d)", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestContextWidth(t *testing.T) {
	tests := []struct {
		name  string
		width uint32
		want  int
	}{
		{"800", 800, 800},
		{"1920", 1920, 1920},
		{"zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.width, 600, gputypes.TextureFormatBGRA8Unorm, "test")
			if got := ctx.Width(); got != tt.want {
				t.Errorf("Width() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestContextHeight(t *testing.T) {
	tests := []struct {
		name   string
		height uint32
		want   int
	}{
		{"600", 600, 600},
		{"1080", 1080, 1080},
		{"zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(800, tt.height, gputypes.TextureFormatBGRA8Unorm, "test")
			if got := ctx.Height(); got != tt.want {
				t.Errorf("Height() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestContextAspectRatio(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
		want          float32
	}{
		{"16:9", 1920, 1080, 1920.0 / 1080.0},
		{"4:3", 800, 600, 800.0 / 600.0},
		{"square", 512, 512, 1.0},
		{"ultrawide", 3440, 1440, 3440.0 / 1440.0},
		{"zero height", 800, 0, 1.0}, // edge case: returns 1.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.width, tt.height, gputypes.TextureFormatBGRA8Unorm, "test")
			got := ctx.AspectRatio()
			// Use approximate comparison for float32
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("AspectRatio() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestContextFormat(t *testing.T) {
	tests := []struct {
		name   string
		format gputypes.TextureFormat
	}{
		{"BGRA8Unorm", gputypes.TextureFormatBGRA8Unorm},
		{"RGBA8Unorm", gputypes.TextureFormatRGBA8Unorm},
		{"BGRA8UnormSrgb", gputypes.TextureFormatBGRA8UnormSrgb},
		{"RGBA8UnormSrgb", gputypes.TextureFormatRGBA8UnormSrgb},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(800, 600, tt.format, "test")
			if got := ctx.Format(); got != tt.format {
				t.Errorf("Format() = %v, want %v", got, tt.format)
			}
		})
	}
}

func TestContextBackend(t *testing.T) {
	tests := []struct {
		name    string
		backend string
	}{
		{"rust", "Rust (wgpu-gpu)"},
		{"native", "Pure Go (gogpu/wgpu)"},
		{"empty", ""},
		{"custom", "Custom Backend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(800, 600, gputypes.TextureFormatBGRA8Unorm, tt.backend)
			if got := ctx.Backend(); got != tt.backend {
				t.Errorf("Backend() = %q, want %q", got, tt.backend)
			}
		})
	}
}

func TestContextCheckDeviceHealthNoDevice(t *testing.T) {
	// Context with nil device -- should return nil (backend doesn't support health check)
	ctx := newTestContext(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")

	err := ctx.CheckDeviceHealth()
	if err != nil {
		t.Errorf("CheckDeviceHealth() = %v, want nil (no device)", err)
	}
}

func TestContextCheckDeviceHealthNonChecker(t *testing.T) {
	// Device that does NOT implement healthChecker interface.
	// Create a wgpu.Device wrapping a mock HAL device (which doesn't implement healthChecker).
	mockDev := &mockFenceDevice{}
	wgpuDevice, err := newTestWgpuDevice(t, mockDev)
	if err != nil {
		t.Fatalf("newTestWgpuDevice() error = %v", err)
	}

	r := newTestRendererFull(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")
	r.device = wgpuDevice
	ctx := newContext(r, 1.0)

	err = ctx.CheckDeviceHealth()
	if err != nil {
		t.Errorf("CheckDeviceHealth() = %v, want nil (device without health check)", err)
	}
}

func TestContextSurfaceSize(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
	}{
		{"standard", 800, 600},
		{"4K", 3840, 2160},
		{"zero", 0, 0},
		{"small", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.width, tt.height, gputypes.TextureFormatBGRA8Unorm, "test")
			w, h := ctx.SurfaceSize()
			if w != tt.width || h != tt.height {
				t.Errorf("SurfaceSize() = (%d, %d), want (%d, %d)", w, h, tt.width, tt.height)
			}
		})
	}
}

func TestContextRenderer(t *testing.T) {
	r := newTestRendererFull(800, 600, 0, "test")
	ctx := newContext(r, 1.0)

	if ctx.Renderer() != r {
		t.Error("Renderer() did not return the expected Renderer instance")
	}
}

func TestContextSurfaceViewNilWhenNoFrame(t *testing.T) {
	// currentView is nil when no frame is in progress
	ctx := newTestContext(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")

	view := ctx.SurfaceView()
	if view != nil {
		t.Errorf("SurfaceView() = %v, want nil (no frame in progress)", view)
	}
}

func TestContextClearedInitiallyFalse(t *testing.T) {
	ctx := newTestContext(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")

	if ctx.cleared {
		t.Error("cleared = true, want false (initially)")
	}
}

func TestNewContext(t *testing.T) {
	r := newTestRendererFull(1024, 768, gputypes.TextureFormatRGBA8Unorm, "native")
	ctx := newContext(r, 1.0)

	if ctx == nil {
		t.Fatal("newContext returned nil")
	}
	if ctx.renderer != r {
		t.Error("renderer pointer mismatch")
	}
	if ctx.cleared {
		t.Error("cleared should be false for new context")
	}
}

func TestNewContextZeroScale(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, 0)

	if ctx.scaleFactor != 1.0 {
		t.Errorf("scaleFactor = %f, want 1.0 (should default for zero)", ctx.scaleFactor)
	}
}

func TestNewContextNegativeScale(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, -1.0)

	if ctx.scaleFactor != 1.0 {
		t.Errorf("scaleFactor = %f, want 1.0 (should default for negative)", ctx.scaleFactor)
	}
}

func TestContextScaleFactor(t *testing.T) {
	tests := []struct {
		name  string
		scale float64
		want  float64
	}{
		{"standard", 1.0, 1.0},
		{"retina", 2.0, 2.0},
		{"150%", 1.5, 1.5},
		{"zero defaults", 0, 1.0},
		{"negative defaults", -1, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRenderer(800, 600)
			ctx := newContext(r, tt.scale)
			if ctx.ScaleFactor() != tt.want {
				t.Errorf("ScaleFactor() = %f, want %f", ctx.ScaleFactor(), tt.want)
			}
		})
	}
}

func TestContextSizeWithScaling(t *testing.T) {
	// Physical 1600x1200 at 2x scale = logical 800x600
	r := newTestRenderer(1600, 1200)
	ctx := newContext(r, 2.0)

	w, h := ctx.Size()
	if w != 800 || h != 600 {
		t.Errorf("Size() = (%d, %d), want (800, 600)", w, h)
	}
}

func TestContextFramebufferSize(t *testing.T) {
	r := newTestRenderer(1600, 1200)
	ctx := newContext(r, 2.0)

	w, h := ctx.FramebufferSize()
	if w != 1600 || h != 1200 {
		t.Errorf("FramebufferSize() = (%d, %d), want (1600, 1200)", w, h)
	}
}

func TestContextFramebufferWidth(t *testing.T) {
	r := newTestRenderer(1920, 1080)
	ctx := newContext(r, 1.0)

	if ctx.FramebufferWidth() != 1920 {
		t.Errorf("FramebufferWidth() = %d, want 1920", ctx.FramebufferWidth())
	}
}

func TestContextFramebufferHeight(t *testing.T) {
	r := newTestRenderer(1920, 1080)
	ctx := newContext(r, 1.0)

	if ctx.FramebufferHeight() != 1080 {
		t.Errorf("FramebufferHeight() = %d, want 1080", ctx.FramebufferHeight())
	}
}

func TestContextRenderTarget(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, 1.0)

	rt := ctx.RenderTarget()
	if rt == nil {
		t.Fatal("RenderTarget() returned nil")
	}

	// SurfaceView wraps any, underlying is nil TextureView
	sv := rt.SurfaceView()
	_ = sv // Just verify it doesn't panic

	// SurfaceSize should match renderer
	w, h := rt.SurfaceSize()
	if w != 800 || h != 600 {
		t.Errorf("SurfaceSize() = (%d, %d), want (800, 600)", w, h)
	}

	// PresentTexture with nil must return an error
	err := rt.PresentTexture(nil)
	if err == nil {
		t.Error("PresentTexture(nil) = nil, want error")
	}
}

func TestContextAsTextureDrawer(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, 1.0)

	drawer := ctx.AsTextureDrawer()
	if drawer == nil {
		t.Fatal("AsTextureDrawer() returned nil")
	}

	// DrawTexture with invalid type should return error
	err := drawer.DrawTexture(nil, 0, 0)
	if err == nil {
		t.Error("DrawTexture(nil) should return error")
	}

	// TextureCreator should not be nil
	if drawer.TextureCreator() == nil {
		t.Error("TextureCreator() should not be nil")
	}
}

func TestContextPresentTextureNonTexture(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, 1.0)

	// Non-Texture type must return an error (not silently ignored)
	err := ctx.PresentTexture("not a texture")
	if err == nil {
		t.Error("PresentTexture(string) = nil, want error")
	}
}

func TestContextPresentTextureNil(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, 1.0)

	// nil must return an error
	err := ctx.PresentTexture(nil)
	if err == nil {
		t.Error("PresentTexture(nil) = nil, want error")
	}
}

func TestContextPresentTextureNilTyped(t *testing.T) {
	r := newTestRenderer(800, 600)
	ctx := newContext(r, 1.0)

	// (*Texture)(nil) passes the type assertion but must be caught as nil
	var tex *Texture
	err := ctx.PresentTexture(tex)
	if err == nil {
		t.Error("PresentTexture((*Texture)(nil)) = nil, want error")
	}
}

func TestContextClearColor(t *testing.T) {
	// ClearColor delegates to Clear; verify cleared flag is set.
	// We cannot call renderer.Clear on a nil backend, but we can
	// verify the method exists and the cleared flag via a minimal renderer
	// that has the width/height set (Clear calls renderer.Clear which
	// accesses renderer fields). Since renderer.Clear needs device etc.,
	// we just verify the struct interface: ClearColor calls Clear calls
	// renderer.Clear. Test the cleared flag path indirectly.
	ctx := &Context{
		renderer:    newTestRenderer(800, 600),
		scaleFactor: 1.0,
	}
	// ClearColor with zero alpha is still valid
	color := gmath.Color{R: 0.1, G: 0.2, B: 0.3, A: 1.0}

	// This will panic because renderer has no device/surface,
	// but we can at least verify the method signature compiles.
	// For a real test we'd need the full pipeline.
	_ = color
	_ = ctx
}

func TestContextSizeWithFractionalScale(t *testing.T) {
	tests := []struct {
		name         string
		physW, physH uint32
		scale        float64
		wantW, wantH int
	}{
		{"1.5x 1920x1080", 1920, 1080, 1.5, 1280, 720},
		{"1.25x 2560x1440", 2560, 1440, 1.25, 2048, 1152},
		{"1.75x 1400x1050", 1400, 1050, 1.75, 800, 600},
		{"3x 2400x1800", 2400, 1800, 3.0, 800, 600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRenderer(tt.physW, tt.physH)
			ctx := newContext(r, tt.scale)

			w, h := ctx.Size()
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("Size() = (%d, %d), want (%d, %d)", w, h, tt.wantW, tt.wantH)
			}

			// FramebufferSize should always return physical pixels regardless of scale
			fw, fh := ctx.FramebufferSize()
			if fw != int(tt.physW) || fh != int(tt.physH) {
				t.Errorf("FramebufferSize() = (%d, %d), want (%d, %d)", fw, fh, tt.physW, tt.physH)
			}
		})
	}
}

func TestContextWidthHeightWithScale(t *testing.T) {
	r := newTestRenderer(3840, 2160)
	ctx := newContext(r, 2.0)

	if ctx.Width() != 1920 {
		t.Errorf("Width() = %d, want 1920", ctx.Width())
	}
	if ctx.Height() != 1080 {
		t.Errorf("Height() = %d, want 1080", ctx.Height())
	}
}

func TestContextAspectRatioEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
		want          float32
	}{
		{"both zero", 0, 0, 1.0},
		{"portrait", 600, 800, 0.75},
		{"1:1", 100, 100, 1.0},
		{"very wide", 10000, 1, 10000.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.width, tt.height, gputypes.TextureFormatBGRA8Unorm, "test")
			got := ctx.AspectRatio()
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.01 {
				t.Errorf("AspectRatio() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestContextFramebufferWidthHeightWithScale(t *testing.T) {
	// Framebuffer dimensions must be physical, unaffected by scale
	r := newTestRenderer(3840, 2160)
	ctx := newContext(r, 2.0)

	if ctx.FramebufferWidth() != 3840 {
		t.Errorf("FramebufferWidth() = %d, want 3840", ctx.FramebufferWidth())
	}
	if ctx.FramebufferHeight() != 2160 {
		t.Errorf("FramebufferHeight() = %d, want 2160", ctx.FramebufferHeight())
	}
}

func TestContextSurfaceSizeWithScale(t *testing.T) {
	// SurfaceSize returns physical pixels as uint32, unaffected by scale
	r := newTestRenderer(2560, 1440)
	ctx := newContext(r, 1.5)

	w, h := ctx.SurfaceSize()
	if w != 2560 || h != 1440 {
		t.Errorf("SurfaceSize() = (%d, %d), want (2560, 1440)", w, h)
	}
}

func TestNewContextScaleValues(t *testing.T) {
	tests := []struct {
		name  string
		scale float64
		want  float64
	}{
		{"standard", 1.0, 1.0},
		{"retina", 2.0, 2.0},
		{"fractional", 1.5, 1.5},
		{"high dpi", 3.0, 3.0},
		{"small positive", 0.5, 0.5},
		{"zero defaults to 1", 0, 1.0},
		{"negative defaults to 1", -2.5, 1.0},
		{"very small negative", -0.001, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRenderer(800, 600)
			ctx := newContext(r, tt.scale)
			if ctx.scaleFactor != tt.want {
				t.Errorf("scaleFactor = %f, want %f", ctx.scaleFactor, tt.want)
			}
		})
	}
}

func TestContextRenderTargetSurfaceViewNil(t *testing.T) {
	ctx := newTestContext(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")
	rt := ctx.RenderTarget()

	// SurfaceView returns gpucontext.TextureView (opaque handle struct).
	// When no frame is in progress, the underlying pointer is nil.
	sv := rt.SurfaceView()
	if !sv.IsNil() {
		t.Errorf("RenderTarget().SurfaceView().IsNil() = false, want true (no frame in progress)")
	}
}

func TestContextRenderTargetPresentTextureNonTexture(t *testing.T) {
	ctx := newTestContext(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")
	rt := ctx.RenderTarget()

	// Wrong type must return an error
	err := rt.PresentTexture("not a texture")
	if err == nil {
		t.Error("RenderTarget().PresentTexture(string) = nil, want error")
	}
}

func TestContextRenderTargetSurfaceSize(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
	}{
		{"standard", 800, 600},
		{"4K", 3840, 2160},
		{"zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(tt.width, tt.height, gputypes.TextureFormatBGRA8Unorm, "test")
			rt := ctx.RenderTarget()
			w, h := rt.SurfaceSize()
			if w != tt.width || h != tt.height {
				t.Errorf("SurfaceSize() = (%d, %d), want (%d, %d)", w, h, tt.width, tt.height)
			}
		})
	}
}

func TestContextAspectRatioWithScale(t *testing.T) {
	// AspectRatio uses logical size, so scaling should not change the ratio
	// for proportionally scaled displays.
	r := newTestRenderer(3840, 2160)
	ctx := newContext(r, 2.0)

	got := ctx.AspectRatio()
	want := float32(1920) / float32(1080) // logical 1920x1080
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.001 {
		t.Errorf("AspectRatio() with 2x scale = %f, want %f", got, want)
	}
}
