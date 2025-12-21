//go:build linux

package wayland

import (
	"fmt"
	"sync"
)

// wl_seat capability bitmask.
const (
	SeatCapabilityPointer  uint32 = 1 // The seat has pointer (mouse) devices.
	SeatCapabilityKeyboard uint32 = 2 // The seat has keyboard devices.
	SeatCapabilityTouch    uint32 = 4 // The seat has touch devices.
)

// wl_seat opcodes (requests).
const (
	seatGetPointer  Opcode = 0 // get_pointer(id: new_id<wl_pointer>)
	seatGetKeyboard Opcode = 1 // get_keyboard(id: new_id<wl_keyboard>)
	seatGetTouch    Opcode = 2 // get_touch(id: new_id<wl_touch>)
	seatRelease     Opcode = 3 // release() [v5+]
)

// wl_seat event opcodes.
const (
	seatEventCapabilities Opcode = 0 // capabilities(capabilities: uint)
	seatEventName         Opcode = 1 // name(name: string) [v2+]
)

// WlSeat represents the wl_seat interface.
// A seat is a group of input devices (keyboard, pointer, touch) that belong
// together (e.g., a laptop's built-in keyboard and touchpad).
type WlSeat struct {
	display *Display
	id      ObjectID
	version uint32

	mu sync.Mutex

	// Current state
	capabilities uint32
	name         string

	// Event handlers
	onCapabilities func(capabilities uint32)
	onName         func(name string)
}

// NewWlSeat creates a WlSeat from a bound object ID.
// The objectID should be obtained from Registry.BindSeat().
func NewWlSeat(display *Display, objectID ObjectID, version uint32) *WlSeat {
	return &WlSeat{
		display: display,
		id:      objectID,
		version: version,
	}
}

// ID returns the object ID of the seat.
func (s *WlSeat) ID() ObjectID {
	return s.id
}

// Version returns the interface version.
func (s *WlSeat) Version() uint32 {
	return s.version
}

// Capabilities returns the current capability bitmask.
func (s *WlSeat) Capabilities() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capabilities
}

// HasPointer returns true if the seat has a pointer device.
func (s *WlSeat) HasPointer() bool {
	return s.Capabilities()&SeatCapabilityPointer != 0
}

// HasKeyboard returns true if the seat has a keyboard device.
func (s *WlSeat) HasKeyboard() bool {
	return s.Capabilities()&SeatCapabilityKeyboard != 0
}

// HasTouch returns true if the seat has a touch device.
func (s *WlSeat) HasTouch() bool {
	return s.Capabilities()&SeatCapabilityTouch != 0
}

// Name returns the seat name (empty if not yet received or version < 2).
func (s *WlSeat) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// GetPointer creates a wl_pointer object for this seat.
// Returns an error if the seat does not have pointer capability.
func (s *WlSeat) GetPointer() (*WlPointer, error) {
	if !s.HasPointer() {
		return nil, fmt.Errorf("wayland: seat %d does not have pointer capability", s.id)
	}

	pointerID := s.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(pointerID)
	msg := builder.BuildMessage(s.id, seatGetPointer)

	if err := s.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewWlPointer(s.display, pointerID), nil
}

// GetKeyboard creates a wl_keyboard object for this seat.
// Returns an error if the seat does not have keyboard capability.
func (s *WlSeat) GetKeyboard() (*WlKeyboard, error) {
	if !s.HasKeyboard() {
		return nil, fmt.Errorf("wayland: seat %d does not have keyboard capability", s.id)
	}

	keyboardID := s.display.AllocID()

	builder := NewMessageBuilder()
	builder.PutNewID(keyboardID)
	msg := builder.BuildMessage(s.id, seatGetKeyboard)

	if err := s.display.SendMessage(msg); err != nil {
		return nil, err
	}

	return NewWlKeyboard(s.display, keyboardID), nil
}

// Release destroys the seat object (v5+).
// This releases any resources held by the server for this seat binding.
func (s *WlSeat) Release() error {
	if s.version < 5 {
		return fmt.Errorf("wayland: seat.release requires version 5+, have %d", s.version)
	}

	builder := NewMessageBuilder()
	msg := builder.BuildMessage(s.id, seatRelease)

	return s.display.SendMessage(msg)
}

