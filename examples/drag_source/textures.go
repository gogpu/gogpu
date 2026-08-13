package main

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/gogpu/gogpu"
)

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
	img := []byte{90, 95, 110, 255}
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
	return [7]byte{0x1F, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1F}
}
