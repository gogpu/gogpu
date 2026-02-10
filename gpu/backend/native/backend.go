//go:build windows || linux || darwin

// Package native provides the WebGPU backend using pure Go (gogpu/wgpu).
// This backend offers zero dependencies and simple cross-compilation.
//
// Implementation uses gogpu/wgpu HAL (Hardware Abstraction Layer) with
// platform-specific backends: Vulkan on Windows/Linux, Metal on macOS.
package native

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

// Backend implements gpu.Backend using pure Go wgpu HAL.
type Backend struct {
	registry    *ResourceRegistry
	backend     hal.Backend
	bufferSizes map[types.Buffer]uint64
}

// New creates a new Pure Go backend.
func New() *Backend {
	return &Backend{
		registry:    NewResourceRegistry(),
		backend:     newHalBackend(),
		bufferSizes: make(map[types.Buffer]uint64),
	}
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return halBackendName()
}

// Init initializes the backend.
func (b *Backend) Init() error {
	// Backend is stateless, no initialization needed
	// Actual initialization happens when creating instance
	return nil
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	// Wait for all GPU operations to complete before destroying resources.
	// This prevents hangs/crashes when closing the window.
	b.registry.WaitAllDevicesIdle()

	// Clear the registry (does not destroy HAL resources, but they will be GC'd)
	b.registry.Clear()
}

// CreateInstance creates a WebGPU instance.
func (b *Backend) CreateInstance() (types.Instance, error) {
	// Create HAL instance with default config
	desc := &hal.InstanceDescriptor{
		Backends: gputypes.Backends(1 << halBackendVariant()),
		Flags:    0, // No debug for now
	}

	halInstance, err := b.backend.CreateInstance(desc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create instance: %w", err)
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
		return 0, fmt.Errorf("gpu: no adapters found")
	}

	// Sort adapters based on power preference (matches wgpu-core behavior)
	preferIntegrated := opts != nil && opts.PowerPreference == gputypes.PowerPreferenceLowPower
	sort.SliceStable(adapters, func(i, j int) bool {
		return adapterOrder(adapters[i].Info.DeviceType, preferIntegrated) <
			adapterOrder(adapters[j].Info.DeviceType, preferIntegrated)
	})

	// Pick best adapter (first after sorting)
	exposed := adapters[0]

	// Register and return handle
	handle := b.registry.RegisterAdapter(exposed.Adapter)
	return handle, nil
}

// adapterOrder returns the priority order for adapter selection.
// Lower values = higher priority. Matches wgpu-core's request_adapter behavior.
func adapterOrder(deviceType gputypes.DeviceType, preferIntegrated bool) int {
	switch deviceType {
	case gputypes.DeviceTypeDiscreteGPU:
		if preferIntegrated {
			return 2
		}
		return 1 // Best for high performance
	case gputypes.DeviceTypeIntegratedGPU:
		if preferIntegrated {
			return 1 // Best for low power
		}
		return 2
	case gputypes.DeviceTypeOther:
		return 3 // Unknown (could be OpenGL)
	case gputypes.DeviceTypeVirtualGPU:
		return 4
	case gputypes.DeviceTypeCPU:
		return 5 // Software fallback (worst)
	default:
		return 6
	}
}

