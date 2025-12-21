//go:build linux

package wayland

import (
	"encoding/binary"
	"testing"
)

// TestSeatCapabilityConstants verifies seat capability bitmask values.
func TestSeatCapabilityConstants(t *testing.T) {
	tests := []struct {
		name     string
		cap      uint32
		expected uint32
	}{
		{"pointer", SeatCapabilityPointer, 1},
		{"keyboard", SeatCapabilityKeyboard, 2},
		{"touch", SeatCapabilityTouch, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cap != tt.expected {
				t.Errorf("capability %s = %d, want %d", tt.name, tt.cap, tt.expected)
			}
		})
	}
}

// TestSeatOpcodes verifies wl_seat opcode constants match Wayland protocol spec.
func TestSeatOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"get_pointer", seatGetPointer, 0},
		{"get_keyboard", seatGetKeyboard, 1},
		{"get_touch", seatGetTouch, 2},
		{"release", seatRelease, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestSeatEventOpcodes verifies wl_seat event opcode constants.
func TestSeatEventOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"capabilities", seatEventCapabilities, 0},
		{"name", seatEventName, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("event opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestPointerButtonConstants verifies Linux input button codes.
func TestPointerButtonConstants(t *testing.T) {
	tests := []struct {
		name     string
		button   uint32
		expected uint32
	}{
		{"left", ButtonLeft, 0x110},
		{"right", ButtonRight, 0x111},
		{"middle", ButtonMiddle, 0x112},
		{"side", ButtonSide, 0x113},
		{"extra", ButtonExtra, 0x114},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.button != tt.expected {
				t.Errorf("button %s = 0x%x, want 0x%x", tt.name, tt.button, tt.expected)
			}
		})
	}
}

// TestPointerButtonStateConstants verifies button state values.
func TestPointerButtonStateConstants(t *testing.T) {
	if PointerButtonStateReleased != 0 {
		t.Errorf("PointerButtonStateReleased = %d, want 0", PointerButtonStateReleased)
	}
	if PointerButtonStatePressed != 1 {
		t.Errorf("PointerButtonStatePressed = %d, want 1", PointerButtonStatePressed)
	}
}

// TestPointerAxisConstants verifies pointer axis values.
func TestPointerAxisConstants(t *testing.T) {
	if PointerAxisVerticalScroll != 0 {
		t.Errorf("PointerAxisVerticalScroll = %d, want 0", PointerAxisVerticalScroll)
	}
	if PointerAxisHorizontalScroll != 1 {
		t.Errorf("PointerAxisHorizontalScroll = %d, want 1", PointerAxisHorizontalScroll)
	}
}

// TestPointerAxisSourceConstants verifies pointer axis source values (v5+).
func TestPointerAxisSourceConstants(t *testing.T) {
	tests := []struct {
		name     string
		source   uint32
		expected uint32
	}{
		{"wheel", PointerAxisSourceWheel, 0},
		{"finger", PointerAxisSourceFinger, 1},
		{"continuous", PointerAxisSourceContinuous, 2},
		{"wheel_tilt", PointerAxisSourceWheelTilt, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.source != tt.expected {
				t.Errorf("axis source %s = %d, want %d", tt.name, tt.source, tt.expected)
			}
		})
	}
}

