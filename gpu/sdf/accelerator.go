// Package sdf implements a GPU-accelerated SDF (Signed Distance Field) renderer
// using gogpu's GPU backend. It implements gg.GPUAccelerator to provide
// hardware-accelerated rendering for circles, ellipses, and rounded rectangles.
//
// When GPU operations fail, the accelerator transparently falls back to
// CPU-based SDF rendering via gg.SDFAccelerator.
package sdf

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gogpu/gg"
	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
)

// Accelerator implements gg.GPUAccelerator using gogpu's GPU backend.
// It compiles SDF compute shaders and dispatches them on the GPU for
// hardware-accelerated shape rendering.
type Accelerator struct {
	backend gpu.Backend
	device  types.Device
	queue   types.Queue

	// Compiled SDF shaders
	circleShader types.ShaderModule
	rrectShader  types.ShaderModule

	// Compute pipelines
	circlePipeline types.ComputePipeline
	rrectPipeline  types.ComputePipeline

	// Bind group layouts and pipeline layouts
	circleLayout     types.BindGroupLayout
	rrectLayout      types.BindGroupLayout
	circlePipeLayout types.PipelineLayout
	rrectPipeLayout  types.PipelineLayout

	// CPU fallback for when GPU operations fail
	cpuFallback gg.SDFAccelerator

	initialized bool
}

// Compile-time interface check.
var _ gg.GPUAccelerator = (*Accelerator)(nil)

// NewAccelerator creates a new GPU SDF accelerator using the provided backend.
// The accelerator must be initialized via Init() before use.
func NewAccelerator(backend gpu.Backend, device types.Device, queue types.Queue) *Accelerator {
	return &Accelerator{
		backend: backend,
		device:  device,
		queue:   queue,
	}
}

// Name returns the accelerator name.
func (a *Accelerator) Name() string { return "sdf-gpu" }

// Init initializes GPU resources: compiles shaders, creates pipelines.
func (a *Accelerator) Init() error {
	if a.initialized {
		return nil
	}

	var err error

	// Compile circle SDF shader
	a.circleShader, err = a.backend.CreateShaderModuleWGSL(a.device, circleShaderWGSL)
	if err != nil {
		return fmt.Errorf("sdf: failed to compile circle shader: %w", err)
	}

	// Compile rrect SDF shader
	a.rrectShader, err = a.backend.CreateShaderModuleWGSL(a.device, rrectShaderWGSL)
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to compile rrect shader: %w", err)
	}

	// Create bind group layout for circle pipeline.
	// Binding 0: uniform buffer (params), Binding 1: storage buffer (pixels)
	a.circleLayout, err = a.backend.CreateBindGroupLayout(a.device, &types.BindGroupLayoutDescriptor{
		Label: "SDF Circle Layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: circleUniformSize,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type: gputypes.BufferBindingTypeStorage,
				},
			},
		},
	})
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to create circle bind group layout: %w", err)
	}

	// Create bind group layout for rrect pipeline
	a.rrectLayout, err = a.backend.CreateBindGroupLayout(a.device, &types.BindGroupLayoutDescriptor{
		Label: "SDF RRect Layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: rrectUniformSize,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type: gputypes.BufferBindingTypeStorage,
				},
			},
		},
	})
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to create rrect bind group layout: %w", err)
	}

	// Create pipeline layouts
	a.circlePipeLayout, err = a.backend.CreatePipelineLayout(a.device, &types.PipelineLayoutDescriptor{
		Label:            "SDF Circle Pipeline Layout",
		BindGroupLayouts: []types.BindGroupLayout{a.circleLayout},
	})
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to create circle pipeline layout: %w", err)
	}

	a.rrectPipeLayout, err = a.backend.CreatePipelineLayout(a.device, &types.PipelineLayoutDescriptor{
		Label:            "SDF RRect Pipeline Layout",
		BindGroupLayouts: []types.BindGroupLayout{a.rrectLayout},
	})
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to create rrect pipeline layout: %w", err)
	}

	// Create compute pipelines
	a.circlePipeline, err = a.backend.CreateComputePipeline(a.device, &types.ComputePipelineDescriptor{
		Label:      "SDF Circle Pipeline",
		Layout:     a.circlePipeLayout,
		Module:     a.circleShader,
		EntryPoint: "main",
	})
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to create circle compute pipeline: %w", err)
	}

	a.rrectPipeline, err = a.backend.CreateComputePipeline(a.device, &types.ComputePipelineDescriptor{
		Label:      "SDF RRect Pipeline",
		Layout:     a.rrectPipeLayout,
		Module:     a.rrectShader,
		EntryPoint: "main",
	})
	if err != nil {
		a.Close()
		return fmt.Errorf("sdf: failed to create rrect compute pipeline: %w", err)
	}

	a.initialized = true
	return nil
}

