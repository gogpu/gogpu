//go:build linux

package wayland

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// xdg_wm_base opcodes (requests)
const (
	xdgWmBaseDestroy          Opcode = 0 // destroy()
	xdgWmBaseCreatePositioner Opcode = 1 // create_positioner(id: new_id<xdg_positioner>)
	xdgWmBaseGetXdgSurface    Opcode = 2 // get_xdg_surface(id: new_id<xdg_surface>, surface: object<wl_surface>)
	xdgWmBasePong             Opcode = 3 // pong(serial: uint)
)

// xdg_wm_base event opcodes
const (
	xdgWmBaseEventPing Opcode = 0 // ping(serial: uint)
)

// xdg_surface opcodes (requests)
const (
	xdgSurfaceDestroy           Opcode = 0 // destroy()
	xdgSurfaceGetToplevel       Opcode = 1 // get_toplevel(id: new_id<xdg_toplevel>)
	xdgSurfaceGetPopup          Opcode = 2 // get_popup(id: new_id<xdg_popup>, parent: object<xdg_surface>, positioner: object<xdg_positioner>)
	xdgSurfaceSetWindowGeometry Opcode = 3 // set_window_geometry(x: int, y: int, width: int, height: int)
	xdgSurfaceAckConfigure      Opcode = 4 // ack_configure(serial: uint)
)

// xdg_surface event opcodes
const (
	xdgSurfaceEventConfigure Opcode = 0 // configure(serial: uint)
)

// xdg_toplevel opcodes (requests)
const (
	xdgToplevelDestroy         Opcode = 0  // destroy()
	xdgToplevelSetParent       Opcode = 1  // set_parent(parent: object<xdg_toplevel>)
	xdgToplevelSetTitle        Opcode = 2  // set_title(title: string)
	xdgToplevelSetAppID        Opcode = 3  // set_app_id(app_id: string)
	xdgToplevelShowWindowMenu  Opcode = 4  // show_window_menu(seat: object<wl_seat>, serial: uint, x: int, y: int)
	xdgToplevelMove            Opcode = 5  // move(seat: object<wl_seat>, serial: uint)
	xdgToplevelResize          Opcode = 6  // resize(seat: object<wl_seat>, serial: uint, edges: uint)
	xdgToplevelSetMaxSize      Opcode = 7  // set_max_size(width: int, height: int)
	xdgToplevelSetMinSize      Opcode = 8  // set_min_size(width: int, height: int)
	xdgToplevelSetMaximized    Opcode = 9  // set_maximized()
	xdgToplevelUnsetMaximized  Opcode = 10 // unset_maximized()
	xdgToplevelSetFullscreen   Opcode = 11 // set_fullscreen(output: object<wl_output>)
	xdgToplevelUnsetFullscreen Opcode = 12 // unset_fullscreen()
	xdgToplevelSetMinimized    Opcode = 13 // set_minimized()
)

// xdg_toplevel event opcodes
const (
	xdgToplevelEventConfigure Opcode = 0 // configure(width: int, height: int, states: array)
	xdgToplevelEventClose     Opcode = 1 // close()
)

// XdgToplevel state values.
// These are passed in the states array of the configure event.
const (
	XdgToplevelStateMaximized   uint32 = 1 // Window is maximized
	XdgToplevelStateFullscreen  uint32 = 2 // Window is fullscreen
	XdgToplevelStateResizing    uint32 = 3 // Window is being resized
	XdgToplevelStateActivated   uint32 = 4 // Window is focused/activated
	XdgToplevelStateTiledLeft   uint32 = 5 // Window is tiled on left edge
	XdgToplevelStateTiledRight  uint32 = 6 // Window is tiled on right edge
	XdgToplevelStateTiledTop    uint32 = 7 // Window is tiled on top edge
	XdgToplevelStateTiledBottom uint32 = 8 // Window is tiled on bottom edge
)

