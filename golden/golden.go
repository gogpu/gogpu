// Package golden provides deterministic headless image comparisons for gogpu
// rendering tests. It always uses the Pure-Go software backend so a comparison
// does not depend on a display server or a host GPU.
package golden

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/gogpu"
)

var updateGolden = flag.Bool("update-golden", false, "write golden PNG files instead of comparing")
var updateGoldens = flag.Bool("update-goldens", false, "write golden PNG files instead of comparing")

// Options controls a comparison. GoldenDir defaults to testdata/golden when
// empty. Threshold is the maximum percentage of pixels that may differ; each
// channel still permits a one-unit rounding difference.
type Options struct {
	GoldenDir string
	Threshold float64
	Update    bool
}

// Compare renders one scene with the deterministic software renderer and
// compares it with testdata/golden/<name>.png. Missing references and renderer
// failures fail the test; they are never silently skipped. Pass -update-goldens
// (or set Options.Update) to regenerate a reference.
func Compare(t testing.TB, name string, width, height int, draw func(*gogpu.Context)) {
	CompareWithOptions(t, name, width, height, draw, Options{})
}

// CompareWithOptions is Compare with an explicit reference directory,
// threshold, and update mode.
func CompareWithOptions(t testing.TB, name string, width, height int, draw func(*gogpu.Context), options Options) {
	t.Helper()

	if err := validate(name, width, height, draw, options); err != nil {
		t.Fatalf("golden: %v", err)
	}
	if options.GoldenDir == "" {
		options.GoldenDir = filepath.Join("testdata", "golden")
	}
	path := filepath.Join(options.GoldenDir, name+".png")

	renderer, err := gogpu.NewHeadlessRenderer()
	if err != nil {
		t.Fatalf("golden %q: create headless renderer: %v", name, err)
	}
	defer func() {
		renderer.Destroy()
		renderer.ReleaseInstance()
	}()

	got, err := renderer.RenderToImage(width, height, draw)
	if err != nil {
		t.Fatalf("golden %q: render: %v", name, err)
	}

	if options.Update || *updateGolden || *updateGoldens {
		if err := writePNG(path, got); err != nil {
			t.Fatalf("golden %q: update %s: %v", name, path, err)
		}
		t.Logf("golden updated: %s", path)
		return
	}

	want, err := readPNG(path)
	if err != nil {
		t.Fatalf("golden %q: read %s: %v (run with -update-goldens to create it)", name, path, err)
	}
	if !sameBounds(got, want) {
		saveArtifacts(t, options.GoldenDir, name, got, want)
		t.Fatalf("golden %q: dimensions differ: got %s, want %s", name, got.Bounds(), want.Bounds())
	}

	diffPct, diffCount := compareRGBA(got, want)
	t.Logf("golden %q: diff %d pixels (%.3f%%), threshold %.3f%%", name, diffCount, diffPct, options.Threshold)
	if diffPct > options.Threshold {
		saveArtifacts(t, options.GoldenDir, name, got, want)
		t.Errorf("golden %q: %.3f%% pixel diff exceeds threshold %.3f%%", name, diffPct, options.Threshold)
	}
}

func validate(name string, width, height int, draw func(*gogpu.Context), options Options) error {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) || strings.HasSuffix(name, ".") {
		return fmt.Errorf("invalid scene name %q", name)
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid size %dx%d", width, height)
	}
	if draw == nil {
		return errors.New("draw callback is nil")
	}
	if options.Threshold < 0 || options.Threshold > 100 || math.IsNaN(options.Threshold) {
		return fmt.Errorf("invalid threshold %.3f", options.Threshold)
	}
	return nil
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".golden-*.png")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func readPNG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			out.SetRGBA(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
		}
	}
	return out, nil
}

func sameBounds(a, b image.Image) bool { return a.Bounds() == b.Bounds() }

func compareRGBA(a, b *image.RGBA) (float64, int) {
	bounds := a.Bounds()
	total := bounds.Dx() * bounds.Dy()
	diffCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ac, bc := a.RGBAAt(x, y), b.RGBAAt(x, y)
			if absDiff(ac.R, bc.R) > 1 || absDiff(ac.G, bc.G) > 1 || absDiff(ac.B, bc.B) > 1 || absDiff(ac.A, bc.A) > 1 {
				diffCount++
			}
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(diffCount) / float64(total) * 100, diffCount
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func saveArtifacts(t testing.TB, goldenDir, name string, got, want *image.RGBA) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(goldenDir), "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("golden %q: create artifact dir: %v", name, err)
		return
	}
	gotPath := filepath.Join(dir, "golden_got_"+name+".png")
	diffPath := filepath.Join(dir, "golden_diff_"+name+".png")
	if err := writePNG(gotPath, got); err != nil {
		t.Logf("golden %q: save actual: %v", name, err)
	} else {
		t.Logf("golden %q: actual image: %s", name, gotPath)
	}
	diff := image.NewRGBA(got.Bounds())
	for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
		for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
			gc := got.RGBAAt(x, y)
			wc := color.RGBA{}
			if image.Pt(x, y).In(want.Bounds()) {
				wc = want.RGBAAt(x, y)
			}
			d := math.Max(math.Max(float64(absDiff(gc.R, wc.R)), float64(absDiff(gc.G, wc.G))), math.Max(float64(absDiff(gc.B, wc.B)), float64(absDiff(gc.A, wc.A))))
			if d > 1 {
				mag := uint8(d)
				if mag < 32 {
					mag = 32
				}
				diff.SetRGBA(x, y, color.RGBA{R: mag, A: 255})
			} else {
				gray := uint8((uint32(gc.R) + uint32(gc.G) + uint32(gc.B)) / 3)
				diff.SetRGBA(x, y, color.RGBA{G: gray / 2, A: 255})
			}
		}
	}
	if err := writePNG(diffPath, diff); err != nil {
		t.Logf("golden %q: save diff: %v", name, err)
	} else {
		t.Logf("golden %q: diff image: %s", name, diffPath)
	}
}
