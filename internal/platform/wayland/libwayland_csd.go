//go:build linux

package wayland

import (
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"golang.org/x/sys/unix"
)

// CSD edge indices for the 4 decoration subsurfaces.
const (
	csdTop    = 0
	csdLeft   = 1
	csdRight  = 2
	csdBottom = 3
)

// SetupCSD creates client-side decoration subsurfaces on the C display.
// This must be called after the main surface and xdg_toplevel are set up.
// subcompName/Version and shmName/Version come from the Pure Go registry.
func (h *LibwaylandHandle) SetupCSD(subcompName, subcompVersion, shmName, shmVersion, seatName, seatVersion uint32, contentW, contentH int, title string, painter CSDPainter, onClose func()) error {
	if painter == nil {
		painter = DefaultCSDPainter{}
	}

	// Bind wl_subcompositor on C display
	subcomp, err := h.registryBind(subcompName, h.subcompositorInterface, 1)
	if err != nil {
		return fmt.Errorf("csd: bind wl_subcompositor: %w", err)
	}
	h.subcompositor = subcomp

	// Bind wl_shm on C display
	shm, err := h.registryBind(shmName, h.shmInterface, 1)
	if err != nil {
		return fmt.Errorf("csd: bind wl_shm: %w", err)
	}
	h.shm = shm

	// Flush to ensure binds reach compositor
	if err := h.flush(); err != nil {
		return fmt.Errorf("csd: flush after bind: %w", err)
	}

	// Roundtrip to get shm format events
	_ = h.roundtrip()

	tbH := painter.TitleBarHeight()
	bW := painter.BorderWidth()
	totalW := contentW + bW*2

	// Create 4 decoration subsurfaces
	specs := [4]struct {
		w, h int
		x, y int32
	}{
		{totalW, tbH, -int32(bW), -int32(tbH)},    // top
		{bW, contentH, -int32(bW), 0},             // left
		{bW, contentH, int32(contentW), 0},        // right
		{totalW, bW, -int32(bW), int32(contentH)}, // bottom
	}

	state := CSDState{Title: title, Focused: true}

	for i, spec := range specs {
		if spec.w <= 0 || spec.h <= 0 {
			continue
		}

		// Create wl_surface
		surf, err := h.marshalConstructor(h.compositor, 0, h.surfaceInterface)
		if err != nil {
			return fmt.Errorf("csd: create surface [%d]: %w", i, err)
		}
		h.csdSurfaces[i] = surf

		// Create wl_subsurface: get_subsurface(new_id, surface, parent)
		// wl_subcompositor::get_subsurface opcode=1, signature "noo"
		subsrf, err := h.marshalConstructor2Obj(h.subcompositor, 1, h.subsurfaceInterface, surf, h.surface)
		if err != nil {
			return fmt.Errorf("csd: get_subsurface [%d]: %w", i, err)
		}
		h.csdSubsurf[i] = subsrf

		// set_position(x, y) — opcode 1 on subsurface
		h.marshalVoid(subsrf, 1, uintptr(uint32(spec.x)), uintptr(uint32(spec.y)))

		// set_sync — opcode 4 on subsurface
		h.marshalVoid(subsrf, 4)

		// Create SHM buffer
		stride := spec.w * 4
		size := stride * spec.h

		fd, err := createShmFD(size)
		if err != nil {
			return fmt.Errorf("csd: create shm fd [%d]: %w", i, err)
		}
		h.csdFDs[i] = fd

		data, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
		if err != nil {
			unix.Close(fd)
			return fmt.Errorf("csd: mmap [%d]: %w", i, err)
		}
		h.csdData[i] = data
		h.csdSizes[i] = [2]int{spec.w, spec.h}

		// wl_shm::create_pool(new_id, fd, size) — opcode 0
		// fd is passed as wl_argument with .h field
		pool, err := h.marshalConstructorFD(h.shm, 0, h.shmPoolInterface, fd, int32(size))
		if err != nil {
			unix.Munmap(data)
			unix.Close(fd)
			return fmt.Errorf("csd: create_pool [%d]: %w", i, err)
		}
		h.csdPools[i] = pool

		// wl_shm_pool::create_buffer(new_id, offset, width, height, stride, format) — opcode 0
		buffer, err := h.marshalConstructorArgs(pool, 0, h.bufferInterface,
			0,               // offset
			uintptr(spec.w), // width
			uintptr(spec.h), // height
			uintptr(stride), // stride
			0,               // format: ARGB8888 = 0
		)
		if err != nil {
			unix.Munmap(data)
			unix.Close(fd)
			return fmt.Errorf("csd: create_buffer [%d]: %w", i, err)
		}
		h.csdBuffers[i] = buffer

		// Paint decoration
		switch i {
		case csdTop:
			painter.PaintTitleBar(data, spec.w, spec.h, state)
		default:
			painter.PaintBorder(data, spec.w, spec.h, CSDEdge(i))
		}

		// Attach buffer, damage, commit
		h.marshalVoid(surf, 1, buffer, 0, 0)                                           // attach(buffer, 0, 0)
		h.marshalVoid(surf, 2, 0, 0, uintptr(uint32(spec.w)), uintptr(uint32(spec.h))) // damage
		h.marshalVoid(surf, 6)                                                         // commit
	}

	// Set window geometry so compositor knows content area (excludes CSD borders).
	h.marshalVoid(h.xdgSurface, 3, 0, 0, uintptr(uint32(contentW)), uintptr(uint32(contentH)))

	// Commit parent surface (subsurfaces apply atomically in sync mode)
	h.marshalVoid(h.surface, 6)

	if err := h.flush(); err != nil {
		return fmt.Errorf("csd: final flush: %w", err)
	}

	h.csdActive = true
	h.csdContentW = contentW
	h.csdContentH = contentH
	h.csdPainter = painter
	h.csdState = state
	h.onCSDClose = onClose
	csdCallbackHandle = h

	// Setup pointer input for CSD hit-testing
	if err := h.setupCSDPointer(seatName, seatVersion); err != nil {
		// Non-fatal: decorations render but aren't interactive
		slog.Warn("CSD pointer setup failed, decorations not interactive", "err", err)
	}

	return nil
}