// TestPointerOpcodes verifies wl_pointer opcode constants.
func TestPointerOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"set_cursor", pointerSetCursor, 0},
		{"release", pointerRelease, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestPointerEventOpcodes verifies wl_pointer event opcode constants.
func TestPointerEventOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"enter", pointerEventEnter, 0},
		{"leave", pointerEventLeave, 1},
		{"motion", pointerEventMotion, 2},
		{"button", pointerEventButton, 3},
		{"axis", pointerEventAxis, 4},
		{"frame", pointerEventFrame, 5},
		{"axis_source", pointerEventAxisSource, 6},
		{"axis_stop", pointerEventAxisStop, 7},
		{"axis_discrete", pointerEventAxisDiscrete, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("event opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestKeyboardKeymapFormatConstants verifies keymap format values.
func TestKeyboardKeymapFormatConstants(t *testing.T) {
	if KeyboardKeymapFormatNoKeymap != 0 {
		t.Errorf("KeyboardKeymapFormatNoKeymap = %d, want 0", KeyboardKeymapFormatNoKeymap)
	}
	if KeyboardKeymapFormatXKBV1 != 1 {
		t.Errorf("KeyboardKeymapFormatXKBV1 = %d, want 1", KeyboardKeymapFormatXKBV1)
	}
}

// TestKeyStateConstants verifies key state values.
func TestKeyStateConstants(t *testing.T) {
	if KeyStateReleased != 0 {
		t.Errorf("KeyStateReleased = %d, want 0", KeyStateReleased)
	}
	if KeyStatePressed != 1 {
		t.Errorf("KeyStatePressed = %d, want 1", KeyStatePressed)
	}
}

// TestKeyboardOpcodes verifies wl_keyboard opcode constants.
func TestKeyboardOpcodes(t *testing.T) {
	if keyboardRelease != 0 {
		t.Errorf("keyboardRelease = %d, want 0", keyboardRelease)
	}
}

// TestKeyboardEventOpcodes verifies wl_keyboard event opcode constants.
func TestKeyboardEventOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"keymap", keyboardEventKeymap, 0},
		{"enter", keyboardEventEnter, 1},
		{"leave", keyboardEventLeave, 2},
		{"key", keyboardEventKey, 3},
		{"modifiers", keyboardEventModifiers, 4},
		{"repeat_info", keyboardEventRepeatInfo, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("event opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestWlSeatCreation verifies WlSeat struct initialization.
func TestWlSeatCreation(t *testing.T) {
	seat := NewWlSeat(nil, ObjectID(5), 7)

	if seat.ID() != ObjectID(5) {
		t.Errorf("WlSeat.ID() = %d, want 5", seat.ID())
	}
	if seat.Version() != 7 {
		t.Errorf("WlSeat.Version() = %d, want 7", seat.Version())
	}
	if seat.Capabilities() != 0 {
		t.Errorf("WlSeat.Capabilities() = %d, want 0", seat.Capabilities())
	}
	if seat.Name() != "" {
		t.Errorf("WlSeat.Name() = %q, want empty", seat.Name())
	}
}

// TestWlSeatHasCapabilities verifies capability checking methods.
func TestWlSeatHasCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		capabilities uint32
		hasPointer   bool
		hasKeyboard  bool
		hasTouch     bool
	}{
		{"none", 0, false, false, false},
		{"pointer only", SeatCapabilityPointer, true, false, false},
		{"keyboard only", SeatCapabilityKeyboard, false, true, false},
		{"touch only", SeatCapabilityTouch, false, false, true},
		{"pointer+keyboard", SeatCapabilityPointer | SeatCapabilityKeyboard, true, true, false},
		{"all", SeatCapabilityPointer | SeatCapabilityKeyboard | SeatCapabilityTouch, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seat := NewWlSeat(nil, ObjectID(1), 1)
			seat.capabilities = tt.capabilities

			if seat.HasPointer() != tt.hasPointer {
				t.Errorf("HasPointer() = %v, want %v", seat.HasPointer(), tt.hasPointer)
			}
			if seat.HasKeyboard() != tt.hasKeyboard {
				t.Errorf("HasKeyboard() = %v, want %v", seat.HasKeyboard(), tt.hasKeyboard)
			}
			if seat.HasTouch() != tt.hasTouch {
				t.Errorf("HasTouch() = %v, want %v", seat.HasTouch(), tt.hasTouch)
			}
		})
	}
}

