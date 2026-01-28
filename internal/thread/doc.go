// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package thread provides thread management for GPU operations.
//
// The multi-thread architecture follows the Ebiten/Gio pattern:
//
//	Main Thread (OS Thread 0)       Render Thread (Dedicated)
//	├─ runtime.LockOSThread()       ├─ runtime.LockOSThread()
//	├─ Win32/Cocoa/X11 Messages     ├─ GPU Initialization
//	├─ Window Events                ├─ ConsumePendingResize()
//	├─ RequestResize()              ├─ Surface.Configure()
//	└─ User Input                   └─ Acquire → Render → Present
//
// This separation ensures window responsiveness during heavy GPU operations
// like swapchain recreation, which requires vkDeviceWaitIdle.
//
// # Usage
//
// Create a RenderLoop and use RequestResize/ConsumePendingResize pattern:
//
//	// Main thread
//	renderLoop := thread.NewRenderLoop()
//	defer renderLoop.Stop()
//
//	// In event handler (main thread)
//	if event.Type == EventResize {
//	    renderLoop.RequestResize(width, height)
//	}
//
//	// In render loop (main thread, but GPU work on render thread)
//	renderLoop.RunOnRenderThreadVoid(func() {
//	    // Apply pending resize on render thread
//	    if w, h, ok := renderLoop.ConsumePendingResize(); ok {
//	        surface.Configure(device, w, h)
//	    }
//	    // Render frame
//	    renderFrame()
//	})
package thread