// CSDActive returns true if client-side decorations are active.
func (h *LibwaylandHandle) CSDActive() bool {
	return h.csdActive
}

// IsMaximized returns true if the window is currently maximized.
func (h *LibwaylandHandle) IsMaximized() bool {
	return h.csdState.Maximized
}

// CSDBorders returns the CSD border dimensions: titleBarHeight, borderWidth.
// Used to subtract CSD borders from configure dimensions (GLFW pattern).
func (h *LibwaylandHandle) CSDBorders() (titleBarH, borderW int) {
	if !h.csdActive || h.csdPainter == nil {
		return 0, 0
	}
	return h.csdPainter.TitleBarHeight(), h.csdPainter.BorderWidth()
}

// DispatchCSDEvents processes pending events on the C display (non-blocking).
// Flushes outgoing requests and dispatches any already-queued events.
// Must be called from the event loop (PollEvents).
// Returns error if the Wayland connection is broken.
func (h *LibwaylandHandle) DispatchCSDEvents() error {
	if !h.csdActive {
		return nil
	}
	if err := h.flush(); err != nil {
		return fmt.Errorf("csd dispatch: flush failed: %w", err)
	}

	// Dispatch CSD event queue (separate from default xdg queue).
	if err := h.flush(); err != nil {
		return fmt.Errorf("csd dispatch: flush failed: %w", err)
	}
	h.dispatchCSDQueue()

	// Process pending repaint (deferred from goffi callbacks to avoid nested FFI).
	if h.csdPendingRepaint {
		h.csdPendingRepaint = false
		h.repaintCSDTitleBar()
	}

	// Process pending actions (deferred from goffi callbacks to avoid nested FFI).
	if h.csdPendingAction != CSDHitNone {
		action := h.csdPendingAction
		serial := h.csdPendingSerial
		h.csdPendingAction = CSDHitNone
		h.csdPendingSerial = 0
		h.processCSDAction(action, serial)
	}
	return nil
}