// TestWlPointerCreation verifies WlPointer struct initialization.
func TestWlPointerCreation(t *testing.T) {
	pointer := NewWlPointer(nil, ObjectID(10))

	if pointer.ID() != ObjectID(10) {
		t.Errorf("WlPointer.ID() = %d, want 10", pointer.ID())
	}
	if pointer.EnteredSurface() != 0 {
		t.Errorf("WlPointer.EnteredSurface() = %d, want 0", pointer.EnteredSurface())
	}
	x, y := pointer.Position()
	if x != 0 || y != 0 {
		t.Errorf("WlPointer.Position() = (%f, %f), want (0, 0)", x, y)
	}
	if pointer.LastSerial() != 0 {
		t.Errorf("WlPointer.LastSerial() = %d, want 0", pointer.LastSerial())
	}
}

// TestWlKeyboardCreation verifies WlKeyboard struct initialization.
func TestWlKeyboardCreation(t *testing.T) {
	keyboard := NewWlKeyboard(nil, ObjectID(20))

	if keyboard.ID() != ObjectID(20) {
		t.Errorf("WlKeyboard.ID() = %d, want 20", keyboard.ID())
	}
	if keyboard.FocusedSurface() != 0 {
		t.Errorf("WlKeyboard.FocusedSurface() = %d, want 0", keyboard.FocusedSurface())
	}
	if keyboard.LastSerial() != 0 {
		t.Errorf("WlKeyboard.LastSerial() = %d, want 0", keyboard.LastSerial())
	}
	if keyboard.KeymapFD() != -1 {
		t.Errorf("WlKeyboard.KeymapFD() = %d, want -1", keyboard.KeymapFD())
	}
	if keyboard.KeymapSize() != 0 {
		t.Errorf("WlKeyboard.KeymapSize() = %d, want 0", keyboard.KeymapSize())
	}

	rate, delay := keyboard.RepeatInfo()
	if rate != 25 {
		t.Errorf("default repeat rate = %d, want 25", rate)
	}
	if delay != 400 {
		t.Errorf("default repeat delay = %d, want 400", delay)
	}
}

// TestFixedCoordinateConversion verifies Fixed type conversion in pointer events.
func TestFixedCoordinateConversion(t *testing.T) {
	tests := []struct {
		name     string
		fixed    Fixed
		expected float64
		epsilon  float64
	}{
		{"zero", FixedFromFloat(0.0), 0.0, 0.001},
		{"positive integer", FixedFromFloat(100.0), 100.0, 0.001},
		{"negative integer", FixedFromFloat(-100.0), -100.0, 0.001},
		{"positive fraction", FixedFromFloat(50.5), 50.5, 0.01},
		{"negative fraction", FixedFromFloat(-50.5), -50.5, 0.01},
		{"small fraction", FixedFromFloat(0.25), 0.25, 0.01},
		{"large value", FixedFromFloat(1920.0), 1920.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fixed.Float()
			diff := got - tt.expected
			if diff < -tt.epsilon || diff > tt.epsilon {
				t.Errorf("Fixed(%v).Float() = %f, want %f (diff: %f)",
					tt.fixed, got, tt.expected, diff)
			}
		})
	}
}

// TestPointerEnterEventParsing verifies parsing of wl_pointer.enter event.
func TestPointerEnterEventParsing(t *testing.T) {
	serial := uint32(12345)
	surface := ObjectID(50)
	surfaceX := float64(150.5)
	surfaceY := float64(200.25)

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutObject(surface)
	builder.PutFixed(FixedFromFloat(surfaceX))
	builder.PutFixed(FixedFromFloat(surfaceY))

	msg := builder.BuildMessage(ObjectID(100), pointerEventEnter)

	// Parse the event
	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotSurface, _ := dec.Object()
	gotXFixed, _ := dec.Fixed()
	gotYFixed, _ := dec.Fixed()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotSurface != surface {
		t.Errorf("surface = %d, want %d", gotSurface, surface)
	}

	gotX := gotXFixed.Float()
	gotY := gotYFixed.Float()

	epsilon := 0.01
	if diff := gotX - surfaceX; diff < -epsilon || diff > epsilon {
		t.Errorf("surface_x = %f, want %f", gotX, surfaceX)
	}
	if diff := gotY - surfaceY; diff < -epsilon || diff > epsilon {
		t.Errorf("surface_y = %f, want %f", gotY, surfaceY)
	}
}

