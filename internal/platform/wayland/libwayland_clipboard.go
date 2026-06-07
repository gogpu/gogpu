//go:build linux

package wayland

// libwayland_clipboard.go — Wayland clipboard (copy/paste) via wl_data_device_manager.
//
// Implements clipboard read/write using the wl_data_device_manager protocol:
//   - wl_data_device_manager: global manager, binds from registry
//   - wl_data_device: per-seat device for clipboard events (selection changed)
//   - wl_data_source: created when we write to clipboard (offers our data)
//   - wl_data_offer: received when another app writes to clipboard (we can read)
//
// Write flow: create data_source → offer mime → set_selection → handle send events.
// Read flow: if we own → return local; else → pipe + receive from current offer.
//
// Uses the same C-compatible interface descriptor pattern as libwayland_cursor_shape.go.

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// clipboardMIME is the standard MIME type for UTF-8 text clipboard data.
const clipboardMIME = "text/plain;charset=utf-8"

// clipboardInterfaces holds C-compatible interface descriptors for
// wl_data_device_manager, wl_data_source, wl_data_offer, and wl_data_device.
// Constructed once, live for program lifetime.
var clipboardInterfaces struct {
	once sync.Once

	// Interface descriptors
	manager cWlInterface // wl_data_device_manager
	source  cWlInterface // wl_data_source
	offer   cWlInterface // wl_data_offer
	device  cWlInterface // wl_data_device

	// Method arrays (indexed by opcode)
	managerMethods [2]cWlMessage // create_data_source, get_data_device
	sourceMethods  [2]cWlMessage // offer, destroy
	offerMethods   [3]cWlMessage // accept, receive, destroy
	deviceMethods  [3]cWlMessage // start_drag, set_selection, release

	// Event arrays
	sourceEvents [3]cWlMessage // target, send, canceled
	offerEvents  [1]cWlMessage // offer
	deviceEvents [6]cWlMessage // data_offer, enter, leave, motion, drop, selection

	// NULL types array (shared by messages without new_id arguments)
	nullTypes [8]uintptr

	// dataOfferTypes: types array for data_device.data_offer event (signature "n").
	// types[0] must point to wl_data_offer interface so that libwayland's
	// create_proxies() assigns the correct interface to the new proxy.
	// Without this, the proxy gets interface=NULL → SIGSEGV in queue_event
	// at proxy->object.interface->event_count (offset 0x18 from NULL).
	dataOfferTypes [1]uintptr
}

