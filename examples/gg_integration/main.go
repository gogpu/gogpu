// Example: gg + gogpu integration via ggcanvas
//
// This example demonstrates rendering 2D graphics with gg
// directly into a gogpu window using the ggcanvas integration package.
//
// Architecture:
//
//	gg.Context (draw) → ggcanvas.Canvas → gogpu.Context (GPU) → Window
//
// Requirements:
//   - gogpu v0.13.3+
//   - gg v0.21.4+
package main

import (
	"log"
	"math"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	const width, height = 800, 600

	// Create gogpu application
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU + gg Integration via ggcanvas").
		WithSize(width, height))

	// Canvas for 2D rendering (created lazily)
	var canvas *ggcanvas.Canvas
	var frame int

	app.OnDraw(func(dc *gogpu.Context) {
		// Print backend on first frame
		if frame == 0 {
			log.Printf("Backend: %s", dc.Backend())
		}

		// Get actual window size
		w, h := dc.Width(), dc.Height()
		if w <= 0 || h <= 0 {
			return
		}

		// Clear window background
		dc.ClearColor(gmath.Hex(0x1a1a2e))

		// Lazy canvas initialization (needs GPUContextProvider)
		if canvas == nil {
			provider := app.GPUContextProvider()
			if provider == nil {
				return // Not ready yet
			}

			var err error
			canvas, err = ggcanvas.New(provider, w, h)
			if err != nil {
				log.Fatalf("Failed to create canvas: %v", err)
			}
			log.Printf("Canvas created: %dx%d", w, h)
		}

		// Draw 2D graphics using gg API
		ctx := canvas.Context()
		cw, ch := canvas.Size()
		renderFrame(ctx, frame, cw, ch)

		// Debug: save first frame to PNG
		if frame == 0 {
			_ = ctx.SavePNG("debug_canvas.png")
			log.Printf("Saved debug_canvas.png (%dx%d)", cw, ch)
		}
		frame++

		// Render canvas to gogpu window (handles texture upload automatically)
		if err := canvas.RenderTo(dc.AsTextureDrawer()); err != nil {
			log.Printf("RenderTo error: %v", err)
		}
	})

	// Handle window resize
	app.EventSource().OnResize(func(w, h int) {
		if canvas != nil {
			if err := canvas.Resize(w, h); err != nil {
				log.Printf("Resize error: %v", err)
			}
		}
	})

	// Run application
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

	// Clean up
	if canvas != nil {
		canvas.Close()
	}
}

// renderFrame draws animated 2D graphics using gg
func renderFrame(ctx *gg.Context, frame int, width, height int) {
	// Clear with transparent background
	ctx.SetRGBA(0, 0, 0, 0)
	ctx.Clear()

	// Animation parameters
	t := float64(frame) * 0.02
	centerX, centerY := float64(width)/2, float64(height)/2

	// Draw animated circles
	for i := 0; i < 12; i++ {
		angle := float64(i)*math.Pi/6 + t
		radius := 150.0
		x := centerX + math.Cos(angle)*radius
		y := centerY + math.Sin(angle)*radius

		// Gradient color based on angle
		hue := float64(i) / 12.0
		r, g, b := hsvToRGB(hue, 0.8, 1.0)
		ctx.SetRGB(r, g, b)

		// Draw filled circle
		circleRadius := 30 + 10*math.Sin(t*2+float64(i))
		ctx.DrawCircle(x, y, circleRadius)
		ctx.Fill()
	}

	// Draw center text
	ctx.SetRGB(1, 1, 1)
	ctx.DrawStringAnchored("gg + gogpu", centerX, centerY, 0.5, 0.5)
}

// hsvToRGB converts HSV to RGB
func hsvToRGB(h, s, v float64) (r, g, b float64) {
	if s == 0 {
		return v, v, v
	}

	h *= 6
	i := math.Floor(h)
	f := h - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	switch int(i) % 6 {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}
