// Example: lifecycle — demonstrates ADR-026 Universal App Lifecycle.
//
// Creates two windows. Closing either window does NOT exit the app —
// the remaining window keeps rendering. App exits only when the last
// window is closed (QuitOnLastWindowClosed default behavior).
//
// This demonstrates:
//   - Device survives window destruction (ADR-026 Phase 1)
//   - RenderTarget per window, independently closable
//   - QuitOnLastWindowClosed (ADR-026 Phase 3)
//
// Usage:
//
//	go run ./examples/lifecycle/
//	GOGPU_GRAPHICS_API=gles go run ./examples/lifecycle/
package main

import (
	"fmt"
	"log"

	"github.com/gogpu/gogpu"
)

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Lifecycle — Primary (close me first)").
		WithSize(500, 350))

	var secondaryCreated bool

	app.OnDraw(func(ctx *gogpu.Context) {
		ctx.Clear(0.15, 0.25, 0.65, 1.0) // Blue
	})

	app.OnUpdate(func(dt float64) {
		if secondaryCreated {
			return
		}
		secondaryCreated = true

		w2, err := app.NewWindow(gogpu.DefaultConfig().
			WithTitle("Lifecycle — Secondary (survives primary close)").
			WithSize(500, 350))
		if err != nil {
			log.Printf("NewWindow error: %v", err)
			return
		}

		w2.SetOnDraw(func(ctx *gogpu.Context) {
			ctx.Clear(0.65, 0.15, 0.15, 1.0) // Red
		})

		fmt.Println("Two windows created. Close the primary (blue) — secondary (red) keeps rendering.")
		fmt.Println("Close the last window to exit.")
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("App exited cleanly.")
}
