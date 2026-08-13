package gogpu

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/internal/compositor"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Context provides drawing operations for a single frame.
// It is only valid during the OnDraw callback and should not be stored.
//
// In multi-window mode, each Context targets a specific window surface.
// The surface field points to the target RenderTarget; if nil, the
// renderer's primary surface is used (single-window backward compat).
type Context struct {
	renderer    *Renderer
	surface     *RenderTarget // target window surface (nil = use renderer.primary)
	scaleFactor float64       // DPI scale factor (1.0 = standard, 2.0 = Retina/HiDPI)
	cleared     bool
}

// newContext creates a new drawing context for the primary window.
func newContext(renderer *Renderer, scaleFactor float64) *Context {
	if scaleFactor <= 0 {
		scaleFactor = 1.0
	}
	return &Context{
		renderer:    renderer,
		scaleFactor: scaleFactor,
	}
}

// newContextForSurface creates a Context targeting a specific window surface.
// Used by the multi-window frame loop to create per-window contexts.
func newContextForSurface(renderer *Renderer, ws *RenderTarget, scaleFactor float64) *Context {
	if scaleFactor <= 0 {
		scaleFactor = 1.0
	}
	return &Context{
		renderer:    renderer,
		surface:     ws,
		scaleFactor: scaleFactor,
	}
}

// activeSurface returns the RenderTarget targeted by this Context.
func (c *Context) activeSurface() *RenderTarget {
	if c.surface != nil {
		return c.surface
	}
	return c.renderer.primary
}

// RequestPresentationSync asks the platform compositor to acknowledge the next
// successfully presented frame before another frame is rendered. It is useful
// for infrequent visual state transitions in applications that otherwise
// render without compositor pacing. Repeated calls before that presentation
// are idempotent.
//
// Call this during OnDraw, before the frame is presented. This method neither
// requests a redraw nor forces an otherwise empty OnDraw to present; the request
// remains pending until a later frame is presented. Platforms that already
// synchronize every frame need no additional work.
func (c *Context) RequestPresentationSync() {
	c.activeSurface().presentationSyncRequested = true
}

// RegisterDamageSource registers a named damage source with the compositor.
// Each independent renderer (gg, g3d, video, compose) registers once at init
// and reports per-frame damage through the returned DamageReporter.
//
// The returned DamageReporter implements gpucontext.DamageReporter. The source
// is assigned a palette color for debug overlay rendering. Sources are unioned
// at present time — if ANY source reports full damage, the entire surface is
// presented. See ADR-065 for the multi-renderer damage tracking design.
//
// All damage operations happen on the render thread (same goroutine as OnDraw).
func (c *Context) RegisterDamageSource(name string) gpucontext.DamageReporter {
	ws := c.activeSurface()
	ds := compositor.NewDamageSource(name, len(ws.damageSources))
	ws.damageSources = append(ws.damageSources, ds)
	return ds
}

// RegisterDebugOverlay adds a debug overlay to the active surface.
// Overlays draw in registration order after all content renderers have
// finished, before present. GTK4 pattern: GtkInspectorOverlay list
// iterated by compositor.
//
// The overlay's Name must be unique among registered overlays. Registering
// a duplicate name replaces the existing overlay (allowing hot-swap between
// basic and text-enhanced versions, e.g., gogpu built-in -> gg override).
//
// Called once at init, not per-frame. Thread safety: render thread only
// (same goroutine as OnDraw).
func (c *Context) RegisterDebugOverlay(overlay gpucontext.DebugOverlay) {
	ws := c.activeSurface()
	name := overlay.Name()
	for i, existing := range ws.debugOverlays {
		if existing.Name() == name {
			ws.debugOverlays[i] = overlay
			return
		}
	}
	ws.debugOverlays = append(ws.debugOverlays, overlay)
}

// RemoveDebugOverlay removes a debug overlay by name.
// No-op if the name is not found. Thread safety: render thread only.
func (c *Context) RemoveDebugOverlay(name string) {
	ws := c.activeSurface()
	for i, overlay := range ws.debugOverlays {
		if overlay.Name() == name {
			ws.debugOverlays = append(ws.debugOverlays[:i], ws.debugOverlays[i+1:]...)
			return
		}
	}
}

