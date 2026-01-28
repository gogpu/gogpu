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

func TestTextureFormatDistinct(t *testing.T) {
	// Verify that texture formats are distinct (not checking specific values)
	formats := []TextureFormat{
		TextureFormatUndefined,
		TextureFormatRGBA8Unorm,
		TextureFormatRGBA8UnormSrgb,
		TextureFormatBGRA8Unorm,
		TextureFormatBGRA8UnormSrgb,
	}

	seen := make(map[TextureFormat]bool)
	for _, f := range formats {
		if seen[f] {
			t.Errorf("Duplicate texture format value: %d", f)
		}
		seen[f] = true
	}
}

func TestTextureUsageDistinct(t *testing.T) {
	// Verify that texture usage flags are distinct (bit flags)
	if TextureUsageNone != 0 {
		t.Errorf("TextureUsageNone = %d, want 0", TextureUsageNone)
	}

	// Verify all flags are distinct non-zero values
	flags := []TextureUsage{
		TextureUsageCopySrc,
		TextureUsageCopyDst,
		TextureUsageTextureBinding,
		TextureUsageStorageBinding,
		TextureUsageRenderAttachment,
	}

	for i, f1 := range flags {
		if f1 == 0 {
			t.Errorf("TextureUsage flag %d is zero", i)
		}
		for j := i + 1; j < len(flags); j++ {
			f2 := flags[j]
			// Flags should not overlap
			if f1&f2 != 0 {
				t.Errorf("TextureUsage flags %d and %d overlap: 0x%x & 0x%x", i, j, f1, f2)
			}
		}
	}
}

