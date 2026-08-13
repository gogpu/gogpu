// Package main demonstrates bidirectional OS file drag-and-drop.
//
// Window is split in half:
//
//	LEFT  "OUT" — drag icons out to Desktop / Finder (or into IN)
//	RIGHT "IN"  — drop files from Finder (or from OUT); icons can be dragged out again
//
// Moving an icon between the two panes removes it from the source pane.
package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/input"
)

const (
	winW = 560
	winH = 280

	iconW     float32 = 64
	iconH     float32 = 80
	dragSlop2 float32 = 25 // 5px²
	labelH    float32 = 28
	pad       float32 = 16
)

type pane int

const (
	paneOut pane = iota
	paneIn
)

type fileItem struct {
	path string
	pane pane
}

type demo struct {
	app *gogpu.App

	mu    sync.Mutex
	items []fileItem

	inDrag   bool
	canDrag  bool
	hasClick bool
	clickX   float32
	clickY   float32
	dragIdx  int
	hoverIdx int
	scale    float32
	half     float32

	docTex   *gogpu.Texture
	outBgTex *gogpu.Texture
	inBgTex  *gogpu.Texture
	divTex   *gogpu.Texture
	texErr   error
	labelTex map[string]*gogpu.Texture
}

func main() {
	tmpFile, cleanup, err := createSeedFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	fmt.Printf("Seed file: %s\n", tmpFile)
	fmt.Println("LEFT  OUT — drag icons to Desktop or into IN")
	fmt.Println("RIGHT IN  — drop files from Finder; drag icons back out")

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Drag & Drop — OUT | IN").
		WithSize(winW, winH).
		WithContinuousRender(true))

	d := &demo{
		app:      app,
		items:    []fileItem{{path: tmpFile, pane: paneOut}},
		dragIdx:  -1,
		hoverIdx: -1,
		scale:    1,
		half:     float32(winW) / 2,
		labelTex: map[string]*gogpu.Texture{},
	}

	app.OnDragDrop(d.onDrop)
	app.OnUpdate(d.onUpdate)
	app.OnDraw(d.onDraw)
	app.OnClose(d.onClose)

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createSeedFile() (path string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp("", "gogpu-drag-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	path = filepath.Join(tmpDir, "hello-from-gogpu.txt")
	content := fmt.Sprintf("Hello from GoGPU drag-and-drop!\nTimestamp: %s\n", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	return path, cleanup, nil
}

func (d *demo) onDrop(paths []string, x, y float64) {
	lx := float32(x) / d.scale
	fmt.Printf("[drop] received %d file(s) at physical (%.1f, %.1f):\n", len(paths), x, y)
	dest := paneIn
	if lx < d.half {
		dest = paneOut
	}
	for _, path := range paths {
		fmt.Printf("  → %s  (%s)\n", filepath.Base(path), paneName(dest))
		d.moveToPane(path, dest)
	}
}

func (d *demo) onUpdate(dt float64) {
	if s := float32(d.app.ScaleFactor()); s > 0 {
		d.scale = s
	}
	mouse := d.app.Input().Mouse()
	mx, my := mouse.Position()

	d.mu.Lock()
	local := append([]fileItem(nil), d.items...)
	d.hoverIdx = d.hitTest(local, mx, my)
	d.mu.Unlock()

	if !mouse.Pressed(input.MouseButtonLeft) {
		d.canDrag = true
		d.hasClick = false
		d.dragIdx = -1
	}

	if mouse.JustPressed(input.MouseButtonLeft) {
		d.logClick(local, mx)
	}
	d.maybeStartDrag(local, mx, my, mouse)
}

func (d *demo) logClick(local []fileItem, mx float32) {
	if d.hoverIdx >= 0 {
		it := local[d.hoverIdx]
		fmt.Printf("[mouse] %q (%s)\n", filepath.Base(it.path), paneName(it.pane))
		return
	}
	side := "IN"
	if mx < d.half {
		side = "OUT"
	}
	fmt.Printf("[mouse] empty %s pane\n", side)
}

func (d *demo) maybeStartDrag(local []fileItem, mx, my float32, mouse *input.MouseState) {
	if !d.canDrag || !mouse.Pressed(input.MouseButtonLeft) || d.inDrag {
		return
	}
	if !d.hasClick {
		d.clickX, d.clickY = mx, my
		d.hasClick = true
		d.dragIdx = d.hitTest(local, d.clickX, d.clickY)
	}
	if d.dragIdx < 0 || d.dragIdx >= len(local) {
		return
	}
	dx := mx - d.clickX
	dy := my - d.clickY
	if dx*dx+dy*dy <= dragSlop2 {
		return
	}

	d.canDrag = false
	d.inDrag = true
	item := local[d.dragIdx]
	fmt.Printf("[drag] starting %q from %s...\n", filepath.Base(item.path), paneName(item.pane))

	d.app.StartDrag(gogpu.DragData{
		FilePaths: []string{item.path},
	}, func(result gogpu.DragResult) {
		d.inDrag = false
		switch result {
		case gogpu.DragCopied:
			fmt.Println("[drag] result: COPIED (source kept)")
		case gogpu.DragMoved:
			d.removePath(item.path)
			fmt.Println("[drag] result: MOVED (removed from window)")
		default:
			fmt.Println("[drag] result: CANCELED")
		}
	})
}

func (d *demo) onDraw(dc *gogpu.Context) {
	if s := float32(dc.ScaleFactor()); s > 0 {
		d.scale = s
	}
	dc.ClearColor(gmath.Hex(0x1E1F22))

	pw, ph := dc.FramebufferSize()
	halfPhys := float32(pw) / 2

	if !d.ensureTextures(dc.Renderer()) {
		return
	}
	d.drawPanes(dc, halfPhys, float32(ph))
	d.drawItems(dc)
}

func (d *demo) ensureTextures(r *gogpu.Renderer) bool {
	if d.docTex != nil || d.texErr != nil {
		return d.docTex != nil
	}
	d.docTex, d.texErr = createDocumentIcon(r)
	d.outBgTex, _ = createPaneBackground(r, 280, 280, "OUT", color.RGBA{70, 120, 230, 255})
	d.inBgTex, _ = createPaneBackground(r, 280, 280, "IN", color.RGBA{60, 180, 120, 255})
	d.divTex = makeDividerTex(r)
	return d.docTex != nil
}

func (d *demo) drawPanes(dc *gogpu.Context, halfPhys, ph float32) {
	if d.outBgTex != nil {
		_ = dc.DrawTextureScaled(d.outBgTex, 0, 0, halfPhys, ph)
	}
	if d.inBgTex != nil {
		_ = dc.DrawTextureScaled(d.inBgTex, halfPhys, 0, halfPhys, ph)
	}
	if d.divTex != nil {
		_ = dc.DrawTextureScaled(d.divTex, halfPhys-d.scale, 0, 2*d.scale, ph)
	}
}

func (d *demo) drawItems(dc *gogpu.Context) {
	d.mu.Lock()
	snapshot := append([]fileItem(nil), d.items...)
	hi := d.hoverIdx
	inDrag := d.inDrag
	d.mu.Unlock()

	for i, it := range snapshot {
		lx, ly, lw, lh := d.iconRect(it, d.paneIndex(snapshot, i))
		if i == hi && !inDrag {
			ly -= 2
		}
		_ = dc.DrawTextureScaled(d.docTex, lx*d.scale, ly*d.scale, lw*d.scale, lh*d.scale)
		d.drawItemLabel(dc, it.path, lx, ly, lw, lh)
	}
}

func (d *demo) drawItemLabel(dc *gogpu.Context, path string, lx, ly, lw, lh float32) {
	name := truncate(filepath.Base(path), 10)
	lt := d.labelTex[name]
	if lt == nil {
		var err error
		lt, err = createLabelTexture(dc.Renderer(), name, color.RGBA{220, 220, 225, 255})
		if err != nil {
			return
		}
		d.labelTex[name] = lt
	}
	tw := float32(lt.Width()) * d.scale * 0.5
	th := float32(lt.Height()) * d.scale * 0.5
	_ = dc.DrawTextureScaled(lt, lx*d.scale+(lw*d.scale-tw)/2, (ly+lh+4)*d.scale, tw, th)
}

func (d *demo) onClose() {
	destroyTex(d.docTex)
	destroyTex(d.outBgTex)
	destroyTex(d.inBgTex)
	destroyTex(d.divTex)
	for _, t := range d.labelTex {
		destroyTex(t)
	}
}

func destroyTex(t *gogpu.Texture) {
	if t != nil {
		t.Destroy()
	}
}

func (d *demo) moveToPane(path string, dest pane) {
	d.mu.Lock()
	defer d.mu.Unlock()
	dst := d.items[:0]
	for _, it := range d.items {
		if it.path != path {
			dst = append(dst, it)
		}
	}
	d.items = append(dst, fileItem{path: path, pane: dest})
}

func (d *demo) removePath(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	dst := d.items[:0]
	for _, it := range d.items {
		if it.path != path {
			dst = append(dst, it)
		}
	}
	d.items = dst
}

func (d *demo) iconRect(it fileItem, indexInPane int) (x, y, w, h float32) {
	col := indexInPane % 2
	row := indexInPane / 2
	baseX := pad
	if it.pane == paneIn {
		baseX = d.half + pad
	}
	x = baseX + float32(col)*(iconW+12)
	y = labelH + pad + float32(row)*(iconH+28)
	return x, y, iconW, iconH
}

func (d *demo) paneIndex(list []fileItem, globalIdx int) int {
	n := 0
	p := list[globalIdx].pane
	for i := 0; i < globalIdx; i++ {
		if list[i].pane == p {
			n++
		}
	}
	return n
}

func (d *demo) hitTest(list []fileItem, mx, my float32) int {
	for i := len(list) - 1; i >= 0; i-- {
		x, y, w, h := d.iconRect(list[i], d.paneIndex(list, i))
		if mx >= x && mx < x+w && my >= y && my < y+h {
			return i
		}
	}
	return -1
}

func paneName(p pane) string {
	if p == paneOut {
		return "OUT"
	}
	return "IN"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
