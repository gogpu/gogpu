// IME inspector: run go run . then switch to a Japanese or Chinese input
// source and type. Enter commits, Escape cancels, Tab toggles editor focus.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gogpu/gg"
	_ "github.com/gogpu/gg/gpu"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gg/text"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().WithTitle("GoGPU IME inspector").WithSize(900, 520).WithResizable(false))
	var mu sync.Mutex
	var committed, preedit, last string
	focused := true
	var canvas *ggcanvas.Canvas
	var font *text.FontSource
	for _, p := range []string{"/System/Library/Fonts/Supplemental/Arial Unicode.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "C:/Windows/Fonts/arial.ttf"} {
		if _, err := os.Stat(p); err == nil {
			font, err = text.NewFontSourceFromFile(p)
			if err == nil {
				break
			}
		}
	}
	if font == nil {
		log.Fatal("No supported system font found")
	}
	update := func() {
		app.SetIMESurroundingText(gpucontext.IMESurroundingText{Text: committed, Cursor: len(committed), Anchor: len(committed)})
		app.RequestRedraw()
	}
	ev := app.EventSource()
	ev.OnTextInput(func(s string) {
		log.Printf("[IME app] text=%q", s)
		mu.Lock()
		defer mu.Unlock()
		committed += s
		last = fmt.Sprintf("text: %q", s)
		update()
	})
	ev.OnIMECompositionStart(func() {
		log.Print("[IME app] composition start")
		mu.Lock()
		defer mu.Unlock()
		last = "composition started"
		app.RequestRedraw()
	})
	ev.OnIMECompositionUpdate(func(s gpucontext.IMEState) {
		log.Printf("[IME app] preedit=%q cursor=%+v selection=%+v", s.CompositionText, s.Cursor, s.Selection)
		mu.Lock()
		defer mu.Unlock()
		preedit = s.CompositionText
		last = fmt.Sprintf("preedit: %q  cursor bytes: %d..%d", preedit, s.Cursor.Start, s.Cursor.End)
		app.RequestRedraw()
	})
	ev.OnIMECompositionEnd(func(s string) {
		log.Printf("[IME app] commit=%q", s)
		mu.Lock()
		defer mu.Unlock()
		committed += s
		preedit = ""
		last = fmt.Sprintf("commit: %q", s)
		update()
	})
	if ime, ok := ev.(gpucontext.IMEEventSource); ok {
		ime.OnIMECanceled(func() { mu.Lock(); defer mu.Unlock(); preedit = ""; last = "composition canceled"; app.RequestRedraw() })
		ime.OnIMEDisabled(func() { mu.Lock(); defer mu.Unlock(); preedit = ""; last = "IME disabled"; app.RequestRedraw() })
	}
	ev.OnKeyPress(func(k gpucontext.Key, mods gpucontext.Modifiers) {
		log.Printf("[IME app] key=%v mods=%v", k, mods)
		mu.Lock()
		defer mu.Unlock()
		switch k {
		case gpucontext.KeyTab:
			focused = !focused
			app.SetIMEEnabled(focused)
			if focused {
				update()
			}
			last = fmt.Sprintf("editor focus: %v", focused)
			app.RequestRedraw()
		case gpucontext.KeyBackspace:
			if focused && preedit == "" && committed != "" {
				_, n := utf8.DecodeLastRuneInString(committed)
				committed = committed[:len(committed)-n]
				last = "backspace"
				update()
			}
		}
	})
	ev.OnFocus(func(f bool) {
		log.Printf("[IME app] window focus=%v", f)
		mu.Lock()
		defer mu.Unlock()
		if f && focused {
			update()
		}
		if strings.HasPrefix(last, "window focus:") || last == "" {
			last = fmt.Sprintf("window focus: %v", f)
		}
		app.RequestRedraw()
	})
	log.Printf("[IME app] requesting IME; capabilities=%v; set GOGPU_IME_TRACE=1 for native trace (includes typed text)", app.IMECapabilities())
	app.SetIMEEnabled(true)
	app.OnDraw(func(dc *gogpu.Context) {
		mu.Lock()
		defer mu.Unlock()
		pw, ph := app.PhysicalSize()
		if canvas == nil {
			var err error
			canvas, err = ggcanvas.New(app.GPUContextProvider(), pw, ph)
			if err != nil {
				log.Fatal(err)
			}
		}
		err := canvas.Draw(func(c *gg.Context) {
			c.SetDeviceScale(float64(pw) / 900)
			c.SetRGB(0.06, 0.08, 0.12)
			c.Clear()
			c.SetFont(font.Face(25))
			c.SetRGB(0.95, 0.96, 1)
			c.DrawString("GoGPU  /  IME inspector", 40, 58)
			c.SetFont(font.Face(15))
			c.SetRGB(0.67, 0.73, 0.82)
			c.DrawString("Switch input source in macOS, then type below.", 40, 95)
			c.DrawString("Enter: commit   Esc: cancel   Backspace: delete   Tab: toggle focus", 40, 120)
			c.SetRGB(0.14, 0.18, 0.25)
			c.DrawRoundedRectangle(40, 150, 820, 100, 8)
			c.Fill()
			c.SetFont(font.Face(26))
			c.SetRGB(0.96, 0.97, 1)
			c.DrawString(committed, 56, 208)
			w, _ := c.MeasureString(committed)
			c.SetRGB(0.38, 0.78, 1)
			c.DrawString(preedit, 56+w, 208)
			p, _ := c.MeasureString(preedit)
			if preedit != "" {
				c.DrawRectangle(56+w, 214, p, 2)
				c.Fill()
			}
			if focused {
				c.SetRGB(0.45, 0.85, 1)
				c.DrawRectangle(56+w+p, 180, 2, 32)
				c.Fill()
			}
			app.SetIMECursorArea(gpucontext.IMECursorArea{X: 56 + w + p, Y: 180, Width: 2, Height: 32})
			c.SetFont(font.Face(15))
			c.SetRGB(0.67, 0.73, 0.82)
			c.DrawString("Latest event:", 40, 300)
			c.SetRGB(0.9, 0.93, 1)
			c.DrawString(last, 40, 330)
			c.SetRGB(0.67, 0.73, 0.82)
			c.DrawString("Candidate window should appear at the blue caret.", 40, 385)
		})
		if err != nil {
			log.Print(err)
			return
		}
		if err := canvas.RenderDirect(dc.RenderTarget().SurfaceView(), uint32(pw), uint32(ph)); err != nil {
			log.Print(err)
		}
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	if canvas != nil {
		if err := canvas.Close(); err != nil {
			log.Print(err)
		}
	}
}