// SetCapabilitiesHandler sets a callback for the capabilities event.
// The handler is called when the seat's capabilities change.
func (s *WlSeat) SetCapabilitiesHandler(handler func(capabilities uint32)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCapabilities = handler
}

// SetNameHandler sets a callback for the name event (v2+).
// The handler is called when the seat name is received.
func (s *WlSeat) SetNameHandler(handler func(name string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onName = handler
}

// dispatch handles wl_seat events.
func (s *WlSeat) dispatch(msg *Message) error {
	switch msg.Opcode {
	case seatEventCapabilities:
		return s.handleCapabilities(msg)
	case seatEventName:
		return s.handleName(msg)
	default:
		return nil
	}
}

// handleCapabilities handles the wl_seat.capabilities event.
func (s *WlSeat) handleCapabilities(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	capabilities, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_seat.capabilities: failed to decode: %w", err)
	}

	s.mu.Lock()
	s.capabilities = capabilities
	handler := s.onCapabilities
	s.mu.Unlock()

	if handler != nil {
		handler(capabilities)
	}

	return nil
}

// handleName handles the wl_seat.name event.
func (s *WlSeat) handleName(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	name, err := decoder.String()
	if err != nil {
		return fmt.Errorf("wayland: wl_seat.name: failed to decode: %w", err)
	}

	s.mu.Lock()
	s.name = name
	handler := s.onName
	s.mu.Unlock()

	if handler != nil {
		handler(name)
	}

	return nil
}

// Linux input event button codes (from linux/input-event-codes.h).
const (
	ButtonLeft   uint32 = 0x110 // Left mouse button (BTN_LEFT).
	ButtonRight  uint32 = 0x111 // Right mouse button (BTN_RIGHT).
	ButtonMiddle uint32 = 0x112 // Middle mouse button (BTN_MIDDLE).
	ButtonSide   uint32 = 0x113 // Side mouse button (BTN_SIDE).
	ButtonExtra  uint32 = 0x114 // Extra mouse button (BTN_EXTRA).
)

// Pointer button state values.
const (
	PointerButtonStateReleased uint32 = 0 // Button is not pressed.
	PointerButtonStatePressed  uint32 = 1 // Button is pressed.
)

// Pointer axis values.
const (
	PointerAxisVerticalScroll   uint32 = 0 // Vertical scroll axis.
	PointerAxisHorizontalScroll uint32 = 1 // Horizontal scroll axis.
)

// Pointer axis source values (v5+).
const (
	PointerAxisSourceWheel      uint32 = 0 // Scroll wheel.
	PointerAxisSourceFinger     uint32 = 1 // Finger on touchpad.
	PointerAxisSourceContinuous uint32 = 2 // Continuous coordinate space.
	PointerAxisSourceWheelTilt  uint32 = 3 // Wheel tilt.
)

// wl_pointer opcodes (requests).
const (
	pointerSetCursor Opcode = 0 // set_cursor(serial: uint, surface: object, hotspot_x: int, hotspot_y: int)
	pointerRelease   Opcode = 1 // release() [v3+]
)

// wl_pointer event opcodes.
const (
	pointerEventEnter        Opcode = 0 // enter(serial: uint, surface: object, surface_x: fixed, surface_y: fixed)
	pointerEventLeave        Opcode = 1 // leave(serial: uint, surface: object)
	pointerEventMotion       Opcode = 2 // motion(time: uint, surface_x: fixed, surface_y: fixed)
	pointerEventButton       Opcode = 3 // button(serial: uint, time: uint, button: uint, state: uint)
	pointerEventAxis         Opcode = 4 // axis(time: uint, axis: uint, value: fixed)
	pointerEventFrame        Opcode = 5 // frame() [v5+]
	pointerEventAxisSource   Opcode = 6 // axis_source(axis_source: uint) [v5+]
	pointerEventAxisStop     Opcode = 7 // axis_stop(time: uint, axis: uint) [v5+]
	pointerEventAxisDiscrete Opcode = 8 // axis_discrete(axis: uint, discrete: int) [v5+]
)

// PointerEnterEvent contains data for the pointer enter event.
type PointerEnterEvent struct {
	Serial   uint32   // Serial number for cursor changes.
	Surface  ObjectID // The surface the pointer entered.
	SurfaceX float64  // X position in surface-local coordinates.
	SurfaceY float64  // Y position in surface-local coordinates.
}

