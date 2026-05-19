package main

import (
	"fmt"

	"github.com/gogpu/gogpu"
)

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().WithTitle("My App"))

	// Replacing the main menu
	app.SetMenu(gogpu.NewMenu().
		AddItem(gogpu.MenuItem{Title: "Open", Action: func() { fmt.Println("Open") }}).
		AddItem(gogpu.MenuItem{Separator: true}).
		AddItem(gogpu.MenuItem{Title: "Preferences…", Role: gogpu.RolePreferences}).
		AddItem(gogpu.MenuItem{Title: "Save", Action: func() { fmt.Println("Save") }, Enabled: false}).
		AddItem(gogpu.MenuItem{Title: "Quit", Role: gogpu.RoleQuit}),
	)

	// Add an item to the menu (the app's standard menu)
	if menu := app.GetSystemMenu(gogpu.SystemMenuApplication); menu != nil {
		menu.AddItem(gogpu.MenuItem{
			Title:  "Custom Settings…",
			Action: func() { fmt.Println("Custom Settings") },
		})
	}

	// Add an item to the Window menu
	if windowMenu := app.GetSystemMenu(gogpu.SystemMenuWindow); windowMenu != nil {
		windowMenu.AddItem(gogpu.MenuItem{
			Title:  "My Window Command",
			Action: func() { fmt.Println("Window command") },
		})
	}

	// Example of update handling (to keep the application running)
	app.OnUpdate(func(dt float64) {
	})

	app.Run()
}
