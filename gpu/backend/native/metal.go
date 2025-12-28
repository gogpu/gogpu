//go:build darwin

// Package native provides the WebGPU backend using pure Go (gogpu/wgpu).
// This backend offers zero dependencies and simple cross-compilation.
//
// Implementation uses gogpu/wgpu HAL (Hardware Abstraction Layer) with Metal backend.
package native

import (
	"fmt"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/wgpu/hal"
	"github.com/gogpu/wgpu/hal/metal"
	wgputypes "github.com/gogpu/wgpu/types"
)

// Backend implements gpu.Backend using pure Go wgpu HAL.
type Backend struct {
	registry *ResourceRegistry
	backend  hal.Backend
}

// New creates a new Pure Go backend.
func New() *Backend {
	return &Backend{
		registry: NewResourceRegistry(),
		backend:  metal.Backend{}, // Metal is the HAL implementation for macOS
	}
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "Pure Go (gogpu/wgpu/metal)"
}

// Init initializes the backend.
func (b *Backend) Init() error {
	// Backend is stateless, no initialization needed
	// Actual initialization happens when creating instance
	return nil
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	// Note: This does NOT destroy HAL resources!
	// Caller must explicitly release all handles before calling Destroy.
	// This just clears the registry.
	b.registry.Clear()
}

// CreateInstance creates a WebGPU instance.
func (b *Backend) CreateInstance() (types.Instance, error) {
	// Create HAL instance with default config
	desc := &hal.InstanceDescriptor{
		Backends: wgputypes.Backends(1 << wgputypes.BackendMetal), // Metal backend
		Flags:    0,                                               // No debug for now
	}

	halInstance, err := b.backend.CreateInstance(desc)
	if err != nil {
		return 0, fmt.Errorf("native: failed to create instance: %w", err)
	}

	// Register and return handle
	handle := b.registry.RegisterInstance(halInstance)
	return handle, nil
}

// RequestAdapter requests a GPU adapter.
func (b *Backend) RequestAdapter(instance types.Instance, opts *types.AdapterOptions) (types.Adapter, error) {
	halInstance, err := b.registry.GetInstance(instance)
	if err != nil {
		return 0, err
	}

	// Enumerate adapters
	adapters := halInstance.EnumerateAdapters(nil) // nil = no surface hint
	if len(adapters) == 0 {
		return 0, fmt.Errorf("native: no adapters found")
	}

	// Pick first adapter for now
	// TODO: Support power preference from opts
	exposed := adapters[0]

	// Register and return handle
	handle := b.registry.RegisterAdapter(exposed.Adapter)
	return handle, nil
}

// RequestDevice requests a GPU device.
func (b *Backend) RequestDevice(adapter types.Adapter, opts *types.DeviceOptions) (types.Device, error) {
	halAdapter, err := b.registry.GetAdapter(adapter)
	if err != nil {
		return 0, err
	}

	// Open device with default features and limits
	openDevice, err := halAdapter.Open(wgputypes.Features(0), wgputypes.DefaultLimits())
	if err != nil {
		return 0, fmt.Errorf("native: failed to open device: %w", err)
	}

	// Register device and queue
	deviceHandle := b.registry.RegisterDevice(openDevice.Device)
	queueHandle := b.registry.RegisterQueue(openDevice.Queue)

	// Store device->queue mapping
	b.registry.RegisterDeviceQueue(deviceHandle, queueHandle)

	return deviceHandle, nil
}

// GetQueue gets the device queue.
func (b *Backend) GetQueue(device types.Device) types.Queue {
	queue, err := b.registry.GetQueueForDevice(device)
	if err != nil {
		return 0
	}
	return queue
}

