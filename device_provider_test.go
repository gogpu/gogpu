package gogpu

import (
	"testing"

	"github.com/gogpu/gputypes"
)

// TestDeviceProviderInterface verifies the DeviceProvider interface contract.
func TestDeviceProviderInterface(t *testing.T) {
	// Verify interface methods exist with HAL types
	var _ DeviceProvider = (*rendererDeviceProvider)(nil)
}

// TestDeviceProviderNilBeforeRun verifies DeviceProvider returns nil before Run().
func TestDeviceProviderNilBeforeRun(t *testing.T) {
	app := NewApp(DefaultConfig())

	provider := app.DeviceProvider()
	if provider != nil {
		t.Error("DeviceProvider should return nil before Run() is called")
	}
}

// TestRendererDeviceProviderImplementation verifies rendererDeviceProvider implements DeviceProvider.
func TestRendererDeviceProviderImplementation(t *testing.T) {
	// Compile-time check that rendererDeviceProvider implements DeviceProvider
	var _ DeviceProvider = (*rendererDeviceProvider)(nil)
}

// TestRendererDeviceProviderMethods tests the methods of rendererDeviceProvider.
func TestRendererDeviceProviderMethods(t *testing.T) {
	// Create a renderer with nil wgpu objects (no actual GPU needed)
	renderer := &Renderer{
		device: nil, // wgpu device is nil in test
		format: gputypes.TextureFormatBGRA8Unorm,
	}

	provider := &rendererDeviceProvider{renderer: renderer}

	t.Run("Device", func(t *testing.T) {
		// Device is nil (no actual GPU in test)
		if provider.Device() != nil {
			t.Error("Device() should be nil with nil device")
		}
	})

	t.Run("Queue", func(t *testing.T) {
		// Queue is nil (no actual GPU in test)
		if provider.Queue() != nil {
			t.Error("Queue() should be nil with nil queue")
		}
	})

	t.Run("SurfaceFormat", func(t *testing.T) {
		if provider.SurfaceFormat() != gputypes.TextureFormatBGRA8Unorm {
			t.Errorf("SurfaceFormat() = %v, want %v", provider.SurfaceFormat(), gputypes.TextureFormatBGRA8Unorm)
		}
	})
}

// TestDefaultConfig verifies default configuration.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Title == "" {
		t.Error("DefaultConfig should have a non-empty Title")
	}
	if config.Width <= 0 {
		t.Error("DefaultConfig should have positive Width")
	}
	if config.Height <= 0 {
		t.Error("DefaultConfig should have positive Height")
	}
}

// TestConfigBuilder tests the fluent configuration API.
func TestConfigBuilder(t *testing.T) {
	config := DefaultConfig().
		WithTitle("Test Window").
		WithSize(1024, 768).
		WithBackend(BackendGo)

	if config.Title != "Test Window" {
		t.Errorf("Title = %q, want %q", config.Title, "Test Window")
	}
	if config.Width != 1024 {
		t.Errorf("Width = %d, want %d", config.Width, 1024)
	}
	if config.Height != 768 {
		t.Errorf("Height = %d, want %d", config.Height, 768)
	}
	if config.Backend != BackendGo {
		t.Errorf("Backend = %v, want %v", config.Backend, BackendGo)
	}
}