func TestTextureUsageCombinations(t *testing.T) {
	// Test that usage flags can be combined
	usage := TextureUsageCopySrc | TextureUsageRenderAttachment

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

func TestPresentModeDistinct(t *testing.T) {
	// Verify that present modes are distinct (values come from gputypes)
	modes := []PresentMode{
		PresentModeAutoVsync,
		PresentModeAutoNoVsync,
		PresentModeFifo,
		PresentModeFifoRelaxed,
		PresentModeImmediate,
		PresentModeMailbox,
	}

	seen := make(map[PresentMode]bool)
	for _, m := range modes {
		if seen[m] {
			t.Errorf("Duplicate present mode value: %d", m)
		}
		seen[m] = true
	}
}

func TestLoadStoreOpDistinct(t *testing.T) {
	// Verify that load/store ops are distinct
	if LoadOpClear == LoadOpLoad {
		t.Error("LoadOpClear and LoadOpLoad should be different")
	}
	if StoreOpStore == StoreOpDiscard {
		t.Error("StoreOpStore and StoreOpDiscard should be different")
	}
}

func TestPrimitiveTopologyDistinct(t *testing.T) {
	// Verify that primitive topologies are distinct
	topologies := []PrimitiveTopology{
		PrimitiveTopologyPointList,
		PrimitiveTopologyLineList,
		PrimitiveTopologyLineStrip,
		PrimitiveTopologyTriangleList,
		PrimitiveTopologyTriangleStrip,
	}

	seen := make(map[PrimitiveTopology]bool)
	for _, topo := range topologies {
		if seen[topo] {
			t.Errorf("Duplicate primitive topology value: %d", topo)
		}
		seen[topo] = true
	}
}

func TestCullModeValues(t *testing.T) {
	// gputypes uses iota starting at 0
	if CullModeNone != 0 {
		t.Errorf("CullModeNone = %d, want 0", CullModeNone)
	}
	if CullModeFront != 1 {
		t.Errorf("CullModeFront = %d, want 1", CullModeFront)
	}
	if CullModeBack != 2 {
		t.Errorf("CullModeBack = %d, want 2", CullModeBack)
	}
}

func TestFrontFaceValues(t *testing.T) {
	// gputypes uses iota starting at 0
	if FrontFaceCCW != 0 {
		t.Errorf("FrontFaceCCW = %d, want 0", FrontFaceCCW)
	}
	if FrontFaceCW != 1 {
		t.Errorf("FrontFaceCW = %d, want 1", FrontFaceCW)
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

func TestShaderStageDistinct(t *testing.T) {
	// ShaderStage is a different type in gputypes - it's not a bit flag
	// Just verify they are distinct values
	stages := []ShaderStage{
		ShaderStageNone,
		ShaderStageVertex,
		ShaderStageFragment,
		ShaderStageCompute,
	}

	seen := make(map[ShaderStage]bool)
	for _, s := range stages {
		if seen[s] && s != ShaderStageNone {
			t.Errorf("Duplicate shader stage value")
		}
		seen[s] = true
	}
}

func TestBufferUsageDistinct(t *testing.T) {
	// Verify buffer usage flags are distinct bit flags
	if BufferUsageNone != 0 {
		t.Errorf("BufferUsageNone = %d, want 0", BufferUsageNone)
	}

	flags := []BufferUsage{
		BufferUsageMapRead,
		BufferUsageMapWrite,
		BufferUsageCopySrc,
		BufferUsageCopyDst,
		BufferUsageIndex,
		BufferUsageVertex,
		BufferUsageUniform,
		BufferUsageStorage,
		BufferUsageIndirect,
		BufferUsageQueryResolve,
	}

	for i, f1 := range flags {
		if f1 == 0 {
			t.Errorf("BufferUsage flag %d is zero", i)
		}
		for j := i + 1; j < len(flags); j++ {
			f2 := flags[j]
			if f1&f2 != 0 {
				t.Errorf("BufferUsage flags %d and %d overlap: 0x%x & 0x%x", i, j, f1, f2)
			}
		}
	}
}

func TestBufferUsageCombinations(t *testing.T) {
	// Test typical buffer usage combination
	usage := BufferUsageVertex | BufferUsageCopyDst

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
	// gputypes uses iota starting at 0
	if TextureDimension1D != 0 {
		t.Errorf("TextureDimension1D = %d, want 0", TextureDimension1D)
	}
	if TextureDimension2D != 1 {
		t.Errorf("TextureDimension2D = %d, want 1", TextureDimension2D)
	}
	if TextureDimension3D != 2 {
		t.Errorf("TextureDimension3D = %d, want 2", TextureDimension3D)
	}
}

func TestTextureViewDimensionValues(t *testing.T) {
	// gputypes uses iota starting at 0
	tests := []struct {
		dim      TextureViewDimension
		expected TextureViewDimension
		name     string
	}{
		{TextureViewDimensionUndefined, 0, "Undefined"},
		{TextureViewDimension1D, 1, "1D"},
		{TextureViewDimension2D, 2, "2D"},
		{TextureViewDimension2DArray, 3, "2DArray"},
		{TextureViewDimensionCube, 4, "Cube"},
		{TextureViewDimensionCubeArray, 5, "CubeArray"},
		{TextureViewDimension3D, 6, "3D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dim != tt.expected {
				t.Errorf("TextureViewDimension%s = %d, want %d", tt.name, tt.dim, tt.expected)
			}
		})
	}
}

func TestTextureAspectValues(t *testing.T) {
	// gputypes uses iota starting at 0
	if TextureAspectAll != 0 {
		t.Errorf("TextureAspectAll = %d, want 0", TextureAspectAll)
	}
	if TextureAspectStencilOnly != 1 {
		t.Errorf("TextureAspectStencilOnly = %d, want 1", TextureAspectStencilOnly)
	}
	if TextureAspectDepthOnly != 2 {
		t.Errorf("TextureAspectDepthOnly = %d, want 2", TextureAspectDepthOnly)
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
	// gputypes TextureSampleType starts with Float = 0 (no Undefined)
	tests := []struct {
		sampleType TextureSampleType
		expected   TextureSampleType
		name       string
	}{
		{TextureSampleTypeFloat, 0, "Float"},
		{TextureSampleTypeUnfilterableFloat, 1, "UnfilterableFloat"},
		{TextureSampleTypeDepth, 2, "Depth"},
		{TextureSampleTypeSint, 3, "Sint"},
		{TextureSampleTypeUint, 4, "Uint"},
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
