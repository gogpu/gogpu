package types

import (
	"testing"
)

func TestBackendTypeString(t *testing.T) {
	tests := []struct {
		backend  BackendType
		expected string
	}{
		{BackendAuto, "Auto"},
		{BackendRust, "Rust (wgpu-gpu)"},
		{BackendNative, "Native (Pure Go)"},
		{BackendGo, "Native (Pure Go)"}, // Alias should return same string
		{BackendType(99), "Auto"},       // Unknown defaults to Auto
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.backend.String()
			if got != tt.expected {
				t.Errorf("BackendType(%d).String() = %q, want %q", tt.backend, got, tt.expected)
			}
		})
	}
}

func TestBackendTypeValues(t *testing.T) {
	// Verify iota ordering: Auto=0, Native=1 (default), Rust=2 (opt-in)
	if BackendAuto != 0 {
		t.Errorf("BackendAuto = %d, want 0", BackendAuto)
	}
	if BackendNative != 1 {
		t.Errorf("BackendNative = %d, want 1", BackendNative)
	}
	if BackendRust != 2 {
		t.Errorf("BackendRust = %d, want 2", BackendRust)
	}
	// BackendGo is an alias for BackendNative
	if BackendGo != BackendNative {
		t.Errorf("BackendGo = %d, want %d (BackendNative)", BackendGo, BackendNative)
	}
}
