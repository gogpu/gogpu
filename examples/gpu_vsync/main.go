// Test: GPU with VSync (Fifo mode)
// VSync naturally throttles frame rate without aggressive fence polling
package main

import (
	"fmt"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	fmt.Println("GPU Test with VSync (Fifo mode)")
	fmt.Println("This should be more responsive...")

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GPU VSync Test").
		WithSize(800, 600))

	frameCount := 0
	lastReport := time.Now()

	app.OnDraw(func(dc *gogpu.Context) {
		c := gmath.CornflowerBlue
		dc.Clear(c.R, c.G, c.B, c.A)
		frameCount++
	})

	app.OnUpdate(func(dt float64) {
		if time.Since(lastReport) > time.Second {
			fmt.Printf("FPS: %d\n", frameCount)
			frameCount = 0
			lastReport = time.Now()
		}
	})

	if err := app.Run(); err != nil {
		fmt.Println("Error:", err)
	}
}
