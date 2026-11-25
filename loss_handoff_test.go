package gogpu

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/gogpu/wgpu"
)

// --- loss handoff (P2-API-009) ---

func TestLossHandoffSurfaceEpisodeDeliveredOnce(t *testing.T) {
	r := &Renderer{}
	var wakes int
	r.wake = func() { wakes++ }

	r.recordSurfaceLost(errors.New("backend says lost"))

	surface, device := r.ConsumeLoss()
	if surface == nil || device != nil {
		t.Fatalf("ConsumeLoss() = %v, %v; want a surface error and no device error", surface, device)
	}
	if !errors.Is(surface, ErrSurfaceLost) {
		t.Errorf("surface error %v does not match ErrSurfaceLost", surface)
	}
	if errors.Is(surface, ErrDeviceLost) {
		t.Error("a surface error must not match ErrDeviceLost")
	}
	if s, d := r.ConsumeLoss(); s != nil || d != nil {
		t.Errorf("second ConsumeLoss() = %v, %v; want nothing new", s, d)
	}
	if wakes != 1 {
		t.Errorf("wake called %d times, want 1", wakes)
	}
}

func TestLossHandoffRearmsOnNextEpisode(t *testing.T) {
	r := &Renderer{}
	r.recordSurfaceLost(errors.New("first"))
	if s, _ := r.ConsumeLoss(); s == nil {
		t.Fatal("first episode was not delivered")
	}

	// The re-arm itself is the producer-side transition back to configured; the
	// next loss is what the consumer sees, with no cross-side reset.
	r.recordSurfaceLost(errors.New("second"))
	s, _ := r.ConsumeLoss()
	if s == nil {
		t.Fatal("second episode was not delivered")
	}
}

func TestLossHandoffDeviceSubsumesSurface(t *testing.T) {
	r := &Renderer{}
	r.recordSurfaceLost(errors.New("surface"))
	r.recordDeviceLost(errors.New("device"))

	surface, device := r.ConsumeLoss()
	if surface != nil {
		t.Errorf("surface error = %v; device loss must subsume a pending surface episode", surface)
	}
	if device == nil || !errors.Is(device, ErrDeviceLost) {
		t.Fatalf("device error = %v; want one matching ErrDeviceLost", device)
	}

	// Terminal: delivered once, and later surface losses are ignored.
	if s, d := r.ConsumeLoss(); s != nil || d != nil {
		t.Errorf("device loss delivered twice: %v, %v", s, d)
	}
	r.recordSurfaceLost(errors.New("late"))
	if s, d := r.ConsumeLoss(); s != nil || d != nil {
		t.Errorf("surface loss after device loss delivered: %v, %v", s, d)
	}
}

func TestSurfaceLossEscalatesToDeviceLoss(t *testing.T) {
	r := &Renderer{}
	ws := &RenderTarget{renderer: r}

	for i := 0; i < maxSurfaceRecoverAttempts; i++ {
		ws.recoverFromAcquireError(wgpu.ErrSurfaceLost, nil, nil)
	}

	surface, device := r.ConsumeLoss()
	if device == nil || !errors.Is(device, ErrDeviceLost) {
		t.Fatalf("after %d failed recoveries: device = %v; want one matching ErrDeviceLost", maxSurfaceRecoverAttempts, device)
	}
	if surface != nil {
		t.Errorf("surface error = %v; the escalated device loss must subsume it", surface)
	}

	// A successful configure resets the counter, so one new loss after recovery
	// is a plain surface episode again.
	r2 := &Renderer{}
	ws2 := &RenderTarget{renderer: r2}
	ws2.recoverFromAcquireError(wgpu.ErrSurfaceLost, nil, nil)
	ws2.lostFails = 0 // recorded by configure() on success
	ws2.recoverFromAcquireError(wgpu.ErrSurfaceLost, nil, nil)
	if s, d := r2.ConsumeLoss(); s == nil || d != nil {
		t.Errorf("after a successful recovery: %v, %v; want a surface episode, no device loss", s, d)
	}
}

