//go:build darwin

package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDefaultIMECapabilitiesMatchesDarwinContract(t *testing.T) {
	caps := DefaultIMECapabilities()
	if caps.Version != gpucontext.IMEContractVersion {
		t.Fatalf("default IME capability version = %d, want %d", caps.Version, gpucontext.IMEContractVersion)
	}
	if caps.Features&gpucontext.IMECapabilityComposition == 0 {
		t.Fatalf("default Darwin capabilities omit composition: %+v", caps)
	}
}
