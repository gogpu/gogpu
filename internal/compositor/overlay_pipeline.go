package compositor

import (
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// OverlayUniformSize is the uniform buffer size (vec2<f32> + padding = 16 bytes).
const OverlayUniformSize = 16

// OverlayInstanceStride is the per-instance byte stride.
// Layout: rectXY(f32x2=8) + rectWH(f32x2=8) + color(f32x4=16) = 32 bytes.
const OverlayInstanceStride = 32

// OverlayPipeline holds the GPU resources shared by all debug overlay
// renderers (damage, FPS). Uses instanced draw: one Draw(6, N) call
// renders N flat-color quads from per-instance vertex data.
type OverlayPipeline struct {
	shader         *wgpu.ShaderModule
	uniformLayout  *wgpu.BindGroupLayout
	pipelineLayout *wgpu.PipelineLayout
	pipeline       *wgpu.RenderPipeline
	uniformBuffer  *wgpu.Buffer
	uniformBindGrp *wgpu.BindGroup
	uniformData    []byte

	instanceBuf     *wgpu.Buffer
	instanceBufSize uint64

	Inited bool
}

// InitOverlayPipeline creates the GPU pipeline resources for a debug overlay.
func InitOverlayPipeline(
	device *wgpu.Device,
	surfaceFormat gputypes.TextureFormat,
	label string,
	shaderSource string,
) (*OverlayPipeline, error) {
	if device == nil {
		return nil, fmt.Errorf("compositor: %s overlay: no GPU device", label)
	}

	p := &OverlayPipeline{}
	var err error

	p.shader, err = device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: label + " Overlay Shader",
		WGSL:  shaderSource,
	})
	if err != nil {
		return nil, fmt.Errorf("compositor: %s overlay shader: %w", label, err)
	}

	if err := p.initLayouts(device, label); err != nil {
		return nil, err
	}

	p.pipeline, err = p.createRenderPipeline(device, surfaceFormat, label)
	if err != nil {
		return nil, err
	}

	if err := p.initUniforms(device, label); err != nil {
		return nil, err
	}

	p.uniformData = make([]byte, OverlayUniformSize)
	p.Inited = true
	return p, nil
}

// initLayouts creates the bind group layout and pipeline layout.
func (p *OverlayPipeline) initLayouts(device *wgpu.Device, label string) error {
	var err error
	p.uniformLayout, err = device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: label + " Overlay Uniform Layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex,
				Buffer: &gputypes.BufferBindingLayout{
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: OverlayUniformSize,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("compositor: %s overlay uniform layout: %w", label, err)
	}

	p.pipelineLayout, err = device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            label + " Overlay Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{p.uniformLayout},
	})
	if err != nil {
		return fmt.Errorf("compositor: %s overlay pipeline layout: %w", label, err)
	}
	return nil
}

// createRenderPipeline creates the render pipeline with instanced vertex layout.
func (p *OverlayPipeline) createRenderPipeline(
	device *wgpu.Device,
	surfaceFormat gputypes.TextureFormat,
	label string,
) (*wgpu.RenderPipeline, error) {
	return device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  label + " Overlay Pipeline",
		Layout: p.pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     p.shader,
			EntryPoint: ShaderEntryVS,
			Buffers: []gputypes.VertexBufferLayout{
				{
					ArrayStride: OverlayInstanceStride,
					StepMode:    gputypes.VertexStepModeInstance,
					Attributes: []gputypes.VertexAttribute{
						{ShaderLocation: 0, Offset: 0, Format: gputypes.VertexFormatFloat32x2},
						{ShaderLocation: 1, Offset: 8, Format: gputypes.VertexFormatFloat32x2},
						{ShaderLocation: 2, Offset: 16, Format: gputypes.VertexFormatFloat32x4},
					},
				},
			},
		},
		Primitive: gputypes.PrimitiveState{
			Topology: gputypes.PrimitiveTopologyTriangleList,
			CullMode: gputypes.CullModeNone,
		},
		Fragment: &wgpu.FragmentState{
			Module:     p.shader,
			EntryPoint: ShaderEntryFS,
			Targets: []gputypes.ColorTargetState{
				{
					Format:    surfaceFormat,
					WriteMask: gputypes.ColorWriteMaskAll,
					Blend: &gputypes.BlendState{
						Color: gputypes.BlendComponent{
							Operation: gputypes.BlendOperationAdd,
							SrcFactor: gputypes.BlendFactorOne,
							DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
						},
						Alpha: gputypes.BlendComponent{
							Operation: gputypes.BlendOperationAdd,
							SrcFactor: gputypes.BlendFactorOne,
							DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
						},
					},
				},
			},
		},
	})
}

