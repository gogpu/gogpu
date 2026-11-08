package gogpu

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/gogpu/wgpu/hal"
)

// queuedEventManager hands out its windows in order from CreateWindow,
// recording each config it was asked for, and
// returns its events from PollEvents in order, then EventNone. Rewind makes
// the same events pending again.
type queuedEventManager struct {
	mockManager
	windows       []platform.PlatformWindow
	windowConfigs []platform.Config
	createdWindow int
	events        []platform.Event
	nextEvent     int
}

func (m *queuedEventManager) CreateWindow(config platform.Config) (platform.PlatformWindow, error) {
	m.windowConfigs = append(m.windowConfigs, config)
	if m.createdWindow == len(m.windows) {
		return nil, errors.New("queuedEventManager: no window left to create")
	}
	window := m.windows[m.createdWindow]
	m.createdWindow++
	return window, nil
}

func (m *queuedEventManager) PollEvents() platform.Event {
	if m.nextEvent == len(m.events) {
		return platform.Event{}
	}
	event := m.events[m.nextEvent]
	m.nextEvent++
	return event
}

func (m *queuedEventManager) rewind() { m.nextEvent = 0 }

// startedHostLoopWithNoWindow starts a HostLoop on the software backend over a
// platform that hands out windows, in order, to NewWindow, and requests its
// device.
func startedHostLoopWithNoWindow(t *testing.T, windows []platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *queuedEventManager) {
	t.Helper()
	app, loop, manager := hostLoopOver(t, DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware), windows, events...)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := loop.RequestDevice(0, nil); err != nil {
		loop.Stop()
		t.Fatalf("RequestDevice: %v", err)
	}
	return app, loop, manager
}

// hostLoopOver returns a HostLoop, not started, over an app with config and a
// platform that hands out windows, in order, to NewWindow. It locks the test
// to its OS thread, as HostLoop requires: on macOS a frame's autorelease pool
// must be popped on the thread that pushed it.
func hostLoopOver(t *testing.T, config Config, windows []platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *queuedEventManager) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	manager := &queuedEventManager{windows: windows, events: events}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	t.Cleanup(func() { newPlatformManagerFn = oldManager })

	app := NewApp(config)
	return app, NewHostLoop(app), manager
}

// startedHostLoop starts a HostLoop on the software backend, then opens
// window through NewWindow and shows it.
func startedHostLoop(t *testing.T, window platform.PlatformWindow, events ...platform.Event) (*App, *HostLoop, *Window) {
	t.Helper()
	app, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{window}, events...)
	opened, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	opened.Show()
	return app, loop, opened
}

// mockedRunPlatform makes the next Run open window and get a renderer that
// has no GPU: its surface is sized to the window and never configured, so an
// acquire is attempted and fails.
func mockedRunPlatform(t *testing.T, window platform.PlatformWindow) {
	t.Helper()
	manager := &queuedEventManager{windows: []platform.PlatformWindow{window}}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	oldRenderer := newRendererFn
	newRendererFn = func(platformWindow platform.PlatformWindow, _ types.GraphicsAPI, _ bool, _ bool, _ deviceRequest) (*Renderer, error) {
		renderer := &Renderer{}
		width, height := platformWindow.PhysicalSize()
		renderer.primary = &RenderTarget{
			renderer:   renderer,
			platWindow: platformWindow,
			width:      uint32(width),
			height:     uint32(height),
		}
		return renderer, nil
	}
	t.Cleanup(func() {
		newPlatformManagerFn = oldManager
		newRendererFn = oldRenderer
	})
}

// enabledStats turns frame stats on for the test: a failed acquire records a
// skipped present, which is how a test sees that BeginFrame tried to acquire.
func enabledStats(t *testing.T) {
	t.Helper()
	wasEnabled := StatsEnabled()
	EnableStats()
	t.Cleanup(func() {
		if !wasEnabled {
			DisableStats()
		}
	})
}

// skipWhenRaceInstrumented skips an allocation count in a build instrumented
// by github.com/kolkov/racedetector, the race detector CI runs: the calls it
// adds to the code allocate, so the count no longer measures the code. The
// tool runs go test with GONOSUMCHECK naming its own module, and the test
// binary inherits that environment.
func skipWhenRaceInstrumented(t *testing.T) {
	t.Helper()
	if strings.Contains(os.Getenv("GONOSUMCHECK"), "github.com/kolkov/racedetector") {
		t.Skip("allocation counts include racedetector's instrumentation")
	}
}

func TestHostLoop_StartTwice(t *testing.T) {
	_, loop, _ := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if err := loop.Start(); !errors.Is(err, ErrHostLoopStarted) {
		t.Errorf("second Start: got %v, want ErrHostLoopStarted", err)
	}
}

func TestHostLoop_RunRefusesWhileStarted(t *testing.T) {
	app, loop, _ := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if err := app.Run(); !errors.Is(err, ErrHostLoopStarted) {
		t.Errorf("Run: got %v, want ErrHostLoopStarted", err)
	}
}

func TestHostLoop_StartOpensNoWindowAndRequestsNoDevice(t *testing.T) {
	app, loop, manager := hostLoopOver(t, DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware), nil)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer loop.Stop()

	if manager.createdWindow != 0 {
		t.Errorf("Start created %d platform windows, want 0", manager.createdWindow)
	}
	if count := app.WindowCount(); count != 0 {
		t.Errorf("WindowCount after Start: got %d, want 0", count)
	}
	if !loop.Running() {
		t.Error("Running after Start with no window: got false, want true")
	}
	if app.GPUContextProvider().Adapter().IsNil() {
		t.Error("Start brought up no adapter")
	}
	if app.renderer.device != nil {
		t.Error("Start brought up a device before RequestDevice")
	}
}

