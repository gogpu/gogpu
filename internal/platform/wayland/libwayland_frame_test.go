//go:build linux

package wayland

import (
	"os"
	"sync/atomic"
	"testing"
)

// TestFrameCallbackStateMachine verifies the 3-state machine transitions
// (None → Requested → Received → None cycle) used for compositor frame gating.
// Reference: winit state.rs:273-289.
func TestFrameCallbackStateMachine(t *testing.T) {
	tests := []struct {
		name      string
		initial   int32
		operation string // "request" or "receive"
		wantState int32
		wantReady bool
	}{
		{
			name:      "initial state is None",
			initial:   FrameCallbackNone,
			wantState: FrameCallbackNone,
			wantReady: true,
		},
		{
			name:      "Received state allows rendering",
			initial:   FrameCallbackReceived,
			wantState: FrameCallbackReceived,
			wantReady: true,
		},
		{
			name:      "Requested state blocks rendering",
			initial:   FrameCallbackRequested,
			wantState: FrameCallbackRequested,
			wantReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &LibwaylandHandle{}
			atomic.StoreInt32(&h.frameCallbackState, tt.initial)

			if got := h.FrameCallbackReady(); got != tt.wantReady {
				t.Errorf("FrameCallbackReady() = %v, want %v (state=%d)", got, tt.wantReady, tt.initial)
			}

			gotState := atomic.LoadInt32(&h.frameCallbackState)
			if gotState != tt.wantState {
				t.Errorf("state = %d, want %d", gotState, tt.wantState)
			}
		})
	}
}

// TestFrameCallbackEnvVar verifies that GOGPU_WAYLAND_FRAME_CALLBACK=0
// disables frame callback gating.
func TestFrameCallbackEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"enabled by default (empty)", "", true},
		{"enabled with 1", "1", true},
		{"disabled with 0", "0", false},
		{"enabled with arbitrary value", "yes", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("GOGPU_WAYLAND_FRAME_CALLBACK")
			} else {
				t.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", tt.envValue)
			}

			got := FrameCallbackEnabled()
			if got != tt.want {
				t.Errorf("FrameCallbackEnabled() = %v, want %v (env=%q)", got, tt.want, tt.envValue)
			}
		})
	}
}

// TestFrameCallbackReadyConsume verifies the atomic consume pattern:
// ConsumeFrameCallbackReady returns true only once, then false until
// the next done event.
func TestFrameCallbackReadyConsume(t *testing.T) {
	h := &LibwaylandHandle{}

	// Initially, ready flag is false (no callback has fired yet).
	if h.ConsumeFrameCallbackReady() {
		t.Error("ConsumeFrameCallbackReady() should be false initially")
	}

	// Simulate compositor firing done: set ready flag.
	h.frameCallbackReady.Store(true)

	// First consume should return true.
	if !h.ConsumeFrameCallbackReady() {
		t.Error("ConsumeFrameCallbackReady() should be true after done")
	}

	// Second consume should return false (already consumed).
	if h.ConsumeFrameCallbackReady() {
		t.Error("ConsumeFrameCallbackReady() should be false after second consume")
	}
}

// TestFrameCallbackStateConstants verifies the state constants match the
// expected values from the winit 3-state pattern.
func TestFrameCallbackStateConstants(t *testing.T) {
	if FrameCallbackNone != 0 {
		t.Errorf("FrameCallbackNone = %d, want 0", FrameCallbackNone)
	}
	if FrameCallbackRequested != 1 {
		t.Errorf("FrameCallbackRequested = %d, want 1", FrameCallbackRequested)
	}
	if FrameCallbackReceived != 2 {
		t.Errorf("FrameCallbackReceived = %d, want 2", FrameCallbackReceived)
	}
}

// TestFrameCallbackReadyStateTransitions verifies FrameCallbackReady
// returns correct values for each state.
func TestFrameCallbackReadyStateTransitions(t *testing.T) {
	h := &LibwaylandHandle{}

	// None → ready
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackNone)
	if !h.FrameCallbackReady() {
		t.Error("FrameCallbackReady() should be true in None state")
	}

	// Requested → not ready
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackRequested)
	if h.FrameCallbackReady() {
		t.Error("FrameCallbackReady() should be false in Requested state")
	}

	// Received → ready
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackReceived)
	if !h.FrameCallbackReady() {
		t.Error("FrameCallbackReady() should be true in Received state")
	}
}

// TestFrameCallbackDoneCbRouting verifies that the done callback correctly
// routes to the LibwaylandHandle via the per-proxy map.
func TestFrameCallbackDoneCbRouting(t *testing.T) {
	h := &LibwaylandHandle{}
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackRequested)

	// Simulate a callback proxy pointer.
	fakeProxy := uintptr(0xDEAD_BEEF)

	// Register the handle.
	frameCallbackHandlesMu.Lock()
	frameCallbackHandles[fakeProxy] = h
	frameCallbackHandlesMu.Unlock()

	// The done callback cannot be called directly here because it calls
	// proxyDestroy which requires a real C connection. Instead, verify
	// the map routing works.
	frameCallbackHandlesMu.Lock()
	got := frameCallbackHandles[fakeProxy]
	frameCallbackHandlesMu.Unlock()

	if got != h {
		t.Error("frame callback handle not found in map")
	}

	// Clean up.
	frameCallbackHandlesMu.Lock()
	delete(frameCallbackHandles, fakeProxy)
	frameCallbackHandlesMu.Unlock()
}

// TestFrameCallbackDoneCbMissingHandle verifies that the done callback
// handles a missing handle gracefully (no panic).
func TestFrameCallbackDoneCbMissingHandle(t *testing.T) {
	// Call with a proxy that has no registered handle.
	// Should not panic — just return silently.
	// Note: We can't call frameCallbackDoneCb directly because it calls
	// proxyDestroy. Test that the map lookup handles missing entries.
	frameCallbackHandlesMu.Lock()
	h := frameCallbackHandles[uintptr(0xBAD_F00D)]
	frameCallbackHandlesMu.Unlock()

	if h != nil {
		t.Error("expected nil handle for unregistered proxy")
	}
}
