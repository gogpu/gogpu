//go:build linux

package wayland

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestXdgWmBaseOpcodes verifies xdg_wm_base opcode constants match protocol spec.
func TestXdgWmBaseOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"destroy", xdgWmBaseDestroy, 0},
		{"create_positioner", xdgWmBaseCreatePositioner, 1},
		{"get_xdg_surface", xdgWmBaseGetXdgSurface, 2},
		{"pong", xdgWmBasePong, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgWmBaseEventOpcodes verifies xdg_wm_base event opcode constants.
func TestXdgWmBaseEventOpcodes(t *testing.T) {
	if xdgWmBaseEventPing != 0 {
		t.Errorf("xdgWmBaseEventPing = %d, want 0", xdgWmBaseEventPing)
	}
}

// TestXdgSurfaceOpcodes verifies xdg_surface opcode constants match protocol spec.
func TestXdgSurfaceOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"destroy", xdgSurfaceDestroy, 0},
		{"get_toplevel", xdgSurfaceGetToplevel, 1},
		{"get_popup", xdgSurfaceGetPopup, 2},
		{"set_window_geometry", xdgSurfaceSetWindowGeometry, 3},
		{"ack_configure", xdgSurfaceAckConfigure, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgSurfaceEventOpcodes verifies xdg_surface event opcode constants.
func TestXdgSurfaceEventOpcodes(t *testing.T) {
	if xdgSurfaceEventConfigure != 0 {
		t.Errorf("xdgSurfaceEventConfigure = %d, want 0", xdgSurfaceEventConfigure)
	}
}

// TestXdgToplevelOpcodes verifies xdg_toplevel opcode constants match protocol spec.
func TestXdgToplevelOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"destroy", xdgToplevelDestroy, 0},
		{"set_parent", xdgToplevelSetParent, 1},
		{"set_title", xdgToplevelSetTitle, 2},
		{"set_app_id", xdgToplevelSetAppID, 3},
		{"show_window_menu", xdgToplevelShowWindowMenu, 4},
		{"move", xdgToplevelMove, 5},
		{"resize", xdgToplevelResize, 6},
		{"set_max_size", xdgToplevelSetMaxSize, 7},
		{"set_min_size", xdgToplevelSetMinSize, 8},
		{"set_maximized", xdgToplevelSetMaximized, 9},
		{"unset_maximized", xdgToplevelUnsetMaximized, 10},
		{"set_fullscreen", xdgToplevelSetFullscreen, 11},
		{"unset_fullscreen", xdgToplevelUnsetFullscreen, 12},
		{"set_minimized", xdgToplevelSetMinimized, 13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgToplevelEventOpcodes verifies xdg_toplevel event opcode constants.
func TestXdgToplevelEventOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"configure", xdgToplevelEventConfigure, 0},
		{"close", xdgToplevelEventClose, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("event opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgToplevelStateConstants verifies xdg_toplevel state constants match protocol spec.
func TestXdgToplevelStateConstants(t *testing.T) {
	tests := []struct {
		name     string
		state    uint32
		expected uint32
	}{
		{"maximized", XdgToplevelStateMaximized, 1},
		{"fullscreen", XdgToplevelStateFullscreen, 2},
		{"resizing", XdgToplevelStateResizing, 3},
		{"activated", XdgToplevelStateActivated, 4},
		{"tiled_left", XdgToplevelStateTiledLeft, 5},
		{"tiled_right", XdgToplevelStateTiledRight, 6},
		{"tiled_top", XdgToplevelStateTiledTop, 7},
		{"tiled_bottom", XdgToplevelStateTiledBottom, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.state != tt.expected {
				t.Errorf("state %s = %d, want %d", tt.name, tt.state, tt.expected)
			}
		})
	}
}

// TestXdgToplevelResizeEdgeConstants verifies resize edge constants.
func TestXdgToplevelResizeEdgeConstants(t *testing.T) {
	tests := []struct {
		name     string
		edge     uint32
		expected uint32
	}{
		{"none", XdgToplevelResizeEdgeNone, 0},
		{"top", XdgToplevelResizeEdgeTop, 1},
		{"bottom", XdgToplevelResizeEdgeBottom, 2},
		{"left", XdgToplevelResizeEdgeLeft, 4},
		{"top_left", XdgToplevelResizeEdgeTopLeft, 5},
		{"bottom_left", XdgToplevelResizeEdgeBottomLeft, 6},
		{"right", XdgToplevelResizeEdgeRight, 8},
		{"top_right", XdgToplevelResizeEdgeTopRight, 9},
		{"bottom_right", XdgToplevelResizeEdgeBottomRight, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.edge != tt.expected {
				t.Errorf("resize edge %s = %d, want %d", tt.name, tt.edge, tt.expected)
			}
		})
	}
}

