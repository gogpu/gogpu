//go:build js

package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDefaultIMECapabilitiesBrowserContract(t *testing.T) {
	caps := DefaultIMECapabilities()
	if caps.Version != gpucontext.IMEContractVersion {
		t.Fatalf("browser IME capability version = %d, want %d", caps.Version, gpucontext.IMEContractVersion)
	}
}