func TestHostLoop_RequestDeviceBringsUpTheDevice(t *testing.T) {
	app, loop, _ := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if app.renderer.device == nil {
		t.Error("RequestDevice brought up no device")
	}
}

func TestHostLoop_RequestDeviceBeforeStartFails(t *testing.T) {
	loop := NewHostLoop(NewApp(DefaultConfig()))
	if err := loop.RequestDevice(0, nil); err == nil {
		t.Error("RequestDevice before Start: got nil error")
	}
}

func TestHostLoop_RequestDeviceTwiceFails(t *testing.T) {
	_, loop, _ := startedHostLoopWithNoWindow(t, nil)
	defer loop.Stop()

	if err := loop.RequestDevice(0, nil); err == nil {
		t.Error("second RequestDevice: got nil error")
	}
}

func TestHostLoop_NewWindowBeforeRequestDeviceFails(t *testing.T) {
	_, loop, manager := hostLoopOver(t, DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware), []platform.PlatformWindow{&mockWindow{width: 800, height: 600}})
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer loop.Stop()

	if _, err := loop.NewWindow(DefaultConfig()); err == nil {
		t.Error("NewWindow before RequestDevice: got nil error")
	}
	if manager.createdWindow != 0 {
		t.Errorf("NewWindow before RequestDevice created %d platform windows, want 0", manager.createdWindow)
	}
}

func TestHostLoop_StartRequestsTheFallbackAdapter(t *testing.T) {
	config := DefaultConfig().WithGraphicsAPI(types.GraphicsAPIAuto)
	config.ForceFallbackAdapter = true
	app, loop, _ := hostLoopOver(t, config, nil)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer loop.Stop()

	if got := app.renderer.adapter.Info().DeviceType; got != gputypes.DeviceTypeCPU {
		t.Errorf("adapter type with ForceFallbackAdapter: got %v, want %v", got, gputypes.DeviceTypeCPU)
	}
}

func TestHostLoop_RequestDeviceKeepsTheRequiredLimits(t *testing.T) {
	limits := gputypes.DefaultLimits()
	limits.MaxTextureDimension2D = 1024
	app, loop, _ := hostLoopOver(t, DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware), nil)
	if err := loop.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer loop.Stop()
	if err := loop.RequestDevice(0, &limits); err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}

	device := app.renderer.device
	if got := device.Limits(); got != limits {
		t.Errorf("device limits: got %+v, want %+v", got, limits)
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: 2048, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding,
	})
	if err == nil {
		texture.Release()
		t.Error("CreateTexture beyond the required MaxTextureDimension2D: got no error")
	}
}

func TestHostLoop_RequestDeviceFailsNamingWhatTheAdapterLacks(t *testing.T) {
	beyondBindGroups := gputypes.DefaultLimits()
	beyondBindGroups.MaxBindGroups = 1 << 20
	belowAlignment := gputypes.DefaultLimits()
	belowAlignment.MinUniformBufferOffsetAlignment = 1
	tests := []struct {
		name     string
		features gputypes.Features
		limits   *gputypes.Limits
		want     []string
	}{
		{"feature", gputypes.Features(gputypes.FeatureTimestampQuery), nil, []string{"TimestampQuery"}},
		{"maximum beyond", 0, &beyondBindGroups, []string{"MaxBindGroups"}},
		{"alignment below", 0, &belowAlignment, []string{"MinUniformBufferOffsetAlignment"}},
		{"feature and limit", gputypes.Features(gputypes.FeatureTimestampQuery), &beyondBindGroups, []string{"TimestampQuery", "MaxBindGroups"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, loop, _ := hostLoopOver(t, DefaultConfig().WithGraphicsAPI(types.GraphicsAPISoftware), nil)
			if err := loop.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer loop.Stop()
			err := loop.RequestDevice(test.features, test.limits)
			if err == nil {
				t.Fatal("RequestDevice: got no error")
			}
			for _, name := range test.want {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("RequestDevice error %q does not name %s", err, name)
				}
			}
			if app.renderer.device != nil {
				t.Error("a refused RequestDevice brought up a device")
			}
		})
	}
}

func TestHostLoop_NewWindowBeforeStartFails(t *testing.T) {
	loop := NewHostLoop(NewApp(DefaultConfig()))
	if _, err := loop.NewWindow(DefaultConfig()); err == nil {
		t.Error("NewWindow before Start: got nil error")
	}
}

func TestHostLoop_NewWindowOpensHidden(t *testing.T) {
	platformWindow := &mockWindow{width: 800, height: 600}
	_, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{platformWindow})
	defer loop.Stop()

	window, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if platformWindow.visible || window.Visible() {
		t.Error("NewWindow showed the window")
	}
	if _, ok, err := loop.BeginFrame(window); ok || err != nil {
		t.Errorf("BeginFrame before Show: got ok=%v, error %v; want false, nil", ok, err)
	}
	window.Show()
	if !platformWindow.visible || !window.Visible() {
		t.Error("Show left the window hidden")
	}
}