// SetDamageOverlayRenderer registers a custom renderer for the damage debug
// overlay. Libraries with text rendering capability (e.g., gg) use this to
// provide anti-aliased borders, text labels per source, and richer visuals
// than the built-in flat-color quads.
//
// When set, the built-in damage overlay delegates rendering to this renderer
// instead of using its own GPU pipeline. The renderer receives a
// DamageOverlayInfo snapshot each frame containing per-source damage data.
//
// If the damage overlay is not yet registered (GOGPU_DEBUG_DAMAGE not set),
// calling this forces overlay activation so the custom renderer takes effect.
//
// Called once at init, not per-frame. Thread safety: render thread only
// (same goroutine as OnDraw).
func (c *Context) SetDamageOverlayRenderer(renderer gpucontext.DamageOverlayRenderer) {
	ws := c.activeSurface()
	ws.setCustomDamageRenderer(renderer)
}

// MarkPreserveContent signals that the active surface already contains GPU
// content that subsequent render passes must preserve. This sets the internal
// state so the next render pass uses LoadOp::Load instead of LoadOp::Clear.
//
// Use case: an external renderer (e.g., g3d) has submitted GPU commands to
// the surface via CommandEncoder. Call MarkPreserveContent after the external
// renderer finishes so subsequent renderers (gg, ui) draw on top instead of
// clearing the surface.
//
// This is the GPU LoadOp concern split out from the removed MarkExternalContent
// (ADR-065). Damage reporting is now handled separately via RegisterDamageSource.
func (c *Context) MarkPreserveContent() {
	ws := c.activeSurface()
	if !ws.ensureFrameStarted() {
		return
	}
	ws.frameCleared = true
	ws.externalContent = true
	ws.hasGPUWork = true
}

// Clear clears the framebuffer with the specified RGBA color.
// Values should be in the range [0.0, 1.0].
func (c *Context) Clear(r, g, b, a float32) {
	c.renderer.Clear(float64(r), float64(g), float64(b), float64(a))
	c.cleared = true
}

// ClearColor clears the framebuffer with a Color value.
func (c *Context) ClearColor(color gmath.Color) {
	c.Clear(color.R, color.G, color.B, color.A)
}

// Size returns the window dimensions in logical points (DIP).
// Use this for layout, UI coordinates, and user-facing dimensions.
// On Retina/HiDPI displays, this is smaller than FramebufferSize by ScaleFactor.
func (c *Context) Size() (width, height int) {
	pw, ph := c.renderer.Size()
	return int(float64(pw) / c.scaleFactor), int(float64(ph) / c.scaleFactor)
}

// Width returns the window width in logical points (DIP).
func (c *Context) Width() int {
	w, _ := c.Size()
	return w
}

// Height returns the window height in logical points (DIP).
func (c *Context) Height() int {
	_, h := c.Size()
	return h
}

// FramebufferSize returns the GPU framebuffer dimensions in physical device pixels.
// Use this for GPU operations, texture allocation, and pixel-precise rendering.
func (c *Context) FramebufferSize() (width, height int) {
	return c.renderer.Size()
}

// FramebufferWidth returns the GPU framebuffer width in physical device pixels.
func (c *Context) FramebufferWidth() int {
	w, _ := c.renderer.Size()
	return w
}

// FramebufferHeight returns the GPU framebuffer height in physical device pixels.
func (c *Context) FramebufferHeight() int {
	_, h := c.renderer.Size()
	return h
}

// ScaleFactor returns the DPI scale factor.
// 1.0 = standard (96 DPI on Windows), 2.0 = Retina/HiDPI.
func (c *Context) ScaleFactor() float64 {
	return c.scaleFactor
}

// AspectRatio returns width/height as a float32 (based on logical size).
func (c *Context) AspectRatio() float32 {
	w, h := c.Size()
	if h == 0 {
		return 1.0
	}
	return float32(w) / float32(h)
}

// Format returns the surface texture format.
// Useful for creating compatible pipelines.
func (c *Context) Format() gputypes.TextureFormat {
	return c.renderer.Format()
}

// Backend returns the name of the active backend.
// Returns "Rust (wgpu-gpu)" or "Pure Go (gogpu/wgpu)".
func (c *Context) Backend() string {
	return c.renderer.Backend()
}

// DrawTriangle draws a built-in RGB-colored triangle.
// This is a convenience method for quick demos and testing.
// The background is cleared with the specified color before drawing.
func (c *Context) DrawTriangle(bgR, bgG, bgB, bgA float32) error {
	err := c.renderer.DrawTriangle(float64(bgR), float64(bgG), float64(bgB), float64(bgA))

	c.cleared = true
	return err
}

