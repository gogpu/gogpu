//go:build linux

package x11

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestFP1616ToFloat64(t *testing.T) {
	tests := []struct {
		name string
		raw  int32
		want float64
	}{
		{"zero", 0, 0.0},
		{"one", 1 << 16, 1.0},
		{"minus_one", -(1 << 16), -1.0},
		{"half", 1 << 15, 0.5},
		{"400.5", 0x01908000, 400.5},
		{"small_fraction", 1, 1.0 / 65536.0},
		{"negative_half", -(1 << 15), -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fp1616ToFloat64(tt.raw)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("fp1616ToFloat64(%d): got %f, want %f", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFP3232ToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		integral int32
		frac     uint32
		want     float64
	}{
		{"zero", 0, 0, 0.0},
		{"one", 1, 0, 1.0},
		{"minus_one", -1, 0, -1.0},
		{"half", 0, 1 << 31, 0.5},
		{"quarter", 0, 1 << 30, 0.25},
		{"100.75", 100, 3 << 30, 100.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fp3232ToFloat64(tt.integral, tt.frac)
			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("fp3232ToFloat64(%d, %d): got %f, want %f", tt.integral, tt.frac, got, tt.want)
			}
		})
	}
}

func TestPopcount(t *testing.T) {
	tests := []struct {
		name string
		mask []byte
		want int
	}{
		{"empty", nil, 0},
		{"zero", []byte{0x00}, 0},
		{"one_bit", []byte{0x01}, 1},
		{"all_bits", []byte{0xFF}, 8},
		{"mixed", []byte{0x0A}, 2},                  // bits 1,3
		{"multi_byte", []byte{0x01, 0x80}, 2},       // bit 0 of byte 0, bit 7 of byte 1
		{"touch_mask", []byte{0x00, 0x00, 0x1C}, 3}, // bits 18,19,20
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := popcount(tt.mask)
			if got != tt.want {
				t.Errorf("popcount(%v): got %d, want %d", tt.mask, got, tt.want)
			}
		})
	}
}

func TestXiSetMask(t *testing.T) {
	// Test setting touch event mask bits
	mask := make([]byte, 4)
	xiSetMask(mask, XITouchBegin)  // bit 18
	xiSetMask(mask, XITouchUpdate) // bit 19
	xiSetMask(mask, XITouchEnd)    // bit 20

	// bits 18,19,20 in byte[2] = 0x04 | 0x08 | 0x10 = 0x1C
	want := []byte{0x00, 0x00, 0x1C, 0x00}
	for i := range want {
		if mask[i] != want[i] {
			t.Errorf("xiSetMask: mask[%d] = %02x, want %02x", i, mask[i], want[i])
		}
	}
}

func TestParseGenericEvent(t *testing.T) {
	// Build a 32-byte GenericEvent buffer (little-endian)
	buf := make([]byte, 32)
	buf[0] = EventGenericEvent                             // type
	buf[1] = 131                                           // extension major opcode
	binary.LittleEndian.PutUint16(buf[2:4], 42)            // sequence
	binary.LittleEndian.PutUint32(buf[4:8], 0)             // length (no additional data)
	binary.LittleEndian.PutUint16(buf[8:10], XITouchBegin) // evtype

	c := &Connection{byteOrder: LSBFirst}
	event, err := c.parseGenericEvent(buf)
	if err != nil {
		t.Fatalf("parseGenericEvent: unexpected error: %v", err)
	}

	ge, ok := event.(*GenericEvent)
	if !ok {
		t.Fatalf("parseGenericEvent: got type %T, want *GenericEvent", event)
	}

	if ge.Extension != 131 {
		t.Errorf("Extension: got %d, want 131", ge.Extension)
	}
	if ge.Sequence != 42 {
		t.Errorf("Sequence: got %d, want 42", ge.Sequence)
	}
	if ge.EventType != XITouchBegin {
		t.Errorf("EventType: got %d, want %d", ge.EventType, XITouchBegin)
	}
}

func TestParseGenericEventInParseEvent(t *testing.T) {
	// Verify parseEvent routes GenericEvent correctly
	buf := make([]byte, 32)
	buf[0] = EventGenericEvent
	buf[1] = 99 // some extension opcode
	binary.LittleEndian.PutUint16(buf[2:4], 7)
	binary.LittleEndian.PutUint32(buf[4:8], 0)
	binary.LittleEndian.PutUint16(buf[8:10], 5)

	c := &Connection{byteOrder: LSBFirst}
	event, err := c.parseEvent(buf)
	if err != nil {
		t.Fatalf("parseEvent(GenericEvent): unexpected error: %v", err)
	}

	ge, ok := event.(*GenericEvent)
	if !ok {
		t.Fatalf("parseEvent: got type %T, want *GenericEvent", event)
	}

	if ge.Extension != 99 {
		t.Errorf("Extension: got %d, want 99", ge.Extension)
	}
	if ge.EventType != 5 {
		t.Errorf("EventType: got %d, want 5", ge.EventType)
	}
}