// TestPointerMotionEventParsing verifies parsing of wl_pointer.motion event.
func TestPointerMotionEventParsing(t *testing.T) {
	time := uint32(54321)
	surfaceX := float64(300.75)
	surfaceY := float64(400.25)

	builder := NewMessageBuilder()
	builder.PutUint32(time)
	builder.PutFixed(FixedFromFloat(surfaceX))
	builder.PutFixed(FixedFromFloat(surfaceY))

	msg := builder.BuildMessage(ObjectID(101), pointerEventMotion)

	dec := NewDecoder(msg.Args)

	gotTime, _ := dec.Uint32()
	gotXFixed, _ := dec.Fixed()
	gotYFixed, _ := dec.Fixed()

	if gotTime != time {
		t.Errorf("time = %d, want %d", gotTime, time)
	}

	gotX := gotXFixed.Float()
	gotY := gotYFixed.Float()

	epsilon := 0.01
	if diff := gotX - surfaceX; diff < -epsilon || diff > epsilon {
		t.Errorf("surface_x = %f, want %f", gotX, surfaceX)
	}
	if diff := gotY - surfaceY; diff < -epsilon || diff > epsilon {
		t.Errorf("surface_y = %f, want %f", gotY, surfaceY)
	}
}

// TestPointerButtonEventParsing verifies parsing of wl_pointer.button event.
func TestPointerButtonEventParsing(t *testing.T) {
	serial := uint32(11111)
	time := uint32(22222)
	button := ButtonLeft
	state := PointerButtonStatePressed

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutUint32(time)
	builder.PutUint32(button)
	builder.PutUint32(state)

	msg := builder.BuildMessage(ObjectID(102), pointerEventButton)

	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotTime, _ := dec.Uint32()
	gotButton, _ := dec.Uint32()
	gotState, _ := dec.Uint32()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotTime != time {
		t.Errorf("time = %d, want %d", gotTime, time)
	}
	if gotButton != button {
		t.Errorf("button = 0x%x, want 0x%x", gotButton, button)
	}
	if gotState != state {
		t.Errorf("state = %d, want %d", gotState, state)
	}
}

// TestPointerAxisEventParsing verifies parsing of wl_pointer.axis event.
func TestPointerAxisEventParsing(t *testing.T) {
	time := uint32(33333)
	axis := PointerAxisVerticalScroll
	value := float64(15.0) // Scroll amount

	builder := NewMessageBuilder()
	builder.PutUint32(time)
	builder.PutUint32(axis)
	builder.PutFixed(FixedFromFloat(value))

	msg := builder.BuildMessage(ObjectID(103), pointerEventAxis)

	dec := NewDecoder(msg.Args)

	gotTime, _ := dec.Uint32()
	gotAxis, _ := dec.Uint32()
	gotValueFixed, _ := dec.Fixed()

	if gotTime != time {
		t.Errorf("time = %d, want %d", gotTime, time)
	}
	if gotAxis != axis {
		t.Errorf("axis = %d, want %d", gotAxis, axis)
	}

	gotValue := gotValueFixed.Float()
	epsilon := 0.01
	if diff := gotValue - value; diff < -epsilon || diff > epsilon {
		t.Errorf("value = %f, want %f", gotValue, value)
	}
}

