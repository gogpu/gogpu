//go:build windows

// Package rust provides the WebGPU backend using wgpu-native (Rust) via go-webgpu/webgpu.
// This backend offers maximum performance and is battle-tested in production.
// Currently only available on Windows due to go-webgpu/goffi limitations.
package rust

import (
	"fmt"

	"github.com/go-webgpu/webgpu/wgpu"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
)

// Backend implements gpu.Backend using wgpu-native.
type Backend struct {
	// Store native handles for cleanup
	instances        map[types.Instance]*wgpu.Instance
	adapters         map[types.Adapter]*wgpu.Adapter
	devices          map[types.Device]*wgpu.Device
	queues           map[types.Queue]*wgpu.Queue
	surfaces         map[types.Surface]*wgpu.Surface
	shaders          map[types.ShaderModule]*wgpu.ShaderModule
	pipelines        map[types.RenderPipeline]*wgpu.RenderPipeline
	encoders         map[types.CommandEncoder]*wgpu.CommandEncoder
	cmdBuffers       map[types.CommandBuffer]*wgpu.CommandBuffer
	passes           map[types.RenderPass]*wgpu.RenderPassEncoder
	textures         map[types.Texture]*wgpu.Texture
	views            map[types.TextureView]*wgpu.TextureView
	samplers         map[types.Sampler]*wgpu.Sampler
	gpuBuffers       map[types.Buffer]*wgpu.Buffer
	bindGroupLayouts map[types.BindGroupLayout]*wgpu.BindGroupLayout
	bindGroups       map[types.BindGroup]*wgpu.BindGroup
	pipelineLayouts  map[types.PipelineLayout]*wgpu.PipelineLayout

	nextHandle uintptr
}

// IsAvailable returns true on Windows where go-webgpu/goffi is supported.
func IsAvailable() bool {
	return true
}

// New creates a new Rust backend.
func New() *Backend {
	return &Backend{
		instances:        make(map[types.Instance]*wgpu.Instance),
		adapters:         make(map[types.Adapter]*wgpu.Adapter),
		devices:          make(map[types.Device]*wgpu.Device),
		queues:           make(map[types.Queue]*wgpu.Queue),
		surfaces:         make(map[types.Surface]*wgpu.Surface),
		shaders:          make(map[types.ShaderModule]*wgpu.ShaderModule),
		pipelines:        make(map[types.RenderPipeline]*wgpu.RenderPipeline),
		encoders:         make(map[types.CommandEncoder]*wgpu.CommandEncoder),
		cmdBuffers:       make(map[types.CommandBuffer]*wgpu.CommandBuffer),
		passes:           make(map[types.RenderPass]*wgpu.RenderPassEncoder),
		textures:         make(map[types.Texture]*wgpu.Texture),
		views:            make(map[types.TextureView]*wgpu.TextureView),
		samplers:         make(map[types.Sampler]*wgpu.Sampler),
		gpuBuffers:       make(map[types.Buffer]*wgpu.Buffer),
		bindGroupLayouts: make(map[types.BindGroupLayout]*wgpu.BindGroupLayout),
		bindGroups:       make(map[types.BindGroup]*wgpu.BindGroup),
		pipelineLayouts:  make(map[types.PipelineLayout]*wgpu.PipelineLayout),
		nextHandle:       1,
	}
}

func (b *Backend) newHandle() uintptr {
	h := b.nextHandle
	b.nextHandle++
	return h
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "Rust (wgpu-native)"
}

// Init initializes the backend.
func (b *Backend) Init() error {
	return nil
}

// Destroy releases all backend resources.
//
//nolint:gocognit,gocyclo,cyclop // Cleanup code is inherently repetitive but straightforward.
func (b *Backend) Destroy() {
	// Release in reverse order of creation
	for _, pl := range b.pipelineLayouts {
		if pl != nil {
			pl.Release()
		}
	}
	for _, bg := range b.bindGroups {
		if bg != nil {
			bg.Release()
		}
	}
	for _, bgl := range b.bindGroupLayouts {
		if bgl != nil {
			bgl.Release()
		}
	}
	for _, buf := range b.gpuBuffers {
		if buf != nil {
			buf.Release()
		}
	}
	for _, s := range b.samplers {
		if s != nil {
			s.Release()
		}
	}
	for _, v := range b.views {
		if v != nil {
			v.Release()
		}
	}
	for _, t := range b.textures {
		if t != nil {
			t.Release()
		}
	}
	for _, p := range b.pipelines {
		if p != nil {
			p.Release()
		}
	}
	for _, s := range b.shaders {
		if s != nil {
			s.Release()
		}
	}
	for _, s := range b.surfaces {
		if s != nil {
			s.Release()
		}
	}
	for _, q := range b.queues {
		if q != nil {
			q.Release()
		}
	}
	for _, d := range b.devices {
		if d != nil {
			d.Release()
		}
	}
	for _, a := range b.adapters {
		if a != nil {
			a.Release()
		}
	}
	for _, i := range b.instances {
		if i != nil {
			i.Release()
		}
	}
}

