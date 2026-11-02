package gogpu

import (
	"bytes"
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/software"
)

// newIdentityPresentTarget returns a software renderer whose primary target
// is a configured headless BGRA8 surface of w×h (gogpu's surface format), so
// a frame runs through the same BeginFrame/EndFrame path as a window.
func newIdentityPresentTarget(t *testing.T, w, h uint32) (*Renderer, *RenderTarget, *Context) {
	t.Helper()
	r, err := NewHeadlessRenderer()
	if err != nil {
		t.Fatalf("NewHeadlessRenderer: %v", err)
	}
	t.Cleanup(func() {
		r.Destroy()
		r.ReleaseInstance()
	})
	if !r.pixelPresent {
		t.Fatal("software adapter did not enable the pixel-present route")
	}
	surface, err := r.instance.CreateSurfaceFromTarget(wgpu.HeadlessSurfaceTarget{})
	if err != nil {
		t.Fatalf("CreateSurfaceFromTarget: %v", err)
	}
	ws := r.primary
	ws.surface = surface
	ws.width, ws.height = w, h
	ws.format = r.surfaceFormat
	if err := ws.configure(r.device, r.adapter); err != nil {
		t.Fatalf("configure: %v", err)
	}
	ws.state = SurfaceConfigured
	return r, ws, newContext(r, 1)
}

// newTestTexture uploads w×h RGBA pixels as a gogpu texture.
func newTestTexture(t *testing.T, r *Renderer, w, h int, pix []byte, premultiplied bool) *Texture {
	t.Helper()
	opts := DefaultTextureOptions()
	opts.Premultiplied = premultiplied
	tex, err := r.NewTextureFromRGBAWithOptions(w, h, pix, opts)
	if err != nil {
		t.Fatalf("NewTextureFromRGBAWithOptions: %v", err)
	}
	t.Cleanup(tex.Destroy)
	return tex
}

// patternRGBA returns w×h premultiplied RGBA pixels that differ per pixel and
// per channel, with a few translucent ones, so a swizzle, a row-padding or an
// offset mistake shows.
func patternRGBA(w, h int) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		a := byte(255)
		if i%3 == 1 {
			a = 128
		}
		pix[i*4+0] = byte(i*37) & a
		pix[i*4+1] = byte(i*59+11) & a
		pix[i*4+2] = byte(i*83+23) & a
		pix[i*4+3] = a
	}
	return pix
}

func readSurface(t *testing.T, ws *RenderTarget) []byte {
	t.Helper()
	pix, err := ws.surface.ReadPixels()
	if err != nil {
		t.Fatalf("ReadPixels: %v", err)
	}
	return pix
}

// TestPresentTextureIdentityPresentsPixels: a full-surface PresentTexture as
// the frame's only content reaches the surface byte-exact without the
// textured-quad pipeline. 3×2 makes the readback rows (12 bytes) padded to 256.
func TestPresentTextureIdentityPresentsPixels(t *testing.T) {
	for _, tc := range []struct {
		name  string
		clear bool
	}{
		{name: "no clear"},
		{name: "transparent clear", clear: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, ws, ctx := newIdentityPresentTarget(t, 3, 2)
			want := patternRGBA(3, 2)
			tex := newTestTexture(t, r, 3, 2, want, true)

			if tc.clear {
				ctx.Clear(0, 0, 0, 0)
			}
			if err := ctx.PresentTexture(tex); err != nil {
				t.Fatalf("PresentTexture: %v", err)
			}
			if ws.identityPresent != tex {
				t.Fatal("identity PresentTexture was not deferred")
			}
			r.endFrameForSurface(ws)

			if r.texQuadPipelineInited {
				t.Error("identity frame ran the textured-quad pipeline")
			}
			if got := readSurface(t, ws); !bytes.Equal(got, want) {
				t.Errorf("surface pixels\n got %v\nwant %v", got, want)
			}
		})
	}
}

// TestPresentTextureIdentityPremultipliesStraightAlpha: a straight-alpha
// texture is presented as the shader would write it, color times alpha.
func TestPresentTextureIdentityPremultipliesStraightAlpha(t *testing.T) {
	r, ws, ctx := newIdentityPresentTarget(t, 2, 1)
	straight := []byte{200, 100, 50, 128, 255, 255, 255, 0}
	want := []byte{100, 50, 25, 128, 0, 0, 0, 0}
	tex := newTestTexture(t, r, 2, 1, straight, false)

	if err := ctx.PresentTexture(tex); err != nil {
		t.Fatalf("PresentTexture: %v", err)
	}
	r.endFrameForSurface(ws)

	if got := readSurface(t, ws); !bytes.Equal(got, want) {
		t.Errorf("surface pixels = %v, want %v", got, want)
	}
}

