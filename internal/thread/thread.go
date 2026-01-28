// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package thread provides thread abstraction for GPU operations.
// Based on Ebiten's thread architecture for professional responsiveness.
//
// Architecture:
//   - Main thread: Window events, user input (must be OS main thread on Windows/macOS)
//   - Render thread: All GPU operations (device, swapchain, commands)
//
// This separation ensures window responsiveness during heavy GPU operations
// like swapchain recreation, which requires vkDeviceWaitIdle.
package thread

import (
	"runtime"
	"sync/atomic"
)

// Thread represents a dedicated OS thread for specific operations.
// All function calls are serialized and executed on the same thread.
type Thread struct {
	funcs   chan func()
	done    chan struct{}
	running atomic.Bool
}

// New creates a new thread and starts it.
// The thread is locked to an OS thread (runtime.LockOSThread).
func New() *Thread {
	t := &Thread{
		funcs: make(chan func(), 16), // Buffered for async calls
		done:  make(chan struct{}),
	}
	t.running.Store(true)

	ready := make(chan struct{})

	go func() {
		// Lock this goroutine to an OS thread.
		// Critical for Vulkan/OpenGL context operations.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		close(ready) // Signal that thread is ready

		for {
			select {
			case f := <-t.funcs:
				f()
			case <-t.done:
				return
			}
		}
	}()

	<-ready // Wait for thread to be ready
	return t
}

// Call executes f on the thread and waits for completion.
// Returns the result from f.
func (t *Thread) Call(f func() any) any {
	if !t.running.Load() {
		return nil
	}

	done := make(chan any, 1)
	t.funcs <- func() {
		done <- f()
	}
	return <-done
}

// CallVoid executes f on the thread and waits for completion.
// Use when no return value is needed.
func (t *Thread) CallVoid(f func()) {
	if !t.running.Load() {
		return
	}

	done := make(chan struct{})
	t.funcs <- func() {
		f()
		close(done)
	}
	<-done
}

// CallAsync executes f on the thread without waiting.
// Use for fire-and-forget operations.
func (t *Thread) CallAsync(f func()) {
	if !t.running.Load() {
		return
	}

	select {
	case t.funcs <- f:
	default:
		// Channel full - execute synchronously to avoid deadlock
		t.CallVoid(f)
	}
}

// Stop stops the thread.
func (t *Thread) Stop() {
	if t.running.Swap(false) {
		close(t.done)
	}
}

// IsRunning returns true if the thread is running.
func (t *Thread) IsRunning() bool {
	return t.running.Load()
}
