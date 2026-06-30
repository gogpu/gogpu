package gogpu

import (
	"image"
	"testing"

	"github.com/gogpu/gogpu/gpu/types"
)

func TestDefaultConfigValues(t *testing.T) {
	// Clear env to avoid interference from GOGPU_GRAPHICS_API.
	t.Setenv("GOGPU_GRAPHICS_API", "")

	cfg := DefaultConfig()

	if cfg.Title != "GoGPU Application" {
		t.Errorf("Title = %q, want %q", cfg.Title, "GoGPU Application")
	}
	if cfg.Width != 800 {
		t.Errorf("Width = %d, want 800", cfg.Width)
	}
	if cfg.Height != 600 {
		t.Errorf("Height = %d, want 600", cfg.Height)
	}
	if !cfg.Resizable {
		t.Error("Resizable = false, want true")
	}
	if !cfg.VSync {
		t.Error("VSync = false, want true")
	}
	if cfg.Fullscreen {
		t.Error("Fullscreen = true, want false")
	}
	if cfg.Backend != types.BackendAuto {
		t.Errorf("Backend = %v, want BackendAuto", cfg.Backend)
	}
	if cfg.GraphicsAPI != types.GraphicsAPIAuto {
		t.Errorf("GraphicsAPI = %v, want GraphicsAPIAuto", cfg.GraphicsAPI)
	}
	if cfg.ContinuousRender {
		t.Error("ContinuousRender = true, want false")
	}
	if cfg.Frameless {
		t.Error("Frameless = true, want false")
	}
}

func TestConfigWithTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{"normal title", "My App"},
		{"empty title", ""},
		{"unicode title", "GPU App"},
		{"long title", "A Very Long Application Title That Exceeds Normal Length Requirements"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig().WithTitle(tt.title)
			if cfg.Title != tt.title {
				t.Errorf("Title = %q, want %q", cfg.Title, tt.title)
			}
		})
	}
}

func TestConfigWithSize(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
	}{
		{"standard", 1920, 1080},
		{"small", 320, 240},
		{"square", 512, 512},
		{"zero width", 0, 600},
		{"zero height", 800, 0},
		{"both zero", 0, 0},
		{"negative width", -1, 600},
		{"negative height", 800, -1},
		{"4K", 3840, 2160},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig().WithSize(tt.width, tt.height)
			if cfg.Width != tt.width {
				t.Errorf("Width = %d, want %d", cfg.Width, tt.width)
			}
			if cfg.Height != tt.height {
				t.Errorf("Height = %d, want %d", cfg.Height, tt.height)
			}
		})
	}
}

func TestConfigWithBackend(t *testing.T) {
	tests := []struct {
		name    string
		backend types.BackendType
	}{
		{"auto", types.BackendAuto},
		{"rust", types.BackendRust},
		{"native", types.BackendNative},
		{"go alias", types.BackendGo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig().WithBackend(tt.backend)
			if cfg.Backend != tt.backend {
				t.Errorf("Backend = %v, want %v", cfg.Backend, tt.backend)
			}
		})
	}
}

func TestConfigWithGraphicsAPI(t *testing.T) {
	tests := []struct {
		name string
		api  types.GraphicsAPI
	}{
		{"auto", types.GraphicsAPIAuto},
		{"vulkan", types.GraphicsAPIVulkan},
		{"dx12", types.GraphicsAPIDX12},
		{"metal", types.GraphicsAPIMetal},
		{"gles", types.GraphicsAPIGLES},
		{"software", types.GraphicsAPISoftware},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig().WithGraphicsAPI(tt.api)
			if cfg.GraphicsAPI != tt.api {
				t.Errorf("GraphicsAPI = %v, want %v", cfg.GraphicsAPI, tt.api)
			}
		})
	}
}

