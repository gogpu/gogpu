//go:build windows || linux || darwin

package native

import (
	"errors"
	"runtime"
	"testing"

	"github.com/gogpu/gogpu/gpu"
)

// TestBackendNotStub verifies that the native backend is properly implemented
// for supported platforms (Windows, Linux, macOS) and not using the stub.
func TestBackendNotStub(t *testing.T) {
	b := New()

	// Backend should not be nil
	if b == nil {
		t.Fatal("New() returned nil backend")
	}

	// Name should indicate real implementation, not stub
	name := b.Name()
	if name == "" {
		t.Error("Backend name is empty")
	}

	t.Logf("Platform: %s/%s, Backend: %s", runtime.GOOS, runtime.GOARCH, name)

	// Verify backend name matches expected implementation
	switch runtime.GOOS {
	case "windows", "linux":
		if name != "Pure Go (gogpu/wgpu/vulkan)" {
			t.Errorf("Expected Vulkan backend on %s, got: %s", runtime.GOOS, name)
		}
	case "darwin":
		if name != "Pure Go (gogpu/wgpu/metal)" {
			t.Errorf("Expected Metal backend on darwin, got: %s", runtime.GOOS)
		}
	}
}

// TestBackendInitNotStub verifies that Init() doesn't return ErrNotImplemented.
// This test catches the case where a stub backend is accidentally used on
// supported platforms.
func TestBackendInitNotStub(t *testing.T) {
	b := New()

	err := b.Init()

	// The key check: Init() should NOT return ErrNotImplemented on supported platforms
	if errors.Is(err, gpu.ErrNotImplemented) {
		t.Fatalf("Backend.Init() returned ErrNotImplemented on %s - stub backend is being used instead of real implementation!", runtime.GOOS)
	}

	// Init may fail for other reasons (no GPU, driver issues, etc.)
	// That's OK - the important thing is it's not the stub
	if err != nil {
		t.Logf("Backend.Init() returned error (expected on CI without GPU): %v", err)
	} else {
		t.Log("Backend.Init() succeeded")
		// Clean up
		b.Destroy()
	}
}

// TestBackendRegistry verifies that ResourceRegistry is properly initialized.
func TestBackendRegistry(t *testing.T) {
	b := New()

	if b.registry == nil {
		t.Fatal("Backend registry is nil")
	}
}