// macOS and Windows apply a window's fullscreen themselves, from the
// platform config NewWindow passes on.
func TestHostLoop_NewWindowAsksThePlatformForFullscreen(t *testing.T) {
	_, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{&mockWindow{width: 800, height: 600}})
	defer loop.Stop()

	if _, err := loop.NewWindow(DefaultConfig().WithFullscreen()); err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if len(manager.windowConfigs) != 1 || !manager.windowConfigs[0].Fullscreen {
		t.Errorf("platform configs: got %+v, want one asking fullscreen", manager.windowConfigs)
	}
}

func TestHostLoop_NewWindowPresentsWithConfigVSync(t *testing.T) {
	_, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{
		&mockWindow{windowID: 1, width: 800, height: 600},
		&mockWindow{windowID: 2, width: 800, height: 600},
	})
	defer loop.Stop()

	withVSync, err := loop.NewWindow(DefaultConfig().WithVSync(true))
	if err != nil {
		t.Fatalf("NewWindow with vsync: %v", err)
	}
	withoutVSync, err := loop.NewWindow(DefaultConfig().WithVSync(false))
	if err != nil {
		t.Fatalf("NewWindow without vsync: %v", err)
	}
	if !withVSync.surface.vsync || withoutVSync.surface.vsync {
		t.Errorf("surface vsync: got %v and %v, want true and false", withVSync.surface.vsync, withoutVSync.surface.vsync)
	}
}

func TestHostLoop_SeveralWindowsEachDrawAFrame(t *testing.T) {
	app, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{
		&mockWindow{windowID: 1, width: 800, height: 600},
		&mockWindow{windowID: 2, width: 640, height: 480},
		&mockWindow{windowID: 3, width: 320, height: 240},
	})
	defer loop.Stop()

	windows := make([]*Window, 0, 3)
	for range 3 {
		window, err := loop.NewWindow(DefaultConfig())
		if err != nil {
			t.Fatalf("NewWindow: %v", err)
		}
		window.Show()
		windows = append(windows, window)
	}
	if count := app.WindowCount(); count != 3 {
		t.Errorf("WindowCount: got %d, want 3", count)
	}
	for i, window := range windows {
		context, ok, err := loop.BeginFrame(window)
		if !ok {
			t.Fatalf("BeginFrame on window %d: got ok=false, error %v", i, err)
		}
		wantWidth, wantHeight := window.PhysicalSize()
		if width, height := context.FramebufferSize(); width != wantWidth || height != wantHeight {
			t.Errorf("window %d framebuffer: got %dx%d, want %dx%d", i, width, height, wantWidth, wantHeight)
		}
		context.Clear(0.1, 0.2, 0.3, 1)
		if err := loop.EndFrame(context); err != nil {
			t.Errorf("EndFrame on window %d: %v", i, err)
		}
	}
}

func TestHostLoop_StopClosesOpenWindows(t *testing.T) {
	var closedWindows []WindowID
	app, loop, _ := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{
		&mockWindow{windowID: 1, width: 800, height: 600},
		&mockWindow{windowID: 2, width: 800, height: 600},
	})
	app.OnAnyWindowClosed(func(window WindowID) { closedWindows = append(closedWindows, window) })
	first, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	second, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}

	loop.Stop()

	if count := app.WindowCount(); count != 0 {
		t.Errorf("WindowCount after Stop: got %d, want 0", count)
	}
	if first.surface.surface != nil || second.surface.surface != nil {
		t.Error("Stop left a window's surface unreleased")
	}
	if len(closedWindows) != 2 {
		t.Errorf("windows reported closed by Stop: got %v, want both", closedWindows)
	}
}

func TestHostLoop_RunningAfterStart(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	if !loop.Running() {
		t.Error("Running after Start: got false, want true")
	}
}

func TestHostLoop_NotRunningAfterQuit(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	app.Quit()
	if loop.Running() {
		t.Error("Running after Quit: got true, want false")
	}
}

func TestHostLoop_NotRunningAfterLastWindowCloses(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{windowID: 1, width: 800, height: 600},
		platform.Event{Type: platform.EventClose, WindowID: 1})

	loop.PumpEvents()
	if loop.Running() {
		t.Error("Running after the last window closed: got true, want false")
	}
	loop.Stop() // the close destroyed the window; Stop must not panic
}

func TestHostLoop_StopBeforeStartDoesNothing(t *testing.T) {
	loop := NewHostLoop(NewApp(DefaultConfig()))

	loop.Stop() // nothing was created; Stop must not panic
}

func TestHostLoop_StopAfterFailedStartDoesNothing(t *testing.T) {
	manager := &queuedEventManager{}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	errNoGPU := errors.New("no GPU")
	oldRenderer := newWindowlessRendererFn
	newWindowlessRendererFn = func(types.GraphicsAPI, deviceRequest) (*Renderer, error) {
		return nil, errNoGPU
	}
	t.Cleanup(func() {
		newPlatformManagerFn = oldManager
		newWindowlessRendererFn = oldRenderer
	})

	loop := NewHostLoop(NewApp(DefaultConfig()))
	if err := loop.Start(); !errors.Is(err, errNoGPU) {
		t.Fatalf("Start without a renderer: got %v, want %v", err, errNoGPU)
	}
	loop.Stop() // Start destroyed what it created; Stop must not panic
}

func TestHostLoop_StopTwiceTearsDownOnce(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	closeCount := 0
	app.OnClose(func() { closeCount++ })

	loop.Stop()
	loop.Stop()
	if closeCount != 1 {
		t.Errorf("onClose after two Stops: called %d times, want 1", closeCount)
	}
}

// windowlessManager starts, then fails to create any window, and counts how
// often it is destroyed.
type windowlessManager struct {
	mockManager
	destroyCount int
}

