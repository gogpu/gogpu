//go:build linux

package wayland

// libwayland_dnd.go — Wayland file drag-and-drop via wl_data_device.
//
// Implements file DnD using the same wl_data_device_manager + wl_data_device
// infrastructure as clipboard (libwayland_clipboard.go). The clipboard code
// already defines all interface descriptors and listener arrays — DnD reuses
// them. The wl_data_device events (enter, leave, motion, drop) were previously
// no-ops; this file provides the real implementations.
//
// Protocol flow (from the DROP TARGET perspective, which is what we implement):
//   1. data_device.data_offer — compositor introduces a new wl_data_offer proxy.
//      We add a listener to collect advertised MIME types.
//   2. data_device.enter — drag entered our surface. We check if text/uri-list
//      is offered, call wl_data_offer.accept + set_actions, queue EventDragEnter.
//   3. data_device.motion — pointer moves over our surface. Queue EventDragMove.
//   4. data_device.drop — user released. We pipe-read the data via
//      wl_data_offer.receive, parse text/uri-list URIs, call finish, queue
//      EventDragDrop.
//   5. data_device.leave — drag left without drop. Queue EventDragLeave, destroy offer.
//
// SDL3 reference: SDL_waylandevents.c:2781-3016 (Wayland_data_offer_handler,
// Wayland_data_device_handler — the only complete Wayland DnD implementation;
// winit does NOT implement Wayland DnD).
//
// URI parsing reuses x11/xdnd.go parseURIList via ParseURIList (exported wrapper).