func TestConfigWithContinuousRender(t *testing.T) {
	tests := []struct {
		name       string
		continuous bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig().WithContinuousRender(tt.continuous)
			if cfg.ContinuousRender != tt.continuous {
				t.Errorf("ContinuousRender = %v, want %v", cfg.ContinuousRender, tt.continuous)
			}
		})
	}
}

func TestConfigWithFramelessBuilder(t *testing.T) {
	tests := []struct {
		name      string
		frameless bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig().WithFrameless(tt.frameless)
			if cfg.Frameless != tt.frameless {
				t.Errorf("Frameless = %v, want %v", cfg.Frameless, tt.frameless)
			}
		})
	}
}

func TestConfigBuilderChaining(t *testing.T) {
	t.Setenv("GOGPU_GRAPHICS_API", "")

	cfg := DefaultConfig().
		WithTitle("Test App").
		WithSize(1024, 768).
		WithBackend(types.BackendNative).
		WithGraphicsAPI(types.GraphicsAPIVulkan).
		WithContinuousRender(false).
		WithFrameless(true)

	if cfg.Title != "Test App" {
		t.Errorf("Title = %q, want %q", cfg.Title, "Test App")
	}
	if cfg.Width != 1024 {
		t.Errorf("Width = %d, want 1024", cfg.Width)
	}
	if cfg.Height != 768 {
		t.Errorf("Height = %d, want 768", cfg.Height)
	}
	if cfg.Backend != types.BackendNative {
		t.Errorf("Backend = %v, want BackendNative", cfg.Backend)
	}
	if cfg.GraphicsAPI != types.GraphicsAPIVulkan {
		t.Errorf("GraphicsAPI = %v, want GraphicsAPIVulkan", cfg.GraphicsAPI)
	}
	if cfg.ContinuousRender {
		t.Error("ContinuousRender = true, want false")
	}
	if !cfg.Frameless {
		t.Error("Frameless = false, want true")
	}
	// Verify defaults not overridden remain intact.
	if !cfg.Resizable {
		t.Error("Resizable = false, want true (default)")
	}
	if !cfg.VSync {
		t.Error("VSync = false, want true (default)")
	}
}

func TestConfigImmutability(t *testing.T) {
	t.Setenv("GOGPU_GRAPHICS_API", "")

	original := DefaultConfig()

	_ = original.WithTitle("Modified")
	if original.Title != "GoGPU Application" {
		t.Errorf("Original Title was mutated to %q, want %q", original.Title, "GoGPU Application")
	}

	_ = original.WithSize(1920, 1080)
	if original.Width != 800 || original.Height != 600 {
		t.Errorf("Original Size was mutated to %dx%d, want 800x600", original.Width, original.Height)
	}

	_ = original.WithBackend(types.BackendRust)
	if original.Backend != types.BackendAuto {
		t.Errorf("Original Backend was mutated to %v, want BackendAuto", original.Backend)
	}

	_ = original.WithGraphicsAPI(types.GraphicsAPIVulkan)
	if original.GraphicsAPI != types.GraphicsAPIAuto {
		t.Errorf("Original GraphicsAPI was mutated to %v, want GraphicsAPIAuto", original.GraphicsAPI)
	}

	_ = original.WithContinuousRender(true)
	if original.ContinuousRender {
		t.Error("Original ContinuousRender was mutated to true, want false")
	}

	_ = original.WithFrameless(true)
	if original.Frameless {
		t.Error("Original Frameless was mutated to true, want false")
	}
}

func TestReExportedBackendConstants(t *testing.T) {
	if BackendAuto != types.BackendAuto {
		t.Errorf("BackendAuto = %v, want %v", BackendAuto, types.BackendAuto)
	}
	if BackendRust != types.BackendRust {
		t.Errorf("BackendRust = %v, want %v", BackendRust, types.BackendRust)
	}
	if BackendNative != types.BackendNative {
		t.Errorf("BackendNative = %v, want %v", BackendNative, types.BackendNative)
	}
	if BackendGo != types.BackendGo {
		t.Errorf("BackendGo = %v, want %v", BackendGo, types.BackendGo)
	}
}