// CreateInstance creates a WebGPU instance.
func (b *Backend) CreateInstance() (types.Instance, error) {
	inst, err := wgpu.CreateInstance(nil)
	if err != nil {
		return 0, fmt.Errorf("rust backend: create instance: %w", err)
	}
	handle := types.Instance(b.newHandle())
	b.instances[handle] = inst
	return handle, nil
}

// RequestAdapter requests a GPU adapter.
func (b *Backend) RequestAdapter(instance types.Instance, opts *types.AdapterOptions) (types.Adapter, error) {
	inst := b.instances[instance]
	if inst == nil {
		return 0, fmt.Errorf("rust backend: invalid instance")
	}

	var wgpuOpts *wgpu.RequestAdapterOptions
	if opts != nil {
		wgpuOpts = &wgpu.RequestAdapterOptions{
			PowerPreference: wgpu.PowerPreference(opts.PowerPreference),
		}
	}

	adapter, err := inst.RequestAdapter(wgpuOpts)
	if err != nil {
		return 0, fmt.Errorf("rust backend: request adapter: %w", err)
	}

	handle := types.Adapter(b.newHandle())
	b.adapters[handle] = adapter
	return handle, nil
}

// RequestDevice requests a GPU device.
func (b *Backend) RequestDevice(adapter types.Adapter, opts *types.DeviceOptions) (types.Device, error) {
	adpt := b.adapters[adapter]
	if adpt == nil {
		return 0, fmt.Errorf("rust backend: invalid adapter")
	}

	device, err := adpt.RequestDevice(nil)
	if err != nil {
		return 0, fmt.Errorf("rust backend: request device: %w", err)
	}

	handle := types.Device(b.newHandle())
	b.devices[handle] = device
	return handle, nil
}

// GetQueue gets the device queue.
func (b *Backend) GetQueue(device types.Device) types.Queue {
	dev := b.devices[device]
	if dev == nil {
		return 0
	}
	queue := dev.GetQueue()
	handle := types.Queue(b.newHandle())
	b.queues[handle] = queue
	return handle
}

// CreateSurface creates a rendering surface.
func (b *Backend) CreateSurface(instance types.Instance, sh types.SurfaceHandle) (types.Surface, error) {
	inst := b.instances[instance]
	if inst == nil {
		return 0, fmt.Errorf("rust backend: invalid instance")
	}

	surface, err := inst.CreateSurfaceFromWindowsHWND(sh.Instance, sh.Window)
	if err != nil {
		return 0, fmt.Errorf("rust backend: create surface: %w", err)
	}

	handle := types.Surface(b.newHandle())
	b.surfaces[handle] = surface
	return handle, nil
}

// ConfigureSurface configures the surface.
func (b *Backend) ConfigureSurface(surface types.Surface, device types.Device, config *types.SurfaceConfig) {
	surf := b.surfaces[surface]
	dev := b.devices[device]
	if surf == nil || dev == nil {
		return
	}

	surf.Configure(&wgpu.SurfaceConfiguration{
		Device:      dev,
		Format:      wgpu.TextureFormat(config.Format),
		Usage:       wgpu.TextureUsage(config.Usage),
		Width:       config.Width,
		Height:      config.Height,
		PresentMode: wgpu.PresentMode(config.PresentMode),
		AlphaMode:   wgpu.CompositeAlphaMode(config.AlphaMode),
	})
}

