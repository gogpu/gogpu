package gogpu_test

// A CPU reference for the textured golden scenes.
//
// The goldens in testdata/golden/examples/ are produced by the software
// backend itself, so on their own they can only catch changes, not errors.
// Up to wgpu v0.34.6 that backend turned these draws into a plain
// full-surface copy, and the texture and hidpi-lowres goldens recorded the
// wrong frames. This test renders the same two scenes on the CPU with WebGPU
// semantics and checks the committed goldens against it:
//
//   - a fragment is shaded when its pixel center lies inside the quad;
//   - the texture is sampled at the pixel center, bilinear, clamp-to-edge,
//     as gogpu's default sampler;
//   - the quad's alpha multiplies the texel alpha, and the result is blended
//     with premultiplied source-over;
//   - the frame is rounded to 8 bits per channel.
//
// The software backend computes in float32 and this test in float64, so a
// channel may differ by 1.

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// refTexture is a texture with straight-alpha texels in [0, 1].
type refTexture struct {
	w, h int
	px   [][4]float64
}

func newRefTexture(img image.Image) refTexture {
	b := img.Bounds()
	t := refTexture{w: b.Dx(), h: b.Dy(), px: make([][4]float64, b.Dx()*b.Dy())}
	for y := 0; y < t.h; y++ {
		for x := 0; x < t.w; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			t.px[y*t.w+x] = [4]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255, float64(c.A) / 255}
		}
	}
	return t
}

func (t refTexture) texel(x, y int) [4]float64 {
	x = min(max(x, 0), t.w-1)
	y = min(max(y, 0), t.h-1)
	return t.px[y*t.w+x]
}

// sample is a bilinear, clamp-to-edge lookup at normalized coordinates.
func (t refTexture) sample(u, v float64) [4]float64 {
	fx, fy := u*float64(t.w)-0.5, v*float64(t.h)-0.5
	x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
	ax, ay := fx-float64(x0), fy-float64(y0)
	var out [4]float64
	for c := range out {
		top := t.texel(x0, y0)[c]*(1-ax) + t.texel(x0+1, y0)[c]*ax
		bottom := t.texel(x0, y0+1)[c]*(1-ax) + t.texel(x0+1, y0+1)[c]*ax
		out[c] = top*(1-ay) + bottom*ay
	}
	return out
}

// refFrame holds premultiplied color in [0, 1].
type refFrame struct {
	w, h int
	px   [][4]float64
}

func newRefFrame(w, h int, r, g, b float64) refFrame {
	f := refFrame{w: w, h: h, px: make([][4]float64, w*h)}
	for i := range f.px {
		f.px[i] = [4]float64{r, g, b, 1}
	}
	return f
}

// drawQuad draws t into the rectangle (x, y, w, h) at the given opacity.
func (f refFrame) drawQuad(t refTexture, x, y, w, h, alpha float64) {
	for py := 0; py < f.h; py++ {
		for px := 0; px < f.w; px++ {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			if cx < x || cx >= x+w || cy < y || cy >= y+h {
				continue
			}
			s := t.sample((cx-x)/w, (cy-y)/h)
			a := s[3] * alpha
			src := [4]float64{s[0] * a, s[1] * a, s[2] * a, a}
			dst := &f.px[py*f.w+px]
			for c := range dst {
				dst[c] = src[c] + dst[c]*(1-a)
			}
		}
	}
}

// compareWithGolden fails the test if any color channel of the committed
// golden differs from the reference by more than one 8-bit step.
func (f refFrame) compareWithGolden(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", "examples", name+".png")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer file.Close()
	got, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}
	if b := got.Bounds(); b.Dx() != f.w || b.Dy() != f.h {
		t.Fatalf("golden %s is %dx%d, reference is %dx%d", path, b.Dx(), b.Dy(), f.w, f.h)
	}
	const tolerance = 1
	bad := 0
	for y := 0; y < f.h; y++ {
		for x := 0; x < f.w; x++ {
			g := color.RGBAModel.Convert(got.At(x, y)).(color.RGBA)
			have := [3]int{int(g.R), int(g.G), int(g.B)}
			for c := range have {
				want := int(math.Round(f.px[y*f.w+x][c] * 255))
				if d := have[c] - want; d > tolerance || d < -tolerance {
					if bad < 5 {
						t.Errorf("%s (%d,%d) channel %d = %d, reference %d", name, x, y, c, have[c], want)
					}
					bad++
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("%s: %d channels differ from the CPU reference by more than %d", name, bad, tolerance)
	}
}

func TestGoldenReference_Texture(t *testing.T) {
	checker := image.NewRGBA(image.Rect(0, 0, 64, 64))
	copy(checker.Pix, goldenCreateCheckerboard(64, 64, 8))
	checkerTex := newRefTexture(checker)
	gradientTex := newRefTexture(goldenCreateGradientImage(128, 128))

	bg := float64(0x2D) / 255
	f := newRefFrame(800, 600, bg, bg, bg)
	f.drawQuad(checkerTex, 50, 50, 64, 64, 1)      // DrawTexture
	f.drawQuad(checkerTex, 150, 50, 128, 128, 1)   // DrawTextureScaled
	f.drawQuad(checkerTex, 300, 50, 64, 64, 0.5)   // DrawTextureEx, Alpha 0.5
	f.drawQuad(gradientTex, 50, 200, 128, 128, 1)  // DrawTexture
	f.drawQuad(gradientTex, 200, 200, 256, 256, 1) // DrawTextureScaled
	f.compareWithGolden(t, "texture")
}

func TestGoldenReference_HiDPILowRes(t *testing.T) {
	f := newRefFrame(800, 600, 0.06, 0.06, 0.08)
	f.drawQuad(newRefTexture(goldenBuildCheckerImage(400, 300, 4)), 0, 0, 800, 600, 1)
	f.compareWithGolden(t, "hidpi-lowres")
}
