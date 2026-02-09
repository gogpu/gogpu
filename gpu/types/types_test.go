package types

import (
	"testing"
)

func TestBackendTypeString(t *testing.T) {
	tests := []struct {
		backend  BackendType
		expected string
	}{
		{BackendAuto, "Auto"},
		{BackendRust, "Rust (wgpu-gpu)"},
		{BackendNative, "Native (Pure Go)"},
		{BackendGo, "Native (Pure Go)"}, // Alias should return same string
		{BackendType(99), "Auto"},       // Unknown defaults to Auto
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.backend.String()
			if got != tt.expected {
				t.Errorf("BackendType(%d).String() = %q, want %q", tt.backend, got, tt.expected)
			}
		})
	}
}

func TestBackendTypeValues(t *testing.T) {
	// Verify iota ordering: Auto=0, Native=1 (default), Rust=2 (opt-in)
	if BackendAuto != 0 {
		t.Errorf("BackendAuto = %d, want 0", BackendAuto)
	}
	if BackendNative != 1 {
		t.Errorf("BackendNative = %d, want 1", BackendNative)
	}
	if BackendRust != 2 {
		t.Errorf("BackendRust = %d, want 2", BackendRust)
	}
	// BackendGo is an alias for BackendNative
	if BackendGo != BackendNative {
		t.Errorf("BackendGo = %d, want %d (BackendNative)", BackendGo, BackendNative)
	}
}

func TestSurfaceStatusValues(t *testing.T) {
	// Verify iota ordering
	if SurfaceStatusSuccess != 0 {
		t.Errorf("SurfaceStatusSuccess = %d, want 0", SurfaceStatusSuccess)
	}
	if SurfaceStatusTimeout != 1 {
		t.Errorf("SurfaceStatusTimeout = %d, want 1", SurfaceStatusTimeout)
	}
	if SurfaceStatusOutdated != 2 {
		t.Errorf("SurfaceStatusOutdated = %d, want 2", SurfaceStatusOutdated)
	}
	if SurfaceStatusLost != 3 {
		t.Errorf("SurfaceStatusLost = %d, want 3", SurfaceStatusLost)
	}
	if SurfaceStatusError != 4 {
		t.Errorf("SurfaceStatusError = %d, want 4", SurfaceStatusError)
	}
}

func TestSurfaceTexture(t *testing.T) {
	st := SurfaceTexture{
		Texture: Texture(42),
		Status:  SurfaceStatusSuccess,
	}

	if st.Texture != 42 {
		t.Errorf("SurfaceTexture.Texture = %d, want 42", st.Texture)
	}
	if st.Status != SurfaceStatusSuccess {
		t.Errorf("SurfaceTexture.Status = %d, want %d", st.Status, SurfaceStatusSuccess)
	}
}

func TestSurfaceHandle(t *testing.T) {
	sh := SurfaceHandle{
		Instance: 0x1234,
		Window:   0x5678,
	}

	if sh.Instance != 0x1234 {
		t.Errorf("SurfaceHandle.Instance = 0x%x, want 0x1234", sh.Instance)
	}
	if sh.Window != 0x5678 {
		t.Errorf("SurfaceHandle.Window = 0x%x, want 0x5678", sh.Window)
	}
}

func TestNewHandleTypes(t *testing.T) {
	// Test new handle types added for texture support
	var (
		buffer          Buffer          = 1
		sampler         Sampler         = 2
		bindGroupLayout BindGroupLayout = 3
		bindGroup       BindGroup       = 4
		pipelineLayout  PipelineLayout  = 5
	)

	handles := []uintptr{
		uintptr(buffer),
		uintptr(sampler),
		uintptr(bindGroupLayout),
		uintptr(bindGroup),
		uintptr(pipelineLayout),
	}

	for i, h := range handles {
		expected := uintptr(i + 1)
		if h != expected {
			t.Errorf("New Handle[%d] = %d, want %d", i, h, expected)
		}
	}
}

func TestHandleTypes(t *testing.T) {
	// Verify handles are distinct types (compile-time check via assignments)
	var (
		instance       Instance       = 1
		adapter        Adapter        = 2
		device         Device         = 3
		queue          Queue          = 4
		surface        Surface        = 5
		texture        Texture        = 6
		textureView    TextureView    = 7
		shaderModule   ShaderModule   = 8
		renderPipeline RenderPipeline = 9
		commandEncoder CommandEncoder = 10
		commandBuffer  CommandBuffer  = 11
		renderPass     RenderPass     = 12
	)

	// Verify they hold correct values
	handles := []uintptr{
		uintptr(instance),
		uintptr(adapter),
		uintptr(device),
		uintptr(queue),
		uintptr(surface),
		uintptr(texture),
		uintptr(textureView),
		uintptr(shaderModule),
		uintptr(renderPipeline),
		uintptr(commandEncoder),
		uintptr(commandBuffer),
		uintptr(renderPass),
	}

	for i, h := range handles {
		expected := uintptr(i + 1)
		if h != expected {
			t.Errorf("Handle[%d] = %d, want %d", i, h, expected)
		}
	}
}

func TestImageDataLayout(t *testing.T) {
	layout := ImageDataLayout{
		Offset:       0,
		BytesPerRow:  512 * 4, // 512 pixels * 4 bytes (RGBA)
		RowsPerImage: 256,
	}

	if layout.Offset != 0 {
		t.Errorf("ImageDataLayout.Offset = %d, want 0", layout.Offset)
	}
	if layout.BytesPerRow != 2048 {
		t.Errorf("ImageDataLayout.BytesPerRow = %d, want 2048", layout.BytesPerRow)
	}
	if layout.RowsPerImage != 256 {
		t.Errorf("ImageDataLayout.RowsPerImage = %d, want 256", layout.RowsPerImage)
	}
}
