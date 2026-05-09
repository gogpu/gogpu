//go:build windows

package sound

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmm       = windows.NewLazyDLL("winmm.dll")
	procPlaySnd = winmm.NewProc("PlaySoundW")
)

// PlaySoundW flags.
const (
	sndAsync     = 0x0001     // play asynchronously
	sndNoDefault = 0x0002     // do not play default sound on failure
	sndAlias     = 0x00010000 // pszSound is a registry alias
	sndFilename  = 0x00020000 // pszSound is a file name
)

// windowsSoundAlias maps SystemSound to Windows registry sound aliases.
// These correspond to entries under HKCU\AppEvents\Schemes\Apps\.Default.
func windowsSoundAlias(s SystemSound) string {
	switch s {
	case Click:
		return ".Default"
	case Alert:
		return "SystemNotification"
	case Error:
		return "SystemHand"
	case Warning:
		return "SystemExclamation"
	case Success:
		return "SystemAsterisk"
	default:
		return ".Default"
	}
}

func platformPlay(s SystemSound) {
	alias := windowsSoundAlias(s)
	ptr, err := windows.UTF16PtrFromString(alias)
	if err != nil {
		return
	}
	// PlaySoundW(pszSound, hmod, fdwSound)
	// SND_ALIAS|SND_ASYNC|SND_NODEFAULT: play the registry alias
	// asynchronously; if no sound is configured, skip silently.
	procPlaySnd.Call(
		uintptr(unsafe.Pointer(ptr)),
		0,
		uintptr(sndAlias|sndAsync|sndNoDefault),
	)
}

func platformPlayFile(path string) error {
	// Check file existence first because PlaySoundW with SND_ASYNC
	// returns TRUE even for missing files (it silently does nothing).
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("sound: file not found: %w", err)
	}

	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("sound: invalid path: %w", err)
	}
	ret, _, _ := procPlaySnd.Call(
		uintptr(unsafe.Pointer(ptr)),
		0,
		uintptr(sndFilename|sndAsync|sndNoDefault),
	)
	if ret == 0 {
		return fmt.Errorf("sound: PlaySoundW failed for %q", path)
	}
	return nil
}
