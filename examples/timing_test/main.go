// Test: Timing analysis - find where the blocking happens
package main

import (
	"fmt"
	"time"

	"github.com/gogpu/gogpu/internal/platform"
)

func main() {
	fmt.Println("Timing Test - finding where blocking happens")

	plat := platform.New()
	err := plat.Init(platform.Config{
		Title:  "Timing Test",
		Width:  800,
		Height: 600,
	})
	if err != nil {
		fmt.Println("Failed:", err)
		return
	}
	defer plat.Destroy()

	frameCount := 0
	lastReport := time.Now()

	var maxPoll, maxSleep time.Duration

	for !plat.ShouldClose() {
		// Measure event polling
		t1 := time.Now()
		for {
			event := plat.PollEvents()
			if event.Type == platform.EventNone {
				break
			}
		}
		pollTime := time.Since(t1)
		if pollTime > maxPoll {
			maxPoll = pollTime
		}

		// Simulate "work" - this is where GPU would render
		t2 := time.Now()
		time.Sleep(time.Microsecond * 100) // 0.1ms fake work
		sleepTime := time.Since(t2)
		if sleepTime > maxSleep {
			maxSleep = sleepTime
		}

		frameCount++
		if time.Since(lastReport) > time.Second {
			fmt.Printf("Loops/sec: %d | MaxPoll: %v | MaxSleep: %v\n",
				frameCount, maxPoll, maxSleep)
			frameCount = 0
			maxPoll = 0
			maxSleep = 0
			lastReport = time.Now()
		}
	}
}
