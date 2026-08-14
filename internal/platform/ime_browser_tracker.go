package platform

// browserIMETracker is the platform-independent part of the browser IME
// bridge. Browser engines report a composition through both CompositionEvent
// and beforeinput/input, so the tracker keeps those two paths from committing
// the same text twice.
//
// The tracker deliberately carries text, rather than DOM values. DOM values
// use UTF-16 indexes and are converted at the browser boundary; event payloads
// crossing gpucontext are always UTF-8 strings.
type browserIMETracker struct {
	enabled bool
	active  bool

	// preEndInput collects engines that report insertText before
	// compositionend. It is committed by compositionend exactly once.
	preEndInput string

	// suppressInput is the composition result that the following input event
	// is expected to echo after compositionend.
	suppressInput string
}

func (t *browserIMETracker) setEnabled(enabled bool) (canceled bool) {
	if !enabled && t.active {
		canceled = true
	}
	t.enabled = enabled
	if !enabled {
		t.active = false
		t.preEndInput = ""
		// A disabled input must not retain a stale composition echo. The
		// corresponding input event is prevented by the blur/disable path.
		t.suppressInput = ""
	}
	return canceled
}

func (t *browserIMETracker) start() bool {
	if !t.enabled || t.active {
		return false
	}
	t.active = true
	t.preEndInput = ""
	// A new composition starts a fresh lifecycle. An echo belonging to a
	// previous composition must never swallow this one.
	t.suppressInput = ""
	return true
}

// ensureActive handles browsers that omit compositionstart for a replacement
// or paste path but still send compositionupdate.
//
//lint:ignore U1000 called by the js/wasm-only browser platform file
func (t *browserIMETracker) ensureActive() bool {
	if !t.enabled || t.active {
		return false
	}
	return t.start()
}

// input records an input event. Composition input is consumed by the
// composition lifecycle; ordinary input returns text for EventChar delivery.
func (t *browserIMETracker) input(inputType, data string) (text string, consumed bool) {
	if !t.enabled {
		return "", true
	}
	if data == "" {
		return "", true
	}
	if t.suppressInput != "" {
		if data == t.suppressInput {
			t.suppressInput = ""
			return "", true
		}
		// Do not leave a stale result armed across an unrelated browser edit.
		t.suppressInput = ""
	}
	if t.active {
		if inputType == "insertText" || inputType == "insertReplacementText" {
			t.preEndInput += data
		}
		return "", true
	}
	return data, false
}

// end closes the current composition. An empty data value denotes browser
// cancellation; a pre-end insertText payload is used only when compositionend
// omitted its data (a behavior seen in some mobile engines).
func (t *browserIMETracker) end(data string) (committed string, canceled, ok bool) {
	if !t.active {
		return "", false, false
	}
	committed = data
	if committed == "" {
		committed = t.preEndInput
	}
	t.active = false
	t.preEndInput = ""
	if committed == "" {
		t.suppressInput = ""
		return "", true, true
	}
	t.suppressInput = committed
	return committed, false, true
}

func (t *browserIMETracker) cancel() bool {
	if !t.active {
		return false
	}
	t.active = false
	t.preEndInput = ""
	// A cancellation does not commit any preedit and browsers may still send
	// an empty compositionend afterward.
	t.suppressInput = ""
	return true
}
