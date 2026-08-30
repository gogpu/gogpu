//go:build js && wasm

package platform

import "github.com/gogpu/gpucontext"

func defaultIMECapabilities() gpucontext.IMECapabilities {
	return gpucontext.IMECapabilities{
		Version: gpucontext.IMEContractVersion,
		Features: gpucontext.IMECapabilityComposition |
			gpucontext.IMECapabilityCommit |
			gpucontext.IMECapabilityCancel |
			gpucontext.IMECapabilityDisabled |
			gpucontext.IMECapabilityDeleteSurrounding |
			gpucontext.IMECapabilityCursorArea |
			gpucontext.IMECapabilitySurroundingText |
			gpucontext.IMECapabilityContentPurpose |
			gpucontext.IMECapabilityContentHints,
	}
}
