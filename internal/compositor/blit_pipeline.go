package compositor

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// BlitUniformSize is the uniform buffer size for the blit/composite pipeline.
// Layout: rect(4 floats) + screen(2 floats) + alpha(1 float) + premultiplied(1 float) = 32 bytes.
const BlitUniformSize = 32

// CompositeState holds per-surface persistent bind group state for the MSAA
// overlay alpha-composite path. Recreated only when compositeView changes.
type CompositeState struct {
	BindGroup      *wgpu.BindGroup
	BoundView      *wgpu.TextureView
	PendingRelease *wgpu.BindGroup
}

// Release releases pending composite bind group. Called at frame boundary.
func (cs *CompositeState) Release() {
	if cs.PendingRelease != nil {
		cs.PendingRelease.Release()
		cs.PendingRelease = nil
	}
}

// BlitPipeline owns the dedicated composition blit pipeline resources (ADR-067).
// This pipeline blits the composition texture to the swapchain image.
// It uses the SAME shader as texQuadPipeline (positionedQuadShaderSource)
// but has its OWN pipeline, layout, uniform buffer, and bind groups to
// prevent bind group lifetime conflicts with gg's render session.
type BlitPipeline struct {
	device *wgpu.Device

	pipeline       *wgpu.RenderPipeline
	shader         *wgpu.ShaderModule
	uniformLayout  *wgpu.BindGroupLayout
	textureLayout  *wgpu.BindGroupLayout
	pipelineLayout *wgpu.PipelineLayout
	sampler        *wgpu.Sampler
	uniformBuf     *wgpu.Buffer
	uniformBindGrp *wgpu.BindGroup
	uniformData    []byte

	// compositePipeline is like pipeline but with premultiplied alpha blend.
	// Used by EncodeSurfaceCompositePass for MSAA overlay alpha-blending.
	// Blit pipeline has NO blend (passthrough copy); composite pipeline blends.
	compositePipeline *wgpu.RenderPipeline

	Inited bool
}

// Init creates the blit pipeline GPU resources. shaderSource is the
// positionedQuadShaderSource from the root package (shared with texQuadPipeline).
//
//nolint:funlen // pipeline init is inherently sequential setup code
func (bp *BlitPipeline) Init(device *wgpu.Device, surfaceFormat gputypes.TextureFormat, shaderSource string) error {
	if bp.Inited {
		return nil
	}
	bp.device = device

	var err error

	bp.shader, err = device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Composition Blit Shader",
		WGSL:  shaderSource,
	})
	if err != nil {
		return fmt.Errorf("compositor: blit shader: %w", err)
	}

	// Bind group 0: uniforms (rect, screen, alpha, premultiplied).
	bp.uniformLayout, err = device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Blit Uniform Layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:           gputypes.BufferBindingTypeUniform,
					MinBindingSize: BlitUniformSize,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("compositor: blit uniform layout: %w", err)
	}

	// Bind group 1: sampler + texture.
	bp.textureLayout, err = device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Blit Texture Layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("compositor: blit texture layout: %w", err)
	}

	bp.pipelineLayout, err = device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Blit Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bp.uniformLayout, bp.textureLayout},
	})
	if err != nil {
		return fmt.Errorf("compositor: blit pipeline layout: %w", err)
	}

	// No blending needed: composition texture is the final image;
	// copy it 1:1 to the swapchain.
	bp.pipeline, err = device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "Composition Blit Pipeline",
		Layout: bp.pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     bp.shader,
			EntryPoint: ShaderEntryVS,
		},
		Primitive: gputypes.PrimitiveState{
			Topology: gputypes.PrimitiveTopologyTriangleList,
			CullMode: gputypes.CullModeNone,
		},
		Fragment: &wgpu.FragmentState{
			Module:     bp.shader,
			EntryPoint: ShaderEntryFS,
			Targets: []gputypes.ColorTargetState{
				{
					Format:    surfaceFormat,
					WriteMask: gputypes.ColorWriteMaskAll,
					// No blending: composition texture is the final composited
					// image — copy 1:1 to swapchain (passthrough). Content was
					// already alpha-blended during Stage 1 (gg blit into composView).
					// MSAA composite uses EncodeSurfaceCompositePass which must
					// use its own pipeline with premultiplied alpha blend.
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("compositor: blit pipeline: %w", err)
	}

	bp.sampler, err = device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "Blit Sampler",
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
	})
	if err != nil {
		return fmt.Errorf("compositor: blit sampler: %w", err)
	}

	bp.uniformBuf, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Blit Uniforms",
		Size:  BlitUniformSize,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("compositor: blit uniform buffer: %w", err)
	}

	bp.uniformBindGrp, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Blit Uniform Bind Group",
		Layout: bp.uniformLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  bp.uniformBuf,
				Size:    BlitUniformSize,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("compositor: blit uniform bind group: %w", err)
	}

	bp.uniformData = make([]byte, BlitUniformSize)

	// Composite pipeline — same as blit but WITH premultiplied alpha blend.
	// Used by EncodeSurfaceCompositePass for MSAA overlay alpha-compositing.
	bp.compositePipeline, err = device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "MSAA Composite Pipeline",
		Layout: bp.pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     bp.shader,
			EntryPoint: ShaderEntryVS,
		},
		Primitive: gputypes.PrimitiveState{
			Topology: gputypes.PrimitiveTopologyTriangleList,
			CullMode: gputypes.CullModeNone,
		},
		Fragment: &wgpu.FragmentState{
			Module:     bp.shader,
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
	if err != nil {
		return fmt.Errorf("compositor: composite pipeline: %w", err)
	}

	bp.Inited = true
	return nil
}