// RequestDevice requests a GPU device.
func (b *Backend) RequestDevice(adapter types.Adapter, opts *types.DeviceOptions) (types.Device, error) {
	halAdapter, err := b.registry.GetAdapter(adapter)
	if err != nil {
		return 0, err
	}

	// Open device with default features and limits
	openDevice, err := halAdapter.Open(gputypes.Features(0), gputypes.DefaultLimits())
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to open device: %w", err)
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
		return 0, fmt.Errorf("gpu: failed to create surface: %w", err)
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

	// Store surface -> device mapping for Present()
	b.registry.RegisterSurfaceDevice(surface, device)

	// Convert config - all types are now gputypes aliases, no conversion needed
	halConfig := &hal.SurfaceConfiguration{
		Format:      config.Format,
		Width:       config.Width,
		Height:      config.Height,
		PresentMode: config.PresentMode,
		Usage:       config.Usage,
		AlphaMode:   config.AlphaMode,
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

	// Acquire texture (non-blocking)
	acquired, err := halSurface.AcquireTexture(nil)
	if err != nil {
		// Check for "not ready" - this means skip frame, not an error
		if errors.Is(err, hal.ErrNotReady) {
			return types.SurfaceTexture{Status: types.SurfaceStatusTimeout}, nil
		}
		// Map HAL errors to surface status
		return types.SurfaceTexture{Status: types.SurfaceStatusError}, err
	}

	// Store the SurfaceTexture for Present() to use later
	b.registry.SetCurrentSurfaceTexture(surface, acquired.Texture)

	// Get device for this surface (stored in ConfigureSurface)
	device, err := b.registry.GetDeviceForSurface(surface)
	if err != nil {
		return types.SurfaceTexture{Status: types.SurfaceStatusError}, err
	}

	// Register texture WITH device (required for CreateTextureView)
	textureHandle := b.registry.RegisterTextureForDevice(acquired.Texture, device)

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
		return 0, fmt.Errorf("gpu: failed to create shader module: %w", err)
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

	// Get pipeline layout if provided
	var halLayout hal.PipelineLayout
	if desc.Layout != 0 {
		halLayout, err = b.registry.GetPipelineLayout(desc.Layout)
		if err != nil {
			return 0, fmt.Errorf("gpu: invalid pipeline layout: %w", err)
		}
	}

	// Build HAL descriptor - types are now gputypes aliases, no conversion needed
	halDesc := &hal.RenderPipelineDescriptor{
		Label:  desc.Label,
		Layout: halLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: desc.VertexEntryPoint,
			Buffers:    nil, // No vertex buffers for fullscreen quad
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  desc.Topology,
			FrontFace: desc.FrontFace,
			CullMode:  desc.CullMode,
		},
		DepthStencil: nil, // No depth/stencil
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     fragmentShader,
			EntryPoint: desc.FragmentEntry,
			Targets: []gputypes.ColorTargetState{
				{
					Format:    desc.TargetFormat,
					Blend:     desc.Blend,
					WriteMask: gputypes.ColorWriteMaskAll,
				},
			},
		},
	}

	pipeline, err := halDevice.CreateRenderPipeline(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create render pipeline: %w", err)
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

	// IMPORTANT: HAL requires BeginEncoding before any commands can be recorded.
	// This must be called before BeginRenderPass, BeginComputePass, etc.
	if err := encoder.BeginEncoding("frame"); err != nil {
		return 0
	}

	// Register encoder with device so we can free the command buffer to correct pool
	handle := b.registry.RegisterCommandEncoderForDevice(encoder, device)
	return handle
}

