// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package thread

import (
	"sync/atomic"
)

// RenderLoop manages the render thread and deferred resize pattern.
// This implements the professional multi-thread architecture from Ebiten/Gio.
//
// The pattern:
//   - Main thread: processes window events, captures resize requests
//   - Render thread: applies resizes and performs GPU operations
//
// This prevents window "Not Responding" during heavy GPU operations.
type RenderLoop struct {
	renderThread *Thread

	// Pending resize (set by main thread, consumed by render thread)
	pendingWidth  atomic.Uint32
	pendingHeight atomic.Uint32
	resizePending atomic.Bool
}

// NewRenderLoop creates a new render loop with a dedicated render thread.
func NewRenderLoop() *RenderLoop {
	return &RenderLoop{
		renderThread: New(),
	}
}

// Stop stops the render loop and its thread.
func (rl *RenderLoop) Stop() {
	if rl.renderThread != nil {
		rl.renderThread.Stop()
	}
}

// RequestResize queues a resize to be applied on the render thread.
// This is called from the main thread when a WM_SIZE is received.
// The actual surface reconfiguration happens on the render thread.
func (rl *RenderLoop) RequestResize(width, height uint32) {
	rl.pendingWidth.Store(width)
	rl.pendingHeight.Store(height)
	rl.resizePending.Store(true)
}

// ConsumePendingResize returns the pending resize dimensions if any.
// This is called from the render thread to apply deferred resizes.
// Returns (0, 0, false) if no resize is pending.
func (rl *RenderLoop) ConsumePendingResize() (width, height uint32, ok bool) {
	if !rl.resizePending.Swap(false) {
		return 0, 0, false
	}
	return rl.pendingWidth.Load(), rl.pendingHeight.Load(), true
}

// HasPendingResize returns true if a resize is pending.
func (rl *RenderLoop) HasPendingResize() bool {
	return rl.resizePending.Load()
}

// RunOnRenderThread executes f on the render thread and returns the result.
// This blocks until f completes.
func (rl *RenderLoop) RunOnRenderThread(f func() any) any {
	return rl.renderThread.Call(f)
}

// RunOnRenderThreadVoid executes f on the render thread and waits for completion.
// Use when no return value is needed.
func (rl *RenderLoop) RunOnRenderThreadVoid(f func()) {
	rl.renderThread.CallVoid(f)
}

// RunOnRenderThreadAsync executes f on the render thread without waiting.
// Use for fire-and-forget operations.
func (rl *RenderLoop) RunOnRenderThreadAsync(f func()) {
	rl.renderThread.CallAsync(f)
}

// IsRunning returns true if the render loop is running.
func (rl *RenderLoop) IsRunning() bool {
	return rl.renderThread != nil && rl.renderThread.IsRunning()
}
