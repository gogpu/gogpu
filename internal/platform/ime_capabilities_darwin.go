//go:build darwin

package platform

import "github.com/gogpu/gpucontext"

// defaultIMECapabilities reports the NSTextInputClient operations implemented
// by the AppKit backend. AppKit has no portable delete-surrounding callback,
// so that optional capability remains unset.
func defaultIMECapabilities() gpucontext.IMECapabilities {
	return gpucontext.IMECapabilities{
		Version: gpucontext.IMEContractVersion,
		Features: gpucontext.IMECapabilityComposition |
			gpucontext.IMECapabilityCommit |
			gpucontext.IMECapabilityCancel |
			gpucontext.IMECapabilityDisabled |
			gpucontext.IMECapabilityCursorArea |
			gpucontext.IMECapabilitySurroundingText |
			gpucontext.IMECapabilityContentPurpose |
			gpucontext.IMECapabilityContentHints,
	}
}
