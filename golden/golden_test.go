package golden

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/gogpu"
)

var errTest = errors.New("test error")

type recordingReporter struct {
	logs   []string
	errors []string
	fatals []string
}

func (r *recordingReporter) Helper() {}

func (r *recordingReporter) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}

func (r *recordingReporter) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recordingReporter) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
}

type stubRenderer struct {
	img       *image.RGBA
	err       error
	destroyed bool
	released  bool
}

func (r *stubRenderer) RenderToImage(_ int, _ int, _ func(*gogpu.Context)) (*image.RGBA, error) {
	return r.img, r.err
}

func (r *stubRenderer) Destroy() { r.destroyed = true }

func (r *stubRenderer) ReleaseInstance() { r.released = true }

func TestCompareUsesDeterministicRenderer(t *testing.T) {
	t.Chdir(t.TempDir())

	oldUpdateGolden, oldUpdateGoldens := *updateGolden, *updateGoldens
	*updateGolden, *updateGoldens = true, false
	t.Cleanup(func() { *updateGolden, *updateGoldens = oldUpdateGolden, oldUpdateGoldens })

	Compare(t, "actual", 1, 1, func(ctx *gogpu.Context) {
		ctx.Clear(7.0/255, 11.0/255, 13.0/255, 1)
	})

	goldenDir := filepath.Join("testdata", "golden")
	img, err := readPNG(filepath.Join(goldenDir, "actual.png"))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(0, 0); got != (color.RGBA{R: 7, G: 11, B: 13, A: 255}) {
		t.Fatalf("rendered pixel = %#v", got)
	}
}

func TestCompareWithRuntimeOutcomes(t *testing.T) {
	draw := func(*gogpu.Context) {}
	baseImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	makeRuntime := func(renderer *stubRenderer) compareRuntime {
		return compareRuntime{
			newRenderer:   func() (imageRenderer, error) { return renderer, nil },
			writePNG:      func(string, image.Image) error { return nil },
			readPNG:       func(string) (*image.RGBA, error) { return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil },
			saveArtifacts: func(reporter, string, string, *image.RGBA, *image.RGBA) {},
		}
	}

	t.Run("validation", func(t *testing.T) {
		report := &recordingReporter{}
		compareWithRuntime(report, "", 1, 1, draw, Options{}, compareRuntime{})
		assertReportCount(t, report, 1, 0, 0)
	})

	t.Run("renderer creation", func(t *testing.T) {
		report := &recordingReporter{}
		runtime := makeRuntime(nil)
		runtime.newRenderer = func() (imageRenderer, error) { return nil, errTest }
		compareWithRuntime(report, "scene", 1, 1, draw, Options{}, runtime)
		assertReportCount(t, report, 1, 0, 0)
	})

	t.Run("render", func(t *testing.T) {
		report := &recordingReporter{}
		renderer := &stubRenderer{err: errTest}
		compareWithRuntime(report, "scene", 1, 1, draw, Options{}, makeRuntime(renderer))
		assertReportCount(t, report, 1, 0, 0)
		assertCleanedUp(t, renderer)
	})

	t.Run("update failure", func(t *testing.T) {
		report := &recordingReporter{}
		renderer := &stubRenderer{img: baseImage}
		runtime := makeRuntime(renderer)
		runtime.writePNG = func(string, image.Image) error { return errTest }
		compareWithRuntime(report, "scene", 1, 1, draw, Options{Update: true}, runtime)
		assertReportCount(t, report, 1, 0, 0)
		assertCleanedUp(t, renderer)
	})

	t.Run("update success", func(t *testing.T) {
		report := &recordingReporter{}
		renderer := &stubRenderer{img: baseImage}
		compareWithRuntime(report, "scene", 1, 1, draw, Options{Update: true}, makeRuntime(renderer))
		assertReportCount(t, report, 0, 0, 1)
		assertCleanedUp(t, renderer)
	})

	t.Run("read failure", func(t *testing.T) {
		report := &recordingReporter{}
		renderer := &stubRenderer{img: baseImage}
		runtime := makeRuntime(renderer)
		runtime.readPNG = func(string) (*image.RGBA, error) { return nil, errTest }
		compareWithRuntime(report, "scene", 1, 1, draw, Options{}, runtime)
		assertReportCount(t, report, 1, 0, 0)
		assertCleanedUp(t, renderer)
	})

	t.Run("dimensions", func(t *testing.T) {
		report := &recordingReporter{}
		renderer := &stubRenderer{img: baseImage}
		runtime := makeRuntime(renderer)
		runtime.readPNG = func(string) (*image.RGBA, error) { return image.NewRGBA(image.Rect(0, 0, 2, 1)), nil }
		saved := false
		runtime.saveArtifacts = func(reporter, string, string, *image.RGBA, *image.RGBA) { saved = true }
		compareWithRuntime(report, "scene", 1, 1, draw, Options{}, runtime)
		assertReportCount(t, report, 1, 0, 0)
		if !saved {
			t.Fatal("dimension mismatch did not save artifacts")
		}
		assertCleanedUp(t, renderer)
	})

	t.Run("match", func(t *testing.T) {
		report := &recordingReporter{}
		renderer := &stubRenderer{img: baseImage}
		compareWithRuntime(report, "scene", 1, 1, draw, Options{}, makeRuntime(renderer))
		assertReportCount(t, report, 0, 0, 1)
		assertCleanedUp(t, renderer)
	})

	t.Run("threshold exceeded", func(t *testing.T) {
		report := &recordingReporter{}
		got := image.NewRGBA(image.Rect(0, 0, 1, 1))
		got.SetRGBA(0, 0, color.RGBA{R: 3})
		renderer := &stubRenderer{img: got}
		runtime := makeRuntime(renderer)
		saved := false
		runtime.saveArtifacts = func(reporter, string, string, *image.RGBA, *image.RGBA) { saved = true }
		compareWithRuntime(report, "scene", 1, 1, draw, Options{}, runtime)
		assertReportCount(t, report, 0, 1, 1)
		if !saved {
			t.Fatal("pixel mismatch did not save artifacts")
		}
		assertCleanedUp(t, renderer)
	})
}

