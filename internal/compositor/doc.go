// Package compositor implements the surface compositor for the gogpu framework.
//
// The compositor owns composition textures, blit/composite pipelines, damage
// tracking, and debug overlays. It is the internal engine behind ADR-067
// (compositor-owned render target).
//
// Architecture: Struct Ownership + Callback Interface pattern
// (Flutter flow/ + Go stdlib cmd/compile/internal/ssa).
//
// Dependency direction: root package (renderer.go) → internal/compositor.
// The compositor NEVER imports the root package. When GPU resources are needed,
// the compositor calls through the GPUProvider callback interface.
package compositor
