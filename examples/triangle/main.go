// Example: Basic triangle rendering
//
// This example demonstrates the gogpu API by creating a window
// and clearing it with a cornflower blue color.
//
// Compare this ~20 lines with 480+ lines of raw WebGPU code!
package main

import (
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/math"
)

func main() {
	// Create application with simple configuration
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU - Triangle Example").
		WithSize(800, 600))

	// Set draw callback - called every frame
	app.OnDraw(func(ctx *gogpu.Context) {
		// Draw RGB triangle on dark background
		ctx.DrawTriangleColor(math.DarkGray)
	})

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
