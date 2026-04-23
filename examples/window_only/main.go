// Test: Window only - no GPU rendering
// This tests if the platform layer itself is responsive
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gogpu/gogpu/internal/platform"
)

func main() {
	runtime.LockOSThread()

	mgr := platform.NewManager()
	if err := mgr.Init(); err != nil {
		fmt.Println("Failed to init platform:", err)
		return
	}
	defer mgr.Destroy()

	win, err := mgr.CreateWindow(platform.Config{
		Title:  "Window Only Test - No GPU",
		Width:  800,
		Height: 600,
	})
	if err != nil {
		fmt.Println("Failed to create window:", err)
		return
	}
	defer win.Destroy()

	fmt.Println("Window created. Testing responsiveness WITHOUT GPU...")
	fmt.Println("Try dragging/resizing the window.")

	frameCount := 0
	lastReport := time.Now()

	for !win.ShouldClose() {
		// Just process events - no rendering
		for {
			event := mgr.PollEvents()
			if event.Type == platform.EventNone {
				break
			}
			if event.Type == platform.EventResize {
				fmt.Printf("Resize: %dx%d\n", event.Width, event.Height)
			}
		}

		frameCount++
		if time.Since(lastReport) > time.Second {
			fmt.Printf("Loop iterations/sec: %d\n", frameCount)
			frameCount = 0
			lastReport = time.Now()
		}

		// Small sleep to not burn CPU
		time.Sleep(time.Millisecond)
	}

	fmt.Println("Window closed.")
}
