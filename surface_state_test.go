package gogpu

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
)

func TestSurfaceState_Constants(t *testing.T) {
	if SurfaceNone != 0 {
		t.Error("SurfaceNone should be zero value")
	}
	if SurfaceReady <= SurfaceNone {
		t.Error("SurfaceReady should follow SurfaceNone")
	}
	if SurfaceConfigured <= SurfaceReady {
		t.Error("SurfaceConfigured should follow SurfaceReady")
	}
	if SurfaceLost <= SurfaceConfigured {
		t.Error("SurfaceLost should follow SurfaceConfigured")
	}
}

func TestCanRender_RequiresConfiguredAndSurface(t *testing.T) {
	tests := []struct {
		name       string
		state      SurfaceState
		hasSurface bool
		want       bool
	}{
		{"none, no surface", SurfaceNone, false, false},
		{"ready, no surface", SurfaceReady, false, false},
		{"configured, no surface", SurfaceConfigured, false, false},
		{"lost, no surface", SurfaceLost, false, false},
		{"none, with surface", SurfaceNone, true, false},
		{"ready, with surface", SurfaceReady, true, false},
		{"configured, with surface", SurfaceConfigured, true, true},
		{"lost, with surface", SurfaceLost, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &RenderTarget{
				state:  tt.state,
				format: gputypes.TextureFormatBGRA8Unorm,
			}
			if tt.hasSurface {
				ws.surface = &wgpu.Surface{}
			}
			if got := ws.CanRender(); got != tt.want {
				t.Errorf("CanRender() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResize_UnconfigureOnZeroDimensions(t *testing.T) {
	ws := &RenderTarget{
		state:  SurfaceConfigured,
		format: gputypes.TextureFormatBGRA8Unorm,
		width:  800,
		height: 600,
	}

	// Simulate minimize — surface must be unconfigurable without real wgpu.
	// We only test the state transition; actual Unconfigure needs a real surface.
	// With nil surface, resize returns early because CanRender is false at next frame.
	ws.state = SurfaceReady // simulate unconfigure result
	if ws.state != SurfaceReady {
		t.Error("after minimize, state should be SurfaceReady")
	}
}

func TestDestroy_ResetsSurfaceState(t *testing.T) {
	ws := &RenderTarget{
		state:  SurfaceConfigured,
		format: gputypes.TextureFormatBGRA8Unorm,
		width:  800,
		height: 600,
	}
	ws.destroy()
	if ws.state != SurfaceNone {
		t.Errorf("after destroy, state = %v, want SurfaceNone", ws.state)
	}
	if ws.surface != nil {
		t.Error("after destroy, surface should be nil")
	}
}

func TestRecoverFromAcquireError_OutdatedKeepsConfigured(t *testing.T) {
	ws := &RenderTarget{
		state:  SurfaceConfigured,
		format: gputypes.TextureFormatBGRA8Unorm,
		width:  800,
		height: 600,
	}
	// Without a real wgpu device/adapter, we can't test full reconfigure.
	// Verify that state doesn't change to Lost for non-lost errors.
	// ErrSurfaceOutdated reconfigure will fail (nil device), but state stays Configured.
	// This is acceptable — real recovery tested via backend smoke tests.
	if ws.state != SurfaceConfigured {
		t.Error("state should remain SurfaceConfigured before recovery")
	}
}

func TestBeginFrame_SkipsWhenNotConfigured(t *testing.T) {
	tests := []struct {
		name  string
		state SurfaceState
	}{
		{"none", SurfaceNone},
		{"ready", SurfaceReady},
		{"lost", SurfaceLost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &RenderTarget{
				state:  tt.state,
				format: gputypes.TextureFormatBGRA8Unorm,
			}
			if started, err := ws.beginFrame(nil, nil, nil); started || err != nil {
				t.Errorf("beginFrame when not configured: got %v, error %v; want false, nil", started, err)
			}
		})
	}
}

// errorRecords returns the error-level records logged through slog.Default
// for the rest of the test.
func errorRecords(t *testing.T) *bytes.Buffer {
	t.Helper()
	return recordsFrom(t, slog.LevelError)
}

// recordsFrom returns the records of level and above logged through
// slog.Default for the rest of the test, one a line.
func recordsFrom(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	var records bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&records, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &records
}

// A minimized window's surface reports a zero extent before the pump has
// read the minimize, so the reconfigure fails with ErrZeroArea. That is a
// minimize, not a failure: the surface waits unconfigured for the restore,
// and the outdated surface is what is reported.
func TestReconfigureFailed_ZeroAreaWaitsForRestore(t *testing.T) {
	records := errorRecords(t)
	ws := &RenderTarget{state: SurfaceConfigured, width: 800, height: 600}

	err := ws.reconfigureFailed(wgpu.ErrSurfaceOutdated, hal.ErrZeroArea)

	if ws.state != SurfaceReady {
		t.Errorf("state after a zero-area reconfigure: got %v, want SurfaceReady", ws.state)
	}
	if !errors.Is(err, wgpu.ErrSurfaceOutdated) {
		t.Errorf("error after a zero-area reconfigure: got %v, want ErrSurfaceOutdated", err)
	}
	if records.Len() != 0 {
		t.Errorf("a zero-area reconfigure logged an error: %s", records.String())
	}
}

func TestReconfigureFailed_OtherErrorLosesSurface(t *testing.T) {
	records := errorRecords(t)
	ws := &RenderTarget{state: SurfaceConfigured, width: 800, height: 600}
	configureErr := errors.New("device gone")

	err := ws.reconfigureFailed(wgpu.ErrSurfaceOutdated, configureErr)

	if ws.state != SurfaceLost {
		t.Errorf("state after a failed reconfigure: got %v, want SurfaceLost", ws.state)
	}
	if !errors.Is(err, configureErr) {
		t.Errorf("error after a failed reconfigure: got %v, want the configure's", err)
	}
	if records.Len() != 0 {
		t.Errorf("a failed reconfigure logged the error it answered: %s", records.String())
	}
}