// TestXdgPositionerOpcodes verifies xdg_positioner opcode constants.
func TestXdgPositionerOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"destroy", xdgPositionerDestroy, 0},
		{"set_size", xdgPositionerSetSize, 1},
		{"set_anchor_rect", xdgPositionerSetAnchorRect, 2},
		{"set_anchor", xdgPositionerSetAnchor, 3},
		{"set_gravity", xdgPositionerSetGravity, 4},
		{"set_constraint_adjustment", xdgPositionerSetConstraintAdjust, 5},
		{"set_offset", xdgPositionerSetOffset, 6},
		{"set_reactive", xdgPositionerSetReactive, 7},
		{"set_parent_size", xdgPositionerSetParentSize, 8},
		{"set_parent_configure", xdgPositionerSetParentConfigure, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgPositionerAnchorConstants verifies xdg_positioner anchor constants.
func TestXdgPositionerAnchorConstants(t *testing.T) {
	tests := []struct {
		name     string
		anchor   uint32
		expected uint32
	}{
		{"none", XdgPositionerAnchorNone, 0},
		{"top", XdgPositionerAnchorTop, 1},
		{"bottom", XdgPositionerAnchorBottom, 2},
		{"left", XdgPositionerAnchorLeft, 3},
		{"right", XdgPositionerAnchorRight, 4},
		{"top_left", XdgPositionerAnchorTopLeft, 5},
		{"bottom_left", XdgPositionerAnchorBottomLeft, 6},
		{"top_right", XdgPositionerAnchorTopRight, 7},
		{"bottom_right", XdgPositionerAnchorBottomRight, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.anchor != tt.expected {
				t.Errorf("anchor %s = %d, want %d", tt.name, tt.anchor, tt.expected)
			}
		})
	}
}

// TestXdgPositionerGravityConstants verifies xdg_positioner gravity constants.
func TestXdgPositionerGravityConstants(t *testing.T) {
	tests := []struct {
		name     string
		gravity  uint32
		expected uint32
	}{
		{"none", XdgPositionerGravityNone, 0},
		{"top", XdgPositionerGravityTop, 1},
		{"bottom", XdgPositionerGravityBottom, 2},
		{"left", XdgPositionerGravityLeft, 3},
		{"right", XdgPositionerGravityRight, 4},
		{"top_left", XdgPositionerGravityTopLeft, 5},
		{"bottom_left", XdgPositionerGravityBottomLeft, 6},
		{"top_right", XdgPositionerGravityTopRight, 7},
		{"bottom_right", XdgPositionerGravityBottomRight, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.gravity != tt.expected {
				t.Errorf("gravity %s = %d, want %d", tt.name, tt.gravity, tt.expected)
			}
		})
	}
}