// initClipboardInterfaces constructs C-compatible interface descriptors
// for wl_data_device_manager, wl_data_source, wl_data_offer, wl_data_device.
// Called once during the first clipboard setup.
func initClipboardInterfaces() {
	clipboardInterfaces.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&clipboardInterfaces.nullTypes[0]))

		// === wl_data_device_manager methods ===
		// opcode 0: create_data_source() → new_id<wl_data_source>, signature "n"
		clipboardInterfaces.managerMethods[0] = cWlMessage{cstr("create_data_source\x00"), cstr("n\x00"), nt}
		// opcode 1: get_data_device(seat) → new_id<wl_data_device>, signature "no"
		clipboardInterfaces.managerMethods[1] = cWlMessage{cstr("get_data_device\x00"), cstr("no\x00"), nt}

		// wl_data_device_manager interface (no events)
		clipboardInterfaces.manager = cWlInterface{
			Name:        cstr("wl_data_device_manager\x00"),
			Version:     3,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&clipboardInterfaces.managerMethods[0])),
			EventCount:  0,
			Events:      0,
		}

		// === wl_data_source methods ===
		// opcode 0: offer(mime_type: string), signature "s"
		clipboardInterfaces.sourceMethods[0] = cWlMessage{cstr("offer\x00"), cstr("s\x00"), nt}
		// opcode 1: destroy(), signature ""
		clipboardInterfaces.sourceMethods[1] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}

		// wl_data_source events
		// event 0: target(mime_type: string?), signature "?s"
		clipboardInterfaces.sourceEvents[0] = cWlMessage{cstr("target\x00"), cstr("?s\x00"), nt}
		// event 1: send(mime_type: string, fd: fd), signature "sh"
		clipboardInterfaces.sourceEvents[1] = cWlMessage{cstr("send\x00"), cstr("sh\x00"), nt}
		// event 2: source ownership revoked, signature ""
		clipboardInterfaces.sourceEvents[2] = cWlMessage{cstr("cancelled\x00"), cstr("\x00"), nt} //nolint:misspell // Wayland protocol event name

		// wl_data_source interface
		clipboardInterfaces.source = cWlInterface{
			Name:        cstr("wl_data_source\x00"),
			Version:     3,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&clipboardInterfaces.sourceMethods[0])),
			EventCount:  3,
			Events:      uintptr(unsafe.Pointer(&clipboardInterfaces.sourceEvents[0])),
		}

		// === wl_data_offer methods ===
		// opcode 0: accept(serial: uint, mime_type: string?), signature "u?s"
		clipboardInterfaces.offerMethods[0] = cWlMessage{cstr("accept\x00"), cstr("u?s\x00"), nt}
		// opcode 1: receive(mime_type: string, fd: fd), signature "sh"
		clipboardInterfaces.offerMethods[1] = cWlMessage{cstr("receive\x00"), cstr("sh\x00"), nt}
		// opcode 2: destroy(), signature ""
		clipboardInterfaces.offerMethods[2] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}

		// wl_data_offer events
		// event 0: offer(mime_type: string), signature "s"
		clipboardInterfaces.offerEvents[0] = cWlMessage{cstr("offer\x00"), cstr("s\x00"), nt}

		// wl_data_offer interface
		clipboardInterfaces.offer = cWlInterface{
			Name:        cstr("wl_data_offer\x00"),
			Version:     3,
			MethodCount: 3,
			Methods:     uintptr(unsafe.Pointer(&clipboardInterfaces.offerMethods[0])),
			EventCount:  1,
			Events:      uintptr(unsafe.Pointer(&clipboardInterfaces.offerEvents[0])),
		}

		// === wl_data_device methods ===
		// opcode 0: start_drag(source?, origin, icon?, serial), signature "?oo?ou"
		clipboardInterfaces.deviceMethods[0] = cWlMessage{cstr("start_drag\x00"), cstr("?oo?ou\x00"), nt}
		// opcode 1: set_selection(source?, serial), signature "?ou"
		clipboardInterfaces.deviceMethods[1] = cWlMessage{cstr("set_selection\x00"), cstr("?ou\x00"), nt}
		// opcode 2: release(), signature ""
		clipboardInterfaces.deviceMethods[2] = cWlMessage{cstr("release\x00"), cstr("\x00"), nt}

		// wl_data_device events
		// event 0: data_offer(id: new_id<wl_data_offer>), signature "n"
		// types[0] MUST point to wl_data_offer interface — libwayland's create_proxies()
		// reads it to set the new proxy's interface. NULL here → proxy with interface=NULL
		// → SIGSEGV at proxy->object.interface->event_count (addr=0x18). See gogpu#292.
		clipboardInterfaces.dataOfferTypes[0] = uintptr(unsafe.Pointer(&clipboardInterfaces.offer))
		clipboardInterfaces.deviceEvents[0] = cWlMessage{cstr("data_offer\x00"), cstr("n\x00"),
			uintptr(unsafe.Pointer(&clipboardInterfaces.dataOfferTypes[0]))}
		// event 1: enter(serial, surface, x_fixed, y_fixed, id<data_offer>), signature "uoff?o"
		clipboardInterfaces.deviceEvents[1] = cWlMessage{cstr("enter\x00"), cstr("uoff?o\x00"), nt}
		// event 2: leave(), signature ""
		clipboardInterfaces.deviceEvents[2] = cWlMessage{cstr("leave\x00"), cstr("\x00"), nt}
		// event 3: motion(time, x_fixed, y_fixed), signature "uff"
		clipboardInterfaces.deviceEvents[3] = cWlMessage{cstr("motion\x00"), cstr("uff\x00"), nt}
		// event 4: drop(), signature ""
		clipboardInterfaces.deviceEvents[4] = cWlMessage{cstr("drop\x00"), cstr("\x00"), nt}
		// event 5: selection(id<data_offer>?), signature "?o"
		clipboardInterfaces.deviceEvents[5] = cWlMessage{cstr("selection\x00"), cstr("?o\x00"), nt}

		// wl_data_device interface
		clipboardInterfaces.device = cWlInterface{
			Name:        cstr("wl_data_device\x00"),
			Version:     3,
			MethodCount: 3,
			Methods:     uintptr(unsafe.Pointer(&clipboardInterfaces.deviceMethods[0])),
			EventCount:  6,
			Events:      uintptr(unsafe.Pointer(&clipboardInterfaces.deviceEvents[0])),
		}
	})
}