var errNoWindow = errors.New("no window")

func (m *windowlessManager) CreateWindow(platform.Config) (platform.PlatformWindow, error) {
	return nil, errNoWindow
}

func (m *windowlessManager) Destroy() { m.destroyCount++ }

func TestRun_DestroysPlatformWhenWindowFails(t *testing.T) {
	manager := &windowlessManager{}
	oldManager := newPlatformManagerFn
	newPlatformManagerFn = func() platform.PlatformManager { return manager }
	t.Cleanup(func() { newPlatformManagerFn = oldManager })

	if err := NewApp(DefaultConfig()).Run(); !errors.Is(err, errNoWindow) {
		t.Fatalf("Run without a window: got %v, want %v", err, errNoWindow)
	}
	if manager.destroyCount != 1 {
		t.Errorf("platform after a Run whose window failed: destroyed %d times, want 1", manager.destroyCount)
	}
}

// liveResizeWindow records the live-resize hooks installed on it.
type liveResizeWindow struct {
	mockWindow
	resizeHook     func()
	phaseStartHook func()
	phaseEndHook   func()
}

func (w *liveResizeWindow) SetLiveResizeHook(hook func()) { w.resizeHook = hook }

func (w *liveResizeWindow) SetLiveResizePhaseHooks(start, end func()) {
	w.phaseStartHook = start
	w.phaseEndHook = end
}

func TestHostLoop_NewWindowInstallsNoLiveResizeHooks(t *testing.T) {
	window := &liveResizeWindow{mockWindow: mockWindow{width: 800, height: 600}}
	_, loop, _ := startedHostLoop(t, window)
	defer loop.Stop()

	if window.resizeHook != nil {
		t.Error("live-resize hook after NewWindow: installed, want none, so nothing draws inside PumpEvents")
	}
	if window.phaseStartHook != nil || window.phaseEndHook != nil {
		t.Error("live-resize phase hooks after NewWindow: installed, want none")
	}
}

func TestRun_InstallsLiveResizeHooks(t *testing.T) {
	window := &liveResizeWindow{mockWindow: mockWindow{width: 800, height: 600}}
	mockedRunPlatform(t, window)
	app := NewApp(DefaultConfig())
	app.OnUpdate(func(float64) { app.Quit() })

	if err := app.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if window.resizeHook == nil {
		t.Error("live-resize hook after Run: none, want Run's, which draws during a macOS live resize")
	}
	if window.phaseStartHook == nil || window.phaseEndHook == nil {
		t.Error("live-resize phase hooks after Run: none, want Run's")
	}
}

func TestHostLoop_BeginFrameAcquires(t *testing.T) {
	_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	context, ok, err := loop.BeginFrame(window)
	if !ok || err != nil {
		t.Fatalf("BeginFrame on a shown window: got ok=%v, error %v; want true, nil", ok, err)
	}
	if width, height := context.FramebufferSize(); width != 800 || height != 600 {
		t.Errorf("framebuffer: got %dx%d, want 800x600", width, height)
	}
	if err := loop.EndFrame(context); err != nil {
		t.Errorf("EndFrame: %v", err)
	}
}

// frameErrors are errors a driver answers at acquire or present: those wgpu
// types, the device's loss wrapped as a backend wraps it, and one wgpu does
// not type.
var frameErrors = []error{
	wgpu.ErrSurfaceOutdated,
	wgpu.ErrSurfaceLost,
	wgpu.ErrTimeout,
	fmt.Errorf("vulkan: %w", wgpu.ErrDeviceLost),
	errors.New("driver: no frame"),
}

// failedAcquires makes every acquire for the rest of the test fail with err,
// acquiring nothing.
func failedAcquires(t *testing.T, err error) {
	t.Helper()
	acquire := acquireSurfaceTextureFn
	acquireSurfaceTextureFn = func(*wgpu.Surface) (*wgpu.SurfaceTexture, bool, error) { return nil, false, err }
	t.Cleanup(func() { acquireSurfaceTextureFn = acquire })
}

// failedPresents makes every present for the rest of the test answer err
// once the frame went to the surface, as a driver answers a frame it took.
func failedPresents(t *testing.T, err error) {
	t.Helper()
	present := presentSurfaceTextureFn
	presentSurfaceTextureFn = func(surface *wgpu.Surface, texture *wgpu.SurfaceTexture, damage []image.Rectangle) error {
		if presentErr := present(surface, texture, damage); presentErr != nil {
			return presentErr
		}
		return err
	}
	t.Cleanup(func() { presentSurfaceTextureFn = present })
}

func TestHostLoop_BeginFrameAnswersTheAcquireError(t *testing.T) {
	for _, acquireErr := range frameErrors {
		t.Run(acquireErr.Error(), func(t *testing.T) {
			_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
			defer loop.Stop()
			failedAcquires(t, acquireErr)
			records := errorRecords(t)

			context, ok, err := loop.BeginFrame(window)
			if ok || context != nil {
				t.Fatal("BeginFrame with a failed acquire: got ok=true, want false")
			}
			if !errors.Is(err, acquireErr) {
				t.Errorf("BeginFrame with a failed acquire: got error %v, want %v", err, acquireErr)
			}
			if records.Len() != 0 {
				t.Errorf("BeginFrame logged the error it answered: %s", records.String())
			}
		})
	}
}