// TestPresentTextureNonIdentityUsesPipeline: when the quad is not an identity,
// PresentTexture is not deferred and the pipeline draws it.
func TestPresentTextureNonIdentityUsesPipeline(t *testing.T) {
	for _, tc := range []struct {
		name   string
		texW   int
		before func(ctx *Context, tex *Texture) error
	}{
		{name: "texture smaller than surface", texW: 2},
		{name: "non-zero clear", texW: 4, before: func(ctx *Context, _ *Texture) error {
			ctx.Clear(1, 0, 0, 1)
			return nil
		}},
		{name: "drawn on top of earlier content", texW: 4, before: func(ctx *Context, tex *Texture) error {
			return ctx.DrawTextureEx(tex, DrawTextureOptions{X: 1, Y: 0, Width: 2, Height: 2, Alpha: 0.5})
		}},
		{name: "surface view handed out", texW: 4, before: func(ctx *Context, _ *Texture) error {
			ctx.SurfaceView()
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, ws, ctx := newIdentityPresentTarget(t, 4, 2)
			tex := newTestTexture(t, r, tc.texW, 2, patternRGBA(tc.texW, 2), true)
			if tc.before != nil {
				if err := tc.before(ctx, tex); err != nil {
					t.Fatalf("before: %v", err)
				}
			}
			if err := ctx.PresentTexture(tex); err != nil {
				t.Fatalf("PresentTexture: %v", err)
			}
			if ws.identityPresent != nil {
				t.Fatal("non-identity PresentTexture was deferred")
			}
			if !r.texQuadPipelineInited {
				t.Error("non-identity PresentTexture did not use the pipeline")
			}
			r.endFrameForSurface(ws)
		})
	}
}

// TestDeferredPresentTextureKeepsFrameOrder: work after a deferred
// PresentTexture draws it through the pipeline first, so the frame equals
// the one the pipeline alone produces.
func TestDeferredPresentTextureKeepsFrameOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		after func(ctx *Context, tex *Texture) error
	}{
		{name: "offset half-alpha draw", after: func(ctx *Context, tex *Texture) error {
			return ctx.DrawTextureEx(tex, DrawTextureOptions{X: 1, Y: 0, Width: 2, Height: 2, Alpha: 0.5})
		}},
		{name: "clear", after: func(ctx *Context, _ *Texture) error {
			ctx.Clear(0, 0, 1, 1)
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := func(pixelPresent bool) []byte {
				r, ws, ctx := newIdentityPresentTarget(t, 4, 2)
				r.pixelPresent = pixelPresent
				tex := newTestTexture(t, r, 4, 2, patternRGBA(4, 2), true)
				if err := ctx.PresentTexture(tex); err != nil {
					t.Fatalf("PresentTexture: %v", err)
				}
				if pixelPresent && ws.identityPresent == nil {
					t.Fatal("identity PresentTexture was not deferred")
				}
				if err := tc.after(ctx, tex); err != nil {
					t.Fatalf("after: %v", err)
				}
				if ws.identityPresent != nil {
					t.Fatal("later work did not draw the deferred PresentTexture")
				}
				r.endFrameForSurface(ws)
				return readSurface(t, ws)
			}
			deferred, pipeline := frame(true), frame(false)
			if !bytes.Equal(deferred, pipeline) {
				t.Errorf("frame differs from the pipeline-only frame\n got %v\nwant %v", deferred, pipeline)
			}
		})
	}
}

// TestWriteSurfacePixelsDropsDeferredPresentTexture: pixels written over the
// whole surface replace a deferred PresentTexture.
func TestWriteSurfacePixelsDropsDeferredPresentTexture(t *testing.T) {
	r, ws, ctx := newIdentityPresentTarget(t, 2, 1)
	tex := newTestTexture(t, r, 2, 1, patternRGBA(2, 1), true)
	if err := ctx.PresentTexture(tex); err != nil {
		t.Fatalf("PresentTexture: %v", err)
	}
	want := []byte{9, 8, 7, 255, 6, 5, 4, 255}
	if err := ctx.RenderTarget().WriteSurfacePixels(want, 2, 1); err != nil {
		t.Fatalf("WriteSurfacePixels: %v", err)
	}
	if ws.identityPresent != nil {
		t.Fatal("WriteSurfacePixels kept the deferred PresentTexture")
	}
	r.endFrameForSurface(ws)
	if got := readSurface(t, ws); !bytes.Equal(got, want) {
		t.Errorf("surface pixels = %v, want %v", got, want)
	}
}

func TestPremultiplyRGBA(t *testing.T) {
	for _, tc := range []struct {
		in, want []byte
	}{
		{in: []byte{10, 20, 30, 255}, want: []byte{10, 20, 30, 255}},
		{in: []byte{255, 255, 255, 0}, want: []byte{0, 0, 0, 0}},
		{in: []byte{255, 128, 1, 128}, want: []byte{128, 64, 1, 128}},
		// 2*191/255 = 1.498 rounds down, 3*191/255 = 2.247 rounds down.
		{in: []byte{2, 3, 0, 191}, want: []byte{1, 2, 0, 191}},
	} {
		got := append([]byte(nil), tc.in...)
		premultiplyRGBA(got)
		if !bytes.Equal(got, tc.want) {
			t.Errorf("premultiplyRGBA(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTextureUsageAddsCopySrcOnlyForPixelPresent(t *testing.T) {
	if textureUsage(false)&gputypes.TextureUsageCopySrc != 0 {
		t.Error("GPU adapters got CopySrc")
	}
	if textureUsage(true)&gputypes.TextureUsageCopySrc == 0 {
		t.Error("CPU adapters did not get CopySrc")
	}
}
