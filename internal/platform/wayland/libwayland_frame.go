//go:build linux

package wayland

// libwayland_frame.go — wl_surface.frame callback gating (BUG-WL-006 / FRAME-001).
//
// Implements a 3-state machine (winit pattern: None → Requested → Received) that
// gates the render loop from submitting frames faster than the Wayland compositor
// can composite them with CSD subsurfaces. Without this, the render loop can
// outrun the compositor's presentation rate, causing visible tearing/flicker
// during animation that is only visible on camera (NOT in screen recordings).
//
// Enterprise references:
//   - winit (Rust): state.rs:273-289, event_loop/mod.rs:460
//     3-state machine: None|Requested|Received. frame() only if None|Received.
//   - SDL3: SDL_waylandwindow.c:833-834,2966-2969
//     Perpetual chain: wl_callback_destroy + wl_surface_frame in done handler.
//   - Gio (Go): os_wayland.go:1779-1786
//     lastFrameCallback == nil gate before drawing.
//
// Implementation uses C libwayland FFI (goffi) path because our C wl_surface
// lives on the C display connection. A Pure Go WlCallback would never receive
// the compositor's done event dispatched by C libwayland.

import (
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// FrameCallbackState represents the 3-state machine for frame callback gating.
// Follows the winit pattern (state.rs).
const (
	// FrameCallbackNone means no frame callback is in flight.
	// Rendering is allowed. Next RequestFrameCallback transitions to Requested.
	FrameCallbackNone int32 = 0

	// FrameCallbackRequested means a frame callback has been sent to the compositor
	// and we are waiting for the done event. Rendering MUST be skipped.
	FrameCallbackRequested int32 = 1

	// FrameCallbackReceived means the compositor fired the done event.
	// Rendering is allowed. Next RequestFrameCallback transitions to Requested.
	FrameCallbackReceived int32 = 2
)

// wl_callback interface descriptor — constructed for addListener on the frame callback proxy.
// wl_callback has 0 methods and 1 event (done: opcode 0, signature "u").
var frameCallbackIface struct {
	once     sync.Once
	iface    cWlInterface
	events   [1]cWlMessage
	listener [1]uintptr // done callback
}

// initFrameCallbackIface constructs the wl_callback interface descriptor
// and creates the goffi listener once.
func initFrameCallbackIface() {
	frameCallbackIface.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&xdg.nullTypes[0]))
		// Ensure xdg nullTypes are initialized (shared zero-filled array).
		initXdgInterfaces()

		// wl_callback has 0 methods and 1 event: done(callback_data: uint)
		frameCallbackIface.events[0] = cWlMessage{
			Name:      cstr("done\x00"),
			Signature: cstr("u\x00"),
			Types:     nt,
		}

		frameCallbackIface.iface = cWlInterface{
			Name:        cstr("wl_callback\x00"),
			Version:     1,
			MethodCount: 0,
			Methods:     0,
			EventCount:  1,
			Events:      uintptr(unsafe.Pointer(&frameCallbackIface.events[0])),
		}

		// Create goffi callback for the done handler.
		frameCallbackIface.listener[0] = ffi.NewCallback(frameCallbackDoneCb)
	})
}

// --- Per-proxy callback routing ---
// Frame callbacks are ephemeral (one-shot), so we use a map keyed by
// the wl_callback proxy pointer to route the done event to the correct
// LibwaylandHandle. The entry is inserted in RequestFrameCallback and
// removed in the done handler.

var (
	frameCallbackHandlesMu sync.Mutex
	frameCallbackHandles   = make(map[uintptr]*LibwaylandHandle)
)

// frameCallbackDoneCb is the C-callable handler for wl_callback.done.
// Signature: void(data, wl_callback, callback_data).
//
// Called during DispatchDefaultQueue (which holds displayMu). The callback
// is always dispatched on the main thread — same context as all other
// Wayland event handlers in this codebase.
func frameCallbackDoneCb(data, callback, callbackData uintptr) {
	frameCallbackHandlesMu.Lock()
	h := frameCallbackHandles[callback]
	delete(frameCallbackHandles, callback)
	frameCallbackHandlesMu.Unlock()

	if h == nil {
		return
	}

	slog.Debug("wl_surface.frame done", "callback_data", uint32(callbackData))

	// Destroy the callback proxy (SDL3 pattern: destroy before re-register).
	h.proxyDestroy(callback)

	// Transition: Requested → Received
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackReceived)

	// Signal that a new frame can be drawn. The main loop will call
	// RequestRedraw on the next iteration if animation is active.
	h.frameCallbackReady.Store(true)
}

