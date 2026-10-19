//go:build linux

package wayland

// libwayland_layershell.go — zwlr_layer_shell_v1 role for the C Vulkan surface.
//
// Vulkan on Wayland requires the wl_surface to carry a role. Until now gogpu
// only offered xdg_toplevel (an ordinary window). Shell components - panels,
// overlays, wallpapers, launchers - need zwlr_layer_shell_v1 instead: anchored
// to output edges, undecorated, drawn in a chosen layer, and optionally
// keyboard-interactive on demand. Both roles hang off the same wl_surface, so
// the rest of the pipeline (swapchain, present, frame callbacks, fractional
// scaling, input) is unchanged.
//
// Descriptors are constructed in Go memory exactly like the xdg ones in
// libwayland_xdg.go, because zwlr_layer_shell_v1 is not shipped inside
// libwayland-client.so and gogpu is zero-CGO.
//
// Protocol reference:
// https://gitlab.freedesktop.org/wlroots/wlr-protocols/-/blob/master/unstable/wlr-layer-shell-unstable-v1.xml

import (
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// zwlr_layer_shell_v1 layer values (request get_layer_surface arg "layer").
const (
	LayerBackground uint32 = 0
	LayerBottom     uint32 = 1
	LayerTop        uint32 = 2
	LayerOverlay    uint32 = 3
)

// zwlr_layer_surface_v1 anchor bitmask.
const (
	AnchorTop    uint32 = 1 << 0
	AnchorBottom uint32 = 1 << 1
	AnchorLeft   uint32 = 1 << 2
	AnchorRight  uint32 = 1 << 3
)

// zwlr_layer_surface_v1 keyboard_interactivity values.
const (
	KeyboardInteractivityNone      uint32 = 0
	KeyboardInteractivityExclusive uint32 = 1
	KeyboardInteractivityOnDemand  uint32 = 2 // since protocol version 4
)

// LayerShellOptions describes the layer surface to create.
type LayerShellOptions struct {
	// Width and Height are logical pixels; 0 with all edges anchored means
	// "fill the output", which is what a full-screen overlay wants.
	Width, Height int32
	// Anchor is a bitmask of AnchorTop/Bottom/Left/Right.
	Anchor uint32
	// Layer selects the shell layer (LayerOverlay for an overlay).
	Layer uint32
	// ExclusiveZone is the reserved strut in logical pixels; -1 ignores
	// (request never reserves space), 0 makes it sized by content.
	ExclusiveZone int32
	// KeyboardInteractivity selects none/exclusive/on_demand.
	KeyboardInteractivity uint32
	// Namespace identifies the surface to compositor rules (e.g. niri).
	Namespace string

	// GlobalName/GlobalVersion are the registry global name and version of
	// zwlr_layer_shell_v1, discovered by the pure Go registry pass.
	GlobalName    uint32
	GlobalVersion uint32
}

// zwlr_layer_shell_v1 request opcodes. Opcode order follows the protocol XML
// (get_layer_surface is declared first); destroy exists only since version 3.
const (
	layerShellGetLayerSurface Opcode = 0 // get_layer_surface(id, surface, output, layer, namespace)
	layerShellDestroy         Opcode = 1 // destroy() [since v3]
)

// zwlr_layer_surface_v1 request opcodes.
const (
	layerSurfaceSetSize                  Opcode = 0 // set_size(width, height)
	layerSurfaceSetAnchor                Opcode = 1 // set_anchor(anchor)
	layerSurfaceSetExclusiveZone         Opcode = 2 // set_exclusive_zone(zone)
	layerSurfaceSetMargin                Opcode = 3 // set_margin(top, right, bottom, left)
	layerSurfaceSetKeyboardInteractivity Opcode = 4 // set_keyboard_interactivity(mode)
	layerSurfaceGetPopup                 Opcode = 5 // get_popup(popup)
	layerSurfaceAckConfigure             Opcode = 6 // ack_configure(serial)
	layerSurfaceDestroy                  Opcode = 7 // destroy()
	layerSurfaceSetLayer                 Opcode = 8 // set_layer(layer) [since v2]
	layerSurfaceSetExclusiveEdge         Opcode = 9 // set_exclusive_edge(edge) [since v5]
)

// zwlr_layer_shell_v1 / zwlr_layer_surface_v1 event opcodes.
const (
	layerSurfaceEventConfigure Opcode = 0 // configure(serial, width, height)
	layerSurfaceEventClosed    Opcode = 1 // closed()
)

// layerShellIface holds the C-compatible interface descriptors, built once.
var layerShellIface struct {
	once sync.Once

	shell   cWlInterface // zwlr_layer_shell_v1
	surface cWlInterface // zwlr_layer_surface_v1

	shellMethods [2]cWlMessage // destroy, get_layer_surface

	surfaceMethods [10]cWlMessage // set_size .. set_exclusive_edge
	surfaceEvents  [2]cWlMessage  // configure, closed

	nullTypes [16]uintptr
}

var layerShellListenersOnce sync.Once

var (
	layerSurfaceListener [2]uintptr // configure, closed
)

// layerShellHandlesMu guards layerSurfaceHndls, which routes layer-surface
// callbacks back to the owning handle when several connections are open.
var (
	layerShellHandlesMu sync.Mutex
	layerSurfaceHndls   = make(map[uintptr]*LibwaylandHandle)
)

// initLayerShellInterfaces builds the zwlr_layer_shell_v1 descriptors.
func initLayerShellInterfaces() {
	layerShellIface.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&layerShellIface.nullTypes[0]))

		// === zwlr_layer_shell_v1 methods (XML order: get_layer_surface, destroy) ===
		layerShellIface.shellMethods[0] = cWlMessage{cstr("get_layer_surface\x00"), cstr("no?ous\x00"), nt}
		layerShellIface.shellMethods[1] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}

		layerShellIface.shell = cWlInterface{
			Name:        cstr("zwlr_layer_shell_v1\x00"),
			Version:     5,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&layerShellIface.shellMethods[0])),
			EventCount:  0,
			Events:      0,
		}

		// === zwlr_layer_surface_v1 methods ===
		layerShellIface.surfaceMethods[0] = cWlMessage{cstr("set_size\x00"), cstr("uu\x00"), nt}
		layerShellIface.surfaceMethods[1] = cWlMessage{cstr("set_anchor\x00"), cstr("u\x00"), nt}
		layerShellIface.surfaceMethods[2] = cWlMessage{cstr("set_exclusive_zone\x00"), cstr("i\x00"), nt}
		layerShellIface.surfaceMethods[3] = cWlMessage{cstr("set_margin\x00"), cstr("iiii\x00"), nt}
		layerShellIface.surfaceMethods[4] = cWlMessage{cstr("set_keyboard_interactivity\x00"), cstr("u\x00"), nt}
		layerShellIface.surfaceMethods[5] = cWlMessage{cstr("get_popup\x00"), cstr("o\x00"), nt}
		layerShellIface.surfaceMethods[6] = cWlMessage{cstr("ack_configure\x00"), cstr("u\x00"), nt}
		layerShellIface.surfaceMethods[7] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		layerShellIface.surfaceMethods[8] = cWlMessage{cstr("set_layer\x00"), cstr("u\x00"), nt}
		layerShellIface.surfaceMethods[9] = cWlMessage{cstr("set_exclusive_edge\x00"), cstr("u\x00"), nt}

		// === zwlr_layer_surface_v1 events ===
		layerShellIface.surfaceEvents[0] = cWlMessage{cstr("configure\x00"), cstr("uuu\x00"), nt}
		layerShellIface.surfaceEvents[1] = cWlMessage{cstr("closed\x00"), cstr("\x00"), nt}

		layerShellIface.surface = cWlInterface{
			Name:        cstr("zwlr_layer_surface_v1\x00"),
			Version:     5,
			MethodCount: 10,
			Methods:     uintptr(unsafe.Pointer(&layerShellIface.surfaceMethods[0])),
			EventCount:  2,
			Events:      uintptr(unsafe.Pointer(&layerShellIface.surfaceEvents[0])),
		}
	})
}