// TestXdgPositionerConstraintAdjustmentConstants verifies constraint adjustment flags.
func TestXdgPositionerConstraintAdjustmentConstants(t *testing.T) {
	tests := []struct {
		name       string
		adjustment uint32
		expected   uint32
	}{
		{"none", XdgPositionerConstraintAdjustmentNone, 0},
		{"slide_x", XdgPositionerConstraintAdjustmentSlideX, 1},
		{"slide_y", XdgPositionerConstraintAdjustmentSlideY, 2},
		{"flip_x", XdgPositionerConstraintAdjustmentFlipX, 4},
		{"flip_y", XdgPositionerConstraintAdjustmentFlipY, 8},
		{"resize_x", XdgPositionerConstraintAdjustmentResizeX, 16},
		{"resize_y", XdgPositionerConstraintAdjustmentResizeY, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.adjustment != tt.expected {
				t.Errorf("constraint adjustment %s = %d, want %d", tt.name, tt.adjustment, tt.expected)
			}
		})
	}
}

// TestXdgPopupOpcodes verifies xdg_popup opcode constants.
func TestXdgPopupOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"destroy", xdgPopupDestroy, 0},
		{"grab", xdgPopupGrab, 1},
		{"reposition", xdgPopupReposition, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgPopupEventOpcodes verifies xdg_popup event opcode constants.
func TestXdgPopupEventOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"configure", xdgPopupEventConfigure, 0},
		{"popup_done", xdgPopupEventPopupDone, 1},
		{"repositioned", xdgPopupEventRepositioned, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("event opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestXdgWmBaseCreation verifies XdgWmBase struct initialization.
func TestXdgWmBaseCreation(t *testing.T) {
	wmBase := &XdgWmBase{
		display: nil,
		id:      ObjectID(5),
	}

	if wmBase.ID() != ObjectID(5) {
		t.Errorf("XdgWmBase.ID() = %d, want 5", wmBase.ID())
	}
}

// TestXdgSurfaceCreation verifies XdgSurface struct initialization.
func TestXdgSurfaceCreation(t *testing.T) {
	surface := &WlSurface{id: ObjectID(10)}
	xdgSurface := NewXdgSurface(nil, ObjectID(11), surface)

	if xdgSurface.ID() != ObjectID(11) {
		t.Errorf("XdgSurface.ID() = %d, want 11", xdgSurface.ID())
	}

	if xdgSurface.Surface() != surface {
		t.Error("XdgSurface.Surface() returned wrong surface")
	}

	if xdgSurface.IsConfigured() {
		t.Error("XdgSurface should not be configured initially")
	}
}

// TestXdgToplevelCreation verifies XdgToplevel struct initialization.
func TestXdgToplevelCreation(t *testing.T) {
	surface := &WlSurface{id: ObjectID(10)}
	xdgSurface := NewXdgSurface(nil, ObjectID(11), surface)
	toplevel := NewXdgToplevel(nil, ObjectID(12), xdgSurface)

	if toplevel.ID() != ObjectID(12) {
		t.Errorf("XdgToplevel.ID() = %d, want 12", toplevel.ID())
	}

	if toplevel.XdgSurface() != xdgSurface {
		t.Error("XdgToplevel.XdgSurface() returned wrong xdg_surface")
	}

	if toplevel.Surface() != surface {
		t.Error("XdgToplevel.Surface() returned wrong surface")
	}

	if toplevel.Title() != "" {
		t.Errorf("XdgToplevel.Title() = %q, want empty", toplevel.Title())
	}

	if toplevel.AppID() != "" {
		t.Errorf("XdgToplevel.AppID() = %q, want empty", toplevel.AppID())
	}
}

// TestXdgPositionerCreation verifies XdgPositioner struct initialization.
func TestXdgPositionerCreation(t *testing.T) {
	positioner := NewXdgPositioner(nil, ObjectID(20))

	if positioner.ID() != ObjectID(20) {
		t.Errorf("XdgPositioner.ID() = %d, want 20", positioner.ID())
	}
}

// TestXdgPopupCreation verifies XdgPopup struct initialization.
func TestXdgPopupCreation(t *testing.T) {
	surface := &WlSurface{id: ObjectID(10)}
	xdgSurface := NewXdgSurface(nil, ObjectID(11), surface)
	popup := NewXdgPopup(nil, ObjectID(30), xdgSurface)

	if popup.ID() != ObjectID(30) {
		t.Errorf("XdgPopup.ID() = %d, want 30", popup.ID())
	}

	if popup.XdgSurface() != xdgSurface {
		t.Error("XdgPopup.XdgSurface() returned wrong xdg_surface")
	}

	if popup.Surface() != surface {
		t.Error("XdgPopup.Surface() returned wrong surface")
	}
}

// TestXdgToplevelConfigParsing verifies XdgToplevelConfig.parseStates method.
func TestXdgToplevelConfigParsing(t *testing.T) {
	tests := []struct {
		name        string
		states      []uint32
		maximized   bool
		fullscreen  bool
		resizing    bool
		activated   bool
		tiledLeft   bool
		tiledRight  bool
		tiledTop    bool
		tiledBottom bool
	}{
		{
			name:      "empty states",
			states:    []uint32{},
			maximized: false, fullscreen: false, resizing: false, activated: false,
		},
		{
			name:      "activated only",
			states:    []uint32{XdgToplevelStateActivated},
			activated: true,
		},
		{
			name:       "maximized and activated",
			states:     []uint32{XdgToplevelStateMaximized, XdgToplevelStateActivated},
			maximized:  true,
			activated:  true,
			fullscreen: false,
		},
		{
			name:       "fullscreen",
			states:     []uint32{XdgToplevelStateFullscreen, XdgToplevelStateActivated},
			fullscreen: true,
			activated:  true,
		},
		{
			name:     "resizing",
			states:   []uint32{XdgToplevelStateResizing, XdgToplevelStateActivated},
			resizing: true, activated: true,
		},
		{
			name:        "all tiled states",
			states:      []uint32{XdgToplevelStateTiledLeft, XdgToplevelStateTiledRight, XdgToplevelStateTiledTop, XdgToplevelStateTiledBottom},
			tiledLeft:   true,
			tiledRight:  true,
			tiledTop:    true,
			tiledBottom: true,
		},
		{
			name: "all states",
			states: []uint32{
				XdgToplevelStateMaximized, XdgToplevelStateFullscreen,
				XdgToplevelStateResizing, XdgToplevelStateActivated,
				XdgToplevelStateTiledLeft, XdgToplevelStateTiledRight,
				XdgToplevelStateTiledTop, XdgToplevelStateTiledBottom,
			},
			maximized: true, fullscreen: true, resizing: true, activated: true,
			tiledLeft: true, tiledRight: true, tiledTop: true, tiledBottom: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &XdgToplevelConfig{
				Width:  800,
				Height: 600,
				States: tt.states,
			}
			config.parseStates()

			if config.Maximized != tt.maximized {
				t.Errorf("Maximized = %v, want %v", config.Maximized, tt.maximized)
			}
			if config.Fullscreen != tt.fullscreen {
				t.Errorf("Fullscreen = %v, want %v", config.Fullscreen, tt.fullscreen)
			}
			if config.Resizing != tt.resizing {
				t.Errorf("Resizing = %v, want %v", config.Resizing, tt.resizing)
			}
			if config.Activated != tt.activated {
				t.Errorf("Activated = %v, want %v", config.Activated, tt.activated)
			}
			if config.TiledLeft != tt.tiledLeft {
				t.Errorf("TiledLeft = %v, want %v", config.TiledLeft, tt.tiledLeft)
			}
			if config.TiledRight != tt.tiledRight {
				t.Errorf("TiledRight = %v, want %v", config.TiledRight, tt.tiledRight)
			}
			if config.TiledTop != tt.tiledTop {
				t.Errorf("TiledTop = %v, want %v", config.TiledTop, tt.tiledTop)
			}
			if config.TiledBottom != tt.tiledBottom {
				t.Errorf("TiledBottom = %v, want %v", config.TiledBottom, tt.tiledBottom)
			}
		})
	}
}

// TestSetTitleMessage verifies the message format for xdg_toplevel.set_title.
func TestSetTitleMessage(t *testing.T) {
	builder := NewMessageBuilder()
	title := "My Window Title"

	builder.PutString(title)
	msg := builder.BuildMessage(ObjectID(100), xdgToplevelSetTitle)

	if msg.Opcode != xdgToplevelSetTitle {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelSetTitle)
	}

	dec := NewDecoder(msg.Args)
	gotTitle, err := dec.String()
	if err != nil {
		t.Fatalf("failed to decode title: %v", err)
	}
	if gotTitle != title {
		t.Errorf("title = %q, want %q", gotTitle, title)
	}
}

