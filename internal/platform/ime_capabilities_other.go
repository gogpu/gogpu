//go:build !windows

package platform

import "github.com/gogpu/gpucontext"

// defaultIMECapabilities reports no native IME backend on this target yet.
// The X11, Wayland and macOS backends land in their own changes and extend
// this file with the bits they really implement.
func defaultIMECapabilities() gpucontext.IMECapabilities {
	return 0
}
