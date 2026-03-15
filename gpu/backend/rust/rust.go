//go:build rust

// Package rust provides the WebGPU backend using wgpu-native (Rust) via go-webgpu/webgpu.
// This backend offers maximum performance and is battle-tested in production.
// Supported on Windows, macOS, and Linux.
//
// Build with: go build -tags rust
package rust

import (
	"errors"
	"fmt"
	"math"
	"time"
	"unsafe"

	"github.com/go-webgpu/webgpu/wgpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// ---------------------------------------------------------------------------
// Backend — implements hal.Backend
// ---------------------------------------------------------------------------

// rustBackend implements hal.Backend for the Rust (wgpu-native) backend.
type rustBackend struct{}

// Variant returns the backend type identifier for the current platform.
func (b rustBackend) Variant() gputypes.Backend { return platformVariant() }

// CreateInstance creates a new GPU instance via wgpu-native.
func (b rustBackend) CreateInstance(desc *hal.InstanceDescriptor) (hal.Instance, error) {
	inst, err := wgpu.CreateInstance(nil)
	if err != nil {
		return nil, fmt.Errorf("rust backend: create instance: %w", err)
	}
	return &rustInstance{inst: inst}, nil
}

// ---------------------------------------------------------------------------
// Instance — implements hal.Instance
// ---------------------------------------------------------------------------

// rustInstance wraps wgpu.Instance to implement hal.Instance.
type rustInstance struct {
	inst *wgpu.Instance
}

// CreateSurface creates a rendering surface from platform handles.
// Platform-specific implementation is in rust_{windows,darwin,linux}.go.
func (i *rustInstance) CreateSurface(displayHandle, windowHandle uintptr) (hal.Surface, error) {
	return i.createPlatformSurface(displayHandle, windowHandle)
}

// EnumerateAdapters returns compatible GPU adapters.
// wgpu-native uses RequestAdapter, so we request one adapter and return it as a single-element slice.
func (i *rustInstance) EnumerateAdapters(surfaceHint hal.Surface) []hal.ExposedAdapter {
	opts := &wgpu.RequestAdapterOptions{}
	if surfaceHint != nil {
		if rs, ok := surfaceHint.(*rustSurface); ok {
			opts.CompatibleSurface = rs.surf.Handle()
		}
	}

	adapter, err := i.inst.RequestAdapter(opts)
	if err != nil || adapter == nil {
		return nil
	}

	// Get adapter info
	info, _ := adapter.GetInfo()
	var adapterInfo gputypes.AdapterInfo
	if info != nil {
		adapterInfo.Name = info.Device
		adapterInfo.Vendor = info.Vendor
	}

	// Get adapter limits for validation layer.
	// go-webgpu returns wgpu.SupportedLimits, convert to gputypes.Limits.
	halLimits := gputypes.DefaultLimits()
	if supported, err := adapter.GetLimits(); err == nil && supported != nil {
		wl := supported.Limits
		halLimits.MaxTextureDimension1D = wl.MaxTextureDimension1D
		halLimits.MaxTextureDimension2D = wl.MaxTextureDimension2D
		halLimits.MaxTextureDimension3D = wl.MaxTextureDimension3D
		halLimits.MaxTextureArrayLayers = wl.MaxTextureArrayLayers
		halLimits.MaxBindGroups = wl.MaxBindGroups
		halLimits.MaxSampledTexturesPerShaderStage = wl.MaxSampledTexturesPerShaderStage
		halLimits.MaxSamplersPerShaderStage = wl.MaxSamplersPerShaderStage
		halLimits.MaxStorageBuffersPerShaderStage = wl.MaxStorageBuffersPerShaderStage
		halLimits.MaxStorageTexturesPerShaderStage = wl.MaxStorageTexturesPerShaderStage
		halLimits.MaxUniformBuffersPerShaderStage = wl.MaxUniformBuffersPerShaderStage
		halLimits.MaxUniformBufferBindingSize = wl.MaxUniformBufferBindingSize
		halLimits.MaxStorageBufferBindingSize = wl.MaxStorageBufferBindingSize
		halLimits.MaxVertexBuffers = wl.MaxVertexBuffers
		halLimits.MaxBufferSize = wl.MaxBufferSize
		halLimits.MaxVertexAttributes = wl.MaxVertexAttributes
		halLimits.MaxVertexBufferArrayStride = wl.MaxVertexBufferArrayStride
		halLimits.MaxComputeWorkgroupSizeX = wl.MaxComputeWorkgroupSizeX
		halLimits.MaxComputeWorkgroupSizeY = wl.MaxComputeWorkgroupSizeY
		halLimits.MaxComputeWorkgroupSizeZ = wl.MaxComputeWorkgroupSizeZ
		halLimits.MaxComputeInvocationsPerWorkgroup = wl.MaxComputeInvocationsPerWorkgroup
		halLimits.MaxComputeWorkgroupsPerDimension = wl.MaxComputeWorkgroupsPerDimension
	}

	return []hal.ExposedAdapter{
		{
			Adapter: &rustAdapter{adapter: adapter},
			Info:    adapterInfo,
			Capabilities: hal.Capabilities{
				Limits: halLimits,
			},
		},
	}
}

// Destroy releases the instance.
func (i *rustInstance) Destroy() {
	if i.inst != nil {
		i.inst.Release()
		i.inst = nil
	}
}

// ---------------------------------------------------------------------------
// Adapter — implements hal.Adapter
// ---------------------------------------------------------------------------

// rustAdapter wraps wgpu.Adapter to implement hal.Adapter.
type rustAdapter struct {
	adapter *wgpu.Adapter
}

// Open opens a logical device with the requested features and limits.
func (a *rustAdapter) Open(features gputypes.Features, limits gputypes.Limits) (hal.OpenDevice, error) {
	dev, err := a.adapter.RequestDevice(nil)
	if err != nil {
		return hal.OpenDevice{}, fmt.Errorf("rust backend: request device: %w", err)
	}

	queue := dev.GetQueue()
	if queue == nil {
		dev.Release()
		return hal.OpenDevice{}, fmt.Errorf("rust backend: failed to get queue")
	}

	return hal.OpenDevice{
		Device: &rustDevice{dev: dev},
		Queue:  &rustQueue{q: queue, dev: dev},
	}, nil
}

// TextureFormatCapabilities returns capabilities for a specific texture format.
func (a *rustAdapter) TextureFormatCapabilities(format gputypes.TextureFormat) hal.TextureFormatCapabilities {
	// wgpu-native doesn't expose per-format capabilities directly.
	// Return common capabilities.
	return hal.TextureFormatCapabilities{
		Flags: hal.TextureFormatCapabilitySampled |
			hal.TextureFormatCapabilityRenderAttachment |
			hal.TextureFormatCapabilityBlendable |
			hal.TextureFormatCapabilityMultisample,
	}
}

// SurfaceCapabilities returns capabilities for a specific surface.
func (a *rustAdapter) SurfaceCapabilities(surface hal.Surface) *hal.SurfaceCapabilities {
	rs, ok := surface.(*rustSurface)
	if !ok || rs.surf == nil {
		return nil
	}

	caps, err := rs.surf.GetCapabilities(a.adapter)
	if err != nil || caps == nil {
		return nil
	}

	return &hal.SurfaceCapabilities{
		Formats:      caps.Formats,
		PresentModes: caps.PresentModes,
		AlphaModes:   caps.AlphaModes,
	}
}

// Destroy releases the adapter.
func (a *rustAdapter) Destroy() {
	if a.adapter != nil {
		a.adapter.Release()
		a.adapter = nil
	}
}

// ---------------------------------------------------------------------------
// Device — implements hal.Device
// ---------------------------------------------------------------------------

// rustDevice wraps wgpu.Device to implement hal.Device.
type rustDevice struct {
	dev *wgpu.Device
}

// CreateBuffer creates a GPU buffer.
func (d *rustDevice) CreateBuffer(desc *hal.BufferDescriptor) (hal.Buffer, error) {
	var mappedAtCreation wgpu.Bool
	if desc.MappedAtCreation {
		mappedAtCreation = wgpu.True
	}

	buf := d.dev.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            wgpu.EmptyStringView(),
		Size:             desc.Size,
		Usage:            desc.Usage,
		MappedAtCreation: mappedAtCreation,
	})
	if buf == nil {
		return nil, fmt.Errorf("rust backend: failed to create buffer")
	}
	return &rustBuffer{buf: buf}, nil
}