// TestSetAppIDMessage verifies the message format for xdg_toplevel.set_app_id.
func TestSetAppIDMessage(t *testing.T) {
	builder := NewMessageBuilder()
	appID := "org.gogpu.example"

	builder.PutString(appID)
	msg := builder.BuildMessage(ObjectID(101), xdgToplevelSetAppID)

	if msg.Opcode != xdgToplevelSetAppID {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelSetAppID)
	}

	dec := NewDecoder(msg.Args)
	gotAppID, err := dec.String()
	if err != nil {
		t.Fatalf("failed to decode app_id: %v", err)
	}
	if gotAppID != appID {
		t.Errorf("app_id = %q, want %q", gotAppID, appID)
	}
}

// TestSetMaxSizeMessage verifies the message format for xdg_toplevel.set_max_size.
func TestSetMaxSizeMessage(t *testing.T) {
	builder := NewMessageBuilder()
	width, height := int32(1920), int32(1080)

	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(102), xdgToplevelSetMaxSize)

	if msg.Opcode != xdgToplevelSetMaxSize {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelSetMaxSize)
	}

	dec := NewDecoder(msg.Args)
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotWidth != width || gotHeight != height {
		t.Errorf("max_size = (%d, %d), want (%d, %d)", gotWidth, gotHeight, width, height)
	}
}

