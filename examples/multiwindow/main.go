package main

import (
	"fmt"
	"log"

	"github.com/gogpu/gogpu"
)

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Primary Window").
		WithSize(600, 400))

	app.OnDraw(func(ctx *gogpu.Context) {
		ctx.Clear(0.2, 0.3, 0.8, 1.0) // Blue
	})

	// Create second window after renderer is initialized.
	// OnUpdate runs after init, so NewWindow is safe here.
	var secondCreated bool
	app.OnUpdate(func(dt float64) {
		if secondCreated {
			return
		}
		secondCreated = true

		w2, err := app.NewWindow(gogpu.DefaultConfig().
			WithTitle("Second Window").
			WithSize(400, 300))
		if err != nil {
			log.Printf("NewWindow error: %v", err)
			return
		}
		fmt.Printf("Second window created: ID=%d\n", w2.ID())

		w2.SetOnDraw(func(ctx *gogpu.Context) {
			ctx.Clear(0.8, 0.2, 0.3, 1.0) // Red
		})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
