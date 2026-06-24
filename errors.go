package gogpu

import "errors"

// Common errors
var (
	// ErrNotInitialized is returned when operations are attempted before initialization.
	ErrNotInitialized = errors.New("gogpu: not initialized")

	// ErrPlatformNotSupported is returned on unsupported platforms.
	ErrPlatformNotSupported = errors.New("gogpu: platform not supported")

	// ErrNoGPU is returned when no suitable GPU is found.
	ErrNoGPU = errors.New("gogpu: no suitable GPU found")

	// ErrSurfaceLost is returned when the rendering surface is lost.
	ErrSurfaceLost = errors.New("gogpu: surface lost")

	// ErrWindowClosed is returned when an operation is attempted on a closed window.
	ErrWindowClosed = errors.New("gogpu: window closed")

	// ErrDeviceLost is returned when the GPU device is lost and cannot recover.
	// Unlike ErrSurfaceLost (transient, surface-level), device loss requires
	// full re-initialization of the renderer.
	ErrDeviceLost = errors.New("gogpu: GPU device lost")
)
