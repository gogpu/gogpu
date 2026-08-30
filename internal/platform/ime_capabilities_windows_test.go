//go:build windows

package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDefaultIMECapabilitiesWindowsContract(t *testing.T) {
	caps := DefaultIMECapabilities()
	if caps.Version != gpucontext.IMEContractVersion {
		t.Fatalf("Windows IME capability version = %d, want %d", caps.Version, gpucontext.IMEContractVersion)
	}
}
