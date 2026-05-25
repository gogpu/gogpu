//go:build linux

package wayland

// libwayland_cursor_shape.go — wp_cursor_shape_manager_v1 protocol.
//
// Implements the modern cursor shape protocol for setting cursor shapes
// on Wayland compositors without loading xcursor themes. The protocol
// provides server-side cursor rendering using named cursor shapes.
//
// Protocol objects:
//   - wp_cursor_shape_manager_v1: global manager, binds from registry
//   - wp_cursor_shape_device_v1: per-pointer device, sets shapes by enum
//
// Uses the same C-compatible interface descriptor pattern as
// libwayland_pointer_constraints.go.

import (
	"fmt"
	"log/slog"
	"sync"
	"unsafe"
)

// wp_cursor_shape_device_v1 shape enum values from the Wayland protocol XML.
const (
	wpCursorShapeDefault     uint32 = 1
	wpCursorShapeContextMenu uint32 = 2
	wpCursorShapeHelp        uint32 = 3
	wpCursorShapePointer     uint32 = 4
	wpCursorShapeProgress    uint32 = 5
	wpCursorShapeWait        uint32 = 6
	wpCursorShapeCell        uint32 = 7
	wpCursorShapeCrosshair   uint32 = 8
	wpCursorShapeText        uint32 = 9
	wpCursorShapeMove        uint32 = 13
	wpCursorShapeNotAllowed  uint32 = 15
	wpCursorShapeEResize     uint32 = 18
	wpCursorShapeNResize     uint32 = 19
	wpCursorShapeNEResize    uint32 = 20
	wpCursorShapeNWResize    uint32 = 21
	wpCursorShapeSResize     uint32 = 22
	wpCursorShapeSEResize    uint32 = 23
	wpCursorShapeSWResize    uint32 = 24
	wpCursorShapeWResize     uint32 = 25
	wpCursorShapeEWResize    uint32 = 26
	wpCursorShapeNSResize    uint32 = 27
	wpCursorShapeNESWResize  uint32 = 28
	wpCursorShapeNWSEResize  uint32 = 29
)

// cursorShapeInterfaces holds C-compatible interface descriptors for
// wp_cursor_shape_manager_v1 and wp_cursor_shape_device_v1.
// Constructed once, live for program lifetime.
var cursorShapeInterfaces struct {
	once sync.Once

	// Interface descriptors
	manager cWlInterface // wp_cursor_shape_manager_v1
	device  cWlInterface // wp_cursor_shape_device_v1

	// Method arrays (indexed by opcode)
	managerMethods [2]cWlMessage // destroy, get_pointer
	deviceMethods  [2]cWlMessage // destroy, set_shape

	// NULL types array (shared by all messages)
	nullTypes [4]uintptr
}

// initCursorShapeInterfaces constructs C-compatible interface descriptors
// for wp_cursor_shape_manager_v1 and wp_cursor_shape_device_v1 protocols.
// Called once during the first cursor shape setup.
func initCursorShapeInterfaces() {
	cursorShapeInterfaces.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&cursorShapeInterfaces.nullTypes[0]))

		// === wp_cursor_shape_manager_v1 methods ===
		// opcode 0: destroy (no args)
		cursorShapeInterfaces.managerMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		// opcode 1: get_pointer(new_id<wp_cursor_shape_device_v1>, pointer<wl_pointer>)
		// signature: "no"
		cursorShapeInterfaces.managerMethods[1] = cWlMessage{cstr("get_pointer\x00"), cstr("no\x00"), nt}

		// wp_cursor_shape_manager_v1 interface (no events)
		cursorShapeInterfaces.manager = cWlInterface{
			Name:        cstr("wp_cursor_shape_manager_v1\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&cursorShapeInterfaces.managerMethods[0])),
			EventCount:  0,
			Events:      0,
		}

		// === wp_cursor_shape_device_v1 methods ===
		// opcode 0: destroy (no args)
		cursorShapeInterfaces.deviceMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		// opcode 1: set_shape(serial: uint, shape: uint)
		// signature: "uu"
		cursorShapeInterfaces.deviceMethods[1] = cWlMessage{cstr("set_shape\x00"), cstr("uu\x00"), nt}

		// wp_cursor_shape_device_v1 interface (no events)
		cursorShapeInterfaces.device = cWlInterface{
			Name:        cstr("wp_cursor_shape_device_v1\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&cursorShapeInterfaces.deviceMethods[0])),
			EventCount:  0,
			Events:      0,
		}
	})
}