func initLayerShellListeners() {
	layerShellListenersOnce.Do(func() {
		layerSurfaceListener[0] = ffi.NewCallback(layerSurfaceConfigureCb)
		layerSurfaceListener[1] = ffi.NewCallback(layerSurfaceClosedCb)
	})
}

// layerSurfaceConfigureCb handles zwlr_layer_surface_v1.configure(data, surface, serial, width, height).
// It acks the serial (which maps the surface) and records the compositor's size.
func layerSurfaceConfigureCb(data, layerSurface, serial, width, height uintptr) {
	layerShellHandlesMu.Lock()
	h := layerSurfaceHndls[layerSurface]
	layerShellHandlesMu.Unlock()
	if h == nil {
		return
	}

	// ack_configure = zwlr_layer_surface_v1 opcode 6, arg: serial (uint32).
	h.marshalVoid(layerSurface, uint32(layerSurfaceAckConfigure), serial)

	h.layerMu.Lock()
	h.layerW = int32(uint32(width))
	h.layerH = int32(uint32(height))
	h.layerConfigured = true
	h.layerMu.Unlock()

	slog.Debug("zwlr_layer_surface_v1.configure",
		"serial", uint32(serial), "width", h.layerW, "height", h.layerH)
}

// layerSurfaceClosedCb handles zwlr_layer_surface_v1.closed(data, surface).
// Per protocol, the client must destroy the surface after this event.
func layerSurfaceClosedCb(data, layerSurface uintptr) {
	layerShellHandlesMu.Lock()
	h := layerSurfaceHndls[layerSurface]
	layerShellHandlesMu.Unlock()
	if h == nil {
		return
	}
	h.layerMu.Lock()
	h.layerClosed = true
	h.layerMu.Unlock()
	slog.Info("zwlr_layer_surface_v1.closed — compositor asked the overlay to close")
}