func TestParseXIDeviceEvent(t *testing.T) {
	// Build a minimal XIDeviceEvent for XI_TouchBegin
	// 32 (header) + 64 (fixed) + 0 (buttons) + 0 (valuators) = 96 bytes
	buf := make([]byte, 96)

	// GenericEvent header (32 bytes)
	buf[0] = EventGenericEvent                             // type
	buf[1] = 131                                           // extension
	binary.LittleEndian.PutUint16(buf[2:4], 10)            // sequence
	binary.LittleEndian.PutUint32(buf[4:8], 16)            // length: (96-32)/4 = 16
	binary.LittleEndian.PutUint16(buf[8:10], XITouchBegin) // evtype
	binary.LittleEndian.PutUint16(buf[10:12], 2)           // deviceID
	binary.LittleEndian.PutUint32(buf[12:16], 1000)        // time
	// bytes 16-31: padding/reserved (zeros)

	// Fixed payload at offset 32 (64 bytes)
	binary.LittleEndian.PutUint32(buf[32:36], 42)    // detail (touch ID)
	binary.LittleEndian.PutUint32(buf[36:40], 0x100) // root window
	binary.LittleEndian.PutUint32(buf[40:44], 0x200) // event window
	binary.LittleEndian.PutUint32(buf[44:48], 0)     // child window

	// FP1616 coordinates: 400.5 = 0x01908000, 300.25 = 0x012C4000
	binary.LittleEndian.PutUint32(buf[48:52], 0x01908000) // root_x = 400.5
	binary.LittleEndian.PutUint32(buf[52:56], 0x012C4000) // root_y = 300.25
	binary.LittleEndian.PutUint32(buf[56:60], 0x00C80000) // event_x = 200.0
	binary.LittleEndian.PutUint32(buf[60:64], 0x00960000) // event_y = 150.0

	binary.LittleEndian.PutUint16(buf[64:66], 0)                       // buttons_len
	binary.LittleEndian.PutUint16(buf[66:68], 0)                       // valuators_len
	binary.LittleEndian.PutUint16(buf[68:70], 5)                       // sourceid
	binary.LittleEndian.PutUint16(buf[70:72], 0)                       // pad0
	binary.LittleEndian.PutUint32(buf[72:76], XITouchEmulatingPointer) // flags

	// Modifier info (16 bytes at offset 76)
	binary.LittleEndian.PutUint32(buf[76:80], 0)   // base_mods
	binary.LittleEndian.PutUint32(buf[80:84], 0)   // latched_mods
	binary.LittleEndian.PutUint32(buf[84:88], 0)   // locked_mods
	binary.LittleEndian.PutUint32(buf[88:92], 0x4) // effective_mods (Control)

	// Group info (4 bytes at offset 92)
	// zeros

	ge := &GenericEvent{
		Extension: 131,
		Sequence:  10,
		EventType: XITouchBegin,
		Data:      buf,
	}

	c := &Connection{byteOrder: LSBFirst}
	dev, err := c.ParseXIDeviceEvent(ge)
	if err != nil {
		t.Fatalf("ParseXIDeviceEvent: unexpected error: %v", err)
	}

	if dev.EventType != XITouchBegin {
		t.Errorf("EventType: got %d, want %d", dev.EventType, XITouchBegin)
	}
	if dev.Detail != 42 {
		t.Errorf("Detail (touchID): got %d, want 42", dev.Detail)
	}
	if dev.DeviceID != 2 {
		t.Errorf("DeviceID: got %d, want 2", dev.DeviceID)
	}
	if dev.Time != 1000 {
		t.Errorf("Time: got %d, want 1000", dev.Time)
	}
	if dev.Root != 0x100 {
		t.Errorf("Root: got %x, want %x", dev.Root, 0x100)
	}
	if dev.Event != 0x200 {
		t.Errorf("Event: got %x, want %x", dev.Event, 0x200)
	}

	// Check FP1616 coordinate conversion
	if math.Abs(dev.RootX-400.5) > 0.001 {
		t.Errorf("RootX: got %f, want 400.5", dev.RootX)
	}
	if math.Abs(dev.RootY-300.25) > 0.001 {
		t.Errorf("RootY: got %f, want 300.25", dev.RootY)
	}
	if math.Abs(dev.EventX-200.0) > 0.001 {
		t.Errorf("EventX: got %f, want 200.0", dev.EventX)
	}
	if math.Abs(dev.EventY-150.0) > 0.001 {
		t.Errorf("EventY: got %f, want 150.0", dev.EventY)
	}

	if dev.SourceID != 5 {
		t.Errorf("SourceID: got %d, want 5", dev.SourceID)
	}
	if !dev.IsTouchEmulating() {
		t.Error("IsTouchEmulating: got false, want true")
	}
	if dev.Mods != 0x4 {
		t.Errorf("Mods: got %x, want %x", dev.Mods, 0x4)
	}
}

