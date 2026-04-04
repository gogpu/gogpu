package gogpu

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// mockTexture implements hal.Texture for testing.
type mockTexture struct{}

func (m *mockTexture) Destroy()                            {}
func (m *mockTexture) NativeHandle() uintptr               { return 42 }
func (m *mockTexture) CurrentUsage() gputypes.TextureUsage { return 0 }
func (m *mockTexture) AddPendingRef()                      {}
func (m *mockTexture) DecPendingRef()                      {}

// newMockWgpuTexture creates a *wgpu.Texture wrapping a mock HAL texture for testing.
// The returned texture is non-nil (passes the "is destroyed" check) but
// should not be used for actual GPU operations.
func newMockWgpuTexture() *wgpu.Texture {
	return wgpu.NewTextureFromHAL(&mockTexture{}, nil, 0)
}
