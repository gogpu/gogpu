// Package gogpu provides a cross-platform application and rendering framework
// for GoGPU applications.
//
// An App owns the platform event loop, window lifecycle, and wgpu device. The
// OnDraw callback receives a Context for the current frame:
//
//	package main
//
//	import (
//	    "log"
//	    "github.com/gogpu/gogpu"
//	)
//
//	func main() {
//	    app := gogpu.NewApp(gogpu.DefaultConfig().
//	        WithTitle("Hello GoGPU").
//	        WithSize(800, 600))
//	    app.OnDraw(func(dc *gogpu.Context) {
//	        dc.Clear(0.2, 0.3, 0.4, 1.0)
//	    })
//	    if err := app.Run(); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// Context drawing methods operate on the active frame. SurfaceView exposes
// the current render target for integrations that need zero-copy composition.
// For shared GPU resources, use App.GPUContextProvider, which returns the
// gpucontext.DeviceProvider contract consumed by gg, ui, and other packages.
//
// GoGPU uses these related packages for shared contracts and WebGPU types:
//
//   - github.com/gogpu/gpucontext — opaque device, queue, and event contracts
//   - github.com/gogpu/gputypes — WebGPU value types
//   - github.com/gogpu/wgpu — the pure-Go WebGPU implementation
//
// Backend selection is provided by wgpu build tags. The default build uses
// the pure-Go implementation; use -tags rust for the Rust FFI variant, or
// GOOS=js GOARCH=wasm for browser WebGPU. GraphicsAPI can select Vulkan,
// Metal, DX12, GLES, or software within a supported build.
//
// Window dimensions in Config, App, and Context are logical points (DIP).
// Use Context.FramebufferSize for physical device-pixel dimensions on HiDPI
// displays.
//
// Supported desktop platforms are Windows, macOS, Linux (X11 and Wayland),
// and browser/WASM. Platform-specific implementation details remain internal.
package gogpu
