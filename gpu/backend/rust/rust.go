// Package rust provides the WebGPU backend using wgpu-native (Rust) via go-webgpu/webgpu.
// This backend offers maximum performance and is battle-tested in production.
package rust

import (
	"fmt"

	"github.com/go-webgpu/webgpu/wgpu"

	"github.com/gogpu/gogpu/gpu"
)

// Backend implements gpu.Backend using wgpu-native.
type Backend struct {
	// Store native handles for cleanup
	instances map[gpu.Instance]*wgpu.Instance
	adapters  map[gpu.Adapter]*wgpu.Adapter
	devices   map[gpu.Device]*wgpu.Device
	queues    map[gpu.Queue]*wgpu.Queue
	surfaces  map[gpu.Surface]*wgpu.Surface
	shaders   map[gpu.ShaderModule]*wgpu.ShaderModule
	pipelines map[gpu.RenderPipeline]*wgpu.RenderPipeline
	encoders  map[gpu.CommandEncoder]*wgpu.CommandEncoder
	buffers   map[gpu.CommandBuffer]*wgpu.CommandBuffer
	passes    map[gpu.RenderPass]*wgpu.RenderPassEncoder
	textures  map[gpu.Texture]*wgpu.Texture
	views     map[gpu.TextureView]*wgpu.TextureView

	nextHandle uintptr
}

// New creates a new Rust backend.
func New() *Backend {
	return &Backend{
		instances:  make(map[gpu.Instance]*wgpu.Instance),
		adapters:   make(map[gpu.Adapter]*wgpu.Adapter),
		devices:    make(map[gpu.Device]*wgpu.Device),
		queues:     make(map[gpu.Queue]*wgpu.Queue),
		surfaces:   make(map[gpu.Surface]*wgpu.Surface),
		shaders:    make(map[gpu.ShaderModule]*wgpu.ShaderModule),
		pipelines:  make(map[gpu.RenderPipeline]*wgpu.RenderPipeline),
		encoders:   make(map[gpu.CommandEncoder]*wgpu.CommandEncoder),
		buffers:    make(map[gpu.CommandBuffer]*wgpu.CommandBuffer),
		passes:     make(map[gpu.RenderPass]*wgpu.RenderPassEncoder),
		textures:   make(map[gpu.Texture]*wgpu.Texture),
		views:      make(map[gpu.TextureView]*wgpu.TextureView),
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
func (b *Backend) CreateInstance() (gpu.Instance, error) {
	inst, err := wgpu.CreateInstance(nil)
	if err != nil {
		return 0, fmt.Errorf("rust backend: create instance: %w", err)
	}
	handle := gpu.Instance(b.newHandle())
	b.instances[handle] = inst
	return handle, nil
}

// RequestAdapter requests a GPU adapter.
func (b *Backend) RequestAdapter(instance gpu.Instance, opts *gpu.AdapterOptions) (gpu.Adapter, error) {
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

	handle := gpu.Adapter(b.newHandle())
	b.adapters[handle] = adapter
	return handle, nil
}

// RequestDevice requests a GPU device.
func (b *Backend) RequestDevice(adapter gpu.Adapter, opts *gpu.DeviceOptions) (gpu.Device, error) {
	adpt := b.adapters[adapter]
	if adpt == nil {
		return 0, fmt.Errorf("rust backend: invalid adapter")
	}

	device, err := adpt.RequestDevice(nil)
	if err != nil {
		return 0, fmt.Errorf("rust backend: request device: %w", err)
	}

	handle := gpu.Device(b.newHandle())
	b.devices[handle] = device
	return handle, nil
}

// GetQueue gets the device queue.
func (b *Backend) GetQueue(device gpu.Device) gpu.Queue {
	dev := b.devices[device]
	if dev == nil {
		return 0
	}
	queue := dev.GetQueue()
	handle := gpu.Queue(b.newHandle())
	b.queues[handle] = queue
	return handle
}

// CreateSurface creates a rendering surface.
func (b *Backend) CreateSurface(instance gpu.Instance, sh gpu.SurfaceHandle) (gpu.Surface, error) {
	inst := b.instances[instance]
	if inst == nil {
		return 0, fmt.Errorf("rust backend: invalid instance")
	}

	surface, err := inst.CreateSurfaceFromWindowsHWND(sh.Instance, sh.Window)
	if err != nil {
		return 0, fmt.Errorf("rust backend: create surface: %w", err)
	}

	handle := gpu.Surface(b.newHandle())
	b.surfaces[handle] = surface
	return handle, nil
}

// ConfigureSurface configures the surface.
func (b *Backend) ConfigureSurface(surface gpu.Surface, device gpu.Device, config *gpu.SurfaceConfig) {
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
func (b *Backend) GetCurrentTexture(surface gpu.Surface) (gpu.SurfaceTexture, error) {
	surf := b.surfaces[surface]
	if surf == nil {
		return gpu.SurfaceTexture{}, fmt.Errorf("rust backend: invalid surface")
	}

	tex, err := surf.GetCurrentTexture()
	if err != nil {
		return gpu.SurfaceTexture{Status: gpu.SurfaceStatusError}, err
	}

	handle := gpu.Texture(b.newHandle())
	b.textures[handle] = tex.Texture

	return gpu.SurfaceTexture{
		Texture: handle,
		Status:  gpu.SurfaceStatusSuccess,
	}, nil
}

// Present presents the surface.
func (b *Backend) Present(surface gpu.Surface) {
	surf := b.surfaces[surface]
	if surf != nil {
		surf.Present()
	}
}

// CreateShaderModuleWGSL creates a shader module from WGSL code.
func (b *Backend) CreateShaderModuleWGSL(device gpu.Device, code string) (gpu.ShaderModule, error) {
	dev := b.devices[device]
	if dev == nil {
		return 0, fmt.Errorf("rust backend: invalid device")
	}

	shader := dev.CreateShaderModuleWGSL(code)
	if shader == nil {
		return 0, fmt.Errorf("rust backend: failed to create shader module")
	}

	handle := gpu.ShaderModule(b.newHandle())
	b.shaders[handle] = shader
	return handle, nil
}

// CreateRenderPipeline creates a render pipeline.
func (b *Backend) CreateRenderPipeline(device gpu.Device, desc *gpu.RenderPipelineDescriptor) (gpu.RenderPipeline, error) {
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

	handle := gpu.RenderPipeline(b.newHandle())
	b.pipelines[handle] = pipeline
	return handle, nil
}

// CreateCommandEncoder creates a command encoder.
func (b *Backend) CreateCommandEncoder(device gpu.Device) gpu.CommandEncoder {
	dev := b.devices[device]
	if dev == nil {
		return 0
	}

	encoder := dev.CreateCommandEncoder(nil)
	handle := gpu.CommandEncoder(b.newHandle())
	b.encoders[handle] = encoder
	return handle
}

// BeginRenderPass begins a render pass.
func (b *Backend) BeginRenderPass(encoder gpu.CommandEncoder, desc *gpu.RenderPassDescriptor) gpu.RenderPass {
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
			ClearValue: wgpu.Color{R: att.ClearColor.R, G: att.ClearColor.G, B: att.ClearColor.B, A: att.ClearColor.A},
		}
	}

	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: attachments,
	})

	handle := gpu.RenderPass(b.newHandle())
	b.passes[handle] = pass
	return handle
}