func TestHostLoop_EndFrameAnswersThePresentError(t *testing.T) {
	for _, presentErr := range frameErrors {
		t.Run(presentErr.Error(), func(t *testing.T) {
			_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
			defer loop.Stop()
			failedPresents(t, presentErr)
			records := errorRecords(t)

			context, ok, err := loop.BeginFrame(window)
			if !ok {
				t.Fatalf("BeginFrame: got ok=false, error %v", err)
			}
			context.Clear(0.1, 0.2, 0.3, 1)
			if err := loop.EndFrame(context); !errors.Is(err, presentErr) {
				t.Errorf("EndFrame with a failed present: got error %v, want %v", err, presentErr)
			}
			if records.Len() != 0 {
				t.Errorf("EndFrame logged the error it answered: %s", records.String())
			}
		})
	}
}

// configureErrors are errors a surface's configure answers: those wgpu types,
// the device's loss wrapped as a backend wraps it, and one wgpu does not type.
var configureErrors = []error{
	wgpu.ErrReleased,
	wgpu.ErrSurfaceLost,
	fmt.Errorf("vulkan: %w", wgpu.ErrDeviceLost),
	errors.New("driver: no swapchain"),
}

// failedConfigures makes every surface configure for the rest of the test
// fail with err, configuring nothing.
func failedConfigures(t *testing.T, err error) {
	t.Helper()
	configure := configureSurfaceFn
	configureSurfaceFn = func(*wgpu.Surface, *wgpu.Device, *wgpu.SurfaceConfiguration) error { return err }
	t.Cleanup(func() { configureSurfaceFn = configure })
}

// preparingWindow is a mockWindow whose PrepareFrame reports prepared, and
// no size until it is set, so a size it reports is seen by beginFrame alone,
// and a resize by BeginFrame's sync alone.
type preparingWindow struct {
	mockWindow
	prepared platform.PrepareFrameResult
}

func (w *preparingWindow) PrepareFrame() platform.PrepareFrameResult { return w.prepared }

// beginFrameReconfigures are the ways BeginFrame configures an open window's
// surface again: the window's size changed, preparing the frame reports a new
// size, or an acquire failed. Each makes the next BeginFrame configure the
// surface of a window opened through preparingWindow at 800x600.
var beginFrameReconfigures = []struct {
	name        string
	reconfigure func(*testing.T, *preparingWindow)
}{
	{"resize", func(_ *testing.T, window *preparingWindow) {
		window.width = 1024
		window.height = 768
	}},
	{"prepared size", func(_ *testing.T, window *preparingWindow) {
		window.prepared = platform.PrepareFrameResult{PhysicalWidth: 1024, PhysicalHeight: 768}
	}},
	{"acquire recovery", func(t *testing.T, _ *preparingWindow) {
		failedAcquires(t, errors.New("driver: no frame"))
	}},
}

// beginFrameReconfiguredWith runs the next BeginFrame on a window opened at
// 800x600, after reconfigure, with every configure failing with configureErr,
// and checks that the surface keeps that size. It answers whether BeginFrame
// started a frame, which it ends, what it logged at error level, and its
// error.
func beginFrameReconfiguredWith(t *testing.T, reconfigure func(*testing.T, *preparingWindow), configureErr error) (bool, string, error) {
	t.Helper()
	platformWindow := &preparingWindow{mockWindow: mockWindow{width: 800, height: 600}}
	_, loop, window := startedHostLoop(t, platformWindow)
	t.Cleanup(loop.Stop)
	reconfigure(t, platformWindow)
	failedConfigures(t, configureErr)
	records := errorRecords(t)
	context, ok, err := loop.BeginFrame(window)
	if width, height := window.surface.width, window.surface.height; width != 800 || height != 600 {
		t.Errorf("surface after a failed configure: got %dx%d, want its previous 800x600", width, height)
	}
	if ok {
		if endErr := loop.EndFrame(context); endErr != nil {
			t.Errorf("EndFrame: %v", endErr)
		}
	}
	return ok, records.String(), err
}

// Each configure's error is answered, not logged.
func TestHostLoop_BeginFrameAnswersTheConfigureError(t *testing.T) {
	for _, way := range beginFrameReconfigures {
		for _, configureErr := range configureErrors {
			t.Run(way.name+"/"+configureErr.Error(), func(t *testing.T) {
				ok, records, err := beginFrameReconfiguredWith(t, way.reconfigure, configureErr)
				if ok {
					t.Fatal("BeginFrame with a failed configure: got ok=true, want false")
				}
				if !errors.Is(err, configureErr) {
					t.Errorf("BeginFrame with a failed configure: got error %v, want %v", err, configureErr)
				}
				if records != "" {
					t.Errorf("BeginFrame logged the error it answered: %s", records)
				}
			})
		}
	}
}

// A configure that fails because the surface has zero area is a minimize,
// not a failure: BeginFrame answers no error of it, whichever way it
// configured, and logs none.
func TestHostLoop_BeginFrameAnswersNoZeroAreaConfigure(t *testing.T) {
	for _, way := range beginFrameReconfigures {
		t.Run(way.name, func(t *testing.T) {
			_, records, err := beginFrameReconfiguredWith(t, way.reconfigure, hal.ErrZeroArea)
			if errors.Is(err, hal.ErrZeroArea) {
				t.Errorf("BeginFrame answered a zero-area configure: %v", err)
			}
			if records != "" {
				t.Errorf("BeginFrame logged a zero-area configure: %s", records)
			}
		})
	}
}

