// Example: GLES backend test — triangle rendering
//
// Tests the GLES rendering pipeline with shader compilation.
// Expected: red triangle on dark background.
package main

import (
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("GLES triangle test starting...")

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU GLES: Triangle Test").
		WithGraphicsAPI(gogpu.GraphicsAPIGLES).
		WithSize(800, 600))

	var frame int

	app.OnDraw(func(dc *gogpu.Context) {
		if frame < 3 {
			log.Printf("Frame %d: Backend=%s Size=%dx%d", frame, dc.Backend(), dc.Width(), dc.Height())
		}

		if err := dc.DrawTriangleColor(gmath.DarkGray); err != nil {
			if frame < 5 {
				log.Printf("Frame %d: DrawTriangle error: %v", frame, err)
			}
		}
		frame++
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
