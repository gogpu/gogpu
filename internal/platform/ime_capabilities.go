package platform

import "github.com/gogpu/gpucontext"

// DefaultIMECapabilities reports the IME capabilities of the current build
// target before a native window exists. Platform-specific files provide the
// target's implementation; after the window exists, the window's own
// IMECapabilities reports the active backend.
func DefaultIMECapabilities() gpucontext.IMECapabilities {
	return defaultIMECapabilities()
}
