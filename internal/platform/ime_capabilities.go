package platform

import "github.com/gogpu/gpucontext"

// DefaultIMECapabilities reports the capabilities guaranteed by the current
// build target before a native window exists. Platform-specific files provide
// the target's implementation; callers should still inspect the active
// PlatformWindow provider after creation.
func DefaultIMECapabilities() gpucontext.IMECapabilities {
	return defaultIMECapabilities()
}
