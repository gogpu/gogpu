// Example: Window header alignment
//
// Demonstrates the three header alignment modes available in gogpu,
// mirroring Flutter's AppBar.centerTitle pattern.
//
// Run with:
//
//	go run ./examples/window_header
//
// Platform behavior:
//
//	macOS  — Left: FullSizeContentView + transparent title bar; native title
//	         visible, positioned after the traffic-light buttons (left side).
//	         Right: same transparent title bar + a right-aligned NSTextField
//	         injected into the title bar view; native title is hidden.
//	         Center: standard opaque title bar with centered native title.
//	Linux  — Wayland CSD painter positions the title text left/center/right.
//	         X11 title bar is drawn by the compositor; alignment is accepted
//	         but has no visual effect.
//	Windows — title bar is drawn by DWM; alignment is accepted but has no
//	          visual effect.
//
// Keys (focus any window):
//
//	L — switch primary window to HeaderAlignLeft
//	C — switch primary window to HeaderAlignCenter
//	R — switch primary window to HeaderAlignRight
//	T — cycle primary window title through three example strings
//	Q / Esc — quit
package main

import (
	"fmt"
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

func main() {
	// --- Primary window: HeaderAlignLeft (via Config builder) ----------------
	//
	// On macOS this enables NSWindowStyleMaskFullSizeContentView and makes the
	// title bar transparent, so the GPU surface covers the full window height.
	// The native title remains visible (macOS always centers it), while GPU
	// content fills the entire height including the title-bar zone.
	app := gogpu.NewApp(
		gogpu.DefaultConfig().
			WithTitle("Header: Left — GPU fills title bar").
			WithSize(680, 420).
			WithHeaderAlignment(gogpu.HeaderAlignLeft),
	)

	// Teal background. In a real app draw a header strip in the top ~28 pt
	// title-bar zone using ggcanvas or a custom shader.
	app.OnDraw(func(ctx *gogpu.Context) {
		ctx.Clear(0.07, 0.40, 0.45, 1)
	})

	// Title cycling state for the T key demo.
	titles := []string{
		"Header: Left — GPU fills title bar",
		"App title updated at runtime",
		"Window.SetTitle() is per-window",
	}
	titleIdx := 0

	app.EventSource().OnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) {
		switch key {
		case gpucontext.KeyL:
			app.SetHeaderAlignment(gogpu.HeaderAlignLeft)
			fmt.Println("Primary window → HeaderAlignLeft")
		case gpucontext.KeyC:
			app.SetHeaderAlignment(gogpu.HeaderAlignCenter)
			fmt.Println("Primary window → HeaderAlignCenter")
		case gpucontext.KeyR:
			app.SetHeaderAlignment(gogpu.HeaderAlignRight)
			fmt.Println("Primary window → HeaderAlignRight")
		case gpucontext.KeyT:
			titleIdx = (titleIdx + 1) % len(titles)
			app.SetTitle(titles[titleIdx])
			fmt.Printf("Primary window title → %q\n", titles[titleIdx])
		case gpucontext.KeyQ, gpucontext.KeyEscape:
			app.Quit()
		}
	})

	// Create secondary windows once the renderer is ready.
	var secondaryCreated bool
	app.OnUpdate(func(_ float64) {
		if secondaryCreated {
			return
		}
		secondaryCreated = true

		// --- Secondary window: HeaderAlignCenter (default) -------------------
		//
		// Standard macOS title bar with the title centered. This is the default
		// behavior — equivalent to omitting WithHeaderAlignment entirely.
		wCenter, err := app.NewWindow(
			gogpu.DefaultConfig().
				WithTitle("Header: Center (default)").
				WithSize(560, 360).
				WithHeaderAlignment(gogpu.HeaderAlignCenter),
		)
		if err != nil {
			log.Printf("NewWindow (center): %v", err)
			return
		}
		fmt.Printf("Center window created: ID=%d\n", wCenter.ID())

		// Purple background.
		wCenter.SetOnDraw(func(ctx *gogpu.Context) {
			ctx.Clear(0.38, 0.18, 0.55, 1)
		})

		// Press T while this window is focused to change its title independently
		// of the primary window — demonstrating per-window SetTitle().
		wCenter.SetOnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) {
			if key == gpucontext.KeyT {
				wCenter.SetTitle("Center — changed via Window.SetTitle()")
				fmt.Printf("Center window title → %q\n", wCenter.Title())
			}
		})

		// --- Secondary window: HeaderAlignRight ------------------------------
		//
		// On macOS: transparent title bar (FullSizeContentView) with the native
		// title hidden; a right-aligned NSTextField is injected into the title bar
		// view so the title appears at the right edge.
		// On Wayland: CSD painter draws the title text right-aligned.
		wRight, err := app.NewWindow(
			gogpu.DefaultConfig().
				WithTitle("Header: Right — GPU fills title bar").
				WithSize(560, 360).
				WithHeaderAlignment(gogpu.HeaderAlignRight),
		)
		if err != nil {
			log.Printf("NewWindow (right): %v", err)
			return
		}
		fmt.Printf("Right window created: ID=%d\n", wRight.ID())

		// Amber/orange background.
		wRight.SetOnDraw(func(ctx *gogpu.Context) {
			ctx.Clear(0.80, 0.45, 0.10, 1)
		})

		// Demonstrate runtime alignment switching on a secondary window.
		wRight.SetOnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) {
			switch key {
			case gpucontext.KeyL:
				wRight.SetHeaderAlignment(gogpu.HeaderAlignLeft)
				fmt.Printf("Right window → HeaderAlignLeft (ID %d)\n", wRight.ID())
			case gpucontext.KeyC:
				wRight.SetHeaderAlignment(gogpu.HeaderAlignCenter)
				fmt.Printf("Right window → HeaderAlignCenter (ID %d)\n", wRight.ID())
			case gpucontext.KeyR:
				wRight.SetHeaderAlignment(gogpu.HeaderAlignRight)
				fmt.Printf("Right window → HeaderAlignRight (ID %d)\n", wRight.ID())
			}
		})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