func TestLossHandoffConcurrentProduceConsume(t *testing.T) {
	// Run under -race (WSL Ubuntu, go1.27.4): concurrent producer (render
	// thread) and consumer (event loop) on the atomics-only handoff, including
	// the terminal device path.
	r := &Renderer{}
	r.wake = func() {}

	const episodes = 200
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < episodes; i++ {
			r.recordSurfaceLost(fmt.Errorf("episode %d", i))
		}
		r.recordDeviceLost(errors.New("terminal"))
	}()

	var sawDevice bool
	for {
		s, d := r.ConsumeLoss()
		if s != nil && !errors.Is(s, ErrSurfaceLost) {
			t.Fatalf("surface error %v does not match ErrSurfaceLost", s)
		}
		if d != nil {
			if !errors.Is(d, ErrDeviceLost) {
				t.Fatalf("device error %v does not match ErrDeviceLost", d)
			}
			sawDevice = true
		}
		select {
		case <-done:
			// Producer finished: drain everything still pending.
			for {
				s2, d2 := r.ConsumeLoss()
				if s2 == nil && d2 == nil {
					break
				}
				if s2 != nil && !errors.Is(s2, ErrSurfaceLost) {
					t.Fatalf("surface error %v does not match ErrSurfaceLost", s2)
				}
				if d2 != nil {
					if !errors.Is(d2, ErrDeviceLost) {
						t.Fatalf("device error %v does not match ErrDeviceLost", d2)
					}
					sawDevice = true
				}
			}
			wg.Wait()
			if !sawDevice {
				t.Fatal("terminal device loss was never delivered")
			}
			return
		default:
			runtime.Gosched()
		}
	}
}

func TestDrawingErrorMatchesSurfaceLost(t *testing.T) {
	r := &Renderer{}
	ws := &RenderTarget{renderer: r, state: SurfaceLost}

	err := r.drawTexturedQuad(ws, &Texture{}, DrawTextureOptions{})
	if err == nil {
		t.Fatal("drawTexturedQuad on a lost surface returned nil")
	}
	if !errors.Is(err, ErrSurfaceLost) {
		t.Errorf("draw error %v does not match ErrSurfaceLost", err)
	}

	// A surface that is merely not configured yet keeps its plain error: the
	// recovered-from state is what carries the sentinel, not every missing frame.
	ws2 := &RenderTarget{renderer: r, state: SurfaceNone}
	err = r.drawTexturedQuad(ws2, &Texture{}, DrawTextureOptions{})
	if err == nil {
		t.Fatal("drawTexturedQuad without a surface returned nil")
	}
	if errors.Is(err, ErrSurfaceLost) {
		t.Errorf("a never-configured surface must not match ErrSurfaceLost: %v", err)
	}
}

func TestAppDeliversLossCallbacks(t *testing.T) {
	app := &App{}
	app.renderer = &Renderer{}
	var gotSurface, gotDevice error
	app.OnSurfaceLost(func(err error) { gotSurface = err })
	app.OnDeviceLost(func(err error) { gotDevice = err })

	app.renderer.recordSurfaceLost(errors.New("surface cause"))
	app.deliverLossCallbacks()
	if !errors.Is(gotSurface, ErrSurfaceLost) {
		t.Fatalf("surface callback got %v", gotSurface)
	}
	if gotDevice != nil || app.deviceLostErr != nil {
		t.Fatalf("device callback fired early: %v / %v", gotDevice, app.deviceLostErr)
	}

	app.renderer.recordDeviceLost(errors.New("device cause"))
	app.deliverLossCallbacks()
	if !errors.Is(gotDevice, ErrDeviceLost) {
		t.Fatalf("device callback got %v", gotDevice)
	}
	if !errors.Is(app.deviceLostErr, ErrDeviceLost) {
		t.Fatalf("Run would return %v", app.deviceLostErr)
	}
}

func TestPreRunErrorsMatchNotInitialized(t *testing.T) {
	app := &App{}
	if _, err := app.NewWindow(DefaultConfig()); !errors.Is(err, ErrNotInitialized) {
		t.Errorf("NewWindow before Run = %v; want ErrNotInitialized", err)
	}
	if err := app.StartDrag(DragData{}, nil); !errors.Is(err, ErrNotInitialized) {
		t.Errorf("StartDrag before Run = %v; want ErrNotInitialized", err)
	}
}