func assertReportCount(t *testing.T, report *recordingReporter, fatals, errs, logs int) {
	t.Helper()
	if len(report.fatals) != fatals || len(report.errors) != errs || len(report.logs) != logs {
		t.Fatalf("report counts = fatal %d, error %d, log %d; want %d, %d, %d", len(report.fatals), len(report.errors), len(report.logs), fatals, errs, logs)
	}
}

func assertCleanedUp(t *testing.T, renderer *stubRenderer) {
	t.Helper()
	if !renderer.destroyed || !renderer.released {
		t.Fatalf("renderer cleanup = destroyed %v, released %v", renderer.destroyed, renderer.released)
	}
}

func TestShouldUpdate(t *testing.T) {
	oldUpdateGolden, oldUpdateGoldens := *updateGolden, *updateGoldens
	t.Cleanup(func() { *updateGolden, *updateGoldens = oldUpdateGolden, oldUpdateGoldens })

	*updateGolden, *updateGoldens = false, false
	if shouldUpdate(Options{}) {
		t.Fatal("updates unexpectedly enabled")
	}
	if !shouldUpdate(Options{Update: true}) {
		t.Fatal("Options.Update did not enable updates")
	}
	*updateGolden = true
	if !shouldUpdate(Options{}) {
		t.Fatal("-update-golden did not enable updates")
	}
	*updateGolden, *updateGoldens = false, true
	if !shouldUpdate(Options{}) {
		t.Fatal("-update-goldens did not enable updates")
	}
}

func TestValidate(t *testing.T) {
	draw := func(*gogpu.Context) {}
	tests := []struct {
		name    string
		width   int
		height  int
		draw    func(*gogpu.Context)
		options Options
		valid   bool
	}{
		{name: "ok", width: 1, height: 1, draw: draw, valid: true},
		{name: "", width: 1, height: 1, draw: draw},
		{name: "../escape", width: 1, height: 1, draw: draw},
		{name: "trailing.", width: 1, height: 1, draw: draw},
		{name: "size", width: 0, height: -1, draw: draw},
		{name: "nil", width: 1, height: 1},
		{name: "negative", width: 1, height: 1, draw: draw, options: Options{Threshold: -1}},
		{name: "large", width: 1, height: 1, draw: draw, options: Options{Threshold: 101}},
		{name: "nan", width: 1, height: 1, draw: draw, options: Options{Threshold: math.NaN()}},
	}
	for _, test := range tests {
		err := validate(test.name, test.width, test.height, test.draw, test.options)
		if (err == nil) != test.valid {
			t.Errorf("validate(%q) error=%v, want valid=%v", test.name, err, test.valid)
		}
	}
}

type stubTemporaryFile struct {
	bytes.Buffer
	name     string
	writeErr error
	closeErr error
}

func (f *stubTemporaryFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.Buffer.Write(p)
}