// setupCSDPointer binds wl_seat on C display and gets wl_pointer with listeners.
func (h *LibwaylandHandle) setupCSDPointer(seatName, seatVersion uint32) error {
	if seatName == 0 {
		return fmt.Errorf("wl_seat not available")
	}

	// Create separate event queue for CSD pointer events.
	// This prevents CSD dispatch from interfering with xdg configure/ping
	// callbacks on the default queue.
	var queue uintptr
	queueArgs := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	ffi.CallFunction(&h.cifCreateQueue, h.fnCreateQueue, unsafe.Pointer(&queue), queueArgs[:])
	if queue == 0 {
		return fmt.Errorf("wl_display_create_queue returned NULL")
	}
	h.csdQueue = queue

	// Bind wl_seat on C display (CSD queue — for pointer events)
	version := seatVersion
	if version > 5 {
		version = 5
	}
	seat, err := h.registryBind(seatName, h.seatInterface, version)
	if err != nil {
		return fmt.Errorf("bind wl_seat: %w", err)
	}
	h.csdSeat = seat

	// Assign seat to CSD queue
	setQueueArgs := [2]unsafe.Pointer{unsafe.Pointer(&seat), unsafe.Pointer(&queue)}
	ffi.CallFunction(&h.cifSetQueue, h.fnProxySetQueue, nil, setQueueArgs[:])

	// Bind SECOND seat on default queue — for move/resize (must be same queue as xdg_toplevel)
	seatDefault, err := h.registryBind(seatName, h.seatInterface, version)
	if err != nil {
		return fmt.Errorf("bind wl_seat (default queue): %w", err)
	}
	h.csdSeatDefault = seatDefault
	// No set_queue — stays on default queue

	// Add seat capabilities listener (required before get_pointer)
	initCSDSeatListeners()
	if err := h.addListener(seat, uintptr(unsafe.Pointer(&csdSeatListener[0]))); err != nil {
		return fmt.Errorf("add seat listener: %w", err)
	}

	// Flush + dispatch CSD queue to get capabilities event
	if err := h.flush(); err != nil {
		return fmt.Errorf("flush after seat bind: %w", err)
	}
	// Roundtrip dispatches default queue — we need to dispatch CSD queue instead.
	// But for init we need capabilities, so do one roundtrip + dispatch CSD queue.
	if err := h.roundtrip(); err != nil {
		return fmt.Errorf("roundtrip for seat capabilities: %w", err)
	}
	h.dispatchCSDQueue()

	// wl_seat::get_pointer (opcode 0) — returns wl_pointer
	pointer, err := h.marshalConstructor(seat, 0, h.pointerInterface)
	if err != nil {
		return fmt.Errorf("get_pointer: %w", err)
	}
	h.csdPointer = pointer

	// Assign pointer to CSD queue
	ptrQueueArgs := [2]unsafe.Pointer{unsafe.Pointer(&pointer), unsafe.Pointer(&queue)}
	ffi.CallFunction(&h.cifSetQueue, h.fnProxySetQueue, nil, ptrQueueArgs[:])

	// Add pointer listener (9 events)
	initCSDPointerListeners()
	if err := h.addListener(pointer, uintptr(unsafe.Pointer(&csdPointerListener[0]))); err != nil {
		return fmt.Errorf("add pointer listener: %w", err)
	}

	if err := h.flush(); err != nil {
		return fmt.Errorf("flush after pointer setup: %w", err)
	}

	return nil
}

// --- CSD Pointer Callbacks (C-callable via goffi) ---

var (
	csdPointerListener [9]uintptr // 9 wl_pointer events
	csdListenersOnce   sync.Once
	csdCallbackHandle  *LibwaylandHandle // global ref for callbacks
)

func initCSDPointerListeners() {
	csdListenersOnce.Do(func() {
		csdPointerListener[0] = ffi.NewCallback(csdPointerEnterCb)        // enter
		csdPointerListener[1] = ffi.NewCallback(csdPointerLeaveCb)        // leave
		csdPointerListener[2] = ffi.NewCallback(csdPointerMotionCb)       // motion
		csdPointerListener[3] = ffi.NewCallback(csdPointerButtonCb)       // button
		csdPointerListener[4] = ffi.NewCallback(csdPointerAxisCb)         // axis
		csdPointerListener[5] = ffi.NewCallback(csdPointerFrameCb)        // frame
		csdPointerListener[6] = ffi.NewCallback(csdPointerAxisSourceCb)   // axis_source
		csdPointerListener[7] = ffi.NewCallback(csdPointerAxisStopCb)     // axis_stop
		csdPointerListener[8] = ffi.NewCallback(csdPointerAxisDiscreteCb) // axis_discrete
	})
}

