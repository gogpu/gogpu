//go:build linux

package wayland

import (
	"slices"
	"testing"
)

func TestWheelNotchesReachTheInputCallbacks(t *testing.T) {
	type notches struct {
		axis  uint32
		count float64
	}
	var reached []notches
	handle := &LibwaylandHandle{inputCallbacks: &InputCallbacks{
		OnPointerAxisNotches: func(axis uint32, count float64) {
			reached = append(reached, notches{axis, count})
		},
	}}
	previous := inputCallbackHandle
	inputCallbackHandle = handle
	t.Cleanup(func() { inputCallbackHandle = previous })

	minusOne := int32(-1)
	inputPointerAxisDiscreteCb(0, 0, uintptr(PointerAxisVerticalScroll), uintptr(uint32(minusOne)))
	inputPointerAxisValue120Cb(0, 0, uintptr(PointerAxisVerticalScroll), 120)
	inputPointerAxisValue120Cb(0, 0, uintptr(PointerAxisHorizontalScroll), 30)

	want := []notches{
		{PointerAxisVerticalScroll, -1},
		{PointerAxisVerticalScroll, 1},
		{PointerAxisHorizontalScroll, 0.25},
	}
	if !slices.Equal(reached, want) {
		t.Errorf("notches reached: %v, want %v", reached, want)
	}
}
