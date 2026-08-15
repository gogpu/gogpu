//go:build linux

package platform

import (
	"sync"
	"testing"
	"time"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
)

// newWaylandPrepareFrameFixture builds a minimal primary-window setup that
// exercises PrepareFrame scale-change detection without a live compositor.
func newWaylandPrepareFrameFixture(width, height int, outputScale, lastScale float64) (*waylandPlatformWindow, *waylandWindow) {
	wp := &waylandWindow{
		width:     width,
		height:    height,
		lastScale: lastScale,
		winID:     1,
		events:    eventqueue.New[Event](eventqueue.DefaultCapacity),
		startTime: time.Now(),
	}
	plat := &waylandPlatform{
		primary:     wp,
		outputScale: outputScale,
	}
	return &waylandPlatformWindow{platform: plat, id: 1}, wp
}

// newWaylandSecondaryPrepareFrameFixture mirrors the primary fixture on the
// secondary-window path (own state, shared scale source on the platform).
func newWaylandSecondaryPrepareFrameFixture(width, height int, outputScale, lastScale float64) (*waylandPlatformWindow, *waylandWindow) {
	state := waylandWindow{
		width:     width,
		height:    height,
		lastScale: lastScale,
		winID:     2,
		events:    eventqueue.New[Event](eventqueue.DefaultCapacity),
		startTime: time.Now(),
	}
	plat := &waylandPlatform{
		primary: &waylandWindow{
			events:    eventqueue.New[Event](eventqueue.DefaultCapacity),
			startTime: time.Now(),
		},
		outputScale: outputScale,
	}
	w := &waylandPlatformWindow{
		platform:  plat,
		id:        2,
		secondary: &secondaryWaylandConn{winID: 2, state: state},
	}
	return w, &w.secondary.state
}

func callPrepareFrameWithTimeout(t *testing.T, w *waylandPlatformWindow, timeout time.Duration) PrepareFrameResult {
	t.Helper()
	done := make(chan PrepareFrameResult, 1)
	go func() {
		done <- w.PrepareFrame()
	}()
	select {
	case r := <-done:
		return r
	case <-time.After(timeout):
		t.Fatal("PrepareFrame deadlocked on eventMu during scale change (#448); " +
			"LogicalSize must not be called while holding eventMu")
		return PrepareFrameResult{}
	}
}

func drainScaleChanged(q *eventqueue.Queue[Event]) []Event {
	var out []Event
	for {
		e, ok := q.Pop()
		if !ok {
			return out
		}
		if e.Type == EventScaleChanged {
			out = append(out, e)
		}
	}
}

// TestWaylandPrepareFrame_ScaleChangeNoDeadlock reproduces the #448 freeze:
// preferred_scale change → PrepareFrame holds eventMu → must emit
// EventScaleChanged without re-entering LogicalSize (non-reentrant mutex).
func TestWaylandPrepareFrame_ScaleChangeNoDeadlock(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 1.667, 1.0)

	r := callPrepareFrameWithTimeout(t, w, 2*time.Second)

	if !r.ScaleChanged {
		t.Fatal("ScaleChanged = false, want true after fractional scale transition")
	}
	if r.ScaleFactor != 1.667 {
		t.Fatalf("ScaleFactor = %v, want 1.667", r.ScaleFactor)
	}
	wantW, wantH := uint32(1334), uint32(1000) // round(800*1.667), round(600*1.667)
	if r.PhysicalWidth != wantW || r.PhysicalHeight != wantH {
		t.Fatalf("PhysicalSize = %dx%d, want %dx%d", r.PhysicalWidth, r.PhysicalHeight, wantW, wantH)
	}

	events := drainScaleChanged(wp.events)
	if len(events) != 1 {
		t.Fatalf("EventScaleChanged count = %d, want 1", len(events))
	}
	e := events[0]
	if e.WindowID != 1 {
		t.Errorf("WindowID = %d, want 1", e.WindowID)
	}
	if e.ScaleFactor != 1.667 {
		t.Errorf("event ScaleFactor = %v, want 1.667", e.ScaleFactor)
	}
	if e.Width != 800 || e.Height != 600 {
		t.Errorf("event size = %dx%d, want 800x600 (logical)", e.Width, e.Height)
	}
	if wp.lastScale != 1.667 {
		t.Errorf("lastScale = %v, want 1.667", wp.lastScale)
	}
}

// TestWaylandPrepareFrame_SecondaryScaleChangeNoDeadlock covers the secondary
// window path, which uses a distinct waylandWindow but the same lock pattern.
func TestWaylandPrepareFrame_SecondaryScaleChangeNoDeadlock(t *testing.T) {
	w, wp := newWaylandSecondaryPrepareFrameFixture(1024, 768, 2.0, 1.0)

	r := callPrepareFrameWithTimeout(t, w, 2*time.Second)

	if !r.ScaleChanged {
		t.Fatal("ScaleChanged = false, want true on secondary scale transition")
	}
	events := drainScaleChanged(wp.events)
	if len(events) != 1 {
		t.Fatalf("EventScaleChanged count = %d, want 1", len(events))
	}
	if events[0].WindowID != 2 {
		t.Errorf("WindowID = %d, want 2", events[0].WindowID)
	}
	if events[0].Width != 1024 || events[0].Height != 768 {
		t.Errorf("event size = %dx%d, want 1024x768", events[0].Width, events[0].Height)
	}
}

