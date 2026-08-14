//go:build !windows && !darwin && !js

package platform

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestDefaultIMECapabilitiesOtherTarget(t *testing.T) {
	caps := DefaultIMECapabilities()
	if caps.Version != gpucontext.IMEContractVersion || caps.Features != 0 {
		t.Fatalf("other-target capabilities = %+v, want version-only", caps)
	}
}
