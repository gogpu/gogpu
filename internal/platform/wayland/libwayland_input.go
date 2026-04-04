//go:build linux

package wayland

// libwayland_input.go — Input events (pointer, keyboard, touch) on C libwayland display.
//
// Extends LibwaylandHandle with main surface input handling via goffi callbacks.
// Follows the same pattern as libwayland_csd.go (seat binding, pointer callbacks).
//
// The CSD code already handles pointer events on decoration subsurfaces.
// This file handles pointer/keyboard/touch events on the MAIN surface,
// routing them to Go callbacks registered via SetPointerHandler etc.

import (
	"log/slog"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// InputCallbacks holds Go-side callbacks for input events.
// Set by the platform layer (platform_linux.go) after LibwaylandHandle is created.
type InputCallbacks struct {
	// Pointer events on main surface
	OnPointerEnter  func(serial uint32, x, y float64)
	OnPointerLeave  func(serial uint32)
	OnPointerMotion func(timeMs uint32, x, y float64)
	OnPointerButton func(serial, timeMs, button, state uint32)
	OnPointerAxis   func(timeMs, axis uint32, value float64)

	// Keyboard events
	OnKeyboardKeymap    func(format uint32, fd int, size uint32)
	OnKeyboardEnter     func(serial uint32, keys []uint32)
	OnKeyboardLeave     func(serial uint32)
	OnKeyboardKey       func(serial, timeMs, key, state uint32)
	OnKeyboardModifiers func(serial, modsDepressed, modsLatched, modsLocked, group uint32)
	OnKeyboardRepeat    func(rate, delay int32)

	// Touch events
	OnTouchDown   func(serial, timeMs uint32, id int32, x, y float64)
	OnTouchUp     func(serial, timeMs uint32, id int32)
	OnTouchMotion func(timeMs uint32, id int32, x, y float64)
	OnTouchCancel func()

	// Close event from xdg_toplevel
	OnClose func()

	// Configure event from xdg_toplevel (width, height, states)
	OnConfigure func(width, height int32)
}

// Input-related fields are added to LibwaylandHandle via composition in the
// existing struct (libwayland.go). We add methods here.

// SetInputCallbacks sets the input event callbacks.
// Must be called before SetupInput.
func (h *LibwaylandHandle) SetInputCallbacks(cb *InputCallbacks) {
	h.inputCallbacks = cb
}

// SetupInput binds wl_seat on the C display and sets up pointer, keyboard,
// and touch listeners. Also registers xdg_toplevel listeners for configure/close.
// seatName/seatVersion come from the registry (discovered by registry listener).
func (h *LibwaylandHandle) SetupInput(seatName, seatVersion uint32) error {
	if seatName == 0 {
		slog.Warn("wayland input: wl_seat not available, no input")
		return nil
	}

	version := seatVersion
	if version > 7 {
		version = 7
	}

	// Bind wl_seat on C display (default queue — same queue as main surface events)
	seat, err := h.registryBind(seatName, h.seatInterface, version)
	if err != nil {
		return err
	}
	h.inputSeat = seat

	// Add seat capabilities listener
	initInputSeatListeners()
	if err := h.addListener(seat, uintptr(unsafe.Pointer(&inputSeatListener[0]))); err != nil {
		return err
	}

	// Roundtrip to get capabilities
	if err := h.flush(); err != nil {
		return err
	}
	if err := h.roundtrip(); err != nil {
		return err
	}

	// Get pointer
	if h.inputSeatCaps&SeatCapabilityPointer != 0 {
		pointer, err := h.marshalConstructor(seat, 0, h.pointerInterface)
		if err != nil {
			slog.Warn("wayland input: get_pointer failed", "err", err)
		} else {
			h.inputPointer = pointer
			initInputPointerListeners()
			if err := h.addListener(pointer, uintptr(unsafe.Pointer(&inputPointerListener[0]))); err != nil {
				slog.Warn("wayland input: pointer listener failed", "err", err)
			}
		}
	}

	// Get keyboard
	if h.inputSeatCaps&SeatCapabilityKeyboard != 0 {
		keyboard, err := h.marshalConstructor(seat, 1, h.keyboardInterface)
		if err != nil {
			slog.Warn("wayland input: get_keyboard failed", "err", err)
		} else {
			h.inputKeyboard = keyboard
			initInputKeyboardListeners()
			if err := h.addListener(keyboard, uintptr(unsafe.Pointer(&inputKeyboardListener[0]))); err != nil {
				slog.Warn("wayland input: keyboard listener failed", "err", err)
			}
		}
	}

	// Get touch
	if h.inputSeatCaps&SeatCapabilityTouch != 0 {
		touch, err := h.marshalConstructor(seat, 2, h.touchInterface)
		if err != nil {
			slog.Warn("wayland input: get_touch failed", "err", err)
		} else {
			h.inputTouch = touch
			initInputTouchListeners()
			if err := h.addListener(touch, uintptr(unsafe.Pointer(&inputTouchListener[0]))); err != nil {
				slog.Warn("wayland input: touch listener failed", "err", err)
			}
		}
	}

	return h.flush()
}

// SetupToplevelListeners adds configure and close listeners on the xdg_toplevel.
// Must be called after setupXdgRole.
func (h *LibwaylandHandle) SetupToplevelListeners() error {
	if h.xdgToplevel == 0 {
		return nil
	}
	initInputToplevelListeners()
	return h.addListener(h.xdgToplevel, uintptr(unsafe.Pointer(&inputToplevelListener[0])))
}

// DispatchDefaultQueue dispatches pending events on the default queue (non-blocking).
// This handles xdg events, pointer, keyboard, touch — everything on the main display.
func (h *LibwaylandHandle) DispatchDefaultQueue() error {
	if err := h.flush(); err != nil {
		return err
	}

	// Non-blocking dispatch: prepare_read → read if data available → dispatch_pending.
	dispArgs := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}

	var prepResult int32
	ffi.CallFunction(&h.cifPrepareRead, h.fnPrepareRead, unsafe.Pointer(&prepResult), dispArgs[:])

	if prepResult == 0 {
		// Successfully locked for reading — check if data available
		fd := h.getDisplayFD()
		if fd >= 0 && socketHasData(fd) {
			var readResult int32
			ffi.CallFunction(&h.cifReadEvents, h.fnReadEvents, unsafe.Pointer(&readResult), dispArgs[:])
		} else {
			// No data — cancel read lock
			ffi.CallFunction(&h.cifPrepareRead, h.fnCancelRead, nil, dispArgs[:])
		}
	}

	// Dispatch any events that were read (or already queued)
	var dispResult int32
	ffi.CallFunction(&h.cifDispatchP, h.fnDispatchPend, unsafe.Pointer(&dispResult), dispArgs[:])

	return nil
}

