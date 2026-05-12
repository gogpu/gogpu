package main

import (
	"fmt"

	"github.com/gogpu/gogpu"
)

func main() {
	app := gogpu.NewApp(
		gogpu.DefaultConfig().
			WithTitle("Window 1"),
	)

	var created bool
	app.OnUpdate(
		func(dt float64) {
			if created {
				return
			}
			created = true
			w2, _ := app.NewWindow(
				gogpu.DefaultConfig().
					WithTitle("Window 2"),
			)
			w2.SetOnClose(
				func() bool {
					fmt.Println("Saving tab state for window ID:", w2.ID())
					return true
				},
			)
		},
	)

	app.Run()
}
