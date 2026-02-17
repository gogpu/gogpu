package gogpu

import (
	"runtime"
	"testing"

	"github.com/gogpu/gputypes"
)

// ---------------------------------------------------------------------------
// Config benchmarks
// ---------------------------------------------------------------------------

// BenchmarkDefaultConfig measures the cost of creating a default Config.
// Called once at application startup.
func BenchmarkDefaultConfig(b *testing.B) {
	b.ReportAllocs()
	var result Config
	for b.Loop() {
		result = DefaultConfig()
	}
	runtime.KeepAlive(result)
}

// BenchmarkConfigBuilderChain measures the fluent builder pattern cost.
// Typical usage: DefaultConfig().WithTitle(...).WithSize(...).WithBackend(...)
func BenchmarkConfigBuilderChain(b *testing.B) {
	b.ReportAllocs()
	var result Config
	for b.Loop() {
		result = DefaultConfig().
			WithTitle("Benchmark App").
			WithSize(1920, 1080).
			WithBackend(BackendNative).
			WithGraphicsAPI(GraphicsAPIVulkan).
			WithContinuousRender(true)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// TextureOptions benchmarks
// ---------------------------------------------------------------------------

// BenchmarkDefaultTextureOptions measures the cost of creating default
// texture options. Called once per texture load.
func BenchmarkDefaultTextureOptions(b *testing.B) {
	b.ReportAllocs()
	var result TextureOptions
	for b.Loop() {
		result = DefaultTextureOptions()
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// bytesPerPixelForFormat benchmarks
// ---------------------------------------------------------------------------

// BenchmarkBytesPerPixelForFormat measures texture format lookup cost,
// called during texture updates (UpdateData, UpdateRegion).
func BenchmarkBytesPerPixelForFormat(b *testing.B) {
	formats := []struct {
		name   string
		format gputypes.TextureFormat
	}{
		{"RGBA8Unorm", gputypes.TextureFormatRGBA8Unorm},
		{"BGRA8Unorm", gputypes.TextureFormatBGRA8Unorm},
		{"R8Unorm", gputypes.TextureFormatR8Unorm},
		{"R32Float", gputypes.TextureFormatR32Float},
		{"RGBA32Float", gputypes.TextureFormatRGBA32Float},
	}

	for _, f := range formats {
		b.Run(f.name, func(b *testing.B) {
			b.ReportAllocs()
			var result int
			for b.Loop() {
				result = bytesPerPixelForFormat(f.format)
			}
			runtime.KeepAlive(result)
		})
	}
}

// ---------------------------------------------------------------------------
// Texture metadata benchmarks (CPU-side only, no GPU needed)
// ---------------------------------------------------------------------------

// BenchmarkTextureMetadata measures the cost of reading texture metadata.
// Called per-frame for texture binding and validation.
func BenchmarkTextureMetadata(b *testing.B) {
	tex := &Texture{
		width:  1024,
		height: 768,
		format: gputypes.TextureFormatRGBA8Unorm,
	}

	b.Run("Width", func(b *testing.B) {
		b.ReportAllocs()
		var result int
		for b.Loop() {
			result = tex.Width()
		}
		runtime.KeepAlive(result)
	})
	b.Run("Height", func(b *testing.B) {
		b.ReportAllocs()
		var result int
		for b.Loop() {
			result = tex.Height()
		}
		runtime.KeepAlive(result)
	})
	b.Run("Size", func(b *testing.B) {
		b.ReportAllocs()
		var w, h int
		for b.Loop() {
			w, h = tex.Size()
		}
		runtime.KeepAlive(w)
		runtime.KeepAlive(h)
	})
	b.Run("BytesPerPixel", func(b *testing.B) {
		b.ReportAllocs()
		var result int
		for b.Loop() {
			result = tex.BytesPerPixel()
		}
		runtime.KeepAlive(result)
	})
	b.Run("Format", func(b *testing.B) {
		b.ReportAllocs()
		var result gputypes.TextureFormat
		for b.Loop() {
			result = tex.Format()
		}
		runtime.KeepAlive(result)
	})
}

// ---------------------------------------------------------------------------
// AnimationController benchmarks
// ---------------------------------------------------------------------------

// BenchmarkAnimationControllerIsAnimating measures the cost of checking
// animation status. Called every frame in the render loop.
func BenchmarkAnimationControllerIsAnimating(b *testing.B) {
	b.ReportAllocs()
	ac := &AnimationController{}
	var result bool
	for b.Loop() {
		result = ac.IsAnimating()
	}
	runtime.KeepAlive(result)
}

// BenchmarkAnimationControllerStartStop measures the cost of starting
// and stopping animations, used for transition management.
func BenchmarkAnimationControllerStartStop(b *testing.B) {
	b.ReportAllocs()
	ac := &AnimationController{}
	for b.Loop() {
		token := ac.StartAnimation()
		token.Stop()
	}
}

// BenchmarkAnimationTokenStopIdempotent measures the cost of calling
// Stop() multiple times (must be idempotent).
func BenchmarkAnimationTokenStopIdempotent(b *testing.B) {
	b.ReportAllocs()
	ac := &AnimationController{}
	token := ac.StartAnimation()
	token.Stop() // First stop

	// Subsequent stops should be fast (no-op)
	for b.Loop() {
		token.Stop()
	}
}