// CreateSurface creates a rendering surface.
func (b *Backend) CreateSurface(instance types.Instance, handle types.SurfaceHandle) (types.Surface, error) {
	halInstance, err := b.registry.GetInstance(instance)
	if err != nil {
		return 0, err
	}

	halSurface, err := halInstance.CreateSurface(handle.Instance, handle.Window)
	if err != nil {
		return 0, fmt.Errorf("native: failed to create surface: %w", err)
	}

	surfaceHandle := b.registry.RegisterSurface(halSurface)
	return surfaceHandle, nil
}

// ConfigureSurface configures the surface.
func (b *Backend) ConfigureSurface(surface types.Surface, device types.Device, config *types.SurfaceConfig) {
	halSurface, err := b.registry.GetSurface(surface)
	if err != nil {
		return
	}

	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return
	}

	// Store surface → device mapping for Present()
	b.registry.RegisterSurfaceDevice(surface, device)

	// Convert config
	halConfig := &hal.SurfaceConfiguration{
		Format:      convertTextureFormat(config.Format),
		Width:       config.Width,
		Height:      config.Height,
		PresentMode: convertPresentMode(config.PresentMode),
		Usage:       convertTextureUsage(config.Usage),
		AlphaMode:   hal.CompositeAlphaMode(config.AlphaMode), //nolint:gosec // G115: AlphaMode values are 0-3
	}

	// Configure surface
	_ = halSurface.Configure(halDevice, halConfig)
}

// GetCurrentTexture gets the current surface texture.
func (b *Backend) GetCurrentTexture(surface types.Surface) (types.SurfaceTexture, error) {
	halSurface, err := b.registry.GetSurface(surface)
	if err != nil {
		return types.SurfaceTexture{Status: types.SurfaceStatusError}, err
	}

	// Acquire texture (fence=nil for now)
	acquired, err := halSurface.AcquireTexture(nil)
	if err != nil {
		// Map HAL errors to surface status
		return types.SurfaceTexture{Status: types.SurfaceStatusError}, err
	}

	// Store the SurfaceTexture for Present() to use later
	b.registry.SetCurrentSurfaceTexture(surface, acquired.Texture)

	// Register texture and return
	textureHandle := b.registry.RegisterTexture(acquired.Texture)

	return types.SurfaceTexture{
		Texture: textureHandle,
		Status:  types.SurfaceStatusSuccess,
	}, nil
}

// Present presents the surface.
func (b *Backend) Present(surface types.Surface) {
	// Get the HAL surface
	halSurface, err := b.registry.GetSurface(surface)
	if err != nil {
		return
	}

	// Get the SurfaceTexture stored in GetCurrentTexture
	surfaceTexture := b.registry.GetCurrentSurfaceTexture(surface)
	if surfaceTexture == nil {
		return
	}

	// Get the device for this surface (stored in ConfigureSurface)
	device, err := b.registry.GetDeviceForSurface(surface)
	if err != nil {
		return
	}

	// Get the queue for this device
	queueHandle, err := b.registry.GetQueueForDevice(device)
	if err != nil {
		return
	}

	halQueue, err := b.registry.GetQueue(queueHandle)
	if err != nil {
		return
	}

	// Present the surface texture via HAL queue
	_ = halQueue.Present(halSurface, surfaceTexture)

	// Clear the stored texture (it's consumed after Present)
	b.registry.ClearCurrentSurfaceTexture(surface)
}

// CreateShaderModuleWGSL creates a shader module from WGSL code.
func (b *Backend) CreateShaderModuleWGSL(device types.Device, code string) (types.ShaderModule, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, err
	}

	desc := &hal.ShaderModuleDescriptor{
		Label:  "shader",
		Source: hal.ShaderSource{WGSL: code},
	}

	module, err := halDevice.CreateShaderModule(desc)
	if err != nil {
		return 0, fmt.Errorf("native: failed to create shader module: %w", err)
	}

	handle := b.registry.RegisterShaderModule(module)
	return handle, nil
}

