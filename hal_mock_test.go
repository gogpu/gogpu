package gogpu

// mockTexture implements hal.Texture for testing.
type mockTexture struct{}

func (m *mockTexture) Destroy()              {}
func (m *mockTexture) NativeHandle() uintptr { return 42 }

// mockTextureView implements hal.TextureView for testing.
type mockTextureView struct{}

func (m *mockTextureView) Destroy()              {}
func (m *mockTextureView) NativeHandle() uintptr { return 43 }

// mockSampler implements hal.Sampler for testing.
type mockSampler struct{}

func (m *mockSampler) Destroy()              {}
func (m *mockSampler) NativeHandle() uintptr { return 44 }