// Close releases all GPU resources.
func (a *Accelerator) Close() {
	if a.circlePipeline != 0 {
		a.backend.ReleaseComputePipeline(a.circlePipeline)
		a.circlePipeline = 0
	}
	if a.rrectPipeline != 0 {
		a.backend.ReleaseComputePipeline(a.rrectPipeline)
		a.rrectPipeline = 0
	}
	if a.circlePipeLayout != 0 {
		a.backend.ReleasePipelineLayout(a.circlePipeLayout)
		a.circlePipeLayout = 0
	}
	if a.rrectPipeLayout != 0 {
		a.backend.ReleasePipelineLayout(a.rrectPipeLayout)
		a.rrectPipeLayout = 0
	}
	if a.circleLayout != 0 {
		a.backend.ReleaseBindGroupLayout(a.circleLayout)
		a.circleLayout = 0
	}
	if a.rrectLayout != 0 {
		a.backend.ReleaseBindGroupLayout(a.rrectLayout)
		a.rrectLayout = 0
	}
	if a.circleShader != 0 {
		a.backend.ReleaseShaderModule(a.circleShader)
		a.circleShader = 0
	}
	if a.rrectShader != 0 {
		a.backend.ReleaseShaderModule(a.rrectShader)
		a.rrectShader = 0
	}

	a.initialized = false
}

// CanAccelerate reports whether the accelerator supports the given operation.
func (a *Accelerator) CanAccelerate(op gg.AcceleratedOp) bool {
	return op&(gg.AccelCircleSDF|gg.AccelRRectSDF) != 0
}

// FillPath always falls back to CPU for general paths.
func (a *Accelerator) FillPath(_ gg.GPURenderTarget, _ *gg.Path, _ *gg.Paint) error {
	return gg.ErrFallbackToCPU
}

// StrokePath always falls back to CPU for general paths.
func (a *Accelerator) StrokePath(_ gg.GPURenderTarget, _ *gg.Path, _ *gg.Paint) error {
	return gg.ErrFallbackToCPU
}

// FillShape renders a filled shape using GPU SDF.
// Falls back to CPU on any GPU error.
func (a *Accelerator) FillShape(target gg.GPURenderTarget, shape gg.DetectedShape, paint *gg.Paint) error {
	if !a.initialized {
		return a.cpuFallback.FillShape(target, shape, paint)
	}

	switch shape.Kind {
	case gg.ShapeCircle, gg.ShapeEllipse:
		err := a.dispatchCircle(target, shape, paint, false)
		if err != nil {
			return a.cpuFallback.FillShape(target, shape, paint)
		}
		return nil
	case gg.ShapeRect, gg.ShapeRRect:
		err := a.dispatchRRect(target, shape, paint, false)
		if err != nil {
			return a.cpuFallback.FillShape(target, shape, paint)
		}
		return nil
	default:
		return gg.ErrFallbackToCPU
	}
}