// GetCurrentTexture gets the current surface texture.
func (b *Backend) GetCurrentTexture(surface types.Surface) (types.SurfaceTexture, error) {
	surf := b.surfaces[surface]
	if surf == nil {
		return types.SurfaceTexture{}, fmt.Errorf("rust backend: invalid surface")
	}

	tex, err := surf.GetCurrentTexture()
	if err != nil {
		return types.SurfaceTexture{Status: types.SurfaceStatusError}, err
	}

	handle := types.Texture(b.newHandle())
	b.textures[handle] = tex.Texture

	return types.SurfaceTexture{
		Texture: handle,
		Status:  types.SurfaceStatusSuccess,
	}, nil
}

// Present presents the surface.
func (b *Backend) Present(surface types.Surface) {
	surf := b.surfaces[surface]
	if surf != nil {
		surf.Present()
	}
}

// CreateShaderModuleWGSL creates a shader module from WGSL code.
func (b *Backend) CreateShaderModuleWGSL(device types.Device, code string) (types.ShaderModule, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	shader := dev.CreateShaderModuleWGSL(code)
	if shader == nil {
		return 0, fmt.Errorf("rust backend: failed to create shader module")
	}

	handle := types.ShaderModule(b.newHandle())
	b.shaders[handle] = shader
	return handle, nil
}

// CreateRenderPipeline creates a render pipeline.
func (b *Backend) CreateRenderPipeline(device types.Device, desc *types.RenderPipelineDescriptor) (types.RenderPipeline, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	vertShader := b.shaders[desc.VertexShader]
	fragShader := b.shaders[desc.FragmentShader]
	if vertShader == nil || fragShader == nil {
		return 0, fmt.Errorf("rust backend: invalid shader module")
	}

	pipeline := dev.CreateRenderPipelineSimple(
		nil,
		vertShader, desc.VertexEntryPoint,
		fragShader, desc.FragmentEntry,
		wgpu.TextureFormat(desc.TargetFormat),
	)
	if pipeline == nil {
		return 0, fmt.Errorf("rust backend: failed to create pipeline")
	}

	handle := types.RenderPipeline(b.newHandle())
	b.pipelines[handle] = pipeline
	return handle, nil
}

// CreateCommandEncoder creates a command encoder.
func (b *Backend) CreateCommandEncoder(device types.Device) types.CommandEncoder {
	dev := b.devices[device]
	if dev == nil {
		return 0
	}

	encoder := dev.CreateCommandEncoder(nil)
	handle := types.CommandEncoder(b.newHandle())
	b.encoders[handle] = encoder
	return handle
}

// BeginRenderPass begins a render pass.
func (b *Backend) BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass {
	enc := b.encoders[encoder]
	if enc == nil {
		return 0
	}

	attachments := make([]wgpu.RenderPassColorAttachment, len(desc.ColorAttachments))
	for i, att := range desc.ColorAttachments {
		view := b.views[att.View]
		attachments[i] = wgpu.RenderPassColorAttachment{
			View:       view,
			LoadOp:     wgpu.LoadOp(att.LoadOp),
			StoreOp:    wgpu.StoreOp(att.StoreOp),
			ClearValue: wgpu.Color{R: att.ClearValue.R, G: att.ClearValue.G, B: att.ClearValue.B, A: att.ClearValue.A},
		}
	}

	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: attachments,
	})

	handle := types.RenderPass(b.newHandle())
	b.passes[handle] = pass
	return handle
}

// EndRenderPass ends a render pass.
func (b *Backend) EndRenderPass(pass types.RenderPass) {
	p := b.passes[pass]
	if p != nil {
		p.End()
	}
}

// FinishEncoder finishes the command encoder.
func (b *Backend) FinishEncoder(encoder types.CommandEncoder) types.CommandBuffer {
	enc := b.encoders[encoder]
	if enc == nil {
		return 0
	}

	buffer := enc.Finish(nil)
	handle := types.CommandBuffer(b.newHandle())
	b.cmdBuffers[handle] = buffer
	return handle
}

// Submit submits commands to the queue.
func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer) {
	q := b.queues[queue]
	buf := b.cmdBuffers[commands]
	if q != nil && buf != nil {
		q.Submit(buf)
	}
}

// SetPipeline sets the render pipeline.
func (b *Backend) SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline) {
	p := b.passes[pass]
	pipe := b.pipelines[pipeline]
	if p != nil && pipe != nil {
		p.SetPipeline(pipe)
	}
}

// Draw issues a draw call.
func (b *Backend) Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	p := b.passes[pass]
	if p != nil {
		p.Draw(vertexCount, instanceCount, firstVertex, firstInstance)
	}
}

