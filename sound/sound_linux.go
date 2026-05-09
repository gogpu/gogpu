//go:build linux

package sound

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// xdgSoundPath maps SystemSound to XDG Sound Theme file paths.
// These follow the freedesktop.org Sound Theme Specification.
// Paths are tried in order; the first existing file wins.
func xdgSoundPaths(s SystemSound) []string {
	const stereo = "/usr/share/sounds/freedesktop/stereo"
	switch s {
	case Click:
		return []string{
			filepath.Join(stereo, "button-pressed.oga"),
			filepath.Join(stereo, "message.oga"),
		}
	case Alert:
		return []string{
			filepath.Join(stereo, "bell.oga"),
			filepath.Join(stereo, "message.oga"),
		}
	case Error:
		return []string{
			filepath.Join(stereo, "dialog-error.oga"),
		}
	case Warning:
		return []string{
			filepath.Join(stereo, "dialog-warning.oga"),
		}
	case Success:
		return []string{
			filepath.Join(stereo, "complete.oga"),
			filepath.Join(stereo, "bell.oga"),
		}
	default:
		return []string{
			filepath.Join(stereo, "bell.oga"),
		}
	}
}

// findSoundFile returns the first existing file from the candidate paths.
func findSoundFile(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// playerCommand returns the command and args for the best available
// sound player on the system. It checks paplay (PulseAudio),
// pw-play (PipeWire), and aplay (ALSA) in order.
func playerCommand() (string, bool) {
	for _, name := range []string{"paplay", "pw-play", "aplay"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, true
		}
	}
	return "", false
}

func platformPlay(s SystemSound) {
	// Try canberra-gtk-play first: it respects the user's selected
	// sound theme and handles XDG sound naming conventions natively.
	if canberaPath, err := exec.LookPath("canberra-gtk-play"); err == nil {
		eventID := canberaSoundID(s)
		cmd := exec.Command(canberaPath, "--id", eventID)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return
		}
	}

	// Fallback: play the XDG freedesktop sound file directly.
	paths := xdgSoundPaths(s)
	file := findSoundFile(paths)
	if file == "" {
		return
	}

	player, ok := playerCommand()
	if !ok {
		return
	}

	cmd := exec.Command(player, file)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}

// canberaSoundID maps SystemSound to libcanberra/XDG event IDs.
func canberaSoundID(s SystemSound) string {
	switch s {
	case Click:
		return "button-pressed"
	case Alert:
		return "bell"
	case Error:
		return "dialog-error"
	case Warning:
		return "dialog-warning"
	case Success:
		return "complete"
	default:
		return "bell"
	}
}

func platformPlayFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("sound: file not found: %w", err)
	}

	player, ok := playerCommand()
	if !ok {
		return fmt.Errorf("sound: no audio player found (tried paplay, pw-play, aplay)")
	}

	cmd := exec.Command(player, path)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("sound: failed to start %s: %w", player, err)
	}

	go cmd.Wait()
	return nil
}