// TestWaylandPrepareFrame_FirstFrameNoScaleEvent matches ADR-059 / winit:
// lastScale==0 means "uninitialized", not a real scale change.
func TestWaylandPrepareFrame_FirstFrameNoScaleEvent(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 1.5, 0)

	r := callPrepareFrameWithTimeout(t, w, 2*time.Second)

	if r.ScaleChanged {
		t.Fatal("ScaleChanged = true on first frame, want false")
	}
	if len(drainScaleChanged(wp.events)) != 0 {
		t.Fatal("unexpected EventScaleChanged on first frame")
	}
	if wp.lastScale != 1.5 {
		t.Errorf("lastScale = %v, want 1.5 after first PrepareFrame", wp.lastScale)
	}
}

// TestWaylandPrepareFrame_UnchangedScaleNoEvent ensures steady-state frames
// do not spam EventScaleChanged (multi-display idle after transition).
func TestWaylandPrepareFrame_UnchangedScaleNoEvent(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 1.667, 1.667)

	r := callPrepareFrameWithTimeout(t, w, 2*time.Second)

	if r.ScaleChanged {
		t.Fatal("ScaleChanged = true, want false when scale is unchanged")
	}
	if len(drainScaleChanged(wp.events)) != 0 {
		t.Fatal("unexpected EventScaleChanged when scale is unchanged")
	}
}

// TestWaylandPrepareFrame_RoundTripScaleTransitions models dragging a window
// between 100% and ~166% outputs (the #448 KDE / Asahi dual-display setup).
func TestWaylandPrepareFrame_RoundTripScaleTransitions(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 1.0, 0)

	// Bootstrap lastScale.
	_ = callPrepareFrameWithTimeout(t, w, 2*time.Second)
	if len(drainScaleChanged(wp.events)) != 0 {
		t.Fatal("bootstrap must not emit ScaleChanged")
	}

	transitions := []float64{1.667, 1.0, 1.667, 1.0}
	for i, scale := range transitions {
		w.platform.mu.Lock()
		w.platform.outputScale = scale
		w.platform.mu.Unlock()

		r := callPrepareFrameWithTimeout(t, w, 2*time.Second)
		if !r.ScaleChanged {
			t.Fatalf("transition %d (→%v): ScaleChanged = false", i, scale)
		}
		if r.ScaleFactor != scale {
			t.Fatalf("transition %d: ScaleFactor = %v, want %v", i, r.ScaleFactor, scale)
		}
		events := drainScaleChanged(wp.events)
		if len(events) != 1 {
			t.Fatalf("transition %d: got %d scale events, want 1", i, len(events))
		}
		if events[0].ScaleFactor != scale {
			t.Fatalf("transition %d: event scale = %v, want %v", i, events[0].ScaleFactor, scale)
		}
	}
}

// TestWaylandPrepareFrame_ConcurrentLogicalSizeDuringScaleChange stresses the
// lock protocol: LogicalSize from another goroutine must not wedge PrepareFrame
// (and PrepareFrame must not self-deadlock while emitting ScaleChanged).
func TestWaylandPrepareFrame_ConcurrentLogicalSizeDuringScaleChange(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 1.667, 1.0)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = w.LogicalSize()
			}
		}
	}()

	r := callPrepareFrameWithTimeout(t, w, 3*time.Second)
	close(stop)
	wg.Wait()

	if !r.ScaleChanged {
		t.Fatal("ScaleChanged = false under concurrent LogicalSize")
	}
	if len(drainScaleChanged(wp.events)) != 1 {
		t.Fatal("expected exactly one EventScaleChanged under concurrency")
	}
}

// TestWaylandPrepareFrame_EnvScaleOverridesOutput verifies ScaleFactor priority
// used during PrepareFrame (env > fractional > output), matching production
// multi-display setups that pin GDK_SCALE / QT_SCALE_FACTOR.
func TestWaylandPrepareFrame_EnvScaleOverridesOutput(t *testing.T) {
	w, wp := newWaylandPrepareFrameFixture(800, 600, 1.0, 1.0)
	w.platform.envScaleFactor = 2.0

	r := callPrepareFrameWithTimeout(t, w, 2*time.Second)

	if !r.ScaleChanged || r.ScaleFactor != 2.0 {
		t.Fatalf("got ScaleChanged=%v ScaleFactor=%v, want true / 2.0", r.ScaleChanged, r.ScaleFactor)
	}
	events := drainScaleChanged(wp.events)
	if len(events) != 1 || events[0].ScaleFactor != 2.0 {
		t.Fatalf("EventScaleChanged = %+v, want scale 2.0", events)
	}
}
