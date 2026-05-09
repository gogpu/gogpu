// Example: platform system sounds for UI feedback.
//
// Plays all 5 system sounds (Click, Alert, Error, Warning, Success)
// using OS-native APIs (winmm on Windows, NSSound on macOS,
// PulseAudio/canberra on Linux). Zero CGO.
//
// Run:
//
//	go run ./examples/sound_demo
package main

import (
	"fmt"
	"time"

	"github.com/gogpu/gogpu/sound"
)

func main() {
	sound.SetEnabled(true)
	fmt.Println("GoGPU System Sound Demo")
	fmt.Println("=======================")
	fmt.Println()

	sounds := []sound.SystemSound{
		sound.Click,
		sound.Alert,
		sound.Error,
		sound.Warning,
		sound.Success,
	}

	for _, s := range sounds {
		fmt.Printf("  Playing: %s\n", s)
		sound.Play(s)
		time.Sleep(800 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("Done! All sounds played via platform-native APIs.")
}
