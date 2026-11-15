package types

import (
	"runtime"
	"testing"
)

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
