// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package gogpu

import (
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/gogpu/gogpu/internal/platform"
)

// HostLoop lets a host that owns its loop drive an App one step at a time,
// instead of handing the thread to App.Run:
//
//	loop := gogpu.NewHostLoop(app)
//	if err := loop.Start(); err != nil {
//		return err
//	}
//	window, err := loop.NewWindow(gogpu.DefaultConfig())
//	if err != nil {
//		return err
//	}
//	window.Show()
//	for loop.Running() {
//		loop.PumpEvents()
//		if context, ok := loop.BeginFrame(window); ok {
//			context.Clear(0.1, 0.2, 0.3, 1)
//			loop.EndFrame(context)
//		}
//	}
//	loop.Stop()
//
// Threads: every call must run on the thread that called Start, which must be
// the OS main thread, locked with runtime.LockOSThread by the caller. This is
// documented, not checked, as for Run. gogpu runs no render thread for a
// HostLoop: acquire and present happen on the caller's thread.
//
// Windows: no modal timer runs, so during a move or resize the frame freezes
// and resumes at the new size when the user lets go.
//
// macOS: each frame runs inside an autorelease pool that BeginFrame pushes and
// EndFrame pops.
type HostLoop struct {
	app       *App
	framePool uintptr // macOS autorelease pool open between BeginFrame and EndFrame
}

// NewHostLoop returns a loop over app. Callbacks and windows stay on app.
func NewHostLoop(app *App) *HostLoop {
	return &HostLoop{app: app}
}

// Start brings up the platform and the GPU instance, adapter and device, with
// no window and no surface, and marks the app running, then returns without
// blocking. Windows open through NewWindow. When the platform or the GPU
// cannot start, it returns the error and destroys what it had created. While
// a HostLoop is started, Run and a second Start return ErrHostLoopStarted.
//
// The app's Config chooses the graphics API and the power preference; its
// window fields are not used, since Start opens no window.
func (loop *HostLoop) Start() error {
	a := loop.app
	if a.hostLoopStarted {
		return ErrHostLoopStarted
	}
	if err := a.initPlatformManager(); err != nil {
		return err
	}
	// No modal tick: nothing may call into the frame path from inside PumpEvents.
	a.renderLoop = &inlineRenderLoop{}
	renderer, err := newWindowlessRendererFn(a.config.GraphicsAPI, deviceRequestFrom(&a.config))
	if err != nil {
		a.manager.Destroy()
		return err
	}
	a.renderer = renderer
	a.windowManager = newWindowManager()
	a.startRunLoop()
	a.hostLoopStarted = true
	return nil
}

// NewWindow opens a window with its surface, from config's title, size,
// resizability, frame, transparency, header alignment and macOS tabbing.
// The surface presents with vsync when config.VSync is set. Every window of
// a HostLoop, the first included, opens here.
//
// The window opens hidden, and BeginFrame skips it until Show. A window
// asked fullscreen is shown fullscreen. Its surface is configured at once
// when the window already reports a size, and otherwise at the first
// BeginFrame that finds one.
//
// It must be called after Start, on Start's thread. A window closed by the
// user, or by Window.Close, is destroyed with its surface as App.NewWindow's
// windows are.
func (loop *HostLoop) NewWindow(config Config) (*Window, error) {
	if !loop.app.hostLoopStarted {
		return nil, fmt.Errorf("gogpu: HostLoop.NewWindow called before Start")
	}
	return loop.app.openWindow(config, config.VSync)
}

// PumpEvents clears the previous pump's input edges, then drains every
// pending platform event and dispatches it to the callbacks and the
// input-event queue registered on the App, for every window. A window's
// resize reconfigures its surface at once and calls the window's OnResize.
// A close destroys the window with its surface. It ends the gesture frame.
//
// It never waits, runs onUpdate or draws, and returns once the queue is
// empty. A pump allocates nothing apart from boxing each event it puts into
// the input-event queue, once per event, the text of a typed character
// outside ASCII, once for every text-input callback, and what a resize's
// surface reconfigure allocates. Its first pumps also resolve the system
// functions they call and grow its buffers to their largest size; neither
// recurs.
func (loop *HostLoop) PumpEvents() {
	loop.app.processEventsMultiThread()
}

// Running reports whether the app is still running: false once Quit was
// called, or once the last window closed while the app quits on its last
// window closed (see App.SetQuitOnLastWindowClosed).
func (loop *HostLoop) Running() bool {
	return loop.app.running.Load()
}

