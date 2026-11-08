//go:build windows

package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDefaultIMECapabilitiesOnWindows(t *testing.T) {
	caps := DefaultIMECapabilities()
	if !caps.Has(gpucontext.IMECapabilityCursorArea) {
		t.Fatalf("capabilities = %v, want the cursor-area bit", caps)
	}
	// IMM32 has no API for these, so the backend must not advertise them.
	if caps.Has(gpucontext.IMECapabilityContentType) {
		t.Error("capabilities advertise content type, which IMM32 cannot express")
	}
	if caps.Has(gpucontext.IMECapabilitySurroundingText) {
		t.Error("capabilities advertise surrounding text, which IMM32 cannot express")
	}
}
