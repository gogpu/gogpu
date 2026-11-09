//go:build darwin

package gogpu

import "github.com/gogpu/gogpu/internal/platform/darwin"

func pushFramePool() uintptr { return darwin.PushFramePool() }

func popFramePool(pool uintptr) { darwin.PopFramePool(pool) }

func runInFramePool(fn func()) {
	darwin.RunInFramePool(fn)
}