// --- Listener arrays and callbacks ---

var (
	dataSourceListener  [3]uintptr // target, send, canceled
	dataDeviceListener  [6]uintptr // data_offer, enter, leave, motion, drop, selection
	dataOfferListener   [1]uintptr // offer (mime type advertisement)
	clipboardListenOnce sync.Once
)

// clipboardCallbackHandle is the LibwaylandHandle receiving clipboard events.
// Set during SetupClipboard. Single-window design — one handle per process.
var clipboardCallbackHandle *LibwaylandHandle

func initClipboardListeners() {
	clipboardListenOnce.Do(func() {
		// wl_data_source events
		dataSourceListener[0] = ffi.NewCallback(dataSourceTargetCb)
		dataSourceListener[1] = ffi.NewCallback(dataSourceSendCb)
		dataSourceListener[2] = ffi.NewCallback(dataSourceCancelledCb)

		// wl_data_device events
		dataDeviceListener[0] = ffi.NewCallback(dataDeviceDataOfferCb)
		dataDeviceListener[1] = ffi.NewCallback(dataDeviceEnterCb)
		dataDeviceListener[2] = ffi.NewCallback(dataDeviceLeaveCb)
		dataDeviceListener[3] = ffi.NewCallback(dataDeviceMotionCb)
		dataDeviceListener[4] = ffi.NewCallback(dataDeviceDropCb)
		dataDeviceListener[5] = ffi.NewCallback(dataDeviceSelectionCb)

		// wl_data_offer events
		dataOfferListener[0] = ffi.NewCallback(dataOfferOfferCb)
	})
}

// --- wl_data_source callbacks ---

// dataSourceTargetCb: void(data, wl_data_source, mime_type)
// Fired when the receiver accepts/rejects a mime type. Ignored for clipboard.
func dataSourceTargetCb(data, source, mimeType uintptr) {
	// No-op for clipboard (DnD only).
}

// dataSourceSendCb: void(data, wl_data_source, mime_type, fd)
// Fired when another application requests our clipboard data.
// We write the stored text to the fd and close it.
func dataSourceSendCb(data, source, mimeType, fd uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		syscall.Close(int(fd))
		return
	}

	h.clipboardMu.Lock()
	text := h.clipboardText
	h.clipboardMu.Unlock()

	// Write text to fd. Use os.NewFile for proper Go I/O handling.
	f := os.NewFile(fd, "clipboard-send")
	if f != nil {
		_, _ = f.WriteString(text)
		f.Close()
	}
}

// dataSourceCancelledCb: void(data, wl_data_source)
// Fired when another application takes ownership of the clipboard.
// We must destroy our data source.
func dataSourceCancelledCb(data, source uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	h.clipboardMu.Lock()
	h.ownsClipboard = false
	// Mark source for deferred destruction (we're inside a callback,
	// calling destroy is safe but we clear the pointer).
	h.clipboardSource = 0
	h.clipboardMu.Unlock()

	// Destroy the source: opcode 1 = wl_data_source.destroy
	h.marshalVoid(source, 1)
	h.proxyDestroy(source)
}

// --- wl_data_device callbacks ---