// DestroyBuffer destroys a GPU buffer.
func (d *rustDevice) DestroyBuffer(buffer hal.Buffer) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		rb.buf.Release()
		rb.buf = nil
	}
}

// CreateTexture creates a GPU texture.
func (d *rustDevice) CreateTexture(desc *hal.TextureDescriptor) (hal.Texture, error) {
	tex := d.dev.CreateTexture(&wgpu.TextureDescriptor{
		Label: wgpu.EmptyStringView(),
		Size: gputypes.Extent3D{
			Width:              desc.Size.Width,
			Height:             desc.Size.Height,
			DepthOrArrayLayers: desc.Size.DepthOrArrayLayers,
		},
		MipLevelCount: desc.MipLevelCount,
		SampleCount:   desc.SampleCount,
		Dimension:     desc.Dimension,
		Format:        desc.Format,
		Usage:         desc.Usage,
	})
	if tex == nil {
		return nil, fmt.Errorf("rust backend: failed to create texture")
	}
	return &rustTexture{tex: tex}, nil
}

// DestroyTexture destroys a GPU texture.
func (d *rustDevice) DestroyTexture(texture hal.Texture) {
	if rt, ok := texture.(*rustTexture); ok && rt.tex != nil {
		rt.tex.Release()
		rt.tex = nil
	}
}

// CreateTextureView creates a view into a texture.
func (d *rustDevice) CreateTextureView(texture hal.Texture, desc *hal.TextureViewDescriptor) (hal.TextureView, error) {
	rt, ok := texture.(*rustTexture)
	if !ok || rt.tex == nil {
		// Also check for surface textures
		if st, ok2 := texture.(*rustSurfaceTexture); ok2 && st.tex != nil {
			view := st.tex.CreateView(nil)
			if view == nil {
				return nil, fmt.Errorf("rust backend: failed to create texture view")
			}
			return &rustTextureView{view: view}, nil
		}
		return nil, fmt.Errorf("rust backend: invalid texture")
	}

	// Pass nil descriptor for default view
	var wgpuDesc *wgpu.TextureViewDescriptor
	if desc != nil {
		// Convert HAL conventions to wgpu-native conventions:
		// HAL uses 0 for "all remaining" mip levels/array layers,
		// wgpu-native uses math.MaxUint32 (WGPU_MIP_LEVEL_COUNT_UNDEFINED).
		mipCount := desc.MipLevelCount
		if mipCount == 0 {
			mipCount = math.MaxUint32
		}
		layerCount := desc.ArrayLayerCount
		if layerCount == 0 {
			layerCount = math.MaxUint32
		}
		wgpuDesc = &wgpu.TextureViewDescriptor{
			Label:           wgpu.EmptyStringView(),
			Format:          desc.Format,
			Dimension:       desc.Dimension,
			BaseMipLevel:    desc.BaseMipLevel,
			MipLevelCount:   mipCount,
			BaseArrayLayer:  desc.BaseArrayLayer,
			ArrayLayerCount: layerCount,
		}
	}

	view := rt.tex.CreateView(wgpuDesc)
	if view == nil {
		return nil, fmt.Errorf("rust backend: failed to create texture view")
	}
	return &rustTextureView{view: view}, nil
}

// DestroyTextureView destroys a texture view.
func (d *rustDevice) DestroyTextureView(view hal.TextureView) {
	if rv, ok := view.(*rustTextureView); ok && rv.view != nil {
		rv.view.Release()
		rv.view = nil
	}
}

// CreateSampler creates a texture sampler.
func (d *rustDevice) CreateSampler(desc *hal.SamplerDescriptor) (hal.Sampler, error) {
	wgpuDesc := &wgpu.SamplerDescriptor{
		Label:         wgpu.EmptyStringView(),
		AddressModeU:  desc.AddressModeU,
		AddressModeV:  desc.AddressModeV,
		AddressModeW:  desc.AddressModeW,
		MagFilter:     desc.MagFilter,
		MinFilter:     desc.MinFilter,
		MipmapFilter:  gputypes.MipmapFilterMode(desc.MipmapFilter),
		LodMinClamp:   desc.LodMinClamp,
		LodMaxClamp:   desc.LodMaxClamp,
		Compare:       desc.Compare,
		MaxAnisotropy: desc.Anisotropy,
	}

	sampler := d.dev.CreateSampler(wgpuDesc)
	if sampler == nil {
		return nil, fmt.Errorf("rust backend: failed to create sampler")
	}
	return &rustSampler{sampler: sampler}, nil
}

// DestroySampler destroys a sampler.
func (d *rustDevice) DestroySampler(sampler hal.Sampler) {
	if rs, ok := sampler.(*rustSampler); ok && rs.sampler != nil {
		rs.sampler.Release()
		rs.sampler = nil
	}
}

// CreateBindGroupLayout creates a bind group layout.
func (d *rustDevice) CreateBindGroupLayout(desc *hal.BindGroupLayoutDescriptor) (hal.BindGroupLayout, error) {
	entries := make([]wgpu.BindGroupLayoutEntry, len(desc.Entries))
	for i, entry := range desc.Entries {
		wgpuEntry := wgpu.BindGroupLayoutEntry{
			Binding:    entry.Binding,
			Visibility: entry.Visibility,
		}

		if entry.Buffer != nil {
			var hasDynamicOffset wgpu.Bool
			if entry.Buffer.HasDynamicOffset {
				hasDynamicOffset = wgpu.True
			}
			wgpuEntry.Buffer = wgpu.BufferBindingLayout{
				Type:             entry.Buffer.Type,
				HasDynamicOffset: hasDynamicOffset,
				MinBindingSize:   entry.Buffer.MinBindingSize,
			}
		}

		if entry.Sampler != nil {
			wgpuEntry.Sampler = wgpu.SamplerBindingLayout{
				Type: entry.Sampler.Type,
			}
		}

		if entry.Texture != nil {
			var multisampled wgpu.Bool
			if entry.Texture.Multisampled {
				multisampled = wgpu.True
			}
			wgpuEntry.Texture = wgpu.TextureBindingLayout{
				SampleType:    entry.Texture.SampleType,
				ViewDimension: entry.Texture.ViewDimension,
				Multisampled:  multisampled,
			}
		}

		entries[i] = wgpuEntry
	}

	layout := d.dev.CreateBindGroupLayoutSimple(entries)
	if layout == nil {
		return nil, fmt.Errorf("rust backend: failed to create bind group layout")
	}
	return &rustBindGroupLayout{layout: layout}, nil
}

// DestroyBindGroupLayout destroys a bind group layout.
func (d *rustDevice) DestroyBindGroupLayout(layout hal.BindGroupLayout) {
	if rl, ok := layout.(*rustBindGroupLayout); ok && rl.layout != nil {
		rl.layout.Release()
		rl.layout = nil
	}
}

// CreateBindGroup creates a bind group.
func (d *rustDevice) CreateBindGroup(desc *hal.BindGroupDescriptor) (hal.BindGroup, error) {
	rl, ok := desc.Layout.(*rustBindGroupLayout)
	if !ok || rl.layout == nil {
		return nil, fmt.Errorf("rust backend: invalid bind group layout")
	}

	entries := make([]wgpu.BindGroupEntry, len(desc.Entries))
	for i, entry := range desc.Entries {
		entries[i] = bindGroupEntryToWGPU(entry)
	}

	bg := d.dev.CreateBindGroupSimple(rl.layout, entries)
	if bg == nil {
		return nil, fmt.Errorf("rust backend: failed to create bind group")
	}
	return &rustBindGroup{bg: bg}, nil
}

// DestroyBindGroup destroys a bind group.
func (d *rustDevice) DestroyBindGroup(group hal.BindGroup) {
	if rg, ok := group.(*rustBindGroup); ok && rg.bg != nil {
		rg.bg.Release()
		rg.bg = nil
	}
}

// CreatePipelineLayout creates a pipeline layout.
func (d *rustDevice) CreatePipelineLayout(desc *hal.PipelineLayoutDescriptor) (hal.PipelineLayout, error) {
	layouts := make([]*wgpu.BindGroupLayout, len(desc.BindGroupLayouts))
	for i, layout := range desc.BindGroupLayouts {
		rl, ok := layout.(*rustBindGroupLayout)
		if !ok || rl.layout == nil {
			return nil, fmt.Errorf("rust backend: invalid bind group layout at index %d", i)
		}
		layouts[i] = rl.layout
	}

	pl := d.dev.CreatePipelineLayoutSimple(layouts)
	if pl == nil {
		return nil, fmt.Errorf("rust backend: failed to create pipeline layout")
	}
	return &rustPipelineLayout{layout: pl}, nil
}

