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
	"strings"
	"sync"
	"time"
	"unicode"

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

func main() {
	tmpDir, err := os.MkdirTemp("", "gogpu-drag-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "hello-from-gogpu.txt")
	content := fmt.Sprintf("Hello from GoGPU drag-and-drop!\nTimestamp: %s\n", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Seed file: %s\n", tmpFile)
	fmt.Println("LEFT  OUT — drag icons to Desktop or into IN")
	fmt.Println("RIGHT IN  — drop files from Finder; drag icons back out")

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Drag & Drop — OUT | IN").
		WithSize(winW, winH).
		WithContinuousRender(true))

	var (
		mu       sync.Mutex
		items    = []fileItem{{path: tmpFile, pane: paneOut}}
		inDrag   bool
		canDrag  bool
		hasClick bool
		clickX   float32
		clickY   float32
		dragIdx  = -1
		hoverIdx = -1
		scale    = float32(1)
	)

	half := float32(winW) / 2

	iconRect := func(it fileItem, indexInPane int) (x, y, w, h float32) {
		cols := 2
		col := indexInPane % cols
		row := indexInPane / cols
		baseX := pad
		if it.pane == paneIn {
			baseX = half + pad
		}
		x = baseX + float32(col)*(iconW+12)
		y = labelH + pad + float32(row)*(iconH+28)
		return x, y, iconW, iconH
	}

	paneIndex := func(list []fileItem, globalIdx int) int {
		n := 0
		p := list[globalIdx].pane
		for i := 0; i < globalIdx; i++ {
			if list[i].pane == p {
				n++
			}
		}
		return n
	}

	hitTest := func(list []fileItem, mx, my float32) int {
		for i := len(list) - 1; i >= 0; i-- {
			x, y, w, h := iconRect(list[i], paneIndex(list, i))
			if mx >= x && mx < x+w && my >= y && my < y+h {
				return i
			}
		}
		return -1
	}

	moveToPane := func(path string, dest pane) {
		mu.Lock()
		defer mu.Unlock()
		// Remove any existing entries for this path (move, not duplicate).
		dst := items[:0]
		for _, it := range items {
			if it.path != path {
				dst = append(dst, it)
			}
		}
		items = append(dst, fileItem{path: path, pane: dest})
	}

	removePath := func(path string) {
		mu.Lock()
		defer mu.Unlock()
		dst := items[:0]
		for _, it := range items {
			if it.path != path {
				dst = append(dst, it)
			}
		}
		items = dst
	}

	app.OnDragDrop(func(paths []string, x, y float64) {
		// Drop position is physical pixels — convert to logical for pane test.
		lx := float32(x) / scale
		fmt.Printf("[drop] received %d file(s) at physical (%.1f, %.1f):\n", len(paths), x, y)
		dest := paneIn
		if lx < half {
			dest = paneOut
		}
		for _, path := range paths {
			fmt.Printf("  → %s  (%s)\n", filepath.Base(path), paneName(dest))
			moveToPane(path, dest)
		}
	})

	app.OnUpdate(func(dt float64) {
		if s := float32(app.ScaleFactor()); s > 0 {
			scale = s
		}
		mouse := app.Input().Mouse()
		mx, my := mouse.Position() // logical DIP

		mu.Lock()
		local := append([]fileItem(nil), items...)
		hoverIdx = hitTest(local, mx, my)
		mu.Unlock()

		if !mouse.Pressed(input.MouseButtonLeft) {
			canDrag = true
			hasClick = false
			dragIdx = -1
		}

		if mouse.JustPressed(input.MouseButtonLeft) {
			if hoverIdx >= 0 {
				fmt.Printf("[mouse] %q (%s)\n", filepath.Base(local[hoverIdx].path), paneName(local[hoverIdx].pane))
			} else {
				side := "IN"
				if mx < half {
					side = "OUT"
				}
				fmt.Printf("[mouse] empty %s pane\n", side)
			}
		}

		if canDrag && mouse.Pressed(input.MouseButtonLeft) && !inDrag {
			if !hasClick {
				clickX, clickY = mx, my
				hasClick = true
				dragIdx = hitTest(local, clickX, clickY)
			}
			if dragIdx < 0 || dragIdx >= len(local) {
				return
			}
			dx := mx - clickX
			dy := my - clickY
			if dx*dx+dy*dy <= dragSlop2 {
				return
			}

			canDrag = false
			inDrag = true
			item := local[dragIdx]
			fmt.Printf("[drag] starting %q from %s...\n", filepath.Base(item.path), paneName(item.pane))

			app.StartDrag(gogpu.DragData{
				FilePaths: []string{item.path},
			}, func(result gogpu.DragResult) {
				inDrag = false
				switch result {
				case gogpu.DragCopied:
					fmt.Println("[drag] result: COPIED (source kept)")
				case gogpu.DragMoved:
					removePath(item.path)
					fmt.Println("[drag] result: MOVED (removed from window)")
				default:
					fmt.Println("[drag] result: CANCELED")
				}
			})
		}
	})

	var (
		docTex   *gogpu.Texture
		outBgTex *gogpu.Texture
		inBgTex  *gogpu.Texture
		divTex   *gogpu.Texture
		texErr   error
		labelTex = map[string]*gogpu.Texture{}
	)

	app.OnDraw(func(dc *gogpu.Context) {
		if s := float32(dc.ScaleFactor()); s > 0 {
			scale = s
		}
		dc.ClearColor(gmath.Hex(0x1E1F22))

		pw, ph := dc.FramebufferSize()
		halfPhys := float32(pw) / 2

		if docTex == nil && texErr == nil {
			docTex, texErr = createDocumentIcon(dc.Renderer())
			outBgTex, _ = createPaneBackground(dc.Renderer(), 280, 280, "OUT", color.RGBA{70, 120, 230, 255})
			inBgTex, _ = createPaneBackground(dc.Renderer(), 280, 280, "IN", color.RGBA{60, 180, 120, 255})
			divTex = makeDividerTex(dc.Renderer())
		}
		if docTex == nil {
			return
		}

		// Pane backgrounds — physical pixels (framebuffer space).
		if outBgTex != nil {
			_ = dc.DrawTextureScaled(outBgTex, 0, 0, halfPhys, float32(ph))
		}
		if inBgTex != nil {
			_ = dc.DrawTextureScaled(inBgTex, halfPhys, 0, halfPhys, float32(ph))
		}
		if divTex != nil {
			_ = dc.DrawTextureScaled(divTex, halfPhys-scale, 0, 2*scale, float32(ph))
		}

		mu.Lock()
		snapshot := append([]fileItem(nil), items...)
		hi := hoverIdx
		mu.Unlock()

		for i, it := range snapshot {
			lx, ly, lw, lh := iconRect(it, paneIndex(snapshot, i))
			if i == hi && !inDrag {
				ly -= 2
			}
			// Logical → physical for Retina-correct hit targets.
			_ = dc.DrawTextureScaled(docTex, lx*scale, ly*scale, lw*scale, lh*scale)

			name := truncate(filepath.Base(it.path), 10)
			lt := labelTex[name]
			if lt == nil {
				var err error
				lt, err = createLabelTexture(dc.Renderer(), name, color.RGBA{220, 220, 225, 255})
				if err == nil {
					labelTex[name] = lt
				}
			}
			if lt != nil {
				tw := float32(lt.Width()) * scale * 0.5
				th := float32(lt.Height()) * scale * 0.5
				_ = dc.DrawTextureScaled(lt, lx*scale+(lw*scale-tw)/2, (ly+lh+4)*scale, tw, th)
			}
		}
	})

	app.OnClose(func() {
		if docTex != nil {
			docTex.Destroy()
		}
		if outBgTex != nil {
			outBgTex.Destroy()
		}
		if inBgTex != nil {
			inBgTex.Destroy()
		}
		if divTex != nil {
			divTex.Destroy()
		}
		for _, t := range labelTex {
			t.Destroy()
		}
	})

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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

// --- textures ----------------------------------------------------------------

func createDocumentIcon(renderer *gogpu.Renderer) (*gogpu.Texture, error) {
	const w, h = 96, 120
	img := make([]byte, w*h*4)
	set, fill := pixelHelpers(img, w, h)

	page := color.RGBA{245, 245, 247, 255}
	edge := color.RGBA{180, 182, 188, 255}
	fold := color.RGBA{210, 212, 218, 255}
	accent := color.RGBA{70, 120, 230, 255}
	line := color.RGBA{120, 140, 220, 200}

	fill(8, 4, w-8, h-8, edge)
	fill(9, 5, w-9, h-9, page)
	for dy := 0; dy < 22; dy++ {
		for dx := 0; dx < 22-dy; dx++ {
			set(w-9-dx, 5+dy, fold)
		}
	}
	fill(16, 28, w-16, 36, accent)
	for _, y := range []int{48, 58, 68, 78, 88} {
		x1 := w - 20
		if y == 88 {
			x1 = w/2 + 8
		}
		fill(16, y, x1, y+3, line)
	}
	return renderer.NewTextureFromRGBA(w, h, img)
}

func createPaneBackground(renderer *gogpu.Renderer, w, h int, title string, accent color.RGBA) (*gogpu.Texture, error) {
	img := make([]byte, w*h*4)
	set, fill := pixelHelpers(img, w, h)

	bg := color.RGBA{35, 37, 42, 255}
	header := color.RGBA{accent.R / 3, accent.G / 3, accent.B / 3, 255}
	fill(0, 0, w, h, bg)
	fill(0, 0, w, 36, header)

	drawString(set, w, h, 16, 10, title, accent)
	hint := "drag out"
	if title == "IN" {
		hint = "drop here"
	}
	drawString(set, w, h, 16, 48, hint, color.RGBA{140, 145, 155, 255})

	// Soft inner border
	border := color.RGBA{accent.R, accent.G, accent.B, 100}
	for x := 8; x < w-8; x++ {
		set(x, 40, border)
		set(x, h-9, border)
	}
	for y := 40; y < h-8; y++ {
		set(8, y, border)
		set(w-9, y, border)
	}

	return renderer.NewTextureFromRGBA(w, h, img)
}

func makeDividerTex(renderer *gogpu.Renderer) *gogpu.Texture {
	img := make([]byte, 4)
	img[0], img[1], img[2], img[3] = 90, 95, 110, 255
	t, err := renderer.NewTextureFromRGBA(1, 1, img)
	if err != nil {
		return nil
	}
	return t
}

func createLabelTexture(renderer *gogpu.Renderer, text string, c color.RGBA) (*gogpu.Texture, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		text = "?"
	}
	const scale = 2
	gw, gh := 5*scale, 7*scale
	gap := scale
	w := len([]rune(text))*(gw+gap) + gap
	h := gh + gap*2
	img := make([]byte, w*h*4)
	set, _ := pixelHelpers(img, w, h)
	drawString(set, w, h, gap, gap, text, c)
	return renderer.NewTextureFromRGBA(w, h, img)
}

func pixelHelpers(img []byte, w, h int) (set func(x, y int, c color.RGBA), fill func(x0, y0, x1, y1 int, c color.RGBA)) {
	set = func(x, y int, c color.RGBA) {
		if x < 0 || y < 0 || x >= w || y >= h {
			return
		}
		i := (y*w + x) * 4
		img[i+0], img[i+1], img[i+2], img[i+3] = c.R, c.G, c.B, c.A
	}
	fill = func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				set(x, y, c)
			}
		}
	}
	return set, fill
}

