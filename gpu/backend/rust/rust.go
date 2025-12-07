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
	instances map[types.Instance]*wgpu.Instance
	adapters  map[types.Adapter]*wgpu.Adapter
	devices   map[types.Device]*wgpu.Device
	queues    map[types.Queue]*wgpu.Queue
	surfaces  map[types.Surface]*wgpu.Surface
	shaders   map[types.ShaderModule]*wgpu.ShaderModule
	pipelines map[types.RenderPipeline]*wgpu.RenderPipeline
	encoders  map[types.CommandEncoder]*wgpu.CommandEncoder
	buffers   map[types.CommandBuffer]*wgpu.CommandBuffer
	passes    map[types.RenderPass]*wgpu.RenderPassEncoder
	textures  map[types.Texture]*wgpu.Texture
	views     map[types.TextureView]*wgpu.TextureView

	nextHandle uintptr
}

// IsAvailable returns true on Windows where go-webgpu/goffi is supported.
func IsAvailable() bool {
	return true
}

// New creates a new Rust backend.
func New() *Backend {
	return &Backend{
		instances:  make(map[types.Instance]*wgpu.Instance),
		adapters:   make(map[types.Adapter]*wgpu.Adapter),
		devices:    make(map[types.Device]*wgpu.Device),
		queues:     make(map[types.Queue]*wgpu.Queue),
		surfaces:   make(map[types.Surface]*wgpu.Surface),
		shaders:    make(map[types.ShaderModule]*wgpu.ShaderModule),
		pipelines:  make(map[types.RenderPipeline]*wgpu.RenderPipeline),
		encoders:   make(map[types.CommandEncoder]*wgpu.CommandEncoder),
		buffers:    make(map[types.CommandBuffer]*wgpu.CommandBuffer),
		passes:     make(map[types.RenderPass]*wgpu.RenderPassEncoder),
		textures:   make(map[types.Texture]*wgpu.Texture),
		views:      make(map[types.TextureView]*wgpu.TextureView),
		nextHandle: 1,
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
func (b *Backend) Destroy() {
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
	b.buffers[handle] = buffer
	return handle
}

// Submit submits commands to the queue.
func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer) {
	q := b.queues[queue]
	buf := b.buffers[commands]
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

// ReleaseCommandBuffer releases a command buffer.
func (b *Backend) ReleaseCommandBuffer(buffer types.CommandBuffer) {
	buf := b.buffers[buffer]
	if buf != nil {
		buf.Release()
		delete(b.buffers, buffer)
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
