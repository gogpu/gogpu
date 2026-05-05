package main

import (
	"github.com/gogpu/gogpu"
)

func main() {
	// Create application with first window configured for native macOS tabbing.
	// TabbingPreferred tells macOS this window may be grouped into tabs.
	// TabbingIdentifier groups windows with the same string together.
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Tab 1").
		WithTabbingMode(gogpu.TabbingPreferred).
		WithTabbingIdentifier("com.example.tabs"))

	// Create the second window after the first frame has been rendered.
	// OnUpdate fires once the renderer is ready, making NewWindow safe to call.
	var created bool
	app.OnUpdate(func(dt float64) {
		if created {
			return
		}
		created = true
		_, _ = app.NewWindow(gogpu.DefaultConfig().
			WithTitle("Tab 2").
			WithTabbingMode(gogpu.TabbingPreferred).
			WithTabbingIdentifier("com.example.tabs"))
	})

	app.OnDraw(func(ctx *gogpu.Context) {
		ctx.Clear(0.1, 0.1, 0.1, 1.0)
	})

	app.Run()
}