// TestSetMinSizeMessage verifies the message format for xdg_toplevel.set_min_size.
func TestSetMinSizeMessage(t *testing.T) {
	builder := NewMessageBuilder()
	width, height := int32(320), int32(240)

	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(103), xdgToplevelSetMinSize)

	if msg.Opcode != xdgToplevelSetMinSize {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelSetMinSize)
	}

	dec := NewDecoder(msg.Args)
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotWidth != width || gotHeight != height {
		t.Errorf("min_size = (%d, %d), want (%d, %d)", gotWidth, gotHeight, width, height)
	}
}

// TestPongMessage verifies the message format for xdg_wm_base.pong.
func TestPongMessage(t *testing.T) {
	builder := NewMessageBuilder()
	serial := uint32(12345)

	builder.PutUint32(serial)
	msg := builder.BuildMessage(ObjectID(104), xdgWmBasePong)

	if msg.Opcode != xdgWmBasePong {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgWmBasePong)
	}

	dec := NewDecoder(msg.Args)
	gotSerial, err := dec.Uint32()
	if err != nil {
		t.Fatalf("failed to decode serial: %v", err)
	}
	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
}

// TestAckConfigureMessage verifies the message format for xdg_surface.ack_configure.
func TestAckConfigureMessage(t *testing.T) {
	builder := NewMessageBuilder()
	serial := uint32(98765)

	builder.PutUint32(serial)
	msg := builder.BuildMessage(ObjectID(105), xdgSurfaceAckConfigure)

	if msg.Opcode != xdgSurfaceAckConfigure {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgSurfaceAckConfigure)
	}

	dec := NewDecoder(msg.Args)
	gotSerial, err := dec.Uint32()
	if err != nil {
		t.Fatalf("failed to decode serial: %v", err)
	}
	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
}

// TestSetWindowGeometryMessage verifies the message format for xdg_surface.set_window_geometry.
func TestSetWindowGeometryMessage(t *testing.T) {
	builder := NewMessageBuilder()
	x, y, width, height := int32(10), int32(20), int32(800), int32(600)

	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(106), xdgSurfaceSetWindowGeometry)

	if msg.Opcode != xdgSurfaceSetWindowGeometry {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgSurfaceSetWindowGeometry)
	}

	dec := NewDecoder(msg.Args)
	gotX, _ := dec.Int32()
	gotY, _ := dec.Int32()
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotX != x || gotY != y || gotWidth != width || gotHeight != height {
		t.Errorf("geometry = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
			gotX, gotY, gotWidth, gotHeight, x, y, width, height)
	}
}