// xdg_positioner opcodes (requests)
const (
	xdgPositionerDestroy             Opcode = 0 // destroy()
	xdgPositionerSetSize             Opcode = 1 // set_size(width: int, height: int)
	xdgPositionerSetAnchorRect       Opcode = 2 // set_anchor_rect(x: int, y: int, width: int, height: int)
	xdgPositionerSetAnchor           Opcode = 3 // set_anchor(anchor: uint)
	xdgPositionerSetGravity          Opcode = 4 // set_gravity(gravity: uint)
	xdgPositionerSetConstraintAdjust Opcode = 5 // set_constraint_adjustment(constraint_adjustment: uint)
	xdgPositionerSetOffset           Opcode = 6 // set_offset(x: int, y: int)
	xdgPositionerSetReactive         Opcode = 7 // set_reactive() [v3]
	xdgPositionerSetParentSize       Opcode = 8 // set_parent_size(parent_width: int, parent_height: int) [v3]
	xdgPositionerSetParentConfigure  Opcode = 9 // set_parent_configure(serial: uint) [v3]
)

// XdgWmBase represents the xdg_wm_base interface.
// This is the main interface for creating XDG shell surfaces (windows).
// It must respond to ping events to prove the client is responsive.
type XdgWmBase struct {
	display *Display
	id      ObjectID

	mu sync.Mutex

	// Event handlers
	onPing func(serial uint32)
}

// NewXdgWmBase creates an XdgWmBase from a bound object ID.
// The objectID should be obtained from Registry.BindXdgWmBase().
func NewXdgWmBase(display *Display, objectID ObjectID) *XdgWmBase {
	return &XdgWmBase{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID of the xdg_wm_base.
func (x *XdgWmBase) ID() ObjectID {
	return x.id
}

// Destroy destroys the xdg_wm_base object.
// All xdg_surface objects created through this interface must be destroyed first.
func (x *XdgWmBase) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(x.id, xdgWmBaseDestroy)

	return x.display.SendMessage(msg)
}

// CreatePositioner creates a new xdg_positioner.
// Positioners are used to position popups relative to their parent surface.
func (x *XdgWmBase) CreatePositioner() (*XdgPositioner, error) {
	positionerID := x.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(positionerID)
	msg := builder.BuildMessage(x.id, xdgWmBaseCreatePositioner)

	if err := x.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewXdgPositioner(x.display, positionerID), nil
}

// GetXdgSurface creates an XdgSurface for the given wl_surface.
// The xdg_surface interface is the basis for toplevel windows and popups.
func (x *XdgWmBase) GetXdgSurface(surface *WlSurface) (*XdgSurface, error) {
	xdgSurfaceID := x.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(xdgSurfaceID)
	builder.PutObject(surface.ID())
	msg := builder.BuildMessage(x.id, xdgWmBaseGetXdgSurface)

	if err := x.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewXdgSurface(x.display, xdgSurfaceID, surface), nil
}

// Pong responds to a ping request from the compositor.
// A client must respond to pings to prove it is not hung.
// This is typically called automatically by the event dispatcher.
func (x *XdgWmBase) Pong(serial uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	msg := builder.BuildMessage(x.id, xdgWmBasePong)

	return x.display.SendMessage(msg)
}

// SetPingHandler sets a callback for the ping event.
// By default, XdgWmBase auto-responds to pings. This handler is called
// after the auto-response if set.
func (x *XdgWmBase) SetPingHandler(handler func(serial uint32)) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.onPing = handler
}

// dispatch handles xdg_wm_base events.
func (x *XdgWmBase) dispatch(msg *Message) error {
	switch msg.Opcode {
	case xdgWmBaseEventPing:
		return x.handlePing(msg)
	default:
		return nil
	}
}

// handlePing handles the xdg_wm_base.ping event.
// It automatically responds with pong to keep the client alive.
func (x *XdgWmBase) handlePing(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_wm_base.ping: failed to decode serial: %w", err)
	}

	// Auto-respond to ping
	if err := x.Pong(serial); err != nil {
		return fmt.Errorf("wayland: xdg_wm_base.pong failed: %w", err)
	}

	// Call user handler if set
	x.mu.Lock()
	handler := x.onPing
	x.mu.Unlock()

	if handler != nil {
		handler(serial)
	}

	return nil
}

