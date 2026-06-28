package gogpu

// Golden tests for example rendering output.
//
// These tests compare GPU-rendered frames against reference PNG images stored
// in testdata/golden/examples/. Reference images are generated once on a
// machine with a working GPU (e.g., Windows + discrete GPU) and committed to
// the repo. Future runs on other OSes/GPUs compare against those references
// with a per-pixel threshold to allow for minor driver/backend differences.
//
// Workflow:
//
//  1. On a reference machine (Windows/GPU): go test -run TestGolden -args -update-golden
//     This renders each scene, saves PNGs to testdata/golden/examples/, and exits.
//
//  2. Commit the PNG files.
//
//  3. On any other OS: go test -run TestGolden
//     Renders the same scenes, compares pixel-by-pixel against the stored PNGs.
//     Tests fail if more than Threshold% of pixels differ.
//
// Skipping: tests skip automatically when no GPU adapter is available (CI without
// hardware, etc.).

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
)

var updateGolden = flag.Bool("update-golden", false, "write golden PNG files instead of comparing")

// goldenExamplesDir is the directory holding reference PNGs.
func goldenExamplesDir() string {
	return filepath.Join("testdata", "golden", "examples")
}

// goldenExamplePath returns the path for a named golden PNG.
func goldenExamplePath(name string) string {
	return filepath.Join(goldenExamplesDir(), name+".png")
}

// goldenDirHasPNGs reports whether testdata/golden/examples/ contains at least
// one *.png file. Used to skip tests early before touching the GPU.
func goldenDirHasPNGs() bool {
	entries, err := os.ReadDir(goldenExamplesDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".png" {
			return true
		}
	}
	return false
}

// newHeadlessRenderer initializes a Renderer without a window or display.
// The renderer has a GPU device but no surface — it can only render via
// RenderToImage. Skips the test if no GPU adapter is available.
func newHeadlessRenderer(t *testing.T) *Renderer {
	t.Helper()

	r := &Renderer{
		powerPreference: gputypes.PowerPreferenceHighPerformance,
	}
	r.primary = &RenderTarget{renderer: r}

	if err := r.initInstance(types.GraphicsAPIAuto); err != nil {
		t.Skipf("no GPU instance: %v", err)
	}
	if err := r.initAdapterDevice(nil); err != nil {
		t.Skipf("no GPU adapter/device: %v", err)
	}
	t.Logf("GPU adapter: %s", r.adapter.Info().Name)
	return r
}

// goldenScene describes one example scene for golden comparison.
type goldenScene struct {
	Name      string         // filename stem (no extension)
	Width     int            // render width in pixels
	Height    int            // render height in pixels
	Threshold float64        // max % of pixels allowed to differ
	Draw      func(*Context) // draw callback, identical to the example's OnDraw
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
			Draw: func(dc *Context) {
				_ = dc.DrawTriangleColor(gmath.DarkGray)
			},
		},
		{
			// examples/gles_test/main.go
			Name:      "gles-triangle",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *Context) {
				_ = dc.DrawTriangleColor(gmath.DarkGray)
			},
		},
		{
			// examples/deviceprovider/main.go
			Name:      "deviceprovider",
			Width:     800,
			Height:    600,
			Threshold: 1.0,
			Draw: func(dc *Context) {
				_ = dc.DrawTriangleColor(gmath.CornflowerBlue)
			},
		},
		{
			// examples/lifecycle/main.go (primary window)
			Name:      "lifecycle-blue",
			Width:     800,
			Height:    600,
			Threshold: 0.0, // solid color — must be pixel-perfect
			Draw: func(dc *Context) {
				dc.Clear(0.15, 0.25, 0.65, 1.0)
			},
		},
		{
			// examples/lifecycle/main.go (secondary window)
			Name:      "lifecycle-red",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *Context) {
				dc.Clear(0.65, 0.15, 0.15, 1.0)
			},
		},
		{
			// examples/gpu_timing/main.go — draw is just Clear; timing logic is OnUpdate
			Name:      "gpu-timing",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *Context) {
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
			Draw: func(dc *Context) {
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
			Draw: func(dc *Context) {
				dc.Clear(0.2, 0.3, 0.8, 1.0)
			},
		},
		{
			// examples/multiwindow/main.go — secondary window (400×300)
			Name:      "multiwindow-secondary",
			Width:     400,
			Height:    300,
			Threshold: 0.0,
			Draw: func(dc *Context) {
				dc.Clear(0.8, 0.2, 0.3, 1.0)
			},
		},
		{
			// examples/tabbing/main.go
			Name:      "tabbing",
			Width:     800,
			Height:    600,
			Threshold: 0.0,
			Draw: func(dc *Context) {
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
			Draw: func(dc *Context) {
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
			Draw: func(dc *Context) {
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
			Draw: func(dc *Context) {
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
				_ = dc.DrawTextureEx(checkerTex, DrawTextureOptions{X: 300, Y: 50, Alpha: 0.5})
				_ = dc.DrawTexture(gradientTex, 50, 200)
				_ = dc.DrawTextureScaled(gradientTex, 200, 200, 256, 256)
			},
		},
	}
}

// TestGolden_Examples renders each example scene and either writes or compares
// the output PNG in testdata/golden/examples/.
func TestGolden_Examples(t *testing.T) {
	// In comparison mode, skip early (before any GPU init) if no reference PNGs
	// have been committed yet. Golden files are generated on Windows; on other
	// platforms the directory is empty until that step is done, and attempting
	// headless GPU init crashes on some backends (e.g. Metal on macOS CI).
	if !*updateGolden && !goldenDirHasPNGs() {
		t.Skipf("no golden PNGs in %s — generate them first with -update-golden on a reference machine", goldenExamplesDir())
	}

	r := newHeadlessRenderer(t)
	defer r.Destroy()

	for _, scene := range goldenScenes() {
		scene := scene
		t.Run(scene.Name, func(t *testing.T) {
			img, err := r.RenderToImage(scene.Width, scene.Height, scene.Draw)
			if err != nil {
				t.Fatalf("RenderToImage: %v", err)
			}

			if *updateGolden {
				writeGoldenPNG(t, scene.Name, img)
				return
			}

			golden := readGoldenPNG(t, scene.Name)
			if golden == nil {
				return
			}

			diffPct, diffCount := compareRGBAImages(img, golden)
			t.Logf("diff: %d pixels (%.3f%%) threshold %.1f%%", diffCount, diffPct, scene.Threshold)

			if diffPct > scene.Threshold {
				saveDebugImages(t, scene.Name, img, golden)
				t.Errorf("%.3f%% pixel diff exceeds threshold %.1f%%", diffPct, scene.Threshold)
			}
		})
	}
}

// --- helpers ---

func writeGoldenPNG(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	dir := goldenExamplesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := goldenExamplePath(name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create golden %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode golden %s: %v", path, err)
	}
	t.Logf("wrote golden: %s", path)
}

func readGoldenPNG(t *testing.T, name string) *image.RGBA {
	t.Helper()
	path := goldenExamplePath(name)
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("golden not found: %s — run with -args -update-golden on Windows to generate", path)
		return nil
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}

	// Normalise to *image.RGBA (the same type returned by RenderToImage).
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r32, g32, b32, a32 := src.At(x, y).RGBA()
			out.SetRGBA(x, y, color.RGBA{
				R: uint8(r32 >> 8),
				G: uint8(g32 >> 8),
				B: uint8(b32 >> 8),
				A: uint8(a32 >> 8),
			})
		}
	}
	return out
}