// CreateRenderPipeline creates a render pipeline.
func (b *Backend) CreateRenderPipeline(device types.Device, desc *types.RenderPipelineDescriptor) (types.RenderPipeline, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, err
	}

	// Get shader modules
	vertexShader, err := b.registry.GetShaderModule(desc.VertexShader)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := b.registry.GetShaderModule(desc.FragmentShader)
	if err != nil {
		return 0, err
	}

	// Build HAL descriptor
	halDesc := &hal.RenderPipelineDescriptor{
		Label:  desc.Label,
		Layout: nil, // Auto layout
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: desc.VertexEntryPoint,
			Buffers:    nil, // No vertex buffers for triangle
		},
		Primitive: wgputypes.PrimitiveState{
			Topology:  convertPrimitiveTopology(desc.Topology),
			FrontFace: convertFrontFace(desc.FrontFace),
			CullMode:  convertCullMode(desc.CullMode),
		},
		DepthStencil: nil, // No depth/stencil for triangle
		Multisample:  wgputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     fragmentShader,
			EntryPoint: desc.FragmentEntry,
			Targets: []wgputypes.ColorTargetState{
				{
					Format:    convertTextureFormat(desc.TargetFormat),
					Blend:     nil, // No blending for now
					WriteMask: wgputypes.ColorWriteMaskAll,
				},
			},
		},
	}

	pipeline, err := halDevice.CreateRenderPipeline(halDesc)
	if err != nil {
		return 0, fmt.Errorf("native: failed to create render pipeline: %w", err)
	}

	handle := b.registry.RegisterRenderPipeline(pipeline)
	return handle, nil
}

// CreateCommandEncoder creates a command encoder.
func (b *Backend) CreateCommandEncoder(device types.Device) types.CommandEncoder {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0
	}

	desc := &hal.CommandEncoderDescriptor{
		Label: "command_encoder",
	}

	encoder, err := halDevice.CreateCommandEncoder(desc)
	if err != nil {
		return 0
	}

	handle := b.registry.RegisterCommandEncoder(encoder)
	return handle
}

// BeginRenderPass begins a render pass.
func (b *Backend) BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass {
	halEncoder, err := b.registry.GetCommandEncoder(encoder)
	if err != nil {
		return 0
	}

	// Convert color attachments
	colorAttachments := make([]hal.RenderPassColorAttachment, 0, len(desc.ColorAttachments))
	for _, ca := range desc.ColorAttachments {
		view, err := b.registry.GetTextureView(ca.View)
		if err != nil {
			continue
		}

		colorAttachments = append(colorAttachments, hal.RenderPassColorAttachment{
			View:       view,
			LoadOp:     convertLoadOp(ca.LoadOp),
			StoreOp:    convertStoreOp(ca.StoreOp),
			ClearValue: wgputypes.Color{R: ca.ClearValue.R, G: ca.ClearValue.G, B: ca.ClearValue.B, A: ca.ClearValue.A},
		})
	}

	halDesc := &hal.RenderPassDescriptor{
		Label:            desc.Label,
		ColorAttachments: colorAttachments,
	}

	// Begin render pass
	pass := halEncoder.BeginRenderPass(halDesc)

	handle := b.registry.RegisterRenderPass(pass)
	return handle
}

// EndRenderPass ends a render pass.
func (b *Backend) EndRenderPass(pass types.RenderPass) {
	halPass, err := b.registry.GetRenderPass(pass)
	if err != nil {
		return
	}

	halPass.End()
}

// FinishEncoder finishes the command encoder.
func (b *Backend) FinishEncoder(encoder types.CommandEncoder) types.CommandBuffer {
	halEncoder, err := b.registry.GetCommandEncoder(encoder)
	if err != nil {
		return 0
	}

	cmdBuffer, err := halEncoder.EndEncoding()
	if err != nil {
		return 0
	}

	handle := b.registry.RegisterCommandBuffer(cmdBuffer)
	return handle
}

