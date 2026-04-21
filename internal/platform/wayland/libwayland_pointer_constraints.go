//go:build linux

package wayland

// libwayland_pointer_constraints.go — Pointer constraints and relative pointer protocols.
//
// Implements zwp_pointer_constraints_v1 (lock/confine pointer to surface) and
// zwp_relative_pointer_v1 (relative motion events) for cursor grab / pointer lock.
//
// These protocols are required for FPS-style mouse look (CursorModeLocked) and
// window-confined cursor (CursorModeConfined) on Wayland compositors.
//
// Uses the same C-compatible interface descriptor pattern as libwayland_xdg.go.
// Callbacks via goffi for locked/unlocked and relative_motion events.

import (
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// ptrConstraints holds C-compatible interface descriptors for pointer constraints
// and relative pointer protocols. Constructed once, live for program lifetime.
var ptrConstraints struct {
	once sync.Once

	// Interface descriptors
	constraintsMgr  cWlInterface // zwp_pointer_constraints_v1
	lockedPointer   cWlInterface // zwp_locked_pointer_v1
	confinedPointer cWlInterface // zwp_confined_pointer_v1
	relPointerMgr   cWlInterface // zwp_relative_pointer_manager_v1
	relPointer      cWlInterface // zwp_relative_pointer_v1

	// Method arrays (indexed by opcode)
	constraintsMgrMethods  [3]cWlMessage // destroy, lock_pointer, confine_pointer
	lockedPointerMethods   [3]cWlMessage // destroy, set_cursor_position_hint, set_region
	confinedPointerMethods [2]cWlMessage // destroy, set_region
	relPointerMgrMethods   [2]cWlMessage // destroy, get_relative_pointer
	relPointerMethods      [1]cWlMessage // destroy

	// Event arrays
	lockedPointerEvents   [2]cWlMessage // locked, unlocked
	confinedPointerEvents [2]cWlMessage // confined, unconfined
	relPointerEvents      [1]cWlMessage // relative_motion

	// NULL types array (shared by all messages)
	nullTypes [8]uintptr
}

// initPointerConstraintInterfaces constructs C-compatible interface descriptors
// for zwp_pointer_constraints_v1 and zwp_relative_pointer_v1 protocols.
// Called once during the first pointer constraints setup.
func initPointerConstraintInterfaces() {
	ptrConstraints.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&ptrConstraints.nullTypes[0]))

		// === zwp_pointer_constraints_v1 methods ===
		// opcode 0: destroy (no args)
		ptrConstraints.constraintsMgrMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		// opcode 1: lock_pointer(new_id, surface, pointer, region_or_null, lifetime)
		// signature: "noo?ou"
		ptrConstraints.constraintsMgrMethods[1] = cWlMessage{cstr("lock_pointer\x00"), cstr("noo?ou\x00"), nt}
		// opcode 2: confine_pointer(new_id, surface, pointer, region_or_null, lifetime)
		// signature: "noo?ou"
		ptrConstraints.constraintsMgrMethods[2] = cWlMessage{cstr("confine_pointer\x00"), cstr("noo?ou\x00"), nt}

		// zwp_pointer_constraints_v1 interface (no events)
		ptrConstraints.constraintsMgr = cWlInterface{
			Name:        cstr("zwp_pointer_constraints_v1\x00"),
			Version:     1,
			MethodCount: 3,
			Methods:     uintptr(unsafe.Pointer(&ptrConstraints.constraintsMgrMethods[0])),
			EventCount:  0,
			Events:      0,
		}

		// === zwp_locked_pointer_v1 methods ===
		// opcode 0: destroy
		ptrConstraints.lockedPointerMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		// opcode 1: set_cursor_position_hint(x: fixed, y: fixed)
		ptrConstraints.lockedPointerMethods[1] = cWlMessage{cstr("set_cursor_position_hint\x00"), cstr("ff\x00"), nt}
		// opcode 2: set_region(region: object or null)
		ptrConstraints.lockedPointerMethods[2] = cWlMessage{cstr("set_region\x00"), cstr("?o\x00"), nt}

		// zwp_locked_pointer_v1 events
		// event 0: locked (no args)
		ptrConstraints.lockedPointerEvents[0] = cWlMessage{cstr("locked\x00"), cstr("\x00"), nt}
		// event 1: unlocked (no args)
		ptrConstraints.lockedPointerEvents[1] = cWlMessage{cstr("unlocked\x00"), cstr("\x00"), nt}

		// zwp_locked_pointer_v1 interface
		ptrConstraints.lockedPointer = cWlInterface{
			Name:        cstr("zwp_locked_pointer_v1\x00"),
			Version:     1,
			MethodCount: 3,
			Methods:     uintptr(unsafe.Pointer(&ptrConstraints.lockedPointerMethods[0])),
			EventCount:  2,
			Events:      uintptr(unsafe.Pointer(&ptrConstraints.lockedPointerEvents[0])),
		}

		// === zwp_confined_pointer_v1 methods ===
		// opcode 0: destroy
		ptrConstraints.confinedPointerMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		// opcode 1: set_region(region: object or null)
		ptrConstraints.confinedPointerMethods[1] = cWlMessage{cstr("set_region\x00"), cstr("?o\x00"), nt}

		// zwp_confined_pointer_v1 events
		// event 0: confined (no args)
		ptrConstraints.confinedPointerEvents[0] = cWlMessage{cstr("confined\x00"), cstr("\x00"), nt}
		// event 1: unconfined (no args)
		ptrConstraints.confinedPointerEvents[1] = cWlMessage{cstr("unconfined\x00"), cstr("\x00"), nt}

		// zwp_confined_pointer_v1 interface
		ptrConstraints.confinedPointer = cWlInterface{
			Name:        cstr("zwp_confined_pointer_v1\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&ptrConstraints.confinedPointerMethods[0])),
			EventCount:  2,
			Events:      uintptr(unsafe.Pointer(&ptrConstraints.confinedPointerEvents[0])),
		}

		// === zwp_relative_pointer_manager_v1 methods ===
		// opcode 0: destroy
		ptrConstraints.relPointerMgrMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		// opcode 1: get_relative_pointer(new_id, pointer)
		ptrConstraints.relPointerMgrMethods[1] = cWlMessage{cstr("get_relative_pointer\x00"), cstr("no\x00"), nt}

		// zwp_relative_pointer_manager_v1 interface (no events)
		ptrConstraints.relPointerMgr = cWlInterface{
			Name:        cstr("zwp_relative_pointer_manager_v1\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&ptrConstraints.relPointerMgrMethods[0])),
			EventCount:  0,
			Events:      0,
		}

		// === zwp_relative_pointer_v1 methods ===
		// opcode 0: destroy
		ptrConstraints.relPointerMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}

		// zwp_relative_pointer_v1 events
		// event 0: relative_motion(uhi, ulo, dx_fixed, dy_fixed, dx_unaccel_fixed, dy_unaccel_fixed)
		// signature: "uuffff" — two uint32 for timestamp, four wl_fixed_t for deltas
		ptrConstraints.relPointerEvents[0] = cWlMessage{cstr("relative_motion\x00"), cstr("uuffff\x00"), nt}

		// zwp_relative_pointer_v1 interface
		ptrConstraints.relPointer = cWlInterface{
			Name:        cstr("zwp_relative_pointer_v1\x00"),
			Version:     1,
			MethodCount: 1,
			Methods:     uintptr(unsafe.Pointer(&ptrConstraints.relPointerMethods[0])),
			EventCount:  1,
			Events:      uintptr(unsafe.Pointer(&ptrConstraints.relPointerEvents[0])),
		}
	})
}