// --- LibwaylandHandle methods for cursor shape ---

// SetupCursorShape binds the wp_cursor_shape_manager_v1 global.
// Called during Init if the compositor advertises this protocol.
func (h *LibwaylandHandle) SetupCursorShape(name, version uint32) error {
	initCursorShapeInterfaces()

	v := version
	if v > 1 {
		v = 1
	}

	mgr, err := h.registryBind(name, unsafe.Pointer(&cursorShapeInterfaces.manager), v)
	if err != nil {
		return fmt.Errorf("wayland: failed to bind wp_cursor_shape_manager_v1: %w", err)
	}
	h.cursorShapeMgr = mgr
	return nil
}

// CreateCursorShapeDevice creates a wp_cursor_shape_device_v1 for a wl_pointer.
// This is the main pointer's cursor shape device, used by SetCursorShape.
func (h *LibwaylandHandle) CreateCursorShapeDevice(pointer uintptr) error {
	if h.cursorShapeMgr == 0 {
		return fmt.Errorf("wayland: cursor shape manager not available")
	}
	if pointer == 0 {
		return fmt.Errorf("wayland: pointer is nil")
	}

	initCursorShapeInterfaces()

	// get_pointer: opcode 1 on wp_cursor_shape_manager_v1
	// signature "no": new_id<wp_cursor_shape_device_v1>, pointer<wl_pointer>
	device, err := h.marshalConstructorObj(
		h.cursorShapeMgr, 1,
		unsafe.Pointer(&cursorShapeInterfaces.device),
		pointer,
	)
	if err != nil {
		return fmt.Errorf("wayland: get_pointer (cursor shape) failed: %w", err)
	}
	h.cursorShapeDevice = device
	return nil
}

// CreateCSDCursorShapeDevice creates a wp_cursor_shape_device_v1 for the CSD pointer.
// Used for setting resize/move cursors on decoration subsurfaces.
func (h *LibwaylandHandle) CreateCSDCursorShapeDevice(pointer uintptr) error {
	if h.cursorShapeMgr == 0 {
		return fmt.Errorf("wayland: cursor shape manager not available")
	}
	if pointer == 0 {
		return fmt.Errorf("wayland: CSD pointer is nil")
	}

	initCursorShapeInterfaces()

	// get_pointer: opcode 1 on wp_cursor_shape_manager_v1
	device, err := h.marshalConstructorObj(
		h.cursorShapeMgr, 1,
		unsafe.Pointer(&cursorShapeInterfaces.device),
		pointer,
	)
	if err != nil {
		return fmt.Errorf("wayland: get_pointer (CSD cursor shape) failed: %w", err)
	}
	h.csdCursorShapeDevice = device
	return nil
}

// HasCursorShape returns true if the compositor supports wp_cursor_shape_manager_v1.
func (h *LibwaylandHandle) HasCursorShape() bool {
	return h.cursorShapeMgr != 0
}

// SetCursorShape sets the cursor shape on the main pointer using wp_cursor_shape_device_v1.
// cursorID maps to gpucontext.CursorShape values (0-11).
// For CursorNone (11), falls back to HideCursor via wl_pointer.set_cursor with NULL surface.
func (h *LibwaylandHandle) SetCursorShape(cursorID int, serial uint32) {
	if h.cursorShapeDevice == 0 {
		return
	}

	// CursorNone (11) = hide cursor via wl_pointer.set_cursor with NULL surface
	if cursorID == 11 {
		h.HideCursor(serial)
		return
	}

	shape := cursorIDToWpShape(cursorID)
	// set_shape: opcode 1 on wp_cursor_shape_device_v1
	// Args: serial (uint32), shape (uint32)
	h.marshalVoid(h.cursorShapeDevice, 1, uintptr(serial), uintptr(shape))
}

