// Test: GPU Timing - find exactly where blocking happens
// This test measures timing of each phase in the render loop
package main

import (
	"fmt"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	fmt.Println("GPU Timing Test - finding where blocking happens")
	fmt.Println("Watch for spikes in any phase...")
	fmt.Println()

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GPU Timing Test").
		WithSize(800, 600))

	frameCount := 0
	lastReport := time.Now()

	// Per-frame timing accumulators
	var totalUpdateTime, totalDrawTime time.Duration
	var maxUpdateTime, maxDrawTime time.Duration

	// Track when update was called
	var updateTime time.Time

	app.OnUpdate(func(dt float64) {
		updateTime = time.Now()
	})

	app.OnDraw(func(dc *gogpu.Context) {
		// Time from Update to Draw = includes BeginFrame (where fence waits happen)
		beginFrameTime := time.Since(updateTime)
		totalUpdateTime += beginFrameTime
		if beginFrameTime > maxUpdateTime {
			maxUpdateTime = beginFrameTime
		}

		// Draw timing
		drawStart := time.Now()
		c := gmath.CornflowerBlue
		dc.Clear(c.R, c.G, c.B, c.A)
		drawTime := time.Since(drawStart)
		totalDrawTime += drawTime
		if drawTime > maxDrawTime {
			maxDrawTime = drawTime
		}

		frameCount++

		// Report every second
		if time.Since(lastReport) > time.Second && frameCount > 0 {
			avgBeginFrame := totalUpdateTime / time.Duration(frameCount)
			avgDraw := totalDrawTime / time.Duration(frameCount)

			fmt.Printf("FPS: %d | BeginFrame: avg=%v max=%v | Draw: avg=%v max=%v\n",
				frameCount, avgBeginFrame, maxUpdateTime, avgDraw, maxDrawTime)

			// Reset
			frameCount = 0
			totalUpdateTime, totalDrawTime = 0, 0
			maxUpdateTime, maxDrawTime = 0, 0
			lastReport = time.Now()
		}
	})

	if err := app.Run(); err != nil {
		fmt.Println("Error:", err)
	}
}
