//go:build !rust

package gogpu

import "testing"

func TestRustHalAvailable(t *testing.T) {
	if rustHalAvailable() {
		t.Error("rustHalAvailable() should return false without rust build tag")
	}
}

func TestNewRustHalBackend(t *testing.T) {
	backend, name, variant := newRustHalBackend()
	if backend != nil {
		t.Error("newRustHalBackend() backend should be nil without rust build tag")
	}
	if name != "" {
		t.Errorf("newRustHalBackend() name = %q, want empty", name)
	}
	if variant != 0 {
		t.Errorf("newRustHalBackend() variant = %v, want 0", variant)
	}
}
