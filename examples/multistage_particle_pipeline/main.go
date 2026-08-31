// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT
// Usage: CGO_ENABLED=0 go run .
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"

	_ "embed"
)

const gridWidth = 160 * 2
const gridHeight = 90 * 2
const numCells = gridWidth * gridHeight
const numParticles = numCells / 18
const particleBytes = 8 // vec2<f32> = 8 bytes
const bufSize = uint64(numParticles * particleBytes)
const uniformSize = 12 // 3 x u32

//go:embed interactor.wgsl
var interactorWGSL string

//go:embed renderer.wgsl
var renderWGSL string

//go:embed bouncer.wgsl
var bouncerWGSL string

var frameCounter int

func keepPrinting() {
	for {
		prev := frameCounter
		time.Sleep(time.Second)
		fmt.Printf("%v FPS: %d | Bucketer → Interactor → Bouncer\n", frameCounter, frameCounter-prev)
	}
}

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU Particles — 3-Stage Compute").
		WithSize(1920/2, 1080/2).
		WithContinuousRender(true))

	var s *sim
	app.OnDraw(func(dc *gogpu.Context) {
		p := app.DeviceProvider()
		if p == nil {
			return
		}
		sv := dc.SurfaceView()
		if sv == nil {
			return
		}
		if s == nil {
			var err error
			s, err = newSim(p.Device(), p.SurfaceFormat())
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("GPU: %s | Particles: %d | Cells: %d", dc.Backend(), numParticles, numCells)
		}
		if err := s.frame(sv); err != nil {
			log.Printf("frame error: %v", err)
		}
		frameCounter++
	})
	app.OnClose(func() {
		if s != nil {
			s.release()
		}
	})
	go keepPrinting()
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

type sim struct {
	dev         *wgpu.Device
	bufA        *wgpu.Buffer
	bufB        *wgpu.Buffer
	uniform     *wgpu.Buffer
	interPipe   *wgpu.ComputePipeline
	interBGL    *wgpu.BindGroupLayout
	interPL     *wgpu.PipelineLayout
	bouncerPipe *wgpu.ComputePipeline
	bouncerBGL  *wgpu.BindGroupLayout
	bouncerPL   *wgpu.PipelineLayout
	rendPipe    *wgpu.RenderPipeline
	rendBGL     *wgpu.BindGroupLayout
	rendPL      *wgpu.PipelineLayout
}

func newSim(dev *wgpu.Device, format gputypes.TextureFormat) (*sim, error) {
	s := &sim{dev: dev}

	// Particle buffer usage
	particleUsage := wgpu.BufferUsageStorage | wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst | wgpu.BufferUsageCopySrc
	var err error
	s.bufA, err = dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "Particles A", Size: bufSize, Usage: particleUsage})
	if err != nil {
		return nil, err
	}
	s.bufB, err = dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "Particles B", Size: bufSize, Usage: particleUsage})
	if err != nil {
		return nil, err
	}

	// Uniform buffer (particleCount, cellsW, cellsH + padding floats = 5x u32 = 20 bytes)
	s.uniform, err = dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "uniform", Size: uniformSize, Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst})
	if err != nil {
		return nil, err
	}

	// Initialize particle data (vec2<f32> positions)
	particleDataA := make([]byte, bufSize)
	particleDataB := make([]byte, bufSize)
	for i := 0; i < numParticles; i++ {
		o := i * particleBytes
		var vx, vy, x, y float32
		x, y = rand.Float32(), rand.Float32()
		x *= gridWidth
		y *= gridHeight
		if i == 1 {
			x, y = 0.9, 0.01
			vx, vy = 0, 0.05
		}
		binary.LittleEndian.PutUint32(particleDataA[o:], math.Float32bits(x))
		binary.LittleEndian.PutUint32(particleDataA[o+4:], math.Float32bits(y))
		x += vx
		y += vy
		binary.LittleEndian.PutUint32(particleDataB[o:], math.Float32bits(x))
		binary.LittleEndian.PutUint32(particleDataB[o+4:], math.Float32bits(y))
	}
	dev.Queue().WriteBuffer(s.bufA, 0, particleDataA)
	dev.Queue().WriteBuffer(s.bufB, 0, particleDataB)
	// Initialize uniform data (particleCount, cellsW, cellsH, float gridW, float gridH)
	pd := make([]byte, uniformSize)
	binary.LittleEndian.PutUint32(pd[0:], uint32(numParticles))
	binary.LittleEndian.PutUint32(pd[4:], uint32(gridWidth))
	binary.LittleEndian.PutUint32(pd[8:], uint32(gridHeight))
	dev.Queue().WriteBuffer(s.uniform, 0, pd)

	// ==================== INTERACTOR ====================
	interCS, err := dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: interactorWGSL})
	if err != nil {
		return nil, err
	}
	defer interCS.Release()

	interBGL, err := dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: uniformSize}},
			{Binding: 2, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeStorage}},
			{Binding: 3, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeReadOnlyStorage}},
		},
	})
	if err != nil {
		return nil, err
	}
	s.interBGL = interBGL

	interPL, err := dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{interBGL}})
	if err != nil {
		return nil, err
	}
	s.interPL = interPL

	s.interPipe, err = dev.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{Layout: interPL, Module: interCS, EntryPoint: "main"})
	if err != nil {
		return nil, err
	}
	// Interactor BindGroup is created dynamically in frame()

	// ==================== BOUNCER ====================
	bouncerCS, err := dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: bouncerWGSL})
	if err != nil {
		return nil, err
	}
	defer bouncerCS.Release()

	bouncerBGL, err := dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: uniformSize}},
			{Binding: 1, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeStorage}},
			{Binding: 2, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeStorage}},
		},
	})
	if err != nil {
		return nil, err
	}
	s.bouncerBGL = bouncerBGL

	bouncerPL, err := dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{bouncerBGL}})
	if err != nil {
		return nil, err
	}
	s.bouncerPL = bouncerPL

	s.bouncerPipe, err = dev.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{Layout: bouncerPL, Module: bouncerCS, EntryPoint: "main"})
	if err != nil {
		return nil, err
	}
	// Bouncer BindGroup is created dynamically in frame()

	// ==================== RENDER ====================
	rs, err := dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: renderWGSL})
	if err != nil {
		return nil, err
	}
	defer rs.Release()

	s.rendBGL, err = dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: uniformSize}},
		},
	})
	if err != nil {
		return nil, err
	}

	s.rendPL, err = dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{s.rendBGL}})
	if err != nil {
		return nil, err
	}

	s.rendPipe, err = dev.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Layout: s.rendPL,
		Vertex: wgpu.VertexState{
			Module: rs, EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: particleBytes,
				StepMode:    gputypes.VertexStepModeInstance,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleStrip},
		Fragment: &wgpu.FragmentState{
			Module: rs, EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{Format: format, WriteMask: gputypes.ColorWriteMaskAll}},
		},
	})
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *sim) release() {
	releaseResources := []interface{ Release() }{
		s.rendPipe, s.bouncerPipe, s.interPipe,
		s.rendBGL, s.bouncerBGL, s.interBGL,
		s.rendPL, s.bouncerPL, s.interPL,
		s.uniform, s.bufB, s.bufA,
	}
	for _, r := range releaseResources {
		if r != nil {
			r.Release()
		}
	}
}

