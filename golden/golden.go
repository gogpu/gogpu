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
	"io"
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

type reporter interface {
	Helper()
	Logf(string, ...any)
	Errorf(string, ...any)
	Fatalf(string, ...any)
}

type imageRenderer interface {
	RenderToImage(int, int, func(*gogpu.Context)) (*image.RGBA, error)
	Destroy()
	ReleaseInstance()
}

type compareRuntime struct {
	newRenderer   func() (imageRenderer, error)
	writePNG      func(string, image.Image) error
	readPNG       func(string) (*image.RGBA, error)
	saveArtifacts func(reporter, string, string, *image.RGBA, *image.RGBA)
}

var defaultRuntime = compareRuntime{
	newRenderer:   newHeadlessRenderer,
	writePNG:      writePNG,
	readPNG:       readPNG,
	saveArtifacts: saveArtifacts,
}

// Compare renders one scene with the deterministic software renderer and
// compares it with testdata/golden/<name>.png. Missing references and renderer
// failures fail the test; they are never silently skipped. Pass -update-goldens
// (or set Options.Update) to regenerate a reference.
func Compare(t testing.TB, name string, width, height int, draw func(*gogpu.Context)) {
	t.Helper()
	CompareWithOptions(t, name, width, height, draw, Options{})
}

// CompareWithOptions is Compare with an explicit reference directory,
// threshold, and update mode.
func CompareWithOptions(t testing.TB, name string, width, height int, draw func(*gogpu.Context), options Options) {
	t.Helper()
	compareWithRuntime(t, name, width, height, draw, options, defaultRuntime)
}

func compareWithRuntime(t reporter, name string, width, height int, draw func(*gogpu.Context), options Options, runtime compareRuntime) {
	t.Helper()
	if err := validate(name, width, height, draw, options); err != nil {
		t.Fatalf("golden: %v", err)
		return
	}
	if options.GoldenDir == "" {
		options.GoldenDir = filepath.Join("testdata", "golden")
	}
	path := filepath.Join(options.GoldenDir, name+".png")

	renderer, err := runtime.newRenderer()
	if err != nil {
		t.Fatalf("golden %q: create headless renderer: %v", name, err)
		return
	}
	defer func() {
		renderer.Destroy()
		renderer.ReleaseInstance()
	}()

	got, err := renderer.RenderToImage(width, height, draw)
	if err != nil {
		t.Fatalf("golden %q: render: %v", name, err)
		return
	}

	if shouldUpdate(options) {
		if err := runtime.writePNG(path, got); err != nil {
			t.Fatalf("golden %q: update %s: %v", name, path, err)
			return
		}
		t.Logf("golden updated: %s", path)
		return
	}

	want, err := runtime.readPNG(path)
	if err != nil {
		t.Fatalf("golden %q: read %s: %v (run with -update-goldens to create it)", name, path, err)
		return
	}
	if !sameBounds(got, want) {
		runtime.saveArtifacts(t, options.GoldenDir, name, got, want)
		t.Fatalf("golden %q: dimensions differ: got %s, want %s", name, got.Bounds(), want.Bounds())
		return
	}

	diffPct, diffCount := compareRGBA(got, want)
	t.Logf("golden %q: diff %d pixels (%.3f%%), threshold %.3f%%", name, diffCount, diffPct, options.Threshold)
	if diffPct > options.Threshold {
		runtime.saveArtifacts(t, options.GoldenDir, name, got, want)
		t.Errorf("golden %q: %.3f%% pixel diff exceeds threshold %.3f%%", name, diffPct, options.Threshold)
	}
}

func newHeadlessRenderer() (imageRenderer, error) {
	return gogpu.NewHeadlessRenderer()
}