// Destroy releases all blit pipeline GPU resources.
func (bp *BlitPipeline) Destroy() {
	if bp.uniformBindGrp != nil {
		bp.uniformBindGrp.Release()
		bp.uniformBindGrp = nil
	}
	if bp.uniformBuf != nil {
		bp.uniformBuf.Release()
		bp.uniformBuf = nil
	}
	if bp.sampler != nil {
		bp.sampler.Release()
		bp.sampler = nil
	}
	if bp.pipelineLayout != nil {
		bp.pipelineLayout.Release()
		bp.pipelineLayout = nil
	}
	if bp.textureLayout != nil {
		bp.textureLayout.Release()
		bp.textureLayout = nil
	}
	if bp.uniformLayout != nil {
		bp.uniformLayout.Release()
		bp.uniformLayout = nil
	}
	if bp.shader != nil {
		bp.shader.Release()
		bp.shader = nil
	}
	if bp.pipeline != nil {
		bp.pipeline.Release()
		bp.pipeline = nil
	}
	if bp.compositePipeline != nil {
		bp.compositePipeline.Release()
		bp.compositePipeline = nil
	}
	bp.Inited = false
}

// TextureLayout returns the texture bind group layout for creating per-frame
// or per-surface texture bind groups outside the pipeline.
func (bp *BlitPipeline) TextureLayout() *wgpu.BindGroupLayout {
	return bp.textureLayout
}

// Sampler returns the blit sampler for creating texture bind groups.
func (bp *BlitPipeline) Sampler() *wgpu.Sampler {
	return bp.sampler
}

// BlitToSwapchain draws a full-screen quad sampling the composition texture
// onto the swapchain image. Uses the dedicated blit pipeline (NOT the shared
// texQuadPipeline) to avoid bind group lifetime conflicts with gg's render
// session. See ADR-067.
//
// pendingBindGroup receives the created texture bind group for deferred release
// at next frame boundary (after GPU completion). Caller is responsible for
// storing and releasing it.
func (bp *BlitPipeline) BlitToSwapchain(
	encoder *wgpu.CommandEncoder,
	swapView *wgpu.TextureView,
	composView *wgpu.TextureView,
	w, h uint32,
) (*wgpu.BindGroup, error) {
	if composView == nil || swapView == nil {
		return nil, fmt.Errorf("compositor: nil view in BlitToSwapchain")
	}
	if !bp.Inited {
		return nil, fmt.Errorf("compositor: blit pipeline not initialized")
	}

	// Create per-frame bind group for the composition texture view.
	texBindGrp, err := bp.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Blit Texture Bind Group",
		Layout: bp.textureLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Sampler: bp.sampler,
			},
			{
				Binding:     1,
				TextureView: composView,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("compositor: create blit texture bind group: %w", err)
	}

	bp.writeFullScreenUniforms(w, h)
	if err := bp.device.Queue().WriteBuffer(bp.uniformBuf, 0, bp.uniformData); err != nil {
		texBindGrp.Release()
		return nil, fmt.Errorf("compositor: blit WriteBuffer uniform: %w", err)
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       swapView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 0},
			},
		},
	})
	if err != nil {
		texBindGrp.Release()
		return nil, fmt.Errorf("compositor: blit BeginRenderPass: %w", err)
	}

	renderPass.SetPipeline(bp.pipeline)
	renderPass.SetBindGroup(0, bp.uniformBindGrp, nil)
	renderPass.SetBindGroup(1, texBindGrp, nil)
	renderPass.Draw(gputypes.DrawArgs{VertexCount: 6, InstanceCount: 1}) // 2 triangles for full-screen quad

	if err := renderPass.End(); err != nil {
		slog.Error("compositor: blit EndRenderPass failed", "err", err)
	}

	// Return bind group for deferred release at frame boundary.
	return texBindGrp, nil
}

