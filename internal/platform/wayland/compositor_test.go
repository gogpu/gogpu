//go:build linux

package wayland

import (
	"bytes"
	"testing"
)

// TestCompositorOpcodes verifies compositor opcode constants match Wayland protocol spec.
func TestCompositorOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"create_surface", compositorCreateSurface, 0},
		{"create_region", compositorCreateRegion, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestSurfaceOpcodes verifies surface opcode constants match Wayland protocol spec.
func TestSurfaceOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"destroy", surfaceDestroy, 0},
		{"attach", surfaceAttach, 1},
		{"damage", surfaceDamage, 2},
		{"frame", surfaceFrame, 3},
		{"set_opaque_region", surfaceSetOpaqueRegion, 4},
		{"set_input_region", surfaceSetInputRegion, 5},
		{"commit", surfaceCommit, 6},
		{"set_buffer_transform", surfaceSetBufferTransform, 7},
		{"set_buffer_scale", surfaceSetBufferScale, 8},
		{"damage_buffer", surfaceDamageBuffer, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestSurfaceEventOpcodes verifies surface event opcode constants.
func TestSurfaceEventOpcodes(t *testing.T) {
	tests := []struct {
		name     string
		opcode   Opcode
		expected Opcode
	}{
		{"enter", surfaceEventEnter, 0},
		{"leave", surfaceEventLeave, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opcode != tt.expected {
				t.Errorf("event opcode %s = %d, want %d", tt.name, tt.opcode, tt.expected)
			}
		})
	}
}

// TestWlCompositorCreation verifies WlCompositor struct initialization.
func TestWlCompositorCreation(t *testing.T) {
	// We can't test with a real display, but we can verify the constructor
	// would set the ID correctly by checking the struct directly.
	comp := &WlCompositor{
		display: nil, // Would be a real display in production
		id:      ObjectID(5),
	}

	if comp.ID() != ObjectID(5) {
		t.Errorf("WlCompositor.ID() = %d, want 5", comp.ID())
	}
}

// TestWlSurfaceCreation verifies WlSurface struct initialization.
func TestWlSurfaceCreation(t *testing.T) {
	surface := &WlSurface{
		display: nil,
		id:      ObjectID(10),
	}

	if surface.ID() != ObjectID(10) {
		t.Errorf("WlSurface.ID() = %d, want 10", surface.ID())
	}

	// Test Ptr() returns the object ID as uintptr
	if surface.Ptr() != uintptr(10) {
		t.Errorf("WlSurface.Ptr() = %d, want 10", surface.Ptr())
	}
}

// TestWlCallbackCreation verifies WlCallback struct initialization.
func TestWlCallbackCreation(t *testing.T) {
	callback := NewWlCallback(nil, ObjectID(15))

	if callback.ID() != ObjectID(15) {
		t.Errorf("WlCallback.ID() = %d, want 15", callback.ID())
	}

	// Verify the done channel is created
	if callback.done == nil {
		t.Error("WlCallback.done channel should not be nil")
	}

	// Verify the channel is not closed and has capacity
	select {
	case callback.done <- 123:
		// Successfully sent, this is expected for a buffered channel
	default:
		t.Error("WlCallback.done channel should accept a value")
	}
}

// TestSurfaceAttachMessage verifies the message format for wl_surface.attach.
func TestSurfaceAttachMessage(t *testing.T) {
	builder := NewMessageBuilder()
	buffer := ObjectID(100)
	x := int32(-5)
	y := int32(10)

	builder.PutObject(buffer)
	builder.PutInt32(x)
	builder.PutInt32(y)
	msg := builder.BuildMessage(ObjectID(2), surfaceAttach)

	// Verify message structure
	if msg.ObjectID != ObjectID(2) {
		t.Errorf("ObjectID = %d, want 2", msg.ObjectID)
	}
	if msg.Opcode != surfaceAttach {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceAttach)
	}

	// Decode and verify arguments
	dec := NewDecoder(msg.Args)

	gotBuffer, err := dec.Object()
	if err != nil {
		t.Fatalf("failed to decode buffer: %v", err)
	}
	if gotBuffer != buffer {
		t.Errorf("buffer = %d, want %d", gotBuffer, buffer)
	}

	gotX, err := dec.Int32()
	if err != nil {
		t.Fatalf("failed to decode x: %v", err)
	}
	if gotX != x {
		t.Errorf("x = %d, want %d", gotX, x)
	}

	gotY, err := dec.Int32()
	if err != nil {
		t.Fatalf("failed to decode y: %v", err)
	}
	if gotY != y {
		t.Errorf("y = %d, want %d", gotY, y)
	}
}

