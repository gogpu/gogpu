//go:build darwin

package platform

import "github.com/gogpu/gogpu/internal/platform/darwin"

// SystemScaleFactor returns the primary display DPI scale factor.
// On macOS this queries [NSScreen mainScreen].backingScaleFactor and is safe
// to call before NSApplication initialization or window creation.
func SystemScaleFactor() float64 {
	return darwin.MainScreenScaleFactor()
}

// scaleTransition compares the last observed backing scale with the current
// one. changed reports a change between two known scales, the condition for
// EventScaleChanged / PrepareFrameResult.ScaleChanged. sync reports that the
// scale differs from the last one applied to the CAMetalLayer, which is also
// true on the first observation (last == 0). An unknown current scale (<= 0)
// reports neither.
func scaleTransition(last, cur float64) (changed, sync bool) {
	if cur <= 0 || cur == last {
		return false, false
	}
	return last != 0, true
}