// --- Listener arrays and callbacks ---

var (
	lockedPointerListener      [2]uintptr // locked, unlocked
	confinedPointerListener    [2]uintptr // confined, unconfined
	relPointerListener         [1]uintptr // relative_motion
	ptrConstraintListenersOnce sync.Once
)

func initPointerConstraintListeners() {
	ptrConstraintListenersOnce.Do(func() {
		lockedPointerListener[0] = ffi.NewCallback(lockedPointerLockedCb)
		lockedPointerListener[1] = ffi.NewCallback(lockedPointerUnlockedCb)
		confinedPointerListener[0] = ffi.NewCallback(confinedPointerConfinedCb)
		confinedPointerListener[1] = ffi.NewCallback(confinedPointerUnconfinedCb)
		relPointerListener[0] = ffi.NewCallback(relativePointerMotionCb)
	})
}

// lockedPointerLockedCb: void(data, zwp_locked_pointer_v1)
// Fired when the compositor activates the pointer lock.
func lockedPointerLockedCb(data, lockedPointer uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	slog.Debug("wayland: pointer locked")
	if h.inputCallbacks.OnLockedPointerLocked != nil {
		h.inputCallbacks.OnLockedPointerLocked()
	}
}

// lockedPointerUnlockedCb: void(data, zwp_locked_pointer_v1)
// Fired when the compositor deactivates the pointer lock (e.g., alt-tab).
func lockedPointerUnlockedCb(data, lockedPointer uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	slog.Debug("wayland: pointer unlocked")
	if h.inputCallbacks.OnLockedPointerUnlocked != nil {
		h.inputCallbacks.OnLockedPointerUnlocked()
	}
}