// GetDisplayFD returns the Wayland display file descriptor for polling.
func (h *LibwaylandHandle) GetDisplayFD() int {
	return h.getDisplayFD()
}

// --- Global callback handle for input events ---

var inputCallbackHandle *LibwaylandHandle

// SetAsInputHandler sets this handle as the active input callback target.
func (h *LibwaylandHandle) SetAsInputHandler() {
	inputCallbackHandle = h
}

// --- Seat callbacks ---

var (
	inputSeatListener [2]uintptr // capabilities, name
	inputSeatOnce     sync.Once
)

func initInputSeatListeners() {
	inputSeatOnce.Do(func() {
		inputSeatListener[0] = ffi.NewCallback(inputSeatCapabilitiesCb)
		inputSeatListener[1] = ffi.NewCallback(inputSeatNameCb)
	})
}

func inputSeatCapabilitiesCb(data, seat, capabilities uintptr) {
	h := inputCallbackHandle
	if h == nil {
		return
	}
	h.inputSeatCaps = uint32(capabilities)
}

func inputSeatNameCb(data, seat, name uintptr) {
	// No-op
}

// --- Pointer callbacks (main surface, 9 events) ---

var (
	inputPointerListener [9]uintptr
	inputPointerOnce     sync.Once
)

func initInputPointerListeners() {
	inputPointerOnce.Do(func() {
		inputPointerListener[0] = ffi.NewCallback(inputPointerEnterCb)
		inputPointerListener[1] = ffi.NewCallback(inputPointerLeaveCb)
		inputPointerListener[2] = ffi.NewCallback(inputPointerMotionCb)
		inputPointerListener[3] = ffi.NewCallback(inputPointerButtonCb)
		inputPointerListener[4] = ffi.NewCallback(inputPointerAxisCb)
		inputPointerListener[5] = ffi.NewCallback(inputPointerFrameCb)
		inputPointerListener[6] = ffi.NewCallback(inputPointerAxisSourceCb)
		inputPointerListener[7] = ffi.NewCallback(inputPointerAxisStopCb)
		inputPointerListener[8] = ffi.NewCallback(inputPointerAxisDiscreteCb)
	})
}

