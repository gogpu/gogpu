package gpu

import (
	"testing"

	"github.com/gogpu/gogpu/gpu/types"
)

// mockBackend implements Backend for testing.
// All methods return valid handles (1) - this is intentional for mock testing.
type mockBackend struct {
	name string
}

func (m *mockBackend) Name() string { return m.name }
func (m *mockBackend) Init() error  { return nil }
func (m *mockBackend) Destroy()     {}
func (m *mockBackend) CreateInstance() (types.Instance, error) {
	return 1, nil // Return valid handle for mock
}
func (m *mockBackend) RequestAdapter(types.Instance, *types.AdapterOptions) (types.Adapter, error) {
	return 1, nil
}
func (m *mockBackend) RequestDevice(types.Adapter, *types.DeviceOptions) (types.Device, error) {
	return 1, nil
}
func (m *mockBackend) GetQueue(types.Device) types.Queue { return 1 }
func (m *mockBackend) CreateSurface(types.Instance, types.SurfaceHandle) (types.Surface, error) {
	return 1, nil
}
func (m *mockBackend) ConfigureSurface(types.Surface, types.Device, *types.SurfaceConfig) {}
func (m *mockBackend) GetCurrentTexture(types.Surface) (types.SurfaceTexture, error) {
	return types.SurfaceTexture{Texture: 1}, nil
}
func (m *mockBackend) Present(types.Surface) {}
func (m *mockBackend) CreateShaderModuleWGSL(types.Device, string) (types.ShaderModule, error) {
	return 1, nil
}
func (m *mockBackend) CreateRenderPipeline(types.Device, *types.RenderPipelineDescriptor) (types.RenderPipeline, error) {
	return 1, nil
}
func (m *mockBackend) CreateCommandEncoder(types.Device) types.CommandEncoder { return 1 }
func (m *mockBackend) BeginRenderPass(types.CommandEncoder, *types.RenderPassDescriptor) types.RenderPass {
	return 1
}
func (m *mockBackend) EndRenderPass(types.RenderPass)                         {}
func (m *mockBackend) FinishEncoder(types.CommandEncoder) types.CommandBuffer { return 1 }
func (m *mockBackend) Submit(types.Queue, types.CommandBuffer)                {}
func (m *mockBackend) SetPipeline(types.RenderPass, types.RenderPipeline)     {}
func (m *mockBackend) Draw(types.RenderPass, uint32, uint32, uint32, uint32)  {}
func (m *mockBackend) CreateTexture(types.Device, *types.TextureDescriptor) (types.Texture, error) {
	return 1, nil
}
func (m *mockBackend) CreateTextureView(types.Texture, *types.TextureViewDescriptor) types.TextureView {
	return 1
}
func (m *mockBackend) WriteTexture(types.Queue, *types.ImageCopyTexture, []byte, *types.ImageDataLayout, *types.Extent3D) {
}
func (m *mockBackend) CreateSampler(types.Device, *types.SamplerDescriptor) (types.Sampler, error) {
	return 1, nil
}
func (m *mockBackend) CreateBuffer(types.Device, *types.BufferDescriptor) (types.Buffer, error) {
	return 1, nil
}
func (m *mockBackend) WriteBuffer(types.Queue, types.Buffer, uint64, []byte) {}
func (m *mockBackend) CreateBindGroupLayout(types.Device, *types.BindGroupLayoutDescriptor) (types.BindGroupLayout, error) {
	return 1, nil
}
func (m *mockBackend) CreateBindGroup(types.Device, *types.BindGroupDescriptor) (types.BindGroup, error) {
	return 1, nil
}
func (m *mockBackend) CreatePipelineLayout(types.Device, *types.PipelineLayoutDescriptor) (types.PipelineLayout, error) {
	return 1, nil
}
func (m *mockBackend) SetBindGroup(types.RenderPass, uint32, types.BindGroup, []uint32)       {}
func (m *mockBackend) SetVertexBuffer(types.RenderPass, uint32, types.Buffer, uint64, uint64) {}
func (m *mockBackend) SetIndexBuffer(types.RenderPass, types.Buffer, types.IndexFormat, uint64, uint64) {
}
func (m *mockBackend) DrawIndexed(types.RenderPass, uint32, uint32, uint32, int32, uint32) {}
func (m *mockBackend) ReleaseTexture(types.Texture)                                        {}
func (m *mockBackend) ReleaseTextureView(types.TextureView)                                {}
func (m *mockBackend) ReleaseSampler(types.Sampler)                                        {}
func (m *mockBackend) ReleaseBuffer(types.Buffer)                                          {}
func (m *mockBackend) ReleaseBindGroupLayout(types.BindGroupLayout)                        {}
func (m *mockBackend) ReleaseBindGroup(types.BindGroup)                                    {}
func (m *mockBackend) ReleasePipelineLayout(types.PipelineLayout)                          {}
func (m *mockBackend) ReleaseCommandBuffer(types.CommandBuffer)                            {}
func (m *mockBackend) ReleaseCommandEncoder(types.CommandEncoder)                          {}
func (m *mockBackend) ReleaseRenderPass(types.RenderPass)                                  {}