func TestReExportedGraphicsAPIConstants(t *testing.T) {
	if GraphicsAPIAuto != types.GraphicsAPIAuto {
		t.Errorf("GraphicsAPIAuto = %v, want %v", GraphicsAPIAuto, types.GraphicsAPIAuto)
	}
	if GraphicsAPIVulkan != types.GraphicsAPIVulkan {
		t.Errorf("GraphicsAPIVulkan = %v, want %v", GraphicsAPIVulkan, types.GraphicsAPIVulkan)
	}
	if GraphicsAPIDX12 != types.GraphicsAPIDX12 {
		t.Errorf("GraphicsAPIDX12 = %v, want %v", GraphicsAPIDX12, types.GraphicsAPIDX12)
	}
	if GraphicsAPIMetal != types.GraphicsAPIMetal {
		t.Errorf("GraphicsAPIMetal = %v, want %v", GraphicsAPIMetal, types.GraphicsAPIMetal)
	}
	if GraphicsAPIGLES != types.GraphicsAPIGLES {
		t.Errorf("GraphicsAPIGLES = %v, want %v", GraphicsAPIGLES, types.GraphicsAPIGLES)
	}
	if GraphicsAPISoftware != types.GraphicsAPISoftware {
		t.Errorf("GraphicsAPISoftware = %v, want %v", GraphicsAPISoftware, types.GraphicsAPISoftware)
	}
}

func TestBackendGoIsNativeAlias(t *testing.T) {
	if BackendGo != BackendNative {
		t.Errorf("BackendGo (%v) != BackendNative (%v), expected them to be the same", BackendGo, BackendNative)
	}
}

func TestGraphicsAPIFromEnv(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   types.GraphicsAPI
	}{
		{"vulkan", "vulkan", types.GraphicsAPIVulkan},
		{"vk alias", "vk", types.GraphicsAPIVulkan},
		{"VULKAN uppercase", "VULKAN", types.GraphicsAPIVulkan},
		{"Vulkan mixed case", "Vulkan", types.GraphicsAPIVulkan},
		{"dx12", "dx12", types.GraphicsAPIDX12},
		{"d3d12 alias", "d3d12", types.GraphicsAPIDX12},
		{"directx alias", "directx", types.GraphicsAPIDX12},
		{"DX12 uppercase", "DX12", types.GraphicsAPIDX12},
		{"metal", "metal", types.GraphicsAPIMetal},
		{"Metal mixed case", "Metal", types.GraphicsAPIMetal},
		{"gles", "gles", types.GraphicsAPIGLES},
		{"gl alias", "gl", types.GraphicsAPIGLES},
		{"opengl alias", "opengl", types.GraphicsAPIGLES},
		{"software", "software", types.GraphicsAPISoftware},
		{"sw alias", "sw", types.GraphicsAPISoftware},
		{"cpu alias", "cpu", types.GraphicsAPISoftware},
		{"empty string", "", types.GraphicsAPIAuto},
		{"unknown value", "webgl", types.GraphicsAPIAuto},
		{"unknown random", "foobar", types.GraphicsAPIAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOGPU_GRAPHICS_API", tt.envVal)
			got := graphicsAPIFromEnv()
			if got != tt.want {
				t.Errorf("graphicsAPIFromEnv() with env=%q = %v, want %v", tt.envVal, got, tt.want)
			}
		})
	}
}

func TestDefaultConfigReadsEnv(t *testing.T) {
	t.Setenv("GOGPU_GRAPHICS_API", "vulkan")

	cfg := DefaultConfig()
	if cfg.GraphicsAPI != types.GraphicsAPIVulkan {
		t.Errorf("DefaultConfig().GraphicsAPI = %v, want GraphicsAPIVulkan when env=vulkan", cfg.GraphicsAPI)
	}
}