// CreateTexture creates a texture.
func (b *Backend) CreateTexture(device types.Device, desc *types.TextureDescriptor) (types.Texture, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	wgpuDesc := &wgpu.TextureDescriptor{
		Label: wgpu.EmptyStringView(),
		Size: wgpu.Extent3D{
			Width:              desc.Size.Width,
			Height:             desc.Size.Height,
			DepthOrArrayLayers: desc.Size.DepthOrArrayLayers,
		},
		MipLevelCount: desc.MipLevelCount,
		SampleCount:   desc.SampleCount,
		Dimension:     wgpu.TextureDimension(desc.Dimension),
		Format:        wgpu.TextureFormat(desc.Format),
		Usage:         wgpu.TextureUsage(desc.Usage),
	}

	texture := dev.CreateTexture(wgpuDesc)
	if texture == nil {
		return 0, fmt.Errorf("rust backend: failed to create texture")
	}

	handle := types.Texture(b.newHandle())
	b.textures[handle] = texture
	return handle, nil
}

// CreateTextureView creates a texture view.
func (b *Backend) CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView {
	tex := b.textures[texture]
	if tex == nil {
		return 0
	}

	view := tex.CreateView(nil)
	handle := types.TextureView(b.newHandle())
	b.views[handle] = view
	return handle
}