func (f *stubTemporaryFile) Close() error { return f.closeErr }

func (f *stubTemporaryFile) Name() string { return f.name }

func TestWritePNGWithRuntime(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	makeRuntime := func(file *stubTemporaryFile) pngWriterRuntime {
		return pngWriterRuntime{
			mkdirAll: func(string, os.FileMode) error { return nil },
			encode: func(w io.Writer, _ image.Image) error {
				_, err := w.Write([]byte("png"))
				return err
			},
			createTemp: func(string, string) (temporaryFile, error) { return file, nil },
			rename:     func(string, string) error { return nil },
			remove:     func(string) error { return nil },
		}
	}

	tests := []struct {
		name   string
		mutate func(*pngWriterRuntime, *stubTemporaryFile)
	}{
		{name: "success"},
		{name: "mkdir", mutate: func(r *pngWriterRuntime, _ *stubTemporaryFile) {
			r.mkdirAll = func(string, os.FileMode) error { return errTest }
		}},
		{name: "encode", mutate: func(r *pngWriterRuntime, _ *stubTemporaryFile) {
			r.encode = func(io.Writer, image.Image) error { return errTest }
		}},
		{name: "create", mutate: func(r *pngWriterRuntime, _ *stubTemporaryFile) {
			r.createTemp = func(string, string) (temporaryFile, error) { return nil, errTest }
		}},
		{name: "write", mutate: func(r *pngWriterRuntime, f *stubTemporaryFile) {
			f.writeErr = errTest
			f.closeErr = errors.New("close")
			r.remove = func(string) error { return errors.New("remove") }
		}},
		{name: "close", mutate: func(r *pngWriterRuntime, f *stubTemporaryFile) {
			f.closeErr = errTest
			r.remove = func(string) error { return errors.New("remove") }
		}},
		{name: "rename", mutate: func(r *pngWriterRuntime, _ *stubTemporaryFile) {
			r.rename = func(string, string) error { return errTest }
			r.remove = func(string) error { return errors.New("remove") }
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := &stubTemporaryFile{name: "temporary"}
			runtime := makeRuntime(file)
			var gotMode os.FileMode
			runtime.mkdirAll = func(_ string, mode os.FileMode) error {
				gotMode = mode
				return nil
			}
			if test.mutate != nil {
				test.mutate(&runtime, file)
			}
			err := writePNGWithRuntime("dir/image.png", img, runtime)
			if test.name == "success" && err != nil {
				t.Fatal(err)
			}
			if test.name == "success" && gotMode != 0o750 {
				t.Fatalf("golden directory mode = %#o, want 0750", gotMode)
			}
			if test.name != "success" && !errors.Is(err, errTest) {
				t.Fatalf("error = %v, want test error", err)
			}
		})
	}
}

type stubReadCloser struct {
	io.Reader
	closeErr error
}

func (r stubReadCloser) Close() error { return r.closeErr }