// TestKeyboardKeyEventParsing verifies parsing of wl_keyboard.key event.
func TestKeyboardKeyEventParsing(t *testing.T) {
	serial := uint32(44444)
	time := uint32(55555)
	key := uint32(30) // KEY_A
	state := KeyStatePressed

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutUint32(time)
	builder.PutUint32(key)
	builder.PutUint32(state)

	msg := builder.BuildMessage(ObjectID(200), keyboardEventKey)

	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotTime, _ := dec.Uint32()
	gotKey, _ := dec.Uint32()
	gotState, _ := dec.Uint32()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotTime != time {
		t.Errorf("time = %d, want %d", gotTime, time)
	}
	if gotKey != key {
		t.Errorf("key = %d, want %d", gotKey, key)
	}
	if gotState != state {
		t.Errorf("state = %d, want %d", gotState, state)
	}
}

// TestKeyboardModifiersEventParsing verifies parsing of wl_keyboard.modifiers event.
func TestKeyboardModifiersEventParsing(t *testing.T) {
	serial := uint32(66666)
	modsDepressed := uint32(1) // Shift
	modsLatched := uint32(0)
	modsLocked := uint32(2) // Caps Lock
	group := uint32(0)

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutUint32(modsDepressed)
	builder.PutUint32(modsLatched)
	builder.PutUint32(modsLocked)
	builder.PutUint32(group)

	msg := builder.BuildMessage(ObjectID(201), keyboardEventModifiers)

	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotModsDepressed, _ := dec.Uint32()
	gotModsLatched, _ := dec.Uint32()
	gotModsLocked, _ := dec.Uint32()
	gotGroup, _ := dec.Uint32()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotModsDepressed != modsDepressed {
		t.Errorf("mods_depressed = %d, want %d", gotModsDepressed, modsDepressed)
	}
	if gotModsLatched != modsLatched {
		t.Errorf("mods_latched = %d, want %d", gotModsLatched, modsLatched)
	}
	if gotModsLocked != modsLocked {
		t.Errorf("mods_locked = %d, want %d", gotModsLocked, modsLocked)
	}
	if gotGroup != group {
		t.Errorf("group = %d, want %d", gotGroup, group)
	}
}

// TestKeyboardEnterEventParsing verifies parsing of wl_keyboard.enter event.
func TestKeyboardEnterEventParsing(t *testing.T) {
	serial := uint32(77777)
	surface := ObjectID(60)
	keys := []uint32{30, 31, 32} // Some pressed keys

	// Build keys array
	keysData := make([]byte, len(keys)*4)
	for i, key := range keys {
		binary.LittleEndian.PutUint32(keysData[i*4:], key)
	}

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutObject(surface)
	builder.PutArray(keysData)

	msg := builder.BuildMessage(ObjectID(202), keyboardEventEnter)

	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotSurface, _ := dec.Object()
	gotKeysData, _ := dec.Array()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotSurface != surface {
		t.Errorf("surface = %d, want %d", gotSurface, surface)
	}

	// Parse keys array
	gotKeys := make([]uint32, len(gotKeysData)/4)
	for i := range gotKeys {
		gotKeys[i] = binary.LittleEndian.Uint32(gotKeysData[i*4:])
	}

	if len(gotKeys) != len(keys) {
		t.Fatalf("keys count = %d, want %d", len(gotKeys), len(keys))
	}
	for i, key := range keys {
		if gotKeys[i] != key {
			t.Errorf("keys[%d] = %d, want %d", i, gotKeys[i], key)
		}
	}
}

// TestKeyboardRepeatInfoEventParsing verifies parsing of wl_keyboard.repeat_info event.
func TestKeyboardRepeatInfoEventParsing(t *testing.T) {
	rate := int32(30)   // 30 chars/sec
	delay := int32(500) // 500ms delay

	builder := NewMessageBuilder()
	builder.PutInt32(rate)
	builder.PutInt32(delay)

	msg := builder.BuildMessage(ObjectID(203), keyboardEventRepeatInfo)

	dec := NewDecoder(msg.Args)

	gotRate, _ := dec.Int32()
	gotDelay, _ := dec.Int32()

	if gotRate != rate {
		t.Errorf("rate = %d, want %d", gotRate, rate)
	}
	if gotDelay != delay {
		t.Errorf("delay = %d, want %d", gotDelay, delay)
	}
}