// csdPointerEnterCb: void(data, wl_pointer, serial, surface, sx_fixed, sy_fixed)
func csdPointerEnterCb(data, pointer, serial, surface, sxFixed, syFixed uintptr) {
	h := csdCallbackHandle
	if h == nil {
		return
	}
	h.csdPointerSurface = surface
	h.csdSerial = uint32(serial)
	// Convert fixed-point 24.8 to float
	h.csdPointerX = float64(int32(sxFixed)) / 256.0
	h.csdPointerY = float64(int32(syFixed)) / 256.0
	h.updateCSDHitTest()
}

// csdPointerLeaveCb: void(data, wl_pointer, serial, surface)
func csdPointerLeaveCb(data, pointer, serial, surface uintptr) {
	h := csdCallbackHandle
	if h == nil {
		return
	}
	h.csdPointerSurface = 0
	h.csdHitResult = CSDHitNone
	// Clear all hover states and repaint
	h.csdState.Close.Hovered = false
	h.csdState.Maximize.Hovered = false
	h.csdState.Minimize.Hovered = false
	h.csdPendingRepaint = true
}

// csdPointerMotionCb: void(data, wl_pointer, time, sx_fixed, sy_fixed)
func csdPointerMotionCb(data, pointer, time, sxFixed, syFixed uintptr) {
	h := csdCallbackHandle
	if h == nil {
		return
	}
	h.csdPointerX = float64(int32(sxFixed)) / 256.0
	h.csdPointerY = float64(int32(syFixed)) / 256.0
	h.updateCSDHitTest()
}

// csdPointerButtonCb: void(data, wl_pointer, serial, time, button, state)
func csdPointerButtonCb(data, pointer, serial, time, button, state uintptr) {
	h := csdCallbackHandle
	if h == nil {
		return
	}
	h.csdSerial = uint32(serial)

	// state: 0 = released, 1 = pressed
	if state != 1 {
		return // only handle press
	}

	// BTN_LEFT = 0x110
	if button != 0x110 {
		return
	}

	// Don't call marshalVoid from inside goffi callback — nested FFI calls segfault.
	// Store pending action; process it in DispatchCSDEvents (outside callback context).
	h.csdPendingAction = h.csdHitResult
	h.csdPendingSerial = h.csdSerial
}

// --- CSD Seat Callbacks ---

var (
	csdSeatListener [2]uintptr // capabilities, name
	csdSeatListOnce sync.Once
)

func initCSDSeatListeners() {
	csdSeatListOnce.Do(func() {
		csdSeatListener[0] = ffi.NewCallback(csdSeatCapabilitiesCb) // capabilities
		csdSeatListener[1] = ffi.NewCallback(csdSeatNameCb)         // name
	})
}

// csdSeatCapabilitiesCb: void(data, wl_seat, capabilities)
func csdSeatCapabilitiesCb(data, seat, capabilities uintptr) {
	// WL_SEAT_CAPABILITY_POINTER = 1
	// We just need to know it exists — we already call get_pointer unconditionally
}

// csdSeatNameCb: void(data, wl_seat, name)
func csdSeatNameCb(data, seat, name uintptr) {
	// No-op — we don't need the seat name
}

// No-op callbacks for unused pointer events (goffi requires fixed args, no variadic).
func csdPointerAxisCb(data, pointer, time, axis, value uintptr)      {}
func csdPointerFrameCb(data, pointer uintptr)                        {}
func csdPointerAxisSourceCb(data, pointer, source uintptr)           {}
func csdPointerAxisStopCb(data, pointer, time, axis uintptr)         {}
func csdPointerAxisDiscreteCb(data, pointer, axis, discrete uintptr) {}