// initUniforms creates the uniform buffer and bind group.
func (p *OverlayPipeline) initUniforms(device *wgpu.Device, label string) error {
	var err error
	p.uniformBuffer, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: label + " Overlay Uniforms",
		Size:  OverlayUniformSize,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("compositor: %s overlay uniform buffer: %w", label, err)
	}

	p.uniformBindGrp, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  label + " Overlay Uniform Bind Group",
		Layout: p.uniformLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  p.uniformBuffer,
				Size:    OverlayUniformSize,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("compositor: %s overlay uniform bind group: %w", label, err)
	}
	return nil
}

// AppendInstance packs one instance (32 bytes) into the staging buffer.
func AppendInstance(buf *[]byte, rect image.Rectangle, r, g, b, a float32) {
	var inst [OverlayInstanceStride]byte
	binary.LittleEndian.PutUint32(inst[0:4], math.Float32bits(float32(rect.Min.X)))
	binary.LittleEndian.PutUint32(inst[4:8], math.Float32bits(float32(rect.Min.Y)))
	binary.LittleEndian.PutUint32(inst[8:12], math.Float32bits(float32(rect.Dx())))
	binary.LittleEndian.PutUint32(inst[12:16], math.Float32bits(float32(rect.Dy())))
	binary.LittleEndian.PutUint32(inst[16:20], math.Float32bits(r))
	binary.LittleEndian.PutUint32(inst[20:24], math.Float32bits(g))
	binary.LittleEndian.PutUint32(inst[24:28], math.Float32bits(b))
	binary.LittleEndian.PutUint32(inst[28:32], math.Float32bits(a))
	*buf = append(*buf, inst[:]...)
}

// RenderInstances uploads instance data and issues Draw(6, N) in one render pass.
func (p *OverlayPipeline) RenderInstances(
	device *wgpu.Device,
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	surfW, surfH uint32,
	instanceData []byte,
) {
	instanceCount := uint32(len(instanceData) / OverlayInstanceStride) //nolint:gosec // always positive
	if instanceCount == 0 {
		return
	}

	binary.LittleEndian.PutUint32(p.uniformData[0:4], math.Float32bits(float32(surfW)))
	binary.LittleEndian.PutUint32(p.uniformData[4:8], math.Float32bits(float32(surfH)))
	if err := device.Queue().WriteBuffer(p.uniformBuffer, 0, p.uniformData); err != nil {
		slog.Error("compositor: overlay WriteBuffer (uniform) failed", "err", err)
		return
	}

	needed := uint64(len(instanceData))
	if p.instanceBuf == nil || p.instanceBufSize < needed {
		if p.instanceBuf != nil {
			p.instanceBuf.Release()
		}
		var err error
		p.instanceBuf, err = device.CreateBuffer(&wgpu.BufferDescriptor{
			Label: "Overlay Instance Buffer",
			Size:  needed,
			Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		})
		if err != nil {
			slog.Error("compositor: overlay CreateBuffer (instances) failed", "err", err)
			return
		}
		p.instanceBufSize = needed
	}

	if err := device.Queue().WriteBuffer(p.instanceBuf, 0, instanceData); err != nil {
		slog.Error("compositor: overlay WriteBuffer (instances) failed", "err", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    view,
				LoadOp:  gputypes.LoadOpLoad,
				StoreOp: gputypes.StoreOpStore,
			},
		},
	})
	if err != nil {
		slog.Error("compositor: overlay BeginRenderPass failed", "err", err)
		return
	}

	renderPass.SetPipeline(p.pipeline)
	renderPass.SetBindGroup(0, p.uniformBindGrp, nil)
	renderPass.SetVertexBuffer(0, p.instanceBuf, 0)
	renderPass.Draw(6, instanceCount, 0, 0)

	if err := renderPass.End(); err != nil {
		slog.Error("compositor: overlay End render pass failed", "err", err)
	}
}
