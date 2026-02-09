package types

// BackendType specifies which WebGPU implementation to use.
type BackendType uint8

const (
	// BackendAuto automatically selects the best available backend.
	// Pure Go is default, Rust is opt-in with -tags rust.
	BackendAuto BackendType = iota

	// BackendNative uses pure Go WebGPU implementation (gogpu/wgpu).
	// Zero dependencies, just `go build`. Default backend.
	BackendNative

	// BackendRust uses wgpu-gpu (Rust) via go-webgpu/webgpu.
	// Maximum performance, requires gpu library. Windows only.
	BackendRust

	// BackendGo is an alias for BackendNative.
	// Provided for user convenience ("I want the Go backend").
	BackendGo = BackendNative
)

// String returns the backend name.
func (b BackendType) String() string {
	switch b {
	case BackendRust:
		return "Rust (wgpu-gpu)"
	case BackendNative:
		return "Native (Pure Go)"
	default:
		return "Auto"
	}
}