func shouldUpdate(options Options) bool {
	if options.Update {
		return true
	}
	if *updateGolden {
		return true
	}
	return *updateGoldens
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

type temporaryFile interface {
	io.Writer
	Close() error
	Name() string
}

type pngWriterRuntime struct {
	mkdirAll   func(string, os.FileMode) error
	encode     func(io.Writer, image.Image) error
	createTemp func(string, string) (temporaryFile, error)
	rename     func(string, string) error
	remove     func(string) error
}

var defaultPNGWriterRuntime = pngWriterRuntime{
	mkdirAll: os.MkdirAll,
	encode:   png.Encode,
	createTemp: func(dir, pattern string) (temporaryFile, error) {
		return os.CreateTemp(dir, pattern)
	},
	rename: os.Rename,
	remove: os.Remove,
}

func writePNG(path string, img image.Image) error {
	return writePNGWithRuntime(path, img, defaultPNGWriterRuntime)
}

func writePNGWithRuntime(path string, img image.Image, runtime pngWriterRuntime) error {
	if err := runtime.mkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := runtime.encode(&buf, img); err != nil {
		return err
	}
	tmp, err := runtime.createTemp(filepath.Dir(path), ".golden-*.png")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		return errors.Join(err, tmp.Close(), runtime.remove(tmpName))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(err, runtime.remove(tmpName))
	}
	if err := runtime.rename(tmpName, path); err != nil {
		return errors.Join(err, runtime.remove(tmpName))
	}
	return nil
}

type pngReaderRuntime struct {
	open   func(string) (io.ReadCloser, error)
	decode func(io.Reader) (image.Image, error)
}

var defaultPNGReaderRuntime = pngReaderRuntime{
	open: func(path string) (io.ReadCloser, error) {
		return os.Open(path) //nolint:gosec // G304: caller selects the local fixture directory; scene names are validated.
	},
	decode: png.Decode,
}

func readPNG(path string) (*image.RGBA, error) {
	return readPNGWithRuntime(path, defaultPNGReaderRuntime)
}

func readPNGWithRuntime(path string, runtime pngReaderRuntime) (*image.RGBA, error) {
	f, err := runtime.open(path)
	if err != nil {
		return nil, err
	}
	src, decodeErr := runtime.decode(f)
	closeErr := f.Close()
	if err := errors.Join(decodeErr, closeErr); err != nil {
		return nil, err
	}
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, blue, a := src.At(x, y).RGBA()
			out.SetRGBA(x, y, color.RGBA{R: rgba8(r), G: rgba8(g), B: rgba8(blue), A: rgba8(a)})
		}
	}
	return out, nil
}

func rgba8(value uint32) uint8 {
	return uint8(value >> 8) //nolint:gosec // G115: color.Color.RGBA returns 16-bit channel values.
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

type artifactRuntime struct {
	mkdirAll func(string, os.FileMode) error
	writePNG func(string, image.Image) error
}

var defaultArtifactRuntime = artifactRuntime{mkdirAll: os.MkdirAll, writePNG: writePNG}

func saveArtifacts(t reporter, goldenDir, name string, got, want *image.RGBA) {
	saveArtifactsWithRuntime(t, goldenDir, name, got, want, defaultArtifactRuntime)
}

func saveArtifactsWithRuntime(t reporter, goldenDir, name string, got, want *image.RGBA, runtime artifactRuntime) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(goldenDir), "tmp")
	if err := runtime.mkdirAll(dir, 0o750); err != nil {
		t.Logf("golden %q: create artifact dir: %v", name, err)
		return
	}
	gotPath := filepath.Join(dir, "golden_got_"+name+".png")
	diffPath := filepath.Join(dir, "golden_diff_"+name+".png")
	if err := runtime.writePNG(gotPath, got); err != nil {
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
			d := maxChannelDiff(gc, wc)
			if d > 1 {
				mag := d
				if mag < 32 {
					mag = 32
				}
				diff.SetRGBA(x, y, color.RGBA{R: mag, A: 255})
			} else {
				diff.SetRGBA(x, y, color.RGBA{G: averageRGB(gc) / 2, A: 255})
			}
		}
	}
	if err := runtime.writePNG(diffPath, diff); err != nil {
		t.Logf("golden %q: save diff: %v", name, err)
	} else {
		t.Logf("golden %q: diff image: %s", name, diffPath)
	}
}

func maxChannelDiff(a, b color.RGBA) uint8 {
	return max(max(absDiff(a.R, b.R), absDiff(a.G, b.G)), max(absDiff(a.B, b.B), absDiff(a.A, b.A)))
}

func averageRGB(c color.RGBA) uint8 {
	return uint8((uint16(c.R) + uint16(c.G) + uint16(c.B)) / 3) //nolint:gosec // G115: three uint8 channels average to at most 255.
}