// TestSurfaceDamageMessage verifies the message format for wl_surface.damage.
func TestSurfaceDamageMessage(t *testing.T) {
	builder := NewMessageBuilder()
	x, y, width, height := int32(0), int32(0), int32(800), int32(600)

	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(3), surfaceDamage)

	if msg.Opcode != surfaceDamage {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceDamage)
	}

	// Verify arguments length: 4 int32s = 16 bytes
	if len(msg.Args) != 16 {
		t.Errorf("Args length = %d, want 16", len(msg.Args))
	}

	// Decode and verify
	dec := NewDecoder(msg.Args)

	gotX, _ := dec.Int32()
	gotY, _ := dec.Int32()
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotX != x || gotY != y || gotWidth != width || gotHeight != height {
		t.Errorf("damage rect = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
			gotX, gotY, gotWidth, gotHeight, x, y, width, height)
	}
}

// TestSurfaceCommitMessage verifies the message format for wl_surface.commit.
func TestSurfaceCommitMessage(t *testing.T) {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(ObjectID(4), surfaceCommit)

	if msg.Opcode != surfaceCommit {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceCommit)
	}

	// Commit has no arguments
	if len(msg.Args) != 0 {
		t.Errorf("Args length = %d, want 0", len(msg.Args))
	}
}

// TestSurfaceSetBufferScaleMessage verifies the message format for wl_surface.set_buffer_scale.
func TestSurfaceSetBufferScaleMessage(t *testing.T) {
	builder := NewMessageBuilder()
	scale := int32(2) // HiDPI scale factor

	builder.PutInt32(scale)
	msg := builder.BuildMessage(ObjectID(5), surfaceSetBufferScale)

	if msg.Opcode != surfaceSetBufferScale {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceSetBufferScale)
	}

	// Verify argument
	dec := NewDecoder(msg.Args)
	gotScale, err := dec.Int32()
	if err != nil {
		t.Fatalf("failed to decode scale: %v", err)
	}
	if gotScale != scale {
		t.Errorf("scale = %d, want %d", gotScale, scale)
	}
}

// TestCompositorCreateSurfaceMessage verifies the message format for wl_compositor.create_surface.
func TestCompositorCreateSurfaceMessage(t *testing.T) {
	builder := NewMessageBuilder()
	surfaceID := ObjectID(10)

	builder.PutNewID(surfaceID)
	msg := builder.BuildMessage(ObjectID(1), compositorCreateSurface)

	if msg.Opcode != compositorCreateSurface {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, compositorCreateSurface)
	}

	// Verify argument: new_id is encoded as uint32
	dec := NewDecoder(msg.Args)
	gotID, err := dec.NewID()
	if err != nil {
		t.Fatalf("failed to decode new_id: %v", err)
	}
	if gotID != surfaceID {
		t.Errorf("surface ID = %d, want %d", gotID, surfaceID)
	}
}

// TestSurfaceFrameMessage verifies the message format for wl_surface.frame.
func TestSurfaceFrameMessage(t *testing.T) {
	builder := NewMessageBuilder()
	callbackID := ObjectID(20)

	builder.PutNewID(callbackID)
	msg := builder.BuildMessage(ObjectID(6), surfaceFrame)

	if msg.Opcode != surfaceFrame {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceFrame)
	}

	dec := NewDecoder(msg.Args)
	gotID, err := dec.NewID()
	if err != nil {
		t.Fatalf("failed to decode new_id: %v", err)
	}
	if gotID != callbackID {
		t.Errorf("callback ID = %d, want %d", gotID, callbackID)
	}
}

