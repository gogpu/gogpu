// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package gogpu

import (
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gputypes"
)

// HostLoop lets a host that owns its loop drive an App one step at a time,
// instead of handing the thread to App.Run:
//
//	loop := gogpu.NewHostLoop(app)
//	if err := loop.Start(); err != nil {
//		return err
//	}
//	if err := loop.RequestDevice(0, nil); err != nil {
//		return err
//	}
//	window, err := loop.NewWindow(gogpu.DefaultConfig())
//	if err != nil {
//		return err
//	}
//	window.Show()
//	for loop.Running() {
//		loop.PumpEvents()
//		context, ok, err := loop.BeginFrame(window)
//		if err != nil {
//			log.Print(err)
//		}
//		if ok {
//			context.Clear(0.1, 0.2, 0.3, 1)
//			if err := loop.EndFrame(context); err != nil {
//				log.Print(err)
//			}
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
	started   bool    // Start succeeded and Stop has not torn the app down yet
	framePool uintptr // macOS autorelease pool open between BeginFrame and EndFrame
}

// NewHostLoop returns a loop over app. Callbacks and windows stay on app.
func NewHostLoop(app *App) *HostLoop {
	return &HostLoop{app: app}
}

// Start brings up the platform and the GPU instance and adapter, with no
// device, no window and no surface, and marks the app running, then returns
// without blocking. The device comes from RequestDevice, once the host has
// read the adapter through GPUContextProvider; windows open through
// NewWindow after it. When the platform or the GPU cannot start, it returns
// the error and destroys what it had created. While a HostLoop is started,
// Run and a second Start return ErrHostLoopStarted.
//
// The app's Config chooses the graphics API, the power preference and the
// fallback adapter. Its RequiredFeatures and RequiredLimits are not used,
// since RequestDevice is given its own, nor are its window fields, since
// Start opens no window.
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
	loop.started = true
	return nil
}