// PointerLeaveEvent contains data for the pointer leave event.
type PointerLeaveEvent struct {
	Serial  uint32   // Serial number.
	Surface ObjectID // The surface the pointer left.
}

// PointerMotionEvent contains data for the pointer motion event.
type PointerMotionEvent struct {
	Time     uint32  // Timestamp in milliseconds.
	SurfaceX float64 // X position in surface-local coordinates.
	SurfaceY float64 // Y position in surface-local coordinates.
}

// PointerButtonEvent contains data for the pointer button event.
type PointerButtonEvent struct {
	Serial uint32 // Serial number for grabs.
	Time   uint32 // Timestamp in milliseconds.
	Button uint32 // Button code (BTN_LEFT, BTN_RIGHT, etc.).
	State  uint32 // Button state (pressed/released).
}

// PointerAxisEvent contains data for the pointer axis (scroll) event.
type PointerAxisEvent struct {
	Time  uint32  // Timestamp in milliseconds.
	Axis  uint32  // Axis type (vertical/horizontal).
	Value float64 // Scroll amount (positive = down/right).
}

// WlPointer represents the wl_pointer interface.
// This interface provides access to pointer (mouse) input events.
type WlPointer struct {
	display *Display
	id      ObjectID

	mu sync.Mutex

	// Current state (updated by events)
	enteredSurface ObjectID
	surfaceX       float64
	surfaceY       float64
	lastSerial     uint32

	// Event handlers
	onEnter        func(event *PointerEnterEvent)
	onLeave        func(event *PointerLeaveEvent)
	onMotion       func(event *PointerMotionEvent)
	onButton       func(event *PointerButtonEvent)
	onAxis         func(event *PointerAxisEvent)
	onFrame        func()
	onAxisSource   func(source uint32)
	onAxisStop     func(time uint32, axis uint32)
	onAxisDiscrete func(axis uint32, discrete int32)
}

// NewWlPointer creates a WlPointer from an object ID.
func NewWlPointer(display *Display, objectID ObjectID) *WlPointer {
	return &WlPointer{
		display: display,
		id:      objectID,
	}
}

// ID returns the object ID of the pointer.
func (p *WlPointer) ID() ObjectID {
	return p.id
}

// EnteredSurface returns the surface the pointer is currently over.
// Returns 0 if the pointer is not over any surface.
func (p *WlPointer) EnteredSurface() ObjectID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enteredSurface
}

// Position returns the current pointer position in surface-local coordinates.
func (p *WlPointer) Position() (x, y float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.surfaceX, p.surfaceY
}

// LastSerial returns the last event serial (useful for cursor changes).
func (p *WlPointer) LastSerial() uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastSerial
}

// SetCursor sets the pointer cursor image.
// The serial should be from a recent pointer event (enter/button/motion).
// Pass surface=0 to hide the cursor.
// hotspotX/hotspotY specify the cursor hotspot offset within the surface.
func (p *WlPointer) SetCursor(serial uint32, surface *WlSurface, hotspotX, hotspotY int32) error {
	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	if surface != nil {
		builder.PutObject(surface.ID())
	} else {
		builder.PutObject(0)
	}
	builder.PutInt32(hotspotX)
	builder.PutInt32(hotspotY)
	msg := builder.BuildMessage(p.id, pointerSetCursor)

	return p.display.SendMessage(msg)
}

// Release destroys the pointer object (v3+).
func (p *WlPointer) Release() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(p.id, pointerRelease)

	return p.display.SendMessage(msg)
}

