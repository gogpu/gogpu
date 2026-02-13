// Package types defines GPU configuration types for gogpu.
//
// This package provides:
//   - BackendType: Enum for selecting Rust vs Pure Go backend
//
// # Backend Selection
//
// Use BackendType constants with gogpu.Config.WithBackend():
//
//	config.WithBackend(types.BackendRust)   // Rust backend
//	config.WithBackend(types.BackendNative) // Pure Go backend
//	config.WithBackend(types.BackendAuto)   // Auto-select (default)
//
// # WebGPU Types
//
// For WebGPU resource types (textures, buffers, etc.), use the HAL interfaces
// from github.com/gogpu/wgpu/hal or standard types from github.com/gogpu/gputypes.
package types