// DestroyPipelineLayout destroys a pipeline layout.
func (d *rustDevice) DestroyPipelineLayout(layout hal.PipelineLayout) {
	if rl, ok := layout.(*rustPipelineLayout); ok && rl.layout != nil {
		rl.layout.Release()
		rl.layout = nil
	}
}

// CreateShaderModule creates a shader module.
func (d *rustDevice) CreateShaderModule(desc *hal.ShaderModuleDescriptor) (hal.ShaderModule, error) {
	var shader *wgpu.ShaderModule

	switch {
	case desc.Source.WGSL != "":
		shader = d.dev.CreateShaderModuleWGSL(desc.Source.WGSL)
	case len(desc.Source.SPIRV) > 0:
		// Use SPIR-V via chained struct.
		// unsafe.Pointer is required for FFI interop with wgpu-native.
		spirvSource := shaderSourceSPIRV{
			Chain: wgpu.ChainedStruct{
				Next:  0,
				SType: uint32(wgpu.STypeShaderSourceSPIRV),
			},
			CodeSize: uint32(len(desc.Source.SPIRV)),                 //nolint:gosec // G115: SPIR-V size fits uint32
			Code:     uintptr(unsafe.Pointer(&desc.Source.SPIRV[0])), //nolint:gosec // G103: FFI interop
		}
		wgpuDesc := wgpu.ShaderModuleDescriptor{
			NextInChain: uintptr(unsafe.Pointer(&spirvSource)), //nolint:gosec // G103: FFI interop
			Label:       wgpu.EmptyStringView(),
		}
		shader = d.dev.CreateShaderModule(&wgpuDesc)
	default:
		return nil, fmt.Errorf("rust backend: no shader source provided")
	}

	if shader == nil {
		return nil, fmt.Errorf("rust backend: failed to create shader module")
	}
	return &rustShaderModule{module: shader}, nil
}

// shaderSourceSPIRV provides SPIR-V bytecode for shader creation.
// This matches the wgpu-native WGPUShaderSourceSPIRV chained struct layout.
type shaderSourceSPIRV struct {
	Chain    wgpu.ChainedStruct
	CodeSize uint32
	_        [4]byte // padding for alignment
	Code     uintptr // *uint32
}

// DestroyShaderModule destroys a shader module.
func (d *rustDevice) DestroyShaderModule(module hal.ShaderModule) {
	if rm, ok := module.(*rustShaderModule); ok && rm.module != nil {
		rm.module.Release()
		rm.module = nil
	}
}

// CreateRenderPipeline creates a render pipeline.
func (d *rustDevice) CreateRenderPipeline(desc *hal.RenderPipelineDescriptor) (hal.RenderPipeline, error) {
	vertModule, ok := desc.Vertex.Module.(*rustShaderModule)
	if !ok || vertModule.module == nil {
		return nil, fmt.Errorf("rust backend: invalid vertex shader module")
	}

	wgpuDesc := &wgpu.RenderPipelineDescriptor{
		Label: desc.Label,
		Vertex: wgpu.VertexState{
			Module:     vertModule.module,
			EntryPoint: desc.Vertex.EntryPoint,
		},
		Primitive: wgpu.PrimitiveState{
			Topology:  desc.Primitive.Topology,
			FrontFace: desc.Primitive.FrontFace,
			CullMode:  desc.Primitive.CullMode,
		},
		Multisample: wgpu.MultisampleState{
			Count: desc.Multisample.Count,
			Mask:  uint32(desc.Multisample.Mask), //nolint:gosec // G115: truncation is safe, mask is always 32-bit
		},
	}

	// Default multisample state
	if wgpuDesc.Multisample.Count == 0 {
		wgpuDesc.Multisample.Count = 1
	}
	if wgpuDesc.Multisample.Mask == 0 {
		wgpuDesc.Multisample.Mask = 0xFFFFFFFF
	}

	// Pipeline layout
	if desc.Layout != nil {
		if rl, ok2 := desc.Layout.(*rustPipelineLayout); ok2 {
			wgpuDesc.Layout = rl.layout
		}
	}

	// Vertex buffers
	if len(desc.Vertex.Buffers) > 0 {
		wgpuDesc.Vertex.Buffers = convertVertexBufferLayouts(desc.Vertex.Buffers)
	}

	// Fragment state
	if desc.Fragment != nil {
		fragModule, ok2 := desc.Fragment.Module.(*rustShaderModule)
		if !ok2 || fragModule.module == nil {
			return nil, fmt.Errorf("rust backend: invalid fragment shader module")
		}
		wgpuDesc.Fragment = &wgpu.FragmentState{
			Module:     fragModule.module,
			EntryPoint: desc.Fragment.EntryPoint,
			Targets:    convertColorTargetStates(desc.Fragment.Targets),
		}
	}

	// Depth/stencil state
	if desc.DepthStencil != nil {
		wgpuDesc.DepthStencil = &wgpu.DepthStencilState{
			Format:            desc.DepthStencil.Format,
			DepthWriteEnabled: desc.DepthStencil.DepthWriteEnabled,
			DepthCompare:      desc.DepthStencil.DepthCompare,
			StencilFront: wgpu.StencilFaceState{
				Compare:     desc.DepthStencil.StencilFront.Compare,
				FailOp:      halStencilOpToGPUTypes(desc.DepthStencil.StencilFront.FailOp),
				DepthFailOp: halStencilOpToGPUTypes(desc.DepthStencil.StencilFront.DepthFailOp),
				PassOp:      halStencilOpToGPUTypes(desc.DepthStencil.StencilFront.PassOp),
			},
			StencilBack: wgpu.StencilFaceState{
				Compare:     desc.DepthStencil.StencilBack.Compare,
				FailOp:      halStencilOpToGPUTypes(desc.DepthStencil.StencilBack.FailOp),
				DepthFailOp: halStencilOpToGPUTypes(desc.DepthStencil.StencilBack.DepthFailOp),
				PassOp:      halStencilOpToGPUTypes(desc.DepthStencil.StencilBack.PassOp),
			},
			StencilReadMask:     desc.DepthStencil.StencilReadMask,
			StencilWriteMask:    desc.DepthStencil.StencilWriteMask,
			DepthBias:           desc.DepthStencil.DepthBias,
			DepthBiasSlopeScale: desc.DepthStencil.DepthBiasSlopeScale,
			DepthBiasClamp:      desc.DepthStencil.DepthBiasClamp,
		}
	}

	pipeline := d.dev.CreateRenderPipeline(wgpuDesc)
	if pipeline == nil {
		return nil, fmt.Errorf("rust backend: failed to create render pipeline")
	}
	return &rustRenderPipeline{pipeline: pipeline}, nil
}

// DestroyRenderPipeline destroys a render pipeline.
func (d *rustDevice) DestroyRenderPipeline(pipeline hal.RenderPipeline) {
	if rp, ok := pipeline.(*rustRenderPipeline); ok && rp.pipeline != nil {
		rp.pipeline.Release()
		rp.pipeline = nil
	}
}

// CreateComputePipeline creates a compute pipeline.
func (d *rustDevice) CreateComputePipeline(desc *hal.ComputePipelineDescriptor) (hal.ComputePipeline, error) {
	shader, ok := desc.Compute.Module.(*rustShaderModule)
	if !ok || shader.module == nil {
		return nil, fmt.Errorf("rust backend: invalid compute shader module")
	}

	var layout *wgpu.PipelineLayout
	if desc.Layout != nil {
		if rl, ok2 := desc.Layout.(*rustPipelineLayout); ok2 {
			layout = rl.layout
		}
	}

	pipeline := d.dev.CreateComputePipelineSimple(layout, shader.module, desc.Compute.EntryPoint)
	if pipeline == nil {
		return nil, fmt.Errorf("rust backend: failed to create compute pipeline")
	}
	return &rustComputePipeline{pipeline: pipeline}, nil
}