// updateCSDHitTest recalculates hit-test based on current pointer position and surface.
func (h *LibwaylandHandle) updateCSDHitTest() {
	oldHit := h.csdHitResult

	// Match surface to CSD edge
	switch h.csdPointerSurface {
	case h.csdSurfaces[csdTop]:
		h.csdHitResult = h.csdPainter.HitTestTitleBar(
			int(h.csdPointerX), int(h.csdPointerY),
			h.csdSizes[csdTop][0], h.csdSizes[csdTop][1])
	case h.csdSurfaces[csdLeft]:
		h.csdHitResult = CSDHitResizeW
	case h.csdSurfaces[csdRight]:
		h.csdHitResult = CSDHitResizeE
	case h.csdSurfaces[csdBottom]:
		h.csdHitResult = CSDHitResizeS
	default:
		h.csdHitResult = CSDHitNone
	}

	// Update hover states if changed
	if oldHit != h.csdHitResult {
		h.csdState.Close.Hovered = h.csdHitResult == CSDHitClose
		h.csdState.Maximize.Hovered = h.csdHitResult == CSDHitMaximize
		h.csdState.Minimize.Hovered = h.csdHitResult == CSDHitMinimize
		h.csdPendingRepaint = true
	}
}

// repaintCSDTitleBar repaints the title bar SHM buffer and commits.
func (h *LibwaylandHandle) repaintCSDTitleBar() {
	if !h.csdActive || h.csdData[csdTop] == nil {
		return
	}
	h.csdPainter.PaintTitleBar(h.csdData[csdTop], h.csdSizes[csdTop][0], h.csdSizes[csdTop][1], h.csdState)
	// Attach + damage + commit
	surf := h.csdSurfaces[csdTop]
	buf := h.csdBuffers[csdTop]
	w, ht := h.csdSizes[csdTop][0], h.csdSizes[csdTop][1]
	h.marshalVoid(surf, 1, buf, 0, 0)                                     // attach
	h.marshalVoid(surf, 2, 0, 0, uintptr(uint32(w)), uintptr(uint32(ht))) // damage
	h.marshalVoid(surf, 6)                                                // commit
	_ = h.flush()
}