// DrawTriangleColor draws a triangle with a background Color.
func (c *Context) DrawTriangleColor(bg gmath.Color) error {
	err := c.DrawTriangle(bg.R, bg.G, bg.B, bg.A)
	return err
}

// Renderer returns the underlying Renderer for texture creation.
// This allows creating textures from within the OnDraw callback.
// Note: Textures should be created once and reused, not every frame.
func (c *Context) Renderer() *Renderer {
	return c.renderer
}

// SurfaceView returns the current frame's render target texture view.
// When a composition texture is active (ADR-067, debug overlays present),
// this returns the composition view so content renders into the intermediate
// texture. Otherwise it returns the swapchain view directly.
// Returns nil if no frame is in progress.
//
// Use this with ggcanvas.RenderDirect for zero-copy GPU rendering,
// bypassing the GPU->CPU->GPU readback path.
func (c *Context) SurfaceView() *wgpu.TextureView {
	ws := c.activeSurface()
	if !ws.ensureFrameStarted() {
		return nil
	}
	ws.hasGPUWork = true
	return ws.renderView()
}

// CommandEncoder returns the framework-owned command encoder for the active
// surface frame. External renderers can record additional passes into it so all
// work targeting the swapchain image is submitted once at frame end.
//
// The returned encoder is borrowed and valid only during the current draw
// callback. Callers must not call Finish, Submit, or DiscardEncoding on it.
// Returns nil when the surface frame or encoder cannot be created.
func (c *Context) CommandEncoder() *wgpu.CommandEncoder {
	ws := c.activeSurface()
	if ws == nil {
		return nil
	}
	encoder, err := ws.ensureFrameEncoder()
	if err != nil {
		slog.Error("gogpu: shared frame encoder unavailable", "err", err)
		return nil
	}
	// Deferred clears must precede every externally recorded pass. Flushing
	// here preserves call order; waiting until EndFrame would clear overlays.
	if !ws.flushClear(ws.renderer.device, ws.renderer) {
		return nil
	}
	ws.hasGPUWork = true
	return encoder
}

// PresentTexture draws a texture filling the entire surface.
// This is the universal path for presenting pre-rendered content (e.g., from
// ggcanvas.Flush) on any backend including software.
// The tex parameter must be a *gogpu.Texture. Returns an error if tex is nil
// or not the expected type.
func (c *Context) PresentTexture(tex any) error {
	if tex == nil {
		return fmt.Errorf("gogpu: PresentTexture called with nil texture")
	}
	t, ok := tex.(*Texture)
	if !ok {
		return fmt.Errorf("gogpu: PresentTexture expects *gogpu.Texture, got %T", tex)
	}
	if t == nil {
		return fmt.Errorf("gogpu: PresentTexture called with nil *Texture")
	}
	ws := c.activeSurface()
	slog.Debug("gogpu: PresentTexture",
		"texW", t.width, "texH", t.height,
		"surfaceW", ws.width, "surfaceH", ws.height,
		"scale", c.scaleFactor,
	)
	return c.renderer.drawTexturedQuad(t, DrawTextureOptions{
		Width:  float32(ws.width),
		Height: float32(ws.height),
		Alpha:  1.0,
	})
}

// RenderTarget returns an adapter that satisfies ggcanvas.RenderTarget interface.
// Use with canvas.Render(dc.RenderTarget()) for universal backend-agnostic rendering.
func (c *Context) RenderTarget() *ContextRenderTarget {
	return &ContextRenderTarget{ctx: c}
}

// ContextRenderTarget adapts *Context to ggcanvas.RenderTarget interface.
type ContextRenderTarget struct{ ctx *Context }

// CommandEncoder returns the active framework-owned encoder as an opaque
// handle. This optional capability lets compositors record into the same
// command buffer without depending directly on gogpu or wgpu.
func (r *ContextRenderTarget) CommandEncoder() gpucontext.CommandEncoder {
	encoder := r.ctx.CommandEncoder()
	if encoder == nil {
		return gpucontext.CommandEncoder{}
	}
	return gpucontext.NewCommandEncoder(unsafe.Pointer(encoder)) //nolint:gosec // Go spec Rule 1: *T -> unsafe.Pointer
}

// PreserveContent reports whether the active surface already contains content
// that subsequent render passes must load. This exposes MarkPreserveContent's
// frame state to ggcanvas without introducing a dependency on gg.
func (r *ContextRenderTarget) PreserveContent() bool {
	ws := r.ctx.activeSurface()
	return ws != nil && ws.externalContent
}

