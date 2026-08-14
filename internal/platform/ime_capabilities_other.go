//go:build !windows

package platform

import "github.com/gogpu/gpucontext"

func defaultIMECapabilities() gpucontext.IMECapabilities {
	// Central callback/controller plumbing is present on every target, but the
	// native backend is intentionally advertised only where this phase ships
	// an implementation. Later backend phases opt in explicitly.
	return gpucontext.IMECapabilities{Version: gpucontext.IMEContractVersion}
}