// inputPointerEnterCb: void(data, wl_pointer, serial, surface, sx_fixed, sy_fixed)
func inputPointerEnterCb(data, pointer, serial, surface, sxFixed, syFixed uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	// Only handle events for main surface
	if surface != h.surface {
		return
	}
	x := float64(int32(sxFixed)) / 256.0
	y := float64(int32(syFixed)) / 256.0
	if h.inputCallbacks.OnPointerEnter != nil {
		h.inputCallbacks.OnPointerEnter(uint32(serial), x, y)
	}
}

// inputPointerLeaveCb: void(data, wl_pointer, serial, surface)
func inputPointerLeaveCb(data, pointer, serial, surface uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if surface != h.surface {
		return
	}
	if h.inputCallbacks.OnPointerLeave != nil {
		h.inputCallbacks.OnPointerLeave(uint32(serial))
	}
}

// inputPointerMotionCb: void(data, wl_pointer, time, sx_fixed, sy_fixed)
func inputPointerMotionCb(data, pointer, time, sxFixed, syFixed uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	x := float64(int32(sxFixed)) / 256.0
	y := float64(int32(syFixed)) / 256.0
	if h.inputCallbacks.OnPointerMotion != nil {
		h.inputCallbacks.OnPointerMotion(uint32(time), x, y)
	}
}

// inputPointerButtonCb: void(data, wl_pointer, serial, time, button, state)
func inputPointerButtonCb(data, pointer, serial, time, button, state uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnPointerButton != nil {
		h.inputCallbacks.OnPointerButton(uint32(serial), uint32(time), uint32(button), uint32(state))
	}
}

// inputPointerAxisCb: void(data, wl_pointer, time, axis, value_fixed)
func inputPointerAxisCb(data, pointer, time, axis, valueFixed uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	value := float64(int32(valueFixed)) / 256.0
	if h.inputCallbacks.OnPointerAxis != nil {
		h.inputCallbacks.OnPointerAxis(uint32(time), uint32(axis), value)
	}
}

func inputPointerFrameCb(data, pointer uintptr)                        {}
func inputPointerAxisSourceCb(data, pointer, source uintptr)           {}
func inputPointerAxisStopCb(data, pointer, time, axis uintptr)         {}
func inputPointerAxisDiscreteCb(data, pointer, axis, discrete uintptr) {}

// --- Keyboard callbacks (6 events) ---

var (
	inputKeyboardListener [6]uintptr
	inputKeyboardOnce     sync.Once
)