// TestSetCursorMessage verifies the message format for wl_pointer.set_cursor.
func TestSetCursorMessage(t *testing.T) {
	serial := uint32(88888)
	surface := ObjectID(70)
	hotspotX := int32(5)
	hotspotY := int32(5)

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutObject(surface)
	builder.PutInt32(hotspotX)
	builder.PutInt32(hotspotY)
	msg := builder.BuildMessage(ObjectID(300), pointerSetCursor)

	if msg.Opcode != pointerSetCursor {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, pointerSetCursor)
	}

	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotSurface, _ := dec.Object()
	gotHotspotX, _ := dec.Int32()
	gotHotspotY, _ := dec.Int32()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotSurface != surface {
		t.Errorf("surface = %d, want %d", gotSurface, surface)
	}
	if gotHotspotX != hotspotX {
		t.Errorf("hotspot_x = %d, want %d", gotHotspotX, hotspotX)
	}
	if gotHotspotY != hotspotY {
		t.Errorf("hotspot_y = %d, want %d", gotHotspotY, hotspotY)
	}
}

// TestSetCursorNullSurfaceMessage verifies set_cursor with null surface (hide cursor).
func TestSetCursorNullSurfaceMessage(t *testing.T) {
	serial := uint32(99999)

	builder := NewMessageBuilder()
	builder.PutUint32(serial)
	builder.PutObject(ObjectID(0)) // null surface
	builder.PutInt32(0)
	builder.PutInt32(0)
	msg := builder.BuildMessage(ObjectID(301), pointerSetCursor)

	dec := NewDecoder(msg.Args)

	gotSerial, _ := dec.Uint32()
	gotSurface, _ := dec.Object()

	if gotSerial != serial {
		t.Errorf("serial = %d, want %d", gotSerial, serial)
	}
	if gotSurface != 0 {
		t.Errorf("surface = %d, want 0 (null)", gotSurface)
	}
}

// TestSeatGetPointerMessage verifies the message format for wl_seat.get_pointer.
func TestSeatGetPointerMessage(t *testing.T) {
	pointerID := ObjectID(80)

	builder := NewMessageBuilder()
	builder.PutNewID(pointerID)
	msg := builder.BuildMessage(ObjectID(400), seatGetPointer)

	if msg.Opcode != seatGetPointer {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, seatGetPointer)
	}

	dec := NewDecoder(msg.Args)
	gotID, _ := dec.NewID()

	if gotID != pointerID {
		t.Errorf("pointer ID = %d, want %d", gotID, pointerID)
	}
}

// TestSeatGetKeyboardMessage verifies the message format for wl_seat.get_keyboard.
func TestSeatGetKeyboardMessage(t *testing.T) {
	keyboardID := ObjectID(90)

	builder := NewMessageBuilder()
	builder.PutNewID(keyboardID)
	msg := builder.BuildMessage(ObjectID(401), seatGetKeyboard)

	if msg.Opcode != seatGetKeyboard {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, seatGetKeyboard)
	}

	dec := NewDecoder(msg.Args)
	gotID, _ := dec.NewID()

	if gotID != keyboardID {
		t.Errorf("keyboard ID = %d, want %d", gotID, keyboardID)
	}
}

