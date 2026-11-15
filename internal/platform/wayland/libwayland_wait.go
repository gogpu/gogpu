//go:build linux

package wayland

import (
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"golang.org/x/sys/unix"
)

// EventConn is one Wayland connection's side of libwayland's multi-threaded
// read protocol:
//
//	prepare_read → flush → poll the fd → read_events when readable, cancel_read otherwise
//
// Holding the read intent across the poll is what makes a blocking wait
// correct. A second thread on the same wl_display (Mesa's Vulkan WSI in gogpu,
// ADR-041) that wants to read either takes the intent first — and then our
// PrepareRead fails, so we return instead of sleeping — or waits for ours. An
// event can therefore never be read into our queue by someone else while we
// sleep on an empty socket (gogpu/ui#240).
//
// References: SDL3 src/video/wayland/SDL_waylandevents.c:542-594
// (Wayland_WaitEventTimeout), winit winit-wayland/src/event_loop/mod.rs:783-793
// (the pump_events thread: prepare_read → flush → poll → read_without_dispatch).
type EventConn interface {
	// GetDisplayFD returns the connection's socket fd.
	GetDisplayFD() int
	// PrepareRead takes the read intent for the app queue. It returns false
	// when the display already has events queued or another thread holds the
	// intent: the caller must go back to dispatching instead of sleeping.
	PrepareRead() bool
	// FlushConn sends buffered requests. EAGAIN is retried briefly.
	FlushConn() error
	// ReadEvents reads the socket into the event queues. Only call with the
	// intent held and the fd readable.
	ReadEvents()
	// CancelRead releases the read intent without reading.
	CancelRead()
}

// WaitForReadable blocks until one of the connections or one of the extra fds
// becomes ready, then reads the ready connections and cancels the rest. When a
// connection reports queued events (PrepareRead false) it returns at once, so
// the caller's dispatch pumps them before the loop sleeps again.
//
// No display lock is held during the poll. A render thread inside the Vulkan
// WSI may hold it while libwayland waits for our read intent; blocking on the
// lock here would deadlock both threads.
func WaitForReadable(conns []EventConn, extra []unix.PollFd) {
	if len(conns) == 0 {
		if len(extra) > 0 {
			_, _ = unix.Poll(extra, -1)
		}
		return
	}

	fds := make([]unix.PollFd, 0, len(conns)+len(extra))
	// fdIndex[i] is the poll fd entry of conns[i], or -1 when it has no socket.
	fdIndex := make([]int, len(conns))

	prepared := 0
	for i, conn := range conns {
		if fd := conn.GetDisplayFD(); fd >= 0 {
			fdIndex[i] = len(fds)
			fds = append(fds, unix.PollFd{Fd: int32(fd), Events: unix.POLLIN | unix.POLLERR})
		} else {
			fdIndex[i] = -1
		}
		if !conn.PrepareRead() {
			cancelPrepared(conns[:prepared])
			return
		}
		prepared++
		if err := conn.FlushConn(); err != nil {
			cancelPrepared(conns[:prepared])
			return
		}
	}

	fds = append(fds, extra...)
	if _, err := unix.Poll(fds, -1); err != nil {
		cancelPrepared(conns[:prepared])
		return
	}

	for i, conn := range conns {
		if idx := fdIndex[i]; idx >= 0 && fds[idx].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			conn.ReadEvents()
		} else {
			conn.CancelRead()
		}
	}
}

func cancelPrepared(conns []EventConn) {
	for _, conn := range conns {
		conn.CancelRead()
	}
}

// PrepareRead implements EventConn: wl_display_prepare_read_queue for the app
// queue (falling back to wl_display_prepare_read when it has no name yet). The
// queue-specific variant keeps Mesa's default-queue callbacks out of our read.
func (h *LibwaylandHandle) PrepareRead() bool {
	h.displayMu.Lock()
	defer h.displayMu.Unlock()

	var prepared int32
	if h.appQueue != 0 {
		args := [2]unsafe.Pointer{unsafe.Pointer(&h.display), unsafe.Pointer(&h.appQueue)}
		_, _ = ffi.CallFunction(&h.cifPrepareReadQ, h.fnPrepareReadQueue, unsafe.Pointer(&prepared), args[:])
	} else {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
		_, _ = ffi.CallFunction(&h.cifPrepareRead, h.fnPrepareRead, unsafe.Pointer(&prepared), args[:])
	}
	return prepared == 0
}

// FlushConn implements EventConn.
func (h *LibwaylandHandle) FlushConn() error {
	h.displayMu.Lock()
	defer h.displayMu.Unlock()
	return h.flushWithRetry()
}

// ReadEvents implements EventConn: wl_display_read_events releases the read
// intent and moves the socket's events into their queues.
//
// It takes no displayMu on purpose: the call fires no callbacks, and the read
// intent is its own mutual exclusion with other readers. Taking the lock here
// could block behind a render thread that holds it while libwayland waits for
// this very read intent, deadlocking both.
func (h *LibwaylandHandle) ReadEvents() {
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	var read int32
	_, _ = ffi.CallFunction(&h.cifReadEvents, h.fnReadEvents, unsafe.Pointer(&read), args[:])
}

// CancelRead implements EventConn: wl_display_cancel_read releases the read
// intent without reading. Same locking rule as ReadEvents.
func (h *LibwaylandHandle) CancelRead() {
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	_, _ = ffi.CallFunction(&h.cifPrepareRead, h.fnCancelRead, nil, args[:])
}
