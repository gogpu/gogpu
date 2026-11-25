package gogpu

import "errors"

// Sentinel errors of the root package. Error identity is part of the frozen
// v1.0 shape: consumers match these with errors.Is, and every one of them is
// produced by a real path — the texture/dimension failures below, the pre-Run
// call-order errors, GPU acquisition, and the surface/device loss reports
// (P2-API-008, P2-API-009).
var (
	// ErrNotInitialized is returned when an operation is attempted before the
	// app is initialized (Run has not created the window yet).
	ErrNotInitialized = errors.New("gogpu: not initialized")

	// ErrNoGPU is returned when the app cannot obtain a usable GPU — no
	// adapter, or a device that fails creation. The error carries the backend
	// cause, so errors.Is also matches the backend's own sentinel.
	ErrNoGPU = errors.New("gogpu: no suitable GPU found")

	// ErrSurfaceLost is returned, or delivered to OnSurfaceLost, when the
	// rendering surface is lost. gogpu recovers by recreating the surface;
	// drawing calls fail with an error matching this sentinel until recovery
	// succeeds. Three consecutive failed recreates escalate to ErrDeviceLost.
	ErrSurfaceLost = errors.New("gogpu: surface lost")

	// ErrDeviceLost is returned, or delivered to OnDeviceLost, when the GPU
	// device is lost and cannot recover. Unlike ErrSurfaceLost (transient,
	// surface-level), device loss requires a restart: Run returns this error
	// after the callback.
	ErrDeviceLost = errors.New("gogpu: GPU device lost")

	// ErrTextureNil is returned when attempting to draw a nil texture.
	ErrTextureNil = errors.New("gogpu: texture is nil")

	// ErrTextureDestroyed is returned when attempting to draw a destroyed texture.
	ErrTextureDestroyed = errors.New("gogpu: texture has been destroyed")

	// ErrInvalidDimensions is returned when texture dimensions are invalid.
	ErrInvalidDimensions = errors.New("gogpu: invalid texture dimensions")

	// ErrInvalidTextureType is returned when a texture has an unexpected type.
	ErrInvalidTextureType = errors.New("gogpu: texture must be *Texture")
)
