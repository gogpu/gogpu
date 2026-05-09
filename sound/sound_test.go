package sound

import (
	"testing"
	"time"
)

func TestSetEnabled(t *testing.T) {
	// Reset state after test.
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.mu.Unlock()
	}()

	if Enabled() {
		t.Fatal("expected disabled by default")
	}

	SetEnabled(true)
	if !Enabled() {
		t.Fatal("expected enabled after SetEnabled(true)")
	}

	SetEnabled(false)
	if Enabled() {
		t.Fatal("expected disabled after SetEnabled(false)")
	}
}

func TestSystemSoundString(t *testing.T) {
	tests := []struct {
		sound SystemSound
		want  string
	}{
		{Click, "Click"},
		{Alert, "Alert"},
		{Error, "Error"},
		{Warning, "Warning"},
		{Success, "Success"},
		{SystemSound(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.sound.String(); got != tt.want {
				t.Errorf("SystemSound(%d).String() = %q, want %q", tt.sound, got, tt.want)
			}
		})
	}
}

func TestPlayDisabled(t *testing.T) {
	// Sounds are disabled by default; Play should not panic.
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.mu.Unlock()
	}()

	Play(Click)
	Play(Alert)
	Play(Error)
	Play(Warning)
	Play(Success)
}

func TestPlayFileDisabled(t *testing.T) {
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.mu.Unlock()
	}()

	// When disabled, PlayFile should return nil without attempting playback.
	if err := PlayFile("nonexistent.wav"); err != nil {
		t.Errorf("PlayFile with sounds disabled should return nil, got %v", err)
	}
}

func TestDebounce(t *testing.T) {
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.lastPlay = [5]time.Time{}
		state.mu.Unlock()
	}()

	SetEnabled(true)

	// Manually set lastPlay to "now" so that the next Play is within debounce.
	state.mu.Lock()
	state.lastPlay[int(Click)] = time.Now()
	state.mu.Unlock()

	// This should be debounced (skipped). We can't directly observe the skip
	// from the public API without mocking platformPlay, but we verify no panic
	// and that the lastPlay time does NOT advance.
	state.mu.Lock()
	before := state.lastPlay[int(Click)]
	state.mu.Unlock()

	Play(Click)

	state.mu.Lock()
	after := state.lastPlay[int(Click)]
	state.mu.Unlock()

	if !before.Equal(after) {
		t.Error("debounce should have prevented lastPlay update")
	}
}

func TestDebounceExpired(t *testing.T) {
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.lastPlay = [5]time.Time{}
		state.mu.Unlock()
	}()

	SetEnabled(true)

	// Set lastPlay to well in the past so debounce does not apply.
	past := time.Now().Add(-time.Second)
	state.mu.Lock()
	state.lastPlay[int(Alert)] = past
	state.mu.Unlock()

	Play(Alert)

	state.mu.Lock()
	after := state.lastPlay[int(Alert)]
	state.mu.Unlock()

	// lastPlay should have been updated (moved forward from the past value).
	if !after.After(past) {
		t.Error("expected lastPlay to be updated after debounce expired")
	}
}

func TestPlayOutOfRange(t *testing.T) {
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.mu.Unlock()
	}()

	SetEnabled(true)

	// Out-of-range SystemSound values should not panic.
	Play(SystemSound(-1))
	Play(SystemSound(100))
}

func TestPlayFileEnabled(t *testing.T) {
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.mu.Unlock()
	}()

	SetEnabled(true)

	// Playing a nonexistent file should return an error on all platforms.
	err := PlayFile("this-file-definitely-does-not-exist-12345.wav")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestDifferentSoundsNotDebounced(t *testing.T) {
	defer func() {
		state.mu.Lock()
		state.enabled = false
		state.lastPlay = [5]time.Time{}
		state.mu.Unlock()
	}()

	SetEnabled(true)

	// Set Click as recently played.
	state.mu.Lock()
	state.lastPlay[int(Click)] = time.Now()
	state.mu.Unlock()

	// Alert should NOT be debounced (different sound).
	state.mu.Lock()
	state.lastPlay[int(Alert)] = time.Time{}
	state.mu.Unlock()

	Play(Alert)

	state.mu.Lock()
	alertTime := state.lastPlay[int(Alert)]
	state.mu.Unlock()

	if alertTime.IsZero() {
		t.Error("Alert should not be debounced when only Click was recent")
	}
}