// SetCSDCursorShape sets the cursor shape on the CSD pointer.
// Used by CSD hit-test to show resize/move cursors on decoration borders.
func (h *LibwaylandHandle) SetCSDCursorShape(shape uint32, serial uint32) {
	if h.csdCursorShapeDevice == 0 {
		return
	}
	// set_shape: opcode 1 on wp_cursor_shape_device_v1
	h.marshalVoid(h.csdCursorShapeDevice, 1, uintptr(serial), uintptr(shape))
}

// DestroyCursorShapeDevices destroys both main and CSD cursor shape devices.
func (h *LibwaylandHandle) DestroyCursorShapeDevices() {
	if h.csdCursorShapeDevice != 0 {
		h.marshalVoid(h.csdCursorShapeDevice, 0) // wp_cursor_shape_device_v1::destroy
		h.proxyDestroy(h.csdCursorShapeDevice)
		h.csdCursorShapeDevice = 0
	}
	if h.cursorShapeDevice != 0 {
		h.marshalVoid(h.cursorShapeDevice, 0) // wp_cursor_shape_device_v1::destroy
		h.proxyDestroy(h.cursorShapeDevice)
		h.cursorShapeDevice = 0
	}
}

// cursorIDToWpShape maps gpucontext.CursorShape int values to
// wp_cursor_shape_device_v1 shape enum values.
func cursorIDToWpShape(cursorID int) uint32 {
	switch cursorID {
	case 0: // CursorDefault
		return wpCursorShapeDefault
	case 1: // CursorPointer (hand)
		return wpCursorShapePointer
	case 2: // CursorText (I-beam)
		return wpCursorShapeText
	case 3: // CursorCrosshair
		return wpCursorShapeCrosshair
	case 4: // CursorMove
		return wpCursorShapeMove
	case 5: // CursorResizeNS
		return wpCursorShapeNSResize
	case 6: // CursorResizeEW
		return wpCursorShapeEWResize
	case 7: // CursorResizeNWSE
		return wpCursorShapeNWSEResize
	case 8: // CursorResizeNESW
		return wpCursorShapeNESWResize
	case 9: // CursorNotAllowed
		return wpCursorShapeNotAllowed
	case 10: // CursorWait
		return wpCursorShapeWait
	default:
		return wpCursorShapeDefault
	}
}

// csdHitToWpCursorShape maps a CSD hit-test result to the appropriate
// wp_cursor_shape_device_v1 shape enum value.
func csdHitToWpCursorShape(hit CSDHitResult) uint32 {
	switch hit {
	case CSDHitResizeN:
		return wpCursorShapeNResize
	case CSDHitResizeS:
		return wpCursorShapeSResize
	case CSDHitResizeW:
		return wpCursorShapeWResize
	case CSDHitResizeE:
		return wpCursorShapeEResize
	case CSDHitResizeNW:
		return wpCursorShapeNWResize
	case CSDHitResizeNE:
		return wpCursorShapeNEResize
	case CSDHitResizeSW:
		return wpCursorShapeSWResize
	case CSDHitResizeSE:
		return wpCursorShapeSEResize
	default:
		return wpCursorShapeDefault
	}
}

// UpdateCSDCursor updates the CSD pointer cursor shape based on the current
// hit-test result. Called from updateCSDHitTest when the hit result changes.
// This is deferred (like repaint) to avoid nested FFI calls from within
// goffi callbacks.
func (h *LibwaylandHandle) UpdateCSDCursor() {
	if h.csdCursorShapeDevice == 0 || h.csdSerial == 0 {
		return
	}
	shape := csdHitToWpCursorShape(h.csdHitResult)
	h.SetCSDCursorShape(shape, h.csdSerial)
	if err := h.flush(); err != nil {
		slog.Warn("wayland: cursor shape flush failed", "err", err)
	}
}