// RequestDevice requests the device from the adapter Start brought up, with
// features and limits, as Config's RequiredFeatures and RequiredLimits ask
// for Run: nil limits are the adapter's own. A feature or limit the adapter
// lacks fails the request by name, and no device is created. It must be
// called once, after Start and before NewWindow, on Start's thread.
func (loop *HostLoop) RequestDevice(features gputypes.Features, limits *gputypes.Limits) error {
	a := loop.app
	if !a.hostLoopStarted {
		return fmt.Errorf("gogpu: HostLoop.RequestDevice called before Start")
	}
	if a.renderer.device != nil {
		return fmt.Errorf("gogpu: HostLoop.RequestDevice called twice")
	}
	a.renderer.deviceRequest.requiredFeatures = features
	a.renderer.deviceRequest.requiredLimits = gputypes.Limits{}
	if limits != nil {
		a.renderer.deviceRequest.requiredLimits = *limits
	}
	return a.renderer.requestDevice()
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
// It must be called after RequestDevice, on Start's thread. A window closed by the
// user, or by Window.Close, is destroyed with its surface as App.NewWindow's
// windows are.
func (loop *HostLoop) NewWindow(config Config) (*Window, error) {
	if !loop.app.hostLoopStarted {
		return nil, fmt.Errorf("gogpu: HostLoop.NewWindow called before Start")
	}
	if loop.app.renderer.device == nil {
		return nil, fmt.Errorf("gogpu: HostLoop.NewWindow called before RequestDevice")
	}
	return loop.app.openWindow(config, config.VSync)
}

// PumpEvents clears the previous pump's input edges, then drains every
// pending platform event and dispatches it to the callbacks and the
// input-event queue registered on the App, for every window. A window's
// resize calls the window's OnResize, and its surface takes the new size at
// the window's next BeginFrame. A close destroys the window with its surface.
// It ends the gesture frame.
//
// It never runs onUpdate or draws, and returns once the queue is empty. It
// waits only while the user drags a move or resize (see HostLoop). A pump
// allocates nothing apart from boxing each event it puts into the
// input-event queue, once per event, and the text of a typed character
// outside ASCII, once for every text-input callback. Its first pumps also
// resolve the system functions they call and grow its buffers to their
// largest size; neither recurs.
func (loop *HostLoop) PumpEvents() {
	loop.app.processEventsMultiThread()
}

// Running reports whether the app is still running: false once Quit was
// called, or once the last window closed while the app quits on its last
// window closed (see App.SetQuitOnLastWindowClosed).
func (loop *HostLoop) Running() bool {
	return loop.app.running.Load()
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
// It returns ok=false and no error without acquiring when the window is nil,
// hidden or reports a zero size, and without blocking when the window's
// compositor frame gate (Wayland) is closed. When the surface cannot be
// configured at the window's size, or the acquire fails, it returns ok=false
// and the error, which it does not log, typed as wgpu types it. A failed
// configure keeps the surface's previous size, and the next BeginFrame tries
// it again. A failed acquire answers wgpu.ErrSurfaceOutdated once the
// surface is reconfigured, or the reconfigure's own error when that fails
// too; wgpu.ErrSurfaceLost; wgpu.ErrTimeout; wgpu.ErrDeviceLost. The host
// skips the frame and tries again next tick.
//
// HostLoop does not track whether a window is minimized. On Windows a
// minimized window keeps reporting its last size: a DX12 window keeps
// acquiring and drawing, as Run does, while a Vulkan surface's extent drops
// to zero, so it cannot be configured, and BeginFrame returns ok=false and
// no error until the window is restored. EndFrame answers
// wgpu.ErrSurfaceOutdated for the presents around the minimize and the
// restore.
//
// With Fifo present the acquire blocks for up to about one refresh, which
// paces the loop to vsync.
//
// Each Window reuses one Context. It is valid until EndFrame, which must be
// called once for every BeginFrame that returned ok=true.
func (loop *HostLoop) BeginFrame(window *Window) (*Context, bool, error) {
	a := loop.app
	if window == nil || !window.visible || window.surface == nil {
		return nil, false, nil
	}
	ws := window.surface
	physW, physH := window.platWindow.PhysicalSize()
	if physW <= 0 || physH <= 0 {
		// A surface cannot be configured at zero size: unconfigure it and
		// skip the acquire until the window reports a size again.
		a.renderer.ResizeSurface(ws, 0, 0)
		return nil, false, nil
	}
	if !ws.CanRender() && ws.surface != nil {
		// Unconfigured or lost: force the reconfigure even at an unchanged
		// size, which resize would otherwise skip as a no-op.
		ws.width = 0
		ws.height = 0
	}
	if err := a.syncFrameSurfaceSize(ws, physW, physH); err != nil {
		return nil, false, err
	}
	if gate, ok := window.platWindow.(platform.FrameGater); ok && !gate.FrameCallbackReady() {
		return nil, false, nil
	}

	loop.framePool = pushFramePool()
	a.renderer.DrainDeferredDestroys()
	ws.prepareLazyAcquire()
	a.renderer.currentSurface = ws
	if !ws.ensureFrameStarted() {
		err := ws.acquireErr
		a.renderer.endFrameStats(true)
		ws.resetLazyState()
		a.renderer.currentSurface = nil
		popFramePool(loop.framePool)
		loop.framePool = 0
		return nil, false, err
	}

	window.frameContext = Context{
		renderer:    a.renderer,
		surface:     ws,
		scaleFactor: window.platWindow.ScaleFactor(),
	}
	if window.frameContext.scaleFactor <= 0 {
		window.frameContext.scaleFactor = 1
	}
	return &window.frameContext, true, nil
}

// EndFrame submits what the frame recorded, with gogpu's own overlay and
// composition work, and presents it. It returns the present's error, which
// it does not log, typed as wgpu types it. When the surface turned out
// outdated, the error is wgpu.ErrSurfaceOutdated: that one frame is dropped,
// the surface reconfigured and a redraw requested. A lost surface answers
// wgpu.ErrSurfaceLost, and the next BeginFrame reconfigures it.
func (loop *HostLoop) EndFrame(context *Context) error {
	a := loop.app
	ws := context.surface
	reconfigured, err := a.renderer.endFrameForSurface(ws)
	if reconfigured {
		a.RequestRedraw()
	}
	ws.resetLazyState()
	a.renderer.currentSurface = nil
	a.renderer.pollSubmissions()
	popFramePool(loop.framePool)
	loop.framePool = 0
	return err
}

// Stop closes every window still open, with its surface, as a close does,
// then waits for the GPU and releases the device, if one was requested, and
// the platform, in Run's teardown order: GPU wait, deferred destroys, tracked
// resources and onClose; renderer; platform; GPU instance. The app cannot be
// started again.
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