// TestMoveMessage verifies the message format for xdg_toplevel.move.
func TestMoveMessage(t *testing.T) {
	builder := NewMessageBuilder()
	seat := ObjectID(50)
	serial := uint32(11111)

	builder.PutObject(seat)
	builder.PutUint32(serial)
	msg := builder.BuildMessage(ObjectID(107), xdgToplevelMove)

	if msg.Opcode != xdgToplevelMove {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelMove)
	}

	dec := NewDecoder(msg.Args)
	gotSeat, _ := dec.Object()
	gotSerial, _ := dec.Uint32()

	if gotSeat != seat || gotSerial != serial {
		t.Errorf("move args = (%d, %d), want (%d, %d)", gotSeat, gotSerial, seat, serial)
	}
}

// TestResizeMessage verifies the message format for xdg_toplevel.resize.
func TestResizeMessage(t *testing.T) {
	builder := NewMessageBuilder()
	seat := ObjectID(51)
	serial := uint32(22222)
	edges := XdgToplevelResizeEdgeBottomRight

	builder.PutObject(seat)
	builder.PutUint32(serial)
	builder.PutUint32(edges)
	msg := builder.BuildMessage(ObjectID(108), xdgToplevelResize)

	if msg.Opcode != xdgToplevelResize {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelResize)
	}

	dec := NewDecoder(msg.Args)
	gotSeat, _ := dec.Object()
	gotSerial, _ := dec.Uint32()
	gotEdges, _ := dec.Uint32()

	if gotSeat != seat || gotSerial != serial || gotEdges != edges {
		t.Errorf("resize args = (%d, %d, %d), want (%d, %d, %d)",
			gotSeat, gotSerial, gotEdges, seat, serial, edges)
	}
}

// TestSetFullscreenMessage verifies the message format for xdg_toplevel.set_fullscreen.
func TestSetFullscreenMessage(t *testing.T) {
	tests := []struct {
		name   string
		output ObjectID
	}{
		{"with output", ObjectID(60)},
		{"null output", ObjectID(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewMessageBuilder()
			builder.PutObject(tt.output)
			msg := builder.BuildMessage(ObjectID(109), xdgToplevelSetFullscreen)

			if msg.Opcode != xdgToplevelSetFullscreen {
				t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgToplevelSetFullscreen)
			}

			dec := NewDecoder(msg.Args)
			gotOutput, err := dec.Object()
			if err != nil {
				t.Fatalf("failed to decode output: %v", err)
			}
			if gotOutput != tt.output {
				t.Errorf("output = %d, want %d", gotOutput, tt.output)
			}
		})
	}
}