// confinedPointerConfinedCb: void(data, zwp_confined_pointer_v1)
func confinedPointerConfinedCb(data, confinedPointer uintptr) {
	slog.Debug("wayland: pointer confined")
}

// confinedPointerUnconfinedCb: void(data, zwp_confined_pointer_v1)
func confinedPointerUnconfinedCb(data, confinedPointer uintptr) {
	slog.Debug("wayland: pointer unconfined")
}

// relativePointerMotionCb: void(data, zwp_relative_pointer_v1, uhi, ulo, dx, dy, dx_unaccel, dy_unaccel)
// Arguments are delivered as uintptr: two uint32 for timestamp, four wl_fixed_t (24.8) for deltas.
func relativePointerMotionCb(data, relPointer, uhiArg, uloArg, dxFixed, dyFixed, dxUnaccelFixed, dyUnaccelFixed uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}

	// Reconstruct 64-bit timestamp from high and low uint32 halves.
	timeHi := uint64(uint32(uhiArg))
	timeLo := uint64(uint32(uloArg))
	timeUs := (timeHi << 32) | timeLo

	// Convert wl_fixed_t (24.8 signed fixed-point) to float64.
	dx := float64(int32(dxFixed)) / 256.0
	dy := float64(int32(dyFixed)) / 256.0
	dxUnaccel := float64(int32(dxUnaccelFixed)) / 256.0
	dyUnaccel := float64(int32(dyUnaccelFixed)) / 256.0

	if h.inputCallbacks.OnRelativePointerMotion != nil {
		h.inputCallbacks.OnRelativePointerMotion(timeUs, dx, dy, dxUnaccel, dyUnaccel)
	}
}

// --- LibwaylandHandle methods for pointer constraints ---

// SetupPointerConstraints binds the zwp_pointer_constraints_v1 global.
// Called during Init if the compositor advertises this protocol.
func (h *LibwaylandHandle) SetupPointerConstraints(name, version uint32) error {
	initPointerConstraintInterfaces()

	v := version
	if v > 1 {
		v = 1
	}

	mgr, err := h.registryBind(name, unsafe.Pointer(&ptrConstraints.constraintsMgr), v)
	if err != nil {
		return fmt.Errorf("wayland: failed to bind zwp_pointer_constraints_v1: %w", err)
	}
	h.pointerConstraintsMgr = mgr
	return nil
}

// SetupRelativePointerManager binds the zwp_relative_pointer_manager_v1 global.
// Called during Init if the compositor advertises this protocol.
func (h *LibwaylandHandle) SetupRelativePointerManager(name, version uint32) error {
	initPointerConstraintInterfaces()

	v := version
	if v > 1 {
		v = 1
	}

	mgr, err := h.registryBind(name, unsafe.Pointer(&ptrConstraints.relPointerMgr), v)
	if err != nil {
		return fmt.Errorf("wayland: failed to bind zwp_relative_pointer_manager_v1: %w", err)
	}
	h.relativePointerMgr = mgr
	return nil
}