// BeginRenderPass begins a render pass.
func (b *Backend) BeginRenderPass(encoder types.CommandEncoder, desc *types.RenderPassDescriptor) types.RenderPass {
	halEncoder, err := b.registry.GetCommandEncoder(encoder)
	if err != nil {
		return 0
	}

	// Convert color attachments - types are gputypes aliases now
	colorAttachments := make([]hal.RenderPassColorAttachment, 0, len(desc.ColorAttachments))
	for _, ca := range desc.ColorAttachments {
		view, viewErr := b.registry.GetTextureView(ca.View)
		if viewErr != nil {
			continue
		}

		colorAttachments = append(colorAttachments, hal.RenderPassColorAttachment{
			View:       view,
			LoadOp:     ca.LoadOp,
			StoreOp:    ca.StoreOp,
			ClearValue: ca.ClearValue,
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

	// Get the device this encoder was created from (for proper command buffer freeing)
	device := b.registry.GetCommandEncoderDevice(encoder)

	cmdBuffer, err := halEncoder.EndEncoding()
	if err != nil {
		return 0
	}

	// Register command buffer with device so it can be freed to correct pool
	handle := b.registry.RegisterCommandBufferForDevice(cmdBuffer, device)
	return handle
}

// Submit submits commands to the queue with optional fence signaling.
// If fence is non-zero, it will be signaled with fenceValue when commands complete.
// Returns the submission index for tracking completion.
func (b *Backend) Submit(queue types.Queue, commands types.CommandBuffer, fence types.Fence, fenceValue uint64) types.SubmissionIndex {
	halQueue, err := b.registry.GetQueue(queue)
	if err != nil {
		return 0
	}

	halCmdBuffer, err := b.registry.GetCommandBuffer(commands)
	if err != nil {
		return 0
	}

	// Get HAL fence if provided
	var halFence hal.Fence
	if fence != 0 {
		halFence, _ = b.registry.GetFence(fence)
	}

	// Platform-specific pre-submit hook (Metal drawable attachment)
	platformPreSubmit(halCmdBuffer, b.registry)

	// Submit with fence signaling
	_ = halQueue.Submit([]hal.CommandBuffer{halCmdBuffer}, halFence, fenceValue)

	return types.SubmissionIndex(fenceValue)
}

// GetFenceStatus returns true if the fence is signaled (non-blocking).
func (b *Backend) GetFenceStatus(fence types.Fence) (bool, error) {
	halFence, err := b.registry.GetFence(fence)
	if err != nil {
		return false, err
	}

	deviceHandle, err := b.registry.GetFenceDevice(fence)
	if err != nil {
		return false, err
	}

	halDevice, err := b.registry.GetDevice(deviceHandle)
	if err != nil {
		return false, err
	}

	return halDevice.GetFenceStatus(halFence)
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

// --- Texture operations ---

// CreateTexture creates a GPU texture.
func (b *Backend) CreateTexture(device types.Device, desc *types.TextureDescriptor) (types.Texture, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	halDesc := &hal.TextureDescriptor{
		Label:         desc.Label,
		Size:          hal.Extent3D{Width: desc.Size.Width, Height: desc.Size.Height, DepthOrArrayLayers: desc.Size.DepthOrArrayLayers},
		MipLevelCount: desc.MipLevelCount,
		SampleCount:   desc.SampleCount,
		Dimension:     desc.Dimension,
		Format:        desc.Format,
		Usage:         desc.Usage,
	}

	texture, err := halDevice.CreateTexture(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create texture: %w", err)
	}

	handle := b.registry.RegisterTextureForDevice(texture, device)
	return handle, nil
}

// CreateTextureView creates a texture view.
func (b *Backend) CreateTextureView(texture types.Texture, desc *types.TextureViewDescriptor) types.TextureView {
	halTexture, err := b.registry.GetTexture(texture)
	if err != nil {
		return 0
	}

	// Get device for this texture (stored in RegisterTextureForDevice)
	deviceHandle, err := b.registry.GetDeviceForTexture(texture)
	if err != nil {
		return 0
	}

	halDevice, err := b.registry.GetDevice(deviceHandle)
	if err != nil {
		return 0
	}

	// Convert descriptor (nil is allowed - HAL will use defaults)
	var halDesc *hal.TextureViewDescriptor
	if desc != nil {
		halDesc = &hal.TextureViewDescriptor{
			Format:          desc.Format,
			Dimension:       desc.Dimension,
			Aspect:          desc.Aspect,
			BaseMipLevel:    desc.BaseMipLevel,
			MipLevelCount:   desc.MipLevelCount,
			BaseArrayLayer:  desc.BaseArrayLayer,
			ArrayLayerCount: desc.ArrayLayerCount,
		}
	}

	view, err := halDevice.CreateTextureView(halTexture, halDesc)
	if err != nil {
		return 0
	}

	handle := b.registry.RegisterTextureView(view)
	return handle
}

// WriteTexture writes data to a texture.
func (b *Backend) WriteTexture(queue types.Queue, dst *types.ImageCopyTexture, data []byte, layout *types.ImageDataLayout, size *gputypes.Extent3D) {
	halQueue, err := b.registry.GetQueue(queue)
	if err != nil {
		return // Silent fail for now, matches wgpu behavior
	}

	halTexture, err := b.registry.GetTexture(dst.Texture)
	if err != nil {
		return
	}

	halDst := &hal.ImageCopyTexture{
		Texture:  halTexture,
		MipLevel: dst.MipLevel,
		Origin:   hal.Origin3D{X: dst.Origin.X, Y: dst.Origin.Y, Z: dst.Origin.Z},
		Aspect:   dst.Aspect,
	}

	halLayout := &hal.ImageDataLayout{
		Offset:       layout.Offset,
		BytesPerRow:  layout.BytesPerRow,
		RowsPerImage: layout.RowsPerImage,
	}

	halSize := &hal.Extent3D{
		Width:              size.Width,
		Height:             size.Height,
		DepthOrArrayLayers: size.DepthOrArrayLayers,
	}

	halQueue.WriteTexture(halDst, data, halLayout, halSize)
}

// CreateSampler creates a texture sampler.
func (b *Backend) CreateSampler(device types.Device, desc *types.SamplerDescriptor) (types.Sampler, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	halDesc := &hal.SamplerDescriptor{
		Label:        desc.Label,
		AddressModeU: desc.AddressModeU,
		AddressModeV: desc.AddressModeV,
		AddressModeW: desc.AddressModeW,
		MagFilter:    desc.MagFilter,
		MinFilter:    desc.MinFilter,
		MipmapFilter: gputypes.FilterMode(desc.MipmapFilter),
		LodMinClamp:  desc.LodMinClamp,
		LodMaxClamp:  desc.LodMaxClamp,
		Compare:      desc.Compare,
		Anisotropy:   desc.MaxAnisotropy,
	}

	sampler, err := halDevice.CreateSampler(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create sampler: %w", err)
	}

	handle := b.registry.RegisterSampler(sampler)
	return handle, nil
}

// CreateBuffer creates a GPU buffer.
func (b *Backend) CreateBuffer(device types.Device, desc *types.BufferDescriptor) (types.Buffer, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	// For uniform/copy-dst buffers, we need mapped memory for WriteBuffer to work
	// Native HAL doesn't have staging buffer support yet, so we use host-visible memory
	mappedAtCreation := desc.MappedAtCreation
	if desc.Usage&gputypes.BufferUsageCopyDst != 0 {
		mappedAtCreation = true
	}

	halDesc := &hal.BufferDescriptor{
		Label:            desc.Label,
		Size:             desc.Size,
		Usage:            desc.Usage,
		MappedAtCreation: mappedAtCreation,
	}

	buffer, err := halDevice.CreateBuffer(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create buffer: %w", err)
	}

	handle := b.registry.RegisterBuffer(buffer)
	b.bufferSizes[handle] = desc.Size
	return handle, nil
}

// WriteBuffer writes data to a buffer.
func (b *Backend) WriteBuffer(queue types.Queue, buffer types.Buffer, offset uint64, data []byte) {
	halQueue, err := b.registry.GetQueue(queue)
	if err != nil {
		return // Silent fail, matching Rust backend behavior
	}

	halBuffer, err := b.registry.GetBuffer(buffer)
	if err != nil {
		return
	}

	halQueue.WriteBuffer(halBuffer, offset, data)
}

// CopyBufferToBuffer records a buffer-to-buffer copy command.
func (b *Backend) CopyBufferToBuffer(encoder types.CommandEncoder, src types.Buffer, srcOffset uint64, dst types.Buffer, dstOffset, size uint64) {
	halEncoder, err := b.registry.GetCommandEncoder(encoder)
	if err != nil {
		return
	}
	halSrc, err := b.registry.GetBuffer(src)
	if err != nil {
		return
	}
	halDst, err := b.registry.GetBuffer(dst)
	if err != nil {
		return
	}
	halEncoder.CopyBufferToBuffer(halSrc, halDst, []hal.BufferCopy{
		{SrcOffset: srcOffset, DstOffset: dstOffset, Size: size},
	})
}

// CreateBindGroupLayout creates a bind group layout.
func (b *Backend) CreateBindGroupLayout(device types.Device, desc *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	// Convert entries to HAL format
	halEntries := make([]gputypes.BindGroupLayoutEntry, len(desc.Entries))
	for i, entry := range desc.Entries {
		halEntries[i] = gputypes.BindGroupLayoutEntry{
			Binding:    entry.Binding,
			Visibility: entry.Visibility,
			Buffer:     entry.Buffer,
			Sampler:    entry.Sampler,
			Texture:    entry.Texture,
		}
	}

	halDesc := &hal.BindGroupLayoutDescriptor{
		Label:   desc.Label,
		Entries: halEntries,
	}

	layout, err := halDevice.CreateBindGroupLayout(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create bind group layout: %w", err)
	}

	handle := b.registry.RegisterBindGroupLayout(layout)
	return handle, nil
}

// CreateBindGroup creates a bind group.
func (b *Backend) CreateBindGroup(device types.Device, desc *types.BindGroupDescriptor) (types.BindGroup, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	halLayout, err := b.registry.GetBindGroupLayout(desc.Layout)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid bind group layout: %w", err)
	}

	// Convert entries - need to resolve handles to gpu handles
	halEntries := make([]gputypes.BindGroupEntry, len(desc.Entries))
	for i, entry := range desc.Entries {
		halEntries[i] = gputypes.BindGroupEntry{
			Binding: entry.Binding,
		}

		// Determine which resource is set and convert using NativeHandle
		switch {
		case entry.Buffer != 0:
			halBuffer, bufErr := b.registry.GetBuffer(entry.Buffer)
			if bufErr != nil {
				return 0, fmt.Errorf("gpu: invalid buffer in bind group entry %d: %w", i, bufErr)
			}
			halEntries[i].Resource = gputypes.BufferBinding{
				Buffer: halBuffer.NativeHandle(),
				Offset: entry.Offset,
				Size:   entry.Size,
			}
		case entry.Sampler != 0:
			halSampler, sampErr := b.registry.GetSampler(entry.Sampler)
			if sampErr != nil {
				return 0, fmt.Errorf("gpu: invalid sampler in bind group entry %d: %w", i, sampErr)
			}
			halEntries[i].Resource = gputypes.SamplerBinding{
				Sampler: halSampler.NativeHandle(),
			}
		case entry.TextureView != 0:
			halView, viewErr := b.registry.GetTextureView(entry.TextureView)
			if viewErr != nil {
				return 0, fmt.Errorf("gpu: invalid texture view in bind group entry %d: %w", i, viewErr)
			}
			halEntries[i].Resource = gputypes.TextureViewBinding{
				TextureView: halView.NativeHandle(),
			}
		default:
			return 0, fmt.Errorf("gpu: bind group entry %d has no resource", i)
		}
	}

	halDesc := &hal.BindGroupDescriptor{
		Label:   desc.Label,
		Layout:  halLayout,
		Entries: halEntries,
	}

	group, err := halDevice.CreateBindGroup(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create bind group: %w", err)
	}

	handle := b.registry.RegisterBindGroup(group)
	return handle, nil
}

// CreatePipelineLayout creates a pipeline layout.
func (b *Backend) CreatePipelineLayout(device types.Device, desc *types.PipelineLayoutDescriptor) (types.PipelineLayout, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	// Convert bind group layouts
	halLayouts := make([]hal.BindGroupLayout, len(desc.BindGroupLayouts))
	for i, layout := range desc.BindGroupLayouts {
		halLayout, layoutErr := b.registry.GetBindGroupLayout(layout)
		if layoutErr != nil {
			return 0, fmt.Errorf("gpu: invalid bind group layout at index %d: %w", i, layoutErr)
		}
		halLayouts[i] = halLayout
	}

	halDesc := &hal.PipelineLayoutDescriptor{
		Label:            desc.Label,
		BindGroupLayouts: halLayouts,
	}

	layout, err := halDevice.CreatePipelineLayout(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create pipeline layout: %w", err)
	}

	handle := b.registry.RegisterPipelineLayout(layout)
	return handle, nil
}

// SetBindGroup sets a bind group for a render pass.
func (b *Backend) SetBindGroup(pass types.RenderPass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32) {
	halPass, err := b.registry.GetRenderPass(pass)
	if err != nil {
		return
	}

	halGroup, err := b.registry.GetBindGroup(bindGroup)
	if err != nil {
		return
	}

	halPass.SetBindGroup(index, halGroup, dynamicOffsets)
}

// SetVertexBuffer sets a vertex buffer for a render pass.
func (b *Backend) SetVertexBuffer(pass types.RenderPass, slot uint32, buffer types.Buffer, offset, size uint64) {
	// Not implemented yet
}

// SetIndexBuffer sets an index buffer for a render pass.
func (b *Backend) SetIndexBuffer(pass types.RenderPass, buffer types.Buffer, format gputypes.IndexFormat, offset, size uint64) {
	// Not implemented yet
}

// DrawIndexed issues an indexed draw call.
func (b *Backend) DrawIndexed(pass types.RenderPass, indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	// Not implemented yet
}

// --- Compute shader operations ---

// CreateShaderModuleSPIRV creates a shader module from SPIR-V bytecode.
func (b *Backend) CreateShaderModuleSPIRV(device types.Device, spirv []uint32) (types.ShaderModule, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, err
	}

	desc := &hal.ShaderModuleDescriptor{
		Label:  "shader-spirv",
		Source: hal.ShaderSource{SPIRV: spirv},
	}

	module, err := halDevice.CreateShaderModule(desc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create SPIR-V shader module: %w", err)
	}

	handle := b.registry.RegisterShaderModule(module)
	return handle, nil
}

// CreateComputePipeline creates a compute pipeline.
func (b *Backend) CreateComputePipeline(device types.Device, desc *types.ComputePipelineDescriptor) (types.ComputePipeline, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, err
	}

	// Get shader module
	halModule, err := b.registry.GetShaderModule(desc.Module)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid compute shader module: %w", err)
	}

	// Get pipeline layout if provided
	var halLayout hal.PipelineLayout
	if desc.Layout != 0 {
		halLayout, err = b.registry.GetPipelineLayout(desc.Layout)
		if err != nil {
			return 0, fmt.Errorf("gpu: invalid pipeline layout: %w", err)
		}
	}

	halDesc := &hal.ComputePipelineDescriptor{
		Label:  desc.Label,
		Layout: halLayout,
		Compute: hal.ComputeState{
			Module:     halModule,
			EntryPoint: desc.EntryPoint,
		},
	}

	pipeline, err := halDevice.CreateComputePipeline(halDesc)
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create compute pipeline: %w", err)
	}

	handle := b.registry.RegisterComputePipeline(pipeline)
	return handle, nil
}

