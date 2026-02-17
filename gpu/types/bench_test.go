package types

import (
	"runtime"
	"testing"
)

// BenchmarkBackendTypeString measures the overhead of backend type
// enum-to-string conversion, which occurs in logging and diagnostics.
func BenchmarkBackendTypeString(b *testing.B) {
	b.ReportAllocs()
	backends := []BackendType{BackendAuto, BackendNative, BackendRust}
	var result string
	for b.Loop() {
		for _, bt := range backends {
			result = bt.String()
		}
	}
	runtime.KeepAlive(result)
}

// BenchmarkGraphicsAPIString measures the overhead of graphics API
// enum-to-string conversion, which occurs in logging and diagnostics.
func BenchmarkGraphicsAPIString(b *testing.B) {
	b.ReportAllocs()
	apis := []GraphicsAPI{
		GraphicsAPIAuto, GraphicsAPIVulkan, GraphicsAPIDX12,
		GraphicsAPIMetal, GraphicsAPIGLES, GraphicsAPISoftware,
	}
	var result string
	for b.Loop() {
		for _, api := range apis {
			result = api.String()
		}
	}
	runtime.KeepAlive(result)
}

// BenchmarkBackendTypeComparison measures the cost of backend type
// comparison, used in backend selection logic.
func BenchmarkBackendTypeComparison(b *testing.B) {
	b.ReportAllocs()
	bt := BackendNative
	var result bool
	for b.Loop() {
		result = (bt == BackendRust) || (bt == BackendNative) || (bt == BackendAuto)
	}
	runtime.KeepAlive(result)
}

// BenchmarkGraphicsAPIComparison measures the cost of graphics API
// comparison, used in per-platform API selection.
func BenchmarkGraphicsAPIComparison(b *testing.B) {
	b.ReportAllocs()
	api := GraphicsAPIVulkan
	var result bool
	for b.Loop() {
		result = (api == GraphicsAPIAuto) || (api == GraphicsAPIVulkan) ||
			(api == GraphicsAPIDX12) || (api == GraphicsAPIMetal)
	}
	runtime.KeepAlive(result)
}