// TestPointerDispatch verifies the dispatch method for wl_pointer.
func TestPointerDispatch(t *testing.T) {
	pointer := NewWlPointer(nil, ObjectID(500))

	var enterCalled bool
	var enterEvent *PointerEnterEvent

	pointer.SetEnterHandler(func(event *PointerEnterEvent) {
		enterCalled = true
		enterEvent = event
	})

	// Build enter event
	builder := NewMessageBuilder()
	expectedSerial := uint32(111)
	expectedSurface := ObjectID(222)
	expectedX := float64(100.5)
	expectedY := float64(200.5)

	builder.PutUint32(expectedSerial)
	builder.PutObject(expectedSurface)
	builder.PutFixed(FixedFromFloat(expectedX))
	builder.PutFixed(FixedFromFloat(expectedY))
	msg := builder.BuildMessage(pointer.id, pointerEventEnter)

	// Dispatch
	err := pointer.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !enterCalled {
		t.Error("enter handler was not called")
	}
	if enterEvent == nil {
		t.Fatal("enter event is nil")
	}
	if enterEvent.Serial != expectedSerial {
		t.Errorf("event serial = %d, want %d", enterEvent.Serial, expectedSerial)
	}
	if enterEvent.Surface != expectedSurface {
		t.Errorf("event surface = %d, want %d", enterEvent.Surface, expectedSurface)
	}

	epsilon := 0.01
	if diff := enterEvent.SurfaceX - expectedX; diff < -epsilon || diff > epsilon {
		t.Errorf("event surface_x = %f, want %f", enterEvent.SurfaceX, expectedX)
	}
	if diff := enterEvent.SurfaceY - expectedY; diff < -epsilon || diff > epsilon {
		t.Errorf("event surface_y = %f, want %f", enterEvent.SurfaceY, expectedY)
	}

	// Verify state was updated
	if pointer.EnteredSurface() != expectedSurface {
		t.Errorf("entered surface = %d, want %d", pointer.EnteredSurface(), expectedSurface)
	}
	x, y := pointer.Position()
	if diff := x - expectedX; diff < -epsilon || diff > epsilon {
		t.Errorf("position x = %f, want %f", x, expectedX)
	}
	if diff := y - expectedY; diff < -epsilon || diff > epsilon {
		t.Errorf("position y = %f, want %f", y, expectedY)
	}
}

// TestKeyboardDispatch verifies the dispatch method for wl_keyboard.
func TestKeyboardDispatch(t *testing.T) {
	keyboard := NewWlKeyboard(nil, ObjectID(600))

	var keyCalled bool
	var keyEvent *KeyboardKeyEvent

	keyboard.SetKeyHandler(func(event *KeyboardKeyEvent) {
		keyCalled = true
		keyEvent = event
	})

	// Build key event
	builder := NewMessageBuilder()
	expectedSerial := uint32(333)
	expectedTime := uint32(444)
	expectedKey := uint32(30) // KEY_A
	expectedState := KeyStatePressed

	builder.PutUint32(expectedSerial)
	builder.PutUint32(expectedTime)
	builder.PutUint32(expectedKey)
	builder.PutUint32(expectedState)
	msg := builder.BuildMessage(keyboard.id, keyboardEventKey)

	// Dispatch
	err := keyboard.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !keyCalled {
		t.Error("key handler was not called")
	}
	if keyEvent == nil {
		t.Fatal("key event is nil")
	}
	if keyEvent.Serial != expectedSerial {
		t.Errorf("event serial = %d, want %d", keyEvent.Serial, expectedSerial)
	}
	if keyEvent.Time != expectedTime {
		t.Errorf("event time = %d, want %d", keyEvent.Time, expectedTime)
	}
	if keyEvent.Key != expectedKey {
		t.Errorf("event key = %d, want %d", keyEvent.Key, expectedKey)
	}
	if keyEvent.State != expectedState {
		t.Errorf("event state = %d, want %d", keyEvent.State, expectedState)
	}
}

