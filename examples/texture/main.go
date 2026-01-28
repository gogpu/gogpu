// Example: Texture rendering demonstration
//
// This example demonstrates the texture rendering API in gogpu.
// It shows:
// - Creating a texture from raw RGBA data
// - Drawing textures at different positions
// - Drawing textures with scaling
// - Drawing textures with alpha blending
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
		WithTitle("GoGPU - Texture Rendering Demo").
		WithSize(800, 600))

	// Create textures after app is initialized
	var checkerTex, gradientTex *gogpu.Texture
	var texError error

	app.OnDraw(func(dc *gogpu.Context) {
		// Clear with dark background
		dc.ClearColor(gmath.Hex(0x2D2D2D))

		// Create textures on first frame (renderer is available)
		if checkerTex == nil && texError == nil {
			checkerTex, gradientTex, texError = createDemoTextures(dc.Renderer())
			if texError != nil {
				log.Printf("Failed to create textures: %v", texError)
			}
		}

		// Draw textures if available
		if checkerTex != nil {
			// Draw checkerboard at original size (64x64)
			if err := dc.DrawTexture(checkerTex, 50, 50); err != nil {
				log.Printf("DrawTexture error: %v", err)
			}

			// Draw checkerboard scaled up (2x)
			if err := dc.DrawTextureScaled(checkerTex, 150, 50, 128, 128); err != nil {
				log.Printf("DrawTextureScaled error: %v", err)
			}

			// Draw checkerboard with alpha (50% opacity)
			if err := dc.DrawTextureEx(checkerTex, gogpu.DrawTextureOptions{
				X:     300,
				Y:     50,
				Alpha: 0.5,
			}); err != nil {
				log.Printf("DrawTextureEx error: %v", err)
			}
		}

		if gradientTex != nil {
			// Draw gradient texture
			if err := dc.DrawTexture(gradientTex, 50, 200); err != nil {
				log.Printf("DrawTexture error: %v", err)
			}

			// Draw gradient scaled (2x)
			if err := dc.DrawTextureScaled(gradientTex, 200, 200, 256, 256); err != nil {
				log.Printf("DrawTextureScaled error: %v", err)
			}
		}
	})

	// Run the application
	// Note: Textures created via Renderer are automatically cleaned up
	// when the application exits, but explicit cleanup is good practice.
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

	// Cleanup textures after Run returns
	if checkerTex != nil {
		checkerTex.Destroy()
	}
	if gradientTex != nil {
		gradientTex.Destroy()
	}
}

// createDemoTextures creates demo textures for the example.
func createDemoTextures(renderer *gogpu.Renderer) (*gogpu.Texture, *gogpu.Texture, error) {
	// Create checkerboard texture (64x64)
	checkerData := createCheckerboard(64, 64, 8)
	checkerTex, err := renderer.NewTextureFromRGBA(64, 64, checkerData)
	if err != nil {
		return nil, nil, fmt.Errorf("checkerboard texture: %w", err)
	}

	// Create gradient texture from Go image (128x128)
	gradientImg := createGradientImage(128, 128)
	gradientTex, err := renderer.NewTextureFromImage(gradientImg)
	if err != nil {
		checkerTex.Destroy()
		return nil, nil, fmt.Errorf("gradient texture: %w", err)
	}

	return checkerTex, gradientTex, nil
}

// createCheckerboard creates a checkerboard pattern as RGBA data.
func createCheckerboard(width, height, squareSize int) []byte {
	pixels := make([]byte, width*height*4)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			// Checkerboard pattern
			if ((x/squareSize)+(y/squareSize))%2 == 0 {
				pixels[i] = 255   // R
				pixels[i+1] = 255 // G
				pixels[i+2] = 255 // B
				pixels[i+3] = 255 // A
			} else {
				pixels[i] = 50    // R
				pixels[i+1] = 50  // G
				pixels[i+2] = 200 // B
				pixels[i+3] = 255 // A
			}
		}
	}

	return pixels
}

// createGradientImage creates a gradient test image.
func createGradientImage(width, height int) image.Image {
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
