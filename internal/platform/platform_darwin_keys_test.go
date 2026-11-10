//go:build darwin

package platform

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gpucontext"
)

func TestExtractModifiers(t *testing.T) {
	tests := []struct {
		name  string
		flags darwin.NSEventModifierFlags
		want  gpucontext.Modifiers
	}{
		{name: "none", flags: 0, want: 0},
		{name: "shift", flags: darwin.NSEventModifierFlagShift, want: gpucontext.ModShift},
		{name: "control", flags: darwin.NSEventModifierFlagControl, want: gpucontext.ModControl},
		{name: "option", flags: darwin.NSEventModifierFlagOption, want: gpucontext.ModAlt},
		{name: "command", flags: darwin.NSEventModifierFlagCommand, want: gpucontext.ModSuper},
		{name: "caps lock on", flags: darwin.NSEventModifierFlagCapsLock, want: gpucontext.ModCapsLock},
		{
			name:  "caps lock on with shift",
			flags: darwin.NSEventModifierFlagCapsLock | darwin.NSEventModifierFlagShift,
			want:  gpucontext.ModCapsLock | gpucontext.ModShift,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractModifiers(tt.flags); got != tt.want {
				t.Errorf("extractModifiers(%#x) = %v, want %v", tt.flags, got, tt.want)
			}
		})
	}
}