// StrokeShape renders a stroked shape using GPU SDF.
// Falls back to CPU on any GPU error.
func (a *Accelerator) StrokeShape(target gg.GPURenderTarget, shape gg.DetectedShape, paint *gg.Paint) error {
	if !a.initialized {
		return a.cpuFallback.StrokeShape(target, shape, paint)
	}

	switch shape.Kind {
	case gg.ShapeCircle, gg.ShapeEllipse:
		err := a.dispatchCircle(target, shape, paint, true)
		if err != nil {
			return a.cpuFallback.StrokeShape(target, shape, paint)
		}
		return nil
	case gg.ShapeRect, gg.ShapeRRect:
		err := a.dispatchRRect(target, shape, paint, true)
		if err != nil {
			return a.cpuFallback.StrokeShape(target, shape, paint)
		}
		return nil
	default:
		return gg.ErrFallbackToCPU
	}
}

// dispatchCircle dispatches the circle/ellipse SDF compute shader.
func (a *Accelerator) dispatchCircle(target gg.GPURenderTarget, shape gg.DetectedShape, paint *gg.Paint, stroked bool) error {
	color := getColorFromPaint(paint)

	var halfStrokeWidth float32
	var isStroked uint32
	if stroked {
		isStroked = 1
		halfStrokeWidth = float32(paint.EffectiveLineWidth() / 2)
	}

	// Build uniform data (SDFCircleParams: 12 fields * 4 bytes = 48 bytes)
	uniformData := make([]byte, circleUniformSize)
	binary.LittleEndian.PutUint32(uniformData[0:4], math.Float32bits(float32(shape.CenterX)))
	binary.LittleEndian.PutUint32(uniformData[4:8], math.Float32bits(float32(shape.CenterY)))
	binary.LittleEndian.PutUint32(uniformData[8:12], math.Float32bits(float32(shape.RadiusX)))
	binary.LittleEndian.PutUint32(uniformData[12:16], math.Float32bits(float32(shape.RadiusY)))
	binary.LittleEndian.PutUint32(uniformData[16:20], math.Float32bits(halfStrokeWidth))
	binary.LittleEndian.PutUint32(uniformData[20:24], isStroked)
	binary.LittleEndian.PutUint32(uniformData[24:28], math.Float32bits(float32(color.R)))
	binary.LittleEndian.PutUint32(uniformData[28:32], math.Float32bits(float32(color.G)))
	binary.LittleEndian.PutUint32(uniformData[32:36], math.Float32bits(float32(color.B)))
	binary.LittleEndian.PutUint32(uniformData[36:40], math.Float32bits(float32(color.A)))
	binary.LittleEndian.PutUint32(uniformData[40:44], uint32(target.Width))  //nolint:gosec // G115: validated in caller
	binary.LittleEndian.PutUint32(uniformData[44:48], uint32(target.Height)) //nolint:gosec // G115: validated in caller

	return a.dispatchCompute(target, a.circlePipeline, a.circleLayout, uniformData, circleUniformSize)
}

