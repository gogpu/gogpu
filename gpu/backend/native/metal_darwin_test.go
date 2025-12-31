//go:build darwin

package native

import (
	"testing"
	"time"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/internal/platform/darwin"
)

func TestMetalBackendSurfaceLifecycleDarwin(t *testing.T) {
	backend := New()
	if backend == nil {
		t.Fatal("New returned nil backend")
	}
	if err := backend.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer backend.Destroy()

	layer, err := darwin.NewMetalLayer()
	if err != nil {
		t.Fatalf("NewMetalLayer failed: %v", err)
	}
	defer layer.Release()

	instance, err := backend.CreateInstance()
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	surface, err := backend.CreateSurface(instance, types.SurfaceHandle{Window: layer.Ptr()})
	if err != nil {
		t.Fatalf("CreateSurface failed: %v", err)
	}

	adapter, err := backend.RequestAdapter(instance, nil)
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
	if backend.registry.GetCurrentSurfaceTexture(surface) == nil {
		t.Fatal("surface texture not tracked after GetCurrentTexture")
	}

	encoder := backend.CreateCommandEncoder(device)
	if encoder == 0 {
		t.Fatal("CreateCommandEncoder returned 0")
	}

	cmd := backend.FinishEncoder(encoder)
	backend.ReleaseCommandEncoder(encoder)
	if cmd == 0 {
		t.Fatal("FinishEncoder returned 0")
	}

	backend.Submit(queue, cmd)
	backend.ReleaseCommandBuffer(cmd)

	if backend.registry.GetCurrentSurfaceTexture(surface) == nil {
		t.Fatal("surface texture cleared before Present")
	}

	backend.Present(surface)
	if backend.registry.GetCurrentSurfaceTexture(surface) != nil {
		t.Fatal("surface texture not cleared after Present")
	}

	backend.ReleaseTexture(surfTex.Texture)
}

func acquireSurfaceTexture(t *testing.T, backend *Backend, surface types.Surface) types.SurfaceTexture {
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
