// Package gpu provides the backend abstraction layer for gogpu.
//
// This package defines the Backend interface that abstracts over different
// WebGPU implementations. Users can choose between:
//
//   - Rust backend (wgpu-native): Maximum performance, battle-tested
//   - Pure Go backend: Zero dependencies, simple cross-compilation
//
// # Backend Selection
//
// By default, gogpu auto-selects the best available backend. Users can
// explicitly choose a backend:
//
//	// Auto-select (default)
//	config := gogpu.DefaultConfig()
//
//	// Explicit Rust backend
//	config := gogpu.DefaultConfig().WithBackend(gogpu.BackendRust)
//
//	// Explicit Pure Go backend
//	config := gogpu.DefaultConfig().WithBackend(gogpu.BackendGo)
//
// # Architecture
//
// The gpu package defines:
//
//   - Backend: Interface for WebGPU operations
//   - BackendType: Enum for backend selection (Auto, Rust, Go)
//   - Handle types: Opaque references to GPU objects (Instance, Device, etc.)
//   - Configuration types: Options for adapters, devices, surfaces, pipelines
//
// # Handle Types
//
// GPU objects are represented as opaque handles (uintptr). This allows
// efficient backend switching without exposing implementation details:
//
//	type Instance uintptr
//	type Adapter uintptr
//	type Device uintptr
//	type Queue uintptr
//	type Surface uintptr
//	type Texture uintptr
//	type ShaderModule uintptr
//	type RenderPipeline uintptr
//
// # Subpackages
//
//   - gpu/backend/rust: Rust backend using go-webgpu/webgpu
//   - gpu/backend/native: Native Go backend (stub, in development)
//
// # WebGPU Compatibility
//
// This package follows the WebGPU specification where applicable.
// Types and methods are named to match WebGPU conventions while
// following Go naming conventions.
package gpu
