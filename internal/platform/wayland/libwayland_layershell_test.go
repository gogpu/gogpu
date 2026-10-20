//go:build linux

package wayland

import "testing"

// These tests pin the parts of the layer-shell descriptors that libwayland
// reads at bind and marshal time. Names and signatures live in C memory (that
// is what libwayland needs), so the checks here are structural — opcode order,
// counts, versions, distinctness — rather than string comparisons. The strings
// themselves are copied from wlr-layer-shell-unstable-v1.xml and reviewed there.

// TestLayerShellRequestOpcodes pins the request order of both interfaces.
//
// Opcodes come from the order the protocol XML declares requests in, and they
// are not guessable from request names: zwlr_layer_shell_v1 declares
// get_layer_surface first, and destroy only arrives in version 3. Swapping them
// sends a garbage request, which a compositor reports as "invalid object N"
// followed by a desynchronised stream rather than as a bad opcode, so the order
// is worth an explicit test.
func TestLayerShellRequestOpcodes(t *testing.T) {
	initLayerShellInterfaces()

	if layerShellGetLayerSurface != 0 {
		t.Errorf("zwlr_layer_shell_v1.get_layer_surface = opcode %d, want 0 (declared first in the protocol XML)",
			layerShellGetLayerSurface)
	}
	if layerShellDestroy != 1 {
		t.Errorf("zwlr_layer_shell_v1.destroy = opcode %d, want 1 (added in version 3)",
			layerShellDestroy)
	}

	wantSurfaceOpcodes := []struct {
		name   string
		opcode Opcode
	}{
		{"set_size", layerSurfaceSetSize},
		{"set_anchor", layerSurfaceSetAnchor},
		{"set_exclusive_zone", layerSurfaceSetExclusiveZone},
		{"set_margin", layerSurfaceSetMargin},
		{"set_keyboard_interactivity", layerSurfaceSetKeyboardInteractivity},
		{"get_popup", layerSurfaceGetPopup},
		{"ack_configure", layerSurfaceAckConfigure},
		{"destroy", layerSurfaceDestroy},
		{"set_layer", layerSurfaceSetLayer},
		{"set_exclusive_edge", layerSurfaceSetExclusiveEdge},
	}
	for i, want := range wantSurfaceOpcodes {
		if want.opcode != Opcode(i) {
			t.Errorf("zwlr_layer_surface_v1.%s = opcode %d, want %d (protocol XML order)",
				want.name, want.opcode, i)
		}
	}
}

// TestLayerShellEventOpcodes pins the event order, which fixes the layout of
// the listener array handed to wl_proxy_add_listener.
func TestLayerShellEventOpcodes(t *testing.T) {
	initLayerShellInterfaces()

	if layerSurfaceEventConfigure != 0 {
		t.Errorf("configure = event %d, want 0", layerSurfaceEventConfigure)
	}
	if layerSurfaceEventClosed != 1 {
		t.Errorf("closed = event %d, want 1", layerSurfaceEventClosed)
	}
	if len(layerSurfaceListener) != int(layerShellIface.surface.EventCount) {
		t.Errorf("listener array has %d slots, interface declares %d events",
			len(layerSurfaceListener), layerShellIface.surface.EventCount)
	}
}

// TestLayerShellInterfaceMetadata checks the interface descriptions libwayland
// reads: names, protocol versions, and method/event counts.
func TestLayerShellInterfaceMetadata(t *testing.T) {
	initLayerShellInterfaces()

	if layerShellIface.shell.Name == 0 || layerShellIface.surface.Name == 0 {
		t.Fatal("interface names must be set for libwayland to match protocol messages")
	}
	if layerShellIface.shell.Version != 5 || layerShellIface.surface.Version != 5 {
		t.Errorf("interface versions = %d/%d, want 5/5",
			layerShellIface.shell.Version, layerShellIface.surface.Version)
	}
	if layerShellIface.shell.MethodCount != 2 || layerShellIface.shell.EventCount != 0 {
		t.Errorf("shell methods/events = %d/%d, want 2/0",
			layerShellIface.shell.MethodCount, layerShellIface.shell.EventCount)
	}
	if layerShellIface.surface.MethodCount != 10 || layerShellIface.surface.EventCount != 2 {
		t.Errorf("surface methods/events = %d/%d, want 10/2",
			layerShellIface.surface.MethodCount, layerShellIface.surface.EventCount)
	}
	if int(layerShellIface.shell.MethodCount) != len(layerShellIface.shellMethods) {
		t.Errorf("shell declares %d methods but has %d descriptors",
			layerShellIface.shell.MethodCount, len(layerShellIface.shellMethods))
	}
	if int(layerShellIface.surface.MethodCount) != len(layerShellIface.surfaceMethods) {
		t.Errorf("surface declares %d methods but has %d descriptors",
			layerShellIface.surface.MethodCount, len(layerShellIface.surfaceMethods))
	}
}