// XdgSurface represents the xdg_surface interface.
// An xdg_surface wraps a wl_surface and provides the foundation for
// toplevel windows and popup windows.
type XdgSurface struct {
	display *Display
	id      ObjectID
	surface *WlSurface

	mu sync.Mutex

	// Event handlers
	onConfigure func(serial uint32)

	// Pending configure serial
	pendingSerial uint32
	configured    bool
}

// NewXdgSurface creates an XdgSurface from an object ID.
func NewXdgSurface(display *Display, objectID ObjectID, surface *WlSurface) *XdgSurface {
	return &XdgSurface{
		display: display,
		id:      objectID,
		surface: surface,
	}
}

// ID returns the object ID of the xdg_surface.
func (s *XdgSurface) ID() ObjectID {
	return s.id
}

// Surface returns the underlying wl_surface.
func (s *XdgSurface) Surface() *WlSurface {
	return s.surface
}

// IsConfigured returns true if the surface has received at least one configure event.
func (s *XdgSurface) IsConfigured() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configured
}

// Destroy destroys the xdg_surface.
// The underlying wl_surface is not destroyed.
func (s *XdgSurface) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(s.id, xdgSurfaceDestroy)

	return s.display.SendMessage(msg)
}

// GetToplevel creates an xdg_toplevel role for this surface.
// The surface becomes a toplevel window (not a popup).
func (s *XdgSurface) GetToplevel() (*XdgToplevel, error) {
	toplevelID := s.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(toplevelID)
	msg := builder.BuildMessage(s.id, xdgSurfaceGetToplevel)

	if err := s.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewXdgToplevel(s.display, toplevelID, s), nil
}

// GetPopup creates an xdg_popup role for this surface.
// The parent parameter is the xdg_surface that this popup is relative to.
// The positioner determines where the popup appears.
func (s *XdgSurface) GetPopup(parent *XdgSurface, positioner *XdgPositioner) (*XdgPopup, error) {
	popupID := s.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(popupID)

	// Parent can be null for toplevel popups (rare)
	if parent != nil {
		builder.PutObject(parent.ID())
	} else {
		builder.PutObject(0)
	}

	builder.PutObject(positioner.ID())
	msg := builder.BuildMessage(s.id, xdgSurfaceGetPopup)

	if err := s.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewXdgPopup(s.display, popupID, s), nil
}

// SetWindowGeometry sets the window geometry.
// The geometry defines the visible bounds of the window, excluding
// decorations and shadows. This is important for proper sizing.
func (s *XdgSurface) SetWindowGeometry(x, y, width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(s.id, xdgSurfaceSetWindowGeometry)

	return s.display.SendMessage(msg)
}

// AckConfigure acknowledges a configure event.
// This must be called after receiving a configure event and applying
// the new state. The surface cannot be committed until this is done.
func (s *XdgSurface) AckConfigure(serial uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	msg := builder.BuildMessage(s.id, xdgSurfaceAckConfigure)

	return s.display.SendMessage(msg)
}

// SetConfigureHandler sets a callback for the configure event.
// The handler receives the serial number that must be acknowledged.
func (s *XdgSurface) SetConfigureHandler(handler func(serial uint32)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onConfigure = handler
}

// dispatch handles xdg_surface events.
func (s *XdgSurface) dispatch(msg *Message) error {
	switch msg.Opcode {
	case xdgSurfaceEventConfigure:
		return s.handleConfigure(msg)
	default:
		return nil
	}
}

// handleConfigure handles the xdg_surface.configure event.
func (s *XdgSurface) handleConfigure(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_surface.configure: failed to decode serial: %w", err)
	}

	s.mu.Lock()
	s.pendingSerial = serial
	s.configured = true
	handler := s.onConfigure
	s.mu.Unlock()

	if handler != nil {
		handler(serial)
	}

	return nil
}

