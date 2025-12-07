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

func TestAddressModeValues(t *testing.T) {
	// Verify iota ordering
	if AddressModeClampToEdge != 0 {
		t.Errorf("AddressModeClampToEdge = %d, want 0", AddressModeClampToEdge)
	}
	if AddressModeRepeat != 1 {
		t.Errorf("AddressModeRepeat = %d, want 1", AddressModeRepeat)
	}
	if AddressModeMirrorRepeat != 2 {
		t.Errorf("AddressModeMirrorRepeat = %d, want 2", AddressModeMirrorRepeat)
	}
}

func TestFilterModeValues(t *testing.T) {
	// Verify iota ordering
	if FilterModeNearest != 0 {
		t.Errorf("FilterModeNearest = %d, want 0", FilterModeNearest)
	}
	if FilterModeLinear != 1 {
		t.Errorf("FilterModeLinear = %d, want 1", FilterModeLinear)
	}
}

func TestMipmapFilterModeValues(t *testing.T) {
	// Verify iota ordering
	if MipmapFilterModeNearest != 0 {
		t.Errorf("MipmapFilterModeNearest = %d, want 0", MipmapFilterModeNearest)
	}
	if MipmapFilterModeLinear != 1 {
		t.Errorf("MipmapFilterModeLinear = %d, want 1", MipmapFilterModeLinear)
	}
}

func TestShaderStageValues(t *testing.T) {
	// Values must match WebGPU spec (bit flags)
	if ShaderStageNone != 0 {
		t.Errorf("ShaderStageNone = 0x%x, want 0", ShaderStageNone)
	}
	if ShaderStageVertex != 0x1 {
		t.Errorf("ShaderStageVertex = 0x%x, want 0x1", ShaderStageVertex)
	}
	if ShaderStageFragment != 0x2 {
		t.Errorf("ShaderStageFragment = 0x%x, want 0x2", ShaderStageFragment)
	}
	if ShaderStageCompute != 0x4 {
		t.Errorf("ShaderStageCompute = 0x%x, want 0x4", ShaderStageCompute)
	}
}

func TestShaderStageCombinations(t *testing.T) {
	// Test that shader stage flags can be combined
	stage := ShaderStageVertex | ShaderStageFragment
	if stage != 0x3 {
		t.Errorf("Combined stage = 0x%x, want 0x3", stage)
	}

	// Test individual flag checks
	if stage&ShaderStageVertex == 0 {
		t.Error("Expected Vertex flag to be set")
	}
	if stage&ShaderStageFragment == 0 {
		t.Error("Expected Fragment flag to be set")
	}
	if stage&ShaderStageCompute != 0 {
		t.Error("Expected Compute flag to NOT be set")
	}
}

func TestBufferUsageValues(t *testing.T) {
	// Values must match WebGPU spec (bit flags)
	tests := []struct {
		usage    BufferUsage
		expected BufferUsage
		name     string
	}{
		{BufferUsageMapRead, 0x0001, "MapRead"},
		{BufferUsageMapWrite, 0x0002, "MapWrite"},
		{BufferUsageCopySrc, 0x0004, "CopySrc"},
		{BufferUsageCopyDst, 0x0008, "CopyDst"},
		{BufferUsageIndex, 0x0010, "Index"},
		{BufferUsageVertex, 0x0020, "Vertex"},
		{BufferUsageUniform, 0x0040, "Uniform"},
		{BufferUsageStorage, 0x0080, "Storage"},
		{BufferUsageIndirect, 0x0100, "Indirect"},
		{BufferUsageQueryResolve, 0x0200, "QueryResolve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.usage != tt.expected {
				t.Errorf("BufferUsage%s = 0x%x, want 0x%x", tt.name, tt.usage, tt.expected)
			}
		})
	}
}

func TestBufferUsageCombinations(t *testing.T) {
	// Test typical buffer usage combination
	usage := BufferUsageVertex | BufferUsageCopyDst
	if usage != 0x28 {
		t.Errorf("Combined usage = 0x%x, want 0x28", usage)
	}

	if usage&BufferUsageVertex == 0 {
		t.Error("Expected Vertex flag to be set")
	}
	if usage&BufferUsageCopyDst == 0 {
		t.Error("Expected CopyDst flag to be set")
	}
	if usage&BufferUsageUniform != 0 {
		t.Error("Expected Uniform flag to NOT be set")
	}
}

func TestTextureDimensionValues(t *testing.T) {
	if TextureDimension1D != 0x00 {
		t.Errorf("TextureDimension1D = 0x%x, want 0x00", TextureDimension1D)
	}
	if TextureDimension2D != 0x01 {
		t.Errorf("TextureDimension2D = 0x%x, want 0x01", TextureDimension2D)
	}
	if TextureDimension3D != 0x02 {
		t.Errorf("TextureDimension3D = 0x%x, want 0x02", TextureDimension3D)
	}
}

func TestTextureViewDimensionValues(t *testing.T) {
	tests := []struct {
		dim      TextureViewDimension
		expected TextureViewDimension
		name     string
	}{
		{TextureViewDimensionUndefined, 0x00, "Undefined"},
		{TextureViewDimension1D, 0x01, "1D"},
		{TextureViewDimension2D, 0x02, "2D"},
		{TextureViewDimension2DArray, 0x03, "2DArray"},
		{TextureViewDimensionCube, 0x04, "Cube"},
		{TextureViewDimensionCubeArray, 0x05, "CubeArray"},
		{TextureViewDimension3D, 0x06, "3D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dim != tt.expected {
				t.Errorf("TextureViewDimension%s = 0x%x, want 0x%x", tt.name, tt.dim, tt.expected)
			}
		})
	}
}