// BeginComputePass begins a compute pass.
func (b *Backend) BeginComputePass(encoder types.CommandEncoder) types.ComputePass {
	halEncoder, err := b.registry.GetCommandEncoder(encoder)
	if err != nil {
		return 0
	}

	halDesc := &hal.ComputePassDescriptor{
		Label: "compute_pass",
	}

	pass := halEncoder.BeginComputePass(halDesc)

	handle := b.registry.RegisterComputePass(pass)
	return handle
}

// EndComputePass ends a compute pass.
func (b *Backend) EndComputePass(pass types.ComputePass) {
	halPass, err := b.registry.GetComputePass(pass)
	if err != nil {
		return
	}

	halPass.End()
}

// SetComputePipeline sets the compute pipeline for a compute pass.
func (b *Backend) SetComputePipeline(pass types.ComputePass, pipeline types.ComputePipeline) {
	halPass, err := b.registry.GetComputePass(pass)
	if err != nil {
		return
	}

	halPipeline, err := b.registry.GetComputePipeline(pipeline)
	if err != nil {
		return
	}

	halPass.SetPipeline(halPipeline)
}

// SetComputeBindGroup sets a bind group for a compute pass.
func (b *Backend) SetComputeBindGroup(pass types.ComputePass, index uint32, bindGroup types.BindGroup, dynamicOffsets []uint32) {
	halPass, err := b.registry.GetComputePass(pass)
	if err != nil {
		return
	}

	halGroup, err := b.registry.GetBindGroup(bindGroup)
	if err != nil {
		return
	}

	halPass.SetBindGroup(index, halGroup, dynamicOffsets)
}