// XdgToplevelConfig holds the configuration from a toplevel configure event.
type XdgToplevelConfig struct {
	// Width is the suggested width (0 means client chooses).
	Width int32

	// Height is the suggested height (0 means client chooses).
	Height int32

	// States contains the current window states.
	States []uint32

	// Helper booleans derived from States
	Maximized   bool
	Fullscreen  bool
	Resizing    bool
	Activated   bool
	TiledLeft   bool
	TiledRight  bool
	TiledTop    bool
	TiledBottom bool
}

// parseStates parses the states array and sets helper booleans.
func (c *XdgToplevelConfig) parseStates() {
	for _, state := range c.States {
		switch state {
		case XdgToplevelStateMaximized:
			c.Maximized = true
		case XdgToplevelStateFullscreen:
			c.Fullscreen = true
		case XdgToplevelStateResizing:
			c.Resizing = true
		case XdgToplevelStateActivated:
			c.Activated = true
		case XdgToplevelStateTiledLeft:
			c.TiledLeft = true
		case XdgToplevelStateTiledRight:
			c.TiledRight = true
		case XdgToplevelStateTiledTop:
			c.TiledTop = true
		case XdgToplevelStateTiledBottom:
			c.TiledBottom = true
		}
	}
}

// XdgToplevel represents the xdg_toplevel interface.
// This is the interface for top-level application windows.
type XdgToplevel struct {
	display    *Display
	id         ObjectID
	xdgSurface *XdgSurface

	mu sync.Mutex

	// Event handlers
	onConfigure func(config *XdgToplevelConfig)
	onClose     func()

	// Current state
	title string
	appID string
}

// NewXdgToplevel creates an XdgToplevel from an object ID.
func NewXdgToplevel(display *Display, objectID ObjectID, xdgSurface *XdgSurface) *XdgToplevel {
	return &XdgToplevel{
		display:    display,
		id:         objectID,
		xdgSurface: xdgSurface,
	}
}

// ID returns the object ID of the xdg_toplevel.
func (t *XdgToplevel) ID() ObjectID {
	return t.id
}

// XdgSurface returns the parent xdg_surface.
func (t *XdgToplevel) XdgSurface() *XdgSurface {
	return t.xdgSurface
}

// Surface returns the underlying wl_surface.
func (t *XdgToplevel) Surface() *WlSurface {
	return t.xdgSurface.Surface()
}

// Destroy destroys the xdg_toplevel.
// This removes the toplevel role from the xdg_surface.
func (t *XdgToplevel) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(t.id, xdgToplevelDestroy)

	return t.display.SendMessage(msg)
}

// SetParent sets the parent toplevel for this window.
// This creates a child/parent relationship (e.g., for dialogs).
// Pass nil to remove the parent.
func (t *XdgToplevel) SetParent(parent *XdgToplevel) error {
	builder := NewMessageBuilder()
	if parent != nil {
		builder.PutObject(parent.ID())
	} else {
		builder.PutObject(0)
	}
	msg := builder.BuildMessage(t.id, xdgToplevelSetParent)

	return t.display.SendMessage(msg)
}

// SetTitle sets the window title.
// This is shown in the title bar and task switchers.
func (t *XdgToplevel) SetTitle(title string) error {
	builder := NewMessageBuilder()
	builder.PutString(title)
	msg := builder.BuildMessage(t.id, xdgToplevelSetTitle)

	if err := t.display.SendMessage(msg); err != nil {
		return err
	}

	t.mu.Lock()
	t.title = title
	t.mu.Unlock()

	return nil
}

// Title returns the current window title.
func (t *XdgToplevel) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

// SetAppID sets the application ID.
// This identifies the application for desktop integration (icons, etc.).
// It should match the .desktop file name.
func (t *XdgToplevel) SetAppID(appID string) error {
	builder := NewMessageBuilder()
	builder.PutString(appID)
	msg := builder.BuildMessage(t.id, xdgToplevelSetAppID)

	if err := t.display.SendMessage(msg); err != nil {
		return err
	}

	t.mu.Lock()
	t.appID = appID
	t.mu.Unlock()

	return nil
}

