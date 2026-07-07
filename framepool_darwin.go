//go:build darwin

package gogpu

import "github.com/gogpu/gogpu/internal/platform/darwin"

func runInFramePool(fn func()) {
	darwin.RunInFramePool(fn)
}