// DispatchWorkgroups dispatches compute work.
func (b *Backend) DispatchWorkgroups(pass types.ComputePass, x, y, z uint32) {
	halPass, err := b.registry.GetComputePass(pass)
	if err != nil {
		return
	}

	halPass.Dispatch(x, y, z)
}

// MapBufferRead reads a buffer's contents from the GPU.
// The buffer must have been created with BufferUsageMapRead | BufferUsageCopyDst.
func (b *Backend) MapBufferRead(buffer types.Buffer) ([]byte, error) {
	halBuffer, err := b.registry.GetBuffer(buffer)
	if err != nil {
		return nil, fmt.Errorf("gpu: invalid buffer: %w", err)
	}

	size, ok := b.bufferSizes[buffer]
	if !ok {
		return nil, fmt.Errorf("gpu: unknown buffer size for handle %d", buffer)
	}

	halQueue := b.registry.GetAnyQueue()
	if halQueue == nil {
		return nil, fmt.Errorf("gpu: no queue available for ReadBuffer")
	}

	data := make([]byte, size)
	if err := halQueue.ReadBuffer(halBuffer, 0, data); err != nil {
		return nil, fmt.Errorf("gpu: ReadBuffer failed: %w", err)
	}
	return data, nil
}

// UnmapBuffer is a no-op for the native backend.
// ReadBuffer returns a copy of the data, so no unmapping is needed.
func (b *Backend) UnmapBuffer(buffer types.Buffer) {
	// No-op: ReadBuffer copies data, no persistent mapping to release.
}