// dataDeviceDataOfferCb: void(data, wl_data_device, id)
// Fired when the compositor introduces a new data_offer object.
// The id is the proxy for the new wl_data_offer (already created by libwayland).
func dataDeviceDataOfferCb(data, device, id uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	// Add listener on the offer to receive mime type advertisements.
	initClipboardListeners()
	if err := h.addListener(id, uintptr(unsafe.Pointer(&dataOfferListener[0]))); err != nil {
		slog.Warn("wayland: failed to add data_offer listener", "err", err)
	}

	// Store as pending offer — will be confirmed as selection offer
	// in the subsequent selection event.
	h.clipboardMu.Lock()
	h.clipboardPendingOffer = id
	h.clipboardOfferHasText = false
	h.clipboardMu.Unlock()
}

// dataDeviceEnterCb: void(data, wl_data_device, serial, surface, x, y, id)
// DnD enter event — ignored for clipboard.
func dataDeviceEnterCb(data, device, serial, surface, x, y, id uintptr) {
	// No-op for clipboard (DnD only).
}

// dataDeviceLeaveCb: void(data, wl_data_device)
// DnD leave event — ignored for clipboard.
func dataDeviceLeaveCb(data, device uintptr) {
	// No-op for clipboard (DnD only).
}

// dataDeviceMotionCb: void(data, wl_data_device, time, x, y)
// DnD motion event — ignored for clipboard.
func dataDeviceMotionCb(data, device, timeMs, x, y uintptr) {
	// No-op for clipboard (DnD only).
}

// dataDeviceDropCb: void(data, wl_data_device)
// DnD drop event — ignored for clipboard.
func dataDeviceDropCb(data, device uintptr) {
	// No-op for clipboard (DnD only).
}

// dataDeviceSelectionCb: void(data, wl_data_device, id)
// Fired when the clipboard selection changes (another app copied, or we did).
// id is the wl_data_offer for the new selection (or 0 if clipboard cleared).
func dataDeviceSelectionCb(data, device, id uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	h.clipboardMu.Lock()
	defer h.clipboardMu.Unlock()

	// Destroy old offer if we had one (and it's not our own source)
	if h.clipboardOffer != 0 && h.clipboardOffer != id {
		// destroy: opcode 2 on wl_data_offer
		h.marshalVoid(h.clipboardOffer, 2)
		h.proxyDestroy(h.clipboardOffer)
	}

	if id == 0 {
		// Clipboard cleared
		h.clipboardOffer = 0
		h.clipboardOfferHasText = false
		return
	}

	h.clipboardOffer = id
	// clipboardOfferHasText was set by the offer event callbacks
	// that fire between data_offer and selection events.
}

// --- wl_data_offer callbacks ---

// dataOfferOfferCb: void(data, wl_data_offer, mime_type)
// Fired for each mime type the offer advertises.
func dataOfferOfferCb(data, offer, mimeTypePtr uintptr) {
	h := clipboardCallbackHandle
	if h == nil {
		return
	}

	// Read C string using the purego address-dereference pattern to satisfy go vet.
	// go vet forbids direct unsafe.Pointer(uintptr) but allows &local → *unsafe.Pointer.
	mime := cstrToString(mimeTypePtr)
	if mime == clipboardMIME || mime == "text/plain" {
		h.clipboardMu.Lock()
		h.clipboardOfferHasText = true
		h.clipboardMu.Unlock()
	}
}

// cstrToString reads a null-terminated C string from a uintptr.
// Uses the purego address-dereference pattern: takes address of the uintptr,
// reinterprets as *unsafe.Pointer, dereferences. This satisfies go vet because
// it sees unsafe.Pointer(&local), not the forbidden unsafe.Pointer(uintptr_val).
// See: ebitengine/purego func.go, golang/go#58625.
func cstrToString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	p := *(*unsafe.Pointer)(unsafe.Pointer(&ptr))
	const maxLen = 1024
	data := unsafe.Slice((*byte)(p), maxLen)
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}

// --- LibwaylandHandle methods for clipboard ---