// DestroyComputePipeline destroys a compute pipeline.
func (d *rustDevice) DestroyComputePipeline(pipeline hal.ComputePipeline) {
	if rp, ok := pipeline.(*rustComputePipeline); ok && rp.pipeline != nil {
		rp.pipeline.Release()
		rp.pipeline = nil
	}
}

// CreateQuerySet creates a query set for timestamp or occlusion queries.
func (d *rustDevice) CreateQuerySet(desc *hal.QuerySetDescriptor) (hal.QuerySet, error) {
	qs := d.dev.CreateQuerySet(&wgpu.QuerySetDescriptor{
		Type:  halQueryTypeToWGPU(desc.Type),
		Count: desc.Count,
	})
	if qs == nil {
		return nil, fmt.Errorf("rust backend: failed to create query set")
	}
	return &rustQuerySet{qs: qs}, nil
}

// DestroyQuerySet destroys a query set.
func (d *rustDevice) DestroyQuerySet(querySet hal.QuerySet) {
	if rq, ok := querySet.(*rustQuerySet); ok && rq.qs != nil {
		rq.qs.Release()
		rq.qs = nil
	}
}

// CreateCommandEncoder creates a command encoder.
func (d *rustDevice) CreateCommandEncoder(desc *hal.CommandEncoderDescriptor) (hal.CommandEncoder, error) {
	enc := d.dev.CreateCommandEncoder(nil)
	if enc == nil {
		return nil, fmt.Errorf("rust backend: failed to create command encoder")
	}
	return &rustCommandEncoder{enc: enc}, nil
}

// CreateRenderBundleEncoder creates a render bundle encoder.
func (d *rustDevice) CreateRenderBundleEncoder(desc *hal.RenderBundleEncoderDescriptor) (hal.RenderBundleEncoder, error) {
	wgpuDesc := &wgpu.RenderBundleEncoderDescriptor{
		Label:              wgpu.EmptyStringView(),
		DepthStencilFormat: desc.DepthStencilFormat,
		SampleCount:        desc.SampleCount,
	}

	if desc.DepthReadOnly {
		wgpuDesc.DepthReadOnly = wgpu.True
	}
	if desc.StencilReadOnly {
		wgpuDesc.StencilReadOnly = wgpu.True
	}

	if len(desc.ColorFormats) > 0 {
		wgpuDesc.ColorFormatCount = uintptr(len(desc.ColorFormats))
		wgpuDesc.ColorFormats = &desc.ColorFormats[0]
	}

	rbe := d.dev.CreateRenderBundleEncoder(wgpuDesc)
	if rbe == nil {
		return nil, fmt.Errorf("rust backend: failed to create render bundle encoder")
	}
	return &rustRenderBundleEncoder{enc: rbe}, nil
}

// DestroyRenderBundle destroys a render bundle.
func (d *rustDevice) DestroyRenderBundle(bundle hal.RenderBundle) {
	if rb, ok := bundle.(*rustRenderBundle); ok && rb.bundle != nil {
		rb.bundle.Release()
		rb.bundle = nil
	}
}

// FreeCommandBuffer returns a command buffer to the command pool.
func (d *rustDevice) FreeCommandBuffer(cmdBuffer hal.CommandBuffer) {
	if rcb, ok := cmdBuffer.(*rustCommandBuffer); ok && rcb.buf != nil {
		rcb.buf.Release()
		rcb.buf = nil
	}
}

// CreateFence creates a synchronization fence.
// wgpu-native uses device.poll() for synchronization, not explicit fences.
// This returns a stub fence.
func (d *rustDevice) CreateFence() (hal.Fence, error) {
	return &rustFence{}, nil
}

// DestroyFence destroys a fence.
func (d *rustDevice) DestroyFence(fence hal.Fence) {
	// No-op: wgpu-native uses device.poll() for synchronization.
}

// Wait waits for a fence to reach the specified value.
// wgpu-native uses device.poll(wait=true) for synchronization.
func (d *rustDevice) Wait(fence hal.Fence, value uint64, timeout time.Duration) (bool, error) {
	_ = value
	_ = timeout
	// Block until all GPU work completes via device polling.
	d.dev.Poll(true)
	return true, nil
}

// ResetFence resets a fence to the unsignaled state.
func (d *rustDevice) ResetFence(fence hal.Fence) error {
	// No-op: wgpu-native uses device.poll() for synchronization.
	return nil
}

// GetFenceStatus returns true if the fence is signaled (non-blocking).
func (d *rustDevice) GetFenceStatus(fence hal.Fence) (bool, error) {
	// Poll without blocking. Returns true when queue is empty.
	d.dev.Poll(false)
	return true, nil
}

// WaitIdle waits for all GPU work to complete.
// Uses device.poll(wait=true) to block until the GPU is idle.
func (d *rustDevice) WaitIdle() error {
	if d.dev != nil {
		d.dev.Poll(true)
	}
	return nil
}

// Destroy releases the device.
func (d *rustDevice) Destroy() {
	if d.dev != nil {
		d.dev.Release()
		d.dev = nil
	}
}

// ---------------------------------------------------------------------------
// Queue — implements hal.Queue
// ---------------------------------------------------------------------------

// rustQueue wraps wgpu.Queue to implement hal.Queue.
type rustQueue struct {
	q   *wgpu.Queue
	dev *wgpu.Device
}

// Submit submits command buffers to the GPU.
func (q *rustQueue) Submit(commandBuffers []hal.CommandBuffer, fence hal.Fence, fenceValue uint64) error {
	cmds := make([]*wgpu.CommandBuffer, 0, len(commandBuffers))
	for _, cb := range commandBuffers {
		if rcb, ok := cb.(*rustCommandBuffer); ok && rcb.buf != nil {
			cmds = append(cmds, rcb.buf)
		}
	}
	if len(cmds) > 0 {
		q.q.Submit(cmds...)
	}
	return nil
}

// WriteBuffer writes data to a buffer immediately.
func (q *rustQueue) WriteBuffer(buffer hal.Buffer, offset uint64, data []byte) error {
	rb, ok := buffer.(*rustBuffer)
	if !ok || rb.buf == nil {
		return fmt.Errorf("rust backend: invalid buffer for WriteBuffer")
	}
	q.q.WriteBuffer(rb.buf, offset, data)
	return nil
}

// ReadBuffer reads data from a GPU buffer.
// Uses MapAsync + GetMappedRange + copy + Unmap internally.
func (q *rustQueue) ReadBuffer(buffer hal.Buffer, offset uint64, data []byte) error {
	rb, ok := buffer.(*rustBuffer)
	if !ok || rb.buf == nil {
		return fmt.Errorf("rust backend: invalid buffer")
	}

	size := uint64(len(data))
	if size == 0 {
		return nil
	}

	// Map the buffer for reading (blocks via device polling)
	if err := rb.buf.MapAsync(q.dev, wgpu.MapModeRead, offset, size); err != nil {
		return fmt.Errorf("rust backend: map buffer: %w", err)
	}

	// Get pointer to mapped data
	ptr := rb.buf.GetMappedRange(offset, size)
	if ptr == nil {
		rb.buf.Unmap()
		return fmt.Errorf("rust backend: failed to get mapped range")
	}

	// Copy data to caller's slice.
	// unsafe.Slice is required to access the GPU-mapped memory region.
	copy(data, unsafe.Slice((*byte)(ptr), size)) //nolint:gosec // G103: required for GPU buffer readback

	// Unmap immediately
	rb.buf.Unmap()

	return nil
}

// WriteTexture writes data to a texture immediately.
func (q *rustQueue) WriteTexture(dst *hal.ImageCopyTexture, data []byte, layout *hal.ImageDataLayout, size *hal.Extent3D) error {
	rt, ok := dst.Texture.(*rustTexture)
	if !ok || rt.tex == nil {
		return fmt.Errorf("rust backend: invalid texture for WriteTexture")
	}

	wgpuDst := &wgpu.TexelCopyTextureInfo{
		Texture:  rt.tex.Handle(),
		MipLevel: dst.MipLevel,
		Origin: gputypes.Origin3D{
			X: dst.Origin.X,
			Y: dst.Origin.Y,
			Z: dst.Origin.Z,
		},
		Aspect: wgpu.TextureAspect(dst.Aspect),
	}

	wgpuLayout := &wgpu.TexelCopyBufferLayout{
		Offset:       layout.Offset,
		BytesPerRow:  layout.BytesPerRow,
		RowsPerImage: layout.RowsPerImage,
	}

	wgpuSize := &gputypes.Extent3D{
		Width:              size.Width,
		Height:             size.Height,
		DepthOrArrayLayers: size.DepthOrArrayLayers,
	}

	q.q.WriteTexture(wgpuDst, data, wgpuLayout, wgpuSize)
	return nil
}