// TestLayerShellDescriptorDistinctness catches copy-paste errors where two
// methods end up sharing a name pointer, which is how an opcode gets wired to
// the wrong request. Signatures are deliberately not checked this way: several
// requests share one signature ("u", "uu"), and identical literals may share
// storage, so the only signature invariant worth asserting is how many requests
// take no arguments at all.
func TestLayerShellDescriptorDistinctness(t *testing.T) {
	initLayerShellInterfaces()

	destroySig := layerShellIface.surfaceMethods[uint32(layerSurfaceDestroy)].Signature

	seenNames := map[uintptr]Opcode{}
	emptySigs := 0

	for i, msg := range layerShellIface.surfaceMethods {
		opcode := Opcode(i)
		if prev, dup := seenNames[msg.Name]; dup {
			t.Errorf("surface methods %d and %d share a name pointer", prev, opcode)
		}
		seenNames[msg.Name] = opcode

		if msg.Name == 0 || msg.Types == 0 || msg.Signature == 0 {
			t.Errorf("surface method %d has a nil name, types array or signature", opcode)
		}
		if msg.Signature == destroySig {
			emptySigs++
		}
	}
	if emptySigs != 1 {
		t.Errorf("expected exactly one argument-less request (destroy), found %d", emptySigs)
	}
}

// TestLayerShellEnumValues pins the wire values of the enums a caller selects
// through LayerShellOptions.
func TestLayerShellEnumValues(t *testing.T) {
	if LayerBackground != 0 || LayerBottom != 1 || LayerTop != 2 || LayerOverlay != 3 {
		t.Error("layer enum values must be background=0 bottom=1 top=2 overlay=3")
	}
	if AnchorTop != 1 || AnchorBottom != 2 || AnchorLeft != 4 || AnchorRight != 8 {
		t.Error("anchor bits must be top=1 bottom=2 left=4 right=8")
	}
	if KeyboardInteractivityNone != 0 || KeyboardInteractivityExclusive != 1 || KeyboardInteractivityOnDemand != 2 {
		t.Error("keyboard interactivity must be none=0 exclusive=1 on_demand=2")
	}
}

// TestLayerShellNotEnabledByDefault guards the default path: a handle built
// without layer-shell options must report IsLayerShell false, so toplevel-only
// setup (CSD, title, size constraints) is still performed for ordinary windows.
func TestLayerShellNotEnabledByDefault(t *testing.T) {
	h := &LibwaylandHandle{}
	if h.IsLayerShell() {
		t.Error("IsLayerShell must be false when no layer surface exists")
	}
	if h.LayerConfigured() {
		t.Error("LayerConfigured must be false before any configure event")
	}
	if w, hgt := h.LayerSize(); w != 0 || hgt != 0 {
		t.Errorf("LayerSize = %d,%d before configure, want 0,0", w, hgt)
	}
}

// TestValidateLayerSizeAnchors covers the size/anchor rules the compositor
// enforces. A zero size means "compositor chooses", which is only legal when
// that axis is anchored to both opposite edges; getting it wrong is reported by
// the compositor as "width 0 requested without setting left and right anchors",
// so it is worth catching before a connection is made.
func TestValidateLayerSizeAnchors(t *testing.T) {
	const (
		all    = AnchorTop | AnchorBottom | AnchorLeft | AnchorRight
		corner = AnchorLeft | AnchorTop
	)

	tests := []struct {
		name          string
		width, height uint32
		anchor        uint32
		wantErr       bool
	}{
		{"explicit size with a corner anchor", 100, 100, corner, false},
		{"explicit size with all anchors", 100, 100, all, false},
		{"both axes left to the compositor", 0, 0, all, false},
		{"width stretched, height explicit", 0, 100, all, false},
		{"height stretched, width explicit", 100, 0, all, false},
		{"zero width with a corner anchor", 0, 100, corner, true},
		{"zero height with a corner anchor", 100, 0, corner, true},
		{"zero width, height anchored only", 0, 100, AnchorTop | AnchorBottom, true},
		{"no anchors at all", 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLayerSizeAnchors(tt.width, tt.height, tt.anchor)
			if tt.wantErr && err == nil {
				t.Fatalf("validateLayerSizeAnchors(%d, %d, %d) = nil, want error",
					tt.width, tt.height, tt.anchor)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateLayerSizeAnchors(%d, %d, %d) = %v, want nil",
					tt.width, tt.height, tt.anchor, err)
			}
		})
	}
}

// TestDescribeAnchors checks the error-message rendering, since a bad anchor
// mask is otherwise invisible to the caller.
func TestDescribeAnchors(t *testing.T) {
	tests := []struct {
		anchor uint32
		want   string
	}{
		{0, "none"},
		{AnchorLeft, "left"},
		{AnchorLeft | AnchorTop, "top+left"}, // fixed order: top, bottom, left, right
		{AnchorTop | AnchorBottom | AnchorLeft | AnchorRight, "top+bottom+left+right"},
	}
	for _, tt := range tests {
		if got := describeAnchors(tt.anchor); got != tt.want {
			t.Errorf("describeAnchors(%d) = %q, want %q", tt.anchor, got, tt.want)
		}
	}
}