// EncodeSurfaceCompositePass alpha-blends a transparent overlay resolve texture
// onto the existing single-sample surface using premultiplied alpha blending.
// Used when MSAA overlay rendering resolves into an intermediate texture that
// must be composited on top of previous surface content.
//
// cs holds the per-surface persistent bind group state. The bind group is
// recreated only when compositeView changes.
func (bp *BlitPipeline) EncodeSurfaceCompositePass(
	encoder *wgpu.CommandEncoder,
	view *wgpu.TextureView,
	compositeView *wgpu.TextureView,
	w, h uint32,
	cs *CompositeState,
) error {
	if compositeView == nil || view == nil {
		return fmt.Errorf("compositor: nil view in surface composite pass")
	}
	if !bp.Inited {
		return fmt.Errorf("compositor: blit pipeline not initialized for composite")
	}

	// Persistent bind group — recreated only when compositeView changes.
	if cs.BindGroup == nil || cs.BoundView != compositeView {
		if cs.BindGroup != nil {
			cs.PendingRelease = cs.BindGroup
		}
		bg, err := bp.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Surface Composite Texture Bind Group",
			Layout: bp.textureLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Sampler: bp.sampler},
				{Binding: 1, TextureView: compositeView},
			},
		})
		if err != nil {
			return fmt.Errorf("compositor: create surface composite bind group: %w", err)
		}
		cs.BindGroup = bg
		cs.BoundView = compositeView
	}
	texBindGrp := cs.BindGroup

	bp.writeFullScreenUniforms(w, h)
	if err := bp.device.Queue().WriteBuffer(bp.uniformBuf, 0, bp.uniformData); err != nil {
		return fmt.Errorf("compositor: surface composite WriteBuffer: %w", err)
	}

	rp, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "surface_composite_pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     gputypes.LoadOpLoad,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return fmt.Errorf("compositor: begin surface composite pass: %w", err)
	}
	rp.SetViewport(gputypes.Viewport{X: 0, Y: 0, Width: float32(w), Height: float32(h), MinDepth: 0, MaxDepth: 1})
	rp.SetScissorRect(gputypes.ScissorRect{X: 0, Y: 0, Width: w, Height: h})

	rp.SetPipeline(bp.compositePipeline)
	rp.SetBindGroup(0, bp.uniformBindGrp, nil)
	rp.SetBindGroup(1, texBindGrp, nil)
	rp.Draw(gputypes.DrawArgs{VertexCount: 6, InstanceCount: 1}) // 2 triangles for full-screen quad

	if endErr := rp.End(); endErr != nil {
		return fmt.Errorf("compositor: end surface composite pass: %w", endErr)
	}
	return nil
}

// writeFullScreenUniforms fills the uniform data buffer with full-screen quad
// parameters.
func (bp *BlitPipeline) writeFullScreenUniforms(w, h uint32) {
	binary.LittleEndian.PutUint32(bp.uniformData[0:4], math.Float32bits(0))            // x
	binary.LittleEndian.PutUint32(bp.uniformData[4:8], math.Float32bits(0))            // y
	binary.LittleEndian.PutUint32(bp.uniformData[8:12], math.Float32bits(float32(w)))  // width
	binary.LittleEndian.PutUint32(bp.uniformData[12:16], math.Float32bits(float32(h))) // height
	binary.LittleEndian.PutUint32(bp.uniformData[16:20], math.Float32bits(float32(w))) // screenWidth
	binary.LittleEndian.PutUint32(bp.uniformData[20:24], math.Float32bits(float32(h))) // screenHeight
	binary.LittleEndian.PutUint32(bp.uniformData[24:28], math.Float32bits(1.0))        // alpha
	binary.LittleEndian.PutUint32(bp.uniformData[28:32], math.Float32bits(1.0))        // premultiplied
}