// TestSurfaceEnterEventParsing verifies parsing of wl_surface.enter event.
func TestSurfaceEnterEventParsing(t *testing.T) {
	// Build a fake enter event message
	builder := NewMessageBuilder()
	outputID := ObjectID(50)
	builder.PutObject(outputID)

	msg := builder.BuildMessage(ObjectID(7), surfaceEventEnter)

	// Parse the event
	decoder := NewDecoder(msg.Args)
	gotOutputID, err := decoder.Object()
	if err != nil {
		t.Fatalf("failed to decode output ID: %v", err)
	}
	if gotOutputID != outputID {
		t.Errorf("output ID = %d, want %d", gotOutputID, outputID)
	}
}

// TestSurfaceLeaveEventParsing verifies parsing of wl_surface.leave event.
func TestSurfaceLeaveEventParsing(t *testing.T) {
	builder := NewMessageBuilder()
	outputID := ObjectID(51)
	builder.PutObject(outputID)

	msg := builder.BuildMessage(ObjectID(8), surfaceEventLeave)

	decoder := NewDecoder(msg.Args)
	gotOutputID, err := decoder.Object()
	if err != nil {
		t.Fatalf("failed to decode output ID: %v", err)
	}
	if gotOutputID != outputID {
		t.Errorf("output ID = %d, want %d", gotOutputID, outputID)
	}
}

// TestSurfaceDispatch verifies the dispatch method for wl_surface.
func TestSurfaceDispatch(t *testing.T) {
	surface := &WlSurface{
		display: nil,
		id:      ObjectID(9),
	}

	// Track handler calls
	var enterCalled bool
	var enterOutputID ObjectID

	surface.SetEnterHandler(func(outputID ObjectID) {
		enterCalled = true
		enterOutputID = outputID
	})

	// Build enter event
	builder := NewMessageBuilder()
	expectedOutputID := ObjectID(100)
	builder.PutObject(expectedOutputID)
	msg := builder.BuildMessage(surface.id, surfaceEventEnter)

	// Dispatch
	err := surface.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !enterCalled {
		t.Error("enter handler was not called")
	}
	if enterOutputID != expectedOutputID {
		t.Errorf("enter output ID = %d, want %d", enterOutputID, expectedOutputID)
	}
}

// TestCallbackDispatch verifies the dispatch method for wl_callback.
func TestCallbackDispatch(t *testing.T) {
	callback := NewWlCallback(nil, ObjectID(20))

	// Save reference to done channel BEFORE dispatch
	// (dispatch closes and nils the channel)
	doneChannel := callback.Done()

	// Build done event
	builder := NewMessageBuilder()
	callbackData := uint32(12345)
	builder.PutUint32(callbackData)

	// Note: We use opcode 0 for wl_callback.done (which is callbackEventDone)
	// Build the message properly to get Args
	msg := builder.BuildMessage(callback.id, 0) // callbackEventDone = 0

	// Dispatch
	err := callback.dispatch(msg)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// Check that done channel received the value
	// Note: We use the saved channel reference since dispatch closes and nils it
	select {
	case data := <-doneChannel:
		if data != callbackData {
			t.Errorf("callback data = %d, want %d", data, callbackData)
		}
	default:
		t.Error("done channel did not receive callback data")
	}
}

// TestSurfaceDestroyMessage verifies the message format for wl_surface.destroy.
func TestSurfaceDestroyMessage(t *testing.T) {
	builder := NewMessageBuilder()
	msg := builder.BuildMessage(ObjectID(11), surfaceDestroy)

	if msg.Opcode != surfaceDestroy {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceDestroy)
	}

	// Destroy has no arguments
	if len(msg.Args) != 0 {
		t.Errorf("Args length = %d, want 0", len(msg.Args))
	}
}

