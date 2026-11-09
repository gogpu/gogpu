//go:build darwin

package platform

import "testing"

func TestDarwinPlatformWindow_DestroyPassesThePrimaryOn(t *testing.T) {
	first := &darwinWindow{id: 1}
	second := &darwinWindow{id: 2}
	p := &darwinPlatform{primary: first, windows: []*darwinWindow{first, second}}

	(&darwinPlatformWindow{platform: p, id: first.id}).Destroy()
	if p.primary != second || len(p.windows) != 1 {
		t.Fatalf("after destroying the first window: second is primary %v, %d windows; want true, 1", p.primary == second, len(p.windows))
	}

	(&darwinPlatformWindow{platform: p, id: second.id}).Destroy()
	if p.primary != nil || len(p.windows) != 0 {
		t.Fatalf("after destroying both windows: a primary left %v, %d windows; want false, 0", p.primary != nil, len(p.windows))
	}
	p.WaitEvents() // with no window left, nothing is routed and nothing panics
}