// HasPointerConstraints returns true if the compositor supports pointer constraints.
func (h *LibwaylandHandle) HasPointerConstraints() bool {
	return h.pointerConstraintsMgr != 0
}

// HasRelativePointerManager returns true if the compositor supports relative pointer.
func (h *LibwaylandHandle) HasRelativePointerManager() bool {
	return h.relativePointerMgr != 0
}

// LockPointer creates a zwp_locked_pointer_v1, locking the pointer to the surface.
// lifetime: 0 = oneshot (unlocks on deactivation), 1 = persistent (re-locks on focus).
// Destroys any existing locked pointer first.
func (h *LibwaylandHandle) LockPointer(surface, pointer uintptr, lifetime uint32) error {
	if h.pointerConstraintsMgr == 0 {
		return fmt.Errorf("wayland: pointer constraints manager not available")
	}

	// Destroy existing lock if any
	h.DestroyLockedPointer()

	initPointerConstraintListeners()

	// lock_pointer: opcode 1 on zwp_pointer_constraints_v1
	// signature "noo?ou": new_id, surface, pointer, region(null), lifetime
	var argBuf [5]uintptr
	argBuf[0] = 0       // new_id placeholder
	argBuf[1] = surface // wl_surface
	argBuf[2] = pointer // wl_pointer
	argBuf[3] = 0       // region = NULL (lock entire surface)
	argBuf[4] = uintptr(lifetime)

	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(unsafe.Pointer(&ptrConstraints.lockedPointer))
	opcode := uint32(1)

	var result uintptr
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&h.pointerConstraintsMgr),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
	}
	_ = ffi.CallFunction(&h.cifMarshal, h.fnProxyMarshal, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return fmt.Errorf("wayland: lock_pointer returned NULL")
	}
	h.lockedPointer = result

	// Add listener for locked/unlocked events
	if err := h.addListener(result, uintptr(unsafe.Pointer(&lockedPointerListener[0]))); err != nil {
		slog.Warn("wayland: failed to add locked_pointer listener", "err", err)
	}

	return nil
}

// ConfinePointer creates a zwp_confined_pointer_v1, confining the pointer to the surface.
// lifetime: 0 = oneshot, 1 = persistent.
// Destroys any existing confined pointer first.
func (h *LibwaylandHandle) ConfinePointer(surface, pointer uintptr, lifetime uint32) error {
	if h.pointerConstraintsMgr == 0 {
		return fmt.Errorf("wayland: pointer constraints manager not available")
	}

	// Destroy existing confinement if any
	h.DestroyConfinedPointer()

	initPointerConstraintListeners()

	// confine_pointer: opcode 2 on zwp_pointer_constraints_v1
	// signature "noo?ou": new_id, surface, pointer, region(null), lifetime
	var argBuf [5]uintptr
	argBuf[0] = 0       // new_id placeholder
	argBuf[1] = surface // wl_surface
	argBuf[2] = pointer // wl_pointer
	argBuf[3] = 0       // region = NULL (confine to entire surface)
	argBuf[4] = uintptr(lifetime)

	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(unsafe.Pointer(&ptrConstraints.confinedPointer))
	opcode := uint32(2)

	var result uintptr
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&h.pointerConstraintsMgr),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
	}
	_ = ffi.CallFunction(&h.cifMarshal, h.fnProxyMarshal, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return fmt.Errorf("wayland: confine_pointer returned NULL")
	}
	h.confinedPointer = result

	// Add listener for confined/unconfined events
	if err := h.addListener(result, uintptr(unsafe.Pointer(&confinedPointerListener[0]))); err != nil {
		slog.Warn("wayland: failed to add confined_pointer listener", "err", err)
	}

	return nil
}