// TestMessageEncodingRoundTrip verifies that messages encode and decode correctly.
func TestMessageEncodingRoundTrip(t *testing.T) {
	// Build a complex message: attach with buffer, x, y
	builder := NewMessageBuilder()
	buffer := ObjectID(42)
	x := int32(-100)
	y := int32(200)

	builder.PutObject(buffer)
	builder.PutInt32(x)
	builder.PutInt32(y)

	msg := builder.BuildMessage(ObjectID(99), surfaceAttach)

	// Encode to wire format
	encoded, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("EncodeMessage failed: %v", err)
	}

	// Decode from wire format
	dec := NewDecoder(encoded)
	decoded, err := dec.DecodeMessage()
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	// Verify message fields
	if decoded.ObjectID != msg.ObjectID {
		t.Errorf("ObjectID = %d, want %d", decoded.ObjectID, msg.ObjectID)
	}
	if decoded.Opcode != msg.Opcode {
		t.Errorf("Opcode = %d, want %d", decoded.Opcode, msg.Opcode)
	}
	if !bytes.Equal(decoded.Args, msg.Args) {
		t.Errorf("Args = %x, want %x", decoded.Args, msg.Args)
	}
}

// TestSetOpaqueRegionMessage verifies the message format for wl_surface.set_opaque_region.
func TestSetOpaqueRegionMessage(t *testing.T) {
	tests := []struct {
		name     string
		regionID ObjectID
	}{
		{"with region", ObjectID(30)},
		{"null region", ObjectID(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewMessageBuilder()
			builder.PutObject(tt.regionID)
			msg := builder.BuildMessage(ObjectID(12), surfaceSetOpaqueRegion)

			if msg.Opcode != surfaceSetOpaqueRegion {
				t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceSetOpaqueRegion)
			}

			dec := NewDecoder(msg.Args)
			gotID, err := dec.Object()
			if err != nil {
				t.Fatalf("failed to decode region ID: %v", err)
			}
			if gotID != tt.regionID {
				t.Errorf("region ID = %d, want %d", gotID, tt.regionID)
			}
		})
	}
}

// TestSetInputRegionMessage verifies the message format for wl_surface.set_input_region.
func TestSetInputRegionMessage(t *testing.T) {
	builder := NewMessageBuilder()
	regionID := ObjectID(31)

	builder.PutObject(regionID)
	msg := builder.BuildMessage(ObjectID(13), surfaceSetInputRegion)

	if msg.Opcode != surfaceSetInputRegion {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceSetInputRegion)
	}

	dec := NewDecoder(msg.Args)
	gotID, err := dec.Object()
	if err != nil {
		t.Fatalf("failed to decode region ID: %v", err)
	}
	if gotID != regionID {
		t.Errorf("region ID = %d, want %d", gotID, regionID)
	}
}

// TestDamageBufferMessage verifies the message format for wl_surface.damage_buffer.
func TestDamageBufferMessage(t *testing.T) {
	builder := NewMessageBuilder()
	x, y, width, height := int32(10), int32(20), int32(100), int32(50)

	builder.PutInt32(x)
	builder.PutInt32(y)
	builder.PutInt32(width)
	builder.PutInt32(height)
	msg := builder.BuildMessage(ObjectID(14), surfaceDamageBuffer)

	if msg.Opcode != surfaceDamageBuffer {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceDamageBuffer)
	}

	dec := NewDecoder(msg.Args)
	gotX, _ := dec.Int32()
	gotY, _ := dec.Int32()
	gotWidth, _ := dec.Int32()
	gotHeight, _ := dec.Int32()

	if gotX != x || gotY != y || gotWidth != width || gotHeight != height {
		t.Errorf("damage_buffer rect = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
			gotX, gotY, gotWidth, gotHeight, x, y, width, height)
	}
}

// TestSetBufferTransformMessage verifies the message format for wl_surface.set_buffer_transform.
func TestSetBufferTransformMessage(t *testing.T) {
	builder := NewMessageBuilder()
	transform := int32(1) // WL_OUTPUT_TRANSFORM_90

	builder.PutInt32(transform)
	msg := builder.BuildMessage(ObjectID(15), surfaceSetBufferTransform)

	if msg.Opcode != surfaceSetBufferTransform {
		t.Errorf("Opcode = %d, want %d", msg.Opcode, surfaceSetBufferTransform)
	}

	dec := NewDecoder(msg.Args)
	gotTransform, err := dec.Int32()
	if err != nil {
		t.Fatalf("failed to decode transform: %v", err)
	}
	if gotTransform != transform {
		t.Errorf("transform = %d, want %d", gotTransform, transform)
	}
}
