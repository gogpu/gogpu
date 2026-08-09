package main

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"math"
)

// initResources initializes all resources consumed by the GPU shader.
func initResources(
	device *wgpu.Device,
	iterations float64,
) (
	[]uint32,
	*wgpu.Buffer,
	*wgpu.Buffer,
	*wgpu.BindGroupLayout,
	*wgpu.BindGroupLayout,
	*wgpu.ComputePipeline,
) {
	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: shaderCode})
	if err != nil {
		panic(err)
	}

	paletteColors, paletteBuf := initPaletteBuf(device, iterations)
	uniformBuf := initUniformBuf(device)
	bgLayout0, bgLayout1 := initBindGroupLayouts(device)
	layout := initPipelineLayout(device, bgLayout0, bgLayout1)
	pipeline := initPipeline(device, layout, shader)

	return paletteColors, paletteBuf, uniformBuf, bgLayout0, bgLayout1, pipeline
}

// initPalette initializes the pre-computed color palette used by the GPU shader to render colored pixels on the canvas.
func initPalette(iterations float64) []uint32 {
	colors := make([]uint32, paletteSize)
	for i := range paletteSize {
		iterations := float64(i) * (iterations / float64(paletteSize))
		a := uint32(255)
		r := uint32(math.Sin(0.015*iterations+1.0)*127 + 128)
		g := uint32(math.Sin(0.012*iterations+2.0)*127 + 128)
		b := uint32(math.Sin(0.010*iterations+4.0)*127 + 128)
		colors[i] = r | (g << 8) | (b << 16) | (a << 24)
	}
	return colors
}

// initPaletteBuf initializes the pre-computed color palette and corresponding buffer passed to the GPU shader.
func initPaletteBuf(device *wgpu.Device, iterations float64) ([]uint32, *wgpu.Buffer) {
	paletteColors := initPalette(iterations)
	paletteBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  uint64(len(paletteColors) * 4),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		panic(err)
	}
	return paletteColors, paletteBuf
}

// initUniformBuf initializes the uniform buffer used to pass uniforms to the GPU shader.
func initUniformBuf(device *wgpu.Device) *wgpu.Buffer {
	uniformBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  48,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		panic(err)
	}
	return uniformBuf
}

// initBindGroupLayouts initializes the resource bindings for the GPU shader.
func initBindGroupLayouts(device *wgpu.Device) (*wgpu.BindGroupLayout, *wgpu.BindGroupLayout) {
	bgLayout0, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{ // uniforms
				Binding:    0,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform},
			}, { // pre-computed 2000-color palette
				Binding:    1,
				Visibility: wgpu.ShaderStageCompute,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeStorage},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	bgLayout1, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{{ // storage texture (physical screen)
			Binding:    0,
			Visibility: wgpu.ShaderStageCompute,
			StorageTexture: &gputypes.StorageTextureBindingLayout{
				Format: gputypes.TextureFormatBGRA8Unorm,
				Access: gputypes.StorageTextureAccessWriteOnly,
			},
		}},
	})
	if err != nil {
		panic(err)
	}
	return bgLayout0, bgLayout1
}

// initPipelineLayout initializes the GPU shader compute pipeline layout.
func initPipelineLayout(device *wgpu.Device, bgLayout0, bgLayout1 *wgpu.BindGroupLayout) *wgpu.PipelineLayout {
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgLayout0, bgLayout1},
	})
	if err != nil {
		panic(err)
	}
	return layout
}

// initPipeline initializes the GPU shader compute pipeline.
func initPipeline(device *wgpu.Device, layout *wgpu.PipelineLayout, shader *wgpu.ShaderModule) *wgpu.ComputePipeline {
	pipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Layout:     layout,
		Module:     shader,
		EntryPoint: "main",
	})
	if err != nil {
		panic(err)
	}
	return pipeline
}
