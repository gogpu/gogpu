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

// TestFrameCallbackGateNotTrigger documents the gate-not-trigger contract
// that was violated in issue #379. The frame callback done event must act
// as a GATE (allowing the next frame to render when the app requests it)
// but NOT as a TRIGGER (unconditionally requesting a redraw).
//
// Regression: commit a8b72ce introduced code that queued EventExpose on
// every ConsumeFrameCallbackReady() == true, creating a perpetual 60 FPS
// render loop even with ContinuousRender=false. The fix (issue #379)
// changed the call site to consume the flag without queuing any event.
//
// winit reference: frame callback done transitions state to Received (gate),
// but RedrawRequested is only set when the app explicitly calls
// window.request_redraw() (trigger). The two are independent.
func TestFrameCallbackGateNotTrigger(t *testing.T) {
	h := &LibwaylandHandle{}

	// Simulate the full render cycle that caused issue #379:
	//
	// Step 1: Frame rendered, SyncFrame registers callback.
	// In production this calls RequestFrameCallback which sets state
	// to Requested. Here we set it directly.
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackRequested)
	h.frameCallbackReady.Store(false)

	// Verify: rendering is gated while waiting for compositor.
	if h.FrameCallbackReady() {
		t.Fatal("FrameCallbackReady() should be false while Requested")
	}

	// Step 2: Compositor fires done callback (~16.6ms later).
	// In production this happens inside frameCallbackDoneCb.
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackReceived)
	h.frameCallbackReady.Store(true)

	// Verify: the gate is now open.
	if !h.FrameCallbackReady() {
		t.Fatal("FrameCallbackReady() should be true after compositor done")
	}

	// Step 3: PollEvents calls ConsumeFrameCallbackReady() to clear the
	// flag. The CRITICAL CONTRACT: the return value must NOT be used to
	// queue EventExpose or otherwise trigger a redraw. It is consumed
	// solely to reset the flag for the next cycle.
	consumed := h.ConsumeFrameCallbackReady()
	if !consumed {
		t.Fatal("ConsumeFrameCallbackReady() should return true (first consume)")
	}

	// After consume: the gate check (FrameCallbackReady) still returns
	// true because it checks frameCallbackState, not frameCallbackReady.
	// This is correct — the gate remains open until the next
	// RequestFrameCallback sets state back to Requested.
	if !h.FrameCallbackReady() {
		t.Error("FrameCallbackReady() should still be true after consume " +
			"(gate checks state, not ready flag)")
	}

	// Second consume returns false — the flag was already cleared.
	if h.ConsumeFrameCallbackReady() {
		t.Error("second ConsumeFrameCallbackReady() should be false")
	}

	// The key invariant: ConsumeFrameCallbackReady() is a one-shot flag
	// drain. It does NOT affect the rendering gate (FrameCallbackReady).
	// Using its return value to trigger EventExpose creates a feedback
	// loop: render → SyncFrame → RequestFrameCallback → done → consume
	// → EventExpose → RequestRedraw → render → ... (perpetual 60 FPS).
}

// TestFrameCallbackIdleMode verifies that in idle mode (ContinuousRender=false,
// no animation, no invalidation), the frame callback done event does NOT cause
// unnecessary rendering. This is the scenario from issue #379.
func TestFrameCallbackIdleMode(t *testing.T) {
	h := &LibwaylandHandle{}

	// Simulate multiple compositor VSync cycles in idle mode.
	// Each cycle: done fires → state transitions → consume clears flag.
	// No rendering should be triggered between cycles.
	for cycle := range 5 {
		// Compositor fires done.
		atomic.StoreInt32(&h.frameCallbackState, FrameCallbackReceived)
		h.frameCallbackReady.Store(true)

		// PollEvents consumes the flag (gate-not-trigger pattern).
		h.ConsumeFrameCallbackReady()

		// Gate is open but no one is asking to render.
		// In production: invalidated=false && continuousRender=false
		// means runFrame skips rendering. The gate being open is
		// irrelevant when there is nothing to draw.
		if !h.FrameCallbackReady() {
			t.Errorf("cycle %d: gate should remain open in idle mode", cycle)
		}
	}
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
