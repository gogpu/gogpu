//go:build linux

package wayland

// CSD edge identifiers.
type CSDEdge int

const (
	CSDEdgeTop CSDEdge = iota
	CSDEdgeLeft
	CSDEdgeRight
	CSDEdgeBottom
)

// CSDHitResult identifies what region of a decoration a point falls in.
type CSDHitResult int

const (
	CSDHitNone     CSDHitResult = iota
	CSDHitCaption               // empty title bar area — drag to move
	CSDHitClose                 // close button
	CSDHitMaximize              // maximize/restore button
	CSDHitMinimize              // minimize button
	CSDHitResizeN               // top edge resize
	CSDHitResizeS               // bottom edge resize
	CSDHitResizeW               // left edge resize
	CSDHitResizeE               // right edge resize
	CSDHitResizeNW              // top-left corner resize
	CSDHitResizeNE              // top-right corner resize
	CSDHitResizeSW              // bottom-left corner resize
	CSDHitResizeSE              // bottom-right corner resize
)

// CSDButtonState tracks interaction state for a control button.
type CSDButtonState struct {
	Hovered bool
	Pressed bool
}

// CSDState holds the current decoration state for painting.
type CSDState struct {
	Title     string
	Focused   bool
	Maximized bool
	Close     CSDButtonState
	Maximize  CSDButtonState
	Minimize  CSDButtonState
}

// CSDPainter renders CSD decoration pixels into ARGB8888 buffers.
type CSDPainter interface {
	// TitleBarHeight returns the title bar height in pixels.
	TitleBarHeight() int

	// BorderWidth returns the side/bottom border width in pixels.
	BorderWidth() int

	// PaintTitleBar renders the title bar into an ARGB8888 buffer.
	// buf must be width*height*4 bytes. Stride = width*4.
	PaintTitleBar(buf []byte, width, height int, state CSDState)

	// PaintBorder renders a side/bottom border into an ARGB8888 buffer.
	PaintBorder(buf []byte, width, height int, edge CSDEdge)

	// HitTestTitleBar determines what the pointer is over in the title bar.
	HitTestTitleBar(x, y, width, height int) CSDHitResult
}

// DefaultCSDPainter provides a dark-themed title bar matching ui/core/titlebar/DefaultPainter.
type DefaultCSDPainter struct{}

const (
	defaultTitleBarHeight = 32
	defaultBorderWidth    = 4
	defaultButtonWidth    = 46
	defaultCornerSize     = 8 // corner resize grip
	defaultIconSize       = 10
)

// Colors (ARGB8888 little-endian: B, G, R, A in memory)
var (
	colorTitleBarBg      = [4]byte{0x30, 0x2D, 0x2B, 0xFF} // #2B2D30
	colorTitleBarUnfocus = [4]byte{0x24, 0x22, 0x20, 0xFF} // #202224
	colorBorderBg        = [4]byte{0x30, 0x2D, 0x2B, 0xFF} // #2B2D30
	colorButtonHover     = [4]byte{0x44, 0x41, 0x3E, 0xFF} // #3E4144
	colorButtonPress     = [4]byte{0x38, 0x35, 0x33, 0xFF} // #333538
	colorCloseHover      = [4]byte{0x1C, 0x2B, 0xC4, 0xFF} // #C42B1C
	colorClosePress      = [4]byte{0x1A, 0x2A, 0xB2, 0xFF} // #B22A1A
	colorIconFg          = [4]byte{0xE5, 0xE1, 0xDF, 0xFF} // #DFE1E5
	colorIconCloseHover  = [4]byte{0xFF, 0xFF, 0xFF, 0xFF} // #FFFFFF
)

func (DefaultCSDPainter) TitleBarHeight() int { return defaultTitleBarHeight }
func (DefaultCSDPainter) BorderWidth() int    { return defaultBorderWidth }

func (DefaultCSDPainter) PaintTitleBar(buf []byte, width, height int, state CSDState) {
	// Background
	bg := colorTitleBarBg
	if !state.Focused {
		bg = colorTitleBarUnfocus
	}
	fillRect(buf, width, 0, 0, width, height, bg)

	// Control buttons (right-aligned): [minimize] [maximize] [close]
	btnW := defaultButtonWidth
	closeX := width - btnW
	maxX := closeX - btnW
	minX := maxX - btnW

	// Minimize button
	if state.Minimize.Pressed {
		fillRect(buf, width, minX, 0, btnW, height, colorButtonPress)
	} else if state.Minimize.Hovered {
		fillRect(buf, width, minX, 0, btnW, height, colorButtonHover)
	}
	drawMinimizeIcon(buf, width, minX, 0, btnW, height, colorIconFg)

	// Maximize button
	if state.Maximize.Pressed {
		fillRect(buf, width, maxX, 0, btnW, height, colorButtonPress)
	} else if state.Maximize.Hovered {
		fillRect(buf, width, maxX, 0, btnW, height, colorButtonHover)
	}
	if state.Maximized {
		drawRestoreIcon(buf, width, maxX, 0, btnW, height, colorIconFg)
	} else {
		drawMaximizeIcon(buf, width, maxX, 0, btnW, height, colorIconFg)
	}

	// Close button
	if state.Close.Pressed {
		fillRect(buf, width, closeX, 0, btnW, height, colorClosePress)
	} else if state.Close.Hovered {
		fillRect(buf, width, closeX, 0, btnW, height, colorCloseHover)
		drawCloseIcon(buf, width, closeX, 0, btnW, height, colorIconCloseHover)
	} else {
		drawCloseIcon(buf, width, closeX, 0, btnW, height, colorIconFg)
	}

	// Title text (simple: draw nothing for now, text rendering requires font support)
	// TODO: add simple bitmap font for title text
}