// ResizeCSD recreates SHM buffers for all 4 CSD subsurfaces when the content
// area dimensions change (e.g. maximize, compositor-driven resize).
// Must be called from the event loop, not from inside a goffi callback.
func (h *LibwaylandHandle) ResizeCSD(contentW, contentH int) { //nolint:gocognit // CSD resize is inherently complex
	if !h.csdActive || h.csdPainter == nil {
		return
	}
	if contentW == h.csdContentW && contentH == h.csdContentH {
		slog.Warn("CSD-DEBUG: ResizeCSD skip — same size", "w", contentW, "h", contentH)
		return
	}
	slog.Warn("CSD-DEBUG: ResizeCSD", "newW", contentW, "newH", contentH, "oldW", h.csdContentW, "oldH", h.csdContentH)

	h.csdContentW = contentW
	h.csdContentH = contentH

	tbH := h.csdPainter.TitleBarHeight()
	bW := h.csdPainter.BorderWidth()
	totalW := contentW + bW*2

	// New dimensions and positions for each subsurface.
	// When maximized, title bar goes inside window (0,0) — borders hidden off-screen.
	// When normal, title bar at (-borderW, -titleBarH) with borders around content.
	maximized := h.csdState.Maximized
	specs := [4]struct {
		w, h int
		x, y int32
	}{
		{totalW, tbH, -int32(bW), -int32(tbH)},    // top (normal)
		{bW, contentH, -int32(bW), 0},             // left
		{bW, contentH, int32(contentW), 0},        // right
		{totalW, bW, -int32(bW), int32(contentH)}, // bottom
	}
	if maximized {
		// Title bar inside window, full configure width, no borders
		specs[0] = struct {
			w, h int
			x, y int32
		}{contentW, tbH, 0, 0}
		specs[1] = struct {
			w, h int
			x, y int32
		}{0, 0, 0, 0} // hidden
		specs[2] = struct {
			w, h int
			x, y int32
		}{0, 0, 0, 0} // hidden
		specs[3] = struct {
			w, h int
			x, y int32
		}{0, 0, 0, 0} // hidden
	}

	state := h.csdState

	for i, spec := range specs {
		if h.csdSurfaces[i] == 0 {
			continue
		}
		// Hide subsurface: destroy wl_subsurface (immediate per spec).
		// GLFW pattern: destroy on maximize, recreate on restore.
		if spec.w <= 0 || spec.h <= 0 {
			if h.csdSubsurf[i] != 0 {
				h.marshalVoid(h.csdSubsurf[i], 0) // wl_subsurface.destroy = opcode 0
				h.csdSubsurf[i] = 0
			}
			continue
		}
		if h.csdSurfaces[i] == 0 {
			continue
		}

		// Recreate wl_subsurface if it was destroyed (maximize → restore)
		if h.csdSubsurf[i] == 0 {
			subsrf, err := h.marshalConstructor2Obj(h.subcompositor, 1, h.subsurfaceInterface, h.csdSurfaces[i], h.surface)
			if err != nil {
				slog.Warn("CSD resize: recreate subsurface failed", "edge", i, "err", err)
				continue
			}
			h.csdSubsurf[i] = subsrf
			// Sync mode is default for new subsurfaces
		}

		// Save old resources for cleanup AFTER new buffer is attached
		oldBuffer := h.csdBuffers[i]
		oldPool := h.csdPools[i]
		oldData := h.csdData[i]
		h.csdBuffers[i] = 0
		h.csdPools[i] = 0
		h.csdData[i] = nil

		// Create new SHM fd (don't reuse — compositor may cache old fd mapping)
		stride := spec.w * 4
		size := stride * spec.h

		// Close old fd
		if h.csdFDs[i] >= 0 {
			unix.Close(h.csdFDs[i])
			h.csdFDs[i] = -1
		}

		newFD, err := createShmFD(size)
		if err != nil {
			slog.Warn("CSD resize: createShmFD failed", "edge", i, "err", err)
			continue
		}
		h.csdFDs[i] = newFD

		// Mmap new region
		data, err := unix.Mmap(newFD, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
		if err != nil {
			slog.Warn("CSD resize: mmap failed", "edge", i, "err", err)
			continue
		}
		h.csdData[i] = data
		h.csdSizes[i] = [2]int{spec.w, spec.h}

		// Create new pool + buffer
		pool, err := h.marshalConstructorFD(h.shm, 0, h.shmPoolInterface, h.csdFDs[i], int32(size))
		if err != nil {
			slog.Warn("CSD resize: create_pool failed", "edge", i, "err", err)
			continue
		}
		h.csdPools[i] = pool
		slog.Warn("CSD-RESIZE: pool created", "edge", i, "pool", pool, "size", size)

		buffer, err := h.marshalConstructorArgs(pool, 0, h.bufferInterface,
			0,               // offset
			uintptr(spec.w), // width
			uintptr(spec.h), // height
			uintptr(stride), // stride
			0,               // format: ARGB8888 = 0
		)
		if err != nil {
			slog.Warn("CSD resize: create_buffer failed", "edge", i, "err", err)
			continue
		}
		h.csdBuffers[i] = buffer
		slog.Warn("CSD-RESIZE: buffer created", "edge", i, "buffer", buffer, "w", spec.w, "h", spec.h)

		// Paint content
		switch i {
		case csdTop:
			h.csdPainter.PaintTitleBar(data, spec.w, spec.h, state)
			slog.Warn("CSD-RESIZE: painted title bar", "w", spec.w, "h", spec.h)
		default:
			h.csdPainter.PaintBorder(data, spec.w, spec.h, CSDEdge(i))
			slog.Warn("CSD-RESIZE: painted border", "edge", i, "w", spec.w, "h", spec.h)
		}

		// Update position for all subsurfaces that moved
		h.marshalVoid(h.csdSubsurf[i], 1, uintptr(uint32(spec.x)), uintptr(uint32(spec.y)))
		slog.Warn("CSD-RESIZE: set_position", "edge", i, "x", spec.x, "y", spec.y)

		// Attach new buffer + damage + commit
		surf := h.csdSurfaces[i]
		h.marshalVoid(surf, 1, buffer, 0, 0)                                           // attach(buffer, 0, 0)
		h.marshalVoid(surf, 2, 0, 0, uintptr(uint32(spec.w)), uintptr(uint32(spec.h))) // damage
		h.marshalVoid(surf, 6)                                                         // commit
		slog.Warn("CSD-RESIZE: subsurface committed", "edge", i, "surf", surf)

		// Now destroy old resources (AFTER new buffer attached)
		if oldBuffer != 0 {
			h.marshalVoid(oldBuffer, 0) // wl_buffer.destroy
		}
		if oldPool != 0 {
			h.marshalVoid(oldPool, 1) // wl_shm_pool.destroy
		}
		if oldData != nil {
			unix.Munmap(oldData)
		}
	}

	// Force parent surface to have pending damage so commit triggers subsurface apply.
	// WSLg (Weston) may optimize away empty parent commits.
	h.marshalVoid(h.surface, 2, 0, 0, 1, 1) // damage 1x1 on parent
	h.marshalVoid(h.surface, 6)             // commit parent
	slog.Warn("CSD-RESIZE: parent committed", "surface", h.surface)

	if err := h.flush(); err != nil {
		slog.Warn("CSD resize: flush failed", "err", err)
	} else {
		slog.Warn("CSD-RESIZE: flush OK")
	}
}

// csdHitToResizeEdge maps CSDHitResult to xdg_toplevel resize edge enum.
func csdHitToResizeEdge(hit CSDHitResult) uint32 {
	// xdg_toplevel resize_edge values:
	// none=0, top=1, bottom=2, left=4, top_left=5, bottom_left=6, right=8, top_right=9, bottom_right=10
	switch hit {
	case CSDHitResizeN:
		return 1
	case CSDHitResizeS:
		return 2
	case CSDHitResizeW:
		return 4
	case CSDHitResizeNW:
		return 5
	case CSDHitResizeSW:
		return 6
	case CSDHitResizeE:
		return 8
	case CSDHitResizeNE:
		return 9
	case CSDHitResizeSE:
		return 10
	default:
		return 0
	}
}

// marshalConstructor2Obj creates a new proxy with 2 object arguments.
// Signature "noo" — new_id + object + object.
// Used for wl_subcompositor::get_subsurface(new_id, surface, parent).
func (h *LibwaylandHandle) marshalConstructor2Obj(proxy uintptr, opcode uint32, iface unsafe.Pointer, obj1, obj2 uintptr) (uintptr, error) {
	var argBuf [3]uintptr
	argBuf[0] = 0    // new_id placeholder
	argBuf[1] = obj1 // first object arg
	argBuf[2] = obj2 // second object arg
	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(iface)

	var result uintptr
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&proxy),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
	}
	_ = ffi.CallFunction(&h.cifMarshal, h.fnProxyMarshal, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("marshalConstructor2Obj returned NULL (opcode %d)", opcode)
	}
	return result, nil
}

