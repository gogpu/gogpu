//go:build windows || linux || darwin

package native

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/wgpu/hal"
)

// ResourceRegistry maps uintptr handles (gogpu) to interface objects (wgpu/hal).
// This is the bridge between gogpu's handle-based API and wgpu's interface-based HAL.
//
// Thread-safe: All operations use sync.RWMutex for concurrent access.
type ResourceRegistry struct {
	mu sync.RWMutex

	// nextHandle generates unique handle IDs.
	// We start at 1 to avoid confusion with zero values.
	nextHandle atomic.Uint64

	// Resource maps - uintptr handles → HAL objects
	instances             map[types.Instance]hal.Instance
	adapters              map[types.Adapter]hal.Adapter
	devices               map[types.Device]hal.Device
	queues                map[types.Queue]hal.Queue
	surfaces              map[types.Surface]hal.Surface
	textures              map[types.Texture]hal.Texture
	textureDevices        map[types.Texture]types.Device
	textureViews          map[types.TextureView]hal.TextureView
	shaderModules         map[types.ShaderModule]hal.ShaderModule
	renderPipelines       map[types.RenderPipeline]hal.RenderPipeline
	commandEncoders       map[types.CommandEncoder]hal.CommandEncoder
	commandEncoderDevices map[types.CommandEncoder]types.Device // Track device for encoder's command buffer
	commandBuffers        map[types.CommandBuffer]hal.CommandBuffer
	commandBufferDevices  map[types.CommandBuffer]types.Device // Track device for command buffer freeing
	renderPasses          map[types.RenderPass]hal.RenderPassEncoder
	computePipelines      map[types.ComputePipeline]hal.ComputePipeline
	computePasses         map[types.ComputePass]hal.ComputePassEncoder
	buffers               map[types.Buffer]hal.Buffer
	samplers              map[types.Sampler]hal.Sampler
	bindGroupLayouts      map[types.BindGroupLayout]hal.BindGroupLayout
	bindGroups            map[types.BindGroup]hal.BindGroup
	pipelineLayouts       map[types.PipelineLayout]hal.PipelineLayout
	fences                map[types.Fence]hal.Fence
	fenceDevices          map[types.Fence]types.Device // Track device for fence operations

	// Device → Queue mapping (one queue per device in WebGPU)
	deviceQueues map[types.Device]types.Queue

	// Surface → Device mapping (for Present to find queue)
	surfaceDevices map[types.Surface]types.Device

	// Surface → current SurfaceTexture mapping (for Present)
	currentSurfaceTextures map[types.Surface]hal.SurfaceTexture

	// Reverse maps for cleanup - HAL objects → handles
	instanceHandles        map[hal.Instance]types.Instance
	adapterHandles         map[hal.Adapter]types.Adapter
	deviceHandles          map[hal.Device]types.Device
	queueHandles           map[hal.Queue]types.Queue
	surfaceHandles         map[hal.Surface]types.Surface
	textureHandles         map[hal.Texture]types.Texture
	textureViewHandles     map[hal.TextureView]types.TextureView
	shaderModuleHandles    map[hal.ShaderModule]types.ShaderModule
	renderPipelineHandles  map[hal.RenderPipeline]types.RenderPipeline
	commandEncoderHandles  map[hal.CommandEncoder]types.CommandEncoder
	commandBufferHandles   map[hal.CommandBuffer]types.CommandBuffer
	renderPassHandles      map[hal.RenderPassEncoder]types.RenderPass
	computePipelineHandles map[hal.ComputePipeline]types.ComputePipeline
	computePassHandles     map[hal.ComputePassEncoder]types.ComputePass
	bufferHandles          map[hal.Buffer]types.Buffer
	samplerHandles         map[hal.Sampler]types.Sampler
	bindGroupLayoutHandles map[hal.BindGroupLayout]types.BindGroupLayout
	bindGroupHandles       map[hal.BindGroup]types.BindGroup
	pipelineLayoutHandles  map[hal.PipelineLayout]types.PipelineLayout
	fenceHandles           map[hal.Fence]types.Fence
}