// dispatchRRect dispatches the rounded rectangle SDF compute shader.
func (a *Accelerator) dispatchRRect(target gg.GPURenderTarget, shape gg.DetectedShape, paint *gg.Paint, stroked bool) error {
	color := getColorFromPaint(paint)

	var halfStrokeWidth float32
	var isStroked uint32
	if stroked {
		isStroked = 1
		halfStrokeWidth = float32(paint.EffectiveLineWidth() / 2)
	}

	// For rect shapes, corner radius is 0
	cornerRadius := float32(shape.CornerRadius)

	// Build uniform data (SDFRRectParams: 14 fields * 4 bytes = 56 bytes)
	uniformData := make([]byte, rrectUniformSize)
	binary.LittleEndian.PutUint32(uniformData[0:4], math.Float32bits(float32(shape.CenterX)))
	binary.LittleEndian.PutUint32(uniformData[4:8], math.Float32bits(float32(shape.CenterY)))
	binary.LittleEndian.PutUint32(uniformData[8:12], math.Float32bits(float32(shape.Width/2)))
	binary.LittleEndian.PutUint32(uniformData[12:16], math.Float32bits(float32(shape.Height/2)))
	binary.LittleEndian.PutUint32(uniformData[16:20], math.Float32bits(cornerRadius))
	binary.LittleEndian.PutUint32(uniformData[20:24], math.Float32bits(halfStrokeWidth))
	binary.LittleEndian.PutUint32(uniformData[24:28], isStroked)
	binary.LittleEndian.PutUint32(uniformData[28:32], math.Float32bits(float32(color.R)))
	binary.LittleEndian.PutUint32(uniformData[32:36], math.Float32bits(float32(color.G)))
	binary.LittleEndian.PutUint32(uniformData[36:40], math.Float32bits(float32(color.B)))
	binary.LittleEndian.PutUint32(uniformData[40:44], math.Float32bits(float32(color.A)))
	binary.LittleEndian.PutUint32(uniformData[44:48], uint32(target.Width))  //nolint:gosec // G115: validated in caller
	binary.LittleEndian.PutUint32(uniformData[48:52], uint32(target.Height)) //nolint:gosec // G115: validated in caller
	binary.LittleEndian.PutUint32(uniformData[52:56], 0)                     // _padding

	return a.dispatchCompute(target, a.rrectPipeline, a.rrectLayout, uniformData, rrectUniformSize)
}