// Submit submits commands to the queue.
func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer) {
	halQueue, err := b.registry.GetQueue(queue)
	if err != nil {
		return
	}

	halCmdBuffer, err := b.registry.GetCommandBuffer(commands)
	if err != nil {
		return
	}

	// Attach drawable from current surface texture to command buffer (Metal requirement).
	// The drawable must be scheduled for presentation before commit.
	b.attachDrawableToCommandBuffer(halCmdBuffer)

	// Submit with no fence
	_ = halQueue.Submit([]hal.CommandBuffer{halCmdBuffer}, nil, 0)
}

// attachDrawableToCommandBuffer attaches the current drawable to a command buffer.
// This is required for Metal where presentDrawable: must be called before commit.
func (b *Backend) attachDrawableToCommandBuffer(cmdBuffer hal.CommandBuffer) {
	// Type-assert to Metal command buffer
	metalCmdBuffer, ok := cmdBuffer.(*metal.CommandBuffer)
	if !ok {
		return // Not Metal backend
	}

	// Find any current surface texture and get its drawable.
	// In practice, there's only one surface per frame.
	surfaceTexture := b.registry.GetAnySurfaceTexture()
	if surfaceTexture == nil {
		return
	}

	// Type-assert to Metal surface texture
	metalSurfaceTex, ok := surfaceTexture.(*metal.SurfaceTexture)
	if !ok {
		return
	}

	// Get drawable using accessor and attach to command buffer
	drawable := metalSurfaceTex.Drawable()
	if drawable != 0 {
		metalCmdBuffer.SetDrawable(drawable)
	}
}

// SetPipeline sets the render pipeline.
func (b *Backend) SetPipeline(pass types.RenderPass, pipeline types.RenderPipeline) {
	halPass, err := b.registry.GetRenderPass(pass)
	if err != nil {
		return
	}

	halPipeline, err := b.registry.GetRenderPipeline(pipeline)
	if err != nil {
		return
	}

	halPass.SetPipeline(halPipeline)
}

// Draw issues a draw call.
func (b *Backend) Draw(pass types.RenderPass, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	halPass, err := b.registry.GetRenderPass(pass)
	if err != nil {
		return
	}

	halPass.Draw(vertexCount, instanceCount, firstVertex, firstInstance)
}

// --- Texture operations (stubs for now) ---

func (b *Backend) CreateTexture(device types.Device, desc *types.TextureDescriptor) (types.Texture, error) {
	return 0, gpu.ErrNotImplemented
}

func (b *Backend) CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView {
	halTexture, err := b.registry.GetTexture(texture)
	if err != nil {
		return 0
	}

	halDevice, err := b.registry.GetDevice(types.Device(1)) // HACK: assume device handle is 1
	if err != nil {
		return 0
	}

	// Convert descriptor
	halDesc := &hal.TextureViewDescriptor{
		Format:          convertTextureFormat(desc.Format),
		Dimension:       convertTextureViewDimension(desc.Dimension),
		Aspect:          convertTextureAspect(desc.Aspect),
		BaseMipLevel:    desc.BaseMipLevel,
		MipLevelCount:   desc.MipLevelCount,
		BaseArrayLayer:  desc.BaseArrayLayer,
		ArrayLayerCount: desc.ArrayLayerCount,
	}

	view, err := halDevice.CreateTextureView(halTexture, halDesc)
	if err != nil {
		return 0
	}

	handle := b.registry.RegisterTextureView(view)
	return handle
}

func (b *Backend) WriteTexture(queue types.Queue, dst *types.ImageCopyTexture, data []byte, layout *types.ImageDataLayout, size *types.Extent3D) {
	// Not implemented yet
}

func (b *Backend) CreateSampler(device types.Device, desc *types.SamplerDescriptor) (types.Sampler, error) {
	return 0, gpu.ErrNotImplemented
}

func (b *Backend) CreateBuffer(device types.Device, desc *types.BufferDescriptor) (types.Buffer, error) {
	return 0, gpu.ErrNotImplemented
}

