//go:build darwin

package platform

import (
	"math"
	"reflect"
	"testing"

	"github.com/gogpu/gogpu/internal/platform/eventqueue"
)

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
		{name: "NaN current scale is ignored", last: 2, cur: math.NaN()},
		{name: "+Inf current scale is ignored", last: 2, cur: math.Inf(1)},
		{name: "-Inf current scale is ignored", last: 2, cur: math.Inf(-1)},
		{name: "first NaN current scale is ignored", last: 0, cur: math.NaN()},
		{name: "valid scale after NaN last", last: math.NaN(), cur: 2, changed: true, syn: true},
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

// fakeMetrics feeds a darwinWindow a scripted sequence of window metrics.
type fakeMetrics struct {
	m  windowMetrics
	ok bool
}

func (f *fakeMetrics) read() (windowMetrics, bool) { return f.m, f.ok }

func newTestDarwinWindow(id WindowID, f *fakeMetrics) *darwinWindow {
	w := &darwinWindow{id: id, events: eventqueue.New[Event](eventqueue.DefaultCapacity)}
	w.metrics = f.read
	return w
}

// drainEvents pops every queued event of w.
func drainEvents(w *darwinWindow) []Event {
	var events []Event
	for {
		e, ok := w.events.Pop()
		if !ok {
			return events
		}
		events = append(events, e)
	}
}

func eventTypes(events []Event) []EventType {
	if len(events) == 0 {
		return nil
	}
	types := make([]EventType, len(events))
	for i := range events {
		types[i] = events[i].Type
	}
	return types
}

// TestDarwinWindowScaleEvents drives a window through a sequence of size and
// scale observations along both event-loop paths and checks which events are
// queued, in which order (ADR-059: ScaleChanged before Resize).
func TestDarwinWindowScaleEvents(t *testing.T) {
	retina := windowMetrics{Width: 800, Height: 600, PhysW: 1600, PhysH: 1200, Scale: 2}
	plain := windowMetrics{Width: 800, Height: 600, PhysW: 800, PhysH: 600, Scale: 1}
	bigRetina := windowMetrics{Width: 1000, Height: 700, PhysW: 2000, PhysH: 1400, Scale: 2}
	withScale := func(m windowMetrics, s float64) windowMetrics { m.Scale = s; return m }

	type step struct {
		m         windowMetrics
		ok        bool
		want      []EventType
		lastScale float64 // window's lastScale after the step
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{name: "first observation: resize only", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
		}},
		{name: "2x to 1x: scale changed, then resize", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
			{m: plain, ok: true, want: []EventType{EventScaleChanged, EventResize}, lastScale: 1},
		}},
		{name: "1x to 2x: scale changed, then resize", steps: []step{
			{m: plain, ok: true, want: []EventType{EventResize}, lastScale: 1},
			{m: retina, ok: true, want: []EventType{EventScaleChanged, EventResize}, lastScale: 2},
		}},
		{name: "scale only, size unchanged: scale changed only", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
			{m: withScale(retina, 1), ok: true, want: []EventType{EventScaleChanged}, lastScale: 1},
		}},
		{name: "size only: resize only", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
			{m: bigRetina, ok: true, want: []EventType{EventResize}, lastScale: 2},
		}},
		{name: "unchanged: nothing", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
			{m: retina, ok: true, lastScale: 2},
		}},
		{name: "non-finite scale: no scale event, last scale kept", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
			{m: withScale(retina, math.NaN()), ok: true, lastScale: 2},
			{m: withScale(retina, math.Inf(1)), ok: true, lastScale: 2},
		}},
		{name: "live resize or no window: nothing, then caught up", steps: []step{
			{m: retina, ok: true, want: []EventType{EventResize}, lastScale: 2},
			{m: plain, ok: false, lastScale: 2},
			{m: plain, ok: true, want: []EventType{EventScaleChanged, EventResize}, lastScale: 1},
		}},
	}

	paths := []struct {
		name string
		run  func(w *darwinWindow) []Event
	}{
		{name: "pollEvents", run: func(w *darwinWindow) []Event {
			var events []Event
			for e := w.pollEvents(nil); e.Type != EventNone; e = w.pollEvents(nil) {
				events = append(events, e)
			}
			return events
		}},
		{name: "checkResize", run: func(w *darwinWindow) []Event {
			w.eventMu.Lock()
			w.checkResize()
			w.eventMu.Unlock()
			return drainEvents(w)
		}},
	}

	for _, path := range paths {
		for _, tt := range tests {
			t.Run(path.name+"/"+tt.name, func(t *testing.T) {
				f := &fakeMetrics{}
				w := newTestDarwinWindow(3, f)
				for i, st := range tt.steps {
					f.m, f.ok = st.m, st.ok
					events := path.run(w)
					if got := eventTypes(events); !reflect.DeepEqual(got, st.want) {
						t.Fatalf("step %d: events = %v, want %v", i, got, st.want)
					}
					if w.lastScale != st.lastScale {
						t.Fatalf("step %d: lastScale = %v, want %v", i, w.lastScale, st.lastScale)
					}
					for _, e := range events {
						checkScaleEventPayload(t, e, 3, st.m)
					}
				}
			})
		}
	}
}

