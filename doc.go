// Package gogpu provides a simple, cross-platform GPU graphics API for Go.
//
// GoGPU is designed to make GPU programming accessible while maintaining
// the flexibility for advanced use cases. It wraps WebGPU (via wgpu-native)
// and provides a clean, Go-idiomatic API.
//
// # Quick Start
//
// The simplest gogpu program creates a window and clears it with a color:
//
//	package main
//
//	import (
//	    "log"
//	    "github.com/gogpu/gogpu"
//	)
//
//	func main() {
//	    app := gogpu.NewApp(gogpu.DefaultConfig())
//
//	    app.OnDraw(func(ctx *gogpu.Context) {
//	        ctx.Clear(0.2, 0.3, 0.4, 1.0)
//	    })
//
//	    if err := app.Run(); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// # Architecture
//
// GoGPU uses a layered architecture:
//
//   - App: Application lifecycle, window management, event dispatch
//   - Context: Drawing API available during OnDraw callback
//   - Renderer: Internal WebGPU pipeline management
//   - Platform: OS-specific windowing (internal)
//
// # Configuration
//
// Use Config to customize your application:
//
//	config := gogpu.DefaultConfig().
//	    WithTitle("My App").
//	    WithSize(1280, 720)
//
// # Callbacks
//
// GoGPU uses callbacks for the render loop:
//
//   - OnDraw(func(*Context)): Called each frame for rendering
//   - OnUpdate(func(float64)): Called each frame with delta time for logic
//   - OnResize(func(int, int)): Called when window is resized
//
// # Advanced Usage
//
// For advanced rendering, access the underlying WebGPU objects:
//
//	app.OnDraw(func(ctx *gogpu.Context) {
//	    device := ctx.Device()  // *wgpu.Device
//	    queue := ctx.Queue()    // *wgpu.Queue
//	    view := ctx.TextureView() // Current render target
//	    // Create custom pipelines, shaders, etc.
//	})
//
// # Platform Support
//
//   - Windows: Full support (Win32)
//   - macOS: Planned (Cocoa)
//   - Linux: Planned (X11/Wayland)
//
// # Dependencies
//
// GoGPU depends on:
//   - github.com/go-webgpu/webgpu - Pure Go WebGPU bindings
//   - github.com/go-webgpu/goffi - Pure Go FFI (no CGO)
package gogpu