// AppID returns the current application ID.
func (t *XdgToplevel) AppID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.appID
}

// ShowWindowMenu shows the window menu (right-click menu on title bar).
// The seat and serial identify the input event that triggered this.
func (t *XdgToplevel) ShowWindowMenu(seat ObjectID, serial uint32, x, y int32) error {
	builder := NewMessageBuilder()
	builder.PutObject(seat)
	builder.PutUint32(serial)
	builder.PutInt32(x)
	builder.PutInt32(y)
	msg := builder.BuildMessage(t.id, xdgToplevelShowWindowMenu)

	return t.display.SendMessage(msg)
}

// Move starts an interactive move operation.
// The seat and serial identify the input event that triggered this.
func (t *XdgToplevel) Move(seat ObjectID, serial uint32) error {
	builder := NewMessageBuilder()
	builder.PutObject(seat)
	builder.PutUint32(serial)
	msg := builder.BuildMessage(t.id, xdgToplevelMove)

	return t.display.SendMessage(msg)
}

// ResizeEdge values for interactive resize operations.
const (
	XdgToplevelResizeEdgeNone        uint32 = 0
	XdgToplevelResizeEdgeTop         uint32 = 1
	XdgToplevelResizeEdgeBottom      uint32 = 2
	XdgToplevelResizeEdgeLeft        uint32 = 4
	XdgToplevelResizeEdgeTopLeft     uint32 = 5
	XdgToplevelResizeEdgeBottomLeft  uint32 = 6
	XdgToplevelResizeEdgeRight       uint32 = 8
	XdgToplevelResizeEdgeTopRight    uint32 = 9
	XdgToplevelResizeEdgeBottomRight uint32 = 10
)

// Resize starts an interactive resize operation.
// The seat and serial identify the input event that triggered this.
// The edges parameter indicates which edge(s) are being resized.
func (t *XdgToplevel) Resize(seat ObjectID, serial uint32, edges uint32) error {
	builder := NewMessageBuilder()
	builder.PutObject(seat)
	builder.PutUint32(serial)
	builder.PutUint32(edges)
	msg := builder.BuildMessage(t.id, xdgToplevelResize)

	return t.display.SendMessage(msg)
}

// SetMaxSize sets the maximum window size.
// A size of 0 means no limit for that dimension.
func (t *XdgToplevel) SetMaxSize(width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(t.id, xdgToplevelSetMaxSize)

	return t.display.SendMessage(msg)
}

// SetMinSize sets the minimum window size.
// A size of 0 means no minimum for that dimension.
func (t *XdgToplevel) SetMinSize(width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(t.id, xdgToplevelSetMinSize)

	return t.display.SendMessage(msg)
}

// SetMaximized requests that the window be maximized.
// The compositor may or may not honor this request.
func (t *XdgToplevel) SetMaximized() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(t.id, xdgToplevelSetMaximized)

	return t.display.SendMessage(msg)
}

// UnsetMaximized requests that the window exit maximized state.
func (t *XdgToplevel) UnsetMaximized() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(t.id, xdgToplevelUnsetMaximized)

	return t.display.SendMessage(msg)
}

// SetFullscreen requests that the window go fullscreen.
// The output parameter specifies which output to use (nil for default).
func (t *XdgToplevel) SetFullscreen(output ObjectID) error {
	builder := NewMessageBuilder()
	builder.PutObject(output)
	msg := builder.BuildMessage(t.id, xdgToplevelSetFullscreen)

	return t.display.SendMessage(msg)
}

// UnsetFullscreen requests that the window exit fullscreen mode.
func (t *XdgToplevel) UnsetFullscreen() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(t.id, xdgToplevelUnsetFullscreen)

	return t.display.SendMessage(msg)
}

// SetMinimized requests that the window be minimized.
func (t *XdgToplevel) SetMinimized() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(t.id, xdgToplevelSetMinimized)

	return t.display.SendMessage(msg)
}