// RequestFrameCallback sends a wl_surface.frame request to the compositor.
// Idempotent: if state is already Requested, this is a no-op.
//
// Must be called from the render thread (after present) or main thread.
// The C libwayland calls require displayMu, which is acquired internally.
//
// Follows the winit pattern (state.rs:273-282): only send frame() if
// current state is None or Received (never double-request).
func (h *LibwaylandHandle) RequestFrameCallback() {
	// Fast path: already requested — no-op (idempotent, winit pattern).
	state := atomic.LoadInt32(&h.frameCallbackState)
	if state == FrameCallbackRequested {
		return
	}

	initFrameCallbackIface()

	h.displayMu.Lock()
	defer h.displayMu.Unlock()

	// Double-check under lock (another thread may have raced).
	state = atomic.LoadInt32(&h.frameCallbackState)
	if state == FrameCallbackRequested {
		return
	}

	// wl_surface.frame (opcode 3) creates a new wl_callback.
	// Use marshalConstructor with the wl_callback interface.
	callback, err := h.marshalConstructor(h.surface, 3, unsafe.Pointer(&frameCallbackIface.iface))
	if err != nil {
		slog.Warn("wayland: wl_surface.frame failed", "err", err)
		return
	}

	// Register callback → handle mapping for the done handler.
	frameCallbackHandlesMu.Lock()
	frameCallbackHandles[callback] = h
	frameCallbackHandlesMu.Unlock()

	// Add listener so the compositor's done event reaches our handler.
	if err := h.addListener(callback, uintptr(unsafe.Pointer(&frameCallbackIface.listener[0]))); err != nil {
		slog.Warn("wayland: frame callback listener failed", "err", err)
		// Clean up on failure.
		frameCallbackHandlesMu.Lock()
		delete(frameCallbackHandles, callback)
		frameCallbackHandlesMu.Unlock()
		h.proxyDestroy(callback)
		return
	}

	// wl_surface.frame is double-buffered (Wayland spec: "The frame request
	// will take effect on the next wl_surface.commit"). RequestFrameCallback
	// is called BEFORE Vulkan present (winit pre_present_notify pattern), so
	// the present's internal wl_surface.commit activates the frame callback
	// atomically with the buffer swap. No explicit commit or flush needed.
	//
	// Enterprise refs: winit calls frame() in pre_present_notify BEFORE
	// swap_buffers; SDL3 calls wl_surface_frame in surface_frame_done
	// BEFORE the next eglSwapBuffers; Gio calls wl_surface_frame BEFORE
	// the frame event that triggers rendering + present.

	// Transition: None|Received → Requested
	atomic.StoreInt32(&h.frameCallbackState, FrameCallbackRequested)
	h.frameCallbackReady.Store(false)

	// Prevent GC of the listener array while the callback is in flight.
	runtime.KeepAlive(&frameCallbackIface.listener)

	slog.Debug("wl_surface.frame requested")
}

// FrameCallbackReady reports whether the render loop may submit a frame.
// Returns true when state is None (initial) or Received (compositor said go).
// Returns false when state is Requested (waiting for compositor).
//
// Safe to call from any goroutine (atomic load).
func (h *LibwaylandHandle) FrameCallbackReady() bool {
	return atomic.LoadInt32(&h.frameCallbackState) != FrameCallbackRequested
}

// ConsumeFrameCallbackReady atomically checks and clears the ready flag.
// Returns true if the frame callback was received since the last consume.
// Used by the platform layer to trigger RequestRedraw after compositor ack.
func (h *LibwaylandHandle) ConsumeFrameCallbackReady() bool {
	return h.frameCallbackReady.CompareAndSwap(true, false)
}

// FrameCallbackEnabled reports whether frame callback gating is active.
// Disabled by setting GOGPU_WAYLAND_FRAME_CALLBACK=0.
func FrameCallbackEnabled() bool {
	v := os.Getenv("GOGPU_WAYLAND_FRAME_CALLBACK")
	return v != "0"
}