// --- Resource release ---

// ReleaseTexture releases a texture.
func (b *Backend) ReleaseTexture(texture types.Texture) {
	halTexture, err := b.registry.GetTexture(texture)
	if err == nil && halTexture != nil {
		halTexture.Destroy()
	}
	b.registry.UnregisterTexture(texture)
}

// ReleaseTextureView releases a texture view.
func (b *Backend) ReleaseTextureView(view types.TextureView) {
	halView, err := b.registry.GetTextureView(view)
	if err == nil && halView != nil {
		halView.Destroy()
	}
	b.registry.UnregisterTextureView(view)
}

// ReleaseSampler releases a sampler.
func (b *Backend) ReleaseSampler(sampler types.Sampler) {
	halSampler, err := b.registry.GetSampler(sampler)
	if err == nil && halSampler != nil {
		halSampler.Destroy()
	}
	b.registry.UnregisterSampler(sampler)
}

// ReleaseBuffer releases a buffer.
func (b *Backend) ReleaseBuffer(buffer types.Buffer) {
	halBuffer, err := b.registry.GetBuffer(buffer)
	if err == nil && halBuffer != nil {
		halBuffer.Destroy()
	}
	b.registry.UnregisterBuffer(buffer)
	delete(b.bufferSizes, buffer)
}

