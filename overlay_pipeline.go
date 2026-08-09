package gogpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Shader entry point names shared by all pipelines in the package (renderer,
// damage overlay, FPS overlay). Defined once to satisfy goconst.
const (
	shaderEntryVS = "vs_main"
	shaderEntryFS = "fs_main"
)

// Debug mode string constants shared by damage and FPS overlay parsers.
const (
	overlayModeOverlay = "overlay"
	overlayModeLog     = "log"
)

// overlayUniformSize is the uniform buffer size for the instanced overlay
// shader: screen dimensions (vec2<f32>) + padding = 16 bytes.
const overlayUniformSize = 16

// overlayInstanceStride is the per-instance byte stride.
// Layout: rectXY(f32x2=8) + rectWH(f32x2=8) + color(f32x4=16) = 32 bytes.
const overlayInstanceStride = 32

// overlayPipeline holds the GPU resources shared by all debug overlay
// pipelines (damage, FPS). Uses instanced draw: one Draw(6, N, 0, 0) call
// renders N flat-color quads from per-instance vertex data.
type overlayPipeline struct {
	shader         *wgpu.ShaderModule
	uniformLayout  *wgpu.BindGroupLayout
	pipelineLayout *wgpu.PipelineLayout
	pipeline       *wgpu.RenderPipeline
	uniformBuffer  *wgpu.Buffer
	uniformBindGrp *wgpu.BindGroup
	uniformData    []byte

	// Instance buffer — grow-on-demand (gg SDF pattern).
	instanceBuf     *wgpu.Buffer
	instanceBufSize uint64

	inited bool
}

// initOverlayPipeline creates the GPU pipeline resources for a debug overlay.
// Both damage and FPS overlays share the same instanced pipeline structure:
// flat-color quads with premultiplied alpha blending, per-instance rect and
// color data via VertexStepModeInstance.
func initOverlayPipeline(
	device *wgpu.Device,
	surfaceFormat gputypes.TextureFormat,
	label string,
	shaderSource string,
) (*overlayPipeline, error) {
	if device == nil {
		return nil, fmt.Errorf("gogpu: %s overlay: no GPU device", label)
	}

	p := &overlayPipeline{}
	var err error

	p.shader, err = device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: label + " Overlay Shader",
		WGSL:  shaderSource,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay shader: %w", label, err)
	}

	p.uniformLayout, err = device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: label + " Overlay Uniform Layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex,
				Buffer: &gputypes.BufferBindingLayout{
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: overlayUniformSize,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay uniform layout: %w", label, err)
	}

	p.pipelineLayout, err = device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            label + " Overlay Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{p.uniformLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay pipeline layout: %w", label, err)
	}

	// Alpha blending: premultiplied source over destination.
	// SrcFactor=One because color is pre-multiplied by the caller.
	p.pipeline, err = device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  label + " Overlay Pipeline",
		Layout: p.pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers: []gputypes.VertexBufferLayout{
				{
					ArrayStride: overlayInstanceStride,
					StepMode:    gputypes.VertexStepModeInstance,
					Attributes: []gputypes.VertexAttribute{
						{ShaderLocation: 0, Offset: 0, Format: gputypes.VertexFormatFloat32x2},  // rectXY
						{ShaderLocation: 1, Offset: 8, Format: gputypes.VertexFormatFloat32x2},  // rectWH
						{ShaderLocation: 2, Offset: 16, Format: gputypes.VertexFormatFloat32x4}, // color
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
			EntryPoint: shaderEntryFS,
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
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay pipeline: %w", label, err)
	}

	p.uniformBuffer, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: label + " Overlay Uniforms",
		Size:  overlayUniformSize,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay uniform buffer: %w", label, err)
	}

	p.uniformBindGrp, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  label + " Overlay Uniform Bind Group",
		Layout: p.uniformLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  p.uniformBuffer,
				Size:    overlayUniformSize,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay uniform bind group: %w", label, err)
	}

	p.uniformData = make([]byte, overlayUniformSize)
	p.inited = true
	return p, nil
}

// appendInstance packs one instance (32 bytes) into the staging buffer.
// Rect position and size are in physical surface pixels. Color components
// must already be pre-multiplied by the desired alpha.
func appendInstance(buf *[]byte, rect image.Rectangle, r, g, b, a float32) {
	var inst [overlayInstanceStride]byte
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

// renderInstances uploads instance data, writes the screen-size uniform,
// and issues a single Draw(6, instanceCount) inside one render pass.
// The instance buffer is persistent with grow-on-demand (gg SDF pattern)
// to avoid per-frame buffer creation.
func (p *overlayPipeline) renderInstances(
	device *wgpu.Device,
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	surfW, surfH uint32,
	instanceData []byte,
) {
	instanceCount := uint32(len(instanceData) / overlayInstanceStride) //nolint:gosec // always positive
	if instanceCount == 0 {
		return
	}

	// Write screen dimensions to uniform buffer.
	binary.LittleEndian.PutUint32(p.uniformData[0:4], math.Float32bits(float32(surfW)))
	binary.LittleEndian.PutUint32(p.uniformData[4:8], math.Float32bits(float32(surfH)))
	// Padding bytes 8-15 remain zero.
	if err := device.Queue().WriteBuffer(p.uniformBuffer, 0, p.uniformData); err != nil {
		slog.Error("gogpu: overlay WriteBuffer (uniform) failed", "err", err)
		return
	}

	// Grow-on-demand instance buffer (gg SDF pattern).
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
			slog.Error("gogpu: overlay CreateBuffer (instances) failed", "err", err)
			return
		}
		p.instanceBufSize = needed
	}

	if err := device.Queue().WriteBuffer(p.instanceBuf, 0, instanceData); err != nil {
		slog.Error("gogpu: overlay WriteBuffer (instances) failed", "err", err)
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
		slog.Error("gogpu: overlay BeginRenderPass failed", "err", err)
		return
	}

	renderPass.SetPipeline(p.pipeline)
	renderPass.SetBindGroup(0, p.uniformBindGrp, nil)
	renderPass.SetVertexBuffer(0, p.instanceBuf, 0)
	renderPass.Draw(6, instanceCount, 0, 0)

	if err := renderPass.End(); err != nil {
		slog.Error("gogpu: overlay End render pass failed", "err", err)
	}
}
