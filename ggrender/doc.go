// Package ggrender provides a GPU-accelerated renderer for gg.
//
// This package bridges gogpu's GPU capabilities with gg's 2D drawing API,
// enabling hardware-accelerated rendering while maintaining gg's simple interface.
//
// # Architecture
//
// ggrender implements the gg.Renderer interface using gogpu's DeviceProvider:
//
//	┌──────────────────────────────────────────────────────────┐
//	│                     User Application                      │
//	│                                                          │
//	│   app := gogpu.NewApp(config)                           │
//	│   app.Run() // starts main loop                         │
//	│                                                          │
//	│   // Inside OnDraw callback:                            │
//	│   gpuRenderer := ggrender.New(app.DeviceProvider())     │
//	│   dc := gg.NewContext(800, 600, gg.WithRenderer(gpuRenderer))│
//	│   dc.DrawCircle(400, 300, 100)                          │
//	│   dc.Fill()                                             │
//	│                                                          │
//	└──────────────────────────────────────────────────────────┘
//	                           │
//	                           ▼
//	┌──────────────────────────────────────────────────────────┐
//	│                      ggrender.Renderer                    │
//	│                 (implements gg.Renderer)                  │
//	│                                                          │
//	│   Fill(pixmap, path, paint) → GPU tessellation          │
//	│   Stroke(pixmap, path, paint) → GPU stroke expansion    │
//	└──────────────────────────────────────────────────────────┘
//	                           │
//	                           ▼
//	┌──────────────────────────────────────────────────────────┐
//	│                   gogpu.DeviceProvider                    │
//	│                                                          │
//	│   Backend() → gpu.Backend (rust or native)              │
//	│   Device()  → GPU device handle                         │
//	│   Queue()   → Command queue                             │
//	└──────────────────────────────────────────────────────────┘
//
// # Usage
//
// Basic usage with gogpu application:
//
//	func main() {
//	    app := gogpu.NewApp(gogpu.Config{
//	        Title:  "GPU Drawing",
//	        Width:  800,
//	        Height: 600,
//	    })
//
//	    var dc *gg.Context
//	    var gpuRenderer *ggrender.Renderer
//
//	    app.OnDraw(func(ctx *gogpu.Context) {
//	        if gpuRenderer == nil {
//	            gpuRenderer = ggrender.New(app.DeviceProvider())
//	            dc = gg.NewContext(800, 600, gg.WithRenderer(gpuRenderer))
//	        }
//
//	        dc.Clear()
//	        dc.SetColor(color.RGBA{255, 0, 0, 255})
//	        dc.DrawCircle(400, 300, 100)
//	        dc.Fill()
//	    })
//
//	    app.Run()
//	}
//
// # Implementation Status
//
// Current implementation uses software rendering as a fallback while
// GPU compute shaders are being developed. The API is stable and ready
// for use - GPU acceleration will be enabled transparently when available.
//
// # Thread Safety
//
// Renderer is NOT safe for concurrent use from multiple goroutines.
// Each goroutine should create its own Renderer instance.
package ggrender