// BeginFrame applies the window's latest size to its surface and acquires the
// next swapchain image, so the returned Context can be recorded into at once.
// A surface that was lost or unconfigured is reconfigured, even at an
// unchanged size.
//
// It returns ok=false without acquiring when the window is nil, hidden or
// reports a zero size, and without blocking when the window's compositor
// frame gate (Wayland) is closed. It returns ok=false when the acquire
// fails; the host skips the frame and tries again next tick. HostLoop does
// not track whether a window is minimized: the acquire decides. A minimized
// Vulkan surface fails its acquire; a minimized DX12 window keeps acquiring
// and drawing, as Run does. With Fifo present the acquire blocks for up to
// about one refresh, which paces the loop to vsync.
//
// Each Window reuses one Context. It is valid until EndFrame, which must be
// called once for every BeginFrame that returned ok=true.
func (loop *HostLoop) BeginFrame(window *Window) (*Context, bool) {
	a := loop.app
	if window == nil || !window.visible || window.surface == nil {
		return nil, false
	}
	ws := window.surface
	physW, physH := window.platWindow.PhysicalSize()
	if physW <= 0 || physH <= 0 {
		// A surface cannot be configured at zero size: unconfigure it and
		// skip the acquire until the window reports a size again.
		a.renderer.ResizeSurface(ws, 0, 0)
		return nil, false
	}
	if !ws.CanRender() && ws.surface != nil {
		// Unconfigured or lost: force the reconfigure even at an unchanged
		// size, which resize would otherwise skip as a no-op.
		ws.width = 0
		ws.height = 0
	}
	a.syncFrameSurfaceSize(ws, physW, physH)
	if gate, ok := window.platWindow.(platform.FrameGater); ok && !gate.FrameCallbackReady() {
		return nil, false
	}

	loop.framePool = pushFramePool()
	a.renderer.DrainDeferredDestroys()
	ws.prepareLazyAcquire()
	a.renderer.currentSurface = ws
	if !ws.ensureFrameStarted() {
		a.renderer.endFrameStats(true)
		ws.resetLazyState()
		a.renderer.currentSurface = nil
		popFramePool(loop.framePool)
		loop.framePool = 0
		return nil, false
	}

	window.frameContext = Context{
		renderer:    a.renderer,
		surface:     ws,
		scaleFactor: window.platWindow.ScaleFactor(),
	}
	if window.frameContext.scaleFactor <= 0 {
		window.frameContext.scaleFactor = 1
	}
	return &window.frameContext, true
}

// EndFrame submits what the frame recorded, with gogpu's own overlay and
// composition work, and presents it. When the surface turned out outdated,
// that one frame is dropped and a redraw requested; the next BeginFrame
// reconfigures the surface.
func (loop *HostLoop) EndFrame(context *Context) {
	a := loop.app
	ws := context.surface
	if a.renderer.endFrameForSurface(ws) {
		a.RequestRedraw()
	}
	ws.resetLazyState()
	a.renderer.currentSurface = nil
	a.renderer.pollSubmissions()
	popFramePool(loop.framePool)
	loop.framePool = 0
}

// Stop closes every window still open, with its surface, as a close does,
// then waits for the GPU and releases the device and the platform, in Run's
// teardown order: GPU wait, deferred destroys, tracked resources and
// onClose; renderer; platform; GPU instance. The app cannot be started
// again.
func (loop *HostLoop) Stop() {
	a := loop.app
	a.windowManager.mu.RLock()
	windows := slices.Clone(a.windowManager.order)
	a.windowManager.mu.RUnlock()
	for _, window := range windows {
		a.closeSecondaryWindow(window)
	}
	loop.stop()
}

// stop marks the app stopped and tears it down in Run's order, closing the
// window start created, if any.
func (loop *HostLoop) stop() {
	a := loop.app
	a.running.Store(false)
	a.lifecycle = AppIdle
	a.shutdown(a.startedWindow)
}

// start creates Run's window, device and surface, shows the window and marks
// the app running. GPU work goes through renderLoop: the render thread for
// Run. modalFrameCallback runs frames inside a Windows modal move or resize
// loop; nil runs none. When the platform or the renderer cannot start, it
// destroys what it had created.
func (loop *HostLoop) start(renderLoop appRenderLoop, modalFrameCallback func()) error {
	a := loop.app
	platWindow, err := a.initPlatform()
	if err != nil {
		return err
	}
	platWindow.SetModalFrameCallback(modalFrameCallback)
	a.renderLoop = renderLoop

	if err := a.initRenderer(platWindow); err != nil {
		a.renderLoop.Stop()
		platWindow.Destroy()
		a.manager.Destroy()
		return err
	}

	a.startedWindow = platWindow
	platWindow.Show()
	a.syncInitialSurfaceSize()
	a.registerPrimaryWindow(platWindow)
	a.startRunLoop()
	return nil
}

// inlineRenderLoop runs render-thread work on the calling thread: the
// host-owned path has no render thread.
type inlineRenderLoop struct {
	pendingWidth  atomic.Uint32
	pendingHeight atomic.Uint32
	resizePending atomic.Bool
}

func (loop *inlineRenderLoop) RunOnRenderThreadVoid(fn func()) { fn() }

func (loop *inlineRenderLoop) Stop() {}

func (loop *inlineRenderLoop) RequestResize(width, height uint32) {
	loop.pendingWidth.Store(width)
	loop.pendingHeight.Store(height)
	loop.resizePending.Store(true)
}

func (loop *inlineRenderLoop) ConsumePendingResize() (uint32, uint32, bool) {
	if !loop.resizePending.Swap(false) {
		return 0, 0, false
	}
	return loop.pendingWidth.Load(), loop.pendingHeight.Load(), true
}