func TestRegisterBackend(t *testing.T) {
	// Clean up any existing backends first
	registryMu.Lock()
	oldBackends := backends
	backends = make(map[string]BackendFactory)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		backends = oldBackends
		registryMu.Unlock()
	}()

	// Register a mock backend
	RegisterBackend("test", func() Backend {
		return &mockBackend{name: "test"}
	})

	if !IsBackendRegistered("test") {
		t.Error("backend should be registered")
	}

	available := AvailableBackends()
	if len(available) != 1 || available[0] != "test" {
		t.Errorf("expected [test], got %v", available)
	}
}

func TestCreateBackend(t *testing.T) {
	// Clean up any existing backends first
	registryMu.Lock()
	oldBackends := backends
	backends = make(map[string]BackendFactory)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		backends = oldBackends
		registryMu.Unlock()
	}()

	RegisterBackend("mock", func() Backend {
		return &mockBackend{name: "mock"}
	})

	b := CreateBackend("mock")
	if b == nil {
		t.Fatal("expected backend, got nil")
	}
	if b.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", b.Name())
	}

	// Non-existent backend should return nil
	if CreateBackend("nonexistent") != nil {
		t.Error("expected nil for non-existent backend")
	}
}

func TestSelectBestBackend(t *testing.T) {
	// Clean up any existing backends first
	registryMu.Lock()
	oldBackends := backends
	backends = make(map[string]BackendFactory)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		backends = oldBackends
		registryMu.Unlock()
	}()

	// Register native first, then rust
	RegisterBackend("native", func() Backend {
		return &mockBackend{name: "native"}
	})
	RegisterBackend("rust", func() Backend {
		return &mockBackend{name: "rust"}
	})

	// rust should be selected due to priority
	b := SelectBestBackend()
	if b == nil {
		t.Fatal("expected backend, got nil")
	}
	if b.Name() != "rust" {
		t.Errorf("expected 'rust' (higher priority), got '%s'", b.Name())
	}
}

func TestSelectBestBackendNoBackends(t *testing.T) {
	// Clean up any existing backends first
	registryMu.Lock()
	oldBackends := backends
	backends = make(map[string]BackendFactory)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		backends = oldBackends
		registryMu.Unlock()
	}()

	if SelectBestBackend() != nil {
		t.Error("expected nil when no backends registered")
	}
}

func TestUnregisterBackend(t *testing.T) {
	// Clean up any existing backends first
	registryMu.Lock()
	oldBackends := backends
	backends = make(map[string]BackendFactory)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		backends = oldBackends
		registryMu.Unlock()
	}()

	RegisterBackend("temp", func() Backend {
		return &mockBackend{name: "temp"}
	})

	if !IsBackendRegistered("temp") {
		t.Error("backend should be registered")
	}

	UnregisterBackend("temp")

	if IsBackendRegistered("temp") {
		t.Error("backend should be unregistered")
	}
}

func TestMustSelectBackendPanic(t *testing.T) {
	// Clean up any existing backends first
	registryMu.Lock()
	oldBackends := backends
	backends = make(map[string]BackendFactory)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		backends = oldBackends
		registryMu.Unlock()
	}()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic, got none")
		}
	}()

	MustSelectBackend()
}