// SetEnterHandler sets a callback for the enter event.
func (p *WlPointer) SetEnterHandler(handler func(event *PointerEnterEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEnter = handler
}

// SetLeaveHandler sets a callback for the leave event.
func (p *WlPointer) SetLeaveHandler(handler func(event *PointerLeaveEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onLeave = handler
}

// SetMotionHandler sets a callback for the motion event.
func (p *WlPointer) SetMotionHandler(handler func(event *PointerMotionEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onMotion = handler
}

// SetButtonHandler sets a callback for the button event.
func (p *WlPointer) SetButtonHandler(handler func(event *PointerButtonEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onButton = handler
}

// SetAxisHandler sets a callback for the axis (scroll) event.
func (p *WlPointer) SetAxisHandler(handler func(event *PointerAxisEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onAxis = handler
}

// SetFrameHandler sets a callback for the frame event (v5+).
// The frame event marks the end of a group of related events.
func (p *WlPointer) SetFrameHandler(handler func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onFrame = handler
}

// SetAxisSourceHandler sets a callback for the axis_source event (v5+).
func (p *WlPointer) SetAxisSourceHandler(handler func(source uint32)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onAxisSource = handler
}

// SetAxisStopHandler sets a callback for the axis_stop event (v5+).
func (p *WlPointer) SetAxisStopHandler(handler func(time uint32, axis uint32)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onAxisStop = handler
}

// SetAxisDiscreteHandler sets a callback for the axis_discrete event (v5+).
func (p *WlPointer) SetAxisDiscreteHandler(handler func(axis uint32, discrete int32)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onAxisDiscrete = handler
}

// dispatch handles wl_pointer events.
func (p *WlPointer) dispatch(msg *Message) error {
	switch msg.Opcode {
	case pointerEventEnter:
		return p.handleEnter(msg)
	case pointerEventLeave:
		return p.handleLeave(msg)
	case pointerEventMotion:
		return p.handleMotion(msg)
	case pointerEventButton:
		return p.handleButton(msg)
	case pointerEventAxis:
		return p.handleAxis(msg)
	case pointerEventFrame:
		return p.handleFrame(msg)
	case pointerEventAxisSource:
		return p.handleAxisSource(msg)
	case pointerEventAxisStop:
		return p.handleAxisStop(msg)
	case pointerEventAxisDiscrete:
		return p.handleAxisDiscrete(msg)
	default:
		return nil
	}
}

func (p *WlPointer) handleEnter(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.enter: failed to decode serial: %w", err)
	}

	surface, err := decoder.Object()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.enter: failed to decode surface: %w", err)
	}

	surfaceXFixed, err := decoder.Fixed()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.enter: failed to decode surface_x: %w", err)
	}

	surfaceYFixed, err := decoder.Fixed()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.enter: failed to decode surface_y: %w", err)
	}

	surfaceX := surfaceXFixed.Float()
	surfaceY := surfaceYFixed.Float()

	p.mu.Lock()
	p.enteredSurface = surface
	p.surfaceX = surfaceX
	p.surfaceY = surfaceY
	p.lastSerial = serial
	handler := p.onEnter
	p.mu.Unlock()

	if handler != nil {
		handler(&PointerEnterEvent{
			Serial:   serial,
			Surface:  surface,
			SurfaceX: surfaceX,
			SurfaceY: surfaceY,
		})
	}

	return nil
}

func (p *WlPointer) handleLeave(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.leave: failed to decode serial: %w", err)
	}

	surface, err := decoder.Object()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.leave: failed to decode surface: %w", err)
	}

	p.mu.Lock()
	p.enteredSurface = 0
	p.lastSerial = serial
	handler := p.onLeave
	p.mu.Unlock()

	if handler != nil {
		handler(&PointerLeaveEvent{
			Serial:  serial,
			Surface: surface,
		})
	}

	return nil
}

func (p *WlPointer) handleMotion(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	time, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.motion: failed to decode time: %w", err)
	}

	surfaceXFixed, err := decoder.Fixed()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.motion: failed to decode surface_x: %w", err)
	}

	surfaceYFixed, err := decoder.Fixed()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.motion: failed to decode surface_y: %w", err)
	}

	surfaceX := surfaceXFixed.Float()
	surfaceY := surfaceYFixed.Float()

	p.mu.Lock()
	p.surfaceX = surfaceX
	p.surfaceY = surfaceY
	handler := p.onMotion
	p.mu.Unlock()

	if handler != nil {
		handler(&PointerMotionEvent{
			Time:     time,
			SurfaceX: surfaceX,
			SurfaceY: surfaceY,
		})
	}

	return nil
}

func (p *WlPointer) handleButton(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.button: failed to decode serial: %w", err)
	}

	time, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.button: failed to decode time: %w", err)
	}

	button, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.button: failed to decode button: %w", err)
	}

	state, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.button: failed to decode state: %w", err)
	}

	p.mu.Lock()
	p.lastSerial = serial
	handler := p.onButton
	p.mu.Unlock()

	if handler != nil {
		handler(&PointerButtonEvent{
			Serial: serial,
			Time:   time,
			Button: button,
			State:  state,
		})
	}

	return nil
}

