package gogpu

import (
	"fmt"

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

// overlayPipeline holds the GPU resources shared by all debug overlay
// pipelines (damage, FPS). Both overlays use an identical flat-color quad
// pipeline differing only in labels and shader source.
type overlayPipeline struct {
	shader         *wgpu.ShaderModule
	uniformLayout  *wgpu.BindGroupLayout
	pipelineLayout *wgpu.PipelineLayout
	pipeline       *wgpu.RenderPipeline
	uniformBuffer  *wgpu.Buffer
	uniformBindGrp *wgpu.BindGroup
	uniformData    []byte
	inited         bool
}

// initOverlayPipeline creates the GPU pipeline resources for a debug overlay.
// Both damage and FPS overlays share the same pipeline structure: a flat-color
// quad with premultiplied alpha blending. Only labels and shader source differ.
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
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: damageOverlayUniformSize,
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
		Size:  damageOverlayUniformSize,
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
				Size:    damageOverlayUniformSize,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gogpu: %s overlay uniform bind group: %w", label, err)
	}

	p.uniformData = make([]byte, damageOverlayUniformSize)
	p.inited = true
	return p, nil
}