func initInputKeyboardListeners() {
	inputKeyboardOnce.Do(func() {
		inputKeyboardListener[0] = ffi.NewCallback(inputKeyboardKeymapCb)
		inputKeyboardListener[1] = ffi.NewCallback(inputKeyboardEnterCb)
		inputKeyboardListener[2] = ffi.NewCallback(inputKeyboardLeaveCb)
		inputKeyboardListener[3] = ffi.NewCallback(inputKeyboardKeyCb)
		inputKeyboardListener[4] = ffi.NewCallback(inputKeyboardModifiersCb)
		inputKeyboardListener[5] = ffi.NewCallback(inputKeyboardRepeatInfoCb)
	})
}

// inputKeyboardKeymapCb: void(data, wl_keyboard, format, fd, size)
func inputKeyboardKeymapCb(data, keyboard, format, fd, size uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnKeyboardKeymap != nil {
		h.inputCallbacks.OnKeyboardKeymap(uint32(format), int(fd), uint32(size))
	}
}

// inputKeyboardEnterCb: void(data, wl_keyboard, serial, surface, keys_array)
// Note: keys is a wl_array — we receive the pointer to the struct.
// For simplicity, pass empty keys (full XKB integration is a future task).
func inputKeyboardEnterCb(data, keyboard, serial, surface, keys uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	// Only handle events for our main surface
	if surface != h.surface {
		return
	}
	if h.inputCallbacks.OnKeyboardEnter != nil {
		h.inputCallbacks.OnKeyboardEnter(uint32(serial), nil)
	}
}

// inputKeyboardLeaveCb: void(data, wl_keyboard, serial, surface)
func inputKeyboardLeaveCb(data, keyboard, serial, surface uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnKeyboardLeave != nil {
		h.inputCallbacks.OnKeyboardLeave(uint32(serial))
	}
}

// inputKeyboardKeyCb: void(data, wl_keyboard, serial, time, key, state)
func inputKeyboardKeyCb(data, keyboard, serial, time, key, state uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnKeyboardKey != nil {
		h.inputCallbacks.OnKeyboardKey(uint32(serial), uint32(time), uint32(key), uint32(state))
	}
}

// inputKeyboardModifiersCb: void(data, wl_keyboard, serial, mods_depressed, mods_latched, mods_locked, group)
func inputKeyboardModifiersCb(data, keyboard, serial, modsDepressed, modsLatched, modsLocked, group uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnKeyboardModifiers != nil {
		h.inputCallbacks.OnKeyboardModifiers(uint32(serial), uint32(modsDepressed), uint32(modsLatched), uint32(modsLocked), uint32(group))
	}
}

// inputKeyboardRepeatInfoCb: void(data, wl_keyboard, rate, delay)
func inputKeyboardRepeatInfoCb(data, keyboard, rate, delay uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnKeyboardRepeat != nil {
		h.inputCallbacks.OnKeyboardRepeat(int32(rate), int32(delay))
	}
}

// --- Touch callbacks (7 events) ---

var (
	inputTouchListener [7]uintptr
	inputTouchOnce     sync.Once
)

func initInputTouchListeners() {
	inputTouchOnce.Do(func() {
		inputTouchListener[0] = ffi.NewCallback(inputTouchDownCb)
		inputTouchListener[1] = ffi.NewCallback(inputTouchUpCb)
		inputTouchListener[2] = ffi.NewCallback(inputTouchMotionCb)
		inputTouchListener[3] = ffi.NewCallback(inputTouchFrameCb)
		inputTouchListener[4] = ffi.NewCallback(inputTouchCancelCb)
		inputTouchListener[5] = ffi.NewCallback(inputTouchShapeCb)
		inputTouchListener[6] = ffi.NewCallback(inputTouchOrientationCb)
	})
}

// inputTouchDownCb: void(data, wl_touch, serial, time, surface, id, x_fixed, y_fixed)
func inputTouchDownCb(data, touch, serial, time, surface, id, xFixed, yFixed uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if surface != h.surface {
		return
	}
	x := float64(int32(xFixed)) / 256.0
	y := float64(int32(yFixed)) / 256.0
	if h.inputCallbacks.OnTouchDown != nil {
		h.inputCallbacks.OnTouchDown(uint32(serial), uint32(time), int32(id), x, y)
	}
}

