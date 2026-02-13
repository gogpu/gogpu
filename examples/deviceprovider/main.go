// Example: DeviceProvider Interface
//
// This example demonstrates how to use the DeviceProvider interface
// to access GPU resources for integration with external libraries.
//
// DeviceProvider exposes:
// - Device() - HAL GPU device
// - Queue() - HAL command queue
// - SurfaceFormat() - Preferred texture format
package main

import (
	"fmt"
	"log"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

func main() {
	// Create application
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("GoGPU - DeviceProvider Example").
		WithSize(800, 600))

	// Track if we've printed info
	var printed bool

	// Set draw callback
	app.OnDraw(func(dc *gogpu.Context) {
		// Get DeviceProvider (available after first frame initialization)
		provider := app.DeviceProvider()
		if provider == nil {
			return // Not ready yet
		}

		// Print device info once
		if !printed {
			fmt.Println("=== DeviceProvider Info ===")
			fmt.Printf("Device: %v\n", provider.Device())
			fmt.Printf("Queue: %v\n", provider.Queue())
			fmt.Printf("Surface Format: %v\n", provider.SurfaceFormat())
			fmt.Println("===========================")
			printed = true
		}

		// Example: Access HAL device/queue directly for advanced operations
		// device := provider.Device()
		// queue := provider.Queue()
		//
		// This enables integrating with external libraries that need
		// direct GPU access without creating circular dependencies.

		// Draw something to show the window works
		if err := dc.DrawTriangleColor(gmath.CornflowerBlue); err != nil {
			log.Println("DrawTriangle:", err)
		}
	})

	// Run the application
	fmt.Println("Starting DeviceProvider example...")
	fmt.Println("Close the window to exit.")
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