// Present presents a surface texture to the screen.
// In wgpu-native, presentation is done via Surface.Present, not Queue.Present.
func (q *rustQueue) Present(surface hal.Surface, texture hal.SurfaceTexture) error {
	rs, ok := surface.(*rustSurface)
	if !ok || rs.surf == nil {
		return fmt.Errorf("rust backend: invalid surface")
	}
	rs.surf.Present()
	return nil
}

// GetTimestampPeriod returns the timestamp period in nanoseconds.
func (q *rustQueue) GetTimestampPeriod() float32 {
	// wgpu-native typically returns 1.0 for Vulkan (timestamps in nanoseconds).
	return 1.0
}

// ---------------------------------------------------------------------------
// Surface — implements hal.Surface
// ---------------------------------------------------------------------------

// rustSurface wraps wgpu.Surface to implement hal.Surface.
type rustSurface struct {
	surf *wgpu.Surface
}

// Configure configures the surface with the given device and settings.
func (s *rustSurface) Configure(device hal.Device, config *hal.SurfaceConfiguration) error {
	rd, ok := device.(*rustDevice)
	if !ok || rd.dev == nil {
		return fmt.Errorf("rust backend: invalid device")
	}

	if config.Width == 0 || config.Height == 0 {
		return hal.ErrZeroArea
	}

	s.surf.Configure(&wgpu.SurfaceConfiguration{
		Device:      rd.dev,
		Format:      config.Format,
		Usage:       config.Usage,
		Width:       config.Width,
		Height:      config.Height,
		AlphaMode:   config.AlphaMode,
		PresentMode: config.PresentMode,
	})
	return nil
}

// Unconfigure removes the surface configuration.
func (s *rustSurface) Unconfigure(device hal.Device) {
	s.surf.Unconfigure()
}

// AcquireTexture acquires the next surface texture for rendering.
func (s *rustSurface) AcquireTexture(fence hal.Fence) (*hal.AcquiredSurfaceTexture, error) {
	surfTex, err := s.surf.GetCurrentTexture()
	if err != nil {
		return nil, convertSurfaceError(err)
	}

	suboptimal := surfTex.Status == wgpu.SurfaceGetCurrentTextureStatusSuccessSuboptimal

	return &hal.AcquiredSurfaceTexture{
		Texture:    &rustSurfaceTexture{tex: surfTex.Texture},
		Suboptimal: suboptimal,
	}, nil
}

// DiscardTexture discards a surface texture without presenting it.
func (s *rustSurface) DiscardTexture(texture hal.SurfaceTexture) {
	// wgpu-native: surface textures are automatically discarded if not presented.
	// Release the texture reference.
	if rst, ok := texture.(*rustSurfaceTexture); ok && rst.tex != nil {
		rst.tex.Release()
		rst.tex = nil
	}
}

// Destroy releases the surface.
func (s *rustSurface) Destroy() {
	if s.surf != nil {
		s.surf.Release()
		s.surf = nil
	}
}

// ---------------------------------------------------------------------------
// Command Encoder — implements hal.CommandEncoder
// ---------------------------------------------------------------------------

// rustCommandEncoder wraps wgpu.CommandEncoder to implement hal.CommandEncoder.
type rustCommandEncoder struct {
	enc *wgpu.CommandEncoder
}

// BeginEncoding begins command recording.
// wgpu-native encoding starts at creation, so this is a no-op.
func (e *rustCommandEncoder) BeginEncoding(label string) error {
	return nil
}

// EndEncoding finishes command recording and returns a command buffer.
func (e *rustCommandEncoder) EndEncoding() (hal.CommandBuffer, error) {
	if e.enc == nil {
		return nil, fmt.Errorf("rust backend: encoder is nil")
	}
	buf := e.enc.Finish(nil)
	if buf == nil {
		return nil, fmt.Errorf("rust backend: failed to finish command encoder")
	}
	return &rustCommandBuffer{buf: buf}, nil
}

// DiscardEncoding discards the encoder without creating a command buffer.
func (e *rustCommandEncoder) DiscardEncoding() {
	if e.enc != nil {
		e.enc.Release()
		e.enc = nil
	}
}

// ResetAll resets command buffers for reuse.
// wgpu-native handles command buffer lifecycle automatically.
func (e *rustCommandEncoder) ResetAll(commandBuffers []hal.CommandBuffer) {
	// No-op: wgpu-native manages command buffer memory internally.
}

// TransitionBuffers transitions buffer states for synchronization.
// No-op for wgpu-native — it handles barriers internally.
func (e *rustCommandEncoder) TransitionBuffers(barriers []hal.BufferBarrier) {
	// No-op: wgpu-native tracks resource state automatically.
}

// TransitionTextures transitions texture states for synchronization.
// No-op for wgpu-native — it handles barriers internally.
func (e *rustCommandEncoder) TransitionTextures(barriers []hal.TextureBarrier) {
	// No-op: wgpu-native tracks resource state automatically.
}

// ClearBuffer clears a buffer region to zero.
func (e *rustCommandEncoder) ClearBuffer(buffer hal.Buffer, offset, size uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		e.enc.ClearBuffer(rb.buf, offset, size)
	}
}

// CopyBufferToBuffer copies data between buffers.
func (e *rustCommandEncoder) CopyBufferToBuffer(src, dst hal.Buffer, regions []hal.BufferCopy) {
	srcBuf, ok1 := src.(*rustBuffer)
	dstBuf, ok2 := dst.(*rustBuffer)
	if !ok1 || !ok2 || srcBuf.buf == nil || dstBuf.buf == nil {
		return
	}
	for _, r := range regions {
		e.enc.CopyBufferToBuffer(srcBuf.buf, r.SrcOffset, dstBuf.buf, r.DstOffset, r.Size)
	}
}

// CopyBufferToTexture copies data from a buffer to a texture.
func (e *rustCommandEncoder) CopyBufferToTexture(src hal.Buffer, dst hal.Texture, regions []hal.BufferTextureCopy) {
	srcBuf, ok1 := src.(*rustBuffer)
	dstTex, ok2 := dst.(*rustTexture)
	if !ok1 || !ok2 || srcBuf.buf == nil || dstTex.tex == nil {
		return
	}
	for _, r := range regions {
		bufInfo := &wgpu.TexelCopyBufferInfo{
			Layout: wgpu.TexelCopyBufferLayout{
				Offset:       r.BufferLayout.Offset,
				BytesPerRow:  r.BufferLayout.BytesPerRow,
				RowsPerImage: r.BufferLayout.RowsPerImage,
			},
			Buffer: srcBuf.buf.Handle(),
		}
		texInfo := &wgpu.TexelCopyTextureInfo{
			Texture:  dstTex.tex.Handle(),
			MipLevel: r.TextureBase.MipLevel,
			Origin: gputypes.Origin3D{
				X: r.TextureBase.Origin.X,
				Y: r.TextureBase.Origin.Y,
				Z: r.TextureBase.Origin.Z,
			},
			Aspect: wgpu.TextureAspect(r.TextureBase.Aspect),
		}
		size := &gputypes.Extent3D{
			Width:              r.Size.Width,
			Height:             r.Size.Height,
			DepthOrArrayLayers: r.Size.DepthOrArrayLayers,
		}
		e.enc.CopyBufferToTexture(bufInfo, texInfo, size)
	}
}

