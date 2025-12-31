// Example: Basic triangle rendering
//
// This example demonstrates the gogpu API by creating a window
// and clearing it with a cornflower blue color.
//
// Compare this ~20 lines with 480+ lines of raw WebGPU code!
package main

import (
	"log"
	"runtime"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	// Create application with simple configuration
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU - Triangle Example").
		WithSize(800, 600))

	// Set draw callback - called every frame
	app.OnDraw(func(ctx *gogpu.Context) {
		// Draw RGB triangle on dark background
		err := ctx.DrawTriangleColor(gmath.DarkGray)

		if err != nil {
			println("DrawTriangle failed:", err.Error())
			if runtime.GOOS == "darwin" {
				// Panic can be used to test for segfaults under panic with ObjC
				// panic("DrawTriangle failed: " + err.Error())
			}
		}
	})

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
