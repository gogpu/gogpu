package main

import (
	"log"

	"github.com/gogpu/gogpu"
)

func main() {
	app := gogpu.NewApp(gogpu.DefaultConfig().WithTitle("My App").WithAppName("Example App"))

	// Additional custom menu (initially empty, will not be displayed until items added)
	toolsMenu := gogpu.NewMenu("Tools")
	app.AddCustomMenu("tools", toolsMenu)

	// Later we add items to it
	toolsMenu.AddItem(gogpu.MenuItem{
		Title:  "Settings",
		Action: func() { log.Println("Tools -> Settings clicked") },
	})
	app.UpdateCustomMenu("tools", toolsMenu)

	// Add items to the Window menu using roles
	if windowMenu := app.GetSystemMenu(gogpu.SystemMenuWindow); windowMenu != nil {
		windowMenu.AddItem(gogpu.MenuItem{Separator: true})
		windowMenu.AddItem(gogpu.MenuItem{Title: "Minimize", Role: gogpu.RoleMinimize})
		windowMenu.AddItem(gogpu.MenuItem{Title: "Zoom", Role: gogpu.RoleZoom})
		windowMenu.AddItem(gogpu.MenuItem{Title: "Enter Full Screen", Role: gogpu.RoleFullScreen})
		windowMenu.AddItem(gogpu.MenuItem{Separator: true})
		windowMenu.AddItem(gogpu.MenuItem{Title: "Bring All to Front", Role: gogpu.RoleBringAllToFront})
		windowMenu.AddItem(gogpu.MenuItem{Title: "Close", Role: gogpu.RoleClose})
	}

	// Add custom items to the Application menu
	if appMenu := app.GetSystemMenu(gogpu.SystemMenuApplication); appMenu != nil {
		appMenu.AddItem(gogpu.MenuItem{Separator: true})
		appMenu.AddItem(gogpu.MenuItem{
			Title:  "Custom App Command",
			Action: func() { log.Println("Custom app command executed") },
		})
	}

	// Example of update handling (to keep the application running)
	app.OnUpdate(func(dt float64) {
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