// CopyTextureToBuffer copies data from a texture to a buffer.
func (e *rustCommandEncoder) CopyTextureToBuffer(src hal.Texture, dst hal.Buffer, regions []hal.BufferTextureCopy) {
	srcTex, ok1 := src.(*rustTexture)
	dstBuf, ok2 := dst.(*rustBuffer)
	if !ok1 || !ok2 || srcTex.tex == nil || dstBuf.buf == nil {
		return
	}
	for _, r := range regions {
		texInfo := &wgpu.TexelCopyTextureInfo{
			Texture:  srcTex.tex.Handle(),
			MipLevel: r.TextureBase.MipLevel,
			Origin: gputypes.Origin3D{
				X: r.TextureBase.Origin.X,
				Y: r.TextureBase.Origin.Y,
				Z: r.TextureBase.Origin.Z,
			},
			Aspect: wgpu.TextureAspect(r.TextureBase.Aspect),
		}
		bufInfo := &wgpu.TexelCopyBufferInfo{
			Layout: wgpu.TexelCopyBufferLayout{
				Offset:       r.BufferLayout.Offset,
				BytesPerRow:  r.BufferLayout.BytesPerRow,
				RowsPerImage: r.BufferLayout.RowsPerImage,
			},
			Buffer: dstBuf.buf.Handle(),
		}
		size := &gputypes.Extent3D{
			Width:              r.Size.Width,
			Height:             r.Size.Height,
			DepthOrArrayLayers: r.Size.DepthOrArrayLayers,
		}
		e.enc.CopyTextureToBuffer(texInfo, bufInfo, size)
	}
}

// CopyTextureToTexture copies data between textures.
func (e *rustCommandEncoder) CopyTextureToTexture(src, dst hal.Texture, regions []hal.TextureCopy) {
	srcTex, ok1 := src.(*rustTexture)
	dstTex, ok2 := dst.(*rustTexture)
	if !ok1 || !ok2 || srcTex.tex == nil || dstTex.tex == nil {
		return
	}
	for _, r := range regions {
		srcInfo := &wgpu.TexelCopyTextureInfo{
			Texture:  srcTex.tex.Handle(),
			MipLevel: r.SrcBase.MipLevel,
			Origin: gputypes.Origin3D{
				X: r.SrcBase.Origin.X,
				Y: r.SrcBase.Origin.Y,
				Z: r.SrcBase.Origin.Z,
			},
			Aspect: wgpu.TextureAspect(r.SrcBase.Aspect),
		}
		dstInfo := &wgpu.TexelCopyTextureInfo{
			Texture:  dstTex.tex.Handle(),
			MipLevel: r.DstBase.MipLevel,
			Origin: gputypes.Origin3D{
				X: r.DstBase.Origin.X,
				Y: r.DstBase.Origin.Y,
				Z: r.DstBase.Origin.Z,
			},
			Aspect: wgpu.TextureAspect(r.DstBase.Aspect),
		}
		size := &gputypes.Extent3D{
			Width:              r.Size.Width,
			Height:             r.Size.Height,
			DepthOrArrayLayers: r.Size.DepthOrArrayLayers,
		}
		e.enc.CopyTextureToTexture(srcInfo, dstInfo, size)
	}
}

// ResolveQuerySet copies query results from a query set into a buffer.
func (e *rustCommandEncoder) ResolveQuerySet(querySet hal.QuerySet, firstQuery, queryCount uint32, destination hal.Buffer, destinationOffset uint64) {
	rq, ok1 := querySet.(*rustQuerySet)
	rb, ok2 := destination.(*rustBuffer)
	if !ok1 || !ok2 || rq.qs == nil || rb.buf == nil {
		return
	}
	e.enc.ResolveQuerySet(rq.qs, firstQuery, queryCount, rb.buf, destinationOffset)
}

// BeginRenderPass begins a render pass.
func (e *rustCommandEncoder) BeginRenderPass(desc *hal.RenderPassDescriptor) hal.RenderPassEncoder {
	attachments := make([]wgpu.RenderPassColorAttachment, len(desc.ColorAttachments))
	for i, att := range desc.ColorAttachments {
		var view *wgpu.TextureView
		if rv, ok := att.View.(*rustTextureView); ok && rv.view != nil {
			view = rv.view
		}

		var resolveTarget *wgpu.TextureView
		if att.ResolveTarget != nil {
			if rv, ok := att.ResolveTarget.(*rustTextureView); ok && rv.view != nil {
				resolveTarget = rv.view
			}
		}

		attachments[i] = wgpu.RenderPassColorAttachment{
			View:          view,
			ResolveTarget: resolveTarget,
			LoadOp:        att.LoadOp,
			StoreOp:       att.StoreOp,
			ClearValue: wgpu.Color{
				R: att.ClearValue.R,
				G: att.ClearValue.G,
				B: att.ClearValue.B,
				A: att.ClearValue.A,
			},
		}
	}

	wgpuDesc := &wgpu.RenderPassDescriptor{
		ColorAttachments: attachments,
	}

	// Depth/stencil attachment
	if desc.DepthStencilAttachment != nil {
		dsView, ok := desc.DepthStencilAttachment.View.(*rustTextureView)
		if ok && dsView.view != nil {
			wgpuDesc.DepthStencilAttachment = &wgpu.RenderPassDepthStencilAttachment{
				View:              dsView.view,
				DepthLoadOp:       desc.DepthStencilAttachment.DepthLoadOp,
				DepthStoreOp:      desc.DepthStencilAttachment.DepthStoreOp,
				DepthClearValue:   desc.DepthStencilAttachment.DepthClearValue,
				DepthReadOnly:     desc.DepthStencilAttachment.DepthReadOnly,
				StencilLoadOp:     desc.DepthStencilAttachment.StencilLoadOp,
				StencilStoreOp:    desc.DepthStencilAttachment.StencilStoreOp,
				StencilClearValue: desc.DepthStencilAttachment.StencilClearValue,
				StencilReadOnly:   desc.DepthStencilAttachment.StencilReadOnly,
			}
		}
	}

	pass := e.enc.BeginRenderPass(wgpuDesc)
	if pass == nil {
		return nil
	}
	return &rustRenderPass{pass: pass}
}

// BeginComputePass begins a compute pass.
func (e *rustCommandEncoder) BeginComputePass(desc *hal.ComputePassDescriptor) hal.ComputePassEncoder {
	pass := e.enc.BeginComputePass(nil)
	if pass == nil {
		return nil
	}
	return &rustComputePass{pass: pass}
}

// ---------------------------------------------------------------------------
// Render Pass Encoder — implements hal.RenderPassEncoder
// ---------------------------------------------------------------------------

// rustRenderPass wraps wgpu.RenderPassEncoder to implement hal.RenderPassEncoder.
type rustRenderPass struct {
	pass *wgpu.RenderPassEncoder
}

// End finishes the render pass.
func (p *rustRenderPass) End() {
	if p.pass != nil {
		p.pass.End()
	}
}

// SetPipeline sets the active render pipeline.
func (p *rustRenderPass) SetPipeline(pipeline hal.RenderPipeline) {
	if rp, ok := pipeline.(*rustRenderPipeline); ok && rp.pipeline != nil {
		p.pass.SetPipeline(rp.pipeline)
	}
}

// SetBindGroup sets a bind group for the given index.
func (p *rustRenderPass) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	if rg, ok := group.(*rustBindGroup); ok && rg.bg != nil {
		p.pass.SetBindGroup(index, rg.bg, offsets)
	}
}

// SetVertexBuffer sets a vertex buffer for the given slot.
func (p *rustRenderPass) SetVertexBuffer(slot uint32, buffer hal.Buffer, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		// wgpu-native uses math.MaxUint64 (WGPU_WHOLE_SIZE) for "remaining buffer".
		p.pass.SetVertexBuffer(slot, rb.buf, offset, math.MaxUint64)
	}
}

// SetIndexBuffer sets the index buffer.
func (p *rustRenderPass) SetIndexBuffer(buffer hal.Buffer, format gputypes.IndexFormat, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		// wgpu-native uses math.MaxUint64 (WGPU_WHOLE_SIZE) for "remaining buffer".
		p.pass.SetIndexBuffer(rb.buf, format, offset, math.MaxUint64)
	}
}

// SetViewport sets the viewport transformation.
func (p *rustRenderPass) SetViewport(x, y, width, height, minDepth, maxDepth float32) {
	p.pass.SetViewport(x, y, width, height, minDepth, maxDepth)
}

// SetScissorRect sets the scissor rectangle for clipping.
func (p *rustRenderPass) SetScissorRect(x, y, width, height uint32) {
	p.pass.SetScissorRect(x, y, width, height)
}

// SetBlendConstant sets the blend constant color.
func (p *rustRenderPass) SetBlendConstant(color *gputypes.Color) {
	if color == nil {
		return
	}
	wgpuColor := &wgpu.Color{R: color.R, G: color.G, B: color.B, A: color.A}
	p.pass.SetBlendConstant(wgpuColor)
}