// compareRGBAImages returns the percentage and count of pixels that differ
// by more than 1 in any channel (to allow for rounding across backends).
func compareRGBAImages(a, b *image.RGBA) (diffPct float64, diffCount int) {
	ab, bb := a.Bounds(), b.Bounds()
	w := ab.Dx()
	if bb.Dx() < w {
		w = bb.Dx()
	}
	h := ab.Dy()
	if bb.Dy() < h {
		h = bb.Dy()
	}
	total := w * h

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac := a.RGBAAt(x+ab.Min.X, y+ab.Min.Y)
			bc := b.RGBAAt(x+bb.Min.X, y+bb.Min.Y)
			if absDiffU8(ac.R, bc.R) > 1 ||
				absDiffU8(ac.G, bc.G) > 1 ||
				absDiffU8(ac.B, bc.B) > 1 ||
				absDiffU8(ac.A, bc.A) > 1 {
				diffCount++
			}
		}
	}
	if total > 0 {
		diffPct = float64(diffCount) / float64(total) * 100
	}
	return
}

func absDiffU8(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// saveDebugImages writes the rendered image and a diff map to tmp/ for
// visual inspection when a golden comparison fails.
func saveDebugImages(t *testing.T, name string, got, want *image.RGBA) {
	t.Helper()
	dir := "tmp"
	_ = os.MkdirAll(dir, 0o755)

	savePNG(t, filepath.Join(dir, fmt.Sprintf("golden_got_%s.png", name)), got)

	// Diff map: red = mismatch, green = match.
	b := got.Bounds()
	diff := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gc := got.RGBAAt(x, y)
			wc := want.RGBAAt(x, y)
			d := math.Max(
				math.Max(float64(absDiffU8(gc.R, wc.R)), float64(absDiffU8(gc.G, wc.G))),
				math.Max(float64(absDiffU8(gc.B, wc.B)), float64(absDiffU8(gc.A, wc.A))),
			)
			if d > 1 {
				mag := uint8(d)
				if mag < 32 {
					mag = 32
				}
				diff.SetRGBA(x, y, color.RGBA{R: mag, G: 0, B: 0, A: 255})
			} else {
				gray := uint8((uint32(gc.R) + uint32(gc.G) + uint32(gc.B)) / 3)
				diff.SetRGBA(x, y, color.RGBA{R: 0, G: gray / 2, B: 0, A: 255})
			}
		}
	}
	savePNG(t, filepath.Join(dir, fmt.Sprintf("golden_diff_%s.png", name)), diff)
}

func savePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Logf("warning: cannot save %s: %v", path, err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Logf("warning: cannot encode %s: %v", path, err)
		return
	}
	t.Logf("saved: %s", path)
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