// Close sends a close request to the application.
// This is a hint that the user wants to close the window.
// The application should handle this by cleaning up and destroying the surface.
func (t *XdgToplevel) Close() error {
	// Note: There's no protocol request to close a toplevel.
	// The close event comes FROM the compositor.
	// This method just destroys the toplevel.
	return t.Destroy()
}

// SetConfigureHandler sets a callback for the configure event.
// The handler receives the suggested dimensions and window states.
func (t *XdgToplevel) SetConfigureHandler(handler func(config *XdgToplevelConfig)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConfigure = handler
}

// SetCloseHandler sets a callback for the close event.
// The handler is called when the compositor requests the window close.
func (t *XdgToplevel) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onClose = handler
}

// dispatch handles xdg_toplevel events.
func (t *XdgToplevel) dispatch(msg *Message) error {
	switch msg.Opcode {
	case xdgToplevelEventConfigure:
		return t.handleConfigure(msg)
	case xdgToplevelEventClose:
		return t.handleClose(msg)
	default:
		return nil
	}
}

// handleConfigure handles the xdg_toplevel.configure event.
func (t *XdgToplevel) handleConfigure(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	width, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_toplevel.configure: failed to decode width: %w", err)
	}

	height, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_toplevel.configure: failed to decode height: %w", err)
	}

	statesData, err := decoder.Array()
	if err != nil {
		return fmt.Errorf("wayland: xdg_toplevel.configure: failed to decode states: %w", err)
	}

	// Parse states array (array of uint32)
	states := make([]uint32, len(statesData)/4)
	for i := range states {
		states[i] = binary.LittleEndian.Uint32(statesData[i*4:])
	}

	config := &XdgToplevelConfig{
		Width:  width,
		Height: height,
		States: states,
	}
	config.parseStates()

	t.mu.Lock()
	handler := t.onConfigure
	t.mu.Unlock()

	if handler != nil {
		handler(config)
	}

	return nil
}

// handleClose handles the xdg_toplevel.close event.
func (t *XdgToplevel) handleClose(msg *Message) error {
	_ = msg // close event has no arguments

	t.mu.Lock()
	handler := t.onClose
	t.mu.Unlock()

	if handler != nil {
		handler()
	}

	return nil
}

// XdgPopup represents the xdg_popup interface.
// This is the interface for popup windows (menus, tooltips, etc.).
type XdgPopup struct {
	display    *Display
	id         ObjectID
	xdgSurface *XdgSurface

	mu sync.Mutex

	// Event handlers
	onConfigure    func(x, y, width, height int32)
	onPopupDone    func()
	onRepositioned func(token uint32)
}

// xdg_popup event opcodes
const (
	xdgPopupEventConfigure    Opcode = 0 // configure(x: int, y: int, width: int, height: int)
	xdgPopupEventPopupDone    Opcode = 1 // popup_done()
	xdgPopupEventRepositioned Opcode = 2 // repositioned(token: uint) [v3]
)

// xdg_popup opcodes (requests)
const (
	xdgPopupDestroy    Opcode = 0 // destroy()
	xdgPopupGrab       Opcode = 1 // grab(seat: object<wl_seat>, serial: uint)
	xdgPopupReposition Opcode = 2 // reposition(positioner: object<xdg_positioner>, token: uint) [v3]
)

// NewXdgPopup creates an XdgPopup from an object ID.
func NewXdgPopup(display *Display, objectID ObjectID, xdgSurface *XdgSurface) *XdgPopup {
	return &XdgPopup{
		display:    display,
		id:         objectID,
		xdgSurface: xdgSurface,
	}
}

// ID returns the object ID of the xdg_popup.
func (p *XdgPopup) ID() ObjectID {
	return p.id
}

// XdgSurface returns the parent xdg_surface.
func (p *XdgPopup) XdgSurface() *XdgSurface {
	return p.xdgSurface
}

// Surface returns the underlying wl_surface.
func (p *XdgPopup) Surface() *WlSurface {
	return p.xdgSurface.Surface()
}

