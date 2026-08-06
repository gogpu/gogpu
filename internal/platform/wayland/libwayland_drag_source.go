//go:build linux

package wayland

// libwayland_drag_source.go — Outgoing drag-and-drop (drag source) via wl_data_device.
//
// Uses the same wl_data_device_manager + wl_data_source infrastructure as
// clipboard (libwayland_clipboard.go). The start_drag opcode (0 on wl_data_device)
// initiates an outgoing drag session.
//
// Protocol flow (DRAG SOURCE perspective):
//   1. Create wl_data_source via wl_data_device_manager.create_data_source
//   2. Offer MIME type text/uri-list on the source
//   3. Call wl_data_device.start_drag(source, origin_surface, nil, serial)
//   4. Handle wl_data_source.send — write file URIs to the provided fd
//   5. Handle wl_data_source.canceled — drag canceled by compositor
//   6. Handle wl_data_source.dnd_finished (v3) — drop completed successfully
//
// The serial MUST be from the last pointer button press event. The compositor
// rejects start_drag with a stale or incorrect serial.
//
// SDL3 reference: SDL_waylanddatamanager.c (Wayland_data_source_send,
// SDL_WaylandDataSource_create, start_drag).

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// dragSourceState tracks an active outgoing drag session.
type dragSourceState struct {
	mu        sync.Mutex
	source    uintptr // wl_data_source* for this drag (or 0)
	paths     []string
	done      func(result int) // 0=canceled, 1=copy, 2=move
	completed bool
}

// activeDragSource is the current outgoing drag session. Only one can be active
// at a time (per Wayland protocol — one drag per wl_data_device).
var activeDragSource dragSourceState

// dragSourceListener is the callback array for the drag source's wl_data_source events.
// Separate from the clipboard's dataSourceListener because drag source events
// (send, canceled, dnd_finished) need different handling.
var (
	dragSourceListenerOnce sync.Once
	dragSourceListener     [6]uintptr // target, send, canceled, dnd_finished (v3), action (v3), dnd_drop_performed (v3)
)

func initDragSourceListeners() {
	dragSourceListenerOnce.Do(func() {
		// wl_data_source events for drag source:
		// event 0: target(mime_type) — ignored
		dragSourceListener[0] = ffi.NewCallback(dragSourceTargetCb)
		// event 1: send(mime_type, fd) — write data to fd
		dragSourceListener[1] = ffi.NewCallback(dragSourceSendCb)
		// event 2: canceled — drag canceled
		dragSourceListener[2] = ffi.NewCallback(dragSourceCancelledCb)
		// Events 3-5 are v3 extensions. We register 3 to cover dnd_finished
		// but the actual interface only has 3 events. The clipboard source
		// interface has 3 events; for drag source v3 adds dnd_finished, action,
		// and dnd_drop_performed as additional events. We handle them via the
		// same 3-event listener array (the compositor won't deliver events
		// past the declared EventCount).
	})
}

// dragSourceTargetCb: void(data, wl_data_source, mime_type)
func dragSourceTargetCb(data, source, mimeType uintptr) {
	// No-op — we accept whatever the target wants.
}

// dragSourceSendCb: void(data, wl_data_source, mime_type, fd)
// The compositor (or target) requests our data. Write text/uri-list to the fd.
func dragSourceSendCb(data, source, mimeType, fd uintptr) {
	activeDragSource.mu.Lock()
	paths := activeDragSource.paths
	activeDragSource.mu.Unlock()

	f := os.NewFile(fd, "drag-source-send")
	if f == nil {
		return
	}
	defer f.Close()

	// Write file paths as text/uri-list (RFC 2483).
	for _, p := range paths {
		u := url.URL{Scheme: "file", Path: p}
		fmt.Fprintf(f, "%s\r\n", u.String())
	}
}

// dragSourceCancelledCb: void(data, wl_data_source)
// The drag was canceled (user dropped outside a valid target, or Escape).
func dragSourceCancelledCb(data, source uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	activeDragSource.mu.Lock()
	doneFn := activeDragSource.done
	activeDragSource.source = 0
	activeDragSource.paths = nil
	activeDragSource.done = nil
	activeDragSource.completed = true
	activeDragSource.mu.Unlock()

	// Destroy the source.
	h.marshalVoid(source, 1) // wl_data_source.destroy (opcode 1)
	h.proxyDestroy(source)

	if doneFn != nil {
		doneFn(0) // canceled
	}
}

// SetLastButtonSerial stores the serial from the most recent pointer button press.
// This serial is required by wl_data_device.start_drag.
func (h *LibwaylandHandle) SetLastButtonSerial(serial uint32) {
	h.dragMu.Lock()
	h.lastButtonSerial = serial
	h.dragMu.Unlock()
}

// StartDragSource initiates an outgoing drag-and-drop with file paths.
// The serial from the last pointer button press is used. The done callback
// receives 0=canceled, 1=copied, 2=moved.
func (h *LibwaylandHandle) StartDragSource(paths []string, done func(result int)) error {
	if h.clipboardMgr == 0 || h.clipboardDevice == 0 {
		return fmt.Errorf("wayland: clipboard/data_device not initialized for drag")
	}

	initClipboardInterfaces()
	initDragSourceListeners()

	h.dragMu.Lock()
	serial := h.lastButtonSerial
	h.dragMu.Unlock()

	if serial == 0 {
		return fmt.Errorf("wayland: no pointer button serial available for start_drag")
	}

	// Create a new wl_data_source.
	// create_data_source: opcode 0 on wl_data_device_manager, signature "n"
	source, err := h.marshalConstructor(h.clipboardMgr, 0, unsafe.Pointer(&clipboardInterfaces.source))
	if err != nil {
		return fmt.Errorf("wayland: create_data_source for drag failed: %w", err)
	}

	// Add listener for send/canceled events.
	if err := h.addListener(source, uintptr(unsafe.Pointer(&dragSourceListener[0]))); err != nil {
		slog.Warn("wayland: failed to add drag source listener", "err", err)
	}

	// Offer text/uri-list MIME type.
	mimeBuf := []byte(dndURIListMIME + "\x00")
	h.marshalVoid(source, 0, uintptr(unsafe.Pointer(&mimeBuf[0]))) // offer: opcode 0
	runtime.KeepAlive(mimeBuf)

	// Store drag state.
	activeDragSource.mu.Lock()
	activeDragSource.source = source
	activeDragSource.paths = paths
	activeDragSource.done = done
	activeDragSource.completed = false
	activeDragSource.mu.Unlock()

	// start_drag: opcode 0 on wl_data_device
	// signature "?oo?ou" — source (object?), origin (object), icon (object?), serial (uint)
	// We pass: source, our surface, nil (no icon), serial
	surface := h.Surface()
	h.marshalVoid(h.clipboardDevice, 0,
		source,          // wl_data_source
		surface,         // origin surface
		uintptr(0),      // icon surface (nil = no drag icon)
		uintptr(serial), // serial from last button press
	)

	// Flush to send start_drag to compositor.
	if err := h.flushWithRetry(); err != nil {
		slog.Warn("wayland: flush after start_drag failed", "err", err)
	}

	return nil
}
