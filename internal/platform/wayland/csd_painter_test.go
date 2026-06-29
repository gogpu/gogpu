//go:build linux

package wayland

import "testing"

// titleStartX scans the buffer left-to-right in columns [0, beforeX) and
// rows [yLo, yHi) and returns the leftmost column that contains a colorIconFg
// pixel. Returns -1 when no text pixel is found.
func titleStartX(buf []byte, stride, beforeX, yLo, yHi int) int {
	for col := 0; col < beforeX; col++ {
		for row := yLo; row < yHi; row++ {
			off := (row*stride + col) * 4
			if off+3 < len(buf) &&
				buf[off] == colorIconFg[0] &&
				buf[off+1] == colorIconFg[1] &&
				buf[off+2] == colorIconFg[2] &&
				buf[off+3] == colorIconFg[3] {
				return col
			}
		}
	}
	return -1
}

func TestPaintTitleBarAlignment(t *testing.T) {
	const (
		winH     = defaultTitleBarHeight // 32
		btnW     = defaultButtonWidth    // 46
		glyphW   = 7
		leftPad  = 12
		rightPad = 12
	)
	// 'B' glyph (char 66): rows 2-8 are 0xF0/0x88, so column 0 is always lit.
	// Using 'B' ensures the leftmost glyph pixel lands exactly at titleX.
	const wideTitle = "BB"
	const narrowTitle = "BBBBBBBBBBBBBBBBBBBBBBBB" // 24 chars = 168 px wide

	painter := DefaultCSDPainter{}
	yOff := (winH - 13) / 2 // vertical start of glyph rows in the buffer

	tests := []struct {
		name      string
		winW      int
		title     string
		alignment int // 0=center, 1=left, 2=right
		wantX     int
	}{
		{
			name:      "left",
			winW:      600,
			title:     wideTitle,
			alignment: 1,
			wantX:     leftPad,
		},
		{
			name:      "center_wide",
			winW:      600,
			title:     wideTitle,
			alignment: 0,
			// titleX = (600 - 2*7) / 2 = 293; fits without overlap
			wantX: (600 - len(wideTitle)*glyphW) / 2,
		},
		{
			name:      "center_narrow_fallback",
			winW:      300,
			title:     narrowTitle,
			alignment: 0,
			// titleX = (300 - 168) / 2 = 66; 66+168=234 > minX(162)-rightPad(12)=150 → fallback
			wantX: leftPad,
		},
		{
			name:      "right_wide",
			winW:      600,
			title:     wideTitle,
			alignment: 2,
			// minX = 600 - 3*46 = 462; titleX = 462 - 12 - 14 = 436
			wantX: (600 - 3*btnW) - rightPad - len(wideTitle)*glyphW,
		},
		{
			name:      "right_narrow_fallback",
			winW:      200,
			title:     narrowTitle,
			alignment: 2,
			// minX = 200 - 138 = 62; titleX = 62 - 12 - 168 = -118 < leftPad → fallback
			wantX: leftPad,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.winW*winH*4)
			state := CSDState{
				Title:          tc.title,
				TitleAlignment: tc.alignment,
				Focused:        true,
			}
			painter.PaintTitleBar(buf, tc.winW, winH, state)

			minX := tc.winW - 3*btnW
			got := titleStartX(buf, tc.winW, minX, yOff, yOff+13)
			if got != tc.wantX {
				t.Errorf("alignment=%d winW=%d title=%q: first text pixel at col %d, want %d",
					tc.alignment, tc.winW, tc.title, got, tc.wantX)
			}
		})
	}
}
