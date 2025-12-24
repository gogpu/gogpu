//go:build darwin

// Package darwin implements macOS platform support via Cocoa/AppKit.
//
// This package provides a pure Go implementation of macOS windowing and
// Metal surface creation using Objective-C runtime FFI through goffi.
// No CGO is required.
//
// # Architecture
//
// The implementation uses goffi to call Objective-C runtime functions:
//   - objc_getClass: Get class references (NSApplication, NSWindow, etc.)
//   - objc_msgSend: Send messages to Objective-C objects
//   - sel_registerName: Register selector names
//
// # Main Thread Requirement (CRITICAL)
//
// macOS Cocoa/AppKit requires ALL UI operations to execute on the main thread
// (thread 0). Go's goroutine scheduler can move goroutines between OS threads,
// which would cause crashes when Cocoa methods are called from non-main threads.
//
// This package automatically calls runtime.LockOSThread() in init() to pin
// the main goroutine to the main OS thread. This ensures:
//   - NSApplication event loop works correctly
//   - NSWindow operations don't crash
//   - CAMetalLayer rendering functions properly
//
// User callbacks (OnDraw, OnUpdate) execute on the main thread. Avoid blocking
// operations in callbacks - use goroutines for background work:
//
//	app.OnUpdate(func(dt float64) {
//	    // DON'T block here - it freezes the UI
//	    go expensiveOperation() // Run in background
//	})
//
// This approach is standard across Go GUI libraries (Gio, Ebitengine, Fyne).
//
// # Memory Management
//
// Cocoa uses reference counting for memory management:
//   - Objects from alloc/init/copy must be released
//   - Objects from other methods are autoreleased
//   - Use NSAutoreleasePool for temporary objects
//
// # Metal Integration
//
// For GPU rendering, the window's content view uses a CAMetalLayer:
//   - SetWantsLayer(true) enables layer-backing
//   - SetLayer(metalLayer) attaches the Metal layer
//   - Layer provides drawables for rendering
//
// The layer must be attached to the view BEFORE setting drawable size.
// Drawable size should be set after the window is visible.
//
// # Example
//
//	platform := darwin.NewPlatform()
//	if err := platform.Init(); err != nil {
//	    return err
//	}
//	defer platform.Terminate()
//
//	window, err := platform.CreateWindow(darwin.WindowConfig{
//	    Title:  "My App",
//	    Width:  800,
//	    Height: 600,
//	})
//	if err != nil {
//	    return err
//	}
//
//	for !window.ShouldClose() {
//	    platform.PollEvents()
//	    // ... render ...
//	}
//
// # Build Tags
//
// This package only builds on darwin (macOS):
//
//	//go:build darwin
//
// # References
//
//   - Apple Cocoa Documentation: https://developer.apple.com/documentation/appkit
//   - Objective-C Runtime: https://developer.apple.com/documentation/objectivec
//   - CAMetalLayer: https://developer.apple.com/documentation/quartzcore/cametallayer
package darwin