// Destroy destroys the xdg_popup.
func (p *XdgPopup) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(p.id, xdgPopupDestroy)

	return p.display.SendMessage(msg)
}

// Grab grabs keyboard focus for this popup.
// This makes the popup receive keyboard input until it's dismissed.
// The seat and serial identify the input event that triggered this.
func (p *XdgPopup) Grab(seat ObjectID, serial uint32) error {
	builder := NewMessageBuilder()
	builder.PutObject(seat)
	builder.PutUint32(serial)
	msg := builder.BuildMessage(p.id, xdgPopupGrab)

	return p.display.SendMessage(msg)
}

// Reposition repositions the popup using a new positioner (v3+).
func (p *XdgPopup) Reposition(positioner *XdgPositioner, token uint32) error {
	builder := NewMessageBuilder()
	builder.PutObject(positioner.ID())
	builder.PutUint32(token)
	msg := builder.BuildMessage(p.id, xdgPopupReposition)

	return p.display.SendMessage(msg)
}

// SetConfigureHandler sets a callback for the configure event.
func (p *XdgPopup) SetConfigureHandler(handler func(x, y, width, height int32)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigure = handler
}

// SetPopupDoneHandler sets a callback for the popup_done event.
// This is called when the popup should be dismissed.
func (p *XdgPopup) SetPopupDoneHandler(handler func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onPopupDone = handler
}

// SetRepositionedHandler sets a callback for the repositioned event (v3+).
func (p *XdgPopup) SetRepositionedHandler(handler func(token uint32)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onRepositioned = handler
}

// dispatch handles xdg_popup events.
func (p *XdgPopup) dispatch(msg *Message) error {
	switch msg.Opcode {
	case xdgPopupEventConfigure:
		return p.handleConfigure(msg)
	case xdgPopupEventPopupDone:
		return p.handlePopupDone(msg)
	case xdgPopupEventRepositioned:
		return p.handleRepositioned(msg)
	default:
		return nil
	}
}

// handleConfigure handles the xdg_popup.configure event.
func (p *XdgPopup) handleConfigure(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	x, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_popup.configure: failed to decode x: %w", err)
	}

	y, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_popup.configure: failed to decode y: %w", err)
	}

	width, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_popup.configure: failed to decode width: %w", err)
	}

	height, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_popup.configure: failed to decode height: %w", err)
	}

	p.mu.Lock()
	handler := p.onConfigure
	p.mu.Unlock()

	if handler != nil {
		handler(x, y, width, height)
	}

	return nil
}

// handlePopupDone handles the xdg_popup.popup_done event.
func (p *XdgPopup) handlePopupDone(msg *Message) error {
	_ = msg // popup_done event has no arguments

	p.mu.Lock()
	handler := p.onPopupDone
	p.mu.Unlock()

	if handler != nil {
		handler()
	}

	return nil
}

// handleRepositioned handles the xdg_popup.repositioned event.
func (p *XdgPopup) handleRepositioned(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	token, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: xdg_popup.repositioned: failed to decode token: %w", err)
	}

	p.mu.Lock()
	handler := p.onRepositioned
	p.mu.Unlock()

	if handler != nil {
		handler(token)
	}

	return nil
}

// XdgPositioner represents the xdg_positioner interface.
// Positioners are used to position popup surfaces relative to their parent.
type XdgPositioner struct {
	display *Display
	id      ObjectID
}

// XdgPositionerAnchor values for anchor position.
const (
	XdgPositionerAnchorNone        uint32 = 0
	XdgPositionerAnchorTop         uint32 = 1
	XdgPositionerAnchorBottom      uint32 = 2
	XdgPositionerAnchorLeft        uint32 = 3
	XdgPositionerAnchorRight       uint32 = 4
	XdgPositionerAnchorTopLeft     uint32 = 5
	XdgPositionerAnchorBottomLeft  uint32 = 6
	XdgPositionerAnchorTopRight    uint32 = 7
	XdgPositionerAnchorBottomRight uint32 = 8
)