// SetStencilReference sets the stencil reference value.
func (p *rustRenderPass) SetStencilReference(reference uint32) {
	p.pass.SetStencilReference(reference)
}

// Draw draws primitives.
func (p *rustRenderPass) Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	p.pass.Draw(vertexCount, instanceCount, firstVertex, firstInstance)
}

// DrawIndexed draws indexed primitives.
func (p *rustRenderPass) DrawIndexed(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	p.pass.DrawIndexed(indexCount, instanceCount, firstIndex, baseVertex, firstInstance)
}

// DrawIndirect draws primitives with GPU-generated parameters.
func (p *rustRenderPass) DrawIndirect(buffer hal.Buffer, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		p.pass.DrawIndirect(rb.buf, offset)
	}
}

// DrawIndexedIndirect draws indexed primitives with GPU-generated parameters.
func (p *rustRenderPass) DrawIndexedIndirect(buffer hal.Buffer, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		p.pass.DrawIndexedIndirect(rb.buf, offset)
	}
}

// ExecuteBundle executes a pre-recorded render bundle.
func (p *rustRenderPass) ExecuteBundle(bundle hal.RenderBundle) {
	if rb, ok := bundle.(*rustRenderBundle); ok && rb.bundle != nil {
		p.pass.ExecuteBundles([]*wgpu.RenderBundle{rb.bundle})
	}
}

// ---------------------------------------------------------------------------
// Compute Pass Encoder — implements hal.ComputePassEncoder
// ---------------------------------------------------------------------------

// rustComputePass wraps wgpu.ComputePassEncoder to implement hal.ComputePassEncoder.
type rustComputePass struct {
	pass *wgpu.ComputePassEncoder
}

// End finishes the compute pass.
func (p *rustComputePass) End() {
	if p.pass != nil {
		p.pass.End()
	}
}

// SetPipeline sets the active compute pipeline.
func (p *rustComputePass) SetPipeline(pipeline hal.ComputePipeline) {
	if rp, ok := pipeline.(*rustComputePipeline); ok && rp.pipeline != nil {
		p.pass.SetPipeline(rp.pipeline)
	}
}

// SetBindGroup sets a bind group for the given index.
func (p *rustComputePass) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	if rg, ok := group.(*rustBindGroup); ok && rg.bg != nil {
		p.pass.SetBindGroup(index, rg.bg, offsets)
	}
}

// Dispatch dispatches compute work.
func (p *rustComputePass) Dispatch(x, y, z uint32) {
	p.pass.DispatchWorkgroups(x, y, z)
}

// DispatchIndirect dispatches compute work with GPU-generated parameters.
func (p *rustComputePass) DispatchIndirect(buffer hal.Buffer, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		p.pass.DispatchWorkgroupsIndirect(rb.buf, offset)
	}
}

// ---------------------------------------------------------------------------
// Render Bundle Encoder — implements hal.RenderBundleEncoder
// ---------------------------------------------------------------------------

// rustRenderBundleEncoder wraps wgpu.RenderBundleEncoder to implement hal.RenderBundleEncoder.
type rustRenderBundleEncoder struct {
	enc *wgpu.RenderBundleEncoder
}

// SetPipeline sets the active render pipeline.
func (e *rustRenderBundleEncoder) SetPipeline(pipeline hal.RenderPipeline) {
	if rp, ok := pipeline.(*rustRenderPipeline); ok && rp.pipeline != nil {
		e.enc.SetPipeline(rp.pipeline)
	}
}

// SetBindGroup sets a bind group for the given index.
func (e *rustRenderBundleEncoder) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	if rg, ok := group.(*rustBindGroup); ok && rg.bg != nil {
		e.enc.SetBindGroup(index, rg.bg, offsets)
	}
}

// SetVertexBuffer sets a vertex buffer for the given slot.
func (e *rustRenderBundleEncoder) SetVertexBuffer(slot uint32, buffer hal.Buffer, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		e.enc.SetVertexBuffer(slot, rb.buf, offset, math.MaxUint64)
	}
}

// SetIndexBuffer sets the index buffer.
func (e *rustRenderBundleEncoder) SetIndexBuffer(buffer hal.Buffer, format gputypes.IndexFormat, offset uint64) {
	if rb, ok := buffer.(*rustBuffer); ok && rb.buf != nil {
		e.enc.SetIndexBuffer(rb.buf, format, offset, math.MaxUint64)
	}
}

// Draw draws primitives.
func (e *rustRenderBundleEncoder) Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	e.enc.Draw(vertexCount, instanceCount, firstVertex, firstInstance)
}

// DrawIndexed draws indexed primitives.
func (e *rustRenderBundleEncoder) DrawIndexed(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	e.enc.DrawIndexed(indexCount, instanceCount, firstIndex, baseVertex, firstInstance)
}

// Finish finalizes the bundle and returns it.
func (e *rustRenderBundleEncoder) Finish() hal.RenderBundle {
	if e.enc == nil {
		return nil
	}
	bundle := e.enc.Finish(nil)
	if bundle == nil {
		return nil
	}
	return &rustRenderBundle{bundle: bundle}
}

// ---------------------------------------------------------------------------
// Resource wrappers — implement hal resource interfaces
// ---------------------------------------------------------------------------

// rustBuffer wraps wgpu.Buffer to implement hal.Buffer.
type rustBuffer struct {
	buf *wgpu.Buffer
}

func (b *rustBuffer) Destroy()              { b.buf.Release() }
func (b *rustBuffer) NativeHandle() uintptr { return b.buf.Handle() }

// rustTexture wraps wgpu.Texture to implement hal.Texture.
type rustTexture struct {
	tex *wgpu.Texture
}

func (t *rustTexture) Destroy()              { t.tex.Release() }
func (t *rustTexture) NativeHandle() uintptr { return t.tex.Handle() }

// rustSurfaceTexture wraps wgpu.Texture acquired from a surface.
// Implements hal.SurfaceTexture (extends hal.Texture).
type rustSurfaceTexture struct {
	tex *wgpu.Texture
}

func (t *rustSurfaceTexture) Destroy()              { t.tex.Release() }
func (t *rustSurfaceTexture) NativeHandle() uintptr { return t.tex.Handle() }

// rustTextureView wraps wgpu.TextureView to implement hal.TextureView.
type rustTextureView struct {
	view *wgpu.TextureView
}

func (v *rustTextureView) Destroy()              { v.view.Release() }
func (v *rustTextureView) NativeHandle() uintptr { return v.view.Handle() }

// rustSampler wraps wgpu.Sampler to implement hal.Sampler.
type rustSampler struct {
	sampler *wgpu.Sampler
}

func (s *rustSampler) Destroy()              { s.sampler.Release() }
func (s *rustSampler) NativeHandle() uintptr { return s.sampler.Handle() }

// rustShaderModule wraps wgpu.ShaderModule to implement hal.ShaderModule.
type rustShaderModule struct {
	module *wgpu.ShaderModule
}

func (m *rustShaderModule) Destroy() { m.module.Release() }

// rustBindGroupLayout wraps wgpu.BindGroupLayout to implement hal.BindGroupLayout.
type rustBindGroupLayout struct {
	layout *wgpu.BindGroupLayout
}

func (l *rustBindGroupLayout) Destroy() { l.layout.Release() }

// rustBindGroup wraps wgpu.BindGroup to implement hal.BindGroup.
type rustBindGroup struct {
	bg *wgpu.BindGroup
}

func (g *rustBindGroup) Destroy() { g.bg.Release() }

// rustPipelineLayout wraps wgpu.PipelineLayout to implement hal.PipelineLayout.
type rustPipelineLayout struct {
	layout *wgpu.PipelineLayout
}

func (l *rustPipelineLayout) Destroy() { l.layout.Release() }

// rustRenderPipeline wraps wgpu.RenderPipeline to implement hal.RenderPipeline.
type rustRenderPipeline struct {
	pipeline *wgpu.RenderPipeline
}

func (p *rustRenderPipeline) Destroy() { p.pipeline.Release() }

// rustComputePipeline wraps wgpu.ComputePipeline to implement hal.ComputePipeline.
type rustComputePipeline struct {
	pipeline *wgpu.ComputePipeline
}

func (p *rustComputePipeline) Destroy() { p.pipeline.Release() }

// rustQuerySet wraps wgpu.QuerySet to implement hal.QuerySet.
type rustQuerySet struct {
	qs *wgpu.QuerySet
}