import (
	"bytes"
	"io"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// dndURIListMIME is the MIME type for file URI lists in drag-and-drop.
const dndURIListMIME = "text/uri-list"

// dndActionCopy matches wl_data_device_manager.dnd_action.copy (value 1).
// We always request COPY for file drops — same behavior as SDL3/GTK.
const dndActionCopy = 1

// DnDCallbacks provides Go callbacks for drag-and-drop events.
// Set by the platform layer (platform_linux.go) during Wayland init.
// The surface argument in OnDragEnter allows routing to the correct window
// when multiple windows share the same wl_data_device (it is per-seat).
type DnDCallbacks struct {
	// OnDragEnter is called when files enter a surface.
	// surface is the wl_surface* proxy that the drag entered.
	OnDragEnter func(surface uintptr, x, y float64)

	// OnDragMove is called when files move over the surface.
	OnDragMove func(x, y float64)

	// OnDragDrop is called when files are dropped.
	// paths contains the decoded file paths from text/uri-list.
	OnDragDrop func(paths []string, x, y float64)

	// OnDragLeave is called when files leave the surface without dropping.
	OnDragLeave func()
}

// SetDnDCallbacks registers callbacks for drag-and-drop events.
// Must be called after SetupClipboard (same data_device).
func (h *LibwaylandHandle) SetDnDCallbacks(cb *DnDCallbacks) {
	h.dndMu.Lock()
	h.dndCallbacks = cb
	h.dndMu.Unlock()
}

// --- wl_data_device DnD callbacks (replace no-ops from clipboard) ---

// dataDeviceEnterCb: void(data, wl_data_device, serial, surface, x_fixed, y_fixed, id)
// DnD enter event — drag entered one of our surfaces.
// The id argument is the wl_data_offer proxy for this drag session.
func dataDeviceEnterCb(data, device, serial, surface, xFixed, yFixed, id uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	x := float64(int32(xFixed)) / 256.0
	y := float64(int32(yFixed)) / 256.0

	h.dndMu.Lock()

	// Destroy any leftover DnD offer from a previous session.
	if h.dndOffer != 0 && h.dndOffer != id {
		h.marshalVoid(h.dndOffer, 2) // wl_data_offer.destroy (opcode 2)
		h.proxyDestroy(h.dndOffer)
	}

	h.dndOffer = id
	h.dndSerial = uint32(serial)
	h.dndSurface = surface
	h.dndX = x
	h.dndY = y

	// Check if this offer advertised text/uri-list (set by dataOfferOfferCb
	// which fires between data_device.data_offer and data_device.enter).
	hasURIList := h.dndHasURIList
	cb := h.dndCallbacks
	h.dndMu.Unlock()

	if hasURIList && id != 0 {
		// Accept the text/uri-list MIME type.
		// wl_data_offer.accept: opcode 0, signature "u?s"
		mimeBuf := []byte(dndURIListMIME + "\x00")
		h.marshalVoid(id, 0, uintptr(uint32(serial)), uintptr(unsafe.Pointer(&mimeBuf[0])))
		runtime.KeepAlive(mimeBuf)

		// Set preferred action to COPY (version 3).
		// wl_data_offer.set_actions: opcode 4, signature "uu"
		h.marshalVoid(id, 4, uintptr(dndActionCopy), uintptr(dndActionCopy))
	} else if id != 0 {
		// Reject: accept with NULL mime type.
		// wl_data_offer.accept: opcode 0, signature "u?s"
		h.marshalVoid(id, 0, uintptr(uint32(serial)), 0)
	}

	if cb != nil && cb.OnDragEnter != nil {
		cb.OnDragEnter(surface, x, y)
	}
}

// dataDeviceMotionCb: void(data, wl_data_device, time, x_fixed, y_fixed)
// DnD motion event — drag pointer moved over our surface.
func dataDeviceMotionCb(data, device, timeMs, xFixed, yFixed uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	x := float64(int32(xFixed)) / 256.0
	y := float64(int32(yFixed)) / 256.0

	h.dndMu.Lock()
	h.dndX = x
	h.dndY = y
	cb := h.dndCallbacks
	h.dndMu.Unlock()

	if cb != nil && cb.OnDragMove != nil {
		cb.OnDragMove(x, y)
	}
}

// dataDeviceDropCb: void(data, wl_data_device)
// DnD drop event — user released the drag over our surface.
// We read the data via pipe and parse file URIs.
func dataDeviceDropCb(data, device uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	h.dndMu.Lock()
	offer := h.dndOffer
	hasURIList := h.dndHasURIList
	x := h.dndX
	y := h.dndY
	cb := h.dndCallbacks
	h.dndMu.Unlock()

	if offer == 0 || !hasURIList {
		slog.Debug("wayland DnD drop: no offer or no URI list, ignoring")
		return
	}

	// Create pipe for receiving the URI list data.
	var pipeFDs [2]int
	if err := syscall.Pipe2(pipeFDs[:], syscall.O_CLOEXEC); err != nil {
		slog.Warn("wayland DnD: pipe2 failed", "err", err)
		return
	}
	readFD := pipeFDs[0]
	writeFD := pipeFDs[1]

	// wl_data_offer.receive: opcode 1, signature "sh"
	// The fd is passed via SCM_RIGHTS by libwayland when it sees 'h' in signature.
	mimeBuf := []byte(dndURIListMIME + "\x00")
	h.marshalVoid(offer, 1, uintptr(unsafe.Pointer(&mimeBuf[0])), uintptr(writeFD))
	runtime.KeepAlive(mimeBuf)

	// Flush to send the receive request.
	if err := h.flushWithRetry(); err != nil {
		slog.Warn("wayland DnD: flush after receive failed", "err", err)
		syscall.Close(readFD)
		syscall.Close(writeFD)
		return
	}

	// Close write end — the source app writes data then closes its end.
	syscall.Close(writeFD)

	// Read URI list from pipe with timeout.
	uriData, err := readDnDPipe(readFD, 5*time.Second)
	syscall.Close(readFD)
	if err != nil {
		slog.Warn("wayland DnD: pipe read failed", "err", err)
	}

	// Finish the DnD operation (version 3).
	// wl_data_offer.finish: opcode 3
	h.marshalVoid(offer, 3)

	// Destroy the offer — DnD session is complete.
	h.marshalVoid(offer, 2) // wl_data_offer.destroy (opcode 2)
	h.proxyDestroy(offer)

	h.dndMu.Lock()
	h.dndOffer = 0
	h.dndHasURIList = false
	h.dndMu.Unlock()

	// Parse URIs into file paths.
	paths := ParseURIList(string(uriData))

	if cb != nil && cb.OnDragDrop != nil && len(paths) > 0 {
		cb.OnDragDrop(paths, x, y)
	}
}

// dataDeviceLeaveCb: void(data, wl_data_device)
// DnD leave event — drag left our surface without dropping.
func dataDeviceLeaveCb(data, device uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	h.dndMu.Lock()
	offer := h.dndOffer
	h.dndOffer = 0
	h.dndHasURIList = false
	cb := h.dndCallbacks
	h.dndMu.Unlock()

	// Destroy the offer if it was not consumed by a drop.
	if offer != 0 {
		h.marshalVoid(offer, 2) // wl_data_offer.destroy (opcode 2)
		h.proxyDestroy(offer)
	}

	if cb != nil && cb.OnDragLeave != nil {
		cb.OnDragLeave()
	}
}

// --- wl_data_offer DnD callbacks ---

// dataOfferSourceActionsCb: void(data, wl_data_offer, source_actions)
// Fired when the source announces supported DnD actions.
// We just log it — we always prefer COPY.
func dataOfferSourceActionsCb(data, offer, sourceActions uintptr) {
	slog.Debug("wayland DnD: source_actions", "actions", uint32(sourceActions))
}

// dataOfferActionCb: void(data, wl_data_offer, dnd_action)
// Fired when the compositor negotiated a DnD action.
func dataOfferActionCb(data, offer, dndAction uintptr) {
	slog.Debug("wayland DnD: action", "action", uint32(dndAction))
}

// --- Helpers ---

// ParseURIList parses a text/uri-list string (RFC 2483) into file paths.
// Each line is a URI; lines starting with # are comments.
// Only file:// URIs are converted to local paths; others are skipped.
// Percent-encoded characters (%XX) are decoded.
// Same logic as x11/xdnd.go parseURIList — duplicated because wayland and x11
// are separate packages with different build tags.
func ParseURIList(data string) []string {
	var paths []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse as URL to handle percent-encoding.
		u, err := url.Parse(line)
		if err != nil {
			continue
		}

		if u.Scheme != "file" {
			continue
		}

		// url.Parse handles percent-decoding in Path.
		path := u.Path
		if path == "" {
			continue
		}

		paths = append(paths, path)
	}
	return paths
}

// readDnDPipe reads all data from a file descriptor until EOF or timeout.
// Uses a goroutine with a timer. Same pattern as clipboard readPipeWithTimeout
// but with a descriptive name for DnD context.
func readDnDPipe(fd int, timeout time.Duration) ([]byte, error) {
	f := os.NewFile(uintptr(fd), "dnd-receive")
	if f == nil {
		return nil, nil
	}

	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, f)
		done <- result{buf.Bytes(), err}
	}()

	select {
	case r := <-done:
		return r.data, r.err
	case <-time.After(timeout):
		f.Close()
		return nil, nil
	}
}
