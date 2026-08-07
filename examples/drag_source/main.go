// Package main demonstrates outgoing drag-and-drop (drag source).
//
// Click and hold inside the window, then drag to the desktop or another app.
// The console shows drag events and results.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/input"
)

func main() {
	tmpDir, err := os.MkdirTemp("", "gogpu-drag-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "hello-from-gogpu.txt")
	content := fmt.Sprintf("Hello from GoGPU drag-and-drop!\nTimestamp: %s\n", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Temp file: %s\n", tmpFile)
	fmt.Println("Click inside the window, hold, and drag to desktop.")
	fmt.Println("Release on desktop to drop the file.")

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Drag Source Test").
		WithSize(400, 200).
		WithContinuousRender(true))

	app.OnDragDrop(func(paths []string, x, y float64) {
		fmt.Printf("[drop] received %d file(s) at (%.1f, %.1f):\n", len(paths), x, y)
		for i, path := range paths {
			fmt.Printf("  [%d]: %s\n", i, path)
		}
	})

	dragging := false

	app.OnUpdate(func(dt float64) {
		mouse := app.Input().Mouse()

		if mouse.JustPressed(input.MouseButtonLeft) {
			dragging = true
			fmt.Println("[mouse] button down — hold and drag")
		}

		if dragging && mouse.Pressed(input.MouseButtonLeft) {
			dx, dy := mouse.Delta()
			if dx != 0 || dy != 0 {
				dragging = false
				fmt.Println("[drag] starting...")

				app.StartDrag(gogpu.DragData{
					FilePaths: []string{tmpFile},
				}, func(result gogpu.DragResult) {
					switch result {
					case gogpu.DragCopied:
						fmt.Println("[drag] result: COPIED")
					case gogpu.DragMoved:
						fmt.Println("[drag] result: MOVED")
					default:
						fmt.Println("[drag] result: CANCELED")
					}
				})
			}
		}

		if mouse.JustReleased(input.MouseButtonLeft) {
			dragging = false
		}
	})

	app.OnDraw(func(ctx *gogpu.Context) {})

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
