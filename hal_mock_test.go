package gogpu

import (
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/wgputest"
)

// newMockWgpuTexture creates a *wgpu.Texture handle that is non-nil (passes
// the "is destroyed" check) but carries no backend object: any GPU operation
// on it fails with wgpu's validation error instead of panicking, and Release
// is a no-op. That is all the checks around these tests need.
func newMockWgpuTexture() *wgpu.Texture {
	return wgputest.NewDetachedTexture(nil)
}
