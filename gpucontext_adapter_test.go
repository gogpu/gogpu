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
	// Create a renderer with nil wgpu objects (no actual GPU needed).
	// Surface format is stored on the primary windowSurface.
	renderer := newTestRendererFull(800, 600, gputypes.TextureFormatBGRA8Unorm, "test")
	renderer.adapter = nil
	renderer.device = nil

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

// TestGPUContextAdapterWindowProvider tests WindowProvider delegation.
func TestGPUContextAdapterWindowProvider(t *testing.T) {
	t.Run("Size with app", func(t *testing.T) {
		mock := &mockWindow{width: 1280, height: 720, scaleFactor: 1.0}
		app := &App{platWindow: mock}
		adapter := &gpuContextAdapter{app: app}

		w, h := adapter.Size()
		if w != 1280 || h != 720 {
			t.Errorf("Size() = (%d, %d), want (1280, 720)", w, h)
		}
	})

	t.Run("Size without app", func(t *testing.T) {
		adapter := &gpuContextAdapter{}

		w, h := adapter.Size()
		if w != 0 || h != 0 {
			t.Errorf("Size() = (%d, %d), want (0, 0)", w, h)
		}
	})

	t.Run("ScaleFactor with app", func(t *testing.T) {
		mock := &mockWindow{scaleFactor: 2.0}
		app := &App{platWindow: mock}
		adapter := &gpuContextAdapter{app: app}

		sf := adapter.ScaleFactor()
		if sf != 2.0 {
			t.Errorf("ScaleFactor() = %f, want 2.0", sf)
		}
	})

	t.Run("ScaleFactor without app", func(t *testing.T) {
		adapter := &gpuContextAdapter{}

		sf := adapter.ScaleFactor()
		if sf != 1.0 {
			t.Errorf("ScaleFactor() = %f, want 1.0", sf)
		}
	})

	t.Run("RequestRedraw with app", func(t *testing.T) {
		wokenUp := false
		app := &App{
			invalidator: newInvalidator(func() { wokenUp = true }),
		}
		adapter := &gpuContextAdapter{app: app}

		adapter.RequestRedraw()

		if !wokenUp {
			t.Error("RequestRedraw should trigger wakeup")
		}
	})

	t.Run("RequestRedraw without app", func(t *testing.T) {
		adapter := &gpuContextAdapter{}
		// Should not panic
		adapter.RequestRedraw()
	})
}

// TestGPUContextAdapterResourceTracker tests resource tracking via adapter.
func TestGPUContextAdapterResourceTracker(t *testing.T) {
	t.Run("TrackResource with tracker", func(t *testing.T) {
		tracker := &resourceTracker{}
		adapter := &gpuContextAdapter{tracker: tracker}

		m := newMockCloser("test")
		adapter.TrackResource(m)

		err := tracker.CloseAll()
		if err != nil {
			t.Fatalf("CloseAll error: %v", err)
		}
		if !m.closed {
			t.Error("Resource should have been closed via tracker")
		}
	})

	t.Run("TrackResource nil tracker", func(t *testing.T) {
		adapter := &gpuContextAdapter{}
		// Should not panic
		adapter.TrackResource(newMockCloser("test"))
	})

	t.Run("UntrackResource with tracker", func(t *testing.T) {
		tracker := &resourceTracker{}
		adapter := &gpuContextAdapter{tracker: tracker}

		m := newMockCloser("test")
		adapter.TrackResource(m)
		adapter.UntrackResource(m)

		_ = tracker.CloseAll()
		if m.closed {
			t.Error("Resource should NOT have been closed (was untracked)")
		}
	})

	t.Run("UntrackResource nil tracker", func(t *testing.T) {
		adapter := &gpuContextAdapter{}
		// Should not panic
		adapter.UntrackResource(newMockCloser("test"))
	})
}