// TestSeatDispatch verifies the dispatch method for wl_seat.
func TestSeatDispatch(t *testing.T) {
	seat := NewWlSeat(nil, ObjectID(700), 7)

	var capabilitiesCalled bool
	var gotCapabilities uint32

	seat.SetCapabilitiesHandler(func(capabilities uint32) {
		capabilitiesCalled = true
		gotCapabilities = capabilities
	})

	// Build capabilities event
	builder := NewMessageBuilder()
	expectedCapabilities := SeatCapabilityPointer | SeatCapabilityKeyboard

	builder.PutUint32(expectedCapabilities)
	msg := builder.BuildMessage(seat.id, seatEventCapabilities)

	// Dispatch
	err := seat.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !capabilitiesCalled {
		t.Error("capabilities handler was not called")
	}
	if gotCapabilities != expectedCapabilities {
		t.Errorf("capabilities = %d, want %d", gotCapabilities, expectedCapabilities)
	}

	// Verify state was updated
	if seat.Capabilities() != expectedCapabilities {
		t.Errorf("seat.Capabilities() = %d, want %d", seat.Capabilities(), expectedCapabilities)
	}
}

// TestPointerLeaveDispatch verifies handling of pointer leave event.
func TestPointerLeaveDispatch(t *testing.T) {
	pointer := NewWlPointer(nil, ObjectID(800))

	// Set initial state
	pointer.enteredSurface = ObjectID(999)
	pointer.surfaceX = 100
	pointer.surfaceY = 200

	var leaveCalled bool
	pointer.SetLeaveHandler(func(event *PointerLeaveEvent) {
		leaveCalled = true
	})

	// Build leave event
	builder := NewMessageBuilder()
	builder.PutUint32(12345)         // serial
	builder.PutObject(ObjectID(999)) // surface
	msg := builder.BuildMessage(pointer.id, pointerEventLeave)

	// Dispatch
	err := pointer.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !leaveCalled {
		t.Error("leave handler was not called")
	}

	// Verify entered surface was cleared
	if pointer.EnteredSurface() != 0 {
		t.Errorf("entered surface = %d, want 0", pointer.EnteredSurface())
	}
}

// TestPointerFrameDispatch verifies handling of pointer frame event.
func TestPointerFrameDispatch(t *testing.T) {
	pointer := NewWlPointer(nil, ObjectID(900))

	var frameCalled bool
	pointer.SetFrameHandler(func() {
		frameCalled = true
	})

	// Frame event has no arguments
	msg := &Message{
		ObjectID: pointer.id,
		Opcode:   pointerEventFrame,
		Args:     nil,
	}

	err := pointer.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !frameCalled {
		t.Error("frame handler was not called")
	}
}

// TestKeyboardLeaveDispatch verifies handling of keyboard leave event.
func TestKeyboardLeaveDispatch(t *testing.T) {
	keyboard := NewWlKeyboard(nil, ObjectID(950))

	// Set initial state
	keyboard.focusedSurface = ObjectID(888)

	var leaveCalled bool
	keyboard.SetLeaveHandler(func(event *KeyboardLeaveEvent) {
		leaveCalled = true
	})

	// Build leave event
	builder := NewMessageBuilder()
	builder.PutUint32(54321)         // serial
	builder.PutObject(ObjectID(888)) // surface
	msg := builder.BuildMessage(keyboard.id, keyboardEventLeave)

	// Dispatch
	err := keyboard.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !leaveCalled {
		t.Error("leave handler was not called")
	}

	// Verify focused surface was cleared
	if keyboard.FocusedSurface() != 0 {
		t.Errorf("focused surface = %d, want 0", keyboard.FocusedSurface())
	}
}

// TestReleaseMessages verifies release message formats.
func TestReleaseMessages(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		objectID ObjectID
	}{
		{"pointer.release", pointerRelease, ObjectID(1000)},
		{"keyboard.release", keyboardRelease, ObjectID(1001)},
		{"seat.release", seatRelease, ObjectID(1002)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewMessageBuilder()
			msg := builder.BuildMessage(tt.objectID, tt.opcode)

			if msg.Opcode != tt.opcode {
				t.Errorf("Opcode = %d, want %d", msg.Opcode, tt.opcode)
			}
			if len(msg.Args) != 0 {
				t.Errorf("Args length = %d, want 0", len(msg.Args))
			}
		})
	}
}