// EndRenderPass ends a render pass.
func (b *Backend) EndRenderPass(pass gpu.RenderPass) {
	p := b.passes[pass]
	if p != nil {
		p.End()
	}
}

// FinishEncoder finishes the command encoder.
func (b *Backend) FinishEncoder(encoder gpu.CommandEncoder) gpu.CommandBuffer {
	enc := b.encoders[encoder]
	if enc == nil {
		return 0
	}

	buffer := enc.Finish(nil)
	handle := gpu.CommandBuffer(b.newHandle())
	b.buffers[handle] = buffer
	return handle
}

// Submit submits commands to the queue.
func (b *Backend) Submit(queue gpu.Queue, commands gpu.CommandBuffer) {
	q := b.queues[queue]
	buf := b.buffers[commands]
	if q != nil && buf != nil {
		q.Submit(buf)
	}
}

// SetPipeline sets the render pipeline.
func (b *Backend) SetPipeline(pass gpu.RenderPass, pipeline gpu.RenderPipeline) {
	p := b.passes[pass]
	pipe := b.pipelines[pipeline]
	if p != nil && pipe != nil {
		p.SetPipeline(pipe)
	}
}

// Draw issues a draw call.
func (b *Backend) Draw(pass gpu.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	p := b.passes[pass]
	if p != nil {
		p.Draw(vertexCount, instanceCount, firstVertex, firstInstance)
	}
}

// CreateTextureView creates a texture view.
func (b *Backend) CreateTextureView(texture gpu.Texture, desc *gpu.TextureViewDescriptor) gpu.TextureView {
	tex := b.textures[texture]
	if tex == nil {
		return 0
	}

	view := tex.CreateView(nil)
	handle := gpu.TextureView(b.newHandle())
	b.views[handle] = view
	return handle
}

// ReleaseTextureView releases a texture view.
func (b *Backend) ReleaseTextureView(view gpu.TextureView) {
	v := b.views[view]
	if v != nil {
		v.Release()
		delete(b.views, view)
	}
}

// ReleaseTexture releases a texture.
func (b *Backend) ReleaseTexture(texture gpu.Texture) {
	t := b.textures[texture]
	if t != nil {
		t.Release()
		delete(b.textures, texture)
	}
}

// ReleaseCommandBuffer releases a command buffer.
func (b *Backend) ReleaseCommandBuffer(buffer gpu.CommandBuffer) {
	buf := b.buffers[buffer]
	if buf != nil {
		buf.Release()
		delete(b.buffers, buffer)
	}
}

// ReleaseCommandEncoder releases a command encoder.
func (b *Backend) ReleaseCommandEncoder(encoder gpu.CommandEncoder) {
	enc := b.encoders[encoder]
	if enc != nil {
		enc.Release()
		delete(b.encoders, encoder)
	}
}

// ReleaseRenderPass releases a render pass.
func (b *Backend) ReleaseRenderPass(pass gpu.RenderPass) {
	p := b.passes[pass]
	if p != nil {
		p.Release()
		delete(b.passes, pass)
	}
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
