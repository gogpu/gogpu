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
)
