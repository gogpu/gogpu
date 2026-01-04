//go:build darwin

package gpu_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gogpu/gogpu/gpu"
	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/platform/darwin"
)

const backendTestWGSL = `
@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> @builtin(position) vec4<f32> {
	var positions = array<vec2<f32>, 3>(
		vec2<f32>(0.0, 0.5),
		vec2<f32>(-0.5, -0.5),
		vec2<f32>(0.5, -0.5)
	);
	return vec4<f32>(positions[vertexIndex], 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
	return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`

func TestNativeBackendInterfaceDarwin(t *testing.T) {
	backend := native.New()
	if backend == nil {
		t.Fatal("native.New returned nil")
	}

	gpu.SetBackend(backend)
	defer gpu.SetBackend(nil)
	if got := gpu.GetBackend(); got != backend {
		t.Fatalf("GetBackend = %T, want %T", got, backend)
	}

	if err := backend.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer backend.Destroy()

	layer := newMetalLayer(t)

	instance, err := backend.CreateInstance()
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	surface, err := backend.CreateSurface(instance, types.SurfaceHandle{Window: layer.Ptr()})
	if err != nil {
		t.Fatalf("CreateSurface failed: %v", err)
	}

	adapter, err := backend.RequestAdapter(instance, &types.AdapterOptions{
		PowerPreference: types.PowerPreferenceHighPerformance,
	})
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}

	device, err := backend.RequestDevice(adapter, nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}

	queue := backend.GetQueue(device)
	if queue == 0 {
		t.Fatal("GetQueue returned 0")
	}

	backend.ConfigureSurface(surface, device, &types.SurfaceConfig{
		Format:      types.TextureFormatBGRA8Unorm,
		Usage:       types.TextureUsageRenderAttachment,
		Width:       64,
		Height:      64,
		AlphaMode:   types.AlphaModeOpaque,
		PresentMode: types.PresentModeFifo,
	})

	surfTex := acquireSurfaceTexture(t, backend, surface)
	view := backend.CreateTextureView(surfTex.Texture, nil)
	if view == 0 {
		t.Fatalf("CreateTextureView returned 0")
	}
	defer func() {
		backend.ReleaseTextureView(view)
		backend.ReleaseTexture(surfTex.Texture)
	}()

	shader, err := backend.CreateShaderModuleWGSL(device, backendTestWGSL)
	if err != nil {
		t.Fatalf("CreateShaderModuleWGSL failed: %v", err)
	}

	pipeline, err := backend.CreateRenderPipeline(device, &types.RenderPipelineDescriptor{
		VertexShader:     shader,
		VertexEntryPoint: "vs_main",
		FragmentShader:   shader,
		FragmentEntry:    "fs_main",
		TargetFormat:     types.TextureFormatBGRA8Unorm,
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline failed: %v", err)
	}

	encoder := backend.CreateCommandEncoder(device)
	if encoder == 0 {
		t.Fatal("CreateCommandEncoder returned 0")
	}

	pass := backend.BeginRenderPass(encoder, &types.RenderPassDescriptor{
		ColorAttachments: []types.ColorAttachment{
			{
				View:       view,
				LoadOp:     types.LoadOpClear,
				StoreOp:    types.StoreOpStore,
				ClearValue: types.Color{R: 0.1, G: 0.2, B: 0.3, A: 1.0},
			},
		},
	})
	if pass == 0 {
		t.Fatal("BeginRenderPass returned 0")
	}

	backend.SetPipeline(pass, pipeline)
	backend.Draw(pass, 3, 1, 0, 0)
	backend.SetBindGroup(pass, 0, 0, nil)
	backend.SetVertexBuffer(pass, 0, 0, 0, 0)
	backend.SetIndexBuffer(pass, 0, types.IndexFormatUint16, 0, 0)
	backend.DrawIndexed(pass, 0, 1, 0, 0, 0)

	backend.EndRenderPass(pass)
	backend.ReleaseRenderPass(pass)

	cmd := backend.FinishEncoder(encoder)
	backend.ReleaseCommandEncoder(encoder)
	if cmd == 0 {
		t.Fatal("FinishEncoder returned 0")
	}

	backend.Submit(queue, cmd)
	backend.ReleaseCommandBuffer(cmd)
	backend.Present(surface)

	_, err = backend.CreateTexture(device, nil)
	expectNotImplemented(t, err, "CreateTexture")
	_, err = backend.CreateSampler(device, nil)
	expectNotImplemented(t, err, "CreateSampler")
	_, err = backend.CreateBuffer(device, nil)
	expectNotImplemented(t, err, "CreateBuffer")
	_, err = backend.CreateBindGroupLayout(device, nil)
	expectNotImplemented(t, err, "CreateBindGroupLayout")
	_, err = backend.CreateBindGroup(device, nil)
	expectNotImplemented(t, err, "CreateBindGroup")
	_, err = backend.CreatePipelineLayout(device, nil)
	expectNotImplemented(t, err, "CreatePipelineLayout")

	backend.WriteTexture(queue, nil, nil, nil, nil)
	backend.WriteBuffer(queue, 0, 0, nil)

	backend.ReleaseTexture(0)
	backend.ReleaseTextureView(0)
	backend.ReleaseSampler(0)
	backend.ReleaseBuffer(0)
	backend.ReleaseBindGroupLayout(0)
	backend.ReleaseBindGroup(0)
	backend.ReleasePipelineLayout(0)
	backend.ReleaseCommandBuffer(0)
	backend.ReleaseCommandEncoder(0)
	backend.ReleaseRenderPass(0)
}

func acquireSurfaceTexture(t *testing.T, backend gpu.Backend, surface types.Surface) types.SurfaceTexture {
	t.Helper()

	var last types.SurfaceTexture
	var lastErr error
	for i := 0; i < 5; i++ {
		last, lastErr = backend.GetCurrentTexture(surface)
		if lastErr == nil && last.Status == types.SurfaceStatusSuccess && last.Texture != 0 {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("GetCurrentTexture failed: %v (status=%v)", lastErr, last.Status)
	}
	if last.Status != types.SurfaceStatusSuccess {
		t.Fatalf("GetCurrentTexture status=%v", last.Status)
	}
	t.Fatalf("GetCurrentTexture returned texture=0")
	return types.SurfaceTexture{}
}

func expectNotImplemented(t *testing.T, err error, name string) {
	t.Helper()
	if !errors.Is(err, gpu.ErrNotImplemented) {
		t.Fatalf("%s error = %v, want %v", name, err, gpu.ErrNotImplemented)
	}
}

func newMetalLayer(t *testing.T) *darwin.MetalLayer {
	t.Helper()

	layer, err := darwin.NewMetalLayer()
	if err != nil {
		t.Fatalf("NewMetalLayer failed: %v", err)
	}
	t.Cleanup(layer.Release)
	return layer
}
