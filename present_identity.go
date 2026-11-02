package gogpu

import (
	"log/slog"

	"github.com/gogpu/gputypes"
)

// Identity present on CPU adapters (P1-PRESENT-002).
//
// The software backend runs every fragment shader through an interpreter, so
// drawing a full-surface textured quad costs seconds per frame at desktop
// sizes. When PresentTexture is the frame's only content, covers the whole
// surface at its size with alpha 1, and lands on a transparent-black (or no)
// clear, the quad is an identity: its result is the texture's premultiplied
// pixels. gogpu then reads the texture back and presents it through the
// surface's PresentPixels extension, the route ggcanvas already uses for CPU
// pixmaps (ADR-052), instead of running the shader.
//
// The decision is deferred to EndFrame. Any later work on the surface in the
// same frame (a draw, a clear, a view or encoder handed out) first draws the
// deferred texture through the pipeline, so the frame matches what the
// pipeline would have produced in every case.

// fullSurfaceQuad returns the options PresentTexture draws with.
func fullSurfaceQuad(ws *RenderTarget) DrawTextureOptions {
	return DrawTextureOptions{
		Width:  float32(ws.width),
		Height: float32(ws.height),
		Alpha:  1.0,
	}
}

// canDeferIdentityPresent reports whether PresentTexture(tex) on ws is an
// identity that may be presented as pixels at EndFrame. The frame must have
// been started.
func (ws *RenderTarget) canDeferIdentityPresent(tex *Texture) bool {
	r := ws.renderer
	if r == nil || !r.pixelPresent || ws.surface == nil {
		return false
	}
	if tex.texture == nil || tex.format != gputypes.TextureFormatRGBA8Unorm ||
		tex.width != int(ws.width) || tex.height != int(ws.height) {
		return false
	}
	// Unorm only: an sRGB surface would encode on write, PresentPixels does not.
	if ws.format != gputypes.TextureFormatBGRA8Unorm && ws.format != gputypes.TextureFormatRGBA8Unorm {
		return false
	}
	// Nothing may be on the surface yet: the quad blends src-over, and a copy
	// equals that blend only over transparent black.
	if ws.identityPresent != nil || ws.frameCleared || ws.externalContent ||
		ws.surfaceExposed || ws.frameEncoder != nil {
		return false
	}
	if ws.hasPendingClear && ws.pendingClearColor != (gputypes.Color{}) {
		return false
	}
	// Debug overlays draw after the content at EndFrame; PresentPixels ends
	// the frame's swapchain use before them.
	if len(ws.debugOverlays) > 0 || ws.composView != nil || ws.hasRegisteredOverlayEnv() {
		return false
	}
	return true
}

// materializeIdentityPresent draws a deferred PresentTexture through the
// textured-quad pipeline. Called before any other work on ws in the frame.
func (ws *RenderTarget) materializeIdentityPresent() {
	tex := ws.identityPresent
	if tex == nil {
		return
	}
	ws.identityPresent = nil
	if err := ws.renderer.drawTexturedQuad(ws, tex, fullSurfaceQuad(ws)); err != nil {
		slog.Error("gogpu: deferred PresentTexture draw failed", "err", err)
	}
}

// presentIdentityFrame presents a deferred PresentTexture at EndFrame: as
// pixels when the surface accepts them, otherwise through the pipeline.
func (r *Renderer) presentIdentityFrame(ws *RenderTarget) {
	tex := ws.identityPresent
	if tex == nil {
		return
	}
	if tex.texture != nil {
		pixels, err := r.readIdentityPixels(tex)
		if err == nil {
			// Width and height equal the surface's (canDeferIdentityPresent).
			err = ws.presentPixels(pixels, ws.width, ws.height)
			if err == nil {
				ws.identityPresent = nil
				return
			}
			// This surface is no pixel presenter (a CPU adapter on a GPU API,
			// e.g. llvmpipe): stop trying, keep the pipeline.
			r.pixelPresent = false
		}
		slog.Debug("gogpu: identity present fell back to the pipeline", "err", err)
	}
	ws.materializeIdentityPresent()
}

// readIdentityPixels reads tex back as tightly packed, premultiplied RGBA:
// the bytes the textured-quad shader would write at alpha 1.
func (r *Renderer) readIdentityPixels(tex *Texture) ([]byte, error) {
	w, h := tex.width, tex.height
	pixels := make([]byte, w*h*4)
	err := r.readbackTexture(tex.texture, w, h, "PresentTexture", func(raw []byte, rowBytes int) {
		for y := 0; y < h; y++ {
			copy(pixels[y*w*4:(y+1)*w*4], raw[y*rowBytes:y*rowBytes+w*4])
		}
	})
	if err != nil {
		return nil, err
	}
	if !tex.premultiplied {
		premultiplyRGBA(pixels)
	}
	return pixels, nil
}

// premultiplyRGBA converts straight-alpha RGBA8 to premultiplied in place,
// rounding to nearest as the shader's unorm write does (c*a/255).
func premultiplyRGBA(pix []byte) {
	for i := 0; i+3 < len(pix); i += 4 {
		a := uint32(pix[i+3])
		if a == 255 {
			continue
		}
		pix[i] = uint8((uint32(pix[i])*a + 127) / 255)     //nolint:gosec // G115: result <= 255
		pix[i+1] = uint8((uint32(pix[i+1])*a + 127) / 255) //nolint:gosec // G115: result <= 255
		pix[i+2] = uint8((uint32(pix[i+2])*a + 127) / 255) //nolint:gosec // G115: result <= 255
	}
}
