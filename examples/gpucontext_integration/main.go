// Example: gpucontext Integration
//
// This example demonstrates how to use the gpucontext package for
// enterprise-grade integration with external libraries like gg.
//
// gpucontext.DeviceProvider provides:
// - Device() - GPU device (gpucontext.Device interface)
// - Queue() - Command queue (gpucontext.Queue interface)
// - Adapter() - GPU adapter (gpucontext.Adapter interface)
// - SurfaceFormat() - Texture format (gpucontext.TextureFormat)
//
// gpucontext.EventSource provides input events for UI frameworks:
// - OnKeyPress/OnKeyRelease - Keyboard events
// - OnMouseMove/OnMousePress/OnMouseRelease - Mouse events
// - OnScroll - Scroll wheel events
// - OnResize - Window resize events
// - OnFocus - Focus change events
// - OnIME* - Input Method Editor events
package main

import (
	"fmt"
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gpucontext"
)

func main() {
	// Create application
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU - gpucontext Integration Example").
		WithSize(800, 600))

	// Get EventSource early (can be called before Run)
	events := app.EventSource()

	// Register event callbacks for UI frameworks
	events.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		fmt.Printf("Key pressed: %v, modifiers: %v\n", key, mods)
	})

	events.OnMouseMove(func(x, y float64) {
		// Uncomment to see all mouse movements
		// fmt.Printf("Mouse: %.1f, %.1f\n", x, y)
		_ = x
		_ = y
	})

	events.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		fmt.Printf("Mouse click: button=%v at (%.1f, %.1f)\n", button, x, y)
	})

	events.OnResize(func(width, height int) {
		fmt.Printf("Window resized: %dx%d\n", width, height)
	})

	// Track if we've printed info
	var printed bool

	// Set draw callback
	app.OnDraw(func(dc *gogpu.Context) {
		// Get gpucontext.DeviceProvider (available after initialization)
		provider := app.GPUContextProvider()
		if provider == nil {
			return // Not ready yet
		}

		// Print device info once
		if !printed {
			fmt.Println("=== gpucontext.DeviceProvider Info ===")
			fmt.Printf("Device: %T (non-nil: %v)\n", provider.Device(), provider.Device() != nil)
			fmt.Printf("Queue: %T (non-nil: %v)\n", provider.Queue(), provider.Queue() != nil)
			fmt.Printf("Adapter: %T (non-nil: %v)\n", provider.Adapter(), provider.Adapter() != nil)
			fmt.Printf("SurfaceFormat: %v\n", provider.SurfaceFormat())
			fmt.Println("======================================")
			fmt.Println()
			fmt.Println("Try clicking in the window or pressing keys!")
			fmt.Println("Close the window to exit.")
			printed = true
		}

		// Example: Using the provider with gg (if available)
		//
		//     import "github.com/gogpu/gg"
		//
		//     canvas := gg.NewGPUCanvas(provider)
		//     dc := canvas.Context()
		//     dc.SetRGB(1, 0, 0)
		//     dc.DrawCircle(400, 300, 100)
		//     dc.Fill()
		//

		// Draw something to show the window works
		if err := dc.DrawTriangleColor(gmath.CornflowerBlue); err != nil {
			log.Println("DrawTriangle:", err)
		}
	})

	// Run the application
	fmt.Println("Starting gpucontext integration example...")
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
