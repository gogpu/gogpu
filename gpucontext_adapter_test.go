package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// TestGPUContextAdapterInterface verifies the gpuContextAdapter implements gpucontext.DeviceProvider.
func TestGPUContextAdapterInterface(t *testing.T) {
	var _ gpucontext.DeviceProvider = (*gpuContextAdapter)(nil)
}

// TestGPUContextProviderNilBeforeRun verifies GPUContextProvider returns nil before Run().
func TestGPUContextProviderNilBeforeRun(t *testing.T) {
	app := NewApp(DefaultConfig())

	provider := app.GPUContextProvider()
	if provider != nil {
		t.Error("GPUContextProvider should return nil before Run() is called")
	}
}

// TestGPUContextAdapterMethods tests the methods of gpuContextAdapter.
func TestGPUContextAdapterMethods(t *testing.T) {
	// Create a renderer with nil wgpu objects (no actual GPU needed)
	renderer := &Renderer{
		adapter: nil,
		device:  nil,
		format:  gputypes.TextureFormatBGRA8Unorm,
	}

	adapter := &gpuContextAdapter{renderer: renderer}

	t.Run("Device", func(t *testing.T) {
		// Device returns nil when renderer.device is nil.
		device := adapter.Device()
		if device != nil {
			t.Error("Device() should return nil when renderer.device is nil")
		}
	})

	t.Run("Queue", func(t *testing.T) {
		// Queue returns nil when renderer.device is nil.
		queue := adapter.Queue()
		if queue != nil {
			t.Error("Queue() should return nil when renderer.device is nil")
		}
	})

	t.Run("Adapter", func(t *testing.T) {
		// Adapter returns nil when renderer.adapter is nil.
		adpt := adapter.Adapter()
		if adpt != nil {
			t.Error("Adapter() should return nil when renderer.adapter is nil")
		}
	})

	t.Run("SurfaceFormat", func(t *testing.T) {
		format := adapter.SurfaceFormat()
		if format != gputypes.TextureFormatBGRA8Unorm {
			t.Errorf("SurfaceFormat() = %v, want %v", format, gputypes.TextureFormatBGRA8Unorm)
		}
	})
}

// TestGPUContextAdapterNilRenderer tests methods with nil renderer.
func TestGPUContextAdapterNilRenderer(t *testing.T) {
	adapter := &gpuContextAdapter{renderer: nil}

	t.Run("Device", func(t *testing.T) {
		if adapter.Device() != nil {
			t.Error("Device() should return nil with nil renderer")
		}
	})

	t.Run("Queue", func(t *testing.T) {
		if adapter.Queue() != nil {
			t.Error("Queue() should return nil with nil renderer")
		}
	})

	t.Run("Adapter", func(t *testing.T) {
		if adapter.Adapter() != nil {
			t.Error("Adapter() should return nil with nil renderer")
		}
	})

	t.Run("SurfaceFormat", func(t *testing.T) {
		if adapter.SurfaceFormat() != gputypes.TextureFormatUndefined {
			t.Errorf("SurfaceFormat() should return Undefined with nil renderer")
		}
	})
}

// TestMapTextureFormat tests texture format conversion.
func TestMapTextureFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  gputypes.TextureFormat
		output gputypes.TextureFormat
	}{
		{"RGBA8Unorm", gputypes.TextureFormatRGBA8Unorm, gputypes.TextureFormatRGBA8Unorm},
		{"BGRA8Unorm", gputypes.TextureFormatBGRA8Unorm, gputypes.TextureFormatBGRA8Unorm},
		{"Unknown", gputypes.TextureFormat(0x99), gputypes.TextureFormatUndefined},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapTextureFormat(tt.input)
			if result != tt.output {
				t.Errorf("mapTextureFormat(%v) = %v, want %v", tt.input, result, tt.output)
			}
		})
	}
}