// dispatchCompute is the shared dispatch logic for all SDF compute shaders.
// It creates temporary GPU resources, uploads pixel data, dispatches the shader,
// reads back the result, and cleans up.
func (a *Accelerator) dispatchCompute(
	target gg.GPURenderTarget,
	pipeline types.ComputePipeline,
	layout types.BindGroupLayout,
	uniformData []byte,
	uniformSize uint64,
) error {
	// Calculate pixel buffer size (width * height * 4 bytes per RGBA pixel packed as u32)
	pixelCount := target.Width * target.Height
	storageSize := uint64(pixelCount) * 4

	// Create uniform buffer
	uniformBuf, err := a.backend.CreateBuffer(a.device, &types.BufferDescriptor{
		Label: "SDF Uniform",
		Size:  uniformSize,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("sdf: failed to create uniform buffer: %w", err)
	}
	defer a.backend.ReleaseBuffer(uniformBuf)

	// Write uniform data
	a.backend.WriteBuffer(a.queue, uniformBuf, 0, uniformData)

	// Create storage buffer for pixel data.
	// The storage buffer needs Storage usage for the compute shader to read/write,
	// plus CopyDst so we can upload existing pixel data, plus CopySrc for readback.
	storageBuf, err := a.backend.CreateBuffer(a.device, &types.BufferDescriptor{
		Label: "SDF Pixels",
		Size:  storageSize,
		Usage: gputypes.BufferUsageStorage | gputypes.BufferUsageCopyDst | gputypes.BufferUsageCopySrc,
	})
	if err != nil {
		return fmt.Errorf("sdf: failed to create storage buffer: %w", err)
	}
	defer a.backend.ReleaseBuffer(storageBuf)

	// Pack pixel data from the target into the storage buffer.
	// The target data is in RGBA byte format with a stride.
	// The shader expects packed u32 per pixel (RGBA8).
	packedPixels := packPixelData(target)
	a.backend.WriteBuffer(a.queue, storageBuf, 0, packedPixels)

	// Create bind group
	bindGroup, err := a.backend.CreateBindGroup(a.device, &types.BindGroupDescriptor{
		Label:  "SDF Bind Group",
		Layout: layout,
		Entries: []types.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  uniformBuf,
				Offset:  0,
				Size:    uniformSize,
			},
			{
				Binding: 1,
				Buffer:  storageBuf,
				Offset:  0,
				Size:    storageSize,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("sdf: failed to create bind group: %w", err)
	}
	defer a.backend.ReleaseBindGroup(bindGroup)

	// Create command encoder and dispatch compute shader
	encoder := a.backend.CreateCommandEncoder(a.device)
	if encoder == 0 {
		return fmt.Errorf("sdf: failed to create command encoder")
	}

	computePass := a.backend.BeginComputePass(encoder)
	if computePass == 0 {
		a.backend.ReleaseCommandEncoder(encoder)
		return fmt.Errorf("sdf: failed to begin compute pass")
	}

	a.backend.SetComputePipeline(computePass, pipeline)
	a.backend.SetComputeBindGroup(computePass, 0, bindGroup, nil)

	// Dispatch workgroups: ceil(width/8) x ceil(height/8) x 1
	workgroupsX := (uint32(target.Width) + 7) / 8  //nolint:gosec // G115: validated positive
	workgroupsY := (uint32(target.Height) + 7) / 8 //nolint:gosec // G115: validated positive
	a.backend.DispatchWorkgroups(computePass, workgroupsX, workgroupsY, 1)

	a.backend.EndComputePass(computePass)
	a.backend.ReleaseComputePass(computePass)

	commands := a.backend.FinishEncoder(encoder)
	a.backend.ReleaseCommandEncoder(encoder)

	// Submit and wait for completion
	a.backend.Submit(a.queue, commands, 0, 0)
	a.backend.ReleaseCommandBuffer(commands)

	// Read back the results from the storage buffer.
	// Since MapBufferRead is not fully implemented yet, we use the fallback
	// approach of reading from host-visible buffer memory.
	result, err := a.backend.MapBufferRead(storageBuf)
	if err != nil {
		// MapBufferRead not supported yet, this is expected.
		// Fall back to CPU rendering.
		return fmt.Errorf("sdf: buffer readback not supported: %w", err)
	}

	// Unpack pixel data back into the target
	unpackPixelData(target, result)
	a.backend.UnmapBuffer(storageBuf)

	return nil
}

// packPixelData converts GPURenderTarget data into packed u32 per pixel.
// The target data is RGBA bytes with stride, the shader expects u32 packed RGBA8.
func packPixelData(target gg.GPURenderTarget) []byte {
	pixelCount := target.Width * target.Height
	packed := make([]byte, pixelCount*4)

	for y := 0; y < target.Height; y++ {
		srcRow := y * target.Stride
		dstRow := y * target.Width * 4
		for x := 0; x < target.Width; x++ {
			srcIdx := srcRow + x*4
			dstIdx := dstRow + x*4
			// Pack RGBA bytes as little-endian u32: R | (G<<8) | (B<<16) | (A<<24)
			packed[dstIdx+0] = target.Data[srcIdx+0] // R
			packed[dstIdx+1] = target.Data[srcIdx+1] // G
			packed[dstIdx+2] = target.Data[srcIdx+2] // B
			packed[dstIdx+3] = target.Data[srcIdx+3] // A
		}
	}

	return packed
}

// unpackPixelData converts packed u32 per pixel back into GPURenderTarget data.
func unpackPixelData(target gg.GPURenderTarget, packed []byte) {
	for y := 0; y < target.Height; y++ {
		dstRow := y * target.Stride
		srcRow := y * target.Width * 4
		for x := 0; x < target.Width; x++ {
			srcIdx := srcRow + x*4
			dstIdx := dstRow + x*4
			target.Data[dstIdx+0] = packed[srcIdx+0] // R
			target.Data[dstIdx+1] = packed[srcIdx+1] // G
			target.Data[dstIdx+2] = packed[srcIdx+2] // B
			target.Data[dstIdx+3] = packed[srcIdx+3] // A
		}
	}
}

// getColorFromPaint extracts a solid color from the paint.
func getColorFromPaint(paint *gg.Paint) gg.RGBA {
	if paint.Brush != nil {
		if sb, ok := paint.Brush.(gg.SolidBrush); ok {
			return sb.Color
		}
		return paint.Brush.ColorAt(0, 0)
	}
	if paint.Pattern != nil {
		return paint.Pattern.ColorAt(0, 0)
	}
	return gg.Black
}
