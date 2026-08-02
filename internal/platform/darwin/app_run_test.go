//go:build darwin

package darwin_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gogpu/gogpu"
)

func runAppOnce(t *testing.T, cfg gogpu.Config, setup func(app *gogpu.App), expectPanic bool) {
	t.Helper()
	runOnMainThread(t, func() {
		fmt.Fprintln(os.Stderr, "starting", t.Name())

		app := gogpu.NewApp(cfg)
		if setup != nil {
			setup(app)
		}

		time.AfterFunc(2*time.Second, func() {
			app.Quit()
		})

		if expectPanic {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic, got none")
				}
			}()
		}

		if err := app.Run(); err != nil {
			t.Fatalf("App.Run failed: %v", err)
		}
	})
}

// TestDarwinAppRunSmoke runs the real app/renderer loop on the Go backend.
// This mirrors the production call stack without invoking DrawTriangle (to avoid
// shader compilation failures) while still exercising BeginFrame/EndFrame.
func TestDarwinAppRunSmoke(t *testing.T) {
	// ContinuousRender so OnUpdate advances without waiting for input —
	// default event-driven mode idles in WaitEvents after the first frame.
	cfg := gogpu.DefaultConfig().
		WithTitle("gogpu").
		WithSize(640, 480).
		WithBackend(gogpu.BackendGo).
		WithContinuousRender(true)

	frames := 0
	runAppOnce(t, cfg,
		func(app *gogpu.App) {
			app.OnUpdate(func(dt float64) {
				frames++
				if frames >= 30 {
					app.Quit()
				}
			})
			app.OnDraw(func(ctx *gogpu.Context) {
				ctx.Clear(0, 0, 0, 1)
			})
		},
		false,
	)
	if frames == 0 {
		t.Fatalf("App.Run completed with zero updates")
	}
}

// This isolates teardown without any draw callback or panic.
// If this crashes, the platform teardown alone is unsafe.
func TestDarwinAppRunNoDraw(t *testing.T) {
	cfg := gogpu.DefaultConfig().
		WithTitle("gogpu").
		WithSize(640, 480).
		WithBackend(gogpu.BackendGo).
		WithContinuousRender(true)

	frames := 0
	runAppOnce(t, cfg,
		func(app *gogpu.App) {
			app.OnUpdate(func(dt float64) {
				frames++
				if frames >= 15 {
					app.Quit()
				}
			})
		},
		false,
	)
	if frames == 0 {
		t.Fatalf("App.Run completed with zero updates")
	}
}

// This isolates panic during update (before any draw).
// If this crashes, panic+teardown alone is unsafe.
func TestDarwinAppRunPanicOnUpdate(t *testing.T) {
	cfg := gogpu.DefaultConfig().
		WithTitle("gogpu").
		WithSize(640, 480).
		WithBackend(gogpu.BackendGo).
		WithContinuousRender(true)

	runAppOnce(t, cfg,
		func(app *gogpu.App) {
			app.OnUpdate(func(dt float64) {
				panic("forced update panic")
			})
		},
		true,
	)
}

// This isolates panic during draw without calling DrawTriangle.
// If this crashes, the panic+renderer teardown path is unsafe
// even without shader compilation.
func TestDarwinAppRunPanicOnDrawNoTriangle(t *testing.T) {
	cfg := gogpu.DefaultConfig().
		WithTitle("gogpu").
		WithSize(640, 480).
		WithBackend(gogpu.BackendGo).
		WithContinuousRender(true)

	runAppOnce(t, cfg,
		func(app *gogpu.App) {
			app.OnUpdate(func(dt float64) {})
			app.OnDraw(func(ctx *gogpu.Context) {
				panic("forced draw panic")
			})
		},
		true,
	)
}

// TestDarwinAppRunPanicPath mirrors the triangle example: it calls DrawTriangle
// and panics on error. This exercises the panic/unwind/destroy path that has
// been associated with the intermittent crash.
func TestDarwinAppRunPanicPath(t *testing.T) {
	t.Skip("Test only needed if segfaults are observed during panic")
	cfg := gogpu.DefaultConfig().
		WithTitle("gogpu").
		WithSize(640, 480).
		WithBackend(gogpu.BackendGo).
		WithContinuousRender(true)

	const iterations = 20
	for i := 0; i < iterations; i++ {
		runAppOnce(t, cfg,
			func(app *gogpu.App) {
				app.OnUpdate(func(dt float64) {})
				app.OnDraw(func(ctx *gogpu.Context) {
					// Mirror production path (DrawTriangle) and always force the
					// panic/unwind path regardless of shader compile success.
					_ = ctx.DrawTriangle(0, 0, 0, 1)
					panic("forced DrawTriangle panic")
					// t.Fatalf("expected panic, got none")
				})
			},
			true,
		)
	}
}

// TestDarwinAppQuitUnblocksIdle verifies Quit from a background timer exits
// App.Run while the event-driven loop is blocked in WaitEvents — without
// requiring a mouse/keyboard event (jbunds report on wgpu#294 / gogpu macOS).
func TestDarwinAppQuitUnblocksIdle(t *testing.T) {
	cfg := gogpu.DefaultConfig().
		WithTitle("gogpu").
		WithSize(640, 480).
		WithBackend(gogpu.BackendGo)
	// ContinuousRender left false: after the first frame the loop idles.

	done := make(chan struct{})
	runOnMainThread(t, func() {
		defer close(done)
		app := gogpu.NewApp(cfg)
		var quitOnce sync.Once
		app.OnDraw(func(ctx *gogpu.Context) {
			ctx.Clear(0, 0, 0, 1)
			// Schedule Quit after the first frame so wakeupFn is published
			// and the loop is about to block in WaitEvents.
			quitOnce.Do(func() {
				time.AfterFunc(200*time.Millisecond, func() {
					app.Quit()
				})
			})
		})
		if err := app.Run(); err != nil {
			t.Fatalf("App.Run failed: %v", err)
		}
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("App.Run did not exit after Quit — WaitEvents was not woken")
	}
}
