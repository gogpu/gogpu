//go:build linux

package x11

import (
	"testing"
)

func TestClientMessageEvent_Data32(t *testing.T) {
	event := &ClientMessageEvent{
		Format: 32,
		Data: [20]byte{
			0x01, 0x02, 0x03, 0x04, // data[0] = 0x04030201
			0x05, 0x06, 0x07, 0x08, // data[1] = 0x08070605
			0x09, 0x0A, 0x0B, 0x0C, // data[2] = 0x0C0B0A09
			0x0D, 0x0E, 0x0F, 0x10, // data[3] = 0x100F0E0D
			0x11, 0x12, 0x13, 0x14, // data[4] = 0x14131211
		},
	}

	data := event.Data32()

	expected := [5]uint32{
		0x04030201,
		0x08070605,
		0x0C0B0A09,
		0x100F0E0D,
		0x14131211,
	}

	for i := 0; i < 5; i++ {
		if data[i] != expected[i] {
			t.Errorf("Data32[%d]: got %08x, want %08x", i, data[i], expected[i])
		}
	}
}

func TestClientMessageEvent_IsDeleteWindow(t *testing.T) {
	atoms := &StandardAtoms{
		WMProtocols:    Atom(100),
		WMDeleteWindow: Atom(101),
	}

	tests := []struct {
		name  string
		event *ClientMessageEvent
		want  bool
	}{
		{
			name: "delete window",
			event: &ClientMessageEvent{
				Format: 32,
				Type:   atoms.WMProtocols,
				Data: [20]byte{
					101, 0, 0, 0, // WMDeleteWindow = 101
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
				},
			},
			want: true,
		},
		{
			name: "wrong type",
			event: &ClientMessageEvent{
				Format: 32,
				Type:   Atom(50), // Not WM_PROTOCOLS
				Data: [20]byte{
					101, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
				},
			},
			want: false,
		},
		{
			name: "wrong protocol",
			event: &ClientMessageEvent{
				Format: 32,
				Type:   atoms.WMProtocols,
				Data: [20]byte{
					50, 0, 0, 0, // Not WMDeleteWindow
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
					0, 0, 0, 0,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.IsDeleteWindow(atoms)
			if got != tt.want {
				t.Errorf("IsDeleteWindow: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventMarkers(t *testing.T) {
	// Ensure all event types implement Event interface
	events := []Event{
		&KeyPressEvent{},
		&KeyReleaseEvent{},
		&ButtonPressEvent{},
		&ButtonReleaseEvent{},
		&MotionNotifyEvent{},
		&EnterNotifyEvent{},
		&LeaveNotifyEvent{},
		&FocusInEvent{},
		&FocusOutEvent{},
		&ExposeEvent{},
		&ConfigureNotifyEvent{},
		&MapNotifyEvent{},
		&UnmapNotifyEvent{},
		&DestroyNotifyEvent{},
		&PropertyNotifyEvent{},
		&ClientMessageEvent{},
		&SelectionClearEvent{},
		&MappingNotifyEvent{},
		&UnknownEvent{},
	}

	for _, e := range events {
		// Just verify they all implement Event interface
		e.eventMarker()
	}
}
