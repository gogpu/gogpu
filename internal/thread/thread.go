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
	"fmt"
	"os"
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

// threadResult carries a value or a panic across a goroutine boundary.
// stack holds the original panic stack captured at recover time so re-panic
// on the caller does not lose the true failure site (#481).
type threadResult struct {
	val   any
	panic any
	stack []byte
}

// recoverResult captures panic value and stack from the worker goroutine.
func recoverResult(r *threadResult) {
	if p := recover(); p != nil {
		buf := make([]byte, 64<<10)
		buf = buf[:runtime.Stack(buf, false)]
		r.panic = p
		r.stack = buf
	}
}

// rethrowPanic prints the original stack (from the worker) then re-panics
// with the original value so callers still see both traces.
func rethrowPanic(r threadResult) {
	if r.panic == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "panic on thread: %v\n%s\n", r.panic, r.stack)
	panic(r.panic)
}

// Call executes f on the thread and waits for completion.
// Returns the result from f. Any panic from f is re-panicked on the caller.
func (t *Thread) Call(f func() any) any {
	if !t.running.Load() {
		return nil
	}

	done := make(chan threadResult, 1)
	t.funcs <- func() {
		var r threadResult
		func() {
			defer recoverResult(&r)
			r.val = f()
		}()
		done <- r
	}
	r := <-done
	rethrowPanic(r)
	return r.val
}

// CallVoid executes f on the thread and waits for completion.
// Any panic from f is re-panicked on the caller.
func (t *Thread) CallVoid(f func()) {
	if !t.running.Load() {
		return
	}

	done := make(chan threadResult, 1)
	t.funcs <- func() {
		var r threadResult
		func() {
			defer recoverResult(&r)
			f()
		}()
		done <- r
	}
	r := <-done
	rethrowPanic(r)
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