// GetRelativePointer creates a zwp_relative_pointer_v1 for receiving relative motion events.
// Destroys any existing relative pointer first.
func (h *LibwaylandHandle) GetRelativePointer(pointer uintptr) error {
	if h.relativePointerMgr == 0 {
		return fmt.Errorf("wayland: relative pointer manager not available")
	}

	// Destroy existing relative pointer if any
	h.DestroyRelativePointer()

	initPointerConstraintListeners()

	// get_relative_pointer: opcode 1 on zwp_relative_pointer_manager_v1
	// signature "no": new_id, pointer
	relPtr, err := h.marshalConstructorObj(
		h.relativePointerMgr, 1,
		unsafe.Pointer(&ptrConstraints.relPointer),
		pointer,
	)
	if err != nil {
		return fmt.Errorf("wayland: get_relative_pointer failed: %w", err)
	}
	h.relativePointer = relPtr

	// Add listener for relative_motion events
	if err := h.addListener(relPtr, uintptr(unsafe.Pointer(&relPointerListener[0]))); err != nil {
		slog.Warn("wayland: failed to add relative_pointer listener", "err", err)
	}

	return nil
}

// DestroyLockedPointer destroys the current locked pointer constraint.
func (h *LibwaylandHandle) DestroyLockedPointer() {
	if h.lockedPointer == 0 {
		return
	}
	// destroy: opcode 0 on zwp_locked_pointer_v1
	h.marshalVoid(h.lockedPointer, 0)
	h.proxyDestroy(h.lockedPointer)
	h.lockedPointer = 0
}

// DestroyConfinedPointer destroys the current confined pointer constraint.
func (h *LibwaylandHandle) DestroyConfinedPointer() {
	if h.confinedPointer == 0 {
		return
	}
	// destroy: opcode 0 on zwp_confined_pointer_v1
	h.marshalVoid(h.confinedPointer, 0)
	h.proxyDestroy(h.confinedPointer)
	h.confinedPointer = 0
}

// DestroyRelativePointer destroys the current relative pointer.
func (h *LibwaylandHandle) DestroyRelativePointer() {
	if h.relativePointer == 0 {
		return
	}
	// destroy: opcode 0 on zwp_relative_pointer_v1
	h.marshalVoid(h.relativePointer, 0)
	h.proxyDestroy(h.relativePointer)
	h.relativePointer = 0
}

// InputPointer returns the main input wl_pointer proxy.
// Used by platform layer for pointer constraints and set_cursor.
func (h *LibwaylandHandle) InputPointer() uintptr {
	return h.inputPointer
}

// PointerEnterSerial returns the serial from the last wl_pointer.enter event.
// Required by wl_pointer.set_cursor to authorize cursor changes.
func (h *LibwaylandHandle) PointerEnterSerial() uint32 {
	return h.pointerEnterSerial
}

// HideCursor hides the mouse cursor by setting NULL surface on wl_pointer.
// Uses wl_pointer.set_cursor (opcode 0): signature "u?oii" (serial, surface, hotspot_x, hotspot_y).
// Pass serial=0 for initial hide; the compositor typically accepts it.
func (h *LibwaylandHandle) HideCursor(serial uint32) {
	if h.inputPointer == 0 {
		return
	}
	// wl_pointer.set_cursor: opcode 0
	// Args: serial(uint32), surface(NULL=hide), hotspot_x(0), hotspot_y(0)
	h.marshalVoid(h.inputPointer, 0, uintptr(serial), 0, 0, 0)
}

// ShowCursor restores the default cursor by requesting the compositor set
// the cursor to the default shape. On Wayland, this is done by NOT setting
// a cursor surface — the compositor will use the default cursor shape.
// For a proper implementation, this would need cursor shape manager or
// xcursor theme loading. For now, we set the enter serial so the compositor
// knows we want the default cursor back.
func (h *LibwaylandHandle) ShowCursor(serial uint32) {
	// Wayland cursor restoration requires either:
	// 1. wp_cursor_shape_manager_v1.set_cursor_shape(default)
	// 2. Loading xcursor theme → wl_surface → wl_pointer.set_cursor
	// Both are significant effort (PLAT-008 task).
	//
	// For pointer lock exit, the enter callback in the platform layer
	// will trigger cursor restoration naturally when the pointer re-enters.
	// No-op here — the compositor restores the cursor automatically when
	// the pointer constraint is released and the pointer re-enters the surface.
	_ = serial
}
