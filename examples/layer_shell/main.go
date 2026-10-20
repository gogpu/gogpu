//go:build linux

// Example: a wlr-layer-shell surface.
//
// This example creates a 100x100 cyan square anchored to the top-left corner of
// the output. It is a layer surface, not a window: it has no title bar, no
// decorations, cannot be moved or resized by the user, and does not appear in
// the task switcher. The same API builds panels, overlays, wallpapers and
// launchers.
//
// Requirements: Linux with a Wayland session whose compositor advertises
// zwlr_layer_shell_v1 (niri, sway, Hyprland, river, labwc and friends).
//
// Run it:
//
//	go run ./examples/layer_shell
//	go run ./examples/layer_shell -duration=5s   # auto-quit, handy in scripts
//
// Verify it while it runs:
//
//	niri msg layers        # lists the surface, its namespace and interactivity
//	grim /tmp/layer-shell.ppm
//	                       # the top-left 100x100 of that image is cyan
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

const (
	squareSize = 100 // logical pixels, both axes
	namespace  = "gogpu-example-layer-shell"
)

func main() {
	duration := flag.Duration("duration", 0, "quit after this long (0 = until closed)")
	flag.Parse()

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("layer shell example").
		WithSize(squareSize, squareSize).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace: namespace,

			// The overlay layer draws above ordinary windows, which is where
			// launchers and notifications belong. Use LayerShellTop for panel
			// style surfaces, or LayerShellBackground for wallpapers.
			Layer: gogpu.LayerShellOverlay,

			// Anchoring to the left and top edges pins the surface to the
			// top-left corner of the output. Anchoring opposite edges instead
			// (left and right, or top and bottom) stretches it along that axis,
			// which is how a full-output overlay is built.
			Anchor: gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorTop,

			// Width and Height size the surface itself. Leaving them zero asks
			// the compositor to choose a size, which is only allowed when that
			// axis is anchored to both of its opposite edges.
			Width:  squareSize,
			Height: squareSize,

			// On-demand focus lets the user select the surface and then type
			// into it, the way a launcher needs.
			Keyboard: gogpu.LayerShellKeyboardOnDemand,

			// -1 means "never reserve space": the square floats over the desktop
			// instead of pushing other windows aside, as a panel with an
			// exclusive zone would.
			ExclusiveZone: -1,
		}))

	app.OnDraw(func(dc *gogpu.Context) {
		dc.ClearColor(gmath.Cyan)
	})

	fmt.Printf("layer shell square: %dx%d, namespace %q\n", squareSize, squareSize, namespace)
	fmt.Println("verify with: niri msg layers")

	if *duration > 0 {
		time.AfterFunc(*duration, app.Quit)
	}

	if err := app.Run(); err != nil {
		log.Printf("layer shell example failed: %v", err)
		fmt.Fprintln(os.Stderr,
			"this example needs a Wayland compositor that advertises zwlr_layer_shell_v1")
		os.Exit(1)
	}
	fmt.Println("layer shell square closed")
}
