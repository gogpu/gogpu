//go:build !windows

package platform

import "testing"

func TestDefaultIMECapabilitiesWithoutBackend(t *testing.T) {
	if caps := DefaultIMECapabilities(); caps != 0 {
		t.Fatalf("capabilities = %v, want none until a native backend lands on this target", caps)
	}
}