// TestToplevelConfigureEventParsing verifies parsing of xdg_toplevel.configure event.
func TestToplevelConfigureEventParsing(t *testing.T) {
	// Build a configure event with width, height, and states array
	width := int32(1024)
	height := int32(768)
	states := []uint32{XdgToplevelStateActivated, XdgToplevelStateMaximized}

	// Build the states array bytes
	statesData := make([]byte, len(states)*4)
	for i, state := range states {
		binary.LittleEndian.PutUint32(statesData[i*4:], state)
	}

	builder := NewMessageBuilder()
	builder.PutInt32(width)
	builder.PutInt32(height)
	builder.PutArray(statesData)

	msg := builder.BuildMessage(ObjectID(200), xdgToplevelEventConfigure)

	// Parse the event
	dec := NewDecoder(msg.Args)

	gotWidth, err := dec.Int32()
	if err != nil {
		t.Fatalf("failed to decode width: %v", err)
	}
	if gotWidth != width {
		t.Errorf("width = %d, want %d", gotWidth, width)
	}

	gotHeight, err := dec.Int32()
	if err != nil {
		t.Fatalf("failed to decode height: %v", err)
	}
	if gotHeight != height {
		t.Errorf("height = %d, want %d", gotHeight, height)
	}

	gotStatesData, err := dec.Array()
	if err != nil {
		t.Fatalf("failed to decode states: %v", err)
	}

	if !bytes.Equal(gotStatesData, statesData) {
		t.Errorf("states data = %x, want %x", gotStatesData, statesData)
	}

	// Parse states array
	gotStates := make([]uint32, len(gotStatesData)/4)
	for i := range gotStates {
		gotStates[i] = binary.LittleEndian.Uint32(gotStatesData[i*4:])
	}

	if len(gotStates) != len(states) {
		t.Fatalf("states count = %d, want %d", len(gotStates), len(states))
	}
	for i, state := range states {
		if gotStates[i] != state {
			t.Errorf("states[%d] = %d, want %d", i, gotStates[i], state)
		}
	}
}

// TestSurfaceConfigureEventParsing verifies parsing of xdg_surface.configure event.
func TestSurfaceConfigureEventParsing(t *testing.T) {
	builder := NewMessageBuilder()
	serial := uint32(54321)
	builder.PutUint32(serial)

	msg := builder.BuildMessage(ObjectID(201), xdgSurfaceEventConfigure)

	dec := NewDecoder(msg.Args)
	gotSerial, err := dec.Uint32()
	if err != nil {
		t.Fatalf("failed to decode serial: %v", err)
	}
	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
}

// TestPopupGrabMessage verifies the message format for xdg_popup.grab.
func TestPopupGrabMessage(t *testing.T) {
	builder := NewMessageBuilder()
	seat := ObjectID(70)
	serial := uint32(33333)

	builder.PutObject(seat)
	builder.PutUint32(serial)
	msg := builder.BuildMessage(ObjectID(300), xdgPopupGrab)

	if msg.Opcode != xdgPopupGrab {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgPopupGrab)
	}

	dec := NewDecoder(msg.Args)
	gotSeat, _ := dec.Object()
	gotSerial, _ := dec.Uint32()

	if gotSeat != seat || gotSerial != serial {
		t.Errorf("grab args = (%d, %d), want (%d, %d)", gotSeat, gotSerial, seat, serial)
	}
}

// TestPositionerSetSizeMessage verifies the message format for xdg_positioner.set_size.
func TestPositionerSetSizeMessage(t *testing.T) {
	builder := NewMessageBuilder()
	width, height := int32(200), int32(150)

	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(400), xdgPositionerSetSize)

	if msg.Opcode != xdgPositionerSetSize {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgPositionerSetSize)
	}

	dec := NewDecoder(msg.Args)
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotWidth != width || gotHeight != height {
		t.Errorf("size = (%d, %d), want (%d, %d)", gotWidth, gotHeight, width, height)
	}
}

// TestPositionerSetAnchorRectMessage verifies the message format for xdg_positioner.set_anchor_rect.
func TestPositionerSetAnchorRectMessage(t *testing.T) {
	builder := NewMessageBuilder()
	x, y, width, height := int32(100), int32(50), int32(30), int32(25)

	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(401), xdgPositionerSetAnchorRect)

	if msg.Opcode != xdgPositionerSetAnchorRect {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, xdgPositionerSetAnchorRect)
	}

	dec := NewDecoder(msg.Args)
	gotX, _ := dec.Int32()
	gotY, _ := dec.Int32()
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotX != x || gotY != y || gotWidth != width || gotHeight != height {
		t.Errorf("anchor_rect = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
			gotX, gotY, gotWidth, gotHeight, x, y, width, height)
	}
}