// A minimized Vulkan window on Windows keeps reporting its size while its
// surface's extent drops to zero: the present at the minimize answers
// outdated, and no configure succeeds until the restore. BeginFrame skips the
// window meanwhile, with no error.
func TestHostLoop_BeginFrameSkipsASurfaceOfZeroArea(t *testing.T) {
	_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()
	failedPresents(t, wgpu.ErrSurfaceOutdated)
	failedConfigures(t, hal.ErrZeroArea)
	context, ok, err := loop.BeginFrame(window)
	if !ok {
		t.Fatalf("BeginFrame: got ok=false, error %v", err)
	}
	context.Clear(0.1, 0.2, 0.3, 1)
	if err := loop.EndFrame(context); !errors.Is(err, wgpu.ErrSurfaceOutdated) {
		t.Fatalf("EndFrame at the minimize: got error %v, want %v", err, wgpu.ErrSurfaceOutdated)
	}
	records := errorRecords(t)

	if _, ok, err := loop.BeginFrame(window); ok || err != nil {
		t.Errorf("BeginFrame on a surface of zero area: got ok=%v, error %v; want false, nil", ok, err)
	}
	if records.Len() != 0 {
		t.Errorf("BeginFrame on a surface of zero area logged: %s", records.String())
	}
}

// A resize reaches a window's surface at that window's next BeginFrame, not
// in the pump, which only reports it. Two windows resized in one pump each
// take their own size.
func TestHostLoop_ResizeAppliedAtTheWindowsBeginFrame(t *testing.T) {
	platformWindows := []*mockWindow{
		{windowID: 1, width: 800, height: 600},
		{windowID: 2, width: 640, height: 480},
	}
	_, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{platformWindows[0], platformWindows[1]})
	defer loop.Stop()
	windows := make([]*Window, 0, len(platformWindows))
	for range platformWindows {
		window, err := loop.NewWindow(DefaultConfig())
		if err != nil {
			t.Fatalf("NewWindow: %v", err)
		}
		window.Show()
		windows = append(windows, window)
	}
	oldSizes := []image.Point{image.Pt(800, 600), image.Pt(640, 480)}
	newSizes := []image.Point{image.Pt(1024, 768), image.Pt(320, 240)}
	for i, size := range newSizes {
		platformWindows[i].width = size.X
		platformWindows[i].height = size.Y
		manager.events = append(manager.events, platform.Event{
			Type:     platform.EventResize,
			WindowID: platformWindows[i].windowID,
			Width:    size.X, Height: size.Y,
			PhysicalWidth: size.X, PhysicalHeight: size.Y,
		})
	}
	surfaceSize := func(window *Window) image.Point {
		return image.Pt(int(window.surface.width), int(window.surface.height))
	}

	loop.PumpEvents()

	for i, window := range windows {
		if size := surfaceSize(window); size != oldSizes[i] {
			t.Errorf("window %d's surface after the pump: got %v, want %v until its BeginFrame", i, size, oldSizes[i])
		}
	}
	for i, window := range windows {
		context, ok, err := loop.BeginFrame(window)
		if !ok {
			t.Fatalf("BeginFrame on window %d: got ok=false, error %v", i, err)
		}
		if width, height := context.FramebufferSize(); image.Pt(width, height) != newSizes[i] {
			t.Errorf("window %d's framebuffer: got %dx%d, want %v", i, width, height, newSizes[i])
		}
		context.Clear(0.1, 0.2, 0.3, 1)
		if err := loop.EndFrame(context); err != nil {
			t.Errorf("EndFrame on window %d: %v", i, err)
		}
		for j := i + 1; j < len(windows); j++ {
			if size := surfaceSize(windows[j]); size != oldSizes[j] {
				t.Errorf("window %d's surface after window %d's frame: got %v, want %v until its own BeginFrame", j, i, size, oldSizes[j])
			}
		}
	}
}

func TestHostLoop_BeginFrameSkipsHiddenWindow(t *testing.T) {
	enabledStats(t)
	app, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	window.Hide()
	if _, ok, err := loop.BeginFrame(window); ok || err != nil {
		t.Errorf("BeginFrame on a hidden window: got ok=%v, error %v; want false, nil", ok, err)
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame on a hidden window attempted an acquire")
	}
}

func TestHostLoop_BeginFrameSkipsZeroSizeWindow(t *testing.T) {
	enabledStats(t)
	app, loop, window := startedHostLoop(t, &mockWindow{width: 0, height: 0})
	defer loop.Stop()

	if _, ok, err := loop.BeginFrame(window); ok || err != nil {
		t.Errorf("BeginFrame on a zero-size window: got ok=%v, error %v; want false, nil", ok, err)
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame on a zero-size window attempted an acquire")
	}
}

func TestHostLoop_BeginFrameSkipsClosedFrameGate(t *testing.T) {
	enabledStats(t)
	app, loop, window := startedHostLoop(t, &mockFrameGaterWindow{mockWindow: mockWindow{width: 800, height: 600}, ready: false})
	defer loop.Stop()

	if _, ok, err := loop.BeginFrame(window); ok || err != nil {
		t.Errorf("BeginFrame with the frame gate closed: got ok=%v, error %v; want false, nil", ok, err)
	}
	if app.renderer.GetFrameStats().PresentSkipped {
		t.Error("BeginFrame with the frame gate closed attempted an acquire")
	}
}

