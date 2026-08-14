//go:build windows

package platform

import "github.com/gogpu/gpucontext"

func defaultIMECapabilities() gpucontext.IMECapabilities {
	return gpucontext.IMECapabilities{
		Version: gpucontext.IMEContractVersion,
		Features: gpucontext.IMECapabilityComposition |
			gpucontext.IMECapabilityCommit |
			gpucontext.IMECapabilityCancel |
			gpucontext.IMECapabilityDisabled |
			gpucontext.IMECapabilityCursorArea |
			gpucontext.IMECapabilityContentPurpose |
			gpucontext.IMECapabilityContentHints,
	}
}
