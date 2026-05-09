// Package sound provides platform system sound playback for UI feedback.
//
// It delegates to OS-native APIs (winmm on Windows, NSSound on macOS,
// PulseAudio on Linux) and requires zero CGO.
//
// Sounds are disabled by default. Call [SetEnabled](true) to activate playback.
// All playback is asynchronous and non-blocking.
//
//	sound.SetEnabled(true)
//	sound.Play(sound.Click)   // plays OS button-click sound
//	sound.Play(sound.Error)   // plays OS error sound
package sound

import (
	"sync"
	"time"
)

// SystemSound represents a platform system sound event.
type SystemSound int

const (
	// Click is the default UI interaction sound (button press, menu select).
	Click SystemSound = iota
	// Alert is a notification sound.
	Alert
	// Error indicates an error occurred.
	Error
	// Warning indicates a warning condition.
	Warning
	// Success indicates an operation completed successfully.
	Success
)

// String returns the human-readable name of a SystemSound.
func (s SystemSound) String() string {
	switch s {
	case Click:
		return "Click"
	case Alert:
		return "Alert"
	case Error:
		return "Error"
	case Warning:
		return "Warning"
	case Success:
		return "Success"
	default:
		return "Unknown"
	}
}

// debounceInterval is the minimum time between identical sound events.
// Playing the same sound within this window is silently skipped to avoid
// audible stuttering from rapid-fire UI events.
const debounceInterval = 50 * time.Millisecond

// state holds the global sound subsystem state.
var state struct {
	mu      sync.Mutex
	enabled bool

	// lastPlay tracks the last play time per SystemSound for debounce.
	lastPlay [5]time.Time // indexed by SystemSound
}

// SetEnabled enables or disables UI sound playback globally.
// Disabled by default. Call SetEnabled(true) to activate.
func SetEnabled(enabled bool) {
	state.mu.Lock()
	state.enabled = enabled
	state.mu.Unlock()
}

// Enabled reports whether UI sounds are currently enabled.
func Enabled() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.enabled
}

// Play plays a system sound asynchronously.
// If sounds are disabled or the same sound was played within the debounce
// interval (50ms), the call is silently ignored.
func Play(s SystemSound) {
	state.mu.Lock()
	if !state.enabled {
		state.mu.Unlock()
		return
	}

	idx := int(s)
	if idx < 0 || idx >= len(state.lastPlay) {
		state.mu.Unlock()
		return
	}

	now := time.Now()
	if now.Sub(state.lastPlay[idx]) < debounceInterval {
		state.mu.Unlock()
		return
	}
	state.lastPlay[idx] = now
	state.mu.Unlock()

	platformPlay(s)
}

// PlayFile plays a WAV file from the given path asynchronously.
// Returns an error if the file cannot be played (e.g., path not found,
// unsupported format, or platform API failure).
// If sounds are disabled, returns nil without playing.
func PlayFile(path string) error {
	state.mu.Lock()
	if !state.enabled {
		state.mu.Unlock()
		return nil
	}
	state.mu.Unlock()

	return platformPlayFile(path)
}