func (s *sim) frame(sv *wgpu.TextureView) error {
	enc, err := s.dev.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}

	// Step 2: Interactor (read cellBuffer + particles, write to bufB)
	interBG, err := s.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Layout: s.interBGL, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: s.uniform, Size: uniformSize},
		{Binding: 2, Buffer: s.bufA, Size: bufSize},
		{Binding: 3, Buffer: s.bufB, Size: bufSize},
	}})
	if err != nil {
		return err
	}
	defer interBG.Release()

	cpInter, err := enc.BeginComputePass(nil)
	if err != nil {
		return err
	}
	cpInter.SetPipeline(s.interPipe)
	cpInter.SetBindGroup(0, interBG, nil)
	cpInter.Dispatch(uint32((numParticles+63)/64), 1, 1)
	cpInter.End()

	// Step 3: Bouncer (ensure particles stay within bounds, write back to bufA)
	bouncerBG, err := s.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Layout: s.bouncerBGL, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: s.uniform, Size: uniformSize},
		{Binding: 1, Buffer: s.bufA, Size: bufSize},
		{Binding: 2, Buffer: s.bufB, Size: bufSize},
	}})
	if err != nil {
		return err
	}
	defer bouncerBG.Release()

	cpBouncer, err := enc.BeginComputePass(nil)
	if err != nil {
		return err
	}
	cpBouncer.SetPipeline(s.bouncerPipe)
	cpBouncer.SetBindGroup(0, bouncerBG, nil)
	cpBouncer.Dispatch(uint32((numParticles+63)/64), 1, 1)
	cpBouncer.End()

	// Step 4: Render (draw from bufB which now has updated positions)
	rendBG, err := s.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Layout: s.rendBGL, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: s.uniform, Size: uniformSize},
	}})
	if err != nil {
		return err
	}
	defer rendBG.Release()

	rp, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: sv, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0.02, G: 0.02, B: 0.05, A: 1},
		}},
	})
	if err != nil {
		return err
	}
	rp.SetPipeline(s.rendPipe)
	rp.SetBindGroup(0, rendBG, nil)
	rp.SetVertexBuffer(0, s.bufB, 0)
	rp.Draw(gputypes.DrawArgs{VertexCount: 4, InstanceCount: numParticles})
	rp.End()

	cmds, err := enc.Finish()
	if err != nil {
		return err
	}

	if _, err := s.dev.Queue().Submit(cmds); err != nil {
		return err
	}

	s.bufA, s.bufB = s.bufB, s.bufA

	return nil
}