func TestWithGraphicsAPIOverridesEnv(t *testing.T) {
	t.Setenv("GOGPU_GRAPHICS_API", "vulkan")

	cfg := DefaultConfig().WithGraphicsAPI(types.GraphicsAPIMetal)
	if cfg.GraphicsAPI != types.GraphicsAPIMetal {
		t.Errorf("GraphicsAPI = %v, want GraphicsAPIMetal (WithGraphicsAPI should override env)", cfg.GraphicsAPI)
	}
}

func TestConfigWithResizable(t *testing.T) {
	cfg := DefaultConfig().WithResizable(false)
	if cfg.Resizable {
		t.Error("WithResizable(false): Resizable = true, want false")
	}
	cfg = cfg.WithResizable(true)
	if !cfg.Resizable {
		t.Error("WithResizable(true): Resizable = false, want true")
	}

	original := DefaultConfig()
	_ = original.WithResizable(false)
	if !original.Resizable {
		t.Error("WithResizable mutated original Config, want immutable")
	}
}

func TestConfigWithMinSize(t *testing.T) {
	cfg := DefaultConfig().WithMinSize(200, 150)
	if cfg.MinWidth != 200 {
		t.Errorf("MinWidth = %d, want 200", cfg.MinWidth)
	}
	if cfg.MinHeight != 150 {
		t.Errorf("MinHeight = %d, want 150", cfg.MinHeight)
	}

	// clear
	cfg2 := cfg.WithMinSize(0, 0)
	if cfg2.MinWidth != 0 || cfg2.MinHeight != 0 {
		t.Errorf("WithMinSize(0,0): want 0x0, got %dx%d", cfg2.MinWidth, cfg2.MinHeight)
	}

	// immutability
	original := DefaultConfig()
	_ = original.WithMinSize(100, 100)
	if original.MinWidth != 0 || original.MinHeight != 0 {
		t.Error("WithMinSize mutated original Config")
	}
}

func TestConfigWithMaxSize(t *testing.T) {
	cfg := DefaultConfig().WithMaxSize(1920, 1080)
	if cfg.MaxWidth != 1920 {
		t.Errorf("MaxWidth = %d, want 1920", cfg.MaxWidth)
	}
	if cfg.MaxHeight != 1080 {
		t.Errorf("MaxHeight = %d, want 1080", cfg.MaxHeight)
	}

	// clear
	cfg2 := cfg.WithMaxSize(0, 0)
	if cfg2.MaxWidth != 0 || cfg2.MaxHeight != 0 {
		t.Errorf("WithMaxSize(0,0): want 0x0, got %dx%d", cfg2.MaxWidth, cfg2.MaxHeight)
	}

	// immutability
	original := DefaultConfig()
	_ = original.WithMaxSize(800, 600)
	if original.MaxWidth != 0 || original.MaxHeight != 0 {
		t.Error("WithMaxSize mutated original Config")
	}
}

func TestConfigTabbing(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TabbingMode != TabbingDisallowed {
		t.Fatalf("expected default TabbingDisallowed, got %v", cfg.TabbingMode)
	}

	cfg = cfg.WithTabbingMode(TabbingPreferred).WithTabbingIdentifier("test.id")
	if cfg.TabbingMode != TabbingPreferred {
		t.Fatalf("expected Preferred, got %v", cfg.TabbingMode)
	}
	if cfg.TabbingIdentifier != "test.id" {
		t.Fatalf("expected test.id, got %v", cfg.TabbingIdentifier)
	}
}

func TestConfigWithIcon(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	cfg := DefaultConfig().WithIcon(img)
	if cfg.Icon != img {
		t.Fatal("WithIcon: Icon field not set")
	}
	cfg2 := cfg.WithIcon(nil)
	if cfg2.Icon != nil {
		t.Fatal("WithIcon(nil): Icon field not cleared")
	}
}