// SurfaceView returns the render target texture view as a type-safe opaque handle.
// When a composition texture is active (ADR-067), returns the composition view.
func (r *ContextRenderTarget) SurfaceView() gpucontext.TextureView {
	tv := r.ctx.SurfaceView()
	if tv == nil {
		return gpucontext.TextureView{}
	}
	return gpucontext.NewTextureView(unsafe.Pointer(tv)) //nolint:gosec // Go spec Rule 1: *T -> unsafe.Pointer (ADR-018 opaque handle)
}

// SurfaceSize returns the surface dimensions.
func (r *ContextRenderTarget) SurfaceSize() (uint32, uint32) { return r.ctx.SurfaceSize() }

// PresentTexture draws a texture filling the entire surface.
func (r *ContextRenderTarget) PresentTexture(tex any) error { return r.ctx.PresentTexture(tex) }

// WriteSurfacePixels writes RGBA pixel data directly to the surface and presents
// in a single operation. On the software backend this bypasses the entire WebGPU
// render pass pipeline — one RGBA→BGRA swizzle+copy into the DIB section,
// then BitBlt to the window. Falls back to error on GPU backends.
func (r *ContextRenderTarget) WriteSurfacePixels(data []byte, width, height uint32) error {
	ws := r.ctx.activeSurface()
	if ws == nil || ws.surface == nil {
		return fmt.Errorf("gogpu: no active surface")
	}
	syncAttempt, syncedBeforePresent := syncFrameBeforePixelPresent(ws)
	lockDisplay(ws.platWindow)
	err := ws.surface.PresentPixels(data, width, height, compositor.UnionAllSources(ws.damageSources))
	unlockDisplay(ws.platWindow)
	finishPresentationSync(ws, syncAttempt, err == nil)
	if err == nil && !syncedBeforePresent {
		syncFrameAfterPixelPresent(ws)
	}
	for _, ds := range ws.damageSources {
		ds.Reset()
	}
	if err != nil {
		return err
	}
	ws.pixelPresented = true
	ws.discardFrameEncoder()
	if ws.currentView != nil {
		ws.currentView.Release()
		ws.currentView = nil
	}
	ws.currentSurfaceTexture = nil
	return nil
}

// RegisterDamageSource registers a named damage source with the compositor
// and returns a DamageReporter for reporting per-frame damage rectangles.
// This adapter enables ggcanvas (and other renderers) to register damage
// sources without importing gogpu — they detect this capability via
// interface assertion on the RenderTarget.
//
// See Context.RegisterDamageSource for full documentation.
func (r *ContextRenderTarget) RegisterDamageSource(name string) gpucontext.DamageReporter {
	return r.ctx.RegisterDamageSource(name)
}

// TextureCreator returns the texture creator for promoting pending textures.
// This enables the universal rendering path (CPU pixmap -> GPU texture -> present)
// to create real GPU textures from raw pixel data.
func (r *ContextRenderTarget) TextureCreator() gpucontext.TextureCreator {
	return &rendererTextureCreator{renderer: r.ctx.renderer}
}

// Compositor returns the surface compositor for this render target.
// Content renderers (gg, g3d) use this to query compositor state:
// LoadOp decisions, damage rects, and MSAA overlay compositing.
// Returns nil if no active surface exists.
func (r *ContextRenderTarget) Compositor() gpucontext.SurfaceCompositor {
	ws := r.ctx.activeSurface()
	if ws == nil {
		return nil
	}
	return ws
}

// CheckDeviceHealth returns nil if the GPU device is operational, or an error
// describing why the device was removed. This is a diagnostic method for
// debugging DX12 DEVICE_REMOVED issues.
func (c *Context) CheckDeviceHealth() error {
	type healthChecker interface {
		CheckHealth(label string) error
	}
	// Check the underlying HAL device for health (e.g., DX12 DEVICE_REMOVED).
	if c.renderer.device == nil {
		return nil
	}
	halDev := c.renderer.device.HalDevice()
	if hc, ok := halDev.(healthChecker); ok {
		return hc.CheckHealth("Context.CheckDeviceHealth")
	}
	return nil // Backend doesn't support health check
}

// SurfaceSize returns the current GPU surface dimensions in physical device pixels.
// This is the same as FramebufferSize but returns uint32 for GPU API compatibility.
func (c *Context) SurfaceSize() (width, height uint32) {
	ws := c.activeSurface()
	return ws.width, ws.height
}
