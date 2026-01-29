// Example: gg + gogpu integration
//
// This example demonstrates rendering 2D graphics with gg
// directly into a gogpu window. This is the foundation for UI systems.
//
// Architecture:
//
//	gg (2D) → Context.Image() (CPU) → gogpu.Texture (GPU) → Window
package main

import (
	"image"
	"log"
	"math"
	"time"

	"github.com/gogpu/gg"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	const width, height = 800, 600

	// Create gogpu application
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU + gg Integration Demo").
		WithSize(width, height))

	// gg rendering resources
	var (
		ctx     *gg.Context
		texture *gogpu.Texture
		frame   int
	)

	app.OnDraw(func(dc *gogpu.Context) {
		// Initialize gg context and texture on first frame
		if ctx == nil {
			ctx = gg.NewContext(width, height)

			var err error
			texture, err = dc.Renderer().NewTextureFromRGBA(width, height, make([]byte, width*height*4))
			if err != nil {
				log.Fatalf("Failed to create texture: %v", err)
			}
		}

		// Clear gogpu window
		dc.ClearColor(gmath.Hex(0x1a1a2e))

		// Render 2D graphics with gg
		renderWithGG(ctx, frame)
		frame++

		// Get rendered image and upload to GPU texture
		img := ctx.Image()
		if rgba, ok := img.(*image.RGBA); ok {
			texture.UpdateData(rgba.Pix)
		}

		// Draw gg output to gogpu window
		if err := dc.DrawTexture(texture, 0, 0); err != nil {
			log.Printf("DrawTexture error: %v", err)
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

	if texture != nil {
		texture.Destroy()
	}
}

// renderWithGG draws 2D graphics using gg library
func renderWithGG(ctx *gg.Context, frame int) {
	// Clear with transparent background
	ctx.SetRGBA(0, 0, 0, 0)
	ctx.Clear()

	// Animation parameters
	t := float64(frame) * 0.02
	centerX, centerY := 400.0, 300.0

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

	// Draw FPS counter
	ctx.SetRGB(0.5, 1, 0.5)
	ctx.DrawString("Frame: "+itoa(frame), 10, 20)
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

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func init() {
	// Suppress unused import warning
	_ = time.Second
}
