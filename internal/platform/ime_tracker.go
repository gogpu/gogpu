package platform

import "unicode/utf16"

// imeTracker is the platform-independent part of the IMM32 lifecycle. It
// keeps composition state and consumes the WM_CHAR/WM_IME_CHAR code units that
// Windows may emit after a result string has already been reported through
// WM_IME_COMPOSITION. Keeping this state machine free of syscalls gives us
// deterministic tests on every host OS.
type imeTracker struct {
	enabled bool
	active  bool

	// result accumulates result strings delivered by one composition session.
	// IMM32 may report several GCS_RESULTSTR chunks before WM_IME_ENDCOMPOSITION.
	result string

	// suppress contains UTF-16 code units expected to be echoed by WM_CHAR.
	// It is intentionally retained through WM_IME_ENDCOMPOSITION because the
	// echo can be queued after the end message.
	suppress []uint16

	// Some IMEs deliver the committed result only as WM_CHAR messages after
	// WM_IME_ENDCOMPOSITION. awaitingChars switches that sequence into result
	// collection so it reaches OnIMECompositionEnd exactly once.
	awaitingChars bool
	charResult    []uint16
}

func (t *imeTracker) setEnabled(enabled bool) (canceled bool) {
	if !enabled && t.active {
		canceled = true
	}
	t.enabled = enabled
	if !enabled {
		t.active = false
		t.result = ""
		// Keep result echoes long enough to swallow WM_CHAR messages already
		// queued by the canceled native session. A new keydown or composition
		// start expires this sequence before accepting fresh input.
		t.awaitingChars = false
		t.charResult = nil
	}
	return canceled
}

func (t *imeTracker) start() bool {
	if !t.enabled {
		return false
	}
	if t.active {
		return false
	}
	// A new start marks a new composition session. A stale suppression sequence
	// belongs to the previous result and must not consume this session's text.
	t.suppress = nil
	t.awaitingChars = false
	t.charResult = nil
	t.active = true
	t.result = ""
	return true
}

// ensureActive handles IMEs that send WM_IME_COMPOSITION without a preceding
// WM_IME_STARTCOMPOSITION (seen with some third-party IMEs).
func (t *imeTracker) ensureActive() bool {
	if !t.enabled {
		return false
	}
	if !t.active {
		// A provider that omits START may begin immediately after a prior
		// post-END result. Do not let that stale lifecycle consume this one.
		t.suppress = nil
		t.awaitingChars = false
		t.charResult = nil
		t.active = true
		t.result = ""
		return true
	}
	return false
}

func (t *imeTracker) addResult(result string) {
	if result == "" {
		return
	}
	t.result += result
	t.suppress = append(t.suppress, utf16.Encode([]rune(result))...)
}

func (t *imeTracker) end() (committed string, ok bool) {
	if !t.active {
		return "", false
	}
	committed = t.result
	t.active = false
	t.result = ""
	return committed, true
}

// beginCharResult starts collecting the legacy post-END WM_CHAR result path.
func (t *imeTracker) beginCharResult() {
	t.awaitingChars = true
	t.charResult = nil
}

func (t *imeTracker) consumeCharResultUnit(codeUnit uint16) bool {
	if !t.awaitingChars {
		return false
	}
	t.charResult = append(t.charResult, codeUnit)
	return true
}

func (t *imeTracker) consumeCharResultUnits(units []uint16) bool {
	if !t.awaitingChars || len(units) == 0 {
		return false
	}
	t.charResult = append(t.charResult, units...)
	return true
}

func (t *imeTracker) finishCharResult() (string, bool) {
	if !t.awaitingChars {
		return "", false
	}
	result := string(utf16.Decode(t.charResult))
	t.awaitingChars = false
	t.charResult = nil
	return result, true
}

func (t *imeTracker) hasPendingCharResult() bool {
	return t.awaitingChars
}

func (t *imeTracker) cancel() bool {
	if !t.active {
		return false
	}
	t.active = false
	t.result = ""
	// Keep suppression for result echoes that were queued before cancellation;
	// start/beforeKeyDown expires it before any new gesture is accepted.
	t.awaitingChars = false
	t.charResult = nil
	return true
}

// beforeKeyDown expires an unmatched suppression sequence before a new
// keyboard gesture. If the native echo belongs to the prior composition it is
// already queued after its WM_IME_COMPOSITION; a later keydown starts a new
// gesture and must not be swallowed.
func (t *imeTracker) beforeKeyDown() {
	if len(t.suppress) > 0 {
		t.suppress = nil
	}
}

// consumeCharUnit reports whether codeUnit is the next native echo of an IME
// result. A mismatch clears the sequence and lets the caller dispatch the
// character normally. Matching the full sequence clears it automatically.
func (t *imeTracker) consumeCharUnit(codeUnit uint16) bool {
	if len(t.suppress) == 0 {
		return false
	}
	if t.suppress[0] != codeUnit {
		t.suppress = nil
		return false
	}
	t.suppress = t.suppress[1:]
	return true
}