func (q *rustQuerySet) Destroy() { q.qs.Release() }

// rustCommandBuffer wraps wgpu.CommandBuffer to implement hal.CommandBuffer.
type rustCommandBuffer struct {
	buf *wgpu.CommandBuffer
}

func (b *rustCommandBuffer) Destroy() { b.buf.Release() }

// rustFence is a stub fence — wgpu-native uses device.poll() for synchronization.
type rustFence struct{}

func (f *rustFence) Destroy() {}

// rustRenderBundle wraps wgpu.RenderBundle to implement hal.RenderBundle.
type rustRenderBundle struct {
	bundle *wgpu.RenderBundle
}

func (b *rustRenderBundle) Destroy() { b.bundle.Release() }

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// halStencilOpToGPUTypes converts hal.StencilOperation (uint8, iota from 0)
// to gputypes.StencilOperation (uint32, WebGPU spec values starting at 1).
// HAL: Keep=0, Zero=1, ..., DecrementWrap=7
// gputypes: Undefined=0, Keep=1, Zero=2, ..., DecrementWrap=8
func halStencilOpToGPUTypes(op hal.StencilOperation) gputypes.StencilOperation {
	return gputypes.StencilOperation(op + 1)
}

// halQueryTypeToWGPU converts hal.QueryType (iota from 0) to wgpu.QueryType (WebGPU spec values).
// HAL: Occlusion=0, Timestamp=1
// wgpu: Occlusion=0x1, Timestamp=0x2
func halQueryTypeToWGPU(qt hal.QueryType) wgpu.QueryType {
	switch qt {
	case hal.QueryTypeOcclusion:
		return wgpu.QueryTypeOcclusion
	case hal.QueryTypeTimestamp:
		return wgpu.QueryTypeTimestamp
	default:
		return wgpu.QueryTypeOcclusion
	}
}

// bindGroupEntryToWGPU converts a hal BindGroupEntry to wgpu BindGroupEntry.
// HAL uses gputypes.BindGroupEntry with ResourceBinding interfaces.
func bindGroupEntryToWGPU(entry gputypes.BindGroupEntry) wgpu.BindGroupEntry {
	wgpuEntry := wgpu.BindGroupEntry{
		Binding: entry.Binding,
	}

	// Extract resource binding based on type
	switch r := entry.Resource.(type) {
	case gputypes.BufferBinding:
		wgpuEntry.Buffer = r.Buffer
		wgpuEntry.Offset = r.Offset
		wgpuEntry.Size = r.Size
	case gputypes.SamplerBinding:
		wgpuEntry.Sampler = r.Sampler
	case gputypes.TextureViewBinding:
		wgpuEntry.TextureView = r.TextureView
	}

	return wgpuEntry
}

// convertVertexBufferLayouts converts gputypes.VertexBufferLayout to wgpu.VertexBufferLayout.
// The key difference is that wgpu uses pointer+count (FFI layout) while gputypes uses slices.
func convertVertexBufferLayouts(layouts []gputypes.VertexBufferLayout) []wgpu.VertexBufferLayout {
	result := make([]wgpu.VertexBufferLayout, len(layouts))
	for i, layout := range layouts {
		var attrs *wgpu.VertexAttribute
		var attrCount uintptr
		if len(layout.Attributes) > 0 {
			wgpuAttrs := make([]wgpu.VertexAttribute, len(layout.Attributes))
			for j, attr := range layout.Attributes {
				wgpuAttrs[j] = wgpu.VertexAttribute{
					Format:         attr.Format,
					Offset:         attr.Offset,
					ShaderLocation: attr.ShaderLocation,
				}
			}
			attrs = &wgpuAttrs[0]
			attrCount = uintptr(len(wgpuAttrs))
		}
		result[i] = wgpu.VertexBufferLayout{
			ArrayStride:    layout.ArrayStride,
			StepMode:       layout.StepMode,
			AttributeCount: attrCount,
			Attributes:     attrs,
		}
	}
	return result
}

// convertColorTargetStates converts gputypes.ColorTargetState to wgpu.ColorTargetState.
// The key difference is the BlendState type and BlendComponent field ordering.
func convertColorTargetStates(targets []gputypes.ColorTargetState) []wgpu.ColorTargetState {
	result := make([]wgpu.ColorTargetState, len(targets))
	for i, target := range targets {
		result[i] = wgpu.ColorTargetState{
			Format:    target.Format,
			WriteMask: target.WriteMask,
		}
		if target.Blend != nil {
			result[i].Blend = &wgpu.BlendState{
				Color: wgpu.BlendComponent{
					Operation: target.Blend.Color.Operation,
					SrcFactor: target.Blend.Color.SrcFactor,
					DstFactor: target.Blend.Color.DstFactor,
				},
				Alpha: wgpu.BlendComponent{
					Operation: target.Blend.Alpha.Operation,
					SrcFactor: target.Blend.Alpha.SrcFactor,
					DstFactor: target.Blend.Alpha.DstFactor,
				},
			}
		}
	}
	return result
}

// convertSurfaceError converts wgpu surface errors to hal errors.
func convertSurfaceError(err error) error {
	if err == nil {
		return nil
	}

	// Check for specific surface error types using errors.Is for wrapped error support.
	switch {
	case errors.Is(err, wgpu.ErrSurfaceNeedsReconfigure):
		return hal.ErrSurfaceOutdated
	case errors.Is(err, wgpu.ErrSurfaceLost):
		return hal.ErrSurfaceLost
	case errors.Is(err, wgpu.ErrSurfaceTimeout):
		return hal.ErrTimeout
	case errors.Is(err, wgpu.ErrSurfaceOutOfMemory):
		return hal.ErrDeviceOutOfMemory
	case errors.Is(err, wgpu.ErrSurfaceDeviceLost):
		return hal.ErrDeviceLost
	default:
		return fmt.Errorf("rust backend: surface error: %w", err)
	}
}

// ---------------------------------------------------------------------------
// Public API — used by init.go for HAL backend registration
// ---------------------------------------------------------------------------

// IsAvailable returns true on platforms where go-webgpu/goffi is supported
// (Windows, macOS, Linux).
func IsAvailable() bool {
	return true
}

// NewHalBackend returns the Rust HAL backend.
func NewHalBackend() hal.Backend { return rustBackend{} }

// HalBackendName returns the human-readable backend name.
func HalBackendName() string { return "Rust (wgpu-native)" }

// HalBackendVariant returns the backend variant for instance creation.
func HalBackendVariant() gputypes.Backend { return platformVariant() }

// ---------------------------------------------------------------------------
// Interface compliance checks
// ---------------------------------------------------------------------------

var (
	_ hal.Backend             = rustBackend{}
	_ hal.Instance            = (*rustInstance)(nil)
	_ hal.Adapter             = (*rustAdapter)(nil)
	_ hal.Device              = (*rustDevice)(nil)
	_ hal.Queue               = (*rustQueue)(nil)
	_ hal.Surface             = (*rustSurface)(nil)
	_ hal.CommandEncoder      = (*rustCommandEncoder)(nil)
	_ hal.RenderPassEncoder   = (*rustRenderPass)(nil)
	_ hal.ComputePassEncoder  = (*rustComputePass)(nil)
	_ hal.Buffer              = (*rustBuffer)(nil)
	_ hal.Texture             = (*rustTexture)(nil)
	_ hal.SurfaceTexture      = (*rustSurfaceTexture)(nil)
	_ hal.TextureView         = (*rustTextureView)(nil)
	_ hal.Sampler             = (*rustSampler)(nil)
	_ hal.ShaderModule        = (*rustShaderModule)(nil)
	_ hal.BindGroupLayout     = (*rustBindGroupLayout)(nil)
	_ hal.BindGroup           = (*rustBindGroup)(nil)
	_ hal.PipelineLayout      = (*rustPipelineLayout)(nil)
	_ hal.RenderPipeline      = (*rustRenderPipeline)(nil)
	_ hal.ComputePipeline     = (*rustComputePipeline)(nil)
	_ hal.QuerySet            = (*rustQuerySet)(nil)
	_ hal.CommandBuffer       = (*rustCommandBuffer)(nil)
	_ hal.Fence               = (*rustFence)(nil)
	_ hal.RenderBundle        = (*rustRenderBundle)(nil)
	_ hal.RenderBundleEncoder = (*rustRenderBundleEncoder)(nil)
)