// NewResourceRegistry creates a new empty registry.
func NewResourceRegistry() *ResourceRegistry {
	r := &ResourceRegistry{
		instances:             make(map[types.Instance]hal.Instance),
		adapters:              make(map[types.Adapter]hal.Adapter),
		devices:               make(map[types.Device]hal.Device),
		queues:                make(map[types.Queue]hal.Queue),
		surfaces:              make(map[types.Surface]hal.Surface),
		textures:              make(map[types.Texture]hal.Texture),
		textureDevices:        make(map[types.Texture]types.Device),
		textureViews:          make(map[types.TextureView]hal.TextureView),
		shaderModules:         make(map[types.ShaderModule]hal.ShaderModule),
		renderPipelines:       make(map[types.RenderPipeline]hal.RenderPipeline),
		commandEncoders:       make(map[types.CommandEncoder]hal.CommandEncoder),
		commandEncoderDevices: make(map[types.CommandEncoder]types.Device),
		commandBuffers:        make(map[types.CommandBuffer]hal.CommandBuffer),
		commandBufferDevices:  make(map[types.CommandBuffer]types.Device),
		renderPasses:          make(map[types.RenderPass]hal.RenderPassEncoder),
		computePipelines:      make(map[types.ComputePipeline]hal.ComputePipeline),
		computePasses:         make(map[types.ComputePass]hal.ComputePassEncoder),
		buffers:               make(map[types.Buffer]hal.Buffer),
		samplers:              make(map[types.Sampler]hal.Sampler),
		bindGroupLayouts:      make(map[types.BindGroupLayout]hal.BindGroupLayout),
		bindGroups:            make(map[types.BindGroup]hal.BindGroup),
		pipelineLayouts:       make(map[types.PipelineLayout]hal.PipelineLayout),
		fences:                make(map[types.Fence]hal.Fence),
		fenceDevices:          make(map[types.Fence]types.Device),

		deviceQueues:           make(map[types.Device]types.Queue),
		surfaceDevices:         make(map[types.Surface]types.Device),
		currentSurfaceTextures: make(map[types.Surface]hal.SurfaceTexture),

		instanceHandles:        make(map[hal.Instance]types.Instance),
		adapterHandles:         make(map[hal.Adapter]types.Adapter),
		deviceHandles:          make(map[hal.Device]types.Device),
		queueHandles:           make(map[hal.Queue]types.Queue),
		surfaceHandles:         make(map[hal.Surface]types.Surface),
		textureHandles:         make(map[hal.Texture]types.Texture),
		textureViewHandles:     make(map[hal.TextureView]types.TextureView),
		shaderModuleHandles:    make(map[hal.ShaderModule]types.ShaderModule),
		renderPipelineHandles:  make(map[hal.RenderPipeline]types.RenderPipeline),
		commandEncoderHandles:  make(map[hal.CommandEncoder]types.CommandEncoder),
		commandBufferHandles:   make(map[hal.CommandBuffer]types.CommandBuffer),
		renderPassHandles:      make(map[hal.RenderPassEncoder]types.RenderPass),
		computePipelineHandles: make(map[hal.ComputePipeline]types.ComputePipeline),
		computePassHandles:     make(map[hal.ComputePassEncoder]types.ComputePass),
		bufferHandles:          make(map[hal.Buffer]types.Buffer),
		samplerHandles:         make(map[hal.Sampler]types.Sampler),
		bindGroupLayoutHandles: make(map[hal.BindGroupLayout]types.BindGroupLayout),
		bindGroupHandles:       make(map[hal.BindGroup]types.BindGroup),
		pipelineLayoutHandles:  make(map[hal.PipelineLayout]types.PipelineLayout),
		fenceHandles:           make(map[hal.Fence]types.Fence),
	}
	// Start handles at 1 to avoid zero confusion
	r.nextHandle.Store(1)
	return r
}

// newHandle generates a new unique handle.
func (r *ResourceRegistry) newHandle() uintptr {
	return uintptr(r.nextHandle.Add(1))
}

// --- Instance ---