func checkScaleEventPayload(t *testing.T, e Event, id WindowID, m windowMetrics) {
	t.Helper()
	if e.WindowID != id {
		t.Errorf("%v WindowID = %d, want %d", e.Type, e.WindowID, id)
	}
	switch e.Type {
	case EventScaleChanged:
		if e.ScaleFactor != m.Scale || e.Width != m.Width || e.Height != m.Height {
			t.Errorf("ScaleChanged = %+v, want scale %v size %dx%d", e, m.Scale, m.Width, m.Height)
		}
	case EventResize:
		if e.Width != m.Width || e.Height != m.Height || e.PhysicalWidth != m.PhysW || e.PhysicalHeight != m.PhysH {
			t.Errorf("Resize = %+v, want %+v", e, m)
		}
	}
}

// TestDarwinNonKeyWindowScaleChangeDelivered: a window that is not key moves
// to a display with another scale while the loop idles. The app's sequence,
// WaitEvents followed by draining PollEvents (App.runFrame →
// processEventsMultiThread), must deliver that window's ScaleChanged and
// Resize, in order and with its WindowID.
func TestDarwinNonKeyWindowScaleChangeDelivered(t *testing.T) {
	retina := windowMetrics{Width: 800, Height: 600, PhysW: 1600, PhysH: 1200, Scale: 2}
	plain := windowMetrics{Width: 800, Height: 600, PhysW: 800, PhysH: 600, Scale: 1}

	f1 := &fakeMetrics{m: retina, ok: true}
	f2 := &fakeMetrics{m: retina, ok: true}
	w1 := newTestDarwinWindow(1, f1)
	w2 := newTestDarwinWindow(2, f2)
	p := &darwinPlatform{windows: []*darwinWindow{w1, w2}, primary: w1}

	runFrame := func() []Event {
		p.WaitEvents() // app == nil: no Cocoa wait, only the resize check
		var got []Event
		for e := p.PollEvents(); e.Type != EventNone; e = p.PollEvents() {
			got = append(got, e)
		}
		return got
	}

	runFrame()   // first observation of both windows
	f2.m = plain // only the secondary, non-key window changes display
	got := runFrame()

	if types := eventTypes(got); !reflect.DeepEqual(types, []EventType{EventScaleChanged, EventResize}) {
		t.Fatalf("events = %v, want [ScaleChanged Resize]", types)
	}
	for _, e := range got {
		checkScaleEventPayload(t, e, 2, plain)
	}
}

// recordingScaler records the contentsScale values PrepareFrame applies.
type recordingScaler struct{ scales []float64 }

func (r *recordingScaler) SetContentsScale(scale float64) { r.scales = append(r.scales, scale) }

// TestPrepareFrameSyncsContentsScale guards the fix itself: when the backing
// scale changes, the frame preparation on the render thread must push the
// new scale to the layer (once, not every frame) before the drawable is
// reconfigured, and report ScaleChanged.
func TestPrepareFrameSyncsContentsScale(t *testing.T) {
	rec := &recordingScaler{}
	dw := &darwinPlatformWindow{surface: rec}

	steps := []struct {
		scale        float64
		physW, physH int
		wantChanged  bool
		wantScales   []float64 // cumulative SetContentsScale calls
	}{
		{scale: 2, physW: 1600, physH: 1200, wantScales: []float64{2}},                     // first frame
		{scale: 2, physW: 1600, physH: 1200, wantScales: []float64{2}},                     // unchanged: no call
		{scale: 1, physW: 800, physH: 600, wantChanged: true, wantScales: []float64{2, 1}}, // 2x -> 1x
		{scale: math.NaN(), physW: 800, physH: 600, wantScales: []float64{2, 1}},           // ignored
		{scale: 2, physW: 1600, physH: 1200, wantChanged: true, wantScales: []float64{2, 1, 2}},
	}
	for i, st := range steps {
		got := dw.prepareFrame(st.scale, st.physW, st.physH)
		if got.ScaleChanged != st.wantChanged {
			t.Fatalf("step %d: ScaleChanged = %v, want %v", i, got.ScaleChanged, st.wantChanged)
		}
		if got.PhysicalWidth != uint32(st.physW) || got.PhysicalHeight != uint32(st.physH) {
			t.Fatalf("step %d: physical = %dx%d, want %dx%d", i, got.PhysicalWidth, got.PhysicalHeight, st.physW, st.physH)
		}
		if !reflect.DeepEqual(rec.scales, st.wantScales) {
			t.Fatalf("step %d: SetContentsScale calls = %v, want %v", i, rec.scales, st.wantScales)
		}
	}

	// No surface (Metal surface creation failed): no panic.
	(&darwinPlatformWindow{}).prepareFrame(2, 1600, 1200)
}
