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

// navigationWav is a subtle UI click sound shipped with Windows.
const navigationWav = `C:\Windows\Media\Windows Navigation Start.wav`

func playAlias(alias string) {
	ptr, err := windows.UTF16PtrFromString(alias)
	if err != nil {
		return
	}
	procPlaySnd.Call(
		uintptr(unsafe.Pointer(ptr)),
		0,
		uintptr(sndAlias|sndAsync|sndNoDefault),
	)
}

func playWavFile(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	ret, _, _ := procPlaySnd.Call(
		uintptr(unsafe.Pointer(ptr)),
		0,
		uintptr(sndFilename|sndAsync|sndNoDefault),
	)
	return ret != 0
}

func platformPlay(s SystemSound) {
	switch s {
	case Click, Invoke, Focus, MoveNext, MovePrev, GoBack:
		if playWavFile(navigationWav) {
			return
		}
		playAlias(".Default")
	case Show:
		playAlias("SystemNotification")
	case Hide:
		// Silent -- standard UX, no sound on dismiss.
	case Alert:
		playAlias("SystemNotification")
	case Error:
		playAlias("SystemHand")
	case Warning:
		playAlias("SystemExclamation")
	case Success:
		playAlias("SystemAsterisk")
	}
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
