//go:build linux

package wayland

import (
	"slices"
	"testing"
)

func TestConstraintEventsReachTheInputCallbacks(t *testing.T) {
	var events []string
	handle := &LibwaylandHandle{inputCallbacks: &InputCallbacks{
		OnLockedPointerLocked:       func() { events = append(events, "locked") },
		OnLockedPointerUnlocked:     func() { events = append(events, "unlocked") },
		OnConfinedPointerConfined:   func() { events = append(events, "confined") },
		OnConfinedPointerUnconfined: func() { events = append(events, "unconfined") },
	}}
	previous := inputCallbackHandle
	inputCallbackHandle = handle
	t.Cleanup(func() { inputCallbackHandle = previous })

	lockedPointerLockedCb(0, 0)
	lockedPointerUnlockedCb(0, 0)
	confinedPointerConfinedCb(0, 0)
	confinedPointerUnconfinedCb(0, 0)

	want := []string{"locked", "unlocked", "confined", "unconfined"}
	if !slices.Equal(events, want) {
		t.Errorf("callbacks reached: %v, want %v", events, want)
	}
}