// marshalConstructorFD creates a new proxy with fd + int32 arguments.
// Used for wl_shm::create_pool(new_id, fd, size).
// In wl_argument, fd is stored in the .h field (int32_t).
func (h *LibwaylandHandle) marshalConstructorFD(proxy uintptr, opcode uint32, iface unsafe.Pointer, fd int, size int32) (uintptr, error) {
	var argBuf [3]uintptr
	argBuf[0] = 0                   // new_id placeholder
	argBuf[1] = uintptr(uint32(fd)) // fd in wl_argument.h (int32)
	argBuf[2] = uintptr(uint32(size))
	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(iface)

	var result uintptr
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&proxy),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
	}
	_ = ffi.CallFunction(&h.cifMarshal, h.fnProxyMarshal, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("marshalConstructorFD returned NULL (opcode %d)", opcode)
	}
	return result, nil
}

// marshalConstructorArgs creates a new proxy with multiple int/uint arguments.
// Used for wl_shm_pool::create_buffer(new_id, offset, width, height, stride, format).
func (h *LibwaylandHandle) marshalConstructorArgs(proxy uintptr, opcode uint32, iface unsafe.Pointer, args ...uintptr) (uintptr, error) {
	var argBuf [8]uintptr
	argBuf[0] = 0 // new_id placeholder
	copy(argBuf[1:], args)
	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(iface)

	var result uintptr
	ffiArgs := [4]unsafe.Pointer{
		unsafe.Pointer(&proxy),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
	}
	_ = ffi.CallFunction(&h.cifMarshal, h.fnProxyMarshal, unsafe.Pointer(&result), ffiArgs[:])
	if result == 0 {
		return 0, fmt.Errorf("marshalConstructorArgs returned NULL (opcode %d)", opcode)
	}
	return result, nil
}

