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
		{BackendRust, "Rust (wgpu-native)"},
		{BackendGo, "Pure Go"},
		{BackendType(99), "Auto"}, // Unknown defaults to Auto
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
	// Verify iota ordering
	if BackendAuto != 0 {
		t.Errorf("BackendAuto = %d, want 0", BackendAuto)
	}
	if BackendRust != 1 {
		t.Errorf("BackendRust = %d, want 1", BackendRust)
	}
	if BackendGo != 2 {
		t.Errorf("BackendGo = %d, want 2", BackendGo)
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

func TestTextureFormatValues(t *testing.T) {
	// Values must match WebGPU spec
	if TextureFormatRGBA8Unorm != 0x12 {
		t.Errorf("TextureFormatRGBA8Unorm = 0x%x, want 0x12", TextureFormatRGBA8Unorm)
	}
	if TextureFormatBGRA8Unorm != 0x17 {
		t.Errorf("TextureFormatBGRA8Unorm = 0x%x, want 0x17", TextureFormatBGRA8Unorm)
	}
}

func TestTextureUsageValues(t *testing.T) {
	// Values must match WebGPU spec (bit flags)
	if TextureUsageCopySrc != 0x01 {
		t.Errorf("TextureUsageCopySrc = 0x%x, want 0x01", TextureUsageCopySrc)
	}
	if TextureUsageCopyDst != 0x02 {
		t.Errorf("TextureUsageCopyDst = 0x%x, want 0x02", TextureUsageCopyDst)
	}
	if TextureUsageTextureBinding != 0x04 {
		t.Errorf("TextureUsageTextureBinding = 0x%x, want 0x04", TextureUsageTextureBinding)
	}
	if TextureUsageStorageBinding != 0x08 {
		t.Errorf("TextureUsageStorageBinding = 0x%x, want 0x08", TextureUsageStorageBinding)
	}
	if TextureUsageRenderAttachment != 0x10 {
		t.Errorf("TextureUsageRenderAttachment = 0x%x, want 0x10", TextureUsageRenderAttachment)
	}
}

func TestTextureUsageCombinations(t *testing.T) {
	// Test that usage flags can be combined
	usage := TextureUsageCopySrc | TextureUsageRenderAttachment
	if usage != 0x11 {
		t.Errorf("Combined usage = 0x%x, want 0x11", usage)
	}

	// Test individual flag checks
	if usage&TextureUsageCopySrc == 0 {
		t.Error("Expected CopySrc flag to be set")
	}
	if usage&TextureUsageRenderAttachment == 0 {
		t.Error("Expected RenderAttachment flag to be set")
	}
	if usage&TextureUsageCopyDst != 0 {
		t.Error("Expected CopyDst flag to NOT be set")
	}
}

func TestPresentModeValues(t *testing.T) {
	if PresentModeFifo != 0x01 {
		t.Errorf("PresentModeFifo = 0x%x, want 0x01", PresentModeFifo)
	}
	if PresentModeFifoRelaxed != 0x02 {
		t.Errorf("PresentModeFifoRelaxed = 0x%x, want 0x02", PresentModeFifoRelaxed)
	}
	if PresentModeImmediate != 0x03 {
		t.Errorf("PresentModeImmediate = 0x%x, want 0x03", PresentModeImmediate)
	}
	if PresentModeMailbox != 0x04 {
		t.Errorf("PresentModeMailbox = 0x%x, want 0x04", PresentModeMailbox)
	}
}

func TestLoadStoreOpValues(t *testing.T) {
	if LoadOpClear != 0x01 {
		t.Errorf("LoadOpClear = 0x%x, want 0x01", LoadOpClear)
	}
	if LoadOpLoad != 0x02 {
		t.Errorf("LoadOpLoad = 0x%x, want 0x02", LoadOpLoad)
	}
	if StoreOpStore != 0x01 {
		t.Errorf("StoreOpStore = 0x%x, want 0x01", StoreOpStore)
	}
	if StoreOpDiscard != 0x02 {
		t.Errorf("StoreOpDiscard = 0x%x, want 0x02", StoreOpDiscard)
	}
}

func TestPrimitiveTopologyValues(t *testing.T) {
	if PrimitiveTopologyPointList != 0x00 {
		t.Errorf("PrimitiveTopologyPointList = 0x%x, want 0x00", PrimitiveTopologyPointList)
	}
	if PrimitiveTopologyLineList != 0x01 {
		t.Errorf("PrimitiveTopologyLineList = 0x%x, want 0x01", PrimitiveTopologyLineList)
	}
	if PrimitiveTopologyLineStrip != 0x02 {
		t.Errorf("PrimitiveTopologyLineStrip = 0x%x, want 0x02", PrimitiveTopologyLineStrip)
	}
	if PrimitiveTopologyTriangleList != 0x03 {
		t.Errorf("PrimitiveTopologyTriangleList = 0x%x, want 0x03", PrimitiveTopologyTriangleList)
	}
	if PrimitiveTopologyTriangleStrip != 0x04 {
		t.Errorf("PrimitiveTopologyTriangleStrip = 0x%x, want 0x04", PrimitiveTopologyTriangleStrip)
	}
}

func TestCullModeValues(t *testing.T) {
	if CullModeNone != 0x00 {
		t.Errorf("CullModeNone = 0x%x, want 0x00", CullModeNone)
	}
	if CullModeFront != 0x01 {
		t.Errorf("CullModeFront = 0x%x, want 0x01", CullModeFront)
	}
	if CullModeBack != 0x02 {
		t.Errorf("CullModeBack = 0x%x, want 0x02", CullModeBack)
	}
}

func TestFrontFaceValues(t *testing.T) {
	if FrontFaceCCW != 0x00 {
		t.Errorf("FrontFaceCCW = 0x%x, want 0x00", FrontFaceCCW)
	}
	if FrontFaceCW != 0x01 {
		t.Errorf("FrontFaceCW = 0x%x, want 0x01", FrontFaceCW)
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
