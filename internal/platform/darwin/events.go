//go:build darwin

package darwin

// EventHandler is called for each NSEvent during event polling.
// Return true to dispatch the event to the application, false to consume it.
type EventHandler func(event ID, eventType NSEventType) bool

// PollEventsWithHandler processes all pending events, calling the handler for each.
// The handler can inspect and optionally consume events.
// Returns true if any events were processed.
func (a *Application) PollEventsWithHandler(handler EventHandler) bool {
	if !a.initialized {
		return false
	}

	processed := false

	// Create local autorelease pool for event processing
	pool := classes.NSAutoreleasePool.Send(selectors.new)
	defer pool.Send(selectors.release)

	// Get distant past date for non-blocking poll
	distantPast := classes.NSDate.Send(selectors.distantPast)

	// Get default run loop mode string
	modeStr := NewNSString("kCFRunLoopDefaultMode")
	defer modeStr.Release()

	// Poll for events
	for {
		event := a.nextEvent(distantPast, modeStr.ID())
		if event.IsNil() {
			break
		}

		processed = true

		// Get event type
		eventType := NSEventType(event.GetUint64(selectors.eventType))

		// Let handler inspect the event
		shouldDispatch := true
		if handler != nil {
			shouldDispatch = handler(event, eventType)
		}

		// Dispatch to application if not consumed
		if shouldDispatch {
			a.nsApp.SendPtr(selectors.sendEvent, event.Ptr())
		}
	}

	return processed
}

// EventInfo contains extracted information from an NSEvent for pointer handling.
type EventInfo struct {
	Type          NSEventType
	LocationX     float64
	LocationY     float64
	ModifierFlags NSEventModifierFlags
	ButtonNumber  int64
	ScrollDeltaX  float64
	ScrollDeltaY  float64
	IsPrecise     bool
	KeyCode       uint16 // Virtual key code for keyboard events
	// Tablet/pen properties
	Subtype            NSUInteger
	Pressure           float64 // 0.0-1.0 (pen pressure)
	TiltX              float64 // -1.0 to 1.0
	TiltY              float64 // -1.0 to 1.0
	Rotation           float64 // 0-360 degrees
	PointingDeviceType NSUInteger
}

// GetEventInfo extracts pointer-relevant information from an NSEvent.
func GetEventInfo(event ID) EventInfo {
	if event.IsNil() {
		return EventInfo{}
	}

	initSelectors()

	info := EventInfo{
		Type:          NSEventType(event.GetUint64(selectors.eventType)),
		ModifierFlags: NSEventModifierFlags(event.GetUint64(selectors.modifierFlags)),
	}

	// Get location - all mouse events have this
	location := event.GetPoint(selectors.locationInWindow)
	info.LocationX = location.X
	info.LocationY = location.Y

	// Button number (for button events)
	switch info.Type {
	case NSEventTypeLeftMouseDown, NSEventTypeLeftMouseUp, NSEventTypeLeftMouseDragged:
		info.ButtonNumber = 0
	case NSEventTypeRightMouseDown, NSEventTypeRightMouseUp, NSEventTypeRightMouseDragged:
		info.ButtonNumber = 1
	case NSEventTypeOtherMouseDown, NSEventTypeOtherMouseUp, NSEventTypeOtherMouseDragged:
		info.ButtonNumber = event.GetInt64(selectors.buttonNumber)
	}

	// Tablet/pen properties for mouse events
	switch info.Type {
	case NSEventTypeLeftMouseDown, NSEventTypeLeftMouseUp, NSEventTypeLeftMouseDragged,
		NSEventTypeRightMouseDown, NSEventTypeRightMouseUp, NSEventTypeRightMouseDragged,
		NSEventTypeOtherMouseDown, NSEventTypeOtherMouseUp, NSEventTypeOtherMouseDragged,
		NSEventTypeMouseMoved,
		NSEventTypeTabletPoint:
		info.Subtype = event.GetUint64(selectors.subtype)
		if info.Subtype == NSEventSubtypeTabletPoint || info.Type == NSEventTypeTabletPoint {
			info.Pressure = event.GetDouble(selectors.pressure)
			tilt := event.GetPoint(selectors.tilt)
			info.TiltX = tilt.X
			info.TiltY = tilt.Y
			info.Rotation = event.GetDouble(selectors.rotation)
			info.PointingDeviceType = event.GetUint64(selectors.pointingDeviceType)
		}
	}

	// Scroll deltas
	if info.Type == NSEventTypeScrollWheel {
		info.IsPrecise = event.GetBool(selectors.hasPreciseScrollingDeltas)
		if info.IsPrecise {
			info.ScrollDeltaX = event.GetDouble(selectors.scrollingDeltaX)
			info.ScrollDeltaY = event.GetDouble(selectors.scrollingDeltaY)
		} else {
			// Fall back to legacy delta for non-trackpad devices
			info.ScrollDeltaX = event.GetDouble(selectors.deltaX)
			info.ScrollDeltaY = event.GetDouble(selectors.deltaY)
		}
	}

	// Key code for keyboard events
	switch info.Type {
	case NSEventTypeKeyDown, NSEventTypeKeyUp, NSEventTypeFlagsChanged:
		info.KeyCode = uint16(event.GetUint64(selectors.keyCode))
	}

	return info
}

// GetKeyCode returns the virtual key code from an NSEvent.
func GetKeyCode(event ID) uint16 {
	if event.IsNil() {
		return 0
	}
	initSelectors()
	return uint16(event.GetUint64(selectors.keyCode))
}

// GetCharacters returns the characters produced by a key event.
// Returns the NSString object ID from [NSEvent characters], or 0 if unavailable.
// The caller can use NSStringToRunes to extract the runes.
func GetCharacters(event ID) ID {
	if event.IsNil() {
		return 0
	}
	initSelectors()
	return event.Send(selectors.characters)
}

// NSStringLength returns the length of an NSString.
func NSStringLength(nsstr ID) uint64 {
	if nsstr.IsNil() {
		return 0
	}
	initSelectors()
	return nsstr.GetUint64(selectors.length)
}

// NSStringUTF8Ptr returns a pointer to the UTF-8 bytes of an NSString.
// The pointer is only valid until the NSString is released or the autorelease pool is drained.
func NSStringUTF8Ptr(nsstr ID) uintptr {
	if nsstr.IsNil() {
		return 0
	}
	initSelectors()
	return uintptr(nsstr.Send(selectors.UTF8String))
}
