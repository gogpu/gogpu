package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"time"

	"github.com/gogpu/gogpu"
)

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("HiDPI — logical res → physical res (cycles every 2s)").
		WithSize(800, 600).
		WithContinuousRender(false))

	fmt.Printf("ScaleFactor before Run: %.1f\n", app.ScaleFactor())

	go func() {
		for range time.Tick(2 * time.Second) {
			app.RequestRedraw()
		}
	}()

	var (
		texLow  *gogpu.Texture
		texHigh *gogpu.Texture
		lastPW  int
		lastPH  int
		lw, lh  int
		start   = time.Now()
		phase   = -1
	)

	app.OnDraw(func(dc *gogpu.Context) {
		scale := dc.ScaleFactor()
		pw, ph := dc.FramebufferSize()

		sizeChanged := pw > 0 && ph > 0 && (pw != lastPW || ph != lastPH)
		if sizeChanged {
			if texLow != nil {
				texLow.Destroy()
			}
			if texHigh != nil {
				texHigh.Destroy()
			}

			lw = int(float64(pw) / scale)
			lh = int(float64(ph) / scale)

			var err error
			texLow, err = dc.Renderer().NewTextureFromImage(buildCheckerImage(lw, lh, 4))
			if err != nil {
				log.Printf("texLow: %v", err)
			}
			texHigh, err = dc.Renderer().NewTextureFromImage(buildCheckerImage(pw, ph, 4))
			if err != nil {
				log.Printf("texHigh: %v", err)
			}

			lastPW, lastPH = pw, ph
			phase = -1

			fmt.Printf("scale=%.1f  logical=%dx%d  physical=%dx%d\n", scale, lw, lh, pw, ph)
		}

		cur := int(time.Since(start).Seconds()/2) % 2
		dc.Clear(0.06, 0.06, 0.08, 1.0)

		if cur == 0 && texLow != nil {
			_ = dc.DrawTextureScaled(texLow, 0, 0, float32(pw), float32(ph))
			if cur != phase {
				fmt.Printf("[logical]   texture %dx%d upscaled to %dx%d\n", lw, lh, pw, ph)
			}
		} else if cur == 1 && texHigh != nil {
			_ = dc.DrawTextureScaled(texHigh, 0, 0, float32(pw), float32(ph))
			if cur != phase {
				fmt.Printf("[physical]  texture %dx%d pixel-perfect\n", pw, ph)
			}
		}
		phase = cur
	})

	app.OnClose(func() {
		if texLow != nil {
			texLow.Destroy()
		}
		if texHigh != nil {
			texHigh.Destroy()
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildCheckerImage(w, h, cellPx int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	dark := color.RGBA{40, 40, 40, 255}
	light := color.RGBA{220, 220, 220, 255}
	grid := color.RGBA{100, 180, 255, 255}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x%cellPx == 0 || y%cellPx == 0 {
				img.SetRGBA(x, y, grid)
				continue
			}
			if ((x/cellPx)+(y/cellPx))%2 == 0 {
				img.SetRGBA(x, y, light)
			} else {
				img.SetRGBA(x, y, dark)
			}
		}
	}
	return img
}