func TestTextureAspectValues(t *testing.T) {
	if TextureAspectAll != 0x00 {
		t.Errorf("TextureAspectAll = 0x%x, want 0x00", TextureAspectAll)
	}
	if TextureAspectStencilOnly != 0x01 {
		t.Errorf("TextureAspectStencilOnly = 0x%x, want 0x01", TextureAspectStencilOnly)
	}
	if TextureAspectDepthOnly != 0x02 {
		t.Errorf("TextureAspectDepthOnly = 0x%x, want 0x02", TextureAspectDepthOnly)
	}
}

func TestIndexFormatValues(t *testing.T) {
	if IndexFormatUint16 != 0 {
		t.Errorf("IndexFormatUint16 = %d, want 0", IndexFormatUint16)
	}
	if IndexFormatUint32 != 1 {
		t.Errorf("IndexFormatUint32 = %d, want 1", IndexFormatUint32)
	}
}

func TestVertexStepModeValues(t *testing.T) {
	if VertexStepModeVertex != 0 {
		t.Errorf("VertexStepModeVertex = %d, want 0", VertexStepModeVertex)
	}
	if VertexStepModeInstance != 1 {
		t.Errorf("VertexStepModeInstance = %d, want 1", VertexStepModeInstance)
	}
}

func TestBufferBindingTypeValues(t *testing.T) {
	if BufferBindingTypeUndefined != 0 {
		t.Errorf("BufferBindingTypeUndefined = %d, want 0", BufferBindingTypeUndefined)
	}
	if BufferBindingTypeUniform != 1 {
		t.Errorf("BufferBindingTypeUniform = %d, want 1", BufferBindingTypeUniform)
	}
	if BufferBindingTypeStorage != 2 {
		t.Errorf("BufferBindingTypeStorage = %d, want 2", BufferBindingTypeStorage)
	}
	if BufferBindingTypeReadOnlyStorage != 3 {
		t.Errorf("BufferBindingTypeReadOnlyStorage = %d, want 3", BufferBindingTypeReadOnlyStorage)
	}
}

func TestSamplerBindingTypeValues(t *testing.T) {
	if SamplerBindingTypeUndefined != 0 {
		t.Errorf("SamplerBindingTypeUndefined = %d, want 0", SamplerBindingTypeUndefined)
	}
	if SamplerBindingTypeFiltering != 1 {
		t.Errorf("SamplerBindingTypeFiltering = %d, want 1", SamplerBindingTypeFiltering)
	}
	if SamplerBindingTypeNonFiltering != 2 {
		t.Errorf("SamplerBindingTypeNonFiltering = %d, want 2", SamplerBindingTypeNonFiltering)
	}
	if SamplerBindingTypeComparison != 3 {
		t.Errorf("SamplerBindingTypeComparison = %d, want 3", SamplerBindingTypeComparison)
	}
}

func TestTextureSampleTypeValues(t *testing.T) {
	tests := []struct {
		sampleType TextureSampleType
		expected   TextureSampleType
		name       string
	}{
		{TextureSampleTypeUndefined, 0, "Undefined"},
		{TextureSampleTypeFloat, 1, "Float"},
		{TextureSampleTypeUnfilterableFloat, 2, "UnfilterableFloat"},
		{TextureSampleTypeDepth, 3, "Depth"},
		{TextureSampleTypeSint, 4, "Sint"},
		{TextureSampleTypeUint, 5, "Uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sampleType != tt.expected {
				t.Errorf("TextureSampleType%s = %d, want %d", tt.name, tt.sampleType, tt.expected)
			}
		})
	}
}

func TestCompareFunctionValues(t *testing.T) {
	tests := []struct {
		fn       CompareFunction
		expected CompareFunction
		name     string
	}{
		{CompareFunctionUndefined, 0, "Undefined"},
		{CompareFunctionNever, 1, "Never"},
		{CompareFunctionLess, 2, "Less"},
		{CompareFunctionEqual, 3, "Equal"},
		{CompareFunctionLessEqual, 4, "LessEqual"},
		{CompareFunctionGreater, 5, "Greater"},
		{CompareFunctionNotEqual, 6, "NotEqual"},
		{CompareFunctionGreaterEqual, 7, "GreaterEqual"},
		{CompareFunctionAlways, 8, "Always"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fn != tt.expected {
				t.Errorf("CompareFunction%s = %d, want %d", tt.name, tt.fn, tt.expected)
			}
		})
	}
}

func TestExtent3D(t *testing.T) {
	ext := Extent3D{
		Width:              512,
		Height:             256,
		DepthOrArrayLayers: 1,
	}

	if ext.Width != 512 {
		t.Errorf("Extent3D.Width = %d, want 512", ext.Width)
	}
	if ext.Height != 256 {
		t.Errorf("Extent3D.Height = %d, want 256", ext.Height)
	}
	if ext.DepthOrArrayLayers != 1 {
		t.Errorf("Extent3D.DepthOrArrayLayers = %d, want 1", ext.DepthOrArrayLayers)
	}
}

func TestOrigin3D(t *testing.T) {
	origin := Origin3D{
		X: 10,
		Y: 20,
		Z: 0,
	}

	if origin.X != 10 {
		t.Errorf("Origin3D.X = %d, want 10", origin.X)
	}
	if origin.Y != 20 {
		t.Errorf("Origin3D.Y = %d, want 20", origin.Y)
	}
	if origin.Z != 0 {
		t.Errorf("Origin3D.Z = %d, want 0", origin.Z)
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