func (p *WlPointer) handleAxis(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	time, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis: failed to decode time: %w", err)
	}

	axis, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis: failed to decode axis: %w", err)
	}

	valueFixed, err := decoder.Fixed()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis: failed to decode value: %w", err)
	}

	value := valueFixed.Float()

	p.mu.Lock()
	handler := p.onAxis
	p.mu.Unlock()

	if handler != nil {
		handler(&PointerAxisEvent{
			Time:  time,
			Axis:  axis,
			Value: value,
		})
	}

	return nil
}

func (p *WlPointer) handleFrame(msg *Message) error {
	_ = msg // frame event has no arguments

	p.mu.Lock()
	handler := p.onFrame
	p.mu.Unlock()

	if handler != nil {
		handler()
	}

	return nil
}

func (p *WlPointer) handleAxisSource(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	source, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis_source: failed to decode: %w", err)
	}

	p.mu.Lock()
	handler := p.onAxisSource
	p.mu.Unlock()

	if handler != nil {
		handler(source)
	}

	return nil
}

func (p *WlPointer) handleAxisStop(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	time, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis_stop: failed to decode time: %w", err)
	}

	axis, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis_stop: failed to decode axis: %w", err)
	}

	p.mu.Lock()
	handler := p.onAxisStop
	p.mu.Unlock()

	if handler != nil {
		handler(time, axis)
	}

	return nil
}

func (p *WlPointer) handleAxisDiscrete(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	axis, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis_discrete: failed to decode axis: %w", err)
	}

	discrete, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: wl_pointer.axis_discrete: failed to decode discrete: %w", err)
	}

	p.mu.Lock()
	handler := p.onAxisDiscrete
	p.mu.Unlock()

	if handler != nil {
		handler(axis, discrete)
	}

	return nil
}

// Keyboard keymap format values.
const (
	KeyboardKeymapFormatNoKeymap uint32 = 0 // No keymap; client must interpret raw keycodes.
	KeyboardKeymapFormatXKBV1    uint32 = 1 // XKB keymap format.
)

// Keyboard key state values.
const (
	KeyStateReleased uint32 = 0 // Key is released.
	KeyStatePressed  uint32 = 1 // Key is pressed.
)

// wl_keyboard opcodes (requests).
const (
	keyboardRelease Opcode = 0 // release() [v3+]
)

// wl_keyboard event opcodes.
const (
	keyboardEventKeymap     Opcode = 0 // keymap(format: uint, fd: fd, size: uint)
	keyboardEventEnter      Opcode = 1 // enter(serial: uint, surface: object, keys: array)
	keyboardEventLeave      Opcode = 2 // leave(serial: uint, surface: object)
	keyboardEventKey        Opcode = 3 // key(serial: uint, time: uint, key: uint, state: uint)
	keyboardEventModifiers  Opcode = 4 // modifiers(serial: uint, mods_depressed: uint, mods_latched: uint, mods_locked: uint, group: uint)
	keyboardEventRepeatInfo Opcode = 5 // repeat_info(rate: int, delay: int) [v4+]
)

// KeyboardKeymapEvent contains data for the keymap event.
type KeyboardKeymapEvent struct {
	Format uint32 // Keymap format (XKB or none).
	FD     int    // File descriptor containing the keymap.
	Size   uint32 // Size of the keymap data.
}

// KeyboardEnterEvent contains data for the keyboard enter event.
type KeyboardEnterEvent struct {
	Serial  uint32   // Serial number.
	Surface ObjectID // The surface that gained keyboard focus.
	Keys    []uint32 // Array of currently pressed keys.
}

// KeyboardLeaveEvent contains data for the keyboard leave event.
type KeyboardLeaveEvent struct {
	Serial  uint32   // Serial number.
	Surface ObjectID // The surface that lost keyboard focus.
}

// KeyboardKeyEvent contains data for the key event.
type KeyboardKeyEvent struct {
	Serial uint32 // Serial number.
	Time   uint32 // Timestamp in milliseconds.
	Key    uint32 // Key code (Linux evdev key code).
	State  uint32 // Key state (pressed/released).
}

// KeyboardModifiersEvent contains data for the modifiers event.
type KeyboardModifiersEvent struct {
	Serial        uint32 // Serial number.
	ModsDepressed uint32 // Currently pressed modifiers.
	ModsLatched   uint32 // Latched modifiers (e.g., Caps Lock toggled once).
	ModsLocked    uint32 // Locked modifiers (e.g., Caps Lock on).
	Group         uint32 // Keyboard layout group.
}

