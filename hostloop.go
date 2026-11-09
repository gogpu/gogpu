// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package gogpu

import (
	"sync/atomic"
)

// HostLoop lets a host that owns its loop drive an App one step at a time,
// instead of handing the thread to App.Run:
//
//	loop := gogpu.NewHostLoop(app)
//	if err := loop.Start(); err != nil {
//		return err
//	}
//	for loop.Running() {
//		loop.PumpEvents()
//		if context, ok := loop.BeginFrame(app.PrimaryWindow()); ok {
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
// Windows and macOS: a move or resize the user drags on Windows, and a live
// resize on macOS, run the platform's own loop inside PumpEvents, which
// returns when the user lets go. gogpu draws nothing meanwhile, so the frame
// freezes and resumes at the new size.
//
// macOS: each frame runs inside an autorelease pool that BeginFrame pushes and
// EndFrame pops.
type HostLoop struct {
	app       *App
	started   bool    // start succeeded and Stop has not torn the app down yet
	framePool uintptr // macOS autorelease pool open between BeginFrame and EndFrame
}

// NewHostLoop returns a loop over app. Callbacks and windows stay on app.
func NewHostLoop(app *App) *HostLoop {
	return &HostLoop{app: app}
}

// Start creates the window, device and surface, shows the window and marks
// the app running, then returns without blocking. When the platform or the
// GPU cannot start, it returns the error and destroys what it had created.
// While a HostLoop is started, Run and a second Start return
// ErrHostLoopStarted.
func (loop *HostLoop) Start() error {
	if loop.app.hostLoopStarted {
		return ErrHostLoopStarted
	}
	// No modal tick: nothing may call into the frame path from inside PumpEvents.
	if err := loop.start(&inlineRenderLoop{}, nil); err != nil {
		return err
	}
	loop.app.hostLoopStarted = true
	return nil
}

// PumpEvents clears the previous pump's input edges, then drains every
// pending platform event and dispatches it to the callbacks and the
// input-event queue registered on the App, for every window. The primary
// window's latest resize reaches App.OnResize and EventSource.OnResize, as
// under Run; the surface itself is resized at the next BeginFrame. It ends
// the gesture frame.
//
// It never runs onUpdate or draws, and returns once the queue is empty. It
// waits only while the user drags a move or resize (see HostLoop). A close is
// reported through Running, not acted on. A pump allocates
// nothing apart from boxing each event it puts into the input-event queue,
// once per event, and the text of a typed character outside ASCII, once for
// every text-input callback. Its first pumps also resolve the system
// functions they call and grow its buffers to their largest size; neither
// recurs.
func (loop *HostLoop) PumpEvents() {
	loop.app.processEventsMultiThread()
}

// Running reports whether the app is still running: false once the primary
// window was closed or Quit was called.
func (loop *HostLoop) Running() bool {
	a := loop.app
	return a.running.Load() && (a.platWindow == nil || !a.platWindow.ShouldClose())
}

// ConsumeRedrawRequest reports whether a redraw was requested since its last
// call, and clears the request. Requests come from App.RequestRedraw and the
// GPU context provider's RequestRedraw, from any goroutine; from the events
// PumpEvents dispatches that change what a window shows, such as a resize or
// a focus change; and from an EndFrame whose surface turned out outdated.
// Start requests the first frame.
//
// A host that draws on demand calls it once per tick and draws its windows
// when it reports true, as Run does: the request is the App's, not one
// window's. Config.ContinuousRender does not apply to a HostLoop; the host
// draws whenever it chooses.
func (loop *HostLoop) ConsumeRedrawRequest() bool {
	invalidator := loop.app.invalidator
	return invalidator != nil && invalidator.Consume()
}

// BeginFrame applies the window's latest size to its surface and acquires the
// next swapchain image, so the returned Context can be recorded into at once.
// A surface that was lost or unconfigured is reconfigured, even at an
// unchanged size.
//
// It returns ok=false without acquiring when the window is nil, hidden or
// reports a zero size, and without blocking when the compositor's frame
// gate (Wayland) is closed. It returns ok=false when the acquire fails; the
// host skips the frame and tries again next tick. HostLoop does not track
// whether a window is minimized: the acquire decides. A minimized Vulkan
// surface fails its acquire; a minimized DX12 window keeps acquiring and
// drawing, as Run does. With Fifo present the acquire blocks for up to about
// one refresh, which paces the loop to vsync.
//
// Each Window reuses one Context. It is valid until EndFrame, which must be
// called once for every BeginFrame that returned ok=true.
func (loop *HostLoop) BeginFrame(window *Window) (*Context, bool) {
	a := loop.app
	if window == nil || !window.visible || window.surface == nil {
		return nil, false
	}
	ws := window.surface
	if window == a.primaryWindow {
		if w, h, ok := a.renderLoop.ConsumePendingResize(); ok {
			a.renderer.Resize(int(w), int(h))
		}
	}
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
	if !a.frameCallbackReady() {
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

// Stop waits for the GPU, releases the device and surface and closes the
// window, in Run's teardown order: GPU wait, deferred destroys, tracked
// resources and onClose; renderer; window and platform; GPU instance. It
// works after the user closed the window. The app cannot be started again.
//
// Stop does nothing unless this loop's Start succeeded, and nothing the
// second time: a Start that failed has already destroyed what it created, so
// a deferred Stop is safe whether Start succeeded or not.
func (loop *HostLoop) Stop() {
	if !loop.started {
		return
	}
	loop.started = false
	a := loop.app
	a.running.Store(false)
	a.lifecycle = AppIdle
	a.shutdown(a.startedWindow)
}

// start creates the window, device and surface, shows the window and marks
// the app running. GPU work goes through renderLoop: the calling thread for
// a HostLoop, the render thread for Run. modalFrameCallback runs frames
// inside a Windows modal move or resize loop; nil runs none. When the
// platform or the renderer cannot start, it destroys what it had created.
func (loop *HostLoop) start(renderLoop appRenderLoop, modalFrameCallback func()) error {
	a := loop.app
	platWindow, err := a.initPlatform()
	if err != nil {
		return err
	}
	platWindow.SetModalFrameCallback(modalFrameCallback)
	a.renderLoop = renderLoop

	if err := a.initRenderer(platWindow); err != nil {
		a.destroyRendererAndPlatform(platWindow)
		return err
	}

	a.startedWindow = platWindow
	platWindow.Show()
	a.syncInitialSurfaceSize()
	a.registerPrimaryWindow(platWindow)
	a.startRunLoop()
	loop.started = true
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