// SetupClipboard binds the wl_data_device_manager global and creates
// a wl_data_device for the seat. Must be called after SetupInput (needs seat).
func (h *LibwaylandHandle) SetupClipboard(name, version uint32) error {
	initClipboardInterfaces()
	initClipboardListeners()

	v := version
	if v > 3 {
		v = 3
	}

	// Bind wl_data_device_manager
	mgr, err := h.registryBind(name, unsafe.Pointer(&clipboardInterfaces.manager), v)
	if err != nil {
		return fmt.Errorf("wayland: failed to bind wl_data_device_manager: %w", err)
	}
	h.clipboardMgr = mgr

	// Create wl_data_device for the seat.
	// get_data_device: opcode 1, signature "no" (new_id<wl_data_device>, object<wl_seat>)
	if h.inputSeat == 0 {
		return fmt.Errorf("wayland: cannot create data_device without seat")
	}
	device, err := h.marshalConstructorObj(
		mgr, 1,
		unsafe.Pointer(&clipboardInterfaces.device),
		h.inputSeat,
	)
	if err != nil {
		return fmt.Errorf("wayland: get_data_device failed: %w", err)
	}
	h.clipboardDevice = device

	// Set this handle as the clipboard callback target
	clipboardCallbackHandle = h

	// Add listener for data_device events (data_offer, selection, DnD)
	if err := h.addListener(device, uintptr(unsafe.Pointer(&dataDeviceListener[0]))); err != nil {
		return fmt.Errorf("wayland: failed to add data_device listener: %w", err)
	}

	return nil
}

// ClipboardWrite stores text and announces clipboard ownership to the compositor.
func (h *LibwaylandHandle) ClipboardWrite(text string) error {
	if h.clipboardMgr == 0 || h.clipboardDevice == 0 {
		return fmt.Errorf("wayland: clipboard not initialized")
	}

	initClipboardInterfaces()

	h.clipboardMu.Lock()

	// Destroy old source if we had one
	if h.clipboardSource != 0 {
		oldSource := h.clipboardSource
		h.clipboardSource = 0
		h.clipboardMu.Unlock()
		// destroy: opcode 1 on wl_data_source
		h.marshalVoid(oldSource, 1)
		h.proxyDestroy(oldSource)
		h.clipboardMu.Lock()
	}

	// Store text locally
	h.clipboardText = text
	h.ownsClipboard = true
	h.clipboardMu.Unlock()

	// Create new data source.
	// create_data_source: opcode 0 on wl_data_device_manager, signature "n"
	source, err := h.marshalConstructor(h.clipboardMgr, 0, unsafe.Pointer(&clipboardInterfaces.source))
	if err != nil {
		return fmt.Errorf("wayland: create_data_source failed: %w", err)
	}

	// Add listener for send/canceled events
	if err := h.addListener(source, uintptr(unsafe.Pointer(&dataSourceListener[0]))); err != nil {
		slog.Warn("wayland: failed to add data_source listener", "err", err)
	}

	// Offer our mime type: opcode 0 on wl_data_source, signature "s"
	mimeBuf := []byte(clipboardMIME + "\x00")
	h.marshalVoid(source, 0, uintptr(unsafe.Pointer(&mimeBuf[0])))
	runtime.KeepAlive(mimeBuf)

	// Also offer plain text/plain for broader compatibility
	plainBuf := []byte("text/plain\x00")
	h.marshalVoid(source, 0, uintptr(unsafe.Pointer(&plainBuf[0])))
	runtime.KeepAlive(plainBuf)

	h.clipboardMu.Lock()
	h.clipboardSource = source
	h.clipboardMu.Unlock()

	// set_selection: opcode 1 on wl_data_device, signature "?ou"
	// Args: source (object), serial (uint)
	serial := h.pointerEnterSerial
	h.marshalVoid(h.clipboardDevice, 1, source, uintptr(serial))

	// Flush to ensure the compositor receives our selection
	if err := h.flush(); err != nil {
		slog.Warn("wayland: clipboard flush failed", "err", err)
	}

	return nil
}

