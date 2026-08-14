package gogpu_test

// Golden tests for example rendering output.
//
// These tests compare software-rendered frames against reference PNG images
// stored in testdata/golden/examples/. References are generated with the
// Pure-Go software backend and committed to the repo, making comparisons
// deterministic on machines without a display or GPU.
//
// Workflow:
//
//  1. Run: go test -run TestGolden -args -update-goldens
//     This renders each scene, saves PNGs to testdata/golden/examples/, and exits.
//
//  2. Commit the PNG files.
//
//  3. Run: go test -run TestGolden
//     Renders the same scenes, compares pixel-by-pixel against the stored PNGs.
//     Tests fail if more than Threshold% of pixels differ.

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/golden"
)

// goldenScene describes one example scene for golden comparison.
type goldenScene struct {
	Name      string               // filename stem (no extension)
	Width     int                  // render width in pixels
	Height    int                  // render height in pixels
	Threshold float64              // max % of pixels allowed to differ
	Draw      func(*gogpu.Context) // draw callback, identical to the example's OnDraw
}

// goldenScenes returns the full list of scenes to golden-test.
// Each entry maps directly to a corresponding example under examples/.
//
// Not covered (no GPU rendering or non-deterministic):
//   - closing_window  — no OnDraw
//   - file_dialog     — no OnDraw
//   - menu            — no OnDraw (menu-bar plumbing only)
//   - sound_demo      — audio only, no pixels
//   - timing_test     — uses internal/platform directly, no wgpu
//   - window_only     — no wgpu
//   - gpucontext_integration — no pixel output (prints adapter info)
//   - particles       — GPU compute via DeviceProvider (not available headlessly)
//   - multistage_particle_pipeline — same as particles
func goldenScenes() []goldenScene {
	return []goldenScene{
		{
			// examples/triangle/main.go
			Name:      "triangle",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *gogpu.Context) {
				_ = dc.DrawTriangleColor(gmath.DarkGray)
			},
		},
		{
			// examples/gles_test/main.go
			Name:      "gles-triangle",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *gogpu.Context) {
				_ = dc.DrawTriangleColor(gmath.DarkGray)
			},
		},
		{
			// examples/deviceprovider/main.go
			Name:      "deviceprovider",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *gogpu.Context) {
				_ = dc.DrawTriangleColor(gmath.CornflowerBlue)
			},
		},
		{
			// examples/lifecycle/main.go (primary window)
			Name:      "lifecycle-blue",
			Width:     800,
			Height:    600,
			Threshold: 0.0, // solid color — must be pixel-perfect
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.15, 0.25, 0.65, 1.0)
			},
		},
		{
			// examples/lifecycle/main.go (secondary window)
			Name:      "lifecycle-red",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.65, 0.15, 0.15, 1.0)
			},
		},
		{
			// examples/gpu_timing/main.go — draw is just Clear; timing logic is OnUpdate
			Name:      "gpu-timing",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *gogpu.Context) {
				c := gmath.CornflowerBlue
				dc.Clear(c.R, c.G, c.B, c.A)
			},
		},
		{
			// examples/gpu_vsync/main.go
			Name:      "gpu-vsync",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *gogpu.Context) {
				c := gmath.CornflowerBlue
				dc.Clear(c.R, c.G, c.B, c.A)
			},
		},
		{
			// examples/multiwindow/main.go — primary window
			Name:      "multiwindow-primary",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.2, 0.3, 0.8, 1.0)
			},
		},
		{
			// examples/multiwindow/main.go — secondary window (400×300)
			Name:      "multiwindow-secondary",
			Width:     400,
			Height:    300,
			Threshold: 0.0,
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.8, 0.2, 0.3, 1.0)
			},
		},
		{
			// examples/tabbing/main.go
			Name:      "tabbing",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.1, 0.1, 0.1, 1.0)
			},
		},
		{
			// examples/hidpi/main.go — phase 0: logical-size checker upscaled to fill screen.
			// Simulates a 400×300 texture (logical pixels) scaled to 800×600 viewport.
			Name:      "hidpi-lowres",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.06, 0.06, 0.08, 1.0)
				tex, err := dc.Renderer().NewTextureFromImage(goldenBuildCheckerImage(400, 300, 4))
				if err != nil {
					return
				}
				defer tex.Destroy()
				_ = dc.DrawTextureScaled(tex, 0, 0, 800, 600)
			},
		},
		{
			// examples/hidpi/main.go — phase 1: physical-size checker at 1:1.
			// Simulates a HiDPI texture exactly matching the 800×600 viewport.
			Name:      "hidpi-highres",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *gogpu.Context) {
				dc.Clear(0.06, 0.06, 0.08, 1.0)
				tex, err := dc.Renderer().NewTextureFromImage(goldenBuildCheckerImage(800, 600, 4))
				if err != nil {
					return
				}
				defer tex.Destroy()
				_ = dc.DrawTextureScaled(tex, 0, 0, 800, 600)
			},
		},
		{
			// examples/texture/main.go — checkerboard + gradient textures.
			// Texture data is generated from deterministic CPU code, so the
			// rendered output should be identical across machines.
			Name:      "texture",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *gogpu.Context) {
				dc.ClearColor(gmath.Hex(0x2D2D2D))

				checkerData := goldenCreateCheckerboard(64, 64, 8)
				checkerTex, err := dc.Renderer().NewTextureFromRGBA(64, 64, checkerData)
				if err != nil {
					return
				}
				defer checkerTex.Destroy()

				gradientImg := goldenCreateGradientImage(128, 128)
				gradientTex, err := dc.Renderer().NewTextureFromImage(gradientImg)
				if err != nil {
					return
				}
				defer gradientTex.Destroy()

				_ = dc.DrawTexture(checkerTex, 50, 50)
				_ = dc.DrawTextureScaled(checkerTex, 150, 50, 128, 128)
				_ = dc.DrawTextureEx(checkerTex, gogpu.DrawTextureOptions{X: 300, Y: 50, Alpha: 0.5})
				_ = dc.DrawTexture(gradientTex, 50, 200)
				_ = dc.DrawTextureScaled(gradientTex, 200, 200, 256, 256)
			},
		},
	}
}

// TestGolden_Examples renders each example scene and either writes or compares
// the output PNG in testdata/golden/examples/.
func TestGolden_Examples(t *testing.T) {
	for _, scene := range goldenScenes() {
		scene := scene
		t.Run(scene.Name, func(t *testing.T) {
			golden.CompareWithOptions(t, scene.Name, scene.Width, scene.Height, scene.Draw, golden.Options{
				GoldenDir: filepath.Join("testdata", "golden", "examples"),
				Threshold: scene.Threshold,
			})
		})
	}
}

// --- texture example helpers (mirrors examples/texture/main.go) ---

func goldenCreateCheckerboard(width, height, squareSize int) []byte {
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			if ((x/squareSize)+(y/squareSize))%2 == 0 {
				pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = 255, 255, 255, 255
			} else {
				pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = 50, 50, 200, 255
			}
		}
	}
	return pixels
}

// goldenBuildCheckerImage mirrors examples/hidpi/main.go buildCheckerImage.
func goldenBuildCheckerImage(w, h, cellPx int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	dark := color.RGBA{R: 40, G: 40, B: 40, A: 255}
	light := color.RGBA{R: 220, G: 220, B: 220, A: 255}
	grid := color.RGBA{R: 100, G: 180, B: 255, A: 255}
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

func goldenCreateGradientImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / width),
				G: uint8(y * 255 / height),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}
