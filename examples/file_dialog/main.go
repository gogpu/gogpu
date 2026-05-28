// Demonstrates native file open/save dialogs via keyboard shortcuts.
// Run the app and press the keys shown in stdout to trigger each dialog variant.
// Results are printed to stdout.
package main

import (
	"fmt"
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

func main() {
	app := gogpu.NewApp(
		gogpu.DefaultConfig().
			WithTitle("File Dialog Demo").
			WithSize(640, 400),
	)

	imageFilter := gogpu.FileTypeFilter{
		Name:       "Images",
		Extensions: []string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp"},
	}
	textFilter := gogpu.FileTypeFilter{
		Name:       "Text files",
		Extensions: []string{"*.txt", "*.md"},
	}

	openOne := func() {
		paths, err := app.ShowOpenFileDialog(gogpu.FileDialogOptions{
			Title:   "Open File",
			Filters: []gogpu.FileTypeFilter{imageFilter, textFilter},
		})
		if err != nil {
			log.Println("open error:", err)
			return
		}
		if len(paths) == 0 {
			fmt.Println("open canceled")
			return
		}
		fmt.Println("opened:", paths[0])
	}

	openMany := func() {
		paths, err := app.ShowOpenFileDialog(gogpu.FileDialogOptions{
			Title:    "Open Files",
			Filters:  []gogpu.FileTypeFilter{imageFilter},
			Multiple: true,
		})
		if err != nil {
			log.Println("open-many error:", err)
			return
		}
		if len(paths) == 0 {
			fmt.Println("open-many canceled")
			return
		}
		fmt.Printf("opened %d file(s):\n", len(paths))
		for _, p := range paths {
			fmt.Println(" ", p)
		}
	}

	openDir := func() {
		paths, err := app.ShowOpenFileDialog(gogpu.FileDialogOptions{
			Title:     "Choose Directory",
			Directory: true,
		})
		if err != nil {
			log.Println("open-dir error:", err)
			return
		}
		if len(paths) == 0 {
			fmt.Println("open-dir canceled")
			return
		}
		fmt.Println("directory:", paths[0])
	}

	saveFile := func() {
		path, err := app.ShowSaveFileDialog(gogpu.FileDialogOptions{
			Title:           "Save File",
			Filters:         []gogpu.FileTypeFilter{textFilter},
			DefaultFilename: "output.txt",
		})
		if err != nil {
			log.Println("save error:", err)
			return
		}
		if path == "" {
			fmt.Println("save canceled")
			return
		}
		fmt.Println("save path:", path)
	}

	fmt.Println("File Dialog Demo — keyboard shortcuts:")
	fmt.Println("  1  Open File")
	fmt.Println("  2  Open Files (multi-select)")
	fmt.Println("  3  Open Directory")
	fmt.Println("  4  Save File")
	fmt.Println("  Q  Quit")

	app.EventSource().OnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) {
		switch key {
		case gpucontext.Key1:
			openOne()
		case gpucontext.Key2:
			openMany()
		case gpucontext.Key3:
			openDir()
		case gpucontext.Key4:
			saveFile()
		case gpucontext.KeyQ:
			app.Quit()
		default:
		}
	})

	app.OnUpdate(func(dt float64) {})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