// processCSDAction executes a deferred CSD action outside of goffi callback context.
func (h *LibwaylandHandle) processCSDAction(action CSDHitResult, serial uint32) {
	slog.Info("CSD action", "action", action, "serial", serial, "seat", h.csdSeat, "toplevel", h.xdgToplevel)
	switch action {
	case CSDHitCaption:
		// Use csdSeat (same seat that generated the serial) for move
		if h.xdgToplevel != 0 && h.csdSeat != 0 && serial != 0 {
			slog.Info("CSD: sending xdg_toplevel.move", "toplevel", h.xdgToplevel, "seat", h.csdSeat, "serial", serial)
			h.marshalVoid(h.xdgToplevel, 5, h.csdSeat, uintptr(serial))
			slog.Info("CSD: marshalVoid returned, flushing")
			if err := h.flush(); err != nil {
				slog.Error("CSD move flush failed", "err", err)
			} else {
				slog.Info("CSD: move flushed successfully")
			}
		}
	case CSDHitClose:
		if h.onCSDClose != nil {
			h.onCSDClose()
		}
	case CSDHitMinimize:
		if h.xdgToplevel != 0 {
			h.marshalVoid(h.xdgToplevel, 13)
			if err := h.flush(); err != nil {
				slog.Warn("CSD minimize flush failed", "err", err)
			}
		}
	case CSDHitMaximize:
		if h.xdgToplevel != 0 {
			if h.csdState.Maximized {
				h.marshalVoid(h.xdgToplevel, 10)
			} else {
				h.marshalVoid(h.xdgToplevel, 9)
			}
			if err := h.flush(); err != nil {
				slog.Warn("CSD maximize flush failed", "err", err)
			}
		}
	case CSDHitResizeN, CSDHitResizeS, CSDHitResizeW, CSDHitResizeE,
		CSDHitResizeNW, CSDHitResizeNE, CSDHitResizeSW, CSDHitResizeSE:
		if h.xdgToplevel != 0 && h.csdSeat != 0 && serial != 0 {
			edge := csdHitToResizeEdge(action)
			h.marshalVoid(h.xdgToplevel, 6, h.csdSeat, uintptr(serial), uintptr(edge))
			if err := h.flush(); err != nil {
				slog.Warn("CSD resize flush failed", "err", err)
			}
		}
	}
}

// dispatchCSDQueue dispatches pending events on the CSD event queue (non-blocking).
func (h *LibwaylandHandle) dispatchCSDQueue() {
	if h.csdQueue == 0 {
		return
	}
	var result int32
	args := [2]unsafe.Pointer{unsafe.Pointer(&h.display), unsafe.Pointer(&h.csdQueue)}
	ffi.CallFunction(&h.cifDispatchQP, h.fnDispatchQueueP, unsafe.Pointer(&result), args[:])
}

// getDisplayFD returns the file descriptor for the C display connection.
// Uses wl_display_get_fd(display) -> int.
func (h *LibwaylandHandle) getDisplayFD() int {
	if h.fnGetFD == nil {
		return -1
	}
	var result int32
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	ffi.CallFunction(&h.cifRoundtrip, h.fnGetFD, unsafe.Pointer(&result), args[:])
	// cifRoundtrip has same signature: int(ptr) — reuse it for get_fd
	return int(result)
}

// socketHasData checks if a file descriptor has data ready for reading (non-blocking poll).
func socketHasData(fd int) bool {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 0) // timeout=0 = non-blocking
	return err == nil && n > 0 && fds[0].Revents&unix.POLLIN != 0
}

// roundtrip is defined in libwayland_xdg.go