// ReleaseBindGroupLayout releases a bind group layout.
func (b *Backend) ReleaseBindGroupLayout(layout types.BindGroupLayout) {
	halLayout, err := b.registry.GetBindGroupLayout(layout)
	if err == nil && halLayout != nil {
		halLayout.Destroy()
	}
	b.registry.UnregisterBindGroupLayout(layout)
}

// ReleaseBindGroup releases a bind group.
func (b *Backend) ReleaseBindGroup(group types.BindGroup) {
	halGroup, err := b.registry.GetBindGroup(group)
	if err == nil && halGroup != nil {
		halGroup.Destroy()
	}
	b.registry.UnregisterBindGroup(group)
}

// ReleasePipelineLayout releases a pipeline layout.
func (b *Backend) ReleasePipelineLayout(layout types.PipelineLayout) {
	halLayout, err := b.registry.GetPipelineLayout(layout)
	if err == nil && halLayout != nil {
		halLayout.Destroy()
	}
	b.registry.UnregisterPipelineLayout(layout)
}

// ReleaseCommandBuffer releases a command buffer.
func (b *Backend) ReleaseCommandBuffer(buffer types.CommandBuffer) {
	halBuffer, err := b.registry.GetCommandBuffer(buffer)
	if err == nil && halBuffer != nil {
		// Get device to free command buffer back to pool
		deviceHandle := b.registry.GetCommandBufferDevice(buffer)
		if deviceHandle != 0 {
			halDevice, devErr := b.registry.GetDevice(deviceHandle)
			if devErr == nil {
				halDevice.FreeCommandBuffer(halBuffer)
			}
		}
	}
	b.registry.UnregisterCommandBuffer(buffer)
}

// ReleaseCommandEncoder releases a command encoder.
func (b *Backend) ReleaseCommandEncoder(encoder types.CommandEncoder) {
	// Command encoders don't have Destroy in HAL - they're consumed when EndEncoding() is called.
	// We just unregister the handle from the registry.
	b.registry.UnregisterCommandEncoder(encoder)
}

