package gogpu

import (
	"errors"
	"testing"
)

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrNotInitialized, "gogpu: not initialized"},
		{ErrPlatformNotSupported, "gogpu: platform not supported"},
		{ErrNoGPU, "gogpu: no suitable GPU found"},
		{ErrSurfaceLost, "gogpu: surface lost"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("error = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	errs := []error{
		ErrNotInitialized,
		ErrPlatformNotSupported,
		ErrNoGPU,
		ErrSurfaceLost,
	}

	for i := 0; i < len(errs); i++ {
		for j := i + 1; j < len(errs); j++ {
			if errors.Is(errs[i], errs[j]) {
				t.Errorf("errors should be distinct: %v == %v", errs[i], errs[j])
			}
		}
	}
}