// KeyboardRepeatInfo contains data for the repeat_info event.
type KeyboardRepeatInfo struct {
	Rate  int32 // Key repeat rate in characters per second.
	Delay int32 // Delay before key repeat starts in milliseconds.
}

// WlKeyboard represents the wl_keyboard interface.
// This interface provides access to keyboard input events.
type WlKeyboard struct {
	display *Display
	id      ObjectID

	mu sync.Mutex

	// Current state
	focusedSurface ObjectID
	lastSerial     uint32

	// Keymap file descriptor (needs to be closed by the application)
	keymapFD   int
	keymapSize uint32

	// Key repeat info
	repeatRate  int32
	repeatDelay int32

	// Event handlers
	onKeymap     func(event *KeyboardKeymapEvent)
	onEnter      func(event *KeyboardEnterEvent)
	onLeave      func(event *KeyboardLeaveEvent)
	onKey        func(event *KeyboardKeyEvent)
	onModifiers  func(event *KeyboardModifiersEvent)
	onRepeatInfo func(info *KeyboardRepeatInfo)
}

// NewWlKeyboard creates a WlKeyboard from an object ID.
func NewWlKeyboard(display *Display, objectID ObjectID) *WlKeyboard {
	return &WlKeyboard{
		display:     display,
		id:          objectID,
		keymapFD:    -1,
		repeatRate:  25,  // Default: 25 chars/sec
		repeatDelay: 400, // Default: 400ms
	}
}

// ID returns the object ID of the keyboard.
func (k *WlKeyboard) ID() ObjectID {
	return k.id
}

// FocusedSurface returns the surface that currently has keyboard focus.
// Returns 0 if no surface has focus.
func (k *WlKeyboard) FocusedSurface() ObjectID {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.focusedSurface
}

// LastSerial returns the last event serial.
func (k *WlKeyboard) LastSerial() uint32 {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lastSerial
}

// KeymapFD returns the file descriptor for the keymap.
// Returns -1 if no keymap has been received.
// The caller is responsible for closing this FD when done.
func (k *WlKeyboard) KeymapFD() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.keymapFD
}

// KeymapSize returns the size of the keymap data.
func (k *WlKeyboard) KeymapSize() uint32 {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.keymapSize
}

// RepeatInfo returns the key repeat rate and delay.
func (k *WlKeyboard) RepeatInfo() (rate, delay int32) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.repeatRate, k.repeatDelay
}

// Release destroys the keyboard object (v3+).
func (k *WlKeyboard) Release() error {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(k.id, keyboardRelease)

	return k.display.SendMessage(msg)
}

// SetKeymapHandler sets a callback for the keymap event.
// The handler receives the keymap format, file descriptor, and size.
// Note: The FD must be closed by the application when no longer needed.
func (k *WlKeyboard) SetKeymapHandler(handler func(event *KeyboardKeymapEvent)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onKeymap = handler
}

// SetEnterHandler sets a callback for the enter event.
func (k *WlKeyboard) SetEnterHandler(handler func(event *KeyboardEnterEvent)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onEnter = handler
}

// SetLeaveHandler sets a callback for the leave event.
func (k *WlKeyboard) SetLeaveHandler(handler func(event *KeyboardLeaveEvent)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onLeave = handler
}

// SetKeyHandler sets a callback for the key event.
func (k *WlKeyboard) SetKeyHandler(handler func(event *KeyboardKeyEvent)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onKey = handler
}

// SetModifiersHandler sets a callback for the modifiers event.
func (k *WlKeyboard) SetModifiersHandler(handler func(event *KeyboardModifiersEvent)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onModifiers = handler
}

// SetRepeatInfoHandler sets a callback for the repeat_info event (v4+).
func (k *WlKeyboard) SetRepeatInfoHandler(handler func(info *KeyboardRepeatInfo)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onRepeatInfo = handler
}

// dispatch handles wl_keyboard events.
func (k *WlKeyboard) dispatch(msg *Message) error {
	switch msg.Opcode {
	case keyboardEventKeymap:
		return k.handleKeymap(msg)
	case keyboardEventEnter:
		return k.handleEnter(msg)
	case keyboardEventLeave:
		return k.handleLeave(msg)
	case keyboardEventKey:
		return k.handleKey(msg)
	case keyboardEventModifiers:
		return k.handleModifiers(msg)
	case keyboardEventRepeatInfo:
		return k.handleRepeatInfo(msg)
	default:
		return nil
	}
}