func (b *Backend) WriteBuffer(queue types.Queue, buffer types.Buffer, offset uint64, data []byte) {
	// Not implemented yet
}

func (b *Backend) CreateBindGroupLayout(device types.Device, desc *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error) {
	return 0, gpu.ErrNotImplemented
}

func (b *Backend) CreateBindGroup(device types.Device, desc *types.BindGroupDescriptor) (types.BindGroup, error) {
	return 0, gpu.ErrNotImplemented
}

func (b *Backend) CreatePipelineLayout(device types.Device, desc *types.PipelineLayoutDescriptor) (types.PipelineLayout, error) {
	return 0, gpu.ErrNotImplemented
}

func (b *Backend) SetBindGroup(pass types.RenderPass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32) {
	// Not implemented yet
}

func (b *Backend) SetVertexBuffer(pass types.RenderPass, slot uint32, buffer types.Buffer, offset, size uint64) {
	// Not implemented yet
}

func (b *Backend) SetIndexBuffer(pass types.RenderPass, buffer types.Buffer, format types.IndexFormat, offset, size uint64) {
	// Not implemented yet
}

func (b *Backend) DrawIndexed(pass types.RenderPass, indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	// Not implemented yet
}

// --- Resource release ---

func (b *Backend) ReleaseTexture(texture types.Texture) {
	halTexture, err := b.registry.GetTexture(texture)
	if err == nil && halTexture != nil {
		halTexture.Destroy()
	}
	b.registry.UnregisterTexture(texture)
}

func (b *Backend) ReleaseTextureView(view types.TextureView) {
	halView, err := b.registry.GetTextureView(view)
	if err == nil && halView != nil {
		halView.Destroy()
	}
	b.registry.UnregisterTextureView(view)
}

func (b *Backend) ReleaseSampler(sampler types.Sampler) {
	halSampler, err := b.registry.GetSampler(sampler)
	if err == nil && halSampler != nil {
		halSampler.Destroy()
	}
	b.registry.UnregisterSampler(sampler)
}

func (b *Backend) ReleaseBuffer(buffer types.Buffer) {
	halBuffer, err := b.registry.GetBuffer(buffer)
	if err == nil && halBuffer != nil {
		halBuffer.Destroy()
	}
	b.registry.UnregisterBuffer(buffer)
}

func (b *Backend) ReleaseBindGroupLayout(layout types.BindGroupLayout) {
	halLayout, err := b.registry.GetBindGroupLayout(layout)
	if err == nil && halLayout != nil {
		halLayout.Destroy()
	}
	b.registry.UnregisterBindGroupLayout(layout)
}

func (b *Backend) ReleaseBindGroup(group types.BindGroup) {
	halGroup, err := b.registry.GetBindGroup(group)
	if err == nil && halGroup != nil {
		halGroup.Destroy()
	}
	b.registry.UnregisterBindGroup(group)
}

func (b *Backend) ReleasePipelineLayout(layout types.PipelineLayout) {
	halLayout, err := b.registry.GetPipelineLayout(layout)
	if err == nil && halLayout != nil {
		halLayout.Destroy()
	}
	b.registry.UnregisterPipelineLayout(layout)
}

func (b *Backend) ReleaseCommandBuffer(buffer types.CommandBuffer) {
	halBuffer, err := b.registry.GetCommandBuffer(buffer)
	if err == nil && halBuffer != nil {
		halBuffer.Destroy()
	}
	b.registry.UnregisterCommandBuffer(buffer)
}

func (b *Backend) ReleaseCommandEncoder(encoder types.CommandEncoder) {
	// Command encoders don't have Destroy in HAL - they're consumed when EndEncoding() is called.
	// We just unregister the handle from the registry.
	b.registry.UnregisterCommandEncoder(encoder)
}

func (b *Backend) ReleaseRenderPass(pass types.RenderPass) {
	// Render passes are ended, not destroyed
	b.registry.UnregisterRenderPass(pass)
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
