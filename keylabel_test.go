package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// labelingManager is a platform manager whose layout prints labels.
type labelingManager struct {
	mockManager
	labels map[gpucontext.Key]rune
}

func (m *labelingManager) KeyLabel(key gpucontext.Key) rune { return m.labels[key] }

func TestKeyLabelAnswersWhatTheLayoutPrints(t *testing.T) {
	app := &App{manager: &labelingManager{labels: map[gpucontext.Key]rune{
		gpucontext.KeyQ:      'a',
		gpucontext.Key2:      'é',
		gpucontext.KeyEnter:  '\r',
		gpucontext.KeyEscape: 0x1b,
	}}}

	for key, want := range map[gpucontext.Key]rune{
		gpucontext.KeyQ:      'a',
		gpucontext.Key2:      'é',
		gpucontext.KeyEnter:  0,
		gpucontext.KeyEscape: 0,
		gpucontext.KeyF1:     0,
	} {
		if got := app.KeyLabel(key); got != want {
			t.Errorf("KeyLabel(%v) = %q, want %q", key, got, want)
		}
	}
}

func TestKeyLabelIsZeroWhereThePlatformCannotTell(t *testing.T) {
	for name, app := range map[string]*App{
		"a platform without labels": {manager: &mockManager{}},
		"no platform":               {},
	} {
		if got := app.KeyLabel(gpucontext.KeyQ); got != 0 {
			t.Errorf("with %s, KeyLabel(KeyQ) = %q, want 0", name, got)
		}
	}
}

func TestALayoutChangeReachesTheCallback(t *testing.T) {
	app := &App{windowManager: newWindowManager(), renderLoop: &mockRenderLoop{}}
	app.classifyEvent(&platform.Event{Type: platform.EventKeyboardLayoutChanged}, nil, nil)

	changes := 0
	if got := app.OnKeyboardLayoutChanged(func() { changes++ }); got != app {
		t.Error("OnKeyboardLayoutChanged does not answer the App")
	}
	app.classifyEvent(&platform.Event{Type: platform.EventKeyboardLayoutChanged}, nil, nil)

	if changes != 1 {
		t.Errorf("one layout change called the callback %d times, want once", changes)
	}
}
