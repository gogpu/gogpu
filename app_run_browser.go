//go:build js && wasm

package gogpu

import (
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"

	"github.com/gogpu/gogpu/internal/platform"
)

// browserFrameLoop owns the JavaScript callbacks used while App.Run is
// active. At most one requestAnimationFrame callback may be pending.
type browserFrameLoop struct {
	app *App

	jsDocument         js.Value
	rafCallback        js.Func
	visibilityCallback js.Func

	rafID      int
	rafPending atomic.Bool
	stopped    atomic.Bool
	done       chan struct{}
	doneOnce   sync.Once
}

func newBrowserFrameLoop(app *App) *browserFrameLoop {
	loop := &browserFrameLoop{
		app:        app,
		jsDocument: js.Global().Get("document"),
		done:       make(chan struct{}),
	}
	loop.rafCallback = js.FuncOf(loop.onAnimationFrame)
	loop.visibilityCallback = js.FuncOf(loop.onVisibilityChange)
	return loop
}

// runMainLoop yields frame execution to the browser compositor. Run remains
// blocked until Quit or window close, while JavaScript resumes Go for each RAF.
func (a *App) runMainLoop() {
	loop := newBrowserFrameLoop(a)
	if setter, ok := a.manager.(platform.WakeUpHookSetter); ok {
		setter.SetWakeUpHook(loop.schedule)
	}

	loop.jsDocument.Call("addEventListener", "visibilitychange", loop.visibilityCallback)
	loop.schedule()
	<-loop.done

	loop.stop()
	loop.jsDocument.Call("removeEventListener", "visibilitychange", loop.visibilityCallback)
	if setter, ok := a.manager.(platform.WakeUpHookSetter); ok {
		setter.SetWakeUpHook(nil)
	}
	loop.visibilityCallback.Release()
	loop.rafCallback.Release()
}

func (l *browserFrameLoop) schedule() {
	if l.stopped.Load() {
		return
	}
	if l.shouldExit() {
		l.finish()
		return
	}
	if l.documentHidden() {
		return
	}
	if !l.rafPending.CompareAndSwap(false, true) {
		return
	}
	l.rafID = js.Global().Call("requestAnimationFrame", l.rafCallback).Int()
}

func (l *browserFrameLoop) onAnimationFrame(_ js.Value, _ []js.Value) any {
	l.rafPending.Store(false)
	if l.stopped.Load() {
		return nil
	}
	if l.shouldExit() {
		l.finish()
		return nil
	}
	if l.documentHidden() {
		return nil
	}

	l.app.runFrame()
	if l.shouldExit() {
		l.finish()
		return nil
	}

	if l.app.config.ContinuousRender || l.app.animations.IsAnimating() {
		l.schedule()
	}
	return nil
}

func (l *browserFrameLoop) onVisibilityChange(_ js.Value, _ []js.Value) any {
	if l.stopped.Load() {
		return nil
	}
	if l.documentHidden() {
		l.cancelPendingFrame()
		return nil
	}
	// Do not advance simulation by the time spent in a suspended tab.
	l.app.lastFrame = time.Now()
	l.app.RequestRedraw()
	return nil
}

func (l *browserFrameLoop) finish() {
	l.doneOnce.Do(func() { close(l.done) })
}

func (l *browserFrameLoop) stop() {
	if !l.stopped.CompareAndSwap(false, true) {
		return
	}
	l.cancelPendingFrame()
}

func (l *browserFrameLoop) cancelPendingFrame() {
	if l.rafPending.Swap(false) {
		js.Global().Call("cancelAnimationFrame", l.rafID)
	}
}

func (l *browserFrameLoop) shouldExit() bool {
	return !l.app.running.Load() || (l.app.platWindow != nil && l.app.platWindow.ShouldClose())
}

func (l *browserFrameLoop) documentHidden() bool {
	hidden := l.jsDocument.Get("hidden")
	return !hidden.IsUndefined() && !hidden.IsNull() && hidden.Bool()
}