func (k *WlKeyboard) handleKeymap(msg *Message) error {
	decoder := NewDecoder(msg.Args)
	decoder.fds = msg.FDs

	format, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.keymap: failed to decode format: %w", err)
	}

	fd, err := decoder.FD()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.keymap: failed to get fd: %w", err)
	}

	size, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.keymap: failed to decode size: %w", err)
	}

	k.mu.Lock()
	k.keymapFD = fd
	k.keymapSize = size
	handler := k.onKeymap
	k.mu.Unlock()

	if handler != nil {
		handler(&KeyboardKeymapEvent{
			Format: format,
			FD:     fd,
			Size:   size,
		})
	}

	return nil
}

func (k *WlKeyboard) handleEnter(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.enter: failed to decode serial: %w", err)
	}

	surface, err := decoder.Object()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.enter: failed to decode surface: %w", err)
	}

	keysData, err := decoder.Array()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.enter: failed to decode keys: %w", err)
	}

	// Parse keys array (array of uint32)
	keys := make([]uint32, len(keysData)/4)
	for i := range keys {
		keys[i] = uint32(keysData[i*4]) |
			uint32(keysData[i*4+1])<<8 |
			uint32(keysData[i*4+2])<<16 |
			uint32(keysData[i*4+3])<<24
	}

	k.mu.Lock()
	k.focusedSurface = surface
	k.lastSerial = serial
	handler := k.onEnter
	k.mu.Unlock()

	if handler != nil {
		handler(&KeyboardEnterEvent{
			Serial:  serial,
			Surface: surface,
			Keys:    keys,
		})
	}

	return nil
}

func (k *WlKeyboard) handleLeave(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.leave: failed to decode serial: %w", err)
	}

	surface, err := decoder.Object()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.leave: failed to decode surface: %w", err)
	}

	k.mu.Lock()
	k.focusedSurface = 0
	k.lastSerial = serial
	handler := k.onLeave
	k.mu.Unlock()

	if handler != nil {
		handler(&KeyboardLeaveEvent{
			Serial:  serial,
			Surface: surface,
		})
	}

	return nil
}

func (k *WlKeyboard) handleKey(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.key: failed to decode serial: %w", err)
	}

	time, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.key: failed to decode time: %w", err)
	}

	key, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.key: failed to decode key: %w", err)
	}

	state, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.key: failed to decode state: %w", err)
	}

	k.mu.Lock()
	k.lastSerial = serial
	handler := k.onKey
	k.mu.Unlock()

	if handler != nil {
		handler(&KeyboardKeyEvent{
			Serial: serial,
			Time:   time,
			Key:    key,
			State:  state,
		})
	}

	return nil
}

func (k *WlKeyboard) handleModifiers(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	serial, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.modifiers: failed to decode serial: %w", err)
	}

	modsDepressed, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.modifiers: failed to decode mods_depressed: %w", err)
	}

	modsLatched, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.modifiers: failed to decode mods_latched: %w", err)
	}

	modsLocked, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.modifiers: failed to decode mods_locked: %w", err)
	}

	group, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.modifiers: failed to decode group: %w", err)
	}

	k.mu.Lock()
	k.lastSerial = serial
	handler := k.onModifiers
	k.mu.Unlock()

	if handler != nil {
		handler(&KeyboardModifiersEvent{
			Serial:        serial,
			ModsDepressed: modsDepressed,
			ModsLatched:   modsLatched,
			ModsLocked:    modsLocked,
			Group:         group,
		})
	}

	return nil
}

func (k *WlKeyboard) handleRepeatInfo(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	rate, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.repeat_info: failed to decode rate: %w", err)
	}

	delay, err := decoder.Int32()
	if err != nil {
		return fmt.Errorf("wayland: wl_keyboard.repeat_info: failed to decode delay: %w", err)
	}

	k.mu.Lock()
	k.repeatRate = rate
	k.repeatDelay = delay
	handler := k.onRepeatInfo
	k.mu.Unlock()

	if handler != nil {
		handler(&KeyboardRepeatInfo{
			Rate:  rate,
			Delay: delay,
		})
	}

	return nil
}
