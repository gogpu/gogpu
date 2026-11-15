//go:build windows

package platform

import "github.com/gogpu/gpucontext"

// defaultIMECapabilities reports what the IMM32 backend implements. IMM32
// gives the composition/commit/cancel lifecycle and candidate placement via
// ImmSetCandidateWindow. It has no API for surrounding text or content hints;
// those arrive with the TSF backend (ADR-045).
func defaultIMECapabilities() gpucontext.IMECapabilities {
	return gpucontext.IMECapabilityCursorArea
}
