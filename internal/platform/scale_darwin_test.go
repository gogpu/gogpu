//go:build darwin

package platform

import "testing"

func TestScaleTransition(t *testing.T) {
	tests := []struct {
		name         string
		last, cur    float64
		changed, syn bool
	}{
		{name: "2x to 1x", last: 2, cur: 1, changed: true, syn: true},
		{name: "1x to 2x", last: 1, cur: 2, changed: true, syn: true},
		{name: "unchanged", last: 2, cur: 2},
		{name: "first observation syncs without a change event", last: 0, cur: 2, syn: true},
		{name: "unknown current scale is ignored", last: 2, cur: 0},
		{name: "unknown before and now", last: 0, cur: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed, syn := scaleTransition(tt.last, tt.cur)
			if changed != tt.changed || syn != tt.syn {
				t.Fatalf("scaleTransition(%v, %v) = (%v, %v), want (%v, %v)",
					tt.last, tt.cur, changed, syn, tt.changed, tt.syn)
			}
		})
	}
}

// TestPrepareFrameNoWindow guards the nil paths PrepareFrame takes before
// the window and surface exist.
func TestPrepareFrameNoWindow(t *testing.T) {
	dw := &darwinPlatformWindow{}
	if got := dw.PrepareFrame(); got.ScaleFactor != 1.0 || got.ScaleChanged {
		t.Fatalf("PrepareFrame without window = %+v, want scale 1, unchanged", got)
	}
}
