// Particle simulation demo — full gogpu ecosystem example.
//
// Compute shader updates particle positions on GPU.
// Render pipeline draws particles directly from GPU buffer (zero CPU readback).
//
// Usage: CGO_ENABLED=0 go run .
package main

import (
	"encoding/binary"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const numParticles = 4096
const particleBytes = 16 // x, y, vx, vy — 4x float32
const bufSize = uint64(numParticles * particleBytes)

const computeWGSL = `
struct Particle { pos: vec2<f32>, vel: vec2<f32>, }
struct Params { dt: f32, count: u32, }

@group(0) @binding(0) var<storage, read> pin: array<Particle>;
@group(0) @binding(1) var<storage, read_write> pout: array<Particle>;
@group(0) @binding(2) var<uniform> params: Params;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let i = id.x;
    if (i >= params.count) { return; }
    var p = pin[i];

    // Orbital gravity: attract to center with angular momentum preservation.
    // This creates stable orbits that never collapse to a point.
    let r = length(p.pos);
    let rSafe = max(r, 0.05);
    let gravity = -p.pos / (rSafe * rSafe * rSafe) * 0.15 * params.dt;
    p.vel += gravity;

    // Repel from sampled neighbors (keeps particles spread out)
    let step = max(params.count / 16u, 1u);
    for (var j = 0u; j < params.count; j += step) {
        if (j == i) { continue; }
        let d = p.pos - pin[j].pos;
        let dist = max(length(d), 0.01);
        p.vel += normalize(d) / (dist * dist) * 0.0002 * params.dt;
    }

    // Very light damping — orbits persist indefinitely
    p.vel *= 0.9995;
    p.pos += p.vel * params.dt;

    // Soft boundary: reflect with energy loss at edges
    if (p.pos.x > 1.0) { p.pos.x = 1.0; p.vel.x *= -0.8; }
    if (p.pos.x < -1.0) { p.pos.x = -1.0; p.vel.x *= -0.8; }
    if (p.pos.y > 1.0) { p.pos.y = 1.0; p.vel.y *= -0.8; }
    if (p.pos.y < -1.0) { p.pos.y = -1.0; p.vel.y *= -0.8; }

    pout[i] = p;
}
`

// Render each particle as a small quad via instanced triangle-strip.
// vertex_index 0..3 = quad corners, instance data = particle position+velocity.
const renderWGSL = `
struct Out {
    @builtin(position) pos: vec4<f32>,
    @location(0) col: vec3<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) center: vec2<f32>, @location(1) vel: vec2<f32>) -> Out {
    // Quad corners: 0=(-1,-1), 1=(1,-1), 2=(-1,1), 3=(1,1)
    let x = f32(vid & 1u) * 2.0 - 1.0;
    let y = f32((vid >> 1u) & 1u) * 2.0 - 1.0;
    let size = 0.004; // particle size in NDC

    var o: Out;
    o.pos = vec4<f32>(center.x + x * size, center.y + y * size, 0.0, 1.0);
    let speed = length(vel);
    o.col = vec3<f32>(min(speed * 30.0, 1.0), 0.3, 1.0 - min(speed * 15.0, 1.0));
    return o;
}

@fragment
fn fs_main(@location(0) col: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(col, 1.0);
}
`

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU Particles — Compute + Render").
		WithSize(800, 600).
		WithContinuousRender(true))

	var s *sim
	startTime := time.Now()
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
			log.Printf("GPU: %s | Particles: %d", dc.Backend(), numParticles)
			startTime = time.Now()
		}
		s.frameNum++
		if err := s.frame(sv); err != nil {
			log.Printf("frame %d error: %v", s.frameNum, err)
		}
		if s.frameNum%300 == 0 {
			elapsed := time.Since(startTime).Seconds()
			log.Printf("Frame %d | %.1fs | %.0f FPS | %d particles",
				s.frameNum, elapsed, float64(s.frameNum)/max(elapsed, 0.001), numParticles)
		}
	})

	app.OnClose(func() {
		if s != nil {
			s.release()
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

type sim struct {
	dev      *wgpu.Device
	bufA     *wgpu.Buffer
	bufB     *wgpu.Buffer
	uniform  *wgpu.Buffer
	compPipe *wgpu.ComputePipeline
	compBGL  *wgpu.BindGroupLayout
	compPL   *wgpu.PipelineLayout
	compBG0  *wgpu.BindGroup // A→B
	compBG1  *wgpu.BindGroup // B→A
	rendPipe *wgpu.RenderPipeline
	rendPL   *wgpu.PipelineLayout
	frameNum int
}

func newSim(dev *wgpu.Device, format gputypes.TextureFormat) (*sim, error) {
	s := &sim{dev: dev}

	usage := wgpu.BufferUsageStorage | wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst | wgpu.BufferUsageCopySrc
	var err error
	s.bufA, err = dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "A", Size: bufSize, Usage: usage})
	if err != nil {
		return nil, err
	}
	s.bufB, err = dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "B", Size: bufSize, Usage: usage})
	if err != nil {
		return nil, err
	}
	s.uniform, err = dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "params", Size: 8, Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst})
	if err != nil {
		return nil, err
	}

	// Init particles in orbital ring with tangential velocity
	d := make([]byte, bufSize)
	for i := 0; i < numParticles; i++ {
		o := i * particleBytes
		angle := float64(i) / float64(numParticles) * 2 * math.Pi
		radius := 0.3 + rand.Float64()*0.5 // ring from 0.3 to 0.8
		px := float32(math.Cos(angle) * radius)
		py := float32(math.Sin(angle) * radius)
		// Tangential velocity (perpendicular to radius) for stable orbits
		speed := float32(0.15 + rand.Float64()*0.1)
		vx := float32(-math.Sin(angle)) * speed
		vy := float32(math.Cos(angle)) * speed
		binary.LittleEndian.PutUint32(d[o:], math.Float32bits(px))
		binary.LittleEndian.PutUint32(d[o+4:], math.Float32bits(py))
		binary.LittleEndian.PutUint32(d[o+8:], math.Float32bits(vx))
		binary.LittleEndian.PutUint32(d[o+12:], math.Float32bits(vy))
	}
	dev.Queue().WriteBuffer(s.bufA, 0, d)
	dev.Queue().WriteBuffer(s.bufB, 0, d) // both start same

	pd := make([]byte, 8)
	binary.LittleEndian.PutUint32(pd[0:], math.Float32bits(0.016))
	binary.LittleEndian.PutUint32(pd[4:], numParticles)
	dev.Queue().WriteBuffer(s.uniform, 0, pd)

	// Compute
	cs, err := dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: computeWGSL})
	if err != nil {
		return nil, err
	}
	defer cs.Release()

	bgl, err := dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeReadOnlyStorage}},
			{Binding: 1, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeStorage}},
			{Binding: 2, Visibility: wgpu.ShaderStageCompute, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 8}},
		},
	})
	if err != nil {
		return nil, err
	}
	s.compBGL = bgl

	cpl, err := dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{bgl}})
	if err != nil {
		return nil, err
	}
	s.compPL = cpl

	s.compPipe, err = dev.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{Layout: cpl, Module: cs, EntryPoint: "main"})
	if err != nil {
		return nil, err
	}

	s.compBG0, err = dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Layout: bgl, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: s.bufA, Size: bufSize},
		{Binding: 1, Buffer: s.bufB, Size: bufSize},
		{Binding: 2, Buffer: s.uniform, Size: 8},
	}})
	if err != nil {
		return nil, err
	}
	s.compBG1, err = dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Layout: bgl, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: s.bufB, Size: bufSize},
		{Binding: 1, Buffer: s.bufA, Size: bufSize},
		{Binding: 2, Buffer: s.uniform, Size: 8},
	}})
	if err != nil {
		return nil, err
	}

	// Render
	rs, err := dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: renderWGSL})
	if err != nil {
		return nil, err
	}
	defer rs.Release()

	rpl, err := dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{})
	if err != nil {
		return nil, err
	}
	s.rendPL = rpl

	s.rendPipe, err = dev.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Layout: rpl,
		Vertex: wgpu.VertexState{
			Module: rs, EntryPoint: "vs_main",
			Buffers: []wgpu.VertexBufferLayout{{
				ArrayStride: particleBytes,
				StepMode:    gputypes.VertexStepModeInstance,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
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

func (s *sim) frame(sv *wgpu.TextureView) error {
	// Ping-pong: alternate input/output buffers each frame.
	// Even frames: read A → write B, render B.
	// Odd frames:  read B → write A, render A.
	var bg *wgpu.BindGroup
	var outputBuf *wgpu.Buffer
	if s.frameNum%2 == 0 {
		bg = s.compBG0
		outputBuf = s.bufB
	} else {
		bg = s.compBG1
		outputBuf = s.bufA
	}

	// Compute pass: update particle positions on GPU.
	enc, err := s.dev.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}
	cp, err := enc.BeginComputePass(nil)
	if err != nil {
		return err
	}
	cp.SetPipeline(s.compPipe)
	cp.SetBindGroup(0, bg, nil)
	cp.Dispatch(uint32((numParticles+63)/64), 1, 1)
	cp.End()
	cmds1, err := enc.Finish()
	if err != nil {
		return err
	}
	if _, err := s.dev.Queue().Submit(cmds1); err != nil {
		return err
	}

	// Render pass: draw particles from the compute output buffer.
	enc2, err := s.dev.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}
	rp, err := enc2.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: sv, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0.02, G: 0.02, B: 0.05, A: 1},
		}},
	})
	if err != nil {
		return err
	}
	rp.SetPipeline(s.rendPipe)
	rp.SetVertexBuffer(0, outputBuf, 0)
	rp.Draw(4, numParticles, 0, 0)
	rp.End()
	cmds2, err := enc2.Finish()
	if err != nil {
		return err
	}
	if _, err := s.dev.Queue().Submit(cmds2); err != nil {
		return err
	}
	return nil
}

func (s *sim) release() {
	for _, r := range []interface{ Release() }{
		s.rendPipe, s.rendPL, s.compPipe, s.compPL, s.compBGL,
		s.compBG0, s.compBG1, s.bufA, s.bufB, s.uniform,
	} {
		if r != nil {
			r.Release()
		}
	}
}
