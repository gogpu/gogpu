//go:build !darwin

package gogpu

func pushFramePool() uintptr { return 0 }

func popFramePool(uintptr) {}

func runInFramePool(fn func()) {
	fn()
}
