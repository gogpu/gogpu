package types

import (
	"testing"
)

func TestGraphicsAPIString(t *testing.T) {
	tests := []struct {
		api      GraphicsAPI
		expected string
	}{
		{GraphicsAPIAuto, "Auto"},
		{GraphicsAPIVulkan, "Vulkan"},
		{GraphicsAPIDX12, "DX12"},
		{GraphicsAPIMetal, "Metal"},
		{GraphicsAPIGLES, "GLES"},
		{GraphicsAPISoftware, "Software"},
		{GraphicsAPI(99), "Auto"}, // Unknown defaults to Auto
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.api.String()
			if got != tt.expected {
				t.Errorf("GraphicsAPI(%d).String() = %q, want %q", tt.api, got, tt.expected)
			}
		})
	}
}

func TestGraphicsAPIValues(t *testing.T) {
	if GraphicsAPIAuto != 0 {
		t.Errorf("GraphicsAPIAuto = %d, want 0", GraphicsAPIAuto)
	}
	if GraphicsAPIVulkan != 1 {
		t.Errorf("GraphicsAPIVulkan = %d, want 1", GraphicsAPIVulkan)
	}
	if GraphicsAPIDX12 != 2 {
		t.Errorf("GraphicsAPIDX12 = %d, want 2", GraphicsAPIDX12)
	}
	if GraphicsAPIMetal != 3 {
		t.Errorf("GraphicsAPIMetal = %d, want 3", GraphicsAPIMetal)
	}
	if GraphicsAPIGLES != 4 {
		t.Errorf("GraphicsAPIGLES = %d, want 4", GraphicsAPIGLES)
	}
	if GraphicsAPISoftware != 5 {
		t.Errorf("GraphicsAPISoftware = %d, want 5", GraphicsAPISoftware)
	}
}