// XdgPositionerGravity values for gravity direction.
const (
	XdgPositionerGravityNone        uint32 = 0
	XdgPositionerGravityTop         uint32 = 1
	XdgPositionerGravityBottom      uint32 = 2
	XdgPositionerGravityLeft        uint32 = 3
	XdgPositionerGravityRight       uint32 = 4
	XdgPositionerGravityTopLeft     uint32 = 5
	XdgPositionerGravityBottomLeft  uint32 = 6
	XdgPositionerGravityTopRight    uint32 = 7
	XdgPositionerGravityBottomRight uint32 = 8
)

// XdgPositionerConstraintAdjustment flags for constraint adjustment.
const (
	XdgPositionerConstraintAdjustmentNone    uint32 = 0
	XdgPositionerConstraintAdjustmentSlideX  uint32 = 1
	XdgPositionerConstraintAdjustmentSlideY  uint32 = 2
	XdgPositionerConstraintAdjustmentFlipX   uint32 = 4
	XdgPositionerConstraintAdjustmentFlipY   uint32 = 8
	XdgPositionerConstraintAdjustmentResizeX uint32 = 16
	XdgPositionerConstraintAdjustmentResizeY uint32 = 32
)

// NewXdgPositioner creates an XdgPositioner from an object ID.
func NewXdgPositioner(display *Display, objectID ObjectID) *XdgPositioner {
	return &XdgPositioner{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID of the xdg_positioner.
func (p *XdgPositioner) ID() ObjectID {
	return p.id
}

// Destroy destroys the xdg_positioner.
func (p *XdgPositioner) Destroy() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(p.id, xdgPositionerDestroy)

	return p.display.SendMessage(msg)
}

// SetSize sets the size of the popup to be positioned.
func (p *XdgPositioner) SetSize(width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(p.id, xdgPositionerSetSize)

	return p.display.SendMessage(msg)
}

// SetAnchorRect sets the anchor rectangle in the parent surface coordinates.
// The popup is positioned relative to this rectangle.
func (p *XdgPositioner) SetAnchorRect(x, y, width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(p.id, xdgPositionerSetAnchorRect)

	return p.display.SendMessage(msg)
}

// SetAnchor sets the anchor point on the anchor rectangle.
func (p *XdgPositioner) SetAnchor(anchor uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(anchor)
	msg := builder.BuildMessage(p.id, xdgPositionerSetAnchor)

	return p.display.SendMessage(msg)
}

// SetGravity sets the direction the popup should grow from the anchor.
func (p *XdgPositioner) SetGravity(gravity uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(gravity)
	msg := builder.BuildMessage(p.id, xdgPositionerSetGravity)

	return p.display.SendMessage(msg)
}

// SetConstraintAdjustment sets how the popup position should be adjusted
// if it would be constrained by the compositor (e.g., off-screen).
func (p *XdgPositioner) SetConstraintAdjustment(adjustment uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(adjustment)
	msg := builder.BuildMessage(p.id, xdgPositionerSetConstraintAdjust)

	return p.display.SendMessage(msg)
}

// SetOffset sets an offset from the calculated position.
func (p *XdgPositioner) SetOffset(x, y int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(x)
	builder.PutInt32(y)
	msg := builder.BuildMessage(p.id, xdgPositionerSetOffset)

	return p.display.SendMessage(msg)
}

// SetReactive marks the popup as reactive (v3+).
// Reactive popups can be repositioned when the parent moves.
func (p *XdgPositioner) SetReactive() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(p.id, xdgPositionerSetReactive)

	return p.display.SendMessage(msg)
}

// SetParentSize sets the parent surface size for positioning (v3+).
func (p *XdgPositioner) SetParentSize(width, height int32) error {
	builder := NewMessageBuilder()
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(p.id, xdgPositionerSetParentSize)

	return p.display.SendMessage(msg)
}

// SetParentConfigure sets the parent configure serial (v3+).
func (p *XdgPositioner) SetParentConfigure(serial uint32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	msg := builder.BuildMessage(p.id, xdgPositionerSetParentConfigure)

	return p.display.SendMessage(msg)
}
