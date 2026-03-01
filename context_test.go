package gogpu

import (
	"testing"

	"github.com/gogpu/gputypes"
)

// newTestContext creates a Context with a mock Renderer for testing.
// Only sets up the fields needed for read-only wrapper methods.
func newTestContext(width, height uint32, format gputypes.TextureFormat, backendName string) *Context {
	r := &Renderer{
		width:       width,
		height:      height,
		format:      format,
		backendName: backendName,
	}
	return newContext(r)
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
	// Device that does NOT implement healthChecker interface
	r := &Renderer{
		width:       800,
		height:      600,
		format:      gputypes.TextureFormatBGRA8Unorm,
		backendName: "test",
		device:      &mockFenceDevice{}, // does not implement healthChecker
	}
	ctx := newContext(r)

	err := ctx.CheckDeviceHealth()
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
	r := &Renderer{
		width:       800,
		height:      600,
		backendName: "test",
	}
	ctx := newContext(r)

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
	r := &Renderer{
		width:       1024,
		height:      768,
		format:      gputypes.TextureFormatRGBA8Unorm,
		backendName: "native",
	}
	ctx := newContext(r)

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