func (DefaultCSDPainter) PaintBorder(buf []byte, width, height int, _ CSDEdge) {
	fillRect(buf, width, 0, 0, width, height, colorBorderBg)
}

func (DefaultCSDPainter) HitTestTitleBar(x, y, width, height int) CSDHitResult {
	btnW := defaultButtonWidth

	// Resize grip at top edge
	if y < defaultCornerSize {
		if x < defaultCornerSize {
			return CSDHitResizeNW
		}
		if x >= width-defaultCornerSize {
			return CSDHitResizeNE
		}
		return CSDHitResizeN
	}

	// Control buttons (right-aligned)
	closeX := width - btnW
	maxX := closeX - btnW
	minX := maxX - btnW

	if x >= closeX {
		return CSDHitClose
	}
	if x >= maxX {
		return CSDHitMaximize
	}
	if x >= minX {
		return CSDHitMinimize
	}

	return CSDHitCaption
}

// --- Pixel drawing primitives (ARGB8888, stride = width*4) ---

func fillRect(buf []byte, stride, x, y, w, h int, color [4]byte) {
	for row := y; row < y+h && row*stride*4 < len(buf); row++ {
		for col := x; col < x+w; col++ {
			off := (row*stride + col) * 4
			if off+3 < len(buf) {
				buf[off] = color[0]
				buf[off+1] = color[1]
				buf[off+2] = color[2]
				buf[off+3] = color[3]
			}
		}
	}
}

func setPixel(buf []byte, stride, x, y int, color [4]byte) {
	off := (y*stride + x) * 4
	if off+3 < len(buf) {
		buf[off] = color[0]
		buf[off+1] = color[1]
		buf[off+2] = color[2]
		buf[off+3] = color[3]
	}
}

func drawHLine(buf []byte, stride, x1, x2, y int, color [4]byte) {
	for x := x1; x <= x2; x++ {
		setPixel(buf, stride, x, y, color)
	}
}

func drawVLine(buf []byte, stride, x, y1, y2 int, color [4]byte) {
	for y := y1; y <= y2; y++ {
		setPixel(buf, stride, x, y, color)
	}
}

// drawMinimizeIcon draws a horizontal line (—) centered in the button area.
func drawMinimizeIcon(buf []byte, stride, bx, by, bw, bh int, color [4]byte) {
	cx := bx + bw/2
	cy := by + bh/2
	half := defaultIconSize / 2
	drawHLine(buf, stride, cx-half, cx+half, cy, color)
}

// drawMaximizeIcon draws a square outline centered in the button area.
func drawMaximizeIcon(buf []byte, stride, bx, by, bw, bh int, color [4]byte) {
	cx := bx + bw/2
	cy := by + bh/2
	half := defaultIconSize / 2
	// Top and bottom edges
	drawHLine(buf, stride, cx-half, cx+half, cy-half, color)
	drawHLine(buf, stride, cx-half, cx+half, cy+half, color)
	// Left and right edges
	drawVLine(buf, stride, cx-half, cy-half, cy+half, color)
	drawVLine(buf, stride, cx+half, cy-half, cy+half, color)
}

// drawRestoreIcon draws two overlapping squares (restore from maximize).
func drawRestoreIcon(buf []byte, stride, bx, by, bw, bh int, color [4]byte) {
	cx := bx + bw/2
	cy := by + bh/2
	half := defaultIconSize/2 - 1
	off := 2
	// Back square (shifted up-right)
	drawHLine(buf, stride, cx-half+off, cx+half+off, cy-half-off, color)
	drawHLine(buf, stride, cx-half+off, cx+half+off, cy+half-off, color)
	drawVLine(buf, stride, cx-half+off, cy-half-off, cy+half-off, color)
	drawVLine(buf, stride, cx+half+off, cy-half-off, cy+half-off, color)
	// Front square
	drawHLine(buf, stride, cx-half, cx+half, cy-half, color)
	drawHLine(buf, stride, cx-half, cx+half, cy+half, color)
	drawVLine(buf, stride, cx-half, cy-half, cy+half, color)
	drawVLine(buf, stride, cx+half, cy-half, cy+half, color)
}

// drawCloseIcon draws an X shape centered in the button area.
func drawCloseIcon(buf []byte, stride, bx, by, bw, bh int, color [4]byte) {
	cx := bx + bw/2
	cy := by + bh/2
	half := defaultIconSize / 2
	// Draw two diagonal lines
	for i := -half; i <= half; i++ {
		setPixel(buf, stride, cx+i, cy+i, color)
		setPixel(buf, stride, cx+i, cy-i, color)
		// Thicken by 1px
		setPixel(buf, stride, cx+i+1, cy+i, color)
		setPixel(buf, stride, cx+i+1, cy-i, color)
	}
}

// Compile-time check.
var _ CSDPainter = DefaultCSDPainter{}