// inputTouchUpCb: void(data, wl_touch, serial, time, id)
func inputTouchUpCb(data, touch, serial, time, id uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnTouchUp != nil {
		h.inputCallbacks.OnTouchUp(uint32(serial), uint32(time), int32(id))
	}
}

// inputTouchMotionCb: void(data, wl_touch, time, id, x_fixed, y_fixed)
func inputTouchMotionCb(data, touch, time, id, xFixed, yFixed uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	x := float64(int32(xFixed)) / 256.0
	y := float64(int32(yFixed)) / 256.0
	if h.inputCallbacks.OnTouchMotion != nil {
		h.inputCallbacks.OnTouchMotion(uint32(time), int32(id), x, y)
	}
}

func inputTouchFrameCb(data, touch uintptr) {}
func inputTouchCancelCb(data, touch uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnTouchCancel != nil {
		h.inputCallbacks.OnTouchCancel()
	}
}
func inputTouchShapeCb(data, touch, id, major, minor uintptr)      {}
func inputTouchOrientationCb(data, touch, id, orientation uintptr) {}

// --- xdg_toplevel callbacks (4 events: configure, close, configure_bounds, wm_capabilities) ---

var (
	inputToplevelListener [4]uintptr
	inputToplevelOnce     sync.Once
)

func initInputToplevelListeners() {
	inputToplevelOnce.Do(func() {
		inputToplevelListener[0] = ffi.NewCallback(inputToplevelConfigureCb)
		inputToplevelListener[1] = ffi.NewCallback(inputToplevelCloseCb)
		inputToplevelListener[2] = ffi.NewCallback(inputToplevelConfigureBoundsCb)
		inputToplevelListener[3] = ffi.NewCallback(inputToplevelWmCapabilitiesCb)
	})
}

// inputToplevelConfigureCb: void(data, xdg_toplevel, width, height, states_array)
// The states argument is a pointer to wl_array { size_t size; size_t alloc; void *data }.
// Each element is a uint32 xdg_toplevel_state enum value.
func inputToplevelConfigureCb(data, toplevel, width, height, states uintptr) {
	slog.Debug("xdg_toplevel.configure", "width", int32(width), "height", int32(height))
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}

	// Parse states array for maximized state.
	// wl_array layout on 64-bit: { uint64 size, uint64 alloc, uintptr data }
	// xdg_toplevel_state::maximized = 1
	if states != 0 && h.csdActive {
		maximized := wlArrayContainsUint32(states, 1)
		if h.csdState.Maximized != maximized {
			h.csdState.Maximized = maximized
			h.csdPendingRepaint = true
		}
	}

	// Store configure dimensions for set_window_geometry in ack_configure.
	if int32(width) > 0 && int32(height) > 0 {
		h.configuredW = int(int32(width))
		h.configuredH = int(int32(height))
	}

	// Call platform OnConfigure — it calculates CSD content dimensions
	// (handling restore-from-saved, border subtraction) and calls
	// SetPendingCSDResize. The actual CSD resize happens in
	// xdgSurfaceConfigureCb after ack_configure.
	if h.inputCallbacks.OnConfigure != nil {
		h.inputCallbacks.OnConfigure(int32(width), int32(height))
	}
}

// inputToplevelCloseCb: void(data, xdg_toplevel)
func inputToplevelCloseCb(data, toplevel uintptr) {
	h := inputCallbackHandle
	if h == nil || h.inputCallbacks == nil {
		return
	}
	if h.inputCallbacks.OnClose != nil {
		h.inputCallbacks.OnClose()
	}
}

func inputToplevelConfigureBoundsCb(data, toplevel, width, height uintptr) {}
func inputToplevelWmCapabilitiesCb(data, toplevel, capabilities uintptr)   {}