func TestParseXIDeviceEvent_WithValuators(t *testing.T) {
	// Build an event with 1 button mask unit and 1 valuator mask unit
	// with 2 set bits (= 2 axis values)
	buttonsLen := uint16(1)   // 4 bytes
	valuatorsLen := uint16(1) // 4 bytes
	numAxisValues := 2        // 2 set bits in valuator mask
	totalSize := 32 + 64 + int(buttonsLen)*4 + int(valuatorsLen)*4 + numAxisValues*8
	buf := make([]byte, totalSize)

	// GenericEvent header
	buf[0] = EventGenericEvent
	buf[1] = 131
	binary.LittleEndian.PutUint32(buf[4:8], uint32((totalSize-32)/4))
	binary.LittleEndian.PutUint16(buf[8:10], XITouchUpdate)
	binary.LittleEndian.PutUint16(buf[10:12], 3) // deviceID

	// Fixed payload at offset 32
	binary.LittleEndian.PutUint32(buf[32:36], 7)          // detail (touch ID)
	binary.LittleEndian.PutUint32(buf[56:60], 0x00640000) // event_x = 100.0
	binary.LittleEndian.PutUint32(buf[60:64], 0x00C80000) // event_y = 200.0
	binary.LittleEndian.PutUint16(buf[64:66], buttonsLen)
	binary.LittleEndian.PutUint16(buf[66:68], valuatorsLen)

	// Variable part starts at offset 96 (32 + 64)
	varStart := 96

	// Buttons mask (4 bytes)
	buf[varStart] = 0x02 // button 1 pressed

	// Valuators mask (4 bytes) — bits 0 and 2 set (2 axis values)
	buf[varStart+4] = 0x05 // bits 0 and 2

	// Axis values (FP3232, 8 bytes each)
	axisStart := varStart + 8
	// Axis 0: integral=50, frac=0 → 50.0
	binary.LittleEndian.PutUint32(buf[axisStart:axisStart+4], 50)
	binary.LittleEndian.PutUint32(buf[axisStart+4:axisStart+8], 0)
	// Axis 2: integral=75, frac=1<<31 → 75.5
	binary.LittleEndian.PutUint32(buf[axisStart+8:axisStart+12], 75)
	binary.LittleEndian.PutUint32(buf[axisStart+12:axisStart+16], 1<<31)

	ge := &GenericEvent{
		Extension: 131,
		EventType: XITouchUpdate,
		Data:      buf,
	}

	c := &Connection{byteOrder: LSBFirst}
	dev, err := c.ParseXIDeviceEvent(ge)
	if err != nil {
		t.Fatalf("ParseXIDeviceEvent: unexpected error: %v", err)
	}

	if dev.Detail != 7 {
		t.Errorf("Detail: got %d, want 7", dev.Detail)
	}
	if len(dev.ButtonsMask) != 4 {
		t.Fatalf("ButtonsMask length: got %d, want 4", len(dev.ButtonsMask))
	}
	if dev.ButtonsMask[0] != 0x02 {
		t.Errorf("ButtonsMask[0]: got %02x, want %02x", dev.ButtonsMask[0], 0x02)
	}

	if len(dev.AxisValues) != 2 {
		t.Fatalf("AxisValues length: got %d, want 2", len(dev.AxisValues))
	}
	if math.Abs(dev.AxisValues[0]-50.0) > 0.001 {
		t.Errorf("AxisValues[0]: got %f, want 50.0", dev.AxisValues[0])
	}
	if math.Abs(dev.AxisValues[1]-75.5) > 0.001 {
		t.Errorf("AxisValues[1]: got %f, want 75.5", dev.AxisValues[1])
	}
}

func TestParseXIDeviceEvent_TooShort(t *testing.T) {
	ge := &GenericEvent{
		Data: make([]byte, 50), // less than 96 minimum
	}

	c := &Connection{byteOrder: LSBFirst}
	_, err := c.ParseXIDeviceEvent(ge)
	if err == nil {
		t.Error("ParseXIDeviceEvent: expected error for short buffer, got nil")
	}
}

func TestXIDeviceEvent_EventMarker(t *testing.T) {
	// Verify XIDeviceEvent and GenericEvent implement Event interface
	events := []Event{
		&XIDeviceEvent{},
		&GenericEvent{},
	}
	for _, e := range events {
		e.eventMarker()
	}
}

func TestXIDeviceEvent_IsTouchEmulating(t *testing.T) {
	tests := []struct {
		name  string
		flags uint32
		want  bool
	}{
		{"emulating", XITouchEmulatingPointer, true},
		{"not_emulating", 0, false},
		{"other_flags", 0x00000001, false},
		{"mixed_flags", XITouchEmulatingPointer | 0x01, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &XIDeviceEvent{Flags: tt.flags}
			if got := e.IsTouchEmulating(); got != tt.want {
				t.Errorf("IsTouchEmulating: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtensionInfo(t *testing.T) {
	ext := &ExtensionInfo{
		Present:     true,
		MajorOpcode: 131,
		FirstEvent:  0,
		FirstError:  0,
	}

	if !ext.Present {
		t.Error("Present: got false, want true")
	}
	if ext.MajorOpcode != 131 {
		t.Errorf("MajorOpcode: got %d, want 131", ext.MajorOpcode)
	}
}