// ClipboardRead reads text from the clipboard.
// If we own the clipboard, returns the local copy (avoids pipe deadlock).
// Otherwise, reads from the current selection offer via a pipe.
func (h *LibwaylandHandle) ClipboardRead() (string, error) {
	if h.clipboardDevice == 0 {
		return "", fmt.Errorf("wayland: clipboard not initialized")
	}

	h.clipboardMu.Lock()
	// Fast path: we own the clipboard, return local copy
	if h.ownsClipboard {
		text := h.clipboardText
		h.clipboardMu.Unlock()
		return text, nil
	}

	offer := h.clipboardOffer
	hasText := h.clipboardOfferHasText
	h.clipboardMu.Unlock()

	if offer == 0 || !hasText {
		return "", nil // No text available on clipboard
	}

	// Create a pipe for receiving clipboard data
	var pipeFDs [2]int
	if err := syscall.Pipe2(pipeFDs[:], syscall.O_CLOEXEC); err != nil {
		return "", fmt.Errorf("wayland: pipe2 failed: %w", err)
	}
	readFD := pipeFDs[0]
	writeFD := pipeFDs[1]

	// wl_data_offer.receive: opcode 1, signature "sh"
	// Args: mime_type (string), fd (file descriptor)
	// The fd is passed via SCM_RIGHTS by libwayland when it sees 'h' in the signature.
	mimeBuf := []byte(clipboardMIME + "\x00")
	h.marshalVoid(offer, 1, uintptr(unsafe.Pointer(&mimeBuf[0])), uintptr(writeFD))
	runtime.KeepAlive(mimeBuf)

	// Flush to send the receive request to the compositor
	if err := h.flush(); err != nil {
		syscall.Close(readFD)
		syscall.Close(writeFD)
		return "", fmt.Errorf("wayland: flush failed: %w", err)
	}

	// Close write end — the compositor's data source will write to it
	syscall.Close(writeFD)

	// Read from pipe with timeout (data source writes then closes its end)
	text, err := readPipeWithTimeout(readFD, 5*time.Second)
	syscall.Close(readFD)
	if err != nil {
		return "", fmt.Errorf("wayland: clipboard read failed: %w", err)
	}

	return string(text), nil
}

// DestroyClipboard cleans up clipboard resources.
// Called from Close() during shutdown.
func (h *LibwaylandHandle) DestroyClipboard() {
	h.clipboardMu.Lock()
	source := h.clipboardSource
	h.clipboardSource = 0
	offer := h.clipboardOffer
	h.clipboardOffer = 0
	h.ownsClipboard = false
	h.clipboardMu.Unlock()

	if source != 0 {
		// destroy: opcode 1 on wl_data_source
		h.marshalVoid(source, 1)
		h.proxyDestroy(source)
	}
	if offer != 0 {
		// destroy: opcode 2 on wl_data_offer
		h.marshalVoid(offer, 2)
		h.proxyDestroy(offer)
	}
	if h.clipboardDevice != 0 {
		// release: opcode 2 on wl_data_device
		h.marshalVoid(h.clipboardDevice, 2)
		h.proxyDestroy(h.clipboardDevice)
		h.clipboardDevice = 0
	}
	if h.clipboardMgr != 0 {
		h.proxyDestroy(h.clipboardMgr)
		h.clipboardMgr = 0
	}
}

// HasClipboard returns true if the clipboard subsystem was initialized.
func (h *LibwaylandHandle) HasClipboard() bool {
	return h.clipboardMgr != 0 && h.clipboardDevice != 0
}

// --- Helpers ---

// readPipeWithTimeout reads all data from a file descriptor until EOF or timeout.
// Uses a goroutine with a timer since SetReadDeadline doesn't work on pipes.
func readPipeWithTimeout(fd int, timeout time.Duration) ([]byte, error) {
	f := os.NewFile(uintptr(fd), "clipboard-read")
	if f == nil {
		return nil, fmt.Errorf("invalid fd")
	}
	// Note: we do NOT close f here — the caller closes the raw fd.
	// os.NewFile does not take ownership when used this way.

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
		// Close the file to unblock the reading goroutine.
		// This is safe — the goroutine will get an error and exit.
		f.Close()
		return nil, fmt.Errorf("timeout after %v", timeout)
	}
}