// ReleaseRenderPass releases a render pass.
func (b *Backend) ReleaseRenderPass(pass types.RenderPass) {
	// Render passes are ended, not destroyed
	b.registry.UnregisterRenderPass(pass)
}

// ReleaseComputePipeline releases a compute pipeline.
func (b *Backend) ReleaseComputePipeline(pipeline types.ComputePipeline) {
	halPipeline, err := b.registry.GetComputePipeline(pipeline)
	if err == nil && halPipeline != nil {
		halPipeline.Destroy()
	}
	b.registry.UnregisterComputePipeline(pipeline)
}

// ReleaseComputePass releases a compute pass.
func (b *Backend) ReleaseComputePass(pass types.ComputePass) {
	// Compute passes are ended, not destroyed
	b.registry.UnregisterComputePass(pass)
}

// ReleaseShaderModule releases a shader module.
func (b *Backend) ReleaseShaderModule(module types.ShaderModule) {
	halModule, err := b.registry.GetShaderModule(module)
	if err == nil && halModule != nil {
		halModule.Destroy()
	}
	b.registry.UnregisterShaderModule(module)
}

// ResetCommandPool resets the command pool to reclaim command buffer memory.
// This waits for GPU to finish all operations first (blocking).
func (b *Backend) ResetCommandPool(device types.Device) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return
	}

	// Type assert to access ResetCommandPool (available on backends that support it)
	type commandPoolResetter interface {
		WaitIdle() error
		ResetCommandPool() error
	}
	if resetter, ok := halDevice.(commandPoolResetter); ok {
		// Wait for GPU to finish using command buffers
		_ = resetter.WaitIdle()
		// Reset the pool to reclaim memory
		_ = resetter.ResetCommandPool()
	}
}

// CreateFence creates a new fence in the unsignaled state.
func (b *Backend) CreateFence(device types.Device) (types.Fence, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return 0, fmt.Errorf("gpu: invalid device: %w", err)
	}

	halFence, err := halDevice.CreateFence()
	if err != nil {
		return 0, fmt.Errorf("gpu: failed to create fence: %w", err)
	}

	handle := b.registry.RegisterFence(halFence, device)
	return handle, nil
}

// WaitFence waits for a fence to be signaled.
func (b *Backend) WaitFence(device types.Device, fence types.Fence, timeout uint64) (bool, error) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return false, fmt.Errorf("gpu: invalid device: %w", err)
	}

	halFence, err := b.registry.GetFence(fence)
	if err != nil {
		return false, fmt.Errorf("gpu: invalid fence: %w", err)
	}

	// Convert timeout from nanoseconds to time.Duration.
	// Max int64 is ~292 years in nanoseconds - any practical timeout is safe.
	return halDevice.Wait(halFence, 0, time.Duration(timeout)) //nolint:gosec // G115: practical timeouts won't overflow
}

// ResetFence resets a fence to the unsignaled state.
func (b *Backend) ResetFence(device types.Device, fence types.Fence) error {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return fmt.Errorf("gpu: invalid device: %w", err)
	}

	halFence, err := b.registry.GetFence(fence)
	if err != nil {
		return fmt.Errorf("gpu: invalid fence: %w", err)
	}

	return halDevice.ResetFence(halFence)
}

// DestroyFence destroys a fence.
func (b *Backend) DestroyFence(device types.Device, fence types.Fence) {
	halDevice, err := b.registry.GetDevice(device)
	if err != nil {
		return
	}

	halFence, err := b.registry.GetFence(fence)
	if err != nil {
		return
	}

	halDevice.DestroyFence(halFence)
	b.registry.UnregisterFence(fence)
}

// GetHalDevice returns the underlying HAL device for the given handle.
func (b *Backend) GetHalDevice(device types.Device) any {
	dev, err := b.registry.GetDevice(device)
	if err != nil {
		return nil
	}
	return dev
}

// GetHalQueue returns the underlying HAL queue for the given handle.
func (b *Backend) GetHalQueue(queue types.Queue) any {
	q, err := b.registry.GetQueue(queue)
	if err != nil {
		return nil
	}
	return q
}

// Ensure Backend implements gpu.Backend.
var _ gpu.Backend = (*Backend)(nil)