func TestReadPNGWithRuntime(t *testing.T) {
	if _, err := readPNGWithRuntime("missing", pngReaderRuntime{open: func(string) (io.ReadCloser, error) { return nil, errTest }}); !errors.Is(err, errTest) {
		t.Fatalf("open error = %v", err)
	}

	runtime := pngReaderRuntime{
		open: func(string) (io.ReadCloser, error) {
			return stubReadCloser{Reader: strings.NewReader(""), closeErr: errors.New("close")}, nil
		},
		decode: func(io.Reader) (image.Image, error) { return nil, errTest },
	}
	if _, err := readPNGWithRuntime("bad", runtime); !errors.Is(err, errTest) || !strings.Contains(err.Error(), "close") {
		t.Fatalf("decode/close error = %v", err)
	}

	src := image.NewNRGBA(image.Rect(-1, -1, 1, 1))
	src.SetNRGBA(-1, -1, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	runtime = pngReaderRuntime{
		open:   func(string) (io.ReadCloser, error) { return stubReadCloser{Reader: bytes.NewReader(nil)}, nil },
		decode: func(io.Reader) (image.Image, error) { return src, nil },
	}
	got, err := readPNGWithRuntime("ok", runtime)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != src.Bounds() || got.RGBAAt(-1, -1) != (color.RGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatalf("decoded image = %v, %#v", got.Bounds(), got.RGBAAt(-1, -1))
	}
	if rgba8(0xffff) != 0xff || rgba8(0) != 0 {
		t.Fatal("rgba8 did not preserve 16-bit channel endpoints")
	}
}

func TestCompareRGBA(t *testing.T) {
	if !sameBounds(image.NewRGBA(image.Rect(0, 0, 1, 1)), image.NewRGBA(image.Rect(0, 0, 1, 1))) {
		t.Fatal("equal bounds reported unequal")
	}
	if sameBounds(image.NewRGBA(image.Rect(0, 0, 1, 1)), image.NewRGBA(image.Rect(0, 0, 2, 1))) {
		t.Fatal("unequal bounds reported equal")
	}
	if pct, count := compareRGBA(image.NewRGBA(image.Rectangle{}), image.NewRGBA(image.Rectangle{})); pct != 0 || count != 0 {
		t.Fatalf("empty comparison = %v, %d", pct, count)
	}

	got := image.NewRGBA(image.Rect(0, 0, 5, 1))
	want := image.NewRGBA(image.Rect(0, 0, 5, 1))
	got.SetRGBA(0, 0, color.RGBA{R: 1})
	got.SetRGBA(1, 0, color.RGBA{G: 3})
	got.SetRGBA(2, 0, color.RGBA{B: 3})
	got.SetRGBA(3, 0, color.RGBA{A: 3})
	got.SetRGBA(4, 0, color.RGBA{R: 2})
	want.SetRGBA(4, 0, color.RGBA{R: 4})
	pct, count := compareRGBA(got, want)
	if pct != 80 || count != 4 {
		t.Fatalf("compareRGBA() = %.1f%%, %d; want 80%%, 4", pct, count)
	}
	if absDiff(5, 2) != 3 || absDiff(2, 5) != 3 {
		t.Fatal("absDiff is not symmetric")
	}
}

func TestSaveArtifacts(t *testing.T) {
	report := &recordingReporter{}
	dir := filepath.Join(t.TempDir(), "golden")
	got := image.NewRGBA(image.Rect(0, 0, 1, 1))
	want := image.NewRGBA(image.Rect(0, 0, 1, 1))
	saveArtifacts(report, dir, "actual", got, want)
	if len(report.logs) != 2 {
		t.Fatalf("default artifact logs = %v", report.logs)
	}

	report = &recordingReporter{}
	saveArtifactsWithRuntime(report, dir, "mkdir", got, want, artifactRuntime{
		mkdirAll: func(string, os.FileMode) error { return errTest },
	})
	if len(report.logs) != 1 {
		t.Fatalf("mkdir artifact logs = %v", report.logs)
	}

	got = image.NewRGBA(image.Rect(0, 0, 3, 1))
	want = image.NewRGBA(image.Rect(0, 0, 2, 1))
	got.SetRGBA(0, 0, color.RGBA{R: 3, G: 6, B: 9, A: 255})
	want.SetRGBA(0, 0, color.RGBA{R: 3, G: 6, B: 9, A: 255})
	got.SetRGBA(1, 0, color.RGBA{R: 2, A: 255})
	want.SetRGBA(1, 0, color.RGBA{A: 255})
	got.SetRGBA(2, 0, color.RGBA{R: 100, A: 255})
	written := make([]*image.RGBA, 0, 2)
	calls := 0
	report = &recordingReporter{}
	saveArtifactsWithRuntime(report, dir, "mixed", got, want, artifactRuntime{
		mkdirAll: func(string, os.FileMode) error { return nil },
		writePNG: func(_ string, img image.Image) error {
			calls++
			written = append(written, img.(*image.RGBA))
			if calls == 1 {
				return errTest
			}
			return errors.New("diff")
		},
	})
	if len(report.logs) != 2 || len(written) != 2 {
		t.Fatalf("artifact failures = logs %v, writes %d", report.logs, len(written))
	}
	diff := written[1]
	if diff.RGBAAt(0, 0) != (color.RGBA{G: 3, A: 255}) {
		t.Fatalf("unchanged diff pixel = %#v", diff.RGBAAt(0, 0))
	}
	if diff.RGBAAt(1, 0) != (color.RGBA{R: 32, A: 255}) {
		t.Fatalf("small diff pixel = %#v", diff.RGBAAt(1, 0))
	}
	if diff.RGBAAt(2, 0) != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("outside-bounds diff pixel = %#v", diff.RGBAAt(2, 0))
	}
	if maxChannelDiff(color.RGBA{G: 9}, color.RGBA{}) != 9 || averageRGB(color.RGBA{R: 255, G: 255, B: 255}) != 255 {
		t.Fatal("diff channel helpers returned unexpected values")
	}
}