// TestXdgSurfaceDispatch verifies the dispatch method for xdg_surface.
func TestXdgSurfaceDispatch(t *testing.T) {
	surface := &WlSurface{id: ObjectID(10)}
	xdgSurface := NewXdgSurface(nil, ObjectID(11), surface)

	var configureCalled bool
	var configureSerial uint32

	xdgSurface.SetConfigureHandler(func(serial uint32) {
		configureCalled = true
		configureSerial = serial
	})

	// Build configure event
	builder := NewMessageBuilder()
	expectedSerial := uint32(99999)
	builder.PutUint32(expectedSerial)
	msg := builder.BuildMessage(xdgSurface.id, xdgSurfaceEventConfigure)

	// Dispatch
	err := xdgSurface.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !configureCalled {
		t.Error("configure handler was not called")
	}
	if configureSerial != expectedSerial {
		t.Errorf("configure serial = %d, want %d", configureSerial, expectedSerial)
	}
	if !xdgSurface.IsConfigured() {
		t.Error("xdg_surface should be marked as configured")
	}
}

// TestXdgToplevelDispatch verifies the dispatch method for xdg_toplevel.
func TestXdgToplevelDispatch(t *testing.T) {
	surface := &WlSurface{id: ObjectID(10)}
	xdgSurface := NewXdgSurface(nil, ObjectID(11), surface)
	toplevel := NewXdgToplevel(nil, ObjectID(12), xdgSurface)

	var configureCalled bool
	var configWidth, configHeight int32
	var configActivated bool

	toplevel.SetConfigureHandler(func(config *XdgToplevelConfig) {
		configureCalled = true
		configWidth = config.Width
		configHeight = config.Height
		configActivated = config.Activated
	})

	// Build configure event
	states := []uint32{XdgToplevelStateActivated}
	statesData := make([]byte, len(states)*4)
	for i, state := range states {
		binary.LittleEndian.PutUint32(statesData[i*4:], state)
	}

	builder := NewMessageBuilder()
	builder.PutInt32(1280)
	builder.PutInt32(720)
	builder.PutArray(statesData)
	msg := builder.BuildMessage(toplevel.id, xdgToplevelEventConfigure)

	// Dispatch
	err := toplevel.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !configureCalled {
		t.Error("configure handler was not called")
	}
	if configWidth != 1280 || configHeight != 720 {
		t.Errorf("configure size = (%d, %d), want (1280, 720)", configWidth, configHeight)
	}
	if !configActivated {
		t.Error("activated state should be true")
	}
}

// TestXdgToplevelCloseDispatch verifies handling of the close event.
func TestXdgToplevelCloseDispatch(t *testing.T) {
	surface := &WlSurface{id: ObjectID(10)}
	xdgSurface := NewXdgSurface(nil, ObjectID(11), surface)
	toplevel := NewXdgToplevel(nil, ObjectID(12), xdgSurface)

	var closeCalled bool
	toplevel.SetCloseHandler(func() {
		closeCalled = true
	})

	// Build close event (no arguments)
	msg := &Message{
		ObjectID: toplevel.id,
		Opcode:   xdgToplevelEventClose,
		Args:     nil,
	}

	// Dispatch
	err := toplevel.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !closeCalled {
		t.Error("close handler was not called")
	}
}

// TestNoOpMessages verifies that no-argument messages encode correctly.
func TestNoOpMessages(t *testing.T) {
	noArgOpcodes := []struct {
		name   string
		opcode Opcode
	}{
		{"set_maximized", xdgToplevelSetMaximized},
		{"unset_maximized", xdgToplevelUnsetMaximized},
		{"unset_fullscreen", xdgToplevelUnsetFullscreen},
		{"set_minimized", xdgToplevelSetMinimized},
		{"destroy", xdgToplevelDestroy},
	}

	for _, tt := range noArgOpcodes {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewMessageBuilder()
			msg := builder.BuildMessage(ObjectID(500), tt.opcode)

			if msg.Opcode != tt.opcode {
				t.Errorf("Opcode = %d, want %d", msg.Opcode, tt.opcode)
			}
			if len(msg.Args) != 0 {
				t.Errorf("Args length = %d, want 0", len(msg.Args))
			}
		})
	}
}