func TestHostLoop_PumpEventsDispatches(t *testing.T) {
	app, loop, window := startedHostLoop(t, &mockWindow{windowID: 1, width: 800, height: 600},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventResize, WindowID: 1, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768},
	)
	defer loop.Stop()

	var pressedKey, releasedKey gpucontext.Key
	var resizeWidth, resizeHeight int
	app.EventSource().OnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) { pressedKey = key })
	app.EventSource().OnKeyRelease(func(key gpucontext.Key, _ gpucontext.Modifiers) { releasedKey = key })
	window.SetOnResize(func(width, height int) { resizeWidth, resizeHeight = width, height })

	loop.PumpEvents()

	if pressedKey != gpucontext.KeyA {
		t.Errorf("key press: got %v, want KeyA", pressedKey)
	}
	if releasedKey != gpucontext.KeyA {
		t.Errorf("key release: got %v, want KeyA", releasedKey)
	}
	if resizeWidth != 1024 || resizeHeight != 768 {
		t.Errorf("resize: got %dx%d, want 1024x768", resizeWidth, resizeHeight)
	}
	var queuedEventCount int
	for {
		if _, ok := app.PollInputEvent(); !ok {
			break
		}
		queuedEventCount++
	}
	if queuedEventCount != 3 {
		t.Errorf("input event queue: got %d events, want 3", queuedEventCount)
	}
}

func TestHostLoop_FrameAcquiresAndPresents(t *testing.T) {
	app, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	for frame := range 2 {
		context, ok, err := loop.BeginFrame(window)
		if !ok || err != nil {
			t.Fatalf("BeginFrame %d on the software backend: got ok=%v, error %v; want true, nil", frame, ok, err)
		}
		context.Clear(0.1, 0.2, 0.3, 1)
		if err := loop.EndFrame(context); err != nil {
			t.Errorf("EndFrame %d: %v", frame, err)
		}
		if app.renderer.currentSurface != nil {
			t.Errorf("renderer's current surface after EndFrame %d: set, want none", frame)
		}
	}
}

func TestHostLoop_BeginFrameReconfiguresLostSurface(t *testing.T) {
	_, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	surface := window.surface
	surface.state = SurfaceLost
	context, ok, err := loop.BeginFrame(window)
	if !ok || err != nil {
		t.Fatalf("BeginFrame on a lost surface: got ok=%v, error %v; want true, nil after the reconfigure", ok, err)
	}
	if err := loop.EndFrame(context); err != nil {
		t.Errorf("EndFrame: %v", err)
	}
	if !surface.CanRender() {
		t.Error("surface after BeginFrame on a lost one: cannot render, want it reconfigured")
	}
}

func TestHostLoop_ConsumeRedrawRequestBeforeStart(t *testing.T) {
	app := NewApp(DefaultConfig())
	loop := NewHostLoop(app)

	app.RequestRedraw()
	if loop.ConsumeRedrawRequest() {
		t.Error("ConsumeRedrawRequest before Start: got true, want false")
	}
}

func TestHostLoop_ConsumeRedrawRequestReportsEachRequestOnce(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	if !loop.ConsumeRedrawRequest() {
		t.Error("first call after Start: got false, want true for the first frame")
	}
	if loop.ConsumeRedrawRequest() {
		t.Error("call with no request since the last: got true, want false")
	}
	app.RequestRedraw()
	app.RequestRedraw()
	if !loop.ConsumeRedrawRequest() {
		t.Error("call after two RequestRedraw: got false, want true")
	}
	if loop.ConsumeRedrawRequest() {
		t.Error("second call after two RequestRedraw: got true, want false, the requests coalesce")
	}
}

func TestHostLoop_ConsumeRedrawRequestAfterPumpedResize(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{windowID: 7, width: 800, height: 600},
		platform.Event{Type: platform.EventResize, WindowID: 7, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768})
	defer loop.Stop()

	loop.ConsumeRedrawRequest() // Start's first frame
	loop.PumpEvents()
	if !loop.ConsumeRedrawRequest() {
		t.Error("call after a pumped resize: got false, want true")
	}
}

func TestHostLoop_PumpEventsAllocatesNothingWhenIdle(t *testing.T) {
	_, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600})
	defer loop.Stop()

	skipWhenRaceInstrumented(t)
	allocations := testing.AllocsPerRun(100, loop.PumpEvents)
	if allocations != 0 {
		t.Errorf("PumpEvents with no events: %v allocations per pump, want 0", allocations)
	}
}

// Each event entering the input-event queue is boxed into a
// gpucontext.InputEvent, one allocation, as in Run. Nothing else allocates.
func TestHostLoop_PumpEventsAllocatesOnlyQueuedInputEvents(t *testing.T) {
	_, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{&mockWindow{width: 800, height: 600}},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA},
		platform.Event{Type: platform.EventResize, Width: 1024, Height: 768, PhysicalWidth: 1024, PhysicalHeight: 768},
	)
	defer loop.Stop()
	window, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	window.Show()

	skipWhenRaceInstrumented(t)
	allocations := testing.AllocsPerRun(100, func() {
		manager.rewind()
		loop.PumpEvents()
	})
	if allocations != float64(len(manager.events)) {
		t.Errorf("PumpEvents with %d events: %v allocations per pump, want %d", len(manager.events), allocations, len(manager.events))
	}
}