// Tiny 5x7 bitmap font for labels (A-Z, 0-9, a few symbols).
func drawString(set func(x, y int, c color.RGBA), w, h, x0, y0 int, s string, c color.RGBA) {
	const scale = 2
	cx := x0
	for _, r := range s {
		glyph := glyph5x7(unicode.ToUpper(r))
		for row := 0; row < 7; row++ {
			bits := glyph[row]
			for col := 0; col < 5; col++ {
				if bits&(1<<uint(4-col)) != 0 {
					for dy := 0; dy < scale; dy++ {
						for dx := 0; dx < scale; dx++ {
							set(cx+col*scale+dx, y0+row*scale+dy, c)
						}
					}
				}
			}
		}
		cx += 5*scale + scale
		if cx > w {
			break
		}
		_ = h
	}
}

func glyph5x7(r rune) [7]byte {
	// Each row is a 5-bit pattern in the low bits.
	m := map[rune][7]byte{
		' ': {0, 0, 0, 0, 0, 0, 0},
		'-': {0, 0, 0, 0x1F, 0, 0, 0},
		'.': {0, 0, 0, 0, 0, 0x0C, 0x0C},
		'_': {0, 0, 0, 0, 0, 0, 0x1F},
		'…': {0, 0, 0, 0, 0, 0x15, 0},
		'0': {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
		'1': {0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E},
		'2': {0x0E, 0x11, 0x01, 0x06, 0x08, 0x10, 0x1F},
		'3': {0x0E, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0E},
		'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
		'5': {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
		'6': {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
		'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
		'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
		'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C},
		'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
		'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
		'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
		'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
		'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
		'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
		'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0F},
		'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
		'I': {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
		'J': {0x01, 0x01, 0x01, 0x01, 0x11, 0x11, 0x0E},
		'K': {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
		'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
		'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
		'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
		'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
		'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
		'Q': {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
		'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
		'S': {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
		'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
		'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
		'V': {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
		'W': {0x11, 0x11, 0x11, 0x15, 0x15, 0x1B, 0x11},
		'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
		'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
		'Z': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	}
	if g, ok := m[r]; ok {
		return g
	}
	// lowercase → already uppercased by caller; unknown → box
	return [7]byte{0x1F, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1F}
}
