//go:build darwin

package darwin

import "runtime"

// init locks the main goroutine to the main OS thread.
//
// macOS Cocoa/AppKit requires ALL UI operations to execute on the main thread
// (thread 0). Go's goroutine scheduler can move goroutines between OS threads,
// which causes crashes when Cocoa methods like nextEventMatchingMask are called
// from a non-main thread.
//
// By calling runtime.LockOSThread() in init(), we ensure that:
// 1. The main goroutine stays pinned to the main OS thread
// 2. All Cocoa operations (NSApplication, NSWindow, event loop) work correctly
// 3. CAMetalLayer and Metal rendering function properly
//
// This approach is used by all major Go GUI libraries:
// - Gio: https://gioui.org
// - Ebitengine: https://ebitengine.org
// - Fyne: https://fyne.io
// - go-gl/glfw: https://github.com/go-gl/glfw
//
// Note: This lock is permanent for the lifetime of the program.
// User callbacks (OnDraw, OnUpdate) will also execute on the main thread.
// Long-running operations should be offloaded to separate goroutines.
func init() {
	// Pin main goroutine to main OS thread for Cocoa compatibility.
	// This MUST happen before any Cocoa/AppKit calls.
	runtime.LockOSThread()
}