// Events routed to a window, with callbacks on the window and on the event
// source, allocate as unrouted ones do: each is boxed once into the
// input-event queue, apart from a character under 256, which the runtime
// boxes without allocating. A typed character outside ASCII also allocates
// its text, once for both callbacks; an ASCII one does not.
func TestHostLoop_PumpEventsAllocatesOncePerRoutedEvent(t *testing.T) {
	app, loop, manager := startedHostLoopWithNoWindow(t, []platform.PlatformWindow{&mockWindow{windowID: 1, width: 800, height: 600}},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventChar, Char: 'a', WindowID: 1},
		platform.Event{Type: platform.EventKeyUp, Key: gpucontext.KeyA, WindowID: 1},
		platform.Event{Type: platform.EventPointerMove, WindowID: 1},
		platform.Event{Type: platform.EventPointerDown, WindowID: 1},
		platform.Event{Type: platform.EventPointerUp, WindowID: 1},
		platform.Event{Type: platform.EventScroll, WindowID: 1},
		platform.Event{Type: platform.EventChar, Char: 'ж', WindowID: 1},
	)
	defer loop.Stop()
	window, err := loop.NewWindow(DefaultConfig())
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	window.Show()
	var keys, pointers, scrolls int
	var text string
	window.SetOnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) { keys++ })
	window.SetOnTextInput(func(typed string) { text = typed })
	window.SetOnPointer(func(gpucontext.PointerEvent) { pointers++ })
	window.SetOnScroll(func(gpucontext.ScrollEvent) { scrolls++ })
	app.EventSource().OnTextInput(func(typed string) { text = typed })

	skipWhenRaceInstrumented(t)
	allocations := testing.AllocsPerRun(100, func() {
		manager.rewind()
		loop.PumpEvents()
	})
	// One boxing for each of the 8 events but 'a', and one text for 'ж'.
	const want = 8
	if allocations != want {
		t.Errorf("PumpEvents with %d routed events: %v allocations per pump, want %d", len(manager.events), allocations, want)
	}
	if keys == 0 || pointers == 0 || scrolls == 0 || text != "ж" {
		t.Errorf("callbacks: %d keys, %d pointer events, %d scrolls, text %q; want each reached and the text \"ж\"", keys, pointers, scrolls, text)
	}
}

// Run's frame pumps through the same step as PumpEvents, which clears the
// previous pump's edges before applying new events: onUpdate sees a key
// press as JustPressed on the frame it arrived, and not on the next.
func TestRunFrame_OnUpdateSeesEachInputEdgeOnce(t *testing.T) {
	app, loop, _ := startedHostLoop(t, &mockWindow{width: 800, height: 600},
		platform.Event{Type: platform.EventKeyDown, Key: gpucontext.KeyA})
	defer loop.Stop()

	var justPressed []bool
	app.OnUpdate(func(float64) {
		justPressed = append(justPressed, app.Input().Keyboard().JustPressed(input.KeyA))
	})

	app.runFrame()
	app.runFrame()

	if len(justPressed) != 2 || !justPressed[0] || justPressed[1] {
		t.Errorf("JustPressed(KeyA) in onUpdate over two frames: got %v, want [true false]", justPressed)
	}
}

// Run's loop has no caller to give a frame's error to, so it logs it: an
// outdated surface at debug level, any other at error level.
func TestRunFrame_LogsTheFrameErrors(t *testing.T) {
	tests := []struct {
		name  string
		fail  func(*testing.T, error)
		err   error
		level slog.Level
	}{
		{"acquire outdated", failedAcquires, wgpu.ErrSurfaceOutdated, slog.LevelDebug},
		{"acquire lost", failedAcquires, wgpu.ErrSurfaceLost, slog.LevelError},
		{"present outdated", failedPresents, wgpu.ErrSurfaceOutdated, slog.LevelDebug},
		{"present device lost", failedPresents, fmt.Errorf("vulkan: %w", wgpu.ErrDeviceLost), slog.LevelError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, loop, window := startedHostLoop(t, &mockWindow{width: 800, height: 600})
			defer loop.Stop()
			test.fail(t, test.err)
			records := recordsFrom(t, slog.LevelDebug)

			app.renderFrameGPU([]windowFrame{{
				window: window,
				onDraw: func(context *Context) { context.Clear(0.1, 0.2, 0.3, 1) },
				scale:  1,
				physW:  800,
				physH:  600,
			}})

			logged := false
			for _, record := range strings.Split(records.String(), "\n") {
				logged = logged || strings.Contains(record, "level="+test.level.String()) && strings.Contains(record, test.err.Error())
			}
			if !logged {
				t.Errorf("Run's frame did not log %q at %v; logged:\n%s", test.err, test.level, records.String())
			}
		})
	}
}

// Run's loop treats a surface it cannot configure at the window's size as a
// failed acquire: it logs the error at error level, skips the window's draw
// and asks for another frame.
func TestRunFrame_LogsTheConfigureErrorAndSkipsTheWindow(t *testing.T) {
	platformWindow := &mockWindow{width: 800, height: 600}
	app, loop, window := startedHostLoop(t, platformWindow)
	defer loop.Stop()
	configureErr := fmt.Errorf("vulkan: %w", wgpu.ErrDeviceLost)
	failedConfigures(t, configureErr)
	platformWindow.width = 1024
	platformWindow.height = 768
	records := errorRecords(t)
	app.invalidator.Consume()

	drawn := false
	app.renderFrameGPU([]windowFrame{{
		window: window,
		onDraw: func(*Context) { drawn = true },
		scale:  1,
		physW:  1024,
		physH:  768,
	}})

	if !strings.Contains(records.String(), configureErr.Error()) {
		t.Errorf("Run's frame did not log %q at error level; logged:\n%s", configureErr, records.String())
	}
	if drawn {
		t.Error("Run's frame drew a window whose surface it could not configure")
	}
	if !app.invalidator.Consume() {
		t.Error("Run's frame did not ask for another frame after a failed configure")
	}
}