// validateLayerSizeAnchors reports whether the requested size and anchors are a
// valid combination.
//
// The protocol lets a client pass 0 for an axis to let the compositor choose the
// size, but only if that axis is anchored to both its opposite edges: a surface
// asking the compositor to choose a width it cannot derive is a protocol error.
// Compositors report that as "width 0 requested without setting left and right
// anchors", which is hard to connect back to a config value, so it is checked
// here first.
func validateLayerSizeAnchors(width, height, anchor uint32) error {
	const horizontal = AnchorLeft | AnchorRight
	const vertical = AnchorTop | AnchorBottom

	if width == 0 && anchor&horizontal != horizontal {
		return fmt.Errorf(
			"layer surface width 0 asks the compositor to choose a width, which requires both left and right anchors (anchors: %s)",
			describeAnchors(anchor))
	}
	if height == 0 && anchor&vertical != vertical {
		return fmt.Errorf(
			"layer surface height 0 asks the compositor to choose a height, which requires both top and bottom anchors (anchors: %s)",
			describeAnchors(anchor))
	}
	return nil
}

// describeAnchors renders an anchor bitmask for error messages.
func describeAnchors(anchor uint32) string {
	names := make([]string, 0, 4)
	for _, a := range []struct {
		bit  uint32
		name string
	}{
		{AnchorTop, "top"},
		{AnchorBottom, "bottom"},
		{AnchorLeft, "left"},
		{AnchorRight, "right"},
	} {
		if anchor&a.bit != 0 {
			names = append(names, a.name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	out := names[0]
	for _, n := range names[1:] {
		out += "+" + n
	}
	return out
}

// setupLayerShellRole gives the C wl_surface a zwlr_layer_shell_v1 role.
//
// Protocol flow (all state requests MUST precede the first commit):
//  1. Bind zwlr_layer_shell_v1 (global name from the pure Go registry)
//  2. get_layer_surface(wl_surface, output=NULL, layer, namespace) -> zwlr_layer_surface_v1
//  3. set_size, set_anchor, set_exclusive_zone, set_margin, set_keyboard_interactivity, set_layer
//  4. Register configure/closed listener
//  5. First commit (empty) — the compositor replies with configure
//  6. Roundtrip until configure arrives, callback acks it
func (h *LibwaylandHandle) setupLayerShellRole(shellName, shellVersion uint32, opts LayerShellOptions) error {
	if err := validateLayerSizeAnchors(uint32(opts.Width), uint32(opts.Height), opts.Anchor); err != nil {
		return fmt.Errorf("wayland: %w", err)
	}

	initLayerShellInterfaces()
	initLayerShellListeners()

	version := shellVersion
	if version > 5 {
		version = 5
	}

	shell, err := h.registryBind(shellName, unsafe.Pointer(&layerShellIface.shell), version)
	if err != nil {
		return fmt.Errorf("wayland: failed to bind zwlr_layer_shell_v1: %w", err)
	}
	h.layerShell = shell
	h.layerShellVersion = version

	// namespace is a C string argument: NULL-terminated and kept alive for the call.
	ns := opts.Namespace
	if ns == "" {
		ns = "gogpu"
	}
	nsBuf := append([]byte(ns), 0)

	layerSurface, err := h.marshalConstructorArgs(
		shell, uint32(layerShellGetLayerSurface), unsafe.Pointer(&layerShellIface.surface),
		h.surface, // surface (object)
		0,         // output (nullable object) — 0 lets the compositor choose
		uintptr(opts.Layer),
		uintptr(unsafe.Pointer(&nsBuf[0])),
	)
	if err != nil {
		return fmt.Errorf("wayland: failed to create zwlr_layer_surface_v1: %w", err)
	}
	h.layerSurface = layerSurface

	if err := h.addListener(layerSurface, uintptr(unsafe.Pointer(&layerSurfaceListener[0]))); err != nil {
		return fmt.Errorf("wayland: failed to add layer surface listener: %w", err)
	}

	// Layer surface state. All state requests must precede the first commit.
	h.marshalVoid(layerSurface, uint32(layerSurfaceSetSize),
		uintptr(uint32(opts.Width)), uintptr(uint32(opts.Height)))
	h.marshalVoid(layerSurface, uint32(layerSurfaceSetAnchor), uintptr(opts.Anchor))
	h.marshalVoid(layerSurface, uint32(layerSurfaceSetExclusiveZone), uintptr(uint32(opts.ExclusiveZone)))
	h.marshalVoid(layerSurface, uint32(layerSurfaceSetMargin), 0, 0, 0, 0)

	keyboard := opts.KeyboardInteractivity
	if keyboard == KeyboardInteractivityOnDemand && version < 4 {
		slog.Warn("wayland: on_demand keyboard interactivity needs zwlr_layer_shell_v1 v4; using exclusive")
		keyboard = KeyboardInteractivityExclusive
	}
	h.marshalVoid(layerSurface, uint32(layerSurfaceSetKeyboardInteractivity), uintptr(keyboard))

	// set_layer exists since version 2.
	if version >= 2 {
		h.marshalVoid(layerSurface, uint32(layerSurfaceSetLayer), uintptr(opts.Layer))
	}

	// Route callbacks for this surface to this handle before any dispatch.
	layerShellHandlesMu.Lock()
	layerSurfaceHndls[layerSurface] = h
	layerShellHandlesMu.Unlock()

	// First commit: the layer surface exists only after this.
	h.marshalVoid(h.surface, uint32(surfaceCommit))
	if err := h.flush(); err != nil {
		return fmt.Errorf("wayland: flush before layer configure failed: %w", err)
	}

	slog.Debug("wayland: waiting for initial zwlr_layer_surface_v1.configure",
		"namespace", ns, "layer", opts.Layer, "anchor", opts.Anchor)
	if err := h.WaitForLayerConfigure(); err != nil {
		return fmt.Errorf("wayland: wait for layer configure failed: %w", err)
	}
	slog.Debug("wayland: layer surface configured",
		"width", h.layerW, "height", h.layerH)

	// Second commit now that the compositor has configured the surface.
	h.marshalVoid(h.surface, uint32(surfaceCommit))
	if err := h.flush(); err != nil {
		return fmt.Errorf("wayland: flush after layer setup failed: %w", err)
	}

	return nil
}

// WaitForLayerConfigure blocks (bounded) until the compositor configures the
// layer surface, mirroring WaitForConfigure for the xdg role.
func (h *LibwaylandHandle) WaitForLayerConfigure() error {
	const maxRoundtrips = 50
	for i := range maxRoundtrips {
		h.layerMu.Lock()
		done := h.layerConfigured
		closed := h.layerClosed
		h.layerMu.Unlock()
		if done {
			slog.Debug("wayland: layer configure received", "roundtrips", i)
			return nil
		}
		if closed {
			return fmt.Errorf("wayland: layer surface closed before configure")
		}
		if err := h.roundtrip(); err != nil {
			return fmt.Errorf("wayland: wait for layer configure: %w", err)
		}
	}
	slog.Warn("wayland: layer configure not received after max roundtrips, proceeding anyway")
	return nil
}

// IsLayerShell reports whether this handle owns a layer surface rather than an
// xdg_toplevel. Toplevel-only requests must be skipped when it is true.
func (h *LibwaylandHandle) IsLayerShell() bool {
	return h.layerSurface != 0
}

// LayerConfigured reports whether zwlr_layer_surface_v1.configure arrived.
func (h *LibwaylandHandle) LayerConfigured() bool {
	h.layerMu.Lock()
	defer h.layerMu.Unlock()
	return h.layerConfigured
}

// LayerSize returns the compositor-assigned logical size of the layer surface.
func (h *LibwaylandHandle) LayerSize() (int32, int32) {
	h.layerMu.Lock()
	defer h.layerMu.Unlock()
	return h.layerW, h.layerH
}

// LayerSurface returns the zwlr_layer_surface_v1 proxy pointer.
func (h *LibwaylandHandle) LayerSurface() uintptr { return h.layerSurface }

// unregisterLayerSurface removes the callback routing entry and destroys the
// layer surface during Close.
func (h *LibwaylandHandle) unregisterLayerSurface() {
	if h.layerSurface == 0 {
		return
	}
	layerShellHandlesMu.Lock()
	delete(layerSurfaceHndls, h.layerSurface)
	layerShellHandlesMu.Unlock()

	// Ask the compositor to drop the objects, then free the client proxies.
	// Without the proxyDestroy calls the proxies stay attached to the event
	// queue, which libwayland reports at shutdown as "queue destroyed while
	// proxies still attached".
	h.marshalVoid(h.layerSurface, uint32(layerSurfaceDestroy))
	h.proxyDestroy(h.layerSurface)
	if h.layerShell != 0 {
		// zwlr_layer_shell_v1.destroy only exists since protocol version 3.
		if h.layerShellVersion >= 3 {
			h.marshalVoid(h.layerShell, uint32(layerShellDestroy))
		}
		h.proxyDestroy(h.layerShell)
	}
	h.layerSurface = 0
	h.layerShell = 0
}
