// Example: Texture API demonstration (v0.3.0-alpha preview)
//
// This example demonstrates the texture loading API in gogpu.
// Full texture rendering (displaying textures on screen) will be
// available in v0.3.0-alpha when bind groups and pipelines are integrated.
//
// This example shows:
// - Texture creation from raw RGBA data
// - Texture creation from Go image.Image
// - Texture metadata access
// - Texture resource management
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	// Create application
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU - Texture API Demo").
		WithSize(800, 600))

	// Set draw callback
	app.OnDraw(func(ctx *gogpu.Context) {
		// For now, just clear with a color
		// Full texture rendering will be in v0.3.0-alpha
		ctx.ClearColor(gmath.Hex(0x2D2D2D))
	})

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// The following functions demonstrate the Texture API.
// In a real application, these would be called with a valid Renderer.
// These are exported as Example* functions for documentation.

// ExampleTextureFromRGBA shows creating a texture from raw pixel data.
func ExampleTextureFromRGBA(renderer *gogpu.Renderer) {
	// Create a simple checkerboard texture (8x8, 64 pixels)
	width, height := 8, 8
	pixels := make([]byte, width*height*4)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			// Checkerboard pattern
			if (x+y)%2 == 0 {
				pixels[i] = 255   // R
				pixels[i+1] = 255 // G
				pixels[i+2] = 255 // B
				pixels[i+3] = 255 // A
			} else {
				pixels[i] = 100   // R
				pixels[i+1] = 100 // G
				pixels[i+2] = 100 // B
				pixels[i+3] = 255 // A
			}
		}
	}

	// Create texture from raw RGBA data
	tex, err := renderer.NewTextureFromRGBA(width, height, pixels)
	if err != nil {
		log.Printf("Failed to create texture: %v", err)
		return
	}
	defer tex.Destroy()

	fmt.Printf("Checkerboard texture: %dx%d\n", tex.Width(), tex.Height())
}

// ExampleTextureFromImage shows creating a texture from Go image.
func ExampleTextureFromImage(renderer *gogpu.Renderer) {
	// Create a gradient test image
	img := CreateGradientImage(256, 256)

	// Create texture from Go image
	tex, err := renderer.NewTextureFromImage(img)
	if err != nil {
		log.Printf("Failed to create gradient texture: %v", err)
		return
	}
	defer tex.Destroy()

	fmt.Printf("Gradient texture: %dx%d, format=%d\n",
		tex.Width(), tex.Height(), tex.Format())
}

// ExampleTextureFromFile shows loading a texture from file.
func ExampleTextureFromFile(renderer *gogpu.Renderer, path string) {
	// Load texture from PNG or JPEG file
	tex, err := renderer.LoadTexture(path)
	if err != nil {
		log.Printf("Failed to load texture: %v", err)
		return
	}
	defer tex.Destroy()

	fmt.Printf("Loaded texture: %dx%d from %s\n",
		tex.Width(), tex.Height(), path)
}

// CreateGradientImage creates a gradient test image for demos.
func CreateGradientImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(x * 255 / width)
			g := uint8(y * 255 / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}
