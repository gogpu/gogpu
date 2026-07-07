//go:build !darwin

package gogpu

func runInFramePool(fn func()) {
	fn()
}
