package gogpu

import (
	"errors"
	"fmt"
	"testing"
)

// rootSentinels are the nine sentinel errors the root package exports.
//
// Error identity is part of the frozen v1.0 shape: a consumer matches these
// with errors.Is, so their messages and distinctness are pinned here
// (P2-API-008). Every one of them is produced by a real path (P2-API-009):
// the texture/dimension failures, the pre-Run call-order errors, a Run or
// Start while a HostLoop owns the loop, GPU acquisition, and the
// surface/device loss reports.
var rootSentinels = []struct {
	name string
	err  error
	want string
}{
	{"ErrNotInitialized", ErrNotInitialized, "gogpu: not initialized"},
	{"ErrNoGPU", ErrNoGPU, "gogpu: no suitable GPU found"},
	{"ErrSurfaceLost", ErrSurfaceLost, "gogpu: surface lost"},
	{"ErrDeviceLost", ErrDeviceLost, "gogpu: GPU device lost"},
	{"ErrTextureNil", ErrTextureNil, "gogpu: texture is nil"},
	{"ErrTextureDestroyed", ErrTextureDestroyed, "gogpu: texture has been destroyed"},
	{"ErrInvalidDimensions", ErrInvalidDimensions, "gogpu: invalid texture dimensions"},
	{"ErrInvalidTextureType", ErrInvalidTextureType, "gogpu: texture must be *Texture"},
	{"ErrHostLoopStarted", ErrHostLoopStarted, "gogpu: a HostLoop has started"},
}

func TestRootSentinelsIdentity(t *testing.T) {
	if len(rootSentinels) != 9 {
		t.Fatalf("rootSentinels holds %d entries; the root package exports 9 — a new sentinel needs its row here", len(rootSentinels))
	}

	for _, tt := range rootSentinels {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatalf("%s is nil", tt.name)
			}
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("%s.Error() = %q, want %q", tt.name, got, tt.want)
			}
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%s, %s) = false, want true", tt.name, tt.name)
			}
		})
	}
}

func TestRootSentinelsAreDistinct(t *testing.T) {
	for i := range rootSentinels {
		for j := i + 1; j < len(rootSentinels); j++ {
			a, b := rootSentinels[i], rootSentinels[j]
			if errors.Is(a.err, b.err) || errors.Is(b.err, a.err) {
				t.Errorf("%s and %s must not match each other", a.name, b.name)
			}

			// Wrapping keeps the identity of the wrapped sentinel and nothing else.
			wrapped := fmt.Errorf("gogpu: operation failed: %w", a.err)
			if !errors.Is(wrapped, a.err) {
				t.Errorf("errors.Is(wrapped %s, %s) = false, want true", a.name, a.name)
			}
			if errors.Is(wrapped, b.err) {
				t.Errorf("wrapped %s must not match %s", a.name, b.name)
			}
		}
	}
}