func (r *ResourceRegistry) RegisterInstance(instance hal.Instance) types.Instance {
	handle := types.Instance(r.newHandle())
	r.mu.Lock()
	r.instances[handle] = instance
	r.instanceHandles[instance] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetInstance(handle types.Instance) (hal.Instance, error) {
	r.mu.RLock()
	instance, ok := r.instances[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid instance handle: %d", handle)
	}
	return instance, nil
}

func (r *ResourceRegistry) UnregisterInstance(handle types.Instance) {
	r.mu.Lock()
	if instance, ok := r.instances[handle]; ok {
		delete(r.instances, handle)
		delete(r.instanceHandles, instance)
	}
	r.mu.Unlock()
}

// --- Adapter ---

func (r *ResourceRegistry) RegisterAdapter(adapter hal.Adapter) types.Adapter {
	handle := types.Adapter(r.newHandle())
	r.mu.Lock()
	r.adapters[handle] = adapter
	r.adapterHandles[adapter] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetAdapter(handle types.Adapter) (hal.Adapter, error) {
	r.mu.RLock()
	adapter, ok := r.adapters[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid adapter handle: %d", handle)
	}
	return adapter, nil
}

func (r *ResourceRegistry) UnregisterAdapter(handle types.Adapter) {
	r.mu.Lock()
	if adapter, ok := r.adapters[handle]; ok {
		delete(r.adapters, handle)
		delete(r.adapterHandles, adapter)
	}
	r.mu.Unlock()
}

// --- Device ---

func (r *ResourceRegistry) RegisterDevice(device hal.Device) types.Device {
	handle := types.Device(r.newHandle())
	r.mu.Lock()
	r.devices[handle] = device
	r.deviceHandles[device] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetDevice(handle types.Device) (hal.Device, error) {
	r.mu.RLock()
	device, ok := r.devices[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid device handle: %d", handle)
	}
	return device, nil
}

func (r *ResourceRegistry) UnregisterDevice(handle types.Device) {
	r.mu.Lock()
	if device, ok := r.devices[handle]; ok {
		delete(r.devices, handle)
		delete(r.deviceHandles, device)
	}
	r.mu.Unlock()
}

// --- Queue ---

func (r *ResourceRegistry) RegisterQueue(queue hal.Queue) types.Queue {
	handle := types.Queue(r.newHandle())
	r.mu.Lock()
	r.queues[handle] = queue
	r.queueHandles[queue] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetQueue(handle types.Queue) (hal.Queue, error) {
	r.mu.RLock()
	queue, ok := r.queues[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid queue handle: %d", handle)
	}
	return queue, nil
}

// RegisterDeviceQueue stores the device→queue mapping.
func (r *ResourceRegistry) RegisterDeviceQueue(device types.Device, queue types.Queue) {
	r.mu.Lock()
	r.deviceQueues[device] = queue
	r.mu.Unlock()
}

// GetQueueForDevice returns the queue handle associated with a device.
func (r *ResourceRegistry) GetQueueForDevice(device types.Device) (types.Queue, error) {
	r.mu.RLock()
	queue, ok := r.deviceQueues[device]
	r.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("no queue found for device handle: %d", device)
	}
	return queue, nil
}

// RegisterSurfaceDevice stores the surface→device mapping for Present.
func (r *ResourceRegistry) RegisterSurfaceDevice(surface types.Surface, device types.Device) {
	r.mu.Lock()
	r.surfaceDevices[surface] = device
	r.mu.Unlock()
}

// GetDeviceForSurface returns the device handle associated with a surface.
func (r *ResourceRegistry) GetDeviceForSurface(surface types.Surface) (types.Device, error) {
	r.mu.RLock()
	device, ok := r.surfaceDevices[surface]
	r.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("no device found for surface handle: %d", surface)
	}
	return device, nil
}

// SetCurrentSurfaceTexture stores the current surface texture for Present.
func (r *ResourceRegistry) SetCurrentSurfaceTexture(surface types.Surface, texture hal.SurfaceTexture) {
	r.mu.Lock()
	r.currentSurfaceTextures[surface] = texture
	r.mu.Unlock()
}

// GetCurrentSurfaceTexture returns the current surface texture for Present.
func (r *ResourceRegistry) GetCurrentSurfaceTexture(surface types.Surface) hal.SurfaceTexture {
	r.mu.RLock()
	texture := r.currentSurfaceTextures[surface]
	r.mu.RUnlock()
	return texture
}

// ClearCurrentSurfaceTexture clears the current surface texture after Present.
func (r *ResourceRegistry) ClearCurrentSurfaceTexture(surface types.Surface) {
	r.mu.Lock()
	delete(r.currentSurfaceTextures, surface)
	r.mu.Unlock()
}

// GetAnySurfaceTexture returns any current surface texture.
// This is used to get the drawable for Metal presentation.
// In practice, there's only one surface per frame.
func (r *ResourceRegistry) GetAnySurfaceTexture() hal.SurfaceTexture {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tex := range r.currentSurfaceTextures {
		return tex
	}
	return nil
}

// --- Surface ---

func (r *ResourceRegistry) RegisterSurface(surface hal.Surface) types.Surface {
	handle := types.Surface(r.newHandle())
	r.mu.Lock()
	r.surfaces[handle] = surface
	r.surfaceHandles[surface] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetSurface(handle types.Surface) (hal.Surface, error) {
	r.mu.RLock()
	surface, ok := r.surfaces[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid surface handle: %d", handle)
	}
	return surface, nil
}

func (r *ResourceRegistry) UnregisterSurface(handle types.Surface) {
	r.mu.Lock()
	if surface, ok := r.surfaces[handle]; ok {
		delete(r.surfaces, handle)
		delete(r.surfaceHandles, surface)
	}
	r.mu.Unlock()
}

// --- Texture ---

func (r *ResourceRegistry) RegisterTexture(texture hal.Texture) types.Texture {
	handle := types.Texture(r.newHandle())
	r.mu.Lock()
	r.textures[handle] = texture
	r.textureHandles[texture] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) RegisterTextureForDevice(texture hal.Texture, device types.Device) types.Texture {
	handle := types.Texture(r.newHandle())
	r.mu.Lock()
	r.textures[handle] = texture
	r.textureHandles[texture] = handle
	if device != 0 {
		r.textureDevices[handle] = device
	}
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetTexture(handle types.Texture) (hal.Texture, error) {
	r.mu.RLock()
	texture, ok := r.textures[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid texture handle: %d", handle)
	}
	return texture, nil
}

func (r *ResourceRegistry) GetDeviceForTexture(texture types.Texture) (types.Device, error) {
	r.mu.RLock()
	device, ok := r.textureDevices[texture]
	r.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("no device found for texture handle: %d", texture)
	}
	return device, nil
}

func (r *ResourceRegistry) UnregisterTexture(handle types.Texture) {
	r.mu.Lock()
	if texture, ok := r.textures[handle]; ok {
		delete(r.textures, handle)
		delete(r.textureHandles, texture)
		delete(r.textureDevices, handle)
	}
	r.mu.Unlock()
}

// --- TextureView ---

func (r *ResourceRegistry) RegisterTextureView(view hal.TextureView) types.TextureView {
	handle := types.TextureView(r.newHandle())
	r.mu.Lock()
	r.textureViews[handle] = view
	r.textureViewHandles[view] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetTextureView(handle types.TextureView) (hal.TextureView, error) {
	r.mu.RLock()
	view, ok := r.textureViews[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid texture view handle: %d", handle)
	}
	return view, nil
}

func (r *ResourceRegistry) UnregisterTextureView(handle types.TextureView) {
	r.mu.Lock()
	if view, ok := r.textureViews[handle]; ok {
		delete(r.textureViews, handle)
		delete(r.textureViewHandles, view)
	}
	r.mu.Unlock()
}

// --- ShaderModule ---

func (r *ResourceRegistry) RegisterShaderModule(module hal.ShaderModule) types.ShaderModule {
	handle := types.ShaderModule(r.newHandle())
	r.mu.Lock()
	r.shaderModules[handle] = module
	r.shaderModuleHandles[module] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetShaderModule(handle types.ShaderModule) (hal.ShaderModule, error) {
	r.mu.RLock()
	module, ok := r.shaderModules[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid shader module handle: %d", handle)
	}
	return module, nil
}

func (r *ResourceRegistry) UnregisterShaderModule(handle types.ShaderModule) {
	r.mu.Lock()
	if module, ok := r.shaderModules[handle]; ok {
		delete(r.shaderModules, handle)
		delete(r.shaderModuleHandles, module)
	}
	r.mu.Unlock()
}

// --- RenderPipeline ---

func (r *ResourceRegistry) RegisterRenderPipeline(pipeline hal.RenderPipeline) types.RenderPipeline {
	handle := types.RenderPipeline(r.newHandle())
	r.mu.Lock()
	r.renderPipelines[handle] = pipeline
	r.renderPipelineHandles[pipeline] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetRenderPipeline(handle types.RenderPipeline) (hal.RenderPipeline, error) {
	r.mu.RLock()
	pipeline, ok := r.renderPipelines[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid render pipeline handle: %d", handle)
	}
	return pipeline, nil
}

func (r *ResourceRegistry) UnregisterRenderPipeline(handle types.RenderPipeline) {
	r.mu.Lock()
	if pipeline, ok := r.renderPipelines[handle]; ok {
		delete(r.renderPipelines, handle)
		delete(r.renderPipelineHandles, pipeline)
	}
	r.mu.Unlock()
}

// --- CommandEncoder ---

func (r *ResourceRegistry) RegisterCommandEncoder(encoder hal.CommandEncoder) types.CommandEncoder {
	handle := types.CommandEncoder(r.newHandle())
	r.mu.Lock()
	r.commandEncoders[handle] = encoder
	r.commandEncoderHandles[encoder] = handle
	r.mu.Unlock()
	return handle
}

// RegisterCommandEncoderForDevice registers an encoder with its associated device.
// This allows tracking which device the resulting command buffer should be freed to.
func (r *ResourceRegistry) RegisterCommandEncoderForDevice(encoder hal.CommandEncoder, device types.Device) types.CommandEncoder {
	handle := types.CommandEncoder(r.newHandle())
	r.mu.Lock()
	r.commandEncoders[handle] = encoder
	r.commandEncoderHandles[encoder] = handle
	if device != 0 {
		r.commandEncoderDevices[handle] = device
	}
	r.mu.Unlock()
	return handle
}

// GetCommandEncoderDevice returns the device associated with an encoder.
// Returns 0 if no device was registered for this encoder.
func (r *ResourceRegistry) GetCommandEncoderDevice(handle types.CommandEncoder) types.Device {
	r.mu.RLock()
	device := r.commandEncoderDevices[handle]
	r.mu.RUnlock()
	return device
}

func (r *ResourceRegistry) GetCommandEncoder(handle types.CommandEncoder) (hal.CommandEncoder, error) {
	r.mu.RLock()
	encoder, ok := r.commandEncoders[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid command encoder handle: %d", handle)
	}
	return encoder, nil
}

func (r *ResourceRegistry) UnregisterCommandEncoder(handle types.CommandEncoder) {
	r.mu.Lock()
	if encoder, ok := r.commandEncoders[handle]; ok {
		delete(r.commandEncoders, handle)
		delete(r.commandEncoderHandles, encoder)
		delete(r.commandEncoderDevices, handle)
	}
	r.mu.Unlock()
}

// --- CommandBuffer ---

func (r *ResourceRegistry) RegisterCommandBuffer(buffer hal.CommandBuffer) types.CommandBuffer {
	handle := types.CommandBuffer(r.newHandle())
	r.mu.Lock()
	r.commandBuffers[handle] = buffer
	r.commandBufferHandles[buffer] = handle
	r.mu.Unlock()
	return handle
}

// RegisterCommandBufferForDevice registers a command buffer with its associated device.
// This allows proper freeing of the command buffer back to the device's command pool.
func (r *ResourceRegistry) RegisterCommandBufferForDevice(buffer hal.CommandBuffer, device types.Device) types.CommandBuffer {
	handle := types.CommandBuffer(r.newHandle())
	r.mu.Lock()
	r.commandBuffers[handle] = buffer
	r.commandBufferHandles[buffer] = handle
	if device != 0 {
		r.commandBufferDevices[handle] = device
	}
	r.mu.Unlock()
	return handle
}

// GetCommandBufferDevice returns the device associated with a command buffer.
// Returns 0 if no device was registered for this command buffer.
func (r *ResourceRegistry) GetCommandBufferDevice(handle types.CommandBuffer) types.Device {
	r.mu.RLock()
	device := r.commandBufferDevices[handle]
	r.mu.RUnlock()
	return device
}

func (r *ResourceRegistry) GetCommandBuffer(handle types.CommandBuffer) (hal.CommandBuffer, error) {
	r.mu.RLock()
	buffer, ok := r.commandBuffers[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid command buffer handle: %d", handle)
	}
	return buffer, nil
}

func (r *ResourceRegistry) UnregisterCommandBuffer(handle types.CommandBuffer) {
	r.mu.Lock()
	if buffer, ok := r.commandBuffers[handle]; ok {
		delete(r.commandBuffers, handle)
		delete(r.commandBufferHandles, buffer)
		delete(r.commandBufferDevices, handle)
	}
	r.mu.Unlock()
}

// --- RenderPass ---

func (r *ResourceRegistry) RegisterRenderPass(pass hal.RenderPassEncoder) types.RenderPass {
	handle := types.RenderPass(r.newHandle())
	r.mu.Lock()
	r.renderPasses[handle] = pass
	r.renderPassHandles[pass] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetRenderPass(handle types.RenderPass) (hal.RenderPassEncoder, error) {
	r.mu.RLock()
	pass, ok := r.renderPasses[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid render pass handle: %d", handle)
	}
	return pass, nil
}

func (r *ResourceRegistry) UnregisterRenderPass(handle types.RenderPass) {
	r.mu.Lock()
	if pass, ok := r.renderPasses[handle]; ok {
		delete(r.renderPasses, handle)
		delete(r.renderPassHandles, pass)
	}
	r.mu.Unlock()
}

// --- ComputePipeline ---

func (r *ResourceRegistry) RegisterComputePipeline(pipeline hal.ComputePipeline) types.ComputePipeline {
	handle := types.ComputePipeline(r.newHandle())
	r.mu.Lock()
	r.computePipelines[handle] = pipeline
	r.computePipelineHandles[pipeline] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetComputePipeline(handle types.ComputePipeline) (hal.ComputePipeline, error) {
	r.mu.RLock()
	pipeline, ok := r.computePipelines[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid compute pipeline handle: %d", handle)
	}
	return pipeline, nil
}

func (r *ResourceRegistry) UnregisterComputePipeline(handle types.ComputePipeline) {
	r.mu.Lock()
	if pipeline, ok := r.computePipelines[handle]; ok {
		delete(r.computePipelines, handle)
		delete(r.computePipelineHandles, pipeline)
	}
	r.mu.Unlock()
}

// --- ComputePass ---

func (r *ResourceRegistry) RegisterComputePass(pass hal.ComputePassEncoder) types.ComputePass {
	handle := types.ComputePass(r.newHandle())
	r.mu.Lock()
	r.computePasses[handle] = pass
	r.computePassHandles[pass] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetComputePass(handle types.ComputePass) (hal.ComputePassEncoder, error) {
	r.mu.RLock()
	pass, ok := r.computePasses[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid compute pass handle: %d", handle)
	}
	return pass, nil
}

func (r *ResourceRegistry) UnregisterComputePass(handle types.ComputePass) {
	r.mu.Lock()
	if pass, ok := r.computePasses[handle]; ok {
		delete(r.computePasses, handle)
		delete(r.computePassHandles, pass)
	}
	r.mu.Unlock()
}

// --- Buffer ---

func (r *ResourceRegistry) RegisterBuffer(buffer hal.Buffer) types.Buffer {
	handle := types.Buffer(r.newHandle())
	r.mu.Lock()
	r.buffers[handle] = buffer
	r.bufferHandles[buffer] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetBuffer(handle types.Buffer) (hal.Buffer, error) {
	r.mu.RLock()
	buffer, ok := r.buffers[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid buffer handle: %d", handle)
	}
	return buffer, nil
}

func (r *ResourceRegistry) UnregisterBuffer(handle types.Buffer) {
	r.mu.Lock()
	if buffer, ok := r.buffers[handle]; ok {
		delete(r.buffers, handle)
		delete(r.bufferHandles, buffer)
	}
	r.mu.Unlock()
}

// --- Sampler ---

func (r *ResourceRegistry) RegisterSampler(sampler hal.Sampler) types.Sampler {
	handle := types.Sampler(r.newHandle())
	r.mu.Lock()
	r.samplers[handle] = sampler
	r.samplerHandles[sampler] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetSampler(handle types.Sampler) (hal.Sampler, error) {
	r.mu.RLock()
	sampler, ok := r.samplers[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid sampler handle: %d", handle)
	}
	return sampler, nil
}

func (r *ResourceRegistry) UnregisterSampler(handle types.Sampler) {
	r.mu.Lock()
	if sampler, ok := r.samplers[handle]; ok {
		delete(r.samplers, handle)
		delete(r.samplerHandles, sampler)
	}
	r.mu.Unlock()
}

// --- BindGroupLayout ---

func (r *ResourceRegistry) RegisterBindGroupLayout(layout hal.BindGroupLayout) types.BindGroupLayout {
	handle := types.BindGroupLayout(r.newHandle())
	r.mu.Lock()
	r.bindGroupLayouts[handle] = layout
	r.bindGroupLayoutHandles[layout] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetBindGroupLayout(handle types.BindGroupLayout) (hal.BindGroupLayout, error) {
	r.mu.RLock()
	layout, ok := r.bindGroupLayouts[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid bind group layout handle: %d", handle)
	}
	return layout, nil
}

func (r *ResourceRegistry) UnregisterBindGroupLayout(handle types.BindGroupLayout) {
	r.mu.Lock()
	if layout, ok := r.bindGroupLayouts[handle]; ok {
		delete(r.bindGroupLayouts, handle)
		delete(r.bindGroupLayoutHandles, layout)
	}
	r.mu.Unlock()
}

// --- BindGroup ---

func (r *ResourceRegistry) RegisterBindGroup(group hal.BindGroup) types.BindGroup {
	handle := types.BindGroup(r.newHandle())
	r.mu.Lock()
	r.bindGroups[handle] = group
	r.bindGroupHandles[group] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetBindGroup(handle types.BindGroup) (hal.BindGroup, error) {
	r.mu.RLock()
	group, ok := r.bindGroups[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid bind group handle: %d", handle)
	}
	return group, nil
}

func (r *ResourceRegistry) UnregisterBindGroup(handle types.BindGroup) {
	r.mu.Lock()
	if group, ok := r.bindGroups[handle]; ok {
		delete(r.bindGroups, handle)
		delete(r.bindGroupHandles, group)
	}
	r.mu.Unlock()
}

// --- PipelineLayout ---

func (r *ResourceRegistry) RegisterPipelineLayout(layout hal.PipelineLayout) types.PipelineLayout {
	handle := types.PipelineLayout(r.newHandle())
	r.mu.Lock()
	r.pipelineLayouts[handle] = layout
	r.pipelineLayoutHandles[layout] = handle
	r.mu.Unlock()
	return handle
}

func (r *ResourceRegistry) GetPipelineLayout(handle types.PipelineLayout) (hal.PipelineLayout, error) {
	r.mu.RLock()
	layout, ok := r.pipelineLayouts[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid pipeline layout handle: %d", handle)
	}
	return layout, nil
}

func (r *ResourceRegistry) UnregisterPipelineLayout(handle types.PipelineLayout) {
	r.mu.Lock()
	if layout, ok := r.pipelineLayouts[handle]; ok {
		delete(r.pipelineLayouts, handle)
		delete(r.pipelineLayoutHandles, layout)
	}
	r.mu.Unlock()
}

// GetAnyQueue returns any registered queue, or nil if none exist.
// This is used for operations that need a queue but don't have a specific handle,
// such as buffer readback where only the buffer handle is available.
func (r *ResourceRegistry) GetAnyQueue() hal.Queue {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, q := range r.queues {
		return q
	}
	return nil
}

// WaitAllDevicesIdle waits for all registered devices to complete their GPU operations.
// This should be called before destroying resources to prevent hangs.
func (r *ResourceRegistry) WaitAllDevicesIdle() {
	r.mu.RLock()
	devices := make([]hal.Device, 0, len(r.devices))
	for _, device := range r.devices {
		devices = append(devices, device)
	}
	r.mu.RUnlock()

	// Wait for each device to become idle
	for _, device := range devices {
		// Type assert to concrete vulkan.Device to access WaitIdle
		if waiter, ok := device.(interface{ WaitIdle() error }); ok {
			_ = waiter.WaitIdle()
		}
	}
}

// Clear releases all registered resources and clears all maps.
// WARNING: Does NOT destroy HAL objects - caller must destroy them first!
func (r *ResourceRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear forward maps
	r.instances = make(map[types.Instance]hal.Instance)
	r.adapters = make(map[types.Adapter]hal.Adapter)
	r.devices = make(map[types.Device]hal.Device)
	r.queues = make(map[types.Queue]hal.Queue)
	r.surfaces = make(map[types.Surface]hal.Surface)
	r.textures = make(map[types.Texture]hal.Texture)
	r.textureDevices = make(map[types.Texture]types.Device)
	r.textureViews = make(map[types.TextureView]hal.TextureView)
	r.shaderModules = make(map[types.ShaderModule]hal.ShaderModule)
	r.renderPipelines = make(map[types.RenderPipeline]hal.RenderPipeline)
	r.commandEncoders = make(map[types.CommandEncoder]hal.CommandEncoder)
	r.commandEncoderDevices = make(map[types.CommandEncoder]types.Device)
	r.commandBuffers = make(map[types.CommandBuffer]hal.CommandBuffer)
	r.commandBufferDevices = make(map[types.CommandBuffer]types.Device)
	r.renderPasses = make(map[types.RenderPass]hal.RenderPassEncoder)
	r.computePipelines = make(map[types.ComputePipeline]hal.ComputePipeline)
	r.computePasses = make(map[types.ComputePass]hal.ComputePassEncoder)
	r.buffers = make(map[types.Buffer]hal.Buffer)
	r.samplers = make(map[types.Sampler]hal.Sampler)
	r.bindGroupLayouts = make(map[types.BindGroupLayout]hal.BindGroupLayout)
	r.bindGroups = make(map[types.BindGroup]hal.BindGroup)
	r.pipelineLayouts = make(map[types.PipelineLayout]hal.PipelineLayout)

	// Clear device→queue mapping
	r.deviceQueues = make(map[types.Device]types.Queue)
	r.surfaceDevices = make(map[types.Surface]types.Device)
	r.currentSurfaceTextures = make(map[types.Surface]hal.SurfaceTexture)

	// Clear reverse maps
	r.instanceHandles = make(map[hal.Instance]types.Instance)
	r.adapterHandles = make(map[hal.Adapter]types.Adapter)
	r.deviceHandles = make(map[hal.Device]types.Device)
	r.queueHandles = make(map[hal.Queue]types.Queue)
	r.surfaceHandles = make(map[hal.Surface]types.Surface)
	r.textureHandles = make(map[hal.Texture]types.Texture)
	r.textureViewHandles = make(map[hal.TextureView]types.TextureView)
	r.shaderModuleHandles = make(map[hal.ShaderModule]types.ShaderModule)
	r.renderPipelineHandles = make(map[hal.RenderPipeline]types.RenderPipeline)
	r.commandEncoderHandles = make(map[hal.CommandEncoder]types.CommandEncoder)
	r.commandBufferHandles = make(map[hal.CommandBuffer]types.CommandBuffer)
	r.renderPassHandles = make(map[hal.RenderPassEncoder]types.RenderPass)
	r.computePipelineHandles = make(map[hal.ComputePipeline]types.ComputePipeline)
	r.computePassHandles = make(map[hal.ComputePassEncoder]types.ComputePass)
	r.bufferHandles = make(map[hal.Buffer]types.Buffer)
	r.samplerHandles = make(map[hal.Sampler]types.Sampler)
	r.bindGroupLayoutHandles = make(map[hal.BindGroupLayout]types.BindGroupLayout)
	r.bindGroupHandles = make(map[hal.BindGroup]types.BindGroup)
	r.pipelineLayoutHandles = make(map[hal.PipelineLayout]types.PipelineLayout)
	r.fenceHandles = make(map[hal.Fence]types.Fence)
	r.fences = make(map[types.Fence]hal.Fence)
	r.fenceDevices = make(map[types.Fence]types.Device)
}

// --- Fence ---

// RegisterFence registers a fence with its associated device.
func (r *ResourceRegistry) RegisterFence(fence hal.Fence, device types.Device) types.Fence {
	handle := types.Fence(r.newHandle())
	r.mu.Lock()
	r.fences[handle] = fence
	r.fenceDevices[handle] = device
	r.fenceHandles[fence] = handle
	r.mu.Unlock()
	return handle
}

// GetFence returns the HAL fence for a handle.
func (r *ResourceRegistry) GetFence(handle types.Fence) (hal.Fence, error) {
	r.mu.RLock()
	fence, ok := r.fences[handle]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid fence handle: %d", handle)
	}
	return fence, nil
}

// GetFenceDevice returns the device associated with a fence.
func (r *ResourceRegistry) GetFenceDevice(handle types.Fence) (types.Device, error) {
	r.mu.RLock()
	device, ok := r.fenceDevices[handle]
	r.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("invalid fence handle: %d", handle)
	}
	return device, nil
}

// UnregisterFence removes a fence from the registry.
func (r *ResourceRegistry) UnregisterFence(handle types.Fence) {
	r.mu.Lock()
	if fence, ok := r.fences[handle]; ok {
		delete(r.fences, handle)
		delete(r.fenceDevices, handle)
		delete(r.fenceHandles, fence)
	}
	r.mu.Unlock()
}