// WriteTexture writes data to a texture.
func (b *Backend) WriteTexture(queue types.Queue, dst *types.ImageCopyTexture, data []byte, layout *types.ImageDataLayout, size *types.Extent3D) {
	q := b.queues[queue]
	tex := b.textures[dst.Texture]
	if q == nil || tex == nil {
		return
	}

	wgpuDst := &wgpu.TexelCopyTextureInfo{
		Texture:  tex.Handle(),
		MipLevel: dst.MipLevel,
		Origin: wgpu.Origin3D{
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

	wgpuSize := &wgpu.Extent3D{
		Width:              size.Width,
		Height:             size.Height,
		DepthOrArrayLayers: size.DepthOrArrayLayers,
	}

	q.WriteTexture(wgpuDst, data, wgpuLayout, wgpuSize)
}

// CreateSampler creates a sampler.
func (b *Backend) CreateSampler(device types.Device, desc *types.SamplerDescriptor) (types.Sampler, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	wgpuDesc := &wgpu.SamplerDescriptor{
		Label:         wgpu.EmptyStringView(),
		AddressModeU:  wgpu.AddressMode(desc.AddressModeU),
		AddressModeV:  wgpu.AddressMode(desc.AddressModeV),
		AddressModeW:  wgpu.AddressMode(desc.AddressModeW),
		MagFilter:     wgpu.FilterMode(desc.MagFilter),
		MinFilter:     wgpu.FilterMode(desc.MinFilter),
		MipmapFilter:  wgpu.MipmapFilterMode(desc.MipmapFilter),
		LodMinClamp:   desc.LodMinClamp,
		LodMaxClamp:   desc.LodMaxClamp,
		Compare:       wgpu.CompareFunction(desc.Compare),
		MaxAnisotropy: desc.MaxAnisotropy,
	}

	sampler := dev.CreateSampler(wgpuDesc)
	if sampler == nil {
		return 0, fmt.Errorf("rust backend: failed to create sampler")
	}

	handle := types.Sampler(b.newHandle())
	b.samplers[handle] = sampler
	return handle, nil
}

// CreateBuffer creates a buffer.
func (b *Backend) CreateBuffer(device types.Device, desc *types.BufferDescriptor) (types.Buffer, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	var mappedAtCreation wgpu.Bool
	if desc.MappedAtCreation {
		mappedAtCreation = wgpu.True
	} else {
		mappedAtCreation = wgpu.False
	}

	wgpuDesc := &wgpu.BufferDescriptor{
		Label:            wgpu.EmptyStringView(),
		Size:             desc.Size,
		Usage:            wgpu.BufferUsage(desc.Usage),
		MappedAtCreation: mappedAtCreation,
	}

	buffer := dev.CreateBuffer(wgpuDesc)
	if buffer == nil {
		return 0, fmt.Errorf("rust backend: failed to create buffer")
	}

	handle := types.Buffer(b.newHandle())
	b.gpuBuffers[handle] = buffer
	return handle, nil
}

// WriteBuffer writes data to a buffer.
func (b *Backend) WriteBuffer(queue types.Queue, buffer types.Buffer, offset uint64, data []byte) {
	q := b.queues[queue]
	buf := b.gpuBuffers[buffer]
	if q == nil || buf == nil {
		return
	}

	q.WriteBuffer(buf, offset, data)
}

// CreateBindGroupLayout creates a bind group layout.
func (b *Backend) CreateBindGroupLayout(device types.Device, desc *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	entries := make([]wgpu.BindGroupLayoutEntry, len(desc.Entries))
	for i, entry := range desc.Entries {
		wgpuEntry := wgpu.BindGroupLayoutEntry{
			Binding:    entry.Binding,
			Visibility: wgpu.ShaderStage(entry.Visibility),
		}

		if entry.Buffer != nil {
			var hasDynamicOffset wgpu.Bool
			if entry.Buffer.HasDynamicOffset {
				hasDynamicOffset = wgpu.True
			} else {
				hasDynamicOffset = wgpu.False
			}
			wgpuEntry.Buffer = wgpu.BufferBindingLayout{
				Type:             wgpu.BufferBindingType(entry.Buffer.Type),
				HasDynamicOffset: hasDynamicOffset,
				MinBindingSize:   entry.Buffer.MinBindingSize,
			}
		}

		if entry.Sampler != nil {
			wgpuEntry.Sampler = wgpu.SamplerBindingLayout{
				Type: wgpu.SamplerBindingType(entry.Sampler.Type),
			}
		}

		if entry.Texture != nil {
			var multisampled wgpu.Bool
			if entry.Texture.Multisampled {
				multisampled = wgpu.True
			} else {
				multisampled = wgpu.False
			}
			wgpuEntry.Texture = wgpu.TextureBindingLayout{
				SampleType:    wgpu.TextureSampleType(entry.Texture.SampleType),
				ViewDimension: wgpu.TextureViewDimension(entry.Texture.ViewDimension),
				Multisampled:  multisampled,
			}
		}

		entries[i] = wgpuEntry
	}

	layout := dev.CreateBindGroupLayoutSimple(entries)
	if layout == nil {
		return 0, fmt.Errorf("rust backend: failed to create bind group layout")
	}

	handle := types.BindGroupLayout(b.newHandle())
	b.bindGroupLayouts[handle] = layout
	return handle, nil
}

// CreateBindGroup creates a bind group.
func (b *Backend) CreateBindGroup(device types.Device, desc *types.BindGroupDescriptor) (types.BindGroup, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	layout := b.bindGroupLayouts[desc.Layout]
	if layout == nil {
		return 0, fmt.Errorf("rust backend: invalid bind group layout")
	}

	entries := make([]wgpu.BindGroupEntry, len(desc.Entries))
	for i, entry := range desc.Entries {
		wgpuEntry := wgpu.BindGroupEntry{
			Binding: entry.Binding,
		}

		if entry.Buffer != 0 {
			buf := b.gpuBuffers[entry.Buffer]
			if buf != nil {
				wgpuEntry.Buffer = buf.Handle()
				wgpuEntry.Offset = entry.Offset
				wgpuEntry.Size = entry.Size
			}
		}

		if entry.Sampler != 0 {
			sampler := b.samplers[entry.Sampler]
			if sampler != nil {
				wgpuEntry.Sampler = sampler.Handle()
			}
		}

		if entry.TextureView != 0 {
			view := b.views[entry.TextureView]
			if view != nil {
				wgpuEntry.TextureView = view.Handle()
			}
		}

		entries[i] = wgpuEntry
	}

	bindGroup := dev.CreateBindGroupSimple(layout, entries)
	if bindGroup == nil {
		return 0, fmt.Errorf("rust backend: failed to create bind group")
	}

	handle := types.BindGroup(b.newHandle())
	b.bindGroups[handle] = bindGroup
	return handle, nil
}

// CreatePipelineLayout creates a pipeline layout.
func (b *Backend) CreatePipelineLayout(device types.Device, desc *types.PipelineLayoutDescriptor) (types.PipelineLayout, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	layouts := make([]*wgpu.BindGroupLayout, len(desc.BindGroupLayouts))
	for i, layoutHandle := range desc.BindGroupLayouts {
		layout := b.bindGroupLayouts[layoutHandle]
		if layout == nil {
			return 0, fmt.Errorf("rust backend: invalid bind group layout at index %d", i)
		}
		layouts[i] = layout
	}

	pipelineLayout := dev.CreatePipelineLayoutSimple(layouts)
	if pipelineLayout == nil {
		return 0, fmt.Errorf("rust backend: failed to create pipeline layout")
	}

	handle := types.PipelineLayout(b.newHandle())
	b.pipelineLayouts[handle] = pipelineLayout
	return handle, nil
}

// SetBindGroup sets a bind group for rendering.
func (b *Backend) SetBindGroup(pass types.RenderPass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32) {
	p := b.passes[pass]
	bg := b.bindGroups[bindGroup]
	if p == nil || bg == nil {
		return
	}

	p.SetBindGroup(index, bg, dynamicOffsets)
}

// SetVertexBuffer sets a vertex buffer for rendering.
func (b *Backend) SetVertexBuffer(pass types.RenderPass, slot uint32, buffer types.Buffer, offset, size uint64) {
	p := b.passes[pass]
	buf := b.gpuBuffers[buffer]
	if p == nil || buf == nil {
		return
	}

	p.SetVertexBuffer(slot, buf, offset, size)
}

// SetIndexBuffer sets an index buffer for rendering.
func (b *Backend) SetIndexBuffer(pass types.RenderPass, buffer types.Buffer, format types.IndexFormat, offset, size uint64) {
	p := b.passes[pass]
	buf := b.gpuBuffers[buffer]
	if p == nil || buf == nil {
		return
	}

	p.SetIndexBuffer(buf, wgpu.IndexFormat(format), offset, size)
}

// DrawIndexed issues an indexed draw call.
func (b *Backend) DrawIndexed(pass types.RenderPass, indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	p := b.passes[pass]
	if p == nil {
		return
	}

	p.DrawIndexed(indexCount, instanceCount, firstIndex, baseVertex, firstInstance)
}

// ReleaseTextureView releases a texture view.
func (b *Backend) ReleaseTextureView(view types.TextureView) {
	v := b.views[view]
	if v != nil {
		v.Release()
		delete(b.views, view)
	}
}

// ReleaseTexture releases a texture.
func (b *Backend) ReleaseTexture(texture types.Texture) {
	t := b.textures[texture]
	if t != nil {
		t.Release()
		delete(b.textures, texture)
	}
}

// ReleaseSampler releases a sampler.
func (b *Backend) ReleaseSampler(sampler types.Sampler) {
	s := b.samplers[sampler]
	if s != nil {
		s.Release()
		delete(b.samplers, sampler)
	}
}

// ReleaseBuffer releases a buffer.
func (b *Backend) ReleaseBuffer(buffer types.Buffer) {
	buf := b.gpuBuffers[buffer]
	if buf != nil {
		buf.Release()
		delete(b.gpuBuffers, buffer)
	}
}

// ReleaseBindGroupLayout releases a bind group layout.
func (b *Backend) ReleaseBindGroupLayout(layout types.BindGroupLayout) {
	l := b.bindGroupLayouts[layout]
	if l != nil {
		l.Release()
		delete(b.bindGroupLayouts, layout)
	}
}

// ReleaseBindGroup releases a bind group.
func (b *Backend) ReleaseBindGroup(group types.BindGroup) {
	g := b.bindGroups[group]
	if g != nil {
		g.Release()
		delete(b.bindGroups, group)
	}
}

// ReleasePipelineLayout releases a pipeline layout.
func (b *Backend) ReleasePipelineLayout(layout types.PipelineLayout) {
	l := b.pipelineLayouts[layout]
	if l != nil {
		l.Release()
		delete(b.pipelineLayouts, layout)
	}
}

// ReleaseCommandBuffer releases a command buffer.
func (b *Backend) ReleaseCommandBuffer(buffer types.CommandBuffer) {
	buf := b.cmdBuffers[buffer]
	if buf != nil {
		buf.Release()
		delete(b.cmdBuffers, buffer)
	}
}

// ReleaseCommandEncoder releases a command encoder.
func (b *Backend) ReleaseCommandEncoder(encoder types.CommandEncoder) {
	enc := b.encoders[encoder]
	if enc != nil {
		enc.Release()
		delete(b.encoders, encoder)
	}
}

// ReleaseRenderPass releases a render pass.
func (b *Backend) ReleaseRenderPass(pass types.RenderPass) {
	p := b.passes[pass]
	if p != nil {
		p.Release()
		delete(b.passes, pass)
	}
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
